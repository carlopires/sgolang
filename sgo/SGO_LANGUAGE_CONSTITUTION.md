# S-Go language constitution

**Version:** 2.0 · **Date:** 2026-10-02 · **Status:** authoritative project constraints plus explicitly labeled design defaults; not a completed formal language specification.

## 1. User requirement and precedence

The user clarified:

> “the reason for the fork is because I don't want any compatibility path; there is no compromise with the existing ecosystem; I don't want any code that lacks valid specification to exist in the new language.”

This requirement overrides handoff v1.0. The correct product is an independent, specification-defined language built by forking Go implementation machinery. It is not Go with an optional structured-concurrency profile.

Precedence: explicit user requirements → this constitution → frozen S-Go normative clauses and approved amendments → v2 execution plan → task/test ledgers → historical input. Conflicts block admission of the affected feature until resolved; implementation behavior cannot silently settle a specification conflict.

The no-compatibility requirement is not open for the coding agent to trade away. More specific implementation choices below are proposed defaults, not claims that the user selected every API spelling.

## 2. Non-negotiable constraints

**One language.** No `compat` mode, opt-in safety dialect, unsafe/permissive switch, legacy package exemption, raw channel tier, automatic Go fallback, or source pragma that restores old semantics. Production and lab are execution configurations of the same language. They preserve the same ownership, authority, cancellation, and failure rules.

**The S-Go specification is the authority.** Reusing syntax or source does not inherit semantics. Every admitted construct, builtin, callable/effect surface, import, intrinsic, and target-runtime boundary must have a valid S-Go contract. An inherited feature without one is rejected. “The Go compiler already accepts this” is not an admission argument.

**Dependencies are not grandfathered.** Go packages are source material to port or reject. Recompilation alone, a package name, a signature, a comment, a contract file with no enforcement, or a standard-library origin is insufficient. Admission includes the complete dependency graph, initialization, callbacks, indirect calls, generic instantiations, reflection, generated source, and build artifacts.

**Trust does not waive specification.** Target runtime, GC, scheduler, native adapters, and sealed low-level intrinsics require individual contracts and an audited call boundary. Native implementation privileges are not exposed through a general-purpose unsafe application subset. External host build tools are separately identified implementation dependencies, not S-Go source or target objects.

**No unspecified fallback.** A missing, contradictory, stale, or unsupported specification causes a build/admission error. A runtime-dependent precondition has a specified checked failure before the prohibited operation. Warnings, weaker guarantees, disabled checks, and “continue using Go behavior” are not substitutes.

## 3. What a valid specification must contain

For each language/API/intrinsic feature, its admission entry must reference frozen normative clauses covering:

| Contract dimension | Required content |
|---|---|
| Identity | Stable clause ID, version, scope, referenced-clause closure, and content digest. |
| Static legality | Grammar, types, binding, preconditions, capability/effect requirements, and the rules the compiler/linker can check. |
| Value semantics | Evaluation order, results, aliasing rules, state transitions, boundary and resource-exhaustion behavior. |
| Concurrency | Ownership, allowed sharing, synchronization, happens-before edges, permitted nondeterminism, admission/commit linearization. |
| Failure | Error/panic/cancellation outcomes, partial progress, cleanup, termination, and violation diagnostics. |
| Enforcement | Exact compile/link checks, runtime guards, TCB assumptions, and indirect-call/import obligations. |
| Evidence | Positive and negative conformance cases, model/mutation cases where useful, implementation coverage, and actual execution status. |

A field may be not applicable only with a reason. A TODO or “whatever upstream does” is not a specification. A set of allowed nondeterministic outcomes can be a valid specification; unrestricted unspecified behavior cannot.

A callable's checked signature/effect summary and the rules governing its body must fit the admitted subset. A declaration is not trusted merely because the programmer wrote it. Functional pre/postconditions introduced by the language require a defined proof or runtime-checking discipline; the agent must not invent a claim of automatic verification of arbitrary algorithms.

