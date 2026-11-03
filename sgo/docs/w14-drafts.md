# W14 drafts — packages beyond the module (for approval)

Status: **approved 2026-11-03; frozen at spec 0.11.0** (sgo/spec/SPEC-IMPORT.md, SPEC-AUTH.md). Kept as the record of the drafts. These clauses live here, outside `sgo/spec/`, so the
frozen 0.10.1 digest does not move; on approval they freeze at spec 0.11.0. They implement the
approved decisions D103 (`std/`, not trusted), D104 (`require`/`replace`/`sgo.sum`, local only),
D105 (contracts and grants across modules), D109 (no package state), and D110 (process) of
`sgo/docs/stdlib-foundations-plan.md`.

## Small decisions taken in these drafts (flagged for the owner)

| ID | Question | Draft choice |
|---|---|---|
| D111 | What does a `require` version mean without a network or VCS? | A label checked only through `sgo.sum`: `sgo.sum` binds `(path, version)` to the digest of the module's files, so the same label can never silently name different code. Versions have the form `vMAJOR.MINOR.PATCH`. Each module path appears at most once in a build; two modules requiring different versions of one path is `SGO_E_MODULE_VERSION`. No minimum-version selection. |
| D112 | Where is a required module's source? | Only from the main module: a `replace <path> => <dir>` line (a directory relative to the main module root) or else `vendor/<path>/`. Dependencies' own `replace` lines are ignored (as in Go); their `require` lines count. |
| D113 | Who writes `sgo.sum`? | Only `sgo mod sum`, an explicit command. Builds, checks, tests and lab runs never write it; a missing or mismatched entry fails the build with the command to run. `sgo mod verify` rechecks every entry. |
| D114 | Which grants may a dependency's `sgo.mod` contain? | Effect-kind grants only, each also granted by the main module (D105). File and network grants (`grant dir/connect/listen`) are refused in a dependency: their handles come only from the root task and reach a library only as arguments. |
| D115 | Who may import `std/`? What may `std/` import? | Any package may import `std/...`. A `std/` package imports only other `std/` packages and `sgo`, never a user module. `std` is one implicit module that is always present, never required, replaced, or vendored. |

## Clauses

### SGO-IMP-001 — Packages and modules (v2; amends v1)
- **grammar_and_types:** A program is built from its *main module* (the module containing the
  package being built), the modules it requires, transitively (SGO-IMP-003), and the standard
  library (SGO-IMP-004). A module is a directory tree whose root contains an `sgo.mod` declaring
  `module <path>` and `sgo <spec-version>`; package import paths are `<module path>/<relative
  directory>`, and `std/<relative directory>` for the standard library. A module path may not
  equal, contain, or lie under the import path of a target TCB package (`sgo`, `runtime`,
  `internal/...`, D38) or `std` (D103). An import resolves to the module whose path is the longest
  prefix of the import path among the main module, the required modules, and `std`; an import that
  resolves to none is `SGO_E_UNDEFINED`. Import cycles, within or across modules, are a compile
  error, reported once at the import declaration in the cycle's lexically first package (by import
  path) that names the next package of the cycle. Every package of every module, including `std/`,
  is compiled under the same admission and sharing rules; there is no other mode.
- **evaluation_and_value_semantics:** n/a — build structure; there is no package initialization.
- **compile_link_checks:** `SGO_E_USAGE` at the module directive for a module path that collides
  with a TCB package path or `std`; `SGO_E_IMPORT_CYCLE`; `SGO_E_SPEC_VERSION` when any module's
  `sgo` directive differs from the toolchain's spec version; `SGO_E_UNDEFINED` for an import that
  resolves to no module or to a missing package.
- **positive_tests:** fixture:mod-require-local, fixture:mod-require-vendor, fixture:mod-require-transitive, fixture:std-import.
- **negative_tests:** fixture:mod-cycle-across, fixture:mod-path-under-std, fixture:mod-import-unrequired, fixture:mod-dependency-spec-version.

