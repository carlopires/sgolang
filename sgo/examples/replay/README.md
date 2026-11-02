# replay

An ordering bug found by the lab, saved, replayed, and fixed (D66). `transfer` logs three steps,
each done by a task of its own; they must be logged in order: debit, credit, commit. `Buggy`
makes the commit wait for the debit but not for the credit; `Fixed` also makes it wait for the
credit.

In production the bug shows rarely (`sgo run ./buggy` usually prints the right order). The lab
explores schedules deterministically from a seed:

    sgo lab sweep -seeds 0..63 -expect expected.txt ./buggy    # 19 of 64 schedules fail
    sgo lab replay buggy.trace.jsonl ./buggy                    # prints "debit commit credit", every time
    sgo lab sweep -seeds 0..63 -expect expected.txt ./fixed    # every schedule passes
    sgo lab replay buggy.trace.jsonl ./fixed                    # LabReplayDivergence: the fix changed a choice
    sgo test ./...                                              # Fixed's order on 200 runs; quiescence (D64)

`buggy.trace.jsonl` is the first failing trace of that sweep (seed 7), committed. Its header names
the toolchain build that recorded it, so it replays only with that build; another build refuses it
rather than misreading it (D46). `sgo/scripts/test-examples.py` therefore also re-sweeps seed 7 with
the current toolchain, requires the same canonical events as the committed trace, and replays that
trace.
