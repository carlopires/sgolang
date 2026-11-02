# Installing S-Go

An S-Go release is one archive per target: `sgo-<spec>-<os>-<arch>.tar.gz` for darwin/arm64,
linux/amd64, and linux/arm64. It holds the `sgo` command, the compiler, assembler, and linker it
runs, the trusted runtime sources (`src/`, exactly the files of `sgo/TARGET_TCB.json`), the
specification registry, the examples, and Go's `LICENSE` and `PATENTS`. There is no `go` command
and no standard library outside the trusted runtime: nothing else is needed or used.

## Install

1. Check the archive against `SHA256SUMS` from the same release:

       sha256sum -c --ignore-missing SHA256SUMS        # linux
       shasum -a 256 -c --ignore-missing SHA256SUMS    # darwin

2. Extract it anywhere and put its `bin` directory on `PATH`:

       tar xzf sgo-<spec>-<os>-<arch>.tar.gz
       export PATH="$PWD/sgo-<spec>-<os>-<arch>/bin:$PATH"

   `sgo` finds the rest of the installation from its own location. It keeps compiled trusted
   packages in a cache (`$SGO_CACHE`, or the user cache directory), checked before each reuse.

3. Check the installation:

       sgo version          # sgo <toolchain> spec <spec> admission <digest> upstream <commit>

## Try the examples

From the installation directory:

    cd examples/pipeline
    sgo run . | diff - expected.txt                          # no output: the expected result
    sgo test ./...                                           # every test passes
    sgo lab sweep -seeds 0..15 -expect expected.txt .        # every schedule passes

    cd ../service
    sgo run . | diff - expected.txt
    sgo test ./...

    cd ../replay
    sgo lab replay buggy.trace.jsonl ./buggy                 # prints "debit commit credit"
    sgo lab replay buggy.trace.jsonl ./fixed                 # refused: SGO_E_LAB_DIVERGENCE
    sgo lab sweep -seeds 0..63 -expect expected.txt ./fixed  # every schedule passes

`examples/replay/buggy.trace.jsonl` was recorded by the toolchain in this archive, so it replays
with it; a trace recorded by another build is refused rather than misread.

## Write a program

A module is a directory with an `sgo.mod`:

    module example.com/hello

    sgo <spec>

and `.sgo` files. `sgo build`, `sgo run`, `sgo check`, `sgo test`, `sgo fmt`, and `sgo lab` are the
commands (SGO-TOOL-001); the specification is in the source repository under `sgo/spec`.
