# W24 evidence — command-line programs (spec 0.19.0)

Decisions D178–D184 (approved 2026-11-06; D177 withdrawn; D183/D184 amended 2026-11-07: floats
stay denied, D185 later). Clauses frozen at 0.19.0: SGO-PROC-005, SGO-LAB-007, SGO-STD-009,
SGO-STD-010; amended: SGO-PROC-002 v2, SGO-PROC-003 v2, SGO-PROC-004 v3, SGO-FILE-002 v5,
SGO-NET-002 v4. CHANGELOG entry "0.19.0".

| Decision | What | Commits on `w24` |
|---|---|---|
| D178 | `root.Args`; `root.Stderr`, terminals, broken pipe; `root.Stdin`, `ReadSecret` | 82731602e2, cd89a7c56b, ec2c24f325, 9306355057, 9a51287938 |
| D179 | interrupts cancel the root (model first); lab `-interrupt` | 0ecac8a564, 6897dcd198, ae1ca311fa |
| D182 | per-user application directories, `unix:@runtime/NAME` | ba0e0d3d04 |
| D180 | environment by grant, `root.Getenv`, lab `-env` | f2ed1f4723 |
| D181 | argument paths: `grant path args`, `ArgDir`, `OpenArg`, `CreateArg` | c379978372, 3a880db4c5 |
| D183 | `std/fmt`, `std/time` (no floats) | 1871e85ee5, 47443f00e5 |
| D184 | CLI library and the undisk example, `sgo/examples/cli` | 6cfdea754c |

## Results (darwin/arm64)

- Conformance at 0.18.0 on 47443f00e5: 739/739, including the W24 cases `proc-args`,
  `proc-args-child`, `proc-stderr`, `proc-stderr-child`, `proc-stdin`, `proc-stdin-child`,
  `proc-env-granted`, `proc-env-ungranted`, `proc-env-grant-bad`, `file-arg-paths`,
  `file-arg-paths-refused`, `file-arg-ungranted`, `file-arg-create-readonly`, `file-arg-grant-bad`,
  `file-appdir-bad-form`, `file-appdir-path-at`, `file-app-name-bad`, `net-appsocket-bad`,
  `std-fmt-basic`, `std-time-duration`. All 20 gates passed on that commit (test-examples, test-std,
  test-fmt, test-identity, test-tcb, test-lab, test-mutations, test-flags, test-negative,
  test-modules, test-check, test-test, test-cli, legacy-audit, lock-alloc-check, res-kind-check,
  inventory-check, source-map-audit, generated-twin-check, std-api-check).
- `sgo/scripts/test-cli.py`: installed-binary arguments; a broken pipe ends quietly under
  `| head -1`; terminal detection and a 97-column pseudo-terminal; idle stdin ends at its deadline;
  `ReadSecret` hides the secret; lab stdin replays; lab input refusals (`-stdin`, `-env`,
  `-interrupt`, replay flags); SIGINT and SIGTERM unwind a reader and a sleeper and close their
  nursery (exit 1); a second signal exits 130 or 143; lab interrupts replay; application
  directories (creation, mode 0700, persistence, `ro` not created, no HOME) and a runtime-directory
  socket.
- `sgo/scripts/test-std-diff.py`: 10898 checks identical to Go's packages, seeds 1 and 7,
  including 300 random `fmt.Sprintf` formats and 800 `time` formats and parses.
- `sgo/model`: the D179 interrupt protocol (an interrupt racing scope ends).
- `sgo/examples/cli`: `sgo test ./...` (cli: TestParse, TestErrors, TestHelp, TestTreeAtRunTime;
  undisk: TestCommandsFollowTheRole, TestErrors); `sgo run .` matches `expected.txt`;
  `sgo lab sweep -seeds 0..63 -expect expected.txt .`: 64 runs, 0 failed.

## Results at the 0.19.0 freeze (9c9aab6271, on main)

- darwin/arm64: conformance 739/739 and all 20 gates on the freeze commit.
- linux/amd64 and linux/arm64: sgo-w00 run 42044978275 (conformance, identity, lab, test-cli, the examples) and sgo-runtime run 42044978558 passed; sgo-spec run 42044978567 passed.

## Open

- D186 (proposed): a CLI cannot fail with a plain message and status 1, since `main` returning an
  error writes a JSON `MainError` record. The example writes its own `undisk: ...` line, then
  returns the error. Options: keep the record; a usage-error kind that exits 1 with only its
  message; a human form of the record when stderr is a terminal.
- D185: `float64`, then a `float` flag type in the CLI library.
