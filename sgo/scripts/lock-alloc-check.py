#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""lock-alloc-check: no allocation while an S-Go runtime lock is held (runtime-design §3, W12 R9).

A line-based check over src/runtime/sgo_*.go (tests excluded). Within a function it tracks the
locks taken (lock(&...), sgoSelLock) and released (unlock(&...), sgoSelUnlock) by indentation: an
unlock inside a deeper block (an early-return branch, or an unlock-allocate-relock retry) releases
the lock only until that block closes or the lock is retaken at that depth. It reports make(, new(
and &sgoT{ composite literals on lines where a lock is held. A deliberate exception carries the
comment `lock-alloc: ok (<reason>)` on the same line.
"""
import glob, re, sys

LOCK = re.compile(r'(?<![A-Za-z_])(lock\(&|sgoSelLock\()')
UNLOCK = re.compile(r'(?<![A-Za-z_])(unlock\(&|sgoSelUnlock\()')
ALLOC = re.compile(r'\bmake\(|\bnew\(|&sgo[A-Za-z0-9_]+\{')


def indent(line):
    return len(line) - len(line.lstrip('\t'))


def check(path):
    out = []
    stack = []  # (kind, depth): 'held' or 'rel'
    for no, line in enumerate(open(path, encoding='utf-8'), 1):
        code = line.split('//')[0]
        if line.startswith('func '):
            stack = []
            continue
        if not code.strip():
            continue
        d = indent(line)
        while stack and stack[-1][1] > d:
            stack.pop()
        if code.strip() == '}':
            continue
        held = bool(stack) and stack[-1][0] == 'held'
        if held and ALLOC.search(code) and 'lock-alloc: ok' not in line:
            out.append(f'{path}:{no}: allocation under a runtime lock: {line.strip()}')
        for m in re.finditer(r'(?<![A-Za-z_])(unlock\(&|sgoSelUnlock\(|lock\(&|sgoSelLock\()', code):
            if m.group(1) in ('unlock(&', 'sgoSelUnlock('):
                if stack and stack[-1] == ('held', d):
                    stack.pop()
                elif stack and stack[-1][0] == 'held':
                    stack.append(('rel', d))
            else:
                if stack and stack[-1] == ('rel', d):
                    stack.pop()
                else:
                    stack.append(('held', d))
    return out


def main():
    files = sys.argv[1:] or sorted(p for p in glob.glob('src/runtime/sgo_*.go') if not p.endswith('_test.go'))
    bad = [m for p in files for m in check(p)]
    for m in bad:
        print(m)
    print(f'lock-alloc-check: {"ok" if not bad else "FAIL"} ({len(files)} files, {len(bad)} problem(s))')
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main())
