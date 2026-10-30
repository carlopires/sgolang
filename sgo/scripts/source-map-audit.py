#!/usr/bin/env python3
"""source-map-audit.py — B06: resolve every symbol/file named by the original phase documents and
the plan §10.3 source map against the actual checkout. Writes sgo/docs/source-map.md.

Absent or mismatched symbols are recorded as audit findings; they must not be recreated as
unused stubs (plan §10.3). Rerun after rebasing the fork.
"""
import json, os, re, subprocess, sys

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SRC = os.path.join(ROOT, "src")
LOCK = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))

def grep(pattern, path):
    """Return 'file:line' hits for a regex under src/<path> (file or directory)."""
    target = os.path.join(SRC, path)
    if not os.path.exists(target):
        return None
    try:
        out = subprocess.check_output(
            ["grep", "-rnE", "--include=*.go", "--include=*.s", "--include=*.md", pattern, target],
            text=True)
    except subprocess.CalledProcessError:
        return []
    return [os.path.relpath(l.split(":")[0], ROOT) + ":" + l.split(":")[1] for l in out.splitlines()]

# (source document, claim, path under src/, regex, note)
ORIGINAL = [
    ("PHASE1", "func needStackCheck in ssa/stackframe.go", "cmd/compile/internal/ssa/stackframe.go", r"func needStackCheck", ""),
    ("PHASE1", "needStackCheck anywhere in ssa", "cmd/compile/internal/ssa", r"needStackCheck", ""),
    ("PHASE1", "stack check emitted in prologue (stacksplit)", "cmd/internal/obj/x86/obj6.go", r"func stacksplit", "Real location of amd64 prologue checks (R3)."),
    ("PHASE1", "arm64 prologue stack check", "cmd/internal/obj/arm64/obj7.go", r"func \(c \*ctxt7\) stacksplit", "D22: arm64 is first-class; equivalent location."),
    ("PHASE1", "case ssa.OpAMD64LoweredStackCheck in amd64/ssa.go", "cmd/compile/internal/amd64/ssa.go", r"LoweredStackCheck", ""),
    ("PHASE1", "op LoweredStackCheck defined anywhere", "cmd/compile/internal/ssa", r"LoweredStackCheck", ""),
    ("PHASE1", "func newstack in runtime/stack.go", "runtime/stack.go", r"^func newstack\(", ""),
    ("PHASE1", "func sysAlloc", "runtime/mem.go", r"^func sysAlloc\(", "Signature must be checked; snippet passes (size, &memstats.stacks_sys)."),
    ("PHASE1", "func sysMap", "runtime/mem.go", r"^func sysMap\(", ""),
    ("PHASE1", "memstats.stacks_sys", "runtime", r"stacks_sys", ""),
    ("PHASE2", "token list in syntax/tokens.go", "cmd/compile/internal/syntax/tokens.go", r"_Var\b", ""),
    ("PHASE2", "keywordMap map[string]token", "cmd/compile/internal/syntax", r"keywordMap", "Plan §4.1/R15: scanner uses a keyword hash table, not a map."),
    ("PHASE2", "keyword hash table in scanner", "cmd/compile/internal/syntax/scanner.go", r"func keyword|keywordMap|hash\(", ""),
    ("PHASE2", "stmt embedding in syntax/nodes.go", "cmd/compile/internal/syntax/nodes.go", r"^\s*stmt$", ""),
    ("PHASE2", "func (p *parser) blockStmt", "cmd/compile/internal/syntax/parser.go", r"func \(p \*parser\) blockStmt\(", ""),
    ("PHASE2", "func (p *parser) got / want", "cmd/compile/internal/syntax/parser.go", r"func \(p \*parser\) (got|want)\(", ""),
    ("PHASE2", "statement dispatch (stmtOrNil)", "cmd/compile/internal/syntax/parser.go", r"func \(p \*parser\) stmtOrNil\(", "Where a nursery statement would be dispatched."),
    ("PHASE3", "file noder/stmt.go", "cmd/compile/internal/noder/stmt.go", r".", ""),
    ("PHASE3", "type gen (old noder irgen)", "cmd/compile/internal/noder", r"^type gen struct|func \(g \*gen\)", "Old irgen path; Unified IR replaced it (R2, R8)."),
    ("PHASE3", "Unified IR statement writer", "cmd/compile/internal/noder/writer.go", r"func \(w \*writer\) stmt1\(", ""),
    ("PHASE3", "Unified IR statement reader", "cmd/compile/internal/noder/reader.go", r"func \(r \*reader\) stmt1\(", ""),
    ("PHASE3", "typecheck.LookupRuntime", "cmd/compile/internal/typecheck", r"^func LookupRuntime\(", ""),
    ("PHASE3", "ir.OVARDEF", "cmd/compile/internal/ir", r"\bOVARDEF\b", ""),
    ("PHASE3", "ir.NewDeferStmt", "cmd/compile/internal/ir", r"func NewDeferStmt|func NewGoDeferStmt", ""),
    ("PHASE3", "ir.NewDecl / ODCL", "cmd/compile/internal/ir", r"func NewDecl\(|\bODCL\b", ""),
    ("PHASE3", "ir.PAUTO", "cmd/compile/internal/ir", r"\bPAUTO\b", ""),
    ("PHASE3", "runtime NurseryScope/ScopeInit/ScopeQuiesce/ContextInit/getCurTask", "runtime", r"NurseryScope|ScopeInit|ScopeQuiesce|ContextInit|getCurTask", "Proposed names; expected absent."),
    ("PHASE4", "file runtime/lab_sched.go", "runtime/lab_sched.go", r".", "Proposed file; expected absent."),
    ("PHASE4", "upstream synctest runtime support", "runtime/synctest.go", r"^func synctest", "Plan §9.1: study and extend this."),
    ("PHASE4", "type g struct", "runtime/runtime2.go", r"^type g struct", ""),
    ("PHASE5", "type hchan struct", "runtime/chan.go", r"^type hchan struct", ""),
    ("PHASE5", "func chanbuf", "runtime/chan.go", r"^func chanbuf\(", ""),
    ("PHASE5", "func typedmemmove", "runtime/mbarrier.go", r"^func typedmemmove\(", ""),
    ("PHASE5", "func gopark (signature)", "runtime/proc.go", r"^func gopark\(", "Snippet passes traceEvGoBlockSend; check the real trace-reason parameter."),
    ("PHASE5", "waitReasonChanSend", "runtime/runtime2.go", r"waitReasonChanSend\b", ""),
    ("PHASE5", "traceEvGoBlockSend", "runtime", r"traceEvGoBlockSend", "Old trace event name; expected absent in the v2 tracer."),
    ("PHASE5", "traceBlockChanSend", "runtime", r"traceBlockChanSend", "Current trace block reason (if present)."),
]

