# S-Go guarantees

Generated from `sgo/GUARANTEES.json` by `sgo/scripts/guarantee-check.py render`. Do not edit.

The published S-Go claim set (plan §10.1, D76). Every statement here is checked by sgo/scripts/guarantee-check.py against the frozen clauses, SGO_TEST_MATRIX.json and the evidence files. Nothing outside this file is a guarantee.

## Scope of a claim

- **admission**: statically rejected by the compiler, driver or linker
- **runtime_enforced**: checked at run time with a specified failure
- **model_explored**: explored over all interleavings of bounded model scenarios (bounds stated); not a proof for unbounded programs
- **tested**: executed acceptance cases and fixtures only

## Not claimed

- Proof that an arbitrary S-Go program is correct, terminates, or behaves correctly under all schedules
- Compatibility with Go programs or the Go ecosystem
- Instruction-level replay of arbitrary code
- Behavior on targets other than darwin/arm64, linux/amd64, linux/arm64

## Questions a release must answer (plan §15.2)

- **What can I compile?** See [G-LANG](#g-lang), [G-MEM](#g-mem), [G-DENY](#g-deny).
- **Which operations are cancellation-aware?** See [G-CANCEL](#g-cancel), [G-CHAN](#g-chan), [G-IO](#g-io).
- **Which effects are mediated?** See [G-AUTH](#g-auth), [G-IO](#g-io).
- **What exactly can I replay?** See [G-LAB](#g-lab).
- **What is rejected?** See [G-DENY](#g-deny), [G-TRUST](#g-trust), [G-TOOL](#g-tool).
- **What happens on a stuck task?** See [G-CANCEL](#g-cancel), [G-CHAN](#g-chan).
- **Which tests demonstrated each answer?** Each guarantee lists its acceptance cases; `SGO_TEST_MATRIX.json` holds their executed evidence.

## Guarantees

### G-LANG

A program compiles only if every construct it uses is in the closed S-Go subset (spec 0.20.0): .sgo sources, int/float64/byte/bool/string/error, value structs and arrays, named types with value- and pointer-receiver methods, named interface types and any with dynamic dispatch, comma-ok assertions and type switches, slices, insertion-ordered maps, pointers, frozen values (sgo.Frozen[T]), function values, the admitted statements. Task-local data (slices, maps, pointers, and interface values other than error) never crosses tasks except as a deep copy made by sgo.Copy, or frozen by sgo.Freeze, which no task can write (D85, D98–D101). Integer and byte overflow, division by zero, an out-of-range index or slice, a nil dereference (including a method call on a nil interface: there is no typed nil), and a write to a nil map are checked failures with a specified record, never wraparound or undefined behavior; map iteration order is insertion order, the same in production and in the lab. float64 arithmetic is IEEE 754 binary64 rounded once per operation and never fused (the one fused operation is math.FMA), so it gives the same bits on every target; one NaN is observable; a float map key is refused; a conversion of NaN, an infinity, or an out-of-range value to an integer type is a checked failure (IntegerOverflow).

- **Scope:** admission, runtime_enforced, tested
- **Families:** SPEC-LEX, SPEC-BIND, SPEC-TYPE, SPEC-VALUE, SPEC-CONTROL, SPEC-COMPOSITE, SPEC-USERTYPE, SPEC-IFACE, SPEC-FLOAT
- **Clauses:** SGO-LEX-001, SGO-LEX-002, SGO-LEX-003, SGO-LEX-004, SGO-BIND-001, SGO-BIND-002, SGO-BIND-003, SGO-BIND-004, SGO-BIND-005, SGO-TYPE-001, SGO-TYPE-002, SGO-TYPE-003, SGO-TYPE-004, SGO-TYPE-005, SGO-TYPE-006, SGO-TYPE-007, SGO-TYPE-008, SGO-VAL-001, SGO-VAL-002, SGO-VAL-003, SGO-VAL-004, SGO-VAL-005, SGO-CTL-001, SGO-CTL-002, SGO-CTL-003, SGO-CTL-004, SGO-COMP-001, SGO-COMP-002, SGO-COMP-003, SGO-COMP-004, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-COMP-008, SGO-COMP-009, SGO-UTYPE-001, SGO-UTYPE-002, SGO-IFACE-001, SGO-IFACE-002, SGO-IFACE-003, SGO-IFACE-004, SGO-FLOAT-001, SGO-FLOAT-002, SGO-FLOAT-003
- **Acceptance cases:** G02, G03, G07, S02, S03, S05, S09, S10, S16, S17, S19, X01, X02, X03, X04, X05, X06, X07, X08, X10, X11
- **Implementation:** `src/cmd/compile/internal/syntax/`, `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`, `src/cmd/compile/internal/types2/sgo_values.go`, `src/cmd/compile/internal/types2/sgo_local.go`, `src/cmd/compile/internal/types2/sgo_frozen.go`, `src/sgo/frozen.go`, `src/cmd/compile/internal/walk/sgo_map.go`, `src/runtime/sgo_map.go`, `src/runtime/sgo_copy.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`, `src/cmd/compile/internal/ssagen/sgo.go`, `src/cmd/compile/internal/walk/convert.go`, `src/cmd/compile/internal/ssa/func.go`

### G-DENY

float32, complex numbers, anonymous struct types, pointers to handles, interface forms outside SGO-IFACE-002..004 (interface literals, type sets, comparison of interface values, single-value assertions), generics, reflection, math/rand, unsafe, cgo, assembly, go:linkname, plugins and raw system calls are rejected at compile time with SGO_E_UNADMITTED; there is no flag, build mode or environment setting that admits them. (Value structs, arrays, and named types are admitted from spec 0.8.0, slices, maps, and pointers from 0.9.0, range over strings from 0.12.0, interfaces from 0.14.0, and float64 from 0.20.0: G-LANG.)

- **Scope:** admission, tested
- **Families:** SPEC-GENERIC, SPEC-REFLECT, SPEC-NATIVE, SPEC-RANDOM
- **Clauses:** SGO-GEN-001, SGO-REFLECT-001, SGO-NATIVE-001, SGO-RANDOM-001
- **Acceptance cases:** A01, D01, S10, S11, S12, S15, S17, X10
- **Implementation:** `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/sgo/build.go`

### G-MEM

Variables are task-local; a closure passed to another task may capture only effectively-final variables, so admitted programs have no data races on variables. Communication between tasks happens only through channels, permits, Event and CapacityLimiter, whose operations are synchronizing.

- **Scope:** admission, tested
- **Families:** SPEC-MEM
- **Clauses:** SGO-MEM-001, SGO-MEM-002, SGO-MEM-003, SGO-MEM-004
- **Acceptance cases:** O09, S16
- **Implementation:** `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`

### G-OWN

Every task is started in a nursery through its explicit *sgo.Nursery handle and is owned by exactly one nursery; a nursery statement does not complete until all its children have finished (no orphan on any exit, including return, break, panic and cancellation). There is no bare go statement. A value-returning child (sgo.Go, a Group of sgo.NewGroup(n, limit)) is such a child too; its handle always resolves: to the value and error it returns, to its cancellation or failure, or, when it was started into a cancelling nursery and never admitted, to that cancellation at once (SGO17-01). A group's limit bounds its running children, not its queued results.

- **Scope:** admission, runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model (model.go, explore.go): all interleavings of the 18 W01 scenarios (sgo/evidence/w01/README.md)
- **Families:** SPEC-NURSERY, SPEC-SPAWN, SPEC-WAIT
- **Clauses:** SGO-NUR-001, SGO-NUR-002, SGO-NUR-003, SGO-NUR-004, SGO-NUR-005, SGO-SPAWN-001, SGO-SPAWN-002, SGO-SPAWN-003, SGO-SPAWN-004, SGO-SPAWN-005, SGO-SPAWN-006, SGO-SPAWN-007, SGO-SPAWN-008, SGO-SPAWN-009, SGO-WAIT-001, SGO-WAIT-002, SGO-WAIT-003, SGO-WAIT-004
- **Acceptance cases:** C02, C03, C04, O01, O02, O03, O04, O05, S05, S06, S07, S08, X04
- **Implementation:** `src/runtime/sgo_nursery.go`, `src/runtime/sgo_scope.go`, `src/runtime/sgo_record.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/noder/sgo_main.go`, `src/sgo/task.go`, `src/sgo/group.go`, `src/runtime/sgo_taskres.go`, `src/runtime/sgo_group.go`

### G-FAIL

A child failure cancels its siblings and is reported to the owner; the primary error of a nursery is chosen deterministically (lowest creation id), other errors are chained. A panic unwinds defers and nested nurseries to the task boundary. The process exit status is specified (0 success; 1 an error or child error out of main; 2 any other failure kind such as a panic or checked failure; 3 drain budget exceeded; 4 resource exhaustion or runtime defect), and statuses 1-4 write exactly one diagnostic record. An error a child returns through sgo.Go or Group.Go is data, the result's error: it fails neither the nursery nor a sibling. A failure of such a child (a panic, a checked failure) is a child failure as above.

- **Scope:** runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model (model.go, explore.go): all interleavings of the 18 W01 scenarios (sgo/evidence/w01/README.md)
- **Families:** SPEC-DEFER, SPEC-ERRORS, SPEC-PROC
- **Clauses:** SGO-DEF-001, SGO-DEF-002, SGO-DEF-003, SGO-DEF-004, SGO-DEF-005, SGO-ERR-001, SGO-ERR-002, SGO-ERR-003, SGO-ERR-004, SGO-PROC-001, SGO-PROC-002, SGO-PROC-003, SGO-PROC-004, SGO-PROC-005
- **Acceptance cases:** O04, P01, P04, P05, P06, P08, S03, S06, S13, S16, S20, X09, X12
- **Implementation:** `src/runtime/sgo_nursery.go`, `src/runtime/sgo_main.go`, `src/runtime/panic.go`, `src/runtime/sgo_record.go`

### G-CANCEL

Cancellation is sticky and propagates from a nursery to its descendants; it is observed at the specified checkpoints (blocking operations, sleep, join, Checkpoint). A task that does not cooperate within the drain budget makes the process fail-stop with a diagnostic (exit 3) instead of resuming with the task hidden. Non-cooperative code between checkpoints is not interrupted. An error exit (a failing check, or a return with a non-nil error) cancels every nursery it crosses before joining it (D159). A deadline set on a nursery is inherited by its descendants and cannot be extended by them.

- **Scope:** runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model (model.go, explore.go): all interleavings of the 18 W01 scenarios (sgo/evidence/w01/README.md)
- **Families:** SPEC-CANCEL, SPEC-DRAIN, SPEC-TIMER
- **Clauses:** SGO-CANCEL-001, SGO-CANCEL-002, SGO-CANCEL-003, SGO-CANCEL-004, SGO-CANCEL-005, SGO-DRAIN-001, SGO-DRAIN-002, SGO-DRAIN-003, SGO-TIMER-001
- **Acceptance cases:** C01, C02, C03, C04, C07, C09, D06, P07, S17
- **Implementation:** `src/runtime/sgo_cancel.go`, `src/runtime/sgo_nursery.go`, `src/runtime/sgo_scope.go`, `src/runtime/sgo_deadline.go`, `src/runtime/sgo_timer.go`
- **Limitation:** A task in a loop without checkpoints is detected only by the drain budget, not interrupted (SGO-DRAIN-002).

### G-CHAN

Native channels, select, typed managed channels with send permits, readiness handshakes, Event and CapacityLimiter: each operation has a single commit point; a value is delivered at most once; a reserved permit is committed, aborted, or resolved when its task ends (no capacity leak); a blocked operation of a cancelled task unwinds; a global deadlock is reported, not hung.

- **Scope:** admission, runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model chanmodel/nativemodel: all interleavings of the bounded scenarios, largest 71,549 states (sgo/evidence/w06/README.md, w07/README.md)
- **Families:** SPEC-CHAN, SPEC-PERMIT, SPEC-READY, SPEC-SYNC
- **Clauses:** SGO-CHAN-001, SGO-CHAN-002, SGO-CHAN-003, SGO-CHAN-004, SGO-PERMIT-001, SGO-PERMIT-002, SGO-READY-001, SGO-READY-002, SGO-SYNC-001, SGO-SYNC-002
- **Acceptance cases:** H01, H02, H03, H04, H05, H06, H12, N01, N02, N03, N04, N05, N06, N07, N08, N09, R01, R02, R03, R04, S12, S16, S17
- **Implementation:** `src/runtime/sgo_chan.go`, `src/runtime/sgo_native.go`, `src/runtime/sgo_native_ops.go`, `src/runtime/sgo_ready.go`, `src/runtime/sgo_sync.go`, `src/cmd/compile/internal/walk/select.go`, `src/sgo/chan.go`, `src/sgo/ready.go`, `src/sgo/sync.go`

### G-AUTH

Effects (clock, wall clock, random, entropy, files, network, simulated store) require a capability that flows from the root grants declared in sgo.mod; a task's authority can only be attenuated, never widened; handles are opaque and cannot be compared or forged; a dependency cannot use an effect its package does not declare.

- **Scope:** admission, runtime_enforced, tested
- **Families:** SPEC-AUTH, SPEC-CAP
- **Clauses:** SGO-AUTH-001, SGO-AUTH-002, SGO-AUTH-003, SGO-CAP-001
- **Acceptance cases:** A01, A02, A03, A04, A05, A06, A07, A08, A09, A10, S06
- **Implementation:** `src/runtime/sgo_auth.go`, `src/runtime/sgo_effects.go`, `src/cmd/sgo/module.go`, `src/sgo/auth.go`, `src/sgo/effects.go`, `src/cmd/compile/internal/types2/sgo.go`

### G-IO

File access is confined beneath declared directory grants (no '..', absolute-path, prefix or symlink escape, re-verified against concurrent renames); network access is limited to declared TCP endpoints and Unix socket paths. Reads and writes report partial progress; cancelling a task that shares a handle leaves the handle usable by the rest of its scope; closed handles are not reused.

- **Scope:** admission, runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model iopath/iohandle: path resolution with renames (14 scenarios) and handle generations (sgo/evidence/w09/README.md)
- **Families:** SPEC-FILE, SPEC-NET
- **Clauses:** SGO-FILE-001, SGO-FILE-002, SGO-FILE-003, SGO-FILE-004, SGO-NET-001, SGO-NET-002, SGO-NET-003, SGO-NET-004
- **Acceptance cases:** I01, I02, I03, I04, I05
- **Implementation:** `src/runtime/sgo_io.go`, `src/runtime/sgo_iodir.go`, `src/runtime/sgo_fs.go`, `src/runtime/sgo_iogrant.go`, `src/runtime/sgo_ioapi.go`, `src/runtime/sgo_net.go`, `src/cmd/sgo/iogrant.go`, `src/sgo/io.go`
- **Limitation:** Confinement re-verifies the resolved chain; it is not atomic against another process renaming directories concurrently (openat2 RESOLVE_BENEATH is not used).

### G-RES

Every file, connection, listener, dialer, and registered user resource belongs to exactly one scope (a nursery or a task body) and is released exactly once: at an explicit Close, or when its scope ends, after all of the scope's tasks have ended, in reverse order of acquisition. A failed release is never dropped. A program in which a resource could outlive its scope, by assignment, return, capture, send on a longer-lived channel, or copy, does not compile; a resource leaves a nested scope only by an explicit move to an enclosing one. The one implicit move is a value-returning child's successful result: every resource of the child's body that the result holds, directly or in struct fields and array elements, moves into the child's nursery once, in acquisition order (SGO17-02).

- **Scope:** admission, runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model scoperes: 416 programs, 77,736 states, 9 seeded protocol bugs each caught (sgo/evidence/w21/README.md)
- **Families:** SPEC-RES
- **Clauses:** SGO-RES-001, SGO-RES-002, SGO-RES-003, SGO-RES-004, SGO-RES-005, SGO-RES-006
- **Acceptance cases:** 
- **Implementation:** `src/cmd/compile/internal/types2/sgo_resflow.go`, `src/runtime/sgo_res.go`, `src/runtime/sgo_nursery.go`, `src/runtime/sgo_scope.go`, `src/sgo/io.go`, `src/sgo/task.go`
- **Limitation:** The escape analysis is conservative and intraprocedural: it refuses some safe programs (a resource sent through a helper whose channel and value are separate parameters).
- **Limitation:** A user resource is identified by its pointer. Tasks sharing a scope share its user resources; the type serializes its own methods (D158), and the runtime adds no lock.

### G-LAB

A lab build (sgo lab) runs on a deterministic scheduler with virtual time and simulated effects: the same seed and manifest give the same canonical trace, a trace replays exactly, and a divergent or altered trace or a trace of another build is refused. Replay covers managed events (scheduling choices, timers, effect results), not instruction-level replay of arbitrary code. Lab and production artifacts cannot be mixed.

- **Scope:** admission, runtime_enforced, model_explored, tested
- **Model bounds:** sgo/model labmodel: pinned stream goldens and bounded scenarios (sgo/evidence/w08/README.md)
- **Families:** SPEC-LAB
- **Clauses:** SGO-LAB-001, SGO-LAB-002, SGO-LAB-003, SGO-LAB-004, SGO-LAB-005, SGO-LAB-006, SGO-LAB-007
- **Acceptance cases:** D02, D03, D04, D05, D09, S14
- **Implementation:** `src/runtime/sgo_lab.go`, `src/runtime/sgo_lab_run.go`, `src/runtime/sgo_lab_sched.go`, `src/runtime/sgo_lab_boot.go`, `src/runtime/sgo_iofake.go`, `src/runtime/sgo_netfake.go`, `src/cmd/compile/internal/walk/sgo_lab.go`, `src/cmd/sgo/lab.go`, `src/cmd/sgo/lab_cmds.go`

### G-TRUST

Only the trusted files pinned by SHA-256 in TARGET_TCB.json may use unrestricted Go; the driver verifies every trusted Go, assembly and header input before building, and a user package cannot claim a trusted path. Compiled packages carry the spec, admission and TCB identity; stale or foreign objects are refused at import and link. Task stacks are upstream Go's growable stacks (fixed virtual stacks were investigated in W11 and not adopted).

- **Scope:** admission, runtime_enforced, tested
- **Families:** SPEC-IMPORT, SPEC-TCB, SPEC-STACK
- **Clauses:** SGO-IMP-001, SGO-IMP-002, SGO-IMP-003, SGO-IMP-004, SGO-TCB-001, SGO-TCB-002, SGO-STACK-001
- **Acceptance cases:** F02, F03, F04, O07, O08, O10, S09, S10, S11, S13, S15, T01
- **Implementation:** `src/cmd/sgo/tcb.go`, `src/cmd/sgo/tcbasm.go`, `src/cmd/sgo/tcbtable.go`, `src/cmd/compile/internal/sgoadmit/identity.go`, `src/cmd/compile/internal/sgoadmit/tcbtable.go`, `src/cmd/compile/internal/noder/sgo_identity.go`, `src/cmd/link/internal/ld/sgoid.go`

### G-TOOL

The release is a single-language toolchain: sgo build/run/check/test/fmt/lab plus compile, asm and link, with no go command and no flag, mode or environment variable that reopens stock Go semantics. sgo check and sgo build agree; sgo test runs each test in its own process with its own root nursery and fails tests that leave work behind.

- **Scope:** admission, tested
- **Families:** SPEC-TOOL, SPEC-BUILD, SPEC-TEST
- **Clauses:** SGO-TOOL-001, SGO-TOOL-002, SGO-BUILD-001, SGO-TEST-001, SGO-TEST-002, SGO-TEST-003
- **Acceptance cases:** B04, B05, S01, S15, S18, S19, T01, T02, T03
- **Implementation:** `src/cmd/sgo/main.go`, `src/cmd/sgo/build.go`, `src/cmd/sgo/check.go`, `src/cmd/sgo/test.go`, `src/cmd/sgo/fmt.go`, `src/cmd/sgo/toolchain.go`, `sgo/scripts/make-release.py`

### G-STD

The standard library (std/errors, strings, bytes, strconv, utf8, sort, math; spec 0.13.0, with errors.Unwrap and sort.Sort from 0.14.0, io from 0.18.0, fmt and time from 0.19.0, and float64 in math, strconv, and fmt from 0.20.0) is S-Go source compiled under the same admission and sharing rules as user code, verified against its MANIFEST before use; it holds no package state and no capability. Each package exports exactly the declarations its SGO-STD clause lists, and each function with a Go counterpart gives that function's results on generated inputs, apart from the deviations its clause lists (no bitSize in ParseInt, ParseFloat, or FormatFloat; math's integer functions named IntAbs, IntMin, IntMax, IntPow; ASCII-only case mapping, error inputs quoted by QuoteToASCII). math's transcendental functions are Go's portable algorithms in S-Go source, never fused: their results are within 1 ulp and identical on every target, but not always correctly rounded. Every sort is stable and the same on every platform.

- **Scope:** admission, tested
- **Families:** SPEC-STD
- **Clauses:** SGO-STD-001, SGO-STD-002, SGO-STD-003, SGO-STD-004, SGO-STD-005, SGO-STD-006, SGO-STD-007, SGO-STD-008, SGO-STD-009, SGO-STD-010
- **Acceptance cases:** 
- **Implementation:** `src/sgostd/`, `sgo/scripts/std-api-check.py`, `sgo/scripts/std-manifest.py`, `sgo/scripts/test-std-diff.py`
- **Limitation:** Behavior equal to Go's packages is tested on seeded generated inputs (test-std-diff), not proven.
- **Limitation:** errors.Is and errors.As are not provided (D133): errors are distinguished by message or by unwrapping; a sort or search callback cannot capture task-local data (D94).

## Identity

- **upstream:** go1.27.3 (sgo/UPSTREAM.lock.json)
- **bootstrap:** go1.26.10, recorded by digest in sgo/scripts/fetch-bootstrap.sh
- **spec_version_file:** sgo/spec/VERSION
- **admission_digest_file:** sgo/SGO_SPEC_ADMISSION.json
- **tcb_manifest:** sgo/TARGET_TCB.json
- **stack implementation:** upstream growable stacks (SGO-STACK-001; W11 investigated_not_adopted, sgo/evidence/w11/README.md)
- **targets:** darwin/arm64, linux/amd64, linux/arm64

## Known limitations

- **L1** (T07): Benchmarks report wall, CPU and peak RSS but no allocation counts (T07 PARTIAL, D82).
- **L2** (F03 F04 F05): Fixed-stack experiment cases F03-F05 were not run because no prototype was built (W11 investigated_not_adopted).
- **L3** (T03): No language server; editors get sgo check and sgo fmt only (T03).
