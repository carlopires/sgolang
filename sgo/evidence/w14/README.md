# W14 evidence — packages beyond the module (spec 0.11.0)

Decisions D103–D105, D109–D110 (`sgo/docs/stdlib-foundations-plan.md`) and D111–D115
(`sgo/docs/w14-drafts.md`), approved 2026-11-03. Clauses frozen at 0.11.0: SGO-IMP-001 v3,
SGO-IMP-002 v2, SGO-IMP-003 v1, SGO-IMP-004 v1, SGO-AUTH-002 v3.

## What was built

- Driver (`src/cmd/sgo`): `require`/`replace` in `sgo.mod`; the module graph (`modgraph.go`): one
  version per module path, source from the main module's `replace` or `vendor/`, dependency grants
  bounded by the main module's and no file or network grants in dependencies; `sgo.sum` digests
  checked before any compile; `sgo mod sum` and `sgo mod verify` (`modcmd.go`); imports resolved by
  longest module-path prefix; positions in other modules named `<module path>/<file>`.
- Standard library: the implicit module `std` from `src/sgostd`, checked against `MANIFEST`
  (`sgo/scripts/std-manifest.py`); a `std/` package imports only `std/` and `sgo` (driver and
  compiler admission); first package `std/strconv` (`Itoa`, `Atoi`) with tests.
- Release: the archive ships `src/sgostd` and its MANIFEST; `verify-release.sh` runs a program
  importing `std/strconv` from the installed archive.

## Results (darwin/arm64, local)

- Conformance: 490 cases (469 earlier + 21 new `mod-*`/`std-*`), all pass.
- `sgo/scripts/test-modules.py`: no build writes `sgo.sum`; `sgo mod sum` reproduces the recorded
  file; `sgo mod verify` and builds refuse a vendored edit; an edited, an added, and a removed std
  file are each refused with `SGO_E_STD_MANIFEST`; std packages importing Go std or a module
  package, or using a denied construct, are refused.
- `sgo/scripts/test-std.py`: `std/strconv` 4/4 tests.
- Mutants, each caught: skipping the `sgo.sum` check (mod-sum-mismatch, mod-sum-missing fail);
  skipping the grant bound (mod-dependency-grant-exceeds fails); skipping the std MANIFEST check
  (test-modules fails).
- test-identity, test-tcb, test-flags, spec-tool, inventory, policy, guarantee, adoption,
  license-audit, std-manifest: ok.

## Linux

Pending the sgo-w00 run on the freeze commit (linux/amd64, linux/arm64).
