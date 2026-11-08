// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// trustInfo is the build-ID field that records the certificate trust a program was built with
// (D201 (c)): for a program that links std/crypto/x509, the number and digest of the keys that
// package distrusts, and, if it links std/crypto/x509/roots, the root bundle's version and digest.
// The values are read from the standard library sources, which the MANIFEST check has verified.
// A program that links neither has no trust field.
func (t *toolchain) trustInfo(archives map[string]string) (string, error) {
	if _, ok := archives["std/crypto/x509"]; !ok {
		return "", nil
	}
	std := filepath.Join(t.goroot, "src", "sgostd", "crypto", "x509")
	src, err := os.ReadFile(filepath.Join(std, "distrust.sgo"))
	if err != nil {
		return "", err
	}
	keys := regexp.MustCompile(`"([0-9a-f]{64})"`).FindAllStringSubmatch(string(src), -1)
	list := make([]string, len(keys))
	for i, k := range keys {
		list[i] = k[1]
	}
	sort.Strings(list)
	sum := sha256.Sum256([]byte(strings.Join(list, "\n")))
	field := fmt.Sprintf("distrust:%d:%s", len(list), hex.EncodeToString(sum[:8]))
	if _, ok := archives["std/crypto/x509/roots"]; ok {
		b, err := os.ReadFile(filepath.Join(std, "roots", "bundle.sgo"))
		if err != nil {
			return "", err
		}
		v := regexp.MustCompile(`const BundleVersion = "([^"]+)"`).FindStringSubmatch(string(b))
		h := regexp.MustCompile(`const BundleSHA256 = "([0-9a-f]{64})"`).FindStringSubmatch(string(b))
		if v == nil || h == nil {
			return "", fmt.Errorf("std/crypto/x509/roots: no BundleVersion or BundleSHA256")
		}
		field = fmt.Sprintf("roots:%s:%s,%s", v[1], h[1][:16], field)
	}
	return "/trust=" + field, nil
}

// cmdVersionM prints the build metadata of S-Go binaries, one field per line (SGO-TOOL-001,
// D201 (c)): the build ID the toolchain recorded when it linked each.
func cmdVersionM(files []string) error {
	if len(files) == 0 {
		return usage("sgo version -m takes one or more binaries")
	}
	for _, f := range files {
		id, err := readBuildID(f)
		if err != nil {
			return &toolErr{f + ":1:1", "SGO_E_USAGE", fmt.Sprintf("%s: cannot read the build ID: %v", f, err), 1}
		}
		if !strings.HasPrefix(id, "sgo1/") {
			return &toolErr{f + ":1:1", "SGO_E_USAGE", fmt.Sprintf("%s is not an S-Go binary", f), 1}
		}
		fmt.Printf("%s: sgo1\n", f)
		fields := []string{}
		for _, part := range strings.Split(strings.TrimPrefix(id, "sgo1/"), "/") {
			if strings.Contains(part, "=") || len(fields) == 0 {
				fields = append(fields, part)
			} else {
				fields[len(fields)-1] += "/" + part
			}
		}
		for _, kv := range fields {
			k, v, _ := strings.Cut(kv, "=")
			if k == "trust" {
				for _, item := range strings.Split(v, ",") {
					fmt.Printf("\ttrust\t%s\n", strings.Replace(item, ":", " ", 1))
				}
				continue
			}
			fmt.Printf("\t%s\t%s\n", k, v)
		}
	}
	return nil
}

// readBuildID returns the build ID the Go linker wrote in an executable: ELF carries it in the
// .note.go.buildid note; Mach-O and PE carry it as the text-segment marker
// "\xff Go build ID: \"...\"\n \xff".
func readBuildID(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	if ef, err := elf.NewFile(bytes.NewReader(data)); err == nil {
		sec := ef.Section(".note.go.buildid")
		if sec == nil {
			return "", fmt.Errorf("no .note.go.buildid section")
		}
		note, err := sec.Data()
		if err != nil {
			return "", err
		}
		// namesz, descsz, type (4 bytes each), the name "Go\x00\x00", then the ID.
		if len(note) < 16 || string(note[12:16]) != "Go\x00\x00" {
			return "", fmt.Errorf("malformed .note.go.buildid")
		}
		n := int(ef.ByteOrder.Uint32(note[4:8]))
		if len(note) < 16+n {
			return "", fmt.Errorf("malformed .note.go.buildid")
		}
		return string(note[16 : 16+n]), nil
	}
	start := []byte("\xff Go build ID: \"")
	i := bytes.Index(data, start)
	if i < 0 {
		return "", fmt.Errorf("no build ID")
	}
	rest := data[i+len(start):]
	j := bytes.Index(rest, []byte("\"\n \xff"))
	if j < 0 {
		return "", fmt.Errorf("malformed build ID")
	}
	return string(rest[:j]), nil
}
