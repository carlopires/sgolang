# W12 review brief: independent invariant review (Q01, D75)

**Reviewer:** `sgolang-reviewer` (note2, `job:note2/id:77bfab38c5a3`), with its own clone and
context. **Implementer:** the main job, which receives findings and fixes them.
**Scope:** the S-Go change from the upstream pin `2aae1000ea` (go1.27.3) to the review commit
named in the request message. Spec version 0.6.0.

The review is adversarial. Its purpose is to find places where the implementation, the
specification, or a published claim is wrong, weaker than stated, or bypassable. "Looks fine"
needs evidence: a test you ran, or code you traced.

## 1. What to read first (normative order)

1. `sgo/SGO_LANGUAGE_CONSTITUTION.md`: the invariants that override everything else
   (default-deny, no compatibility mode, no unsafe tier, no unspecified fallback).
2. `sgo/spec/SPEC-*.md` plus `sgo/SGO_SPEC_ADMISSION.json` (the closed feature registry) and
   `sgo/TARGET_TCB.json` (trusted files and their digests).
3. `sgo/SGO_FRONTIER_EXECUTION_PLAN.md` §11 (W12), §12.3 (release blockers), §15.2 (the six
   questions a release must answer).
4. `sgo/SGO_DECISIONS.md` (D01–D82): why things are the way they are. A decision is not a
   defense if it contradicts the constitution or a frozen clause.
5. Evidence per work package: `sgo/evidence/w00` … `w11/README.md`.

## 2. Checklist (plan §11/W12)

For each item, verify the code against the clause. Don't take the evidence README's word for it.

| Area | Main code | What to check |
|---|---|---|
| Root paths / ownership | `src/runtime/sgo_scope.go`, `sgo_nursery.go`, `sgo_record.go`, `sgo_main.go` | Every task is owned by exactly one nursery; no orphan on success; scope records reachable by the GC; no pointer into a movable stack |
| Lock order | `sgo_scope.go`, `sgo_cancel.go`, `sgo_chan.go`, `sgo_sync.go`, `sgo_io.go`, `sgo_net.go` | Documented order exists and is followed; no lock held across park; no lost wakeup |
| Linearization points | `sgo_chan.go` (permits, rendezvous, buffered, close), `sgo_native_ops.go`, `sgo_ready.go`, `sgo_sync.go` | Each operation has one commit point; cancel-vs-commit and close-vs-commit races resolve as SPEC-CHAN/SPEC-PERMIT say; no duplicate publication; no capacity leak |
| Failure propagation | `sgo_nursery.go`, `sgo_cancel.go`, `panic.go`/`stack.go` hooks | Primary error selection (lowest id), cancellation of siblings, drain budget, error chains (SPEC-ERRORS), exit codes |
| Exception edges | `panic.go` hooks, defer handling, `Goexit` refusal | Panic inside defer, panic with permits held, unwind across nested nurseries, stack overflow → exit 4 |
| Policy bypasses | `src/cmd/compile/internal/types2/sgo*.go`, `sgoadmit/`, `noder/sgo_*`, `src/cmd/sgo/` (driver, `tcb*.go`, `module.go`, `iogrant.go`), `src/cmd/link/internal/ld/sgoid.go` | Can user code reach anything not admitted: `go` statements, `unsafe`, `reflect`, linkname, cgo, asm, raw syscalls, TCB packages, other modules' grants? Can a flag or environment variable change semantics? Can a dependency acquire an effect it didn't declare? Can a package spoof a trusted path? |
| Effects and grants | `sgo_auth.go`, `sgo_effects.go`, `sgo_iogrant.go`, `sgo_fs.go`, `sgo_iodir.go`, `sgo_net*.go`, `src/sgo/*.go` | Path confinement (`..`, symlinks, absolute paths, races between check and use), endpoint grants, handle lending and revocation, attenuation |
| Replay scope | `sgo_lab*.go`, `sgo_iofake.go`, `sgo_netfake.go`, `walk/sgo_lab.go`, `src/cmd/sgo/lab*.go`, SPEC-LAB | Every source of nondeterminism is either recorded or excluded by admission; a lab binary can't be mistaken for production or vice versa; trace tamper detection; no silent fallback to real effects |
| Identity | `noder/sgo_identity.go`, `sgoadmit/identity.go`, `link/ld/sgoid.go`, `cmd/sgo/identity.go` | Spec/admission/TCB digests and execution configuration (prod/lab/test) can't be mixed; stale artifacts are refused |
| Published claims | `sgo/evidence/*/README.md`, `SGO_TEST_MATRIX.json`, spec text | Any claim stronger than its evidence; any test marked passed from inspection; any limitation that is really a compatibility escape (a release blocker, plan §11/W12) |

## 3. How to build and run (linux/amd64, note2)

Never use a `go` on PATH. The scripts select the pinned bootstrap themselves.

```bash
git fetch origin && git checkout <review commit>
sgo/scripts/build-stage0.sh                                   # stage-0 toolchain into pkg/tool/linux_amd64
python3 sgo/scripts/conformance.py run --sgo pkg/tool/linux_amd64/sgo --pending sgo/testdata/conformance-pending.json
python3 sgo/scripts/test-tcb.py; python3 sgo/scripts/test-flags.py; python3 sgo/scripts/test-identity.py
python3 sgo/scripts/test-lab.py; python3 sgo/scripts/test-isolation.py; python3 sgo/scripts/test-examples.py
sgo/scripts/runtime-lane.sh -run 'TestSGo' runtime            # runtime tests (host-only harness)
sgo/scripts/runtime-lane.sh -race -run 'TestSGo' runtime
python3 sgo/scripts/spec-tool.py check; python3 sgo/scripts/inventory-check.py
```

To write a probe program: make a module under `.work/` with an `sgo.mod` copied from any
`sgo/testdata/conformance/*/sgo.mod` and a `main.sgo`, then
`pkg/tool/linux_amd64/sgo build` / `run` / `check` / `test` / `lab run`. A probe that shows a
bypass is the best evidence a finding can have, so please keep it in the finding.

## 4. Finding format (one per message section, or batched)

```text
### R<n> — <title>
- severity: blocker | major | minor
    blocker: violates the constitution, a frozen clause, or a plan §12.3 release blocker
             (orphan, missing GC root, use-after-recycle, duplicate publication, capacity leak,
             lost wakeup, unreported partial I/O, wrong return/defer, silent policy bypass,
             replay fallback), or a compatibility escape
    major:   a published claim not supported by its evidence, or a reachable defect with a
             specified-but-wrong outcome
    minor:   everything else
- location: file:line (at the review commit)
- clause / claim: SGO-XXX-NNN or the evidence sentence
- evidence: probe source + command + output, or the traced code path
- recommended fix: precise enough that the implementer won't drift (what to change, where,
  and what must not change)
- regression test: the fixture or test that should fail before the fix and pass after
```

Refactor recommendations are welcome when they remove a class of defect; justify each one.

## 5. Process

1. The reviewer reports findings in batches (blockers as soon as found).
2. The implementer records each one in `sgo/evidence/w12/review-findings.json`, fixes it with the
   regression test, and replies with the fix commit.
3. The reviewer re-checks each fix at that commit and marks it `verified` or reopens it.
4. The review ends when the reviewer states the remaining open findings (ideally none of severity
   blocker or major) and that the checklist in §2 is covered, listing anything not reviewed.

The reviewer works read-only on the repository, with temporary files under `.work/` (removed
afterwards). The reviewer does not push.
