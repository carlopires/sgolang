# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""sgotool.py - helpers for test scripts that run the shipped compile and link directly.

The linker requires a -buildid in the driver's format that agrees with the link (W12 R3,
SGO-BUILD-001); buildid() writes one for the toolchain under test.
"""
import os
import platform
import re

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]


def identity():
    """Spec version and admission digest the toolchain was built for (generated identity.go)."""
    src = open(os.path.join(ROOT, "src", "cmd", "sgo", "identity.go")).read()
    get = lambda k: re.search(k + r'\s*=\s*"([^"]*)"', src).group(1)
    return get("SpecVersion"), get("AdmissionDigest")


def buildid(config=None, drain="none", grants="none"):
    """The -buildid flag for a direct link, in the driver's format (build.go buildInfo)."""
    spec, adm = identity()
    c = f"/config={config}" if config else ""
    return (f"-buildid=sgo1/spec={spec}/admission={adm}{c}/drain={drain}/grants={grants}/"
            f"target={GOOS}-{GOARCH}/toolchain=test/upstream=test")