PLAN = [
    ("Compiler overview", "cmd/compile/README.md"),
    ("Private lexer tokens", "cmd/compile/internal/syntax/tokens.go"),
    ("Private scanner", "cmd/compile/internal/syntax/scanner.go"),
    ("Private syntax nodes", "cmd/compile/internal/syntax/nodes.go"),
    ("Parser", "cmd/compile/internal/syntax/parser.go"),
    ("Type checker", "cmd/compile/internal/types2"),
    ("Unified IR writer", "cmd/compile/internal/noder/writer.go"),
    ("Unified IR reader", "cmd/compile/internal/noder/reader.go"),
    ("IR", "cmd/compile/internal/ir"),
    ("Walk", "cmd/compile/internal/walk"),
    ("SSA gen", "cmd/compile/internal/ssagen"),
    ("Escape analysis", "cmd/compile/internal/escape"),
    ("Liveness", "cmd/compile/internal/liveness"),
    ("Compiler runtime builtin declarations", "cmd/compile/internal/typecheck/_builtin/runtime.go"),
    ("Generated builtin table", "cmd/compile/internal/typecheck/builtin.go"),
    ("Public go/token", "go/token"), ("Public go/scanner", "go/scanner"), ("Public go/ast", "go/ast"),
    ("Public go/parser", "go/parser"), ("Public go/types", "go/types"), ("Public go/printer", "go/printer"),
    ("Public go/format", "go/format"), ("gofmt", "cmd/gofmt"),
    ("Task creation", "runtime/proc.go"), ("Task layout", "runtime/runtime2.go"),
    ("Panic/defer", "runtime/panic.go"), ("Stacks", "runtime/stack.go"),
    ("amd64 prologue", "cmd/internal/obj/x86/obj6.go"), ("arm64 prologue", "cmd/internal/obj/arm64/obj7.go"),
    ("Memory", "runtime/mem.go"), ("Channels", "runtime/chan.go"), ("Select", "runtime/select.go"),
    ("Synctest runtime", "runtime/synctest.go"), ("Synctest package", "testing/synctest"),
    ("Timers", "runtime/time.go"), ("Netpoll (linux)", "runtime/netpoll_epoll.go"),
    ("Netpoll (darwin)", "runtime/netpoll_kqueue.go"),
    ("darwin libSystem calls", "runtime/sys_darwin.go"), ("linux raw syscalls (amd64)", "runtime/sys_linux_amd64.s"),
    ("linux raw syscalls (arm64)", "runtime/sys_linux_arm64.s"),
    ("Build driver", "cmd/go"), ("Bootstrap/dist", "cmd/dist"), ("Linker", "cmd/link"),
]

