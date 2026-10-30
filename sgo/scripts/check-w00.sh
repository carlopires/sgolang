#!/usr/bin/env bash
# check-w00.sh — negative checks for W00 test cases B05, S01, S15 (gate level).
# Writes sgo/evidence/w00/checks-<platform>.json. Exit status 0 only if every check passed.
source "$(dirname "$0")/lib.sh"

plat="$(host_platform)"
gate="$SGO_SCRIPTS/sgo-target"
fx="$SGO_DIR/testdata/w00"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/sgo-check.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

# A hostile `go` on PATH: if anything executes it, the marker file appears.
mkdir -p "$tmp/fakebin"
marker="$tmp/FAKE_GO_WAS_EXECUTED"
printf '#!/bin/sh\ntouch "%s"\necho "fake go $*"\nexit 0\n' "$marker" > "$tmp/fakebin/go"
chmod +x "$tmp/fakebin/go"
hostile=(PATH="$tmp/fakebin:$PATH" GOROOT=/nonexistent GOTOOLCHAIN=go1.99.0+auto GOFLAGS=-toolexec=/bin/false
	GOPROXY=https://proxy.golang.org GOEXPERIMENT=sgocompat SGO_COMPAT=1 SGO_LEGACY=1 GODEBUG=sgolegacy=1)

results=()
failures=0
record() { # record ID NAME pass|fail DETAIL
	results+=("$1|$2|$3|$4")
	[ "$3" = pass ] || failures=$((failures + 1))
	printf '%-5s %-4s %s\n' "$1" "$3" "$2" >&2
}

# --- B05: toolchain selection -------------------------------------------------------------
want="$(bootstrap_root)"
got="$(env "${hostile[@]}" "$SGO_SCRIPTS/fetch-bootstrap.sh" 2>/dev/null)" || got="<failed>"
[ "$got" = "$want" ] && [ ! -e "$marker" ] &&
	record B05 "fetch-bootstrap selects recorded bootstrap under hostile PATH/env" pass "$got" ||
	record B05 "fetch-bootstrap selects recorded bootstrap under hostile PATH/env" fail "got=$got marker=$([ -e "$marker" ] && echo yes || echo no)"

cat >"$tmp/probe.sh" <<'PROBE'
g="$(command -v go)"
if [ "$g" = "$SGO_GUARD" ] && ! go version >/dev/null 2>&1; then echo GO_LOOKUP_GUARDED; else echo "GO_LOOKUP_RESOLVES_TO=$g"; fi
env | grep '^GO' | sort
PROBE
scrub="$(env "${hostile[@]}" bash -c 'source "$1/lib.sh"; run_clean SGO_GUARD="$SGO_GUARD_BIN/go" -- /bin/sh "$2"' _ "$SGO_SCRIPTS" "$tmp/probe.sh" 2>&1 | tr '\n' ' ')"
case "$scrub" in
	GO_LOOKUP_GUARDED*"GOFLAGS= GOPROXY=off GOTELEMETRY=off GOTOOLCHAIN=local "*)
		if echo "$scrub" | grep -q 'GOROOT=\|GOEXPERIMENT=\|GODEBUG='; then
			record B05 "run_clean scrubs inherited Go environment and PATH" fail "$scrub"
		else
			record B05 "run_clean scrubs inherited Go environment and PATH" pass "$scrub"
		fi ;;
	*) record B05 "run_clean scrubs inherited Go environment and PATH" fail "$scrub" ;;
esac

for f in "$SGO_EVIDENCE"/reference-build-"$plat"-*.json "$SGO_EVIDENCE/stage0-build-$plat.json"; do
	[ -f "$f" ] || continue
	ok="$(python3 - "$f" "$(bootstrap_root)" <<'EOF'
import json, sys
d = json.load(open(sys.argv[1]))
root = d.get("goroot") or d["tooldir"].rsplit("/pkg/tool/", 1)[0]
paths = [d.get("gotooldir", d.get("tooldir"))]
bad = [p for p in paths if not p.startswith(root + "/")]
print("pass" if d.get("status") == "passed" and not bad and d["bootstrap_goroot"] == sys.argv[2] else "fail:" + ",".join(bad))
EOF
)"
	detail=""; [[ "$ok" == *:* ]] && detail="${ok#*:}"
	record B05 "tool identities inside their build tree: $(basename "$f")" "${ok%%:*}" "$detail"
