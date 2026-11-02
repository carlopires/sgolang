#!/bin/sh
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
#
# verify-release.sh DIR — D78: install and exercise a release from its artifacts alone, following
# INSTALL.md. DIR holds SHA256SUMS and the release archives (as the sgo-release workflow uploads
# them). Needs only sh, tar, sha256sum or shasum, and diff: no checkout, no go command. The
# sgo-release workflow runs it on fresh runners; it also works locally on a downloaded release.
set -eu
dir=$(cd "$1" && pwd)
os=$(uname -s | tr A-Z a-z)
arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
fail() { echo "verify-release: $*" >&2; exit 1; }

cd "$dir"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum -c --ignore-missing SHA256SUMS || fail "checksums do not match"
else
	shasum -a 256 -c --ignore-missing SHA256SUMS || fail "checksums do not match"
fi
archive=$(ls sgo-*-"$os"-"$arch".tar.gz) || fail "no archive for $os/$arch"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tar xzf "$archive" -C "$work"
top=$(ls -d "$work"/sgo-*)
[ -f "$top/INSTALL.md" ] || fail "no INSTALL.md"
if find "$top" -name go -o -name gofmt | grep -q .; then fail "a go command or gofmt is in the archive"; fi

# INSTALL.md: put bin on PATH; nothing else from the environment.
export PATH="$top/bin:/usr/bin:/bin" HOME="$work/home" SGO_CACHE="$work/home/cache"
unset GOROOT GOPATH GOFLAGS GOTOOLCHAIN GODEBUG 2>/dev/null || true
mkdir -p "$HOME"
sgo version | grep -q "spec" || fail "sgo version"

cd "$top/examples/pipeline"
sgo run . | diff - expected.txt || fail "pipeline: sgo run output"
sgo test ./... >/dev/null || fail "pipeline: sgo test"
sgo lab sweep -seeds 0..15 -expect expected.txt . >/dev/null 2>&1 || fail "pipeline: lab sweep"

cd "$top/examples/service"
sgo run . | diff - expected.txt || fail "service: sgo run output"
sgo test ./... >/dev/null || fail "service: sgo test"

cd "$top/examples/replay"
[ "$(sgo lab replay buggy.trace.jsonl ./buggy 2>/dev/null)" = "debit commit credit" ] || fail "replay: the shipped trace does not reproduce the bug"
if sgo lab replay buggy.trace.jsonl ./fixed >/dev/null 2>"$work/div"; then fail "replay: the fix did not diverge"; fi
grep -q SGO_E_LAB_DIVERGENCE "$work/div" || fail "replay: want SGO_E_LAB_DIVERGENCE"
sgo lab sweep -seeds 0..63 -expect expected.txt ./fixed >/dev/null 2>&1 || fail "replay: the fix fails a schedule"

# A program that is not S-Go is refused, with no fallback.
mkdir -p "$work/legacy" && cd "$work/legacy"
printf 'module example.com/legacy\n\nsgo %s\n' "$(cat "$top/sgo/spec/VERSION")" > sgo.mod
printf 'package main\n\nfunc main() { go func() {}() }\n' > main.go
if sgo build -o x . >/dev/null 2>&1; then fail "a Go program built"; fi
[ ! -e x ] || fail "a refused build wrote output"

echo "verify-release: ok: $archive installed from its artifacts, examples built, run, tested, swept; shipped trace replayed; Go input refused"
