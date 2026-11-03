# SPEC-CHAN — Channels and select

Denied in subsets 0 and 1 (spec 0.1.0 and 0.2.0, SGO-CHAN-001). Spec 0.3.0 admits native channels (SGO-CHAN-002), the cancellation unwind of blocked channel operations (SGO-CHAN-003), and `select` (SGO-CHAN-004), which supersede SGO-CHAN-001 (D31-D36).

### SGO-CHAN-001 — Channels and select are denied in subset 0
- **version:** 2
- **status:** superseded
- **reference_closure:** SGO-ERR-001, SGO-CHAN-002, SGO-CHAN-004; D06, D31; plan §7.
- **grammar_and_types:** Superseded in spec 0.3.0 by SGO-CHAN-002 and SGO-CHAN-004, which admit native channels and `select`. Still denied: `len` and `cap` of channels, nil channel values, channel element types other than those of SGO-CHAN-002, comparison of channel values, and every channel API outside SGO-CHAN-002, SGO-CHAN-004, and SGO-PERMIT-002 (including package `reflect` channel operations). No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED` or `SGO_E_TYPE_MISMATCH`, naming the construct and SGO-CHAN-002.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause; the admitted forms are tested under SGO-CHAN-002 and SGO-CHAN-004.
- **negative_tests:** S12; fixture:chan-type, fixture:chan-len-denied, fixture:chan-cap-denied, fixture:chan-compare-denied, fixture:chan-elem-denied.
- **implementation_coverage:** gate level W00; compiler-level denial W03 (`sgoadmit`), narrowed in W07 to the forms listed above (`sgoadmit/admit_expr.go`, `types2/sgo.go`, `types2/sgo_flow.go`).
- **executed_evidence:** `sgo/evidence/w03/README.md` (subset 0 denial); `sgo/evidence/w07/README.md` (0.3.0 denials).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CHAN-002 — Native channels (subset 2)
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-005, SGO-BIND-003, SGO-CHAN-003, SGO-CHAN-004, SGO-PERMIT-002, SGO-DEF-003, SGO-CANCEL-002, SGO-SPAWN-004, SGO-COMP-002; D31, D32, D35, D36; plan §7.
- **grammar_and_types:** Channel types `chan T`, `<-chan T` (receive-only), and `chan<- T` (send-only), where the element type T is `int`, `bool`, `string`, `error`, a value type (SGO-COMP-002, spec 0.8.0), a shareable or task-local type (SGO-COMP-004, spec 0.9.0; a task-local value is sent only as the result of `sgo.Copy`), or a channel type whose element type is one of these, recursively (D36). A bidirectional channel is assignable to either directional type. `make(chan T)` and `make(chan T, n)` (n of type int) are the only constructors. The send statement `ch <- v`, the receive operation `<-ch` and `v, ok := <-ch` (also `=`), `close(ch)`, and `for v := range ch` / `for range ch` are admitted with Go's typing rules: a receive-only channel cannot be sent to or closed, a send-only channel cannot be received from. Channel values are never nil: there is no nil value of a channel type, every variable is initialized (SGO-BIND-003), and a named result of channel type is a compile error. `len`, `cap`, and `==`/`!=` of channels are denied. A range clause whose operand is not a channel stays denied (SGO-COMP-001).
- **preconditions:** `n >= 0` for `make(chan T, n)`. A channel value may be held by any number of tasks; it is synchronized and crosses task boundaries like a handle.
- **evaluation_and_value_semantics:** A channel of capacity C holds at most C values in FIFO order. `ch <- v` evaluates `ch` then `v`, then publishes v exactly once: to a waiting receiver, else into free buffer space, else it waits; it never publishes after its task observed cancellation. `<-ch` returns the oldest value (a waiting sender's value moves into the freed buffer slot in sending order) or, for C = 0, the value of a waiting sender; on a closed and empty channel it returns the zero value of T and `ok = false`, without waiting. `close(ch)` marks the channel closed; closing a closed channel has no effect (unlike Go). After close, values already buffered are still received. Each step of `range` is a receive; the loop ends when the receive reports `ok = false`.
- **ownership_and_lifecycle:** A channel belongs to no task or nursery; its buffered values live until received or until the channel is unreachable. A sender's value is copied when the send starts, so no stack memory of the sender or receiver is referenced while either waits.
- **effect_and_capability_requirements:** None: channels are created by `make` and need no capability. Channel values are not capability handles and carry no authority (handles are not admitted as elements, D36).
- **concurrency_and_memory_order:** A send happens before the receive that returns its value completes. For C = 0, the receive also happens before the send completes. `close` happens before every receive that reports `ok = false` because of it. The race detector observes these edges.
- **allowed_nondeterminism:** Which of several waiting senders or receivers proceeds first, and the interleaving of operations of different tasks; every history is one admitted by the model `sgo/model/nativemodel.go` (no lost, duplicated, or reordered value; buffer bounds; one arm per select).
- **failure_and_partial_progress:** A send on a closed channel fails the sending task with `ClosedChannel` (SGO-DEF-003) and has no effect; a send waiting when the channel is closed fails the same way. `make(chan T, n)` with `n < 0` fails with `InvalidArgument`. Neither failure affects values already published.
- **cancellation_and_cleanup:** A send, receive, or range step that can proceed without waiting does not observe cancellation. One that must wait while the task is cancelled ends with no effect and unwinds the task (SGO-CHAN-003).
- **compile_link_checks:** `SGO_E_UNADMITTED` for denied element types, `len`/`cap`, comparison, and non-channel range; `SGO_E_TYPE_MISMATCH` for nil of a channel type; `SGO_E_UNINITIALIZED` for named results of channel type. Code of S-Go packages calls only the S-Go channel entry points (`runtime.sgomakechan`, `sgochansend1`, `sgochanrecv1`, `sgochanrecv2`, `sgoclosechan`); the compiler fails with an internal error if Go's channel entry points (`chansend1`, `chanrecv1`, `chanrecv2`, `closechan`, `makechan`, `chanlen`, `chancap`, `selectgo`, ...) are requested for an S-Go package, so no raw Go channel path exists.
- **runtime_guards:** Channel invariants under the channel lock (no negative reservation, queued values within capacity, no buffered value at capacity 0) are runtime defects if violated (SGO-ERR-003).
- **tcb_assumptions:** The managed-channel primitive TCB-P12 (`src/runtime/sgo_chan.go`) with native operations TCB-P17 (`src/runtime/sgo_native.go`, `sgo_native_ops.go`); park/unpark and the cancellation registry (TCB-P02, TCB-P04).
- **positive_tests:** H01, H02, H11, H12, N04, N05, N07, N09; fixture:native-buffered-fifo, fixture:native-rendezvous, fixture:native-close-drain, fixture:native-range, fixture:native-directional, fixture:native-chan-of-chan, fixture:native-close-idempotent; the model's bounded histories (`sgo/model/nativemodel.go`) checked against the runtime (TestSGoNativeOutcomes), fixture:comp-copy-send, fixture:comp-array-chan-elem.
- **negative_tests:** S12; fixture:native-send-closed, fixture:native-negative-capacity, fixture:chan-type, fixture:chan-len-denied, fixture:chan-cap-denied, fixture:chan-compare-denied, fixture:chan-elem-denied, fixture:chan-named-result-denied, fixture:chan-recv-only-close, fixture:ctl-range, fixture:comp-uncopied-send, fixture:comp-uncopied-select, fixture:comp-nil-chan-denied.
- **implementation_coverage:** W07: model `sgo/model/nativemodel.go`; runtime `src/runtime/sgo_native.go`, `sgo_native_ops.go`; lowering `typecheck/syms.go` (`sgoRuntimeName`); admission `sgoadmit/admit_expr.go`; type rules `types2/sgo.go`, `types2/sgo_flow.go`.
- **executed_evidence:** `sgo/evidence/w07/README.md` (model lane; runtime lane against the model's outcome sets with GC and stack movement, select stress, race gate; native-* and chan-*-denied conformance cases; objdump check of compiled S-Go code).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CHAN-003 — Cancellation unwind of blocked operations, and deadlock (subset 2)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CHAN-002, SGO-CHAN-004, SGO-CANCEL-001, SGO-CANCEL-002, SGO-SPAWN-004, SGO-DEF-004, SGO-DEF-005, SGO-NUR-004, SGO-PROC-002; D33.
- **grammar_and_types:** n/a — a runtime rule of the operations of SGO-CHAN-002 and SGO-CHAN-004; it adds no syntax.
- **preconditions:** A send, receive, range step, or `select` without `default` must wait (it cannot proceed now) while a scope linked to the task is CANCELLING (SGO-CANCEL-002), or becomes so while it waits.
- **evaluation_and_value_semantics:** The operation ends with no effect (no value published or consumed, every registration removed) and the task unwinds from it: deferred calls run, and a nursery whose body the unwind leaves is cancelled and joined first (SGO-DEF-005). The task ends **Cancelled**, as if it had returned its cancellation error; it does not cancel the scope it belongs to (SGO-SPAWN-004). If a child of a nursery joined during the unwind ended Failed or Errored, that failure is primary and the task ends Failed (SGO-NUR-004 rules 2 and 3); a failure raised by a deferred call during the unwind is the task's failure (SGO-DEF-004). The root task unwinding this way ends the program with `MainError` and status 1, as `main` returning the cancellation error would.
- **ownership_and_lifecycle:** The internal unwind signal is not a value of the program; `recover` is not part of S-Go (SGO-DEF-002), so it cannot be caught, and cancellation stays requested (sticky).
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** The decision between completion and cancellation is one atomic step per operation: a waiting operation that a partner completed is complete even if cancellation follows; one that cancellation claimed has no effect even if a partner arrives later.
- **allowed_nondeterminism:** Whether a waiting operation completes or unwinds when its partner and the cancellation race; both outcomes are in the model's histories.
- **failure_and_partial_progress:** **Deadlock:** if every task waits and none can be woken (no partner, no timer, no cancellation), the program ends with one diagnostic record of kind `Deadlock` and status 2; no deferred call runs.
- **cancellation_and_cleanup:** As above; operations that can proceed without waiting never observe cancellation.
- **compile_link_checks:** n/a — a runtime rule.
- **runtime_guards:** The signal is raised only for a cancelled task (else a runtime defect); a task never finishes with an open nursery (SGO-ERR-003).
- **tcb_assumptions:** Unwinding with nursery joins (TCB-P05, `src/runtime/sgo_nursery.go`), the cancellation registry (TCB-P04), and the deadlock check of the scheduler (`checkdead`, TCB-P02).
- **positive_tests:** H06, N03, N08; fixture:native-cancel-unwind, fixture:native-cancel-joins-children, fixture:native-cancel-child-failure-primary, fixture:native-range-cancel; TestSGoNativeCancelUnwind.
- **negative_tests:** fixture:ctl-select (deadlock), fixture:native-deadlock.
- **implementation_coverage:** W07: `src/runtime/sgo_native.go` (`sgoCancelUnwind`), `sgo_nursery.go` (`sgoUnwindNurseries`), `sgo_scope.go` (`sgoTaskMain`), `sgo_main.go` (`sgoUnwoundOut`, `sgoThrowExit`), `panic.go` (`fatal`).
- **executed_evidence:** `sgo/evidence/w07/README.md` (TestSGoNativeCancelUnwind; native-cancel-* and native-deadlock conformance cases).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CHAN-004 — select (subset 2)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CHAN-002, SGO-CHAN-003; D34.
- **grammar_and_types:** `select { case ch <- v: ... case x := <-ch: ... case x, ok := <-ch: ... case <-ch: ... default: ... }` with Go's syntax and typing; at most one `default`; arms may name the same channel more than once. `select {}` is admitted.
- **preconditions:** As SGO-CHAN-002 for each arm.
- **evaluation_and_value_semantics:** All channel operands and send values are evaluated exactly once, in source order, before any arm is chosen. If one or more arms can proceed, one of them is chosen uniformly at random and takes effect; exactly one arm takes effect per `select`, and a send arm that is not chosen leaves no reservation and publishes nothing. With `default` and no arm ready, `default` runs. Without `default` and no arm ready, the `select` waits until an arm can proceed, then exactly one arm takes effect. The receive variables of the chosen arm are assigned after it takes effect. A send arm chosen on a closed channel fails as SGO-CHAN-002.
- **ownership_and_lifecycle:** No arm of a `select` stays registered on any channel once the `select` completes or unwinds.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** The chosen arm has the ordering of its operation (SGO-CHAN-002).
- **allowed_nondeterminism:** The choice among ready arms (the program cannot observe the generator; production promises no reproducible choice); interleavings as SGO-CHAN-002.
- **failure_and_partial_progress:** As the chosen arm's operation.
- **cancellation_and_cleanup:** A `select` with `default` never waits and never observes cancellation. A `select` without `default` that waits is a cancellation point with the unwind of SGO-CHAN-003; `select {}` waits only for cancellation.
- **compile_link_checks:** Lowered to `runtime.sgoselectgo`, `sgoselectnbsend`, `sgoselectnbrecv`, `sgoblock`, or the single-operation entry points of SGO-CHAN-002 (SGO-CHAN-002 compile checks).
- **runtime_guards:** One terminal decision per waiting `select` (one atomic transition across all arms); channel locks taken in address order with duplicates merged; a woken `select` without a winning arm is a runtime defect.
- **tcb_assumptions:** TCB-P17 (`src/runtime/sgo_native_ops.go`), TCB-P12, TCB-P02, TCB-P04.
- **positive_tests:** N01, N02, N03, N06; fixture:chan-select, fixture:select-ready-arms, fixture:select-default, fixture:select-eval-order, fixture:select-duplicate-arms, fixture:select-send-arm; TestSGoNativeSelectStress.
- **negative_tests:** fixture:ctl-select, fixture:select-send-closed.
- **implementation_coverage:** W07: `src/runtime/sgo_native_ops.go`; lowering through Go's select walk (`walk/select.go`) with the S-Go entry points (`typecheck/syms.go`).
- **executed_evidence:** `sgo/evidence/w07/README.md` (TestSGoNativeSelectStress; select-* conformance cases; race gate).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
