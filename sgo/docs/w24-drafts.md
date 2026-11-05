# W24 drafts — owed for the 0.19.0 freeze

Decisions D178–D184 (approved 2026-11-06; D177 withdrawn). CLI programs as first-class.

## Amendments owed at 0.19.0 (frozen clauses; a new version each)

- **SGO-PROC-004 v2** (D178.1): `root.Args() []string` returns a fresh copy of the arguments after
  the program name; only the root context holds them (a child calling it fails with
  `MissingCapability`); they carry no authority (D181 aside). `sgo run PKG -- ARGS...` and
  `sgo lab run|sweep PKG -- ARGS...` pass them; `sgo test` passes none. The lab records them in the
  trace manifest (`"args"`); a replay runs with the recorded arguments and refuses others
  (`SGO_E_LAB_MANIFEST`). Environment variables stay ignored until D180. Cases: proc-args,
  proc-args-child; driver and lab checks in test-lab.

## Design notes (not yet clauses)

- **D178.2 stdin.** fd 0 is usually a terminal or a pipe shared with the parent shell, so the
  runtime does not set `O_NONBLOCK` on it (that would change the shell's open file). A runtime
  pump thread, started at the first `root.Stdin()`, reads fd 0 with blocking reads into the
  handle's read buffer (SGO-FILE-004); tasks wait for bytes with the cancellable waits of every
  handle, so a read is a cancellation point and an interrupt (D179) ends it. Read-ahead is the
  pump's: input consumed from fd 0 belongs to this process. `ReadSecret` turns the terminal's echo
  off for the line and restores it on every exit path (return, cancellation, interrupt, failure).
  In the lab, stdin is the scenario's text, recorded in the trace.
- **D178.3 broken pipe.** Already true on darwin/arm64: a write to a closed pipe returns an error
  (SIGPIPE does not kill the process) and `main` exits through `MainError` (status 1). Owed:
  `sgo.IsBrokenPipe(err)`, so a program can end quietly; Linux to be checked in CI.
- **SGO-PROC-003 v2** (D178.2–3, done): `root.Stderr() *sgo.Writer`, the stderr capability (root
  only; `MissingCapability` in a child), with `Write`/`WriteString` and conversion to `io.Writer`
  as for stdout. `w.IsTerminal()` and `w.TerminalWidth()` on both streams (false and 0 in the
  lab). A write whose reader has gone returns an error for which `sgo.IsBrokenPipe` holds (kind
  `BrokenPipe`, progress the bytes written); SIGPIPE never ends the process. Stdout write errors
  are now I/O errors with their kind instead of a bare message. Cases: proc-stderr,
  proc-stderr-child; `sgo/scripts/test-cli.py` (installed-binary arguments, broken pipe ending
  quietly under `| head -1`, a 97-column pseudo-terminal), run in sgo-w00.

## D182 — per-user application directories (for the 0.19.0 freeze)

Amendments, applied at the freeze as new clause versions:

- **SGO-FILE-002 v2 (grammar).** `grant dir config|state|cache ro|rw` names a per-user
  application directory without a path. `app NAME` in sgo.mod names the application (1 to 64 of
  `[a-z0-9._-]`, starting with a letter or digit); without it, the last element of the module path,
  which must then have that form. The canonical grant line is `dir KIND MODE @KIND/APP`, part of
  the build identity; the linker accepts an `@` path only in that form, with KIND equal to the
  grant name. A grant path of the explicit form cannot start with `@`.
- **SGO-FILE-002 v2 (evaluation).** The runtime resolves `@KIND/APP` at start under the platform
  convention, reading only HOME and the XDG variable of the kind (an XDG value that is not an
  absolute path is ignored); the program never sees the environment. Linux: `$XDG_CONFIG_HOME/APP`
  (`~/.config/APP`), `$XDG_STATE_HOME/APP` (`~/.local/state/APP`), `$XDG_CACHE_HOME/APP`
  (`~/.cache/APP`). macOS: `~/Library/Application Support/APP`,
  `~/Library/Application Support/APP/State`, `~/Library/Caches/APP`. An `rw` directory that does not
  exist is created with mode 0700, with any missing parents, on the first operation through the
  grant; an `ro` one is never created. With no HOME, every operation fails `NotFound`. The lab uses
  its in-memory tree, unchanged.
- **SGO-NET-002 v2.** `grant connect|listen unix:@runtime/NAME` names a socket in the application's
  runtime directory (`$XDG_RUNTIME_DIR/APP/NAME` on Linux, no default; `$TMPDIR/APP/NAME` on
  macOS, the per-user temporary directory). The program writes `unix:@runtime/NAME`; the canonical
  grant is `unix:@runtime/APP/NAME`. Listen creates the directory with mode 0700. Without a
  runtime directory the endpoint fails `NotFound`; a resolved path longer than 103 bytes fails
  `PathRejected`.

Fixtures: file-appdir-bad-form, file-appdir-path-at, file-app-name-bad, net-appsocket-bad;
`sgo/scripts/test-cli.py` (creation, mode 0700, persistence, ro not created, no HOME, the XDG
override on Linux, a socket round trip and its directory).
