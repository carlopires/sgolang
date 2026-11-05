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
compiler but not to the list would silently stay in the child. It also requires that the lists of
built-in resource types in SGO-RES-001 and SGO-RES-002 (sgo/spec/SPEC-RES.md) name exactly the
types in builtinHandles, so the specification cannot drift from the runtime (W25 review P3).

  res-kind-check.py              check the sources
  res-kind-check.py --selftest   show that a list missing a kind, or naming a non-resource, fails
"""
import sgoargs
import os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
RESFLOW = "src/cmd/compile/internal/types2/sgo_resflow.go"
TASK = "src/sgo/task.go"
SGODIR = "src/sgo"
SPEC = "sgo/spec/SPEC-RES.md"
# Where each clause lists the built-in resource types: the text before the list's parenthesis.
SPEC_LISTS = {"SGO-RES-001": "value of a built-in resource type (",
              "SGO-RES-002": "inside a built-in resource's scope ("}


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


def clause_lists(spec):
    """The built-in resource types each clause of SPEC_LISTS names, by clause."""
    out = {}
    for cid, lead in SPEC_LISTS.items():
        m = re.search(r'\n### ' + cid + r' .*?(?=\n### |\Z)', spec, re.S)
        if not m:
            raise SystemExit(f"res-kind-check: cannot find {cid} in {SPEC}")
        i = m.group(0).find(lead)
        if i < 0:
            raise SystemExit(f"res-kind-check: cannot find the built-in type list of {cid} in {SPEC}")
        body = m.group(0)[i + len(lead):]
        out[cid] = set(re.findall(r'`\*sgo\.(\w+)`', body[:body.index(')')]))
    return out


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


def problems(kinds, builtin, user, lists=None):
    errs = []
    for cid, names in sorted((lists or {}).items()):
        if names != builtin:
            errs.append(f"{cid} ({SPEC}) lists built-in resource types {sorted(names)}, but builtinHandles "
                        f"has {sorted(builtin)}")
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
    return compiler_kinds(read(RESFLOW)), builtin_list(read(TASK)), registered(srcs), clause_lists(read(SPEC))


def selftest():
    kinds, builtin, user, lists = sources()
    short = dict(lists, **{"SGO-RES-002": lists["SGO-RES-002"] - {"Timer"}})
    cases = {"Timer removed": (kinds, builtin - {"Timer"}, user, lists),
             "a new kind": (kinds | {"Pipe"}, builtin, user, lists),
             "a non-resource listed": (kinds, builtin | {"Writer"}, user, lists),
             "Request not registered": (kinds, builtin, user - {"Request"}, lists),
             "a clause list without Timer": (kinds, builtin, user, short)}
    missed = [name for name, c in cases.items() if not problems(*c)]
    if problems(kinds, builtin, user, lists) or missed:
        print(f"res-kind-check: selftest FAIL (missed: {missed})", file=sys.stderr)
        return 1
    print(f"res-kind-check: selftest ok: {len(cases)} mutants rejected")
    return 0


def main():
    sgoargs.check(flags={"--selftest": "bool"})
    if "--selftest" in sys.argv[1:]:
        return selftest()
    kinds, builtin, user, lists = sources()
    errs = problems(kinds, builtin, user, lists)
    for e in errs:
        print("res-kind-check:", e, file=sys.stderr)
    if errs:
        print(f"res-kind-check: FAIL ({len(errs)} problems)", file=sys.stderr)
        return 1
    print(f"res-kind-check: ok: {len(kinds)} resource kinds; built-in {sorted(builtin)}, registered {sorted(user)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
