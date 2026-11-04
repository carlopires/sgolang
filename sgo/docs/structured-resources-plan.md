# W21 — Structured resources and `check` (plan)

Decisions D147–D156 (approved 2026-11-04, `sgo/SGO_DECISIONS.md`). Target: spec **0.16.0**.

## Why

Nurseries are the reason S-Go exists. No task outlives its nursery, so no programmer has to
remember to stop one. W21 gives resources the same guarantee. A resource is acquired *into a
scope* (a nursery or a task body), and the scope releases it when it ends. A forgotten `Close`
stops being a bug, because there is nothing to remember. `check` removes the other half of the
ceremony: the error checks that only return the error.

Before (`examples/happyeyeballs/main.sgo` at 0.15.1): 80 lines, 11 error checks, 6 closes,
4 `LendTo`s, one slot. Target: the same program in about 25 lines, with no `Close`, no `LendTo`,
and no slot.

## The model of a scope

- **Scopes.** A *scope* is a nursery statement's body (its nursery `n`) or a task's body (its
  context: `root`, or the `t` of a spawned callback). Scopes nest: a nursery opened in a body is
  inside that body's scope, and a task spawned into `n` is inside `n`.
- **Acquire.** Every resource is acquired into exactly one scope, named at the call:
  - a method on a scope acquires into that scope (`root.Listen(addr)`, `n.Dial(addr)`);
  - every other acquiring call takes the scope as its first argument (`l.Accept(t)`,
    `d.Dial(t, addr)`, `dir.Open(n, path)`).
- **Use.** Every task running inside the scope may use the resource. That is the acquiring task
  and every task spawned at any depth into a nursery nested in the scope (D150). Per-kind
  concurrency rules decide who waits.
- **End of a nursery.**
  1. Join the children (as today).
  2. Release the nursery's resources in reverse order of acquisition.
  3. Compute the outcome (SGO-NUR-004, with the release step of D149).
- **End of a task body.** Its resources are released before its outcome is published.
- **`Close`.** An optional early release. It returns the release's error to the caller and
  leaves nothing for the scope to do.
- **Escape (D151, static).** A resource value may not flow beyond the lexical extent of the name
  of its scope. `r.MoveTo(s)` re-ties it to an enclosing scope `s`.
- **User resources (D152).** `s.Own(r)` for a type with `Release(ok bool) error`. Release order
  is strict reverse acquisition across built-in and user resources.

## Safety argument (what the model must check)

1. **No use after release.** A resource is released only after every task that can use it has
   ended. Those tasks are the ones inside its scope, and they have all joined at step 1. A task
   outside the scope cannot name the resource, by D151.
2. **Every resource is released exactly once**: by its scope's end or by an explicit `Close`,
   never both, and never zero times. Every scope ends, including on unwind and cancellation
   (D155).
3. **`MoveTo` is safe.** It moves a resource only outward, to a scope `s` that encloses the
   resource's scope. The only tasks that can still use it after the move are the ones inside
   `s`, so property 1 holds for the new scope.
4. **No release error is lost** (D149). Each one reaches the nursery outcome, the task's error,
   or the chain.
5. **Order.** Releases within one scope run in strict reverse order of acquisition, so a user
   resource's `Release` runs while the handles it holds are still open.

## Steps

| step | what | output |
|---|---|---|
| S0 | **Model** (`sgo/model/scoperes.go`): scopes, tasks, resources; the operations acquire, use, close, `MoveTo`, spawn, join/release, cancel, release error; explore every interleaving of small programs; mutants that break each of properties 1–5 must be caught | model tests |
| S1 | **Clause drafts** in `sgo/docs/w21-drafts.md`: SGO-ERR-004 (`check`), SGO-RES-001..00n (structured resources, escape rule, `Own`/`Release`, `MoveTo`), amendments to SGO-LEX-003 (reserved word), SGO-CTL-001, SGO-NUR-004 (release step), SGO-FILE-003 and SGO-NET-002..004 (rewritten; `LendTo`, `HandleAbandoned`, and `ConnSlot` removed) | drafts, `spec-tool check` on them in place |
| S2 | **Runtime**: per-scope resource lists (replacing per-task handle lists), release after join in LIFO order, release-error routing, `MoveTo`, `Own`/`Release`, `ResourceBusy`; remove lending, abandonment, and the slot | runtime lane, linux vet |
| S3 | **Compiler**: `check` (parser, type rules, lowering into the comparison and return it stands for), the escape rule in the flow pass (`sgo_flow.go`, as for `Frozen`), new acquiring signatures in the admission lists | conformance |
| S4 | **API** (`src/sgo/io.go`, std): new signatures; fixtures for each acquire/use/release row, each escape violation, each place a release error can land, `check` positive and negative cases | conformance, lab |
| S5 | **Examples** rewritten (pipeline, service, replay, happyeyeballs); the classifier counts before and after are recorded as evidence | test-examples, lab sweeps |
| S6 | **Freeze 0.16.0**, evidence `sgo/evidence/w21/README.md`, review | review |

## Points settled in the S1 drafts (with the current recommendation)

- **Nursery-scoped root authority.** `n.Listen`, `n.Dial`, and `n.Dialer` are allowed when the
  nursery's owner is the root task, since authority is the root's. Recommendation: yes.
- **What `Release(false)` means for built-in kinds.** A close is a close, so built-in kinds
  ignore the bit.
- **`MoveTo` across tasks.** Moving from a child task's body to the enclosing nursery is the
  Happy Eyeballs case: the winning attempt's connection moves to the caller's nursery, and the
  losers are released when their tasks end. Two concurrent `MoveTo`s of different resources are
  independent. Choosing a single winner is the algorithm's own decision, made through an event
  or a channel as it is today.
- **Tokens and send permits.** These keep their D39 and D26 rules (D156). Folding them into
  scopes is a later decision.
- **Static escape check versus user resource types.** A type is a resource type if it has a
  `Release(bool) error` method, or holds a resource-type field, element, or capture. That makes
  it decidable from the type alone.
