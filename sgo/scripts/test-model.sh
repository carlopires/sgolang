#!/usr/bin/env bash
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
# test-model.sh — W01 step 5: run the executable model (sgo/model) with the pinned bootstrap Go.
# Writes model-<platform>.json to the w01 evidence directory (sgo/evidence/w01 only with SGO_RECORD=1).
source "$(dirname "$0")/lib.sh"
plat="$(host_platform)"
boot="$("$SGO_SCRIPTS/fetch-bootstrap.sh")"
out="$(mktemp "${TMPDIR:-/tmp}/sgo-model.XXXXXX")"
trap 'rm -f "$out"' EXIT
start="$(now_utc)"
set +e
(cd "$SGO_DIR/model" && run_clean GOROOT="$boot" GOCACHE="$SGO_CACHE/gocache/model" -- "$boot/bin/go" vet ./... &&
	run_clean GOROOT="$boot" GOCACHE="$SGO_CACHE/gocache/model" -- "$boot/bin/go" test -count=1 -v ./...) >"$out" 2>&1
status=$?
set -e
cat "$out"
mkdir -p "$SGO_EVIDENCE/../w01"
json_write "$SGO_EVIDENCE/../w01/model-$plat.json" \
	platform="$plat" started_at="$(echo "$start")" finished_at="$(now_utc)" exit_status="$status" \
	go="$(run_clean -- "$boot/bin/go" version)" \
	tests_passed="$(grep -c '^--- PASS' "$out" || true)" tests_failed="$(grep -c '^--- FAIL' "$out" || true)" \
	subtests_passed="$(grep -c '^    --- PASS' "$out" || true)" \
	mutations_detected="$(grep -c 'mutation .* detected by' "$out" || true)" \
	command="go vet ./... && go test -count=1 -v ./... (sgo/model)"
[ $status -eq 0 ] || die "model tests failed"
log "model tests passed ($plat)"
