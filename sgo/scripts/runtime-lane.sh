#!/usr/bin/env bash
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
# runtime-lane.sh [go test args...]
#
# Runtime test lane (stage map "RL"): an isolated worktree at the locked upstream commit, built once
# with make.bash from upstream tool sources, with the fork's trusted runtime packages (the package
# closure in sgo/TARGET_TCB.json) overlaid before each run. `go test` then recompiles the overlaid
# runtime from source. The lane compiler is upstream's, so this lane is a host-only test harness for
# runtime (TCB) code written in Go; it is never an S-Go target path (plan §10.2, SPEC-5).
#
# Examples:  runtime-lane.sh -run 'TestSGo' runtime
#            runtime-lane.sh -race -run 'TestSGo' runtime
source "$(dirname "$0")/lib.sh"

plat="$(host_platform)"
commit="$(lock upstream.commit)"
lane="$SGO_CACHE/runtime-lane/$(lock upstream.tag)"
boot="$("$SGO_SCRIPTS/fetch-bootstrap.sh")"

mkdir -p "$SGO_CACHE/logs"
if [ ! -x "$lane/bin/go" ]; then
	if [ ! -d "$lane" ]; then
		mkdir -p "$(dirname "$lane")"
		git -C "$SGO_ROOT" worktree add --quiet --detach "$lane" "$commit" >&2
	fi
	log "building runtime lane toolchain (upstream tool sources) in $lane"
	(cd "$lane/src" && run_clean GOROOT_BOOTSTRAP="$boot" -- bash ./make.bash) >"$SGO_CACHE/logs/runtime-lane-build.log" 2>&1 ||
		{ tail -20 "$SGO_CACHE/logs/runtime-lane-build.log" >&2; die "runtime lane build failed"; }
fi
[ "$(git -C "$lane" rev-parse HEAD)" = "$commit" ] || die "lane worktree is not at $commit"

# Overlay the fork's TCB packages (only those directories; tool sources stay upstream).
pkgs="$(python3 -c "import json; print(' '.join(json.load(open('$SGO_DIR/TARGET_TCB.json'))['packages']))")"
for p in $pkgs; do
	rsync -a --delete --exclude testdata "$SGO_ROOT/src/$p/" "$lane/src/$p/"
	[ -d "$SGO_ROOT/src/$p/testdata" ] && rsync -a --delete "$SGO_ROOT/src/$p/testdata/" "$lane/src/$p/testdata/"
done
overlay="$(cd "$SGO_ROOT" && python3 -c '
import hashlib, os, sys
h = hashlib.sha256()
for p in sys.argv[1:]:
    d = os.path.join("src", p)
    for n in sorted(os.listdir(d)):
        f = os.path.join(d, n)
        if os.path.isfile(f):
            h.update(f.encode() + b"\0" + hashlib.sha256(open(f, "rb").read()).digest())
print(h.hexdigest())
' $pkgs)"
log "runtime lane: upstream tools at ${commit:0:12}, fork TCB overlay digest ${overlay:0:16}"

cd "$lane/src"
run_clean PATH="$lane/bin:$SGO_SAFE_PATH" GOCACHE="$SGO_CACHE/gocache/runtime-lane" -- "$lane/bin/go" test "$@"