A specification can explicitly adopt a bounded upstream clause, but must freeze its exact revision/content, recursively close normative dependencies, resolve conflicts with S-Go, and supply conformance tests. There is no blanket “Go applies unless overridden” rule and no obligation to follow future Go changes.

This admission policy is not itself a finished specification of the whole language. `SGO_SPEC_ADMISSION.json` is an initial inventory to expand and freeze. Entries start unadmitted. The first compiler must support a small **closed and specified subset**, rather than accept a large partly specified language.

## 4. Recommended v2 task-creation decision

**Remove bare `go f()` from S-Go.** Reject it everywhere in target source, including inside nurseries, dependencies, initializers, and unused functions. Do not silently translate it.

Use an explicit nursery capability:

```go
func helper(n *sgo.Nursery) {
    n.Spawn(func(task *sgo.Context) error {
        return worker(task)
    })
}

func main() {
    nursery (n) {
        helper(n)
    }
}
```

This is the recommended v2 surface, not already implemented code. The spelling may be amended after specification review; explicit authority and no detached execution remain mandatory for this design.

A nursery is not a task context. `n` authorizes child admission; a task context authorizes only its declared cancellation/effect operations. Missing spawn authority is never supplied by ambient state. Current runtime scope metadata is used to **validate** the passed handle, not to choose an implicit owner.

The spawn contract must cover handle provenance/liveness, task membership, nested-scope restrictions, cancellation and join races, callback types/effects, publication ordering, and every callback outcome. An error returned by the child callback is a nursery-observed failure, not a discarded return value. No unrelated task, expired handle, or cleanup context may create a child.

A helper may return while its explicitly delegated nursery still contains a child: the nursery is the named lifetime boundary. A helper without that delegation must use a nursery it closes itself before returning. The specification must not pretend that nursery ownership means all spawned work always finishes at the immediate helper's return.

## 5. Build architecture is not a compatibility exception

The compiler may initially be implemented in host Go and built with a pinned external Go toolchain. That is analogous to a compiler being implemented in another language; it does not make that implementation language valid input to the compiler.

Keep host compiler/runtime sources, host dependencies, and untouched upstream reference tests in separate build identities and caches. The S-Go target compiler must not accept legacy target source, even to rebuild its own implementation. Port the implementation before claiming self-hosting. Replace inherited build-stage assumptions rather than adding a compatibility switch.

Target low-level runtime mechanisms require sealed, individually specified intrinsics and source/build identity checks. They are not an unrestricted source dialect, package-name privilege, or escape hatch available to applications. Stock Go object files are not valid S-Go target objects.

## 6. Mandatory admission/release gates

W00 isolates host/reference tooling and identifies every inherited build route. W01 freezes the first-subset specification, closes its normative dependencies, and creates the complete default-deny inventory. W02/W03 implement explicit task admission and frontend/type/link gates. W05 completes the advertised capability/effect surface without changing default denial. Later work packages add only specified features.

For each shipped feature the trace is: **normative clause → admission rule → compiler/runtime implementation → positive and rejection tests → executed evidence**. Missing steps block the relevant feature. The release must have no uncontracted accepted path, including through optimizer fast paths, source tools that feed builds, runtime services, or cached dependencies.

An old Go program failing to compile can be a correct S-Go result. An upstream test is reusable only when its expected behavior is explicitly adopted by S-Go; otherwise it belongs in the external comparison suite or becomes a negative fixture. Existing ecosystem compatibility is never a release gate.

The fixed-stack investigation remains a separate correctness/performance experiment. Its sequencing does not create a compatibility path: either stack implementation must satisfy the same S-Go runtime contracts.

## 7. Amendment discipline

Changes to API spelling, internal representation, or deliberately adopted semantics require updated clauses, admission entries, tests, and affected artifacts. Changes to the user's no-compatibility/specification-first requirement require an explicit user decision. The coding agent must not weaken it for convenience, library availability, bootstrap shortcuts, benchmark results, or an approaching session limit.
