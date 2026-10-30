// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// toolchain locates the fork's GOROOT and stage-0 tools. The driver is installed in
// $GOROOT/pkg/tool/<host>/, next to compile, asm, and link.
type toolchain struct {
	goroot      string
	tooldir     string
	version     string // S-Go toolchain identity (digest of the tool binaries)
	specVersion string // sgo/spec/VERSION
	admission   string // SHA-256 of sgo/SGO_SPEC_ADMISSION.json
	upstream    string // locked upstream commit
}

// admittedTargets are the targets of SGO-BUILD-001 (D22).
var admittedTargets = map[string]bool{"darwin/arm64": true, "linux/amd64": true, "linux/arm64": true}

func findToolchain() (*toolchain, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	t := &toolchain{tooldir: filepath.Dir(exe)}
	t.goroot = filepath.Clean(filepath.Join(t.tooldir, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(t.goroot, "src", "runtime", "sgo_scope.go")); err != nil {
		return nil, fmt.Errorf("sgo: %s is not an S-Go GOROOT (sgo must be installed in $GOROOT/pkg/tool/<os>_<arch>)", t.goroot)
	}
	h := sha256.New()
	for _, name := range []string{"compile", "asm", "link", "sgo"} {
		data, err := os.ReadFile(filepath.Join(t.tooldir, name))
		if err != nil {
			return nil, fmt.Errorf("sgo: missing tool %s: %v", name, err)
		}
		h.Write(data)
	}
	t.version = "devel-" + hex.EncodeToString(h.Sum(nil))[:12]

	v, err := os.ReadFile(filepath.Join(t.goroot, "sgo", "spec", "VERSION"))
	if err != nil {
		return nil, fmt.Errorf("sgo: spec version: %v", err)
	}
	t.specVersion = strings.TrimSpace(string(v))
	adm, err := os.ReadFile(filepath.Join(t.goroot, "sgo", "SGO_SPEC_ADMISSION.json"))
	if err != nil {
		return nil, fmt.Errorf("sgo: admission registry: %v", err)
	}
	sum := sha256.Sum256(adm)
	t.admission = hex.EncodeToString(sum[:])[:16]
	var lock struct {
		Upstream struct{ Commit string } `json:"upstream"`
	}
	data, err := os.ReadFile(filepath.Join(t.goroot, "sgo", "UPSTREAM.lock.json"))
	if err == nil {
		err = json.Unmarshal(data, &lock)
	}
	if err != nil {
		return nil, fmt.Errorf("sgo: upstream lock: %v", err)
	}
	t.upstream = lock.Upstream.Commit
	return t, nil
}

func (t *toolchain) tool(name string) string { return filepath.Join(t.tooldir, name) }

// checkTarget validates -target (SGO-BUILD-001); the default is the host.
func checkTarget(target string) (goos, goarch string, err error) {
	if target == "" {
		target = runtime.GOOS + "/" + runtime.GOARCH
	}
	if !admittedTargets[target] {
		return "", "", &toolErr{"sgo:1:1", "SGO_E_TARGET", fmt.Sprintf("target %s is not admitted (darwin/arm64, linux/amd64, linux/arm64)", target), 2}
	}
	goos, goarch, _ = strings.Cut(target, "/")
	return goos, goarch, nil
}

// toolEnv is the complete environment of every tool the driver runs. Nothing from the user's
// environment reaches the compiler, assembler, or linker (SGO-TOOL-001).
func (t *toolchain) toolEnv(goos, goarch string) []string {
	env := []string{"GOROOT=" + t.goroot, "GOOS=" + goos, "GOARCH=" + goarch, "GOEXPERIMENT=", "GODEBUG=",
		"CGO_ENABLED=0", "PATH=/usr/bin:/bin", "LANG=C"}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	switch goarch {
	case "amd64":
		env = append(env, "GOAMD64=v1")
	case "arm64":
		env = append(env, "GOARM64=v8.0")
	}
	return env
}
