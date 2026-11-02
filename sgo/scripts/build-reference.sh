#!/usr/bin/env bash
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
# build-reference.sh [--worktree DIR] [--label NAME]
#
# Build the UNTOUCHED upstream commit from UPSTREAM.lock.json in an isolated, detached git
# worktree, using only the pinned bootstrap. This is the upstream reference/comparison build
# (plan §10.2): it is not an S-Go build and never becomes an S-Go target path.
# Test cases: B01 (pinned clean bootstrap), B02 (independent worktree, with --worktree/--label).
source "$(dirname "$0")/lib.sh"

label=b01
wt=""
while [ $# -gt 0 ]; do
	case "$1" in
		--worktree) wt="$2"; shift 2 ;;
		--label) label="$2"; shift 2 ;;
		*) die "unknown argument $1" ;;
	esac
done

plat="$(host_platform)"
tag="$(lock upstream.tag)"
commit="$(lock upstream.commit)"
[ -n "$wt" ] || wt="$SGO_CACHE/reference/$tag-$label"

boot="$("$SGO_SCRIPTS/fetch-bootstrap.sh")"

git -C "$SGO_ROOT" cat-file -e "$commit^{commit}" 2>/dev/null ||
	die "upstream commit $commit not present; run: git fetch upstream refs/tags/$tag:refs/tags/$tag"

if [ ! -d "$wt" ]; then
	log "creating detached worktree $wt at $commit"
	mkdir -p "$(dirname "$wt")"
	git -C "$SGO_ROOT" worktree add --quiet --detach "$wt" "$commit" >&2
fi
wt="$(realdir "$wt")"
head="$(git -C "$wt" rev-parse HEAD)"
[ "$head" = "$commit" ] || die "worktree $wt is at $head, not the locked commit $commit"
dirty="$(git -C "$wt" status --porcelain)"
[ -z "$dirty" ] || die "reference worktree $wt has local changes; it must stay untouched:
$dirty"

logdir="$SGO_EVIDENCE/logs"
mkdir -p "$logdir"
buildlog="$logdir/reference-build-$plat-$label.log"
start="$(now_utc)"
log "building $tag in $wt (log: $buildlog)"
set +e
(cd "$wt/src" && run_clean GOROOT_BOOTSTRAP="$boot" -- bash ./make.bash) >"$buildlog" 2>&1
status=$?
set -e
end="$(now_utc)"
[ $status -eq 0 ] || { tail -20 "$buildlog" >&2; die "make.bash failed with status $status"; }

# Tool identity checks: everything must resolve inside the reference worktree (B01 gate).
gobin="$wt/bin/go"
goversion="$(run_clean -- "$gobin" version)"
goroot="$(run_clean -- "$gobin" env GOROOT)"
tooldir="$(run_clean -- "$gobin" env GOTOOLDIR)"
compile="$tooldir/compile"
[ "$goversion" = "go version $tag ${plat%-*}/${plat#*-}" ] || die "unexpected version: $goversion"
[ "$(realdir "$goroot")" = "$wt" ] || die "GOROOT $goroot is not the worktree $wt"
case "$(realdir "$tooldir")" in "$wt"/*) ;; *) die "GOTOOLDIR $tooldir is outside $wt" ;; esac
[ -x "$compile" ] || die "compiler $compile missing"
compilever="$(run_clean -- "$compile" -V)"

json_write "$SGO_EVIDENCE/reference-build-$plat-$label.json" \
	test_case="$( [ "$label" = b01 ] && echo B01 || echo B02 )" \
	label="$label" platform="$plat" status="passed" \
	started_at="$start" finished_at="$end" \
	upstream_tag="$tag" upstream_commit="$commit" worktree="$wt" worktree_head="$head" \
	bootstrap_goroot="$boot" bootstrap_version="$(run_clean -- "$boot/bin/go" version)" \
	bootstrap_archive_sha256="$(lock "bootstrap.archives.$plat.sha256")" \
	go_version="$goversion" goroot="$goroot" gotooldir="$tooldir" compile_version="$compilever" \
	go_binary_sha256="$(sha256_file "$gobin")" compile_binary_sha256="$(sha256_file "$compile")" \
	link_binary_sha256="$(sha256_file "$tooldir/link")" \
	log="sgo/evidence/w00/logs/$(basename "$buildlog")"
log "reference build ok: $goversion at $wt"
echo "$wt"
