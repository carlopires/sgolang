#!/usr/bin/env bash
# fetch-bootstrap.sh — install the pinned host bootstrap Go (UPSTREAM.lock.json "bootstrap")
# into $SGO_CACHE/bootstrap/, verifying the published SHA-256 before extraction.
# Prints the resulting GOROOT_BOOTSTRAP on stdout. Never uses a Go found on PATH.
source "$(dirname "$0")/lib.sh"

plat="$(host_platform)"
ver="$(lock bootstrap.pinned_version)"
url="$(lock "bootstrap.archives.$plat.url")"
want="$(lock "bootstrap.archives.$plat.sha256")"
root="$(bootstrap_root)"
expect="go version $ver ${plat%-*}/${plat#*-}"

check() {
	local got
	got="$(run_clean -- "$root/bin/go" version)" || return 1
	[ "$got" = "$expect" ] || die "bootstrap at $root reports '$got', want '$expect'"
}

if [ -x "$root/bin/go" ] && [ -f "$root/../.sgo-sha256" ] && [ "$(cat "$root/../.sgo-sha256")" = "$want" ]; then
	check
	echo "$root"
	exit 0
fi

tmp="$(mktemp -d "${TMPDIR:-/tmp}/sgo-bootstrap.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
log "downloading $url"
curl -fsSL --proto '=https' -o "$tmp/go.tar.gz" "$url"
got="$(sha256_file "$tmp/go.tar.gz")"
[ "$got" = "$want" ] || die "checksum mismatch for $url: got $got, want $want"
log "checksum ok ($want)"
mkdir -p "$tmp/x"
tar -xzf "$tmp/go.tar.gz" -C "$tmp/x"
rm -rf "$(dirname "$root")"
mkdir -p "$(dirname "$(dirname "$root")")"
mv "$tmp/x" "$(dirname "$root")"
echo "$want" > "$root/../.sgo-sha256"
check
echo "$root"