FINDINGS = """## 3. Confirmed findings (manual, verified 2026-10-30 at go1.27.3)

1. **Keyword table is a perfect hash, and `nursery` collides.** `syntax/scanner.go` uses `var keywordMap [1 << 6]token`
   indexed by `hash(s) = (s[0]<<4 ^ s[1] + len(s)) & 63`, populated in `init` for tokens `_Break.._Var`, panicking on
   collision. `hash("nursery") = 28 = hash("chan")` (checked by running the function with the pinned bootstrap Go).
   **Fixed in the fork:** the table is now `[1 << 7]token` (128 slots), where all Go keywords plus `nursery` are distinct;
   `TestKeywordHashRoom` fails on upstream and passes on the fork (host test lane). W03 still adds the `_Nursery` token inside
   the `_Break.._Var` range; the PHASE2 `map[string]token` snippet does not apply. `go/scanner` and `go/token` have separate keyword tables that must agree (plan §4.1).
2. **No `needStackCheck` / `LoweredStackCheck`.** Prologue stack checks are emitted by the assemblers' `stacksplit`
   (`obj/x86/obj6.go` for amd64, `obj/arm64/obj7.go` for arm64), not by an SSA op. PHASE1's compiler edits have no target;
   fixed stacks remain the separate W11 investigation.
3. **No `noder/stmt.go`, no `gen` type, no `OVARDEF`.** The frontend is Unified IR (`noder/writer.go` `stmt1`,
   `noder/reader.go` `stmt1`). PHASE3's lowering must be redesigned against Unified IR (plan §5.4); liveness uses
   `VarDef`-style handling inside `liveness/`, not an `ir.OVARDEF` node.
4. **Tracing renamed.** `traceEvGoBlockSend` is gone; channel blocking uses `traceBlockChanSend`. `gopark`'s real
   signature must be read before any channel change (PHASE5).
5. **Lab starting point exists.** `runtime/synctest.go` and `testing/synctest` are present; extend them (plan §9.1).
""".splitlines()


def main():
    commit = subprocess.check_output(["git", "-C", ROOT, "rev-parse", "HEAD"], text=True).strip()
    base = LOCK["upstream"]["commit"]
    changed = subprocess.check_output(["git", "-C", ROOT, "diff", "--name-only", base, "HEAD", "--", "src"], text=True).split()
    lines = [
        "# Source map (generated)", "",
        f"Generated by `sgo/scripts/source-map-audit.py` (test case B06). Upstream `{LOCK['upstream']['tag']}` = `{base}`; "
        f"audited fork commit `{commit}`; fork changes under `src/` relative to upstream: {len(changed)} file(s).",
        "", "Findings are evidence for plan §2/§10.3: an absent symbol from the original documents is an audit finding, "
        "**not** a reason to create it. Line numbers refer to the audited commit.", "",
        "## 1. Symbols named by the original phase documents", "",
        "| Doc | Claim | Result | Location(s) | Note |", "|---|---|---|---|---|",
    ]
    absent = 0
    for doc, claim, path, pat, note in ORIGINAL:
        hits = grep(pat, path)
        if hits is None:
            res, loc = "**ABSENT (no such path)**", f"`src/{path}`"; absent += 1
        elif not hits:
            res, loc = "**ABSENT**", f"`src/{path}`"; absent += 1
        else:
            res = "found"
            loc = ", ".join(f"`{h}`" for h in hits[:3]) + (f" (+{len(hits)-3} more)" if len(hits) > 3 else "")
        lines.append(f"| {doc} | {claim} | {res} | {loc} | {note} |")
    lines += ["", f"{absent} of {len(ORIGINAL)} original claims did not resolve as written.", "",
              "## 2. Plan §10.3 starting points", "", "| Area | Path | Exists |", "|---|---|---|"]
    missing = 0
    for area, path in PLAN:
        ok = os.path.exists(os.path.join(SRC, path))
        missing += not ok
        lines.append(f"| {area} | `src/{path}` | {'yes' if ok else '**NO**'} |")
    lines += ["", f"{missing} of {len(PLAN)} plan paths are missing.", ""]
    lines += FINDINGS
    out = os.path.join(ROOT, "sgo", "docs", "source-map.md")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    open(out, "w").write("\n".join(lines))
    print(f"wrote {os.path.relpath(out, ROOT)}: {absent} unresolved original claims, {missing} missing plan paths")

if __name__ == "__main__":
    main()
