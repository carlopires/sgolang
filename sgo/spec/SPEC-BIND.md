# SPEC-BIND — Names, scopes, imports, initialization, and nursery bindings

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §2, decisions D07, D15, and resolution R3.

### SGO-BIND-001 — Packages and files
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LEX-001, SGO-PROC-001, SGO-IMP-001.
- **grammar_and_types:** Each source file begins with `package <identifier>`. All `.sgo` files in one directory form one package and must name the same package. The package named `main` is a program and must satisfy SGO-PROC-001; any other name is a user library package (SGO-IMP-001). The blank identifier `_` is not a valid package name.
- **preconditions:** SGO-LEX-001 holds for every file of the package.
- **evaluation_and_value_semantics:** n/a — static structure.
- **ownership_and_lifecycle:** n/a — static structure.
- **effect_and_capability_requirements:** n/a — compile-time only.
- **concurrency_and_memory_order:** n/a — compile-time only.
- **allowed_nondeterminism:** None. The result does not depend on file order within a directory.
- **failure_and_partial_progress:** Mismatched or missing package clauses are a compile error for the whole package.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_PACKAGE_CLAUSE`.
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S02; fixture:bind-two-files.
- **negative_tests:** S03; fixture:bind-package-mismatch.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`.
- **executed_evidence:** conformance fixtures bind-package-mismatch, bind-two-files (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-BIND-002 — Imports
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-ERR-001; D15.
- **grammar_and_types:** Import declarations follow the package clause: `import "path"` or a parenthesized list of quoted paths. Admitted paths: `"sgo"` (the canonical S-Go package) and import paths of user packages in the same module. Named (`import x "p"`), dot (`import . "p"`), and blank (`import _ "p"`) imports are denied. Each path appears at most once per file. The package name used in the file is the imported package's declared name.
- **preconditions:** The imported package exists in the module or is `sgo`, and was compiled under the same spec and admission digests (SGO-IMP-002).
- **evaluation_and_value_semantics:** n/a — static binding; subset 0 has no package initialization (SGO-BIND-003).
- **ownership_and_lifecycle:** n/a — static binding.
- **effect_and_capability_requirements:** Importing grants no capability. Capabilities come only from values (SGO-CAP-001).
- **concurrency_and_memory_order:** n/a — compile-time only.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A denied, unknown, duplicate, or unused import is a compile error. Import admission is checked before type checking: if any import of a package is denied, only the import diagnostics are reported for that package.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_UNADMITTED` (standard-library or other non-module path, import forms); `SGO_E_UNUSED` (unused import); `SGO_E_STALE_CONTRACT` (SGO-IMP-002).
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S09; fixture:bind-import-sgo, fixture:imp-user-package.
- **negative_tests:** S13, S17; fixture:bind-import-fmt, fixture:bind-import-alias, fixture:bind-unused-import.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`.
- **executed_evidence:** conformance fixtures bind-import-alias, bind-import-fmt, bind-import-sgo, bind-unused-import, imp-user-package (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-BIND-003 — Declarations
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-VAL-004, SGO-MEM-001, SGO-COMP-009, SGO-UTYPE-002; D07, D136; R3.
- **grammar_and_types:** Package level: `const` declarations (single or parenthesized, each with an explicit value; `iota` denied), `func` declarations with a body, type declarations as SGO-UTYPE-002 admits them, and package-level tables (D136): `var name = sgo.Freeze(lit)` or `var name sgo.Frozen[T] = sgo.Freeze(lit)`, one name per declaration, where `lit` is a composite literal whose elements and keys, at every depth, are constants or composite literals. Every other package-level `var`, and `func init`, are denied, as are functions without bodies. Function level: `const` as above; `var x T = e`, `var x = e`, `x := e`, and multi-name forms `a, b := e1, e2` or `a, b := f()` where the counts match. `var x T` without an initializer is denied. In `:=`, at least one name on the left must be new; a name that is not new must already be declared in the same block.
- **preconditions:** Initializer expressions are admitted (SGO-VAL-*) and assignable to the declared type (SGO-TYPE-002).
- **evaluation_and_value_semantics:** A local declaration evaluates its initializers left to right (SGO-VAL-005), then binds the names. A constant is a compile-time value. A package-level table is built once, before `main` or the first test, in source order; no initializer reads a variable, so the order cannot be observed. It is the same in production and the lab.
- **ownership_and_lifecycle:** A local variable lives for the activation of the function that declares it (and longer if captured, SGO-MEM-002).
- **effect_and_capability_requirements:** n/a — declarations have no effects beyond their initializer expressions.
- **concurrency_and_memory_order:** Locals belong to one task (SGO-MEM-001). A package-level table is frozen (SGO-COMP-009): every task may read it, and no task can write it. Assigning to it, incrementing it, ranging into it, or taking its address is denied, in its package and in every importer.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** If an initializer fails at run time (e.g. SGO-VAL-001 overflow), the declaration does not complete and the failure unwinds (SGO-DEF-003).
- **cancellation_and_cleanup:** n/a — declarations are not cancellation points.
- **compile_link_checks:** `SGO_E_UNADMITTED` (denied forms, including a package-level var that is not a table, a table element that is not a constant or composite literal, and a write to or address of a package-level table); `SGO_E_UNINITIALIZED`; `SGO_E_ASSIGN_COUNT`; `SGO_E_NO_NEW_VARIABLES`.
- **runtime_guards:** n/a — failures come from the initializer expressions.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S16; fixture:bind-declarations., fixture:tbl-basic, fixture:tbl-structs, fixture:tbl-imported
- **negative_tests:** S16, S17; fixture:bind-package-var, fixture:bind-init-func, fixture:bind-uninitialized-var, fixture:bind-iota., fixture:tbl-assign, fixture:tbl-addr, fixture:tbl-imported-assign, fixture:tbl-not-constant, fixture:tbl-reads-variable, fixture:tbl-two-names
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`; W26: `types2/sgo_tables.go` (`sgoPackageTables`, `sgoTableWrites`), `sgoadmit/admit.go` (`freezeOfLiteral`).
- **executed_evidence:** conformance fixtures bind-declarations, bind-init-func, bind-iota, bind-package-var, bind-uninitialized-var (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.; W26: conformance fixtures tbl-* (darwin/arm64; Linux at the W26 merge), `sgo/evidence/w26/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-BIND-004 — Scope and shadowing
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-BIND-003, SGO-NUR-001; R3.
- **grammar_and_types:** Scopes: the universe (predeclared identifiers `bool int string error true false nil len panic`), the package, each file (for imported package names), each function (parameters and results), and each block, including the implicit blocks of `if`, `for`, `switch`, and each `case`. A local name is in scope from the end of its declaration to the end of its block. Within one function, declaring a name already declared in an enclosing block of that function (including parameters and results) is a compile error. Declaring a name that equals a predeclared identifier, an imported package name, or a package-level declaration of the same package is a compile error. Every local variable declared in a function body must be used; assigning to it does not count as a use. Parameters and named results need not be used. The blank identifier `_` declares nothing.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** n/a — static rule.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** n/a — static rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Violations are compile errors.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_SHADOW`, `SGO_E_REDECLARED`, `SGO_E_UNDEFINED`, `SGO_E_UNUSED`.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** X02; fixture:bind-scopes.
- **negative_tests:** X02; fixture:bind-shadow-err, fixture:bind-shadow-predeclared, fixture:bind-unused-var.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`.
- **executed_evidence:** conformance fixtures bind-scopes, bind-shadow-err, bind-shadow-predeclared, bind-unused-var (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-BIND-005 — Exported names
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-BIND-003, SGO-IMP-002.
- **grammar_and_types:** In a user library package, a package-level `const` or `func` whose name starts with an upper-case letter (Unicode category Lu) is exported and may be referenced from importing packages as `pkg.Name`. Nothing else is accessible across packages. Package `main` exports nothing.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** n/a — static rule.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** n/a — static rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Referencing an unexported or missing name is a compile error.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_UNEXPORTED`, `SGO_E_UNDEFINED`. Exported declarations form the package's contract digest (SGO-IMP-002).
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S09; fixture:imp-user-package.
- **negative_tests:** S09; fixture:bind-unexported.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`.
- **executed_evidence:** conformance fixtures bind-unexported, imp-user-package (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