### SGO-IMP-002 — Contract identity of compiled packages (v2; amends v1)
- **evaluation_and_value_semantics:** unchanged in substance, extended in scope: every compiled
  package, of the main module, a required module, or `std/`, records in its export data and object
  file the spec version, the admission digest, its contract digest, the contract digests of every
  package it imports, and the S-Go ABI identity; the importing compiler and the linker verify all of
  them across module boundaries exactly as within one module. The build cache key of a package of a
  required module also includes that module's `sgo.sum` digest (SGO-IMP-003), and that of a `std/`
  package the standard library's manifest digest (SGO-IMP-004).
- **compile_link_checks:** `SGO_E_STALE_CONTRACT`, `SGO_E_SPEC_MISMATCH`, `SGO_E_FOREIGN_OBJECT` as in
  v1, now also for packages of other modules.
- **positive_tests:** fixture:mod-require-local (a required package's contract checked at import).
- **negative_tests:** fixture:mod-stale-dependency-contract.

### SGO-IMP-003 — Required modules, replacements, and sgo.sum (new)
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-AUTH-002, SGO-TOOL-001; D104, D105, D111–D114.
- **grammar_and_types:** `sgo.mod` may contain, besides `module`, `sgo`, and `grant`: lines
  `require <module path> <version>` with `<version>` of the form `vMAJOR.MINOR.PATCH` (each path at
  most once per file), and, in the main module only, `replace <module path> => <dir>` with `<dir>` a
  relative directory (each path at most once). A required module's source is the replacement
  directory if the main module replaces it, else `vendor/<module path>/` under the main module root;
  that directory's `sgo.mod` must declare exactly the required path. `replace` lines of required
  modules are ignored (D112); their `require` lines are followed transitively. `sgo.sum`, next to the
  main module's `sgo.mod`, holds one line `<module path> <version> h1:<hex>` per required module,
  sorted by path, where `<hex>` is the SHA-256 of the module's file tree: the sorted list of its
  `sgo.mod` and `.sgo` files (including `_test.sgo`), each as `<relative slash path>\0<length>\0<bytes>`.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** Every module path appears at most once in a build (D111): if
  two modules of the build require different versions of one path, the build fails. A required
  module is compiled with its own `sgo.mod` (its `sgo` directive must equal the toolchain's spec
  version) but is bounded by the main module's grants (D105): each `grant <kind>` of a dependency
  must also be granted by the main module, and a dependency may not contain file or network grants
  (D114). `sgo build`, `sgo check`, `sgo test`, `sgo run`, and `sgo lab` read `sgo.sum` and never
  write it; `sgo mod sum` computes and writes it; `sgo mod verify` recomputes every entry and
  reports each mismatch (D113).
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** A dependency's grants never extend the program's ceiling:
  the root task's ceiling stays exactly the main module's grants (SGO-AUTH-002).
- **concurrency_and_memory_order:** n/a — build structure.
- **allowed_nondeterminism:** None: `sgo.sum` and its digests are a function of the file contents.
- **failure_and_partial_progress:** Any violation fails the build before any package is compiled; no
  object is produced.
- **cancellation_and_cleanup:** n/a.
- **compile_link_checks:** `SGO_E_MODULE_MISSING` (no replacement and no vendor directory, or the
  directory's `sgo.mod` names another path), `SGO_E_MODULE_SUM` (no `sgo.sum` entry, or the digest
  differs; the message names `sgo mod sum`), `SGO_E_MODULE_VERSION` (two versions of one path, or a
  malformed version), `SGO_E_DEPENDENCY_GRANT` (a dependency grant the main module lacks, or a file
  or network grant in a dependency), `SGO_E_USAGE` (duplicate or malformed `require`/`replace`, or a
  `require` or `replace` of the main module's own path).
- **runtime_guards:** n/a — build time.
- **tcb_assumptions:** The driver computes digests and resolves directories; it is part of the
  toolchain's trusted build path, as for single-module builds.
- **positive_tests:** W14; fixture:mod-require-local, fixture:mod-require-vendor, fixture:mod-require-transitive, fixture:mod-dependency-grant-ok; tool tests of `sgo mod sum` and `sgo mod verify`.
- **negative_tests:** W14; fixture:mod-missing, fixture:mod-wrong-path, fixture:mod-sum-missing, fixture:mod-sum-mismatch, fixture:mod-version-conflict, fixture:mod-version-malformed, fixture:mod-dependency-grant-exceeds, fixture:mod-dependency-io-grant, fixture:mod-replace-twice.
- **implementation_coverage:** to be filled at freeze (planned: `src/cmd/sgo/module.go`, a new `src/cmd/sgo/modsum.go`, `src/cmd/sgo/main.go` for `sgo mod`).
- **executed_evidence:** to be filled at freeze.
- **supported_execution_configurations:** production, lab.

### SGO-IMP-004 — The standard library (new)
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-BIND-003, SGO-TCB-002; D103, D109, D115.
- **grammar_and_types:** The standard library is the implicit module `std`, always present in every
  build: its packages are imported as `std/<dir>` and their sources are the `.sgo` files of
  `$GOROOT/src/sgostd/<dir>` (D103; not `src/std/`, a reserved pattern of the go command). It is
  never required, replaced, or vendored. A `std/` package may import only other `std/` packages and
  `sgo` (D115). Its packages are written in S-Go under the same admission and sharing rules as user
  code: in particular they have no package-level variables (SGO-BIND-003) and so no package state
  (D109); they hold no capability except those passed to them as arguments.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** `std/` packages behave exactly as user packages with the same
  source would. `$GOROOT/src/sgostd/MANIFEST` lists every file of the standard library with its
  SHA-256; the driver refuses to compile a `std/` package whose files differ from the manifest
  (the release archive ships both), so a modified installation is detected rather than trusted.
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** None of its own: `std/` is not part of the TCB (D103) and
  has no grants.
- **concurrency_and_memory_order:** As for user code (SGO-MEM-003); no package state exists to share.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A violation fails the build before any package is compiled.
- **cancellation_and_cleanup:** n/a.
- **compile_link_checks:** `SGO_E_UNDEFINED` (no such `std/` package), `SGO_E_STD_MANIFEST` (a
  `std/` file differs from or is missing from the manifest), `SGO_E_UNADMITTED` (a `std/` package
  importing a user module), and every admission diagnostic for `std/` code as for user code.
- **runtime_guards:** n/a.
- **tcb_assumptions:** None beyond the driver's (SGO-IMP-003).
- **positive_tests:** W14; fixture:std-import (a first `std/` package end to end), `sgo test` of every `std/` package in CI (D110).
- **negative_tests:** W14; fixture:std-missing-package, fixture:std-manifest-mismatch (tool test on a copied installation), fixture:mod-path-under-std; a `std/` package using a denied construct is refused (tool test).
- **implementation_coverage:** to be filled at freeze (planned: `src/cmd/sgo/module.go`, `src/sgostd/`, `sgo/scripts/make-release.py`).
- **executed_evidence:** to be filled at freeze.
- **supported_execution_configurations:** production, lab.

## What else changes at 0.11.0

- SGO-AUTH-002 v2: "Dependencies cannot add grants" becomes the bound of SGO-IMP-003 (a dependency's
  grants must be granted by the main module; file and network grants are refused in dependencies).
- SGO-TOOL-001 v-next: the `sgo mod sum` and `sgo mod verify` commands.
- The release archive ships `src/sgostd/` with its `MANIFEST`; `verify-release.sh` builds a program
  importing `std/`.
- First `std/` package for the end-to-end proof: `std/strconv` with `Itoa(int) string` and
  `Atoi(string) (int, error)` (the library proper is W16).

## Acceptance

A two-module program and a program importing `std/strconv` build and run on darwin-arm64,
linux/amd64, linux/arm64; every negative fixture above is refused with its code; mutants: skip the
`sgo.sum` check (mod-sum-mismatch must fail), skip the grant bound (mod-dependency-grant-exceeds must
fail), skip the `std/` manifest check (std-manifest-mismatch must fail).
