# W09 plan — production I/O adapters (spec 0.5.0)

Decisions D51–D59 (approved 2026-11-02). Acceptance: I01–I07, S13, S17.

## Surface (package `sgo` only)

- Grants in the main `sgo.mod` (D52, D57): `grant dir NAME PATH ro|rw`, `grant connect IP:PORT`, `grant listen IP:PORT`; part of the build identity; the linker records them for the runtime, which opens each granted directory once at start.
- `root.Dir(name) *Dir`, `root.Dial(addr) (*Conn, error)`, `root.Listen(addr) (*Listener, error)`: only declared grants; `LendTo` as the other effect handles.
- `Dir` (D59): `Open`, `Create` (rw), `ReadFile`, `WriteFile`, `Remove`, `Mkdir`, `List`, `Sub`, `ReadOnly`. `File`: `Read`, `Write`, `Close`, `Size`. `Conn`: `Read`, `Write`, `Close`. `Listener`: `Accept`, `Close`.
- Errors (D56): kinds `PathRejected`, `NotFound`, `Exists`, `NotEmpty`, `HandleClosed`, `HandleLent`, `EOF`, `ConnectionRefused`, `ConnectionReset`, `IOError`; `sgo.Progress(err) int`, `sgo.NoEffect(err) bool`, `sgo.IsEOF(err) bool`; cancellation errors carry progress too.

## The handle contract (shared by model, runtime, and fakes)

1. A handle is owned by the task that opened or accepted it. Only the owner closes it; `Close` while lent to a live nursery is `HandleLent`, no effect. Unclosed at the owner's nursery-body exit or task end: closed, `HandleAbandoned` chain entry.
2. Operations on a closed handle are `HandleClosed`, no effect; a handle's generation never reaches a reused descriptor.
3. `Read(max)`: at least 1 and at most `max` bytes, or an error with progress 0 (`EOF`, `HandleClosed`, cancellation, I/O error). `Write(s)`: all of `s`, or an error with the progress so far.
4. Socket operations wait in the netpoller and are cancellation points; cancellation returns the progress so far. File system calls are not interruptible: cancellation is checked before and after each bounded call (≤ 1 MiB).
5. Path resolution: names only (no `""`, `.`, `..`, NUL, leading `/`, at most 255 bytes each, at most 4096 bytes in total); component-wise from the held directory, every component with `O_NOFOLLOW`; a symlink anywhere is `PathRejected`.

## Steps

1. **Model** (`sgo/model/io*.go`): handles with owners, lending, close, generations; reads and writes with injected short transfers and cancellation points; the path resolver over an abstract tree with symlinks and concurrent renames. Invariants: no operation through a closed generation, no close by a non-owner, progress equals bytes transferred, no resolution outside the root, every symlink refused. Pinned outcome sets for scenarios (I03, I04, I05; I01 resolver cases).
2. **Runtime**: descriptor table with generations; confined resolution (`openat` component walk; `openat2` on linux when available); file calls bounded and non-interruptible; sockets on the netpoller with S-Go cancellation registration; handle obligations (as tokens). Runtime tests against the model's outcome sets with fault injection (short transfers).
3. **Package `sgo` and driver**: the API above; `sgo.mod` grant parsing and identity; linker record of grants; admission (types2/sgoadmit) for the new handle types.
4. **Lab fakes** (D58): in-memory trees and a loopback network in the runtime, used only by lab binaries; every operation a choice point; I06 correspondence suite (same scenarios, production against local fixtures and lab fakes, normalized outcomes).
5. **Spec 0.5.0**: SPEC-FILE and SPEC-NET clauses (proposed to the user before freezing), CHANGELOG, identity, fixtures `sgo 0.5.0`; S13/S17 negative fixtures.
6. **Conformance and evidence**: fixtures for I01–I07 (production and lab), evidence `sgo/evidence/w09/README.md`, ledgers, CI.
