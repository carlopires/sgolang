# W13 proposal — the core data model (draft for decisions D85–D92)

Status: **proposal**. Nothing here is admitted, specified, or implemented. It prepares the decisions
the owner must make before any clause is drafted. Plan rules still apply: the constitution
overrides this document, and every admitted construct needs clauses, a model where concurrency is
involved, conformance cases, and executed evidence (constitution §6).

## 1. Why this is next, and what it is not

Subset 0 (spec 0.7.0) admits integers, booleans, strings, functions and closures, nurseries and
tasks, channels and permits, `Ready`/`Event`/`Limiter`, effect handles, and file and network
handles. A program cannot hold a list or a record. Every later step (library modules, interfaces,
a standard library) is built on composite types, so they come first.

This proposal is **not** the standard library and **not** Go's type system feature by feature. It
asks one question first: **how can S-Go admit mutable aggregate data without readmitting data
races?** The answer (a *sharing rule*) shapes slices, maps, structs, and pointers alike.

## 2. Why subset 0 is race-free today

| Mechanism | Clause | What it rules out |
|---|---|---|
| Captured variables are effectively final | SGO-MEM-002 | a closure handed to another task cannot observe a write |
| Values cross tasks only through channels, permits, and join, which are happens-before edges | SGO-MEM-003, SGO-MEM-004, SGO-CHAN-*, SGO-PERMIT-* | unsynchronized communication |
| Every admitted value type is immutable (integers, booleans, strings, function values) | SGO-TYPE-*, SGO-VALUE-* | two tasks mutating one object |
| Handles are capabilities owned by a task, lent only to the owner's nurseries | SGO-CAP-001, SGO-FILE-003 | concurrent use of a stateful object outside the rules |

The third row carries the guarantee. Slices, structs with mutable fields, maps, and pointers break
it: once one task can write memory that another can read, SGO-MEM-003 ("no data races") needs a new
argument. The sharing rule (§5) is that argument.

## 3. Five programs S-Go should be able to express

These are written in a *hypothetical* S-Go: the syntax of today (nurseries, `sgo.Context`, handles)
plus Go-shaped composite types. They do not compile. Their job is to show which features are
needed, in which order, and where data crosses a task boundary (marked **⇄**).

### P1 — log aggregator (reads files, counts by key, prints a report)

```go
type Stat struct { Count int; Bytes int }

func main(root *sgo.Context) error {
	logs := root.Dir("logs")
	names, err := logs.List(".")              // today: *sgo.Listing
	if err != nil { return err }
	totals := map[string]Stat{}               // owned by the root task
	results := make(chan map[string]Stat)     // ⇄ each worker's partial map moves to the root
	nursery (n) {
		for i := 0; i < names.Len(); i++ {
			name := names.Name(i)
			n.Spawn(func(t *sgo.Context) error {
				data, err := logs.ReadFile(name)  // []byte: the worker's own buffer
				if err != nil { return err }
				part := map[string]Stat{}
				for _, line := range bytes.Split(data, '\n') {
					k := keyOf(line)
					s := part[k]; s.Count++; s.Bytes += len(line); part[k] = s
				}
				results <- part                   // ⇄ after this send the worker must not touch part
				return nil
			})
		}
		for i := 0; i < names.Len(); i++ { merge(totals, <-results) }
	}
	for _, k := range sortedKeys(totals) { root.Printf("%s %d %d\n", k, totals[k].Count, totals[k].Bytes) }
	return nil
}
```
Needs: structs, maps (deterministic iteration or sorted keys), `[]byte` and slicing, user
functions over slices, a way to **move** a map to another task.

### P2 — key-value server (TCP, one task per connection, one shared store)

```go
type Store struct { data map[string][]byte }

func serve(root *sgo.Context, ln *sgo.Listener) error {
	cmds := make(chan Request)                   // ⇄ requests go to the single store owner
	nursery (n) {
		n.Spawn(func(t *sgo.Context) error {         // the store task: the only one that touches data
			s := Store{data: map[string][]byte{}}
			for r := range cmds { r.Reply <- s.apply(r) }  // ⇄ the reply value goes back
			return nil
		})
		for {
			c, err := ln.Accept(root)
			if err != nil { return err }
			n.Spawn(func(t *sgo.Context) error { return session(t, c, cmds) })
		}
	}
}
```
Needs: structs with methods, maps of slices, a struct with a channel field (the reply), and
replies that carry **copies** (a `[]byte` value read by the session while the store keeps its own).

### P3 — job scheduler (priority queue, deadlines, workers)

```go
type Job struct { ID int; Prio int; Deadline sgo.Instant; Run func(*sgo.Context) error }
type Queue struct { items []Job }               // a heap owned by the dispatcher task

func (q *Queue) Push(j Job) { ... }             // pointer receiver: mutates q.items
func (q *Queue) Pop() Job   { ... }
```
Needs: methods with pointer receivers, slices that grow (`append`), structs holding function values,
a task that owns a mutable structure and hands **values** (`Job`) to workers.

