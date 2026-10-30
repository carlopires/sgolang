#!/usr/bin/env bash
# test-host-pkg.sh [--src ROOT] PKGDIR [go test flags...]
#
# Host test lane (stage H1 in sgo/docs/stage-map.md): run the unit tests of a fork *host* package
# (compiler/toolchain code, written in Go) with the pinned bootstrap Go. The package is copied into a
# temporary module, so the fork's own compiler never compiles Go source.
#
# Limits (reported, never hidden): only packages whose non-test files import nothing but the standard
# library are supported; test files importing upstream-internal packages (internal/..., cmd/...) are
# excluded and listed. PKGDIR is relative to <ROOT>/src (default ROOT: the fork checkout).
source "$(dirname "$0")/lib.sh"

root="$SGO_ROOT"
if [ "${1:-}" = "--src" ]; then root="$2"; shift 2; fi
[ $# -ge 1 ] || die "usage: test-host-pkg.sh [--src ROOT] PKGDIR [go test flags...]"
pkg="$1"; shift
dir="$root/src/$pkg"
[ -d "$dir" ] || die "no package directory $dir"
boot="$("$SGO_SCRIPTS/fetch-bootstrap.sh")"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/sgo-hosttest.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
name="$(basename "$pkg")"
mkdir -p "$tmp/$name"
printf 'module sgohosttest\n\ngo 1.26\n' >"$tmp/go.mod"

# Import lister using go/parser (ImportsOnly), built with the bootstrap.
mkdir -p "$tmp/lister"
cat >"$tmp/lister/main.go" <<'GO'
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

func main() {
	f, err := parser.ParseFile(token.NewFileSet(), os.Args[1], nil, parser.ImportsOnly)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, s := range f.Imports {
		p, _ := strconv.Unquote(s.Path.Value)
		fmt.Println(p)
	}
}
GO
(cd "$tmp/lister" && printf 'module lister\n\ngo 1.26\n' >go.mod &&
	run_clean GOROOT="$boot" GOCACHE="$SGO_CACHE/gocache/host-test" -- "$boot/bin/go" build -o "$tmp/listimports" .) ||
	die "cannot build import lister"

# Shim for internal/testenv (only what host-lane tests use). MustHaveGoBuild skips: this lane has
# no fork go command by design, so tests that need one are reported as skipped, not passed.
mkdir -p "$tmp/testenv"
cat >"$tmp/testenv/testenv.go" <<'GO'
// Package testenv is the S-Go host-test-lane shim for internal/testenv.
package testenv

import (
	"os"
	"testing"
)

// GOROOT returns the source tree under test (set by test-host-pkg.sh).
func GOROOT(t testing.TB) string { return os.Getenv("SGO_HOST_TEST_GOROOT") }

// MustHaveGoBuild skips: the host lane has no fork go command (sgo/docs/stage-map.md).
func MustHaveGoBuild(t testing.TB) {
	t.Helper()
	t.Skip("S-Go host test lane: no go command for the tree under test")
}

// OptimizationOff reports whether optimization is disabled; the lane builds with defaults.
func OptimizationOff() bool { return false }
GO

excluded=()
shimmed=()
for f in "$dir"/*.go; do
	imports="$("$tmp/listimports" "$f")" || die "cannot parse $f"
	other="$(echo "$imports" | grep -E '^(internal|cmd)/|\.' | grep -vx 'internal/testenv' || true)"
	if [ -n "$other" ]; then
		case "$f" in
			*_test.go) excluded+=("$(basename "$f")"); continue ;;
			*) die "$(basename "$f") imports non-std packages ($(echo "$other" | tr '\n' ' ')); not supported by this lane" ;;
		esac
	fi
	if echo "$imports" | grep -qx 'internal/testenv'; then
		sed 's#"internal/testenv"#"sgohosttest/testenv"#' "$f" >"$tmp/$name/$(basename "$f")"
		shimmed+=("$(basename "$f")")
	else
		cp "$f" "$tmp/$name/"
	fi
done
[ -d "$dir/testdata" ] && cp -R "$dir/testdata" "$tmp/$name/testdata"

log "testing $pkg with $(run_clean -- "$boot/bin/go" version) (testenv shim: ${shimmed[*]:-none}; excluded: ${excluded[*]:-none})"
cd "$tmp/$name"
run_clean GOROOT="$boot" GOCACHE="$SGO_CACHE/gocache/host-test" SGO_HOST_TEST_GOROOT="$root" -- "$boot/bin/go" test "$@" .
