# Intentional rejection report (D68, T05), spec 0.6.0

`sgo check` over the 356 top-level upstream language tests (`test/*.go`, each copied as the
`main.sgo` of a scratch module). The untouched upstream suite runs only with the reference toolchain
(B03); no upstream test is changed or moved into a target lane.

Admitted: 12. Admitted with an expected output and run by S-Go: 0, mismatches: 0.

First rejection of each file, by code (clause named in the diagnostic):

| code | files | clauses |
|---|---|---|
| SGO_E_UNADMITTED | 314 | SGO-BIND-002 (121), SGO-NATIVE-001 (62), SGO-BIND-003 (46), SGO-UTYPE-001 (40) |
| SGO_E_SYNTAX | 13 | - (13) |
| SGO_E_MAIN_SIGNATURE | 12 | SGO-PROC-001 (12) |
| admitted | 12 |  |
| SGO_E_UNSTRUCTURED_SPAWN | 3 | SGO-SPAWN-005 (3) |
| SGO_E_SHADOW | 1 | SGO-BIND-004 (1) |
| SGO_E_SOURCE_ENCODING | 1 | SGO-LEX-001 (1) |

Admitted files: alias3, asmhdr, closure5, ddd2, empty, eof1, import2, import4, linkname, method4, retjmp, uintptrescapes. Of these, 10 are `package ignored` stubs of
directory tests (their code is in `<name>.dir/`, which this report does not cover); the rest declare
no code that needs admitting. Scope: top-level `test/*.go` only; subdirectories and `.dir` trees
are not included.

Per-file outcomes: `rejection-report.json`.
