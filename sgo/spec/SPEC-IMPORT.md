# SPEC-IMPORT — Transitive package/body/initialization contracts and artifact identity

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §14, decisions D15, D103–D105, D109–D115, plan §1.3; required modules and the standard library since 0.11.0 (W14).

### SGO-IMP-001 — Packages and modules
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-BIND-001, SGO-BIND-002, SGO-BIND-005, SGO-TOOL-001, SGO-IMP-003, SGO-IMP-004; D15, D38, D103.
- **grammar_and_types:** A program is built from its *main module* (the module containing the package being built), the modules it requires, transitively (SGO-IMP-003), and the standard library (SGO-IMP-004). A module is a directory tree whose root contains an `sgo.mod` declaring `module <path>` and `sgo <spec-version>`; package import paths are `<module path>/<relative directory>`, and `std/<relative directory>` for the standard library. A module path may not equal, contain, or lie under the import path of a target TCB package (`sgo`, `runtime`, `internal/...`, D38) or `std` (D103). An import resolves to the module whose path is the longest prefix of the import path among the main module, the required modules, and `std`; an import that resolves to none is `SGO_E_UNDEFINED`. Import cycles, within or across modules, are a compile error, reported once at the import declaration in the cycle's lexically first package (by import path) that names the next package of the cycle. Every package of every module, including `std/`, is compiled under the same admission and sharing rules; there is no other mode.
- **preconditions:** Every module's `sgo.mod` names the spec version the toolchain implements; otherwise the build fails.
- **evaluation_and_value_semantics:** n/a — build structure; there is no package initialization.
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** Packages grant no authority (SGO-CAP-001); a dependency's grants are bounded by the main module's (SGO-IMP-003).
- **concurrency_and_memory_order:** n/a — build structure.
- **allowed_nondeterminism:** None: the build result does not depend on directory listing order.
- **failure_and_partial_progress:** Any package failing admission fails the whole build; no partial program is linked.
- **cancellation_and_cleanup:** n/a — build structure.
- **compile_link_checks:** `SGO_E_USAGE` at the module directive for a module path that collides with a TCB package path or `std`; `SGO_E_IMPORT_CYCLE`; `SGO_E_SPEC_VERSION` when any module's `sgo` directive differs from the toolchain's spec version; `SGO_E_UNDEFINED` for an import that resolves to no module or to a missing package.
- **runtime_guards:** n/a — build structure.
- **tcb_assumptions:** n/a — toolchain only.
- **positive_tests:** S09; fixture:imp-user-package, fixture:mod-require-local, fixture:mod-require-vendor, fixture:mod-require-transitive, fixture:std-import.
- **negative_tests:** S09, S15; fixture:imp-cycle, fixture:imp-spec-version-mismatch, fixture:tcb-module-named-sgo, fixture:mod-cycle-across, fixture:mod-path-under-std, fixture:mod-import-unrequired, fixture:mod-dependency-spec-version.
- **implementation_coverage:** W03, W14: `src/cmd/sgo/module.go`, `src/cmd/sgo/modgraph.go`, `src/cmd/sgo/tcb.go`, `src/cmd/sgo/build.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/noder/sgo_identity.go`.
- **executed_evidence:** conformance fixtures above (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`, `sgo/evidence/w14/README.md`.
- **supported_execution_configurations:** production, lab.

### SGO-IMP-002 — Contract identity of compiled packages
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-IMP-003, SGO-IMP-004, SGO-BIND-005, SGO-TOOL-001; plan §1.3; SPEC-1, SPEC-3.
- **grammar_and_types:** n/a — artifact rule.
- **preconditions:** n/a — artifact rule.
- **evaluation_and_value_semantics:** Each compiled package — of the main module, a required module (SGO-IMP-003), or `std/` (SGO-IMP-004) — records in its export data and object file: the spec version, the admission digest, the package's contract digest (SHA-256 over its exported declarations and their types), the contract digests of every package it imports, and the S-Go ABI identity. The linker and the importing compiler verify all of them, across module boundaries exactly as within one module. The driver compiles every user and `std/` package of a build afresh; only TCB packages are cached, keyed by these identities.
- **ownership_and_lifecycle:** n/a — artifact rule.
- **effect_and_capability_requirements:** n/a — artifact rule.
- **concurrency_and_memory_order:** n/a — artifact rule.
- **allowed_nondeterminism:** None: digests are deterministic functions of their inputs.
- **failure_and_partial_progress:** Any mismatch, or an object file without S-Go identity (including every stock Go object), fails the build.
- **cancellation_and_cleanup:** n/a — artifact rule.
- **compile_link_checks:** `SGO_E_STALE_CONTRACT` (dependency contract digest changed), `SGO_E_SPEC_MISMATCH` (spec or admission digest differs), `SGO_E_FOREIGN_OBJECT` (no S-Go identity), for packages of every module. Build-cache keys include all of these identities.
- **runtime_guards:** n/a — link time.
- **tcb_assumptions:** n/a — toolchain only.
- **positive_tests:** S09; fixture:imp-user-package, fixture:mod-require-local (a required package contract checked at import).
- **negative_tests:** S09, S15, S18; fixture:imp-stale-contract (a rebuild after a dependency's exported signature changes must type-check against the new contract, never reuse the stale one), fixture:imp-stock-object (a stock Go object as build input is rejected), fixture:mod-stale-dependency-contract (the same across a module boundary); link-level `SGO_E_STALE_CONTRACT` and `SGO_E_FOREIGN_OBJECT` are compiler/linker unit tests (W03).
- **implementation_coverage:** W03, W14: `src/cmd/compile/internal/noder/sgo_identity.go`, `src/cmd/compile/internal/sgoadmit/identity.go`, `src/cmd/link/internal/ld/sgoid.go`, `src/cmd/sgo/build.go`, `src/cmd/sgo/identity.go`.
- **executed_evidence:** `sgo/scripts/test-identity.py` (stale contract, foreign export data and object, spec mismatch); conformance fixtures imp-stale-contract, imp-stock-object, mod-stale-dependency-contract (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`, `sgo/evidence/w14/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-IMP-003 — Required modules, replacements, and sgo.sum
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-AUTH-002, SGO-TOOL-001; D104, D105, D111–D114.
- **grammar_and_types:** `sgo.mod` may contain, besides `module`, `sgo`, and `grant`: lines `require <module path> <version>` with `<version>` of the form `vMAJOR.MINOR.PATCH` (each path at most once per file), and, in the main module only, `replace <module path> => <dir>` with `<dir>` a relative directory (each path at most once; never the main module's own path or a path under `std`). A required module's source is the replacement directory if the main module replaces it, else `vendor/<module path>/` under the main module root; that directory's `sgo.mod` must declare exactly the required path. `replace` lines of required modules are ignored (D112); their `require` lines are followed transitively. `sgo.sum`, next to the main module's `sgo.mod`, holds one line `<module path> <version> h1:<hex>` per required module, sorted by path, where `<hex>` is the SHA-256 of the module's file tree: the sorted list of its `sgo.mod` and `.sgo` files (including `_test.sgo`; excluding nested modules, `vendor`, `testdata`, and names starting with `.` or `_`), each as `<relative slash path>\0<length>\0<bytes>`.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** Every module path appears at most once in a build (D111): if two modules of the build require different versions of one path, the build fails. A required module is compiled with its own `sgo.mod` (its `sgo` directive must equal the toolchain's spec version) but is bounded by the main module's grants (D105): each `grant <kind>` of a dependency must also be granted by the main module, and a dependency may not contain file or network grants (D114). `sgo build`, `sgo check`, `sgo test`, `sgo run`, and `sgo lab` read `sgo.sum` and never write it; `sgo mod sum` computes and writes it; `sgo mod verify` recomputes every entry and reports each mismatch (D113). Diagnostics in a required module name its files as `<module path>/<relative path>`.
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** A dependency's grants never extend the program's ceiling: the root task's ceiling stays exactly the main module's grants (SGO-AUTH-002).
- **concurrency_and_memory_order:** n/a — build structure.
- **allowed_nondeterminism:** None: `sgo.sum` and its digests are a function of the file contents.
- **failure_and_partial_progress:** Any violation fails the build before any package is compiled; no object is produced.
- **cancellation_and_cleanup:** n/a — build structure.
- **compile_link_checks:** `SGO_E_MODULE_MISSING` (no replacement and no vendor directory, or the directory's `sgo.mod` names another path), `SGO_E_MODULE_SUM` (no `sgo.sum` entry, or the digest differs; the message names `sgo mod sum`), `SGO_E_MODULE_VERSION` (two versions of one path, or a malformed version), `SGO_E_DEPENDENCY_GRANT` (a dependency grant the main module lacks, or a file or network grant in a dependency), `SGO_E_USAGE` (duplicate or malformed `require`/`replace`; a `require` or `replace` of the main module's own path or of `std`).
- **runtime_guards:** n/a — build time.
- **tcb_assumptions:** The driver computes digests and resolves directories; it is part of the toolchain's trusted build path, as for single-module builds.
- **positive_tests:** fixture:mod-require-local, fixture:mod-require-vendor, fixture:mod-require-transitive, fixture:mod-dependency-grant-ok; `sgo/scripts/test-modules.py` (`sgo mod sum` writes exactly the recorded file, `sgo mod verify` accepts it).
- **negative_tests:** fixture:mod-missing, fixture:mod-wrong-path, fixture:mod-sum-missing, fixture:mod-sum-mismatch, fixture:mod-version-conflict, fixture:mod-version-malformed, fixture:mod-dependency-grant-exceeds, fixture:mod-dependency-io-grant, fixture:mod-replace-twice, fixture:mod-replace-main; `sgo/scripts/test-modules.py` (no build writes `sgo.sum`; verify and build refuse a vendored edit). Mutants: skipping the `sgo.sum` check fails mod-sum-mismatch and mod-sum-missing; skipping the grant bound fails mod-dependency-grant-exceeds.
- **implementation_coverage:** W14: `src/cmd/sgo/module.go`, `src/cmd/sgo/modgraph.go`, `src/cmd/sgo/modcmd.go`, `src/cmd/sgo/main.go`, `src/cmd/sgo/build.go`, `src/cmd/sgo/check.go`.
- **executed_evidence:** conformance fixtures above and `sgo/scripts/test-modules.py` (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w14/README.md`.
- **supported_execution_configurations:** production, lab.

### SGO-IMP-004 — The standard library
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-BIND-003, SGO-TCB-002; D103, D109, D115.
- **grammar_and_types:** The standard library is the implicit module `std`, always present in every build: its packages are imported as `std/<dir>` and their sources are the `.sgo` files of `$GOROOT/src/sgostd/<dir>` (D103; not `src/std/`, a reserved pattern of the go command). It is never required, replaced, or vendored. A `std/` package may import only other `std/` packages and `sgo` (D115). Its packages are written in S-Go under the same admission and sharing rules as user code: in particular they have no package-level variables (SGO-BIND-003) and so no package state (D109); they hold no capability except those passed to them as arguments.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** `std/` packages behave exactly as user packages with the same source would. `$GOROOT/src/sgostd/MANIFEST` lists every non-test `.sgo` file of the standard library with its SHA-256; the driver refuses to compile a `std/` package any of whose files differs from, is missing from, or is absent from the manifest (the release archive ships both), so a modified installation is detected rather than trusted. Diagnostics in `std/` name its files as `std/<relative path>`.
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** None of its own: `std/` is not part of the TCB (D103) and has no grants.
- **concurrency_and_memory_order:** As for user code (SGO-MEM-003); no package state exists to share.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A violation fails the build; no object is produced.
- **cancellation_and_cleanup:** n/a — build structure.
- **compile_link_checks:** `SGO_E_UNDEFINED` (no such `std/` package), `SGO_E_STD_MANIFEST` (a `std/` file differs from, is missing from, or is not listed in the manifest), `SGO_E_UNADMITTED` (a `std/` package importing anything but `std/` packages and `sgo`, checked by the driver and again by compiler admission), and every admission diagnostic for `std/` code as for user code.
- **runtime_guards:** n/a — build time.
- **tcb_assumptions:** None beyond the driver's (SGO-IMP-003).
- **positive_tests:** fixture:std-import (`std/strconv` end to end); `sgo/scripts/test-std.py` (`sgo test` of every `std/` package, D110).
- **negative_tests:** fixture:std-missing-package, fixture:mod-path-under-std; `sgo/scripts/test-modules.py` on a copied installation: an edited, an added, and a removed `std/` file are refused with `SGO_E_STD_MANIFEST`; `std/` packages importing a Go standard-library package or a module package, or using a denied construct, are refused. Mutant: skipping the manifest check fails test-modules.
- **implementation_coverage:** W14: `src/cmd/sgo/module.go`, `src/cmd/sgo/modgraph.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/noder/noder.go`, `src/sgostd/`, `sgo/scripts/std-manifest.py`, `sgo/scripts/make-release.py`.
- **executed_evidence:** conformance fixtures above, `sgo/scripts/test-modules.py`, `sgo/scripts/test-std.py` (darwin/arm64, linux/amd64, linux/arm64); `sgo/scripts/verify-release.sh` builds a program importing `std/strconv` from the release archive; `sgo/evidence/w14/README.md`.
- **supported_execution_configurations:** production, lab.
