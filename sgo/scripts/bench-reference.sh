#!/usr/bin/env bash
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
# bench-reference.sh [--count N] — W00 benchmark baseline in the untouched reference worktree.
# Covers the runtime areas S-Go changes: task creation, channels, select, stacks, scheduling.
# Output: sgo/evidence/w00/bench-reference-<platform>.txt (benchstat-compatible) + .json metadata.
source "$(dirname "$0")/lib.sh"

count=5
while [ $# -gt 0 ]; do
	case "$1" in
		--count) count="$2"; shift 2 ;;
		*) die "unknown argument $1" ;;
	esac
done
plat="$(host_platform)"
tag="$(lock upstream.tag)"
wt="$(realdir "$SGO_CACHE/reference/$tag-b01")"
[ -x "$wt/bin/go" ] || die "no reference build at $wt"
[ -z "$(git -C "$wt" status --porcelain)" ] || die "reference worktree $wt has local changes"

bench='^Benchmark(CreateGoroutines|CreateGoroutinesParallel|ChanSync|ChanProdCons(0|10|100)|ChanUncontended|ChanContended|SelectUncontended|SelectSyncContended|SelectProdCons|StackGrowth|StackGrowthDeep|PingPongHog|WakeupParallelSpinning|Defer|DeferMany|PanicRecover)$'
out="$SGO_EVIDENCE/bench-reference-$plat.txt"
start="$(now_utc)"
log "benchmarking runtime in $wt (count=$count)"
(cd "$wt/src" && run_clean PATH="$wt/bin:$SGO_SAFE_PATH" -- "$wt/bin/go" test -run '^$' -bench "$bench" -count "$count" -benchmem runtime) >"$out" 2>&1 ||
	{ tail -20 "$out" >&2; die "benchmark run failed"; }
end="$(now_utc)"
cpu="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- || echo unknown)"
json_write "$SGO_EVIDENCE/bench-reference-$plat.json" \
	platform="$plat" upstream_commit="$(lock upstream.commit)" worktree="$wt" started_at="$start" finished_at="$end" \
	command="go test -run ^\$ -bench <set> -count $count -benchmem runtime" bench_regex="$bench" count="$count" \
	cpu="$(echo "$cpu" | sed 's/^ *//')" ncpu="$(getconf _NPROCESSORS_ONLN)" \
	results="sgo/evidence/w00/$(basename "$out")" \
	note="Host-load-sensitive; compare only with benchstat on the same machine and configuration."
log "wrote $out"