done

# --- B02: independent worktree build used the same pinned inputs; report binary equality honestly --
b1="$SGO_EVIDENCE/reference-build-$plat-b01.json"; b2="$SGO_EVIDENCE/reference-build-$plat-b02.json"
if [ -f "$b1" ] && [ -f "$b2" ]; then
	cmp="$(python3 - "$b1" "$b2" <<'EOF'
import json, sys
a, b = (json.load(open(p)) for p in sys.argv[1:])
same_inputs = all(a[k] == b[k] for k in ("upstream_commit", "bootstrap_archive_sha256", "go_version", "compile_version"))
same_dirs = a["worktree"] != b["worktree"]
eq = {k: a[k] == b[k] for k in ("go_binary_sha256", "compile_binary_sha256", "link_binary_sha256")}
status = "pass" if same_inputs and same_dirs and a["status"] == b["status"] == "passed" else "fail"
print(status + "|binary_sha256_equal=" + ",".join(f"{k.split('_')[0]}:{'yes' if v else 'no'}" for k, v in eq.items()))
EOF
)"
	record B02 "independent worktree build used identical pinned inputs" "${cmp%%|*}" "${cmp#*|}"
fi

# --- S01 / S15: the target entry point denies everything, with no override ----------------
deny() { # deny ID NAME ARGS...
	local id="$1" name="$2"; shift 2
	local out st
	set +e
	out="$(env "${hostile[@]}" "$gate" "$@" 2>&1)"
	st=$?
	set -e
	if [ $st -eq 2 ] && echo "$out" | grep -q 'SGO_E_UNADMITTED' && [ ! -e "$marker" ]; then
		record "$id" "$name" pass "exit=2 SGO_E_UNADMITTED"
	else
		record "$id" "$name" fail "exit=$st marker=$([ -e "$marker" ] && echo yes || echo no) out=$(echo "$out" | head -2 | tr '\n' ' ')"
	fi
}
deny S01 "valid subset-0 program is denied until admitted" build "$fx/subset0-hello/main.sgo"
deny S01 "legacy Go program with bare go is denied" build "$fx/legacy-go/main.go"
deny S01 "build tag / pragma / linkname escape is denied" build -tags sgo_compat "$fx/escape-pragma/main.go"
deny S01 "legacy flag -compat is denied" -compat build "$fx/legacy-go/main.go"
deny S01 "legacy flag --legacy is denied" --legacy run "$fx/legacy-go/main.go"
deny S01 "-lang=go1.27 / -gcflags are denied" build -lang=go1.27 -gcflags=all=-l "$fx/legacy-go/main.go"
deny S01 "no-argument invocation is denied" 
deny S15 "rebuilding host compiler sources as target is denied" build "$SGO_ROOT/src/cmd/compile"
deny S15 "linking a stock object is denied" link -o "$tmp/out" "$tmp/stock.a"
deny S15 "direct tool/backend access through the target entry is denied" tool compile "$fx/legacy-go/main.go"

# Known W00 limitation, recorded rather than hidden: the host-built stage-0 compiler is still the
# unmodified upstream compiler and accepts Go when invoked directly. Compiler/linker-level
# rejection is W03 (plan §1.3, S01/S15). It is not an S-Go target entry point.
results+=("S15|stage-0 compiler invoked directly still accepts Go (W03 work)|not_enforced_until_W03|host tool, not a target entry")

python3 - "$SGO_EVIDENCE/checks-$plat.json" "$plat" "$(now_utc)" "$failures" "${results[@]}" <<'EOF'
import json, sys
out, plat, when, failures, *rows = sys.argv[1:]
checks = [dict(zip(("case", "check", "result", "detail"), r.split("|", 3))) for r in rows]
rec = {"platform": plat, "recorded_at": when, "failures": int(failures), "checks": checks,
       "scope": "Gate-level and script-level checks for B05, S01, S15. Compiler/linker-level enforcement is W03."}
json.dump(rec, open(out, "w"), indent=2, sort_keys=True); open(out, "a").write("\n")
EOF
[ "$failures" -eq 0 ] || die "$failures check(s) failed"
log "all W00 gate checks passed ($plat)"