### P4 — parser and evaluator (tree data, recursion, no concurrency)

```go
type Expr interface { Eval(env *Env) (int, error) }
type Num struct { V int }
type Add struct { L, R Expr }
type Env struct { vars map[string]int; parent *Env }
```
Needs: recursive types through pointers, interfaces (or a closed sum type), maps. No data crosses a
task: the test of whether ordinary single-task code is pleasant to write.

### P5 — fan-out over a shared, read-only table (configuration, a dictionary)

```go
table := loadTable(root)                         // built once by the root task
frozen := freeze(table)                          // ⇄ shared by every worker, read-only
nursery (n) {
	for w := 0; w < 8; w++ {
		n.Spawn(func(t *sgo.Context) error { return work(t, frozen) })
	}
}
```
Needs: sharing a large structure with many tasks **without copying it** and without locks: the
case that rules out "copy everything at a boundary" as the only rule.

## 4. What the programs need

| Feature | P1 | P2 | P3 | P4 | P5 | Family today |
|---|---|---|---|---|---|---|
| structs (value types) | ✓ | ✓ | ✓ | ✓ | ✓ | SPEC-COMPOSITE (denied) |
| arrays | | | | | | SPEC-COMPOSITE |
| slices, `append`, slicing, `[]byte` | ✓ | ✓ | ✓ | | ✓ | SPEC-COMPOSITE |
| string indexing, `range` over bytes | ✓ | ✓ | | ✓ | | SPEC-COMPOSITE |
| maps | ✓ | ✓ | | ✓ | ✓ | SPEC-COMPOSITE |
| named types and methods | ✓ | ✓ | ✓ | ✓ | | SPEC-USERTYPE (denied) |
| pointers / references | | ✓ | ✓ | ✓ | | SPEC-COMPOSITE |
| interfaces or sum types | | | | ✓ | | SPEC-IFACE (denied) |
| aggregate data crossing a task | move | copy | value | — | share read-only | **the sharing rule (§5)** |

Two observations. Every program needs structs, slices, and maps; only one needs interfaces, so they
can wait. And all three kinds of crossing appear: **move** (P1), **copy** (P2, P3), and **shared
read-only** (P5). A rule that offers only one of them forces the others into awkward code.

## 5. Candidate sharing rules

The rule decides what may happen when a mutable aggregate value is sent on a channel, committed
through a permit, returned from a child task, or captured by a closure handed to `Spawn`.

**A — Deep copy at every boundary.** Aggregates are values, and crossing a task copies them
(as Erlang does). Simple to explain and to check; no new compiler analysis beyond "the payload type
is copyable". It costs O(size) per crossing and cannot express P5 without copying the table into
every worker.

**B — Single owner, transfer by move.** Each mutable object has one owning task. Sending it moves
ownership, and the sender's later use of it is a compile error (affine types, as in Rust's `Send` by
value, or Pony's `iso`). It is zero-copy and handles P1. It needs flow analysis of every reference to
an aggregate (aliases through pointers, slices of slices, struct fields), which is the hardest part
to specify and to give good diagnostics for. P5 still needs something else.

**C — Freeze, then share.** A value is mutable only while one task holds it; `freeze` makes it
deeply immutable (a distinct type, like `frozen T`, or a runtime flag), and only frozen values may
be shared by many tasks. This handles P5 with zero copies and is easy to check at the type level.
Alone, it cannot express P1's "build here, hand over, keep mutating there".

**D — Go-style sharing plus synchronization, races caught at run time.** Rejected: SGO-MEM-003 is
a static guarantee, and the constitution forbids admitting a construct whose misuse is a data race
detected only sometimes.

**E — Task-local mutation; crossing by copy, by move, or frozen (recommended).** Combine A, B, and C
with simple, checkable rules:
1. Mutable aggregates (slices, maps, structs with pointers) are **task-local**: they can be captured
   only by closures that run in the creating task, never by a `Spawn` callback.
2. Crossing a task boundary takes one of three explicit forms:
   - **copy** (default for values without internal references, and explicit `sgo.Copy(x)` otherwise);
   - **move**, restricted to a whole local variable sent or committed directly (`ch <- move(part)`),
     after which the variable is dead: a much narrower analysis than general ownership, because only
     whole variables move and the compiler checks the variable, not every alias;
   - **frozen**, a deeply immutable value (`sgo.Freeze(x)`), which any number of tasks may read.
3. Reachability rule: a value that is moved must not be reachable from any other live variable of
   the sender. The first version enforces this conservatively: only values created in the sending
   function and never stored elsewhere may move; anything else must be copied or frozen.

| | A copy | B move | C freeze | E combined |
|---|---|---|---|---|
| Race freedom argument | trivial | needs alias analysis | type-level | per form; move kept narrow |
| P1 (hand over results) | copy cost | zero-copy | ✗ | move |
| P2, P3 (reply values) | ✓ | awkward | ✗ | copy |
| P5 (shared table) | copy per task | ✗ | zero-copy | freeze |
| Compiler work | small | large | medium | medium |
| Diagnostics quality | good | hard | good | good if move stays narrow |
| Lab determinism | unaffected | unaffected | unaffected | unaffected |
| Familiar to Go users | partly | no | partly | partly; explicit forms are new |

