# v2 correction log

**Supersedes:** S-Go execution handoff v1.0, prepared 2026-10-02.

## Why this is a replacement, not an addendum

The user rejected compatibility paths and any compromise with the existing ecosystem, and requires valid specifications for code admitted into S-Go. v1's compatibility profile, implicit dynamic `go` inheritance, preserved standard-library paths, and compatibility release gates contradicted that direction. Those assumptions have been removed from the active plan, start prompt, task ledger, and acceptance matrix rather than overridden in a disconnected note.

## Changes

- Add a constitution that distinguishes the user's fixed requirements from proposed API details.
- Replace compat/strict/lab dialect planning with one S-Go language and production/lab execution configurations carrying identical safety invariants.
- Recommend rejecting bare `go` everywhere and using explicit opaque nursery capabilities. Separate nursery authority from task contexts; observe child callback errors.
- Add a closed, default-deny specification-admission policy for syntax, types/values, callables, imports, generated/indirect paths, effects, intrinsics, and target-runtime services.
- Add a machine-readable specification inventory seed. It is not a completed spec or an allowlist; every entry starts unadmitted.
- Replace inherited Go/stdlib acceptance with deliberate, frozen clause adoption and contract-checked source porting. No preservation requirement remains.
- Separate external host bootstrap/reference tools from S-Go target compiler/runtime/programs. Prohibit a bootstrap parser mode and stock-object fallback; require an explicit host/target stage map.
- Replace compatibility tests with S-Go conformance, legacy rejection, admission coverage, and same-safety production/lab checks.
- Update 13 work packages, revise affected original scenarios, and add 20 specification/no-legacy scenarios: 147 acceptance scenarios total. All are still pending/unrun.
- Preserve the seven original documents byte-for-byte as historical input. They are not active instructions when they conflict with v2.

## Deliberate limits

The exact `n.Spawn` spelling is a recommended new design default, not an explicit user instruction. Removing a compatibility obligation does not logically require every familiar Go syntax form to disappear; reused forms require complete S-Go clauses. The chosen function-defer policy, recoverable cancellation policy, and stack experiment remain explicit engineering choices, not compatibility constraints.

The v2 work corrected planning artifacts and validated document/metadata consistency. It did not implement a compiler, freeze every normative language clause, prove arbitrary program correctness, execute conformance tests, or re-verify external sources carried forward from v1. W00/W01 must establish those implementation/specification baselines honestly.

## 2026-10-30 reference correction

- The input archive's "BlenD2" is not a real concurrency reference. It is most likely a garbled **Bend 2** (`github.com/bendlang/bend`), but Bend 2 contains none of the ideas attributed to it (discrete-event simulation, reserve/commit channels, a seeded scheduler). Those ideas come from asupersync (R13).
- Plan §2 provenance note rewritten; §7 renamed "Reserve/commit/abort contract" and attributed to R13; R13 annotated (advisory production cleanup budgets: S-Go's fail-stop budget is its own design); R18 (Bend 2) added as a non-normative analogue of specification admission only.
- `original/` left byte-for-byte unchanged; it still says "BlenD2" as historical input. No contract, task, or acceptance case changed.

## 2026-10-30 open decisions document

- Added `SGO_DECISIONS.md` (proposed, not approved): 21 decisions with options and recommendations covering the data-race model, integer overflow, subset-0 types and constructs, program entry/exit and output, panic/recover, spec format, user-level contracts, map order, and the upstream pin (`go1.27.3` recommended; `go1.27.1` = `862c888e…`, `go1.27.3` = `2aae1000…`, checked against go.googlesource.com).
- No plan, ledger, or test change yet; amendments follow approval.

## 2026-10-30 decisions approved; subset 0 drafted

- User approved D01–D21 as recommended. D22 (first execution target; development host is darwin/arm64) added and left open; Linux/amd64 stays the default until decided.
- Plan amended: §0 amendment note, §1.2 pin `go1.27.3` and D22 note, §4.2 entry point/globals (D07, D08), §4.6 `recover` denied in subset 0 (D13), §15.1 subset-0 pointer, W00 pin.
- Start prompt: reading order adds `SGO_DECISIONS.md` and `SGO_SPEC_SUBSET0.md`; pin `go1.27.3`; implementation repository `github.com/carlopires/sgolang`.
- `SGO_SPEC_ADMISSION.json`: 10 families added (FLOAT, COMPOSITE, USERTYPE, IFACE, GENERIC, REFLECT, NATIVE, SYNC, PROC, TEST), 37 total; every row tagged `subset0` = partial or denied.
- Added `SGO_SPEC_SUBSET0.md` (draft, not frozen): clauses for subset 0 with IDs per D18, a denied list, and nine open drafting items for W01.

## 2026-10-30 D22 approved

- Targets: darwin/arm64 first (development host); linux/amd64 the real target; linux/arm64 first-class. Plan §1.2 and start prompt amended. From this point the canonical bundle is `sgo/` in `github.com/carlopires/sgolang`.
