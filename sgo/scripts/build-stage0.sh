#!/usr/bin/env bash
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
# build-stage0.sh — B04: build the host-built stage-0 toolchain (stage H1 in sgo/docs/stage-map.md)
# from the fork checkout using ONLY the pinned bootstrap Go. Runs `dist bootstrap -sgo-stage0`, which
# stops after toolchain1; no stage that compiles Go with the fork's compiler runs. make.bash is not used.
source "$(dirname "$0")/lib.sh"

plat="$(host_platform)"
fork="$(realdir "$SGO_ROOT")"
boot="$("$SGO_SCRIPTS/fetch-bootstrap.sh")"
tooldir="$fork/pkg/tool/${plat%-*}_${plat#*-}"
buildlog="$SGO_EVIDENCE/logs/stage0-build-$plat.log"
mkdir -p "$(dirname "$buildlog")"

start="$(now_utc)"
log "building stage-0 host toolchain in $fork (log: $buildlog)"
set +e
(
	cd "$fork/src"
	rm -f cmd/dist/dist
	# Same command make.bash uses to build cmd/dist with the bootstrap (bootstrapenv).
	run_clean GOROOT="$boot" GO111MODULE=off GOENV=off -- "$boot/bin/go" build -o cmd/dist/dist ./cmd/dist &&
		run_clean GOROOT="$fork" GOROOT_BOOTSTRAP="$boot" -- ./cmd/dist/dist bootstrap -a -sgo-stage0
	st=$?
	rm -f cmd/dist/dist
	exit $st
) >"$buildlog" 2>&1
status=$?
set -e
end="$(now_utc)"
[ $status -eq 0 ] || { tail -20 "$buildlog" >&2; die "stage-0 build failed with status $status"; }

# Checks: tools exist inside the fork, and no self-hosted stage output was produced.
for t in compile link asm; do [ -x "$tooldir/$t" ] || die "missing $tooldir/$t"; done
[ ! -e "$fork/bin/go" ] || die "$fork/bin/go exists: a self-hosted stage ran (or a stale build is present)"
grep -q "Building Go toolchain2" "$buildlog" && die "toolchain2 ran; -sgo-stage0 did not stop"
compilever="$(run_clean -- "$tooldir/compile" -V)"

json_write "$SGO_EVIDENCE/stage0-build-$plat.json" \
	test_case=B04 platform="$plat" status=passed started_at="$start" finished_at="$end" \
	fork_commit="$(git -C "$fork" rev-parse HEAD)" fork_dirty="$( [ -z "$(git -C "$fork" status --porcelain -- src)" ] && echo no || echo yes)" \
	bootstrap_goroot="$boot" bootstrap_version="$(run_clean -- "$boot/bin/go" version)" \
	tooldir="$tooldir" compile_version="$compilever" \
	compile_binary_sha256="$(sha256_file "$tooldir/compile")" link_binary_sha256="$(sha256_file "$tooldir/link")" \
	self_hosted_stages_run=no fork_bin_go_present=no \
	note="toolchain1 still identifies as upstream (compile -V). S-Go identity/ABI metadata is W03 work; this compiler is not an S-Go target entry point (sgo/scripts/sgo-target is)." \
	log="sgo/evidence/w00/logs/$(basename "$buildlog")"
log "stage-0 ok: $tooldir ($compilever)"
echo "$tooldir"
