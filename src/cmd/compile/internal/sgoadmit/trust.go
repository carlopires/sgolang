// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgoadmit

// Trust rules for target code (SGO-TCB-002). This file imports only the standard library so that
// the host test lane can test it in isolation (test-host-pkg.sh --only, W12 R1).

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"strings"
)

// IsTCBPackage reports whether pkgpath is the import path of a TCB package (SGO-TCB-002).
func IsTCBPackage(pkgpath string) bool {
	return slices.Contains(strings.Fields(tcbPackageList), pkgpath)
}

// Trusted reports whether the package is trusted target code: its import path is a TCB package
// and every file is one of that package's manifest files with the pinned digest. Trust is a
// function of (import path, file name, digest), never of content alone: a byte copy of TCB files
// under another path is ordinary S-Go code and goes through admission (SGO-TCB-002, W12 R1).
func Trusted(pkgpath string, filenames []string) bool {
	if len(filenames) == 0 || !IsTCBPackage(pkgpath) {
		return false
	}
	files := tcbFiles()
	for _, f := range filenames {
		if msg := tcbFileOK(files, pkgpath, f); msg != "" {
			return false
		}
	}
	return true
}

func tcbFiles() map[string]string {
	files := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(tcbFileList), "\n") {
		if name, digest, ok := strings.Cut(line, " "); ok {
			files[name] = digest
		}
	}
	return files
}

// tcbFileOK checks one file of TCB package pkgpath against the manifest: its manifest name
// pkgpath/basename must exist with the file's digest. It returns "" or the problem. CheckTCB and
// Trusted share it so that the two cannot drift apart.
func tcbFileOK(files map[string]string, pkgpath, f string) string {
	name := pkgpath + "/" + f[strings.LastIndexAny(f, `/\`)+1:]
	want, ok := files[name]
	if !ok {
		return "TCB package " + pkgpath + ": file " + name + " is not in sgo/TARGET_TCB.json"
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return "TCB package " + pkgpath + ": " + err.Error()
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want {
		return "TCB package " + pkgpath + ": file " + name + " differs from its digest in sgo/TARGET_TCB.json"
	}
	return ""
}

// CheckTCB verifies the files of a TCB package against the manifest (SGO-TCB-002): if pkgpath is
// the import path of a TCB package, every file must be one of its manifest files with the pinned
// digest. It returns the first file that is not, with the SGO_E_TCB_DIGEST message, else "". Other
// packages are not TCB packages and are checked by admission. A TCB package path is never
// compiled as ordinary S-Go code: nothing is built from unreviewed runtime source.
func CheckTCB(pkgpath string, filenames []string) (file, msg string) {
	if !IsTCBPackage(pkgpath) {
		return "", ""
	}
	files := tcbFiles()
	for _, f := range filenames {
		if msg := tcbFileOK(files, pkgpath, f); msg != "" {
			return f, msg
		}
	}
	return "", ""
}
