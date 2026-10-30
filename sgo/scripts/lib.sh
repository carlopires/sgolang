# sgo/scripts/lib.sh — shared helpers for S-Go W00 build scripts. Source, do not execute.
#
# Rules (plan §10.2, test cases B05/S01/S15):
#   * Every tool is selected by absolute path from UPSTREAM.lock.json; PATH lookup of `go` never happens.
#   * Builds run in a scrubbed environment (no inherited GO*, CC, CGO_*, etc.).
#   * Host bootstrap, untouched reference, and fork artifacts use separate directories and caches.
#   * Automatic toolchain switching/downloads are disabled (GOTOOLCHAIN=local, GOPROXY=off).

set -euo pipefail

SGO_SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SGO_DIR="$(cd "$SGO_SCRIPTS/.." && pwd)"
SGO_ROOT="$(cd "$SGO_DIR/.." && pwd)"
SGO_LOCK="$SGO_DIR/UPSTREAM.lock.json"
SGO_CACHE="${SGO_CACHE:-$HOME/.cache/sgolang}"
SGO_EVIDENCE="$SGO_DIR/evidence/w00"
# Minimal, explicit PATH for child processes. System directories may contain a Go installation
# (e.g. /usr/bin/go on GitHub's Ubuntu images), so a guard directory comes first: its `go` and
# `gofmt` fail loudly, turning any PATH lookup of Go into an error instead of a silent fallback.
SGO_GUARD_BIN="$SGO_CACHE/guard/bin"
SGO_SAFE_PATH="$SGO_GUARD_BIN:/usr/bin:/bin:/usr/sbin:/sbin"
if [ ! -x "$SGO_GUARD_BIN/go" ] || [ ! -x "$SGO_GUARD_BIN/gofmt" ]; then
	mkdir -p "$SGO_GUARD_BIN"
	for t in go gofmt; do
		printf '#!/bin/sh\necho "sgo: PATH lookup of %s is forbidden; tools are selected by absolute path (sgo/scripts/lib.sh)" >&2\nexit 97\n' "$t" >"$SGO_GUARD_BIN/$t"
		chmod +x "$SGO_GUARD_BIN/$t"
	done
fi

die() { echo "sgo: error: $*" >&2; exit 1; }
log() { echo "sgo: $*" >&2; }

# Host tools the scripts themselves need (recorded in evidence).
for t in python3 git curl tar; do
	command -v "$t" >/dev/null 2>&1 || die "required host tool '$t' not found"
done

# lock KEY.PATH — read a value from UPSTREAM.lock.json.
lock() {
	python3 - "$SGO_LOCK" "$1" <<'EOF'
import json, sys
v = json.load(open(sys.argv[1]))
for k in sys.argv[2].split("."):
    v = v[k]
print(v if not isinstance(v, (dict, list)) else json.dumps(v))
EOF
}

# host_platform — darwin-arm64 | linux-amd64 | linux-arm64 (D22). Anything else is unsupported.
host_platform() {
	local os arch
	case "$(uname -s)" in
		Darwin) os=darwin ;;
		Linux) os=linux ;;
		*) die "unsupported host OS $(uname -s)" ;;
	esac
	case "$(uname -m)" in
		arm64 | aarch64) arch=arm64 ;;
		x86_64 | amd64) arch=amd64 ;;
		*) die "unsupported host arch $(uname -m)" ;;
	esac
	case "$os-$arch" in
		darwin-arm64 | linux-amd64 | linux-arm64) echo "$os-$arch" ;;
		*) die "host $os-$arch is not a D22 target" ;;
	esac
}

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

realdir() { (cd "$1" && pwd -P); }

bootstrap_root() {
	echo "$SGO_CACHE/bootstrap/$(lock bootstrap.pinned_version)-$(host_platform)/go"
}

# run_clean VAR=VAL... -- CMD ARGS... : run CMD with a scrubbed environment.
run_clean() {
	local vars=()
	while [ $# -gt 0 ] && [ "$1" != "--" ]; do vars+=("$1"); shift; done
	[ "${1:-}" = "--" ] && shift
	env -i \
		HOME="$HOME" \
		PATH="$SGO_SAFE_PATH" \
		TMPDIR="${TMPDIR:-/tmp}" \
		LANG=C LC_ALL=C \
		GOTOOLCHAIN=local \
		GOPROXY=off \
		GOTELEMETRY=off \
		GOFLAGS= \
		${vars[@]+"${vars[@]}"} \
		"$@"
}

# json_write FILE key=value... — write an evidence record (string values, keys sorted).
json_write() {
	local file="$1"; shift
	mkdir -p "$(dirname "$file")"
	python3 - "$file" "$@" <<'EOF'
import json, sys
d = dict(a.split("=", 1) for a in sys.argv[2:])
with open(sys.argv[1], "w") as f:
    json.dump(d, f, indent=2, sort_keys=True)
    f.write("\n")
EOF
}

now_utc() { date -u +%Y-%m-%dT%H:%M:%SZ; }
