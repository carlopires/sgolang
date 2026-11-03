# W13 acceptance programs

The five programs of `sgo/docs/w13-data-model-plan.md` §3, written against the data model approved
in D85–D92. They are the acceptance bar for W13 (D92): each becomes a conformance case (`kind: run`,
with the `stdout` below) in the stage that admits the last construct it needs, and must then pass.
Until then they are **not** conformance cases and are not compiled: they use constructs that
spec 0.7.0 denies (`SGO_E_UNADMITTED`), and `sgo.Copy` / `sgo.Freeze` / `sgo.Frozen[T]` do not exist.

| Program | Needs (stage) | Crossing | Expected stdout |
|---|---|---|---|
| `p1-aggregate` | structs, slices, maps (2) | `sgo.Copy` of each worker's partial map | `a 3 9\nb 2 6\nc 1 3\n` |
| `p2-store` | structs, methods, maps of slices (2) | requests are values; replies are `sgo.Copy` | `get k1 v1\nget k2 missing\n` |
| `p3-scheduler` | structs, pointer-receiver methods, `append` (2) | jobs are reference-free values | `ran 3\nran 2\nran 1\n` |
| `p4-eval` | structs, pointers, maps, a closed sum by tag (2) | none (one task) | `14\n` |
| `p5-table` | structs, maps, frozen values (3) | one `sgo.Frozen` table read by four workers | `sum 40\n` |

The model of the sharing rule these programs rely on is `sgo/model/sharemodel.go`
(scenarios `p1-copy` … `p5-frozen-table` in `sharemodel_test.go`).

## Status (spec 0.9.0)

P1–P4 compile and pass as conformance cases `w13-p1-aggregate` … `w13-p4-eval`. Changes from the
drafts, all forced by rules that hold in every stage: rune literals (`'0'`, `'v'`) and `iota` are
denied (SGO-LEX-004, SGO-VAL-004), so the programs use `48`, `118`, and explicit constants; `var b
[]byte` needs an initializer (SGO-BIND-003); P1 uses value-array inputs (D97); P2 copies at the send
(`r.Reply <- sgo.Copy(s.Get(k))`: a task-local value crosses only as an `sgo.Copy` result,
SGO-COMP-004); P3's worker writes through the root's `*sgo.Writer` (stdout is a root grant,
SGO-PROC-003). P5 needs frozen values (stage 3).
