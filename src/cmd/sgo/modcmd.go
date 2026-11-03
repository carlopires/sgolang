// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// sgo mod sum and sgo mod verify (SGO-IMP-003, D113): sgo.sum is written only by sgo mod sum,
// never by a build.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func cmdMod(args []string) error {
	if len(args) != 1 || args[0] != "sum" && args[0] != "verify" {
		return usage("usage: sgo mod sum | sgo mod verify")
	}
	t, err := findToolchain()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = r
	}
	m, err := findModule(cwd, t)
	if err != nil {
		return err
	}
	entries, err := m.requiredDigests(t)
	if err != nil {
		return err
	}
	if args[0] == "sum" {
		var b strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&b, "%s %s %s\n", e[0], e[1], e[2])
		}
		return os.WriteFile(filepath.Join(m.root, "sgo.sum"), []byte(b.String()), 0o666)
	}
	sums, err := readSums(m.root)
	if err != nil {
		return err
	}
	bad := 0
	for _, e := range entries {
		switch want, ok := sums[e[0]+" "+e[1]]; {
		case !ok:
			fmt.Fprintf(os.Stderr, "sgo.sum:1:1: SGO_E_MODULE_SUM: no sgo.sum entry for %s %s\n", e[0], e[1])
			bad++
		case want != e[2]:
			fmt.Fprintf(os.Stderr, "sgo.sum:1:1: SGO_E_MODULE_SUM: %s %s: digest %s, sgo.sum records %s\n", e[0], e[1], e[2], want)
			bad++
		}
	}
	if bad > 0 {
		return buildFailed
	}
	fmt.Printf("sgo mod verify: %d module(s) verified\n", len(entries))
	return nil
}

// requiredDigests resolves the required modules (versions, grants, directories) without the
// sgo.sum check and returns [path, version, digest] for each, sorted by path.
func (m *module) requiredDigests(t *toolchain) ([][3]string, error) {
	mods, versions, err := m.walkRequires(t)
	if err != nil {
		return nil, err
	}
	var out [][3]string
	for p, v := range versions {
		d, err := modDigest(mods[p].root)
		if err != nil {
			return nil, err
		}
		out = append(out, [3]string{p, v, d})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out, nil
}
