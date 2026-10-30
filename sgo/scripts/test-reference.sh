#!/usr/bin/env bash
# test-reference.sh [--worktree DIR] [--run REGEX]
#
# B03: run the upstream distribution tests in the untouched reference worktree built by
# build-reference.sh. Raw JSON events go to $SGO_CACHE/logs (large); a per-package summary
# with every failure and skip goes to sgo/evidence/w00/. Failures are recorded, never hidden.
source "$(dirname "$0")/lib.sh"

plat="$(host_platform)"
tag="$(lock upstream.tag)"
wt="$SGO_CACHE/reference/$tag-b01"
run=""
while [ $# -gt 0 ]; do
	case "$1" in
		--worktree) wt="$2"; shift 2 ;;
		--run) run="$2"; shift 2 ;;
		*) die "unknown argument $1" ;;
	esac
done
wt="$(realdir "$wt")"
[ -x "$wt/bin/go" ] || die "no reference build at $wt; run build-reference.sh first"
[ -z "$(git -C "$wt" status --porcelain)" ] || die "reference worktree $wt has local changes"

rawdir="$SGO_CACHE/logs"
mkdir -p "$rawdir" "$SGO_EVIDENCE"
raw="$rawdir/reference-test-$plat-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
start="$(now_utc)"
args=(tool dist test -k -json)
[ -n "$run" ] && args+=(-run "$run")
log "running: go ${args[*]} (raw events: $raw)"
set +e
(cd "$wt/src" && run_clean PATH="$wt/bin:$SGO_SAFE_PATH" -- "$wt/bin/go" "${args[@]}") >"$raw" 2>&1
status=$?
set -e
end="$(now_utc)"

python3 - "$raw" "$SGO_EVIDENCE/reference-test-$plat.json" "$status" "$start" "$end" "$wt" "$tag" "$(lock upstream.commit)" "$plat" "${run:-<all>}" <<'EOF'
import json, sys, collections
raw, out, status, start, end, wt, tag, commit, plat, run = sys.argv[1:]
tests = collections.Counter()
pkgs = {}
fail_tests, skip_tests, nonjson = [], [], []
for line in open(raw, errors="replace"):
    line = line.strip()
    if not line:
        continue
    try:
        e = json.loads(line)
    except ValueError:
        nonjson.append(line)
        continue
    a, p, t = e.get("Action"), e.get("Package", ""), e.get("Test")
    if a not in ("pass", "fail", "skip"):
        continue
    if t:
        tests[a] += 1
        if a == "fail": fail_tests.append(f"{p}.{t}")
        if a == "skip": skip_tests.append(f"{p}.{t}")
    else:
        pkgs[p] = a
pk = collections.Counter(pkgs.values())
rec = {
    "test_case": "B03", "platform": plat, "upstream_tag": tag, "upstream_commit": commit,
    "worktree": wt, "command": "go tool dist test -k -json" + ("" if run == "<all>" else f" -run {run}"),
    "run_filter": run, "started_at": start, "finished_at": end, "exit_status": int(status),
    "result": "passed" if int(status) == 0 and not pk["fail"] else "failures_recorded",
    "packages": dict(pk), "tests": dict(tests),
    "failed_packages": sorted(p for p, a in pkgs.items() if a == "fail"),
    "failed_tests": sorted(fail_tests), "skipped_tests_count": len(skip_tests),
    "skipped_tests_sample": sorted(skip_tests)[:200],
    "non_json_output_tail": nonjson[-40:],
    "raw_events_file": raw, "note": "Skips are upstream environment/short-mode skips; failures are classified in sgo/evidence/w00/README.md.",
}
json.dump(rec, open(out, "w"), indent=2, sort_keys=True); open(out, "a").write("\n")
print(json.dumps({k: rec[k] for k in ("exit_status", "result", "packages", "tests", "failed_packages")}))
EOF
exit 0