## 6. Recommendation

Adopt **E**, in stages, so each stage is admitted with its own evidence:

1. **Values only:** structs and arrays of admitted types (no pointers), copied wherever they go.
   Crossing a task copies them. No new sharing analysis: they are as safe as integers.
2. **Task-local mutable aggregates:** slices, `append`, `[]byte`, string indexing, maps with a
   specified iteration order, pointers to task-local data. The only cross-task form is **copy**.
   The compiler rejects capturing them in a `Spawn` callback and sending them uncopied.
3. **Freeze:** deeply immutable values shareable by any number of tasks (P5).
4. **Move:** the narrow whole-variable move (P1), added last and only if copy and freeze have proven
   insufficient in practice.
5. **Named types and methods** can proceed alongside stage 1; **interfaces** come after stage 2.

Stages 1–3 make all five programs expressible (P1 with a copy instead of a move). Each stage ends
with a spec version, as earlier work packages did.

## 7. Decisions for the owner (D85–D92)

Each is recorded in `sgo/SGO_DECISIONS.md` as PROPOSED with this recommendation.

| ID | Question | Recommendation |
|---|---|---|
| D85 | Is the core data model the next work package (W13), ahead of library modules and a standard library? | Yes. Standard library work waits for stages 1–3 and library modules |
| D86 | Which sharing rule? | E: task-local mutation; crossing by copy, frozen, and (later) a narrow move |
| D87 | Order of stages | 1 values (structs, arrays) → 2 task-local slices, maps, pointers (copy only) → 3 freeze → 4 move (optional); named types and methods with stage 1; interfaces after stage 2 |
| D88 | Map iteration order | Insertion order, specified and identical in production and the lab. Go's randomized order would make programs differ between runs and weaken replay; sorted order costs more and needs ordered keys |
| D89 | Copy semantics | Explicit `sgo.Copy` for values with internal references, implicit copy only for reference-free values; no copy constructors or custom hooks |
| D90 | What `freeze` produces | A distinct type (`sgo.Frozen[T]`) checked at compile time, not a run-time flag: misuse becomes a compile error, and the TCB does not grow |
| D91 | Bounds and nil | Index out of range, nil map write, and nil dereference are defined failures (`BoundsViolation` and siblings) recorded like other failure kinds, not Go panics with stack traces |
| D92 | Process | Each stage follows the earlier pattern: model first where concurrency is involved (the sharing rule gets a model check of copy, freeze, and move), drafts for approval, conformance cases with negative fixtures, a spec version, and reviewer verification on linux |

## 8. Work breakdown (W13)

| Step | Content | Evidence |
|---|---|---|
| 13.0 | The five programs as conformance fixtures marked pending; a model of the sharing rule (task-local, copy, freeze, move) explored like `chanmodel` and `iopath` | model scenarios; pending fixtures |
| 13.1 | Stage 1: structs and arrays of admitted types; named types and value-receiver methods | clauses (SPEC-COMPOSITE, SPEC-USERTYPE rows split: values first), fixtures, spec 0.8.0 |
| 13.2 | Stage 2: slices, `append`, `[]byte`, string indexing and byte range, maps (D88), pointers and pointer receivers, all task-local; copy at boundaries; capture and send checks | new diagnostics (`SGO_E_TASK_LOCAL_CAPTURE`, `SGO_E_UNCOPIED_SEND`), runtime bounds failures (D91), race lane, spec 0.9.0 |
| 13.3 | Stage 3: `sgo.Frozen[T]` and `sgo.Freeze` | P5 runs; negative fixtures for writes through a frozen value |
| 13.4 | Review and evidence | reviewer verification as in W12; the five programs pass (P1 with copy) |

Interfaces (SPEC-IFACE), library modules (a `require` in `sgo.mod` with cross-module contract
checks), and the standard library are follow-on work packages, each with its own decisions.

## 9. Risks

| Risk | Mitigation |
|---|---|
| The capture and send checks reject reasonable programs | the five programs are the acceptance bar; each rejection needs a clear diagnostic and a stated alternative (copy or freeze) |
| Copy costs make S-Go slow for data-heavy programs | measure with the D67 benchmark harness; freeze for read-mostly data; move as the planned remedy |
| Pointers reopen aliasing between tasks | pointers stay task-local in stage 2; the model checks that no task-local value is reachable from another task |
| Admitting Go's slice aliasing within one task surprises users | specify it exactly (one task, ordinary Go semantics); the hazard is only across tasks, which the rule removes |
| Scope creep toward "all of Go" | each stage admits only what its clauses name; everything else stays `SGO_E_UNADMITTED` |

## 10. What the owner needs to do now

Approve, change, or reject D85–D92. Nothing is implemented until D85–D87 are approved; D88–D92 can
be approved with them or when their stage starts.
