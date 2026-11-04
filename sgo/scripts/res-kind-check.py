#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""res-kind-check: the runtime's promotion of task results knows every resource kind (W23 review L1).

runtime_resPromote (src/runtime/sgo_res.go) moves the resources a value-returning child's result
holds into its nursery (SGO17-02). It recognizes a built-in handle by its type, from the list
builtinHandles in src/sgo/task.go, and a resource registered with runtime_resOwn by its key. The
compiler's resource kinds are those sgoResKind (types2/sgo_resflow.go) names. This check requires
that every compiler resource kind is in builtinHandles or registered through runtime_resOwn in
package sgo, and that builtinHandles names only compiler resource kinds: a kind added to the
compiler but not to the list would silently stay in the child.

  res-kind-check.py              check the sources
  res-kind-check.py --selftest   show that a list missing a kind, or naming a non-resource, fails
"""
import sgoargs
import os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
RESFLOW = "src/cmd/compile/internal/types2/sgo_resflow.go"
TASK = "src/sgo/task.go"
SGODIR = "src/sgo"


def read(rel):
    with open(os.path.join(ROOT, rel)) as f:
        return f.read()


def compiler_kinds(src):
    m = re.search(r'func sgoResKind\(T Type\) bool \{.*?case ([^:]+):', src, re.S)
    if not m:
        raise SystemExit("res-kind-check: cannot find the cases of sgoResKind in " + RESFLOW)
    return set(re.findall(r'"([A-Za-z]+)"', m.group(1)))


def builtin_list(src):
    m = re.search(r'var builtinHandles = \[\d+\]any\{([^}]*)\}', src)
    if not m:
        raise SystemExit("res-kind-check: cannot find builtinHandles in " + TASK)
    return set(re.findall(r'\(\*([A-Za-z]+)\)\(nil\)', m.group(1)))


def registered(srcs):
    """Types whose values package sgo registers with runtime_resOwn under their own pointer: the
    function returning *T that registers the value it returns, runtime_resOwn(..., unsafe.Pointer(v))."""
    out = set()
    for src in srcs:
        for fn in re.finditer(r'\nfunc \([^)]*\) \w+\([^)]*\) \*(\w+)(?:\[[^\]]*\])? \{(.*?)\n\}', src, re.S):
            for v in re.findall(r'runtime_resOwn\([^,]+, [^,]+, unsafe\.Pointer\((\w+)\)', fn.group(2)):
                if re.search(r'\n\treturn ' + v + r'\n', fn.group(2) + '\n'):  # the value returned is the one registered
                    out.add(fn.group(1))
    return out


def problems(kinds, builtin, user):
    errs = []
    for k in sorted(kinds - builtin - user):
        errs.append(f"resource kind {k} (sgoResKind) is neither in builtinHandles ({TASK}) nor "
                    "registered with runtime_resOwn: a result holding it would not move into the nursery")
    for k in sorted(builtin - kinds):
        errs.append(f"builtinHandles names {k}, which sgoResKind does not classify as a resource")
    for k in sorted(builtin & user):
        errs.append(f"{k} is both in builtinHandles and registered with runtime_resOwn")
    return errs


def sources():
    srcs = [read(os.path.join(SGODIR, f)) for f in sorted(os.listdir(os.path.join(ROOT, SGODIR)))
            if f.endswith(".go") and not f.endswith("_test.go")]
    return compiler_kinds(read(RESFLOW)), builtin_list(read(TASK)), registered(srcs)


def selftest():
    kinds, builtin, user = sources()
    cases = {"Timer removed": (kinds, builtin - {"Timer"}, user),
             "a new kind": (kinds | {"Pipe"}, builtin, user),
             "a non-resource listed": (kinds, builtin | {"Writer"}, user),
             "Request not registered": (kinds, builtin, user - {"Request"})}
    missed = [name for name, c in cases.items() if not problems(*c)]
    if problems(kinds, builtin, user) or missed:
        print(f"res-kind-check: selftest FAIL (missed: {missed})", file=sys.stderr)
        return 1
    print(f"res-kind-check: selftest ok: {len(cases)} mutants rejected")
    return 0


def main():
    sgoargs.check(flags={"--selftest": "bool"})
    if "--selftest" in sys.argv[1:]:
        return selftest()
    kinds, builtin, user = sources()
    errs = problems(kinds, builtin, user)
    for e in errs:
        print("res-kind-check:", e, file=sys.stderr)
    if errs:
        print(f"res-kind-check: FAIL ({len(errs)} problems)", file=sys.stderr)
        return 1
    print(f"res-kind-check: ok: {len(kinds)} resource kinds; built-in {sorted(builtin)}, registered {sorted(user)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
