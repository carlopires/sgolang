# pipeline

A producer, four workers, and a collector over channels (D66). A capacity limiter lets at most two
workers compute at once. Record 37 is bad in the second run: the worker that sees it records it and
cancels the nursery, every task ends, and `Run` reports `bad record 37`.

    sgo run .                 # prints expected.txt
    sgo test ./...            # outcomes and quiescence (D64)
    sgo lab sweep -seeds 0..255 -expect expected.txt .

Two S-Go rules shape the code:

- The collector is the nursery's owner, so it receives through an `sgo.Channel`: its `Receive`
  reports cancellation as an error, where a blocked native receive would unwind the owner and, for
  the root task, end the program (SGO-CHAN-003).
- The workers are spawned directly into the nursery they cancel: a task inside an inner nursery
  cannot cancel or spawn into an outer one (SGO-SPAWN-002 (c), SGO-CANCEL-001).

The example's lab sweep found a lab scheduler bug (claimed rendezvous senders not ready at the
decision before a receiver parks), fixed in 4a3beef346; the case lab-rendezvous-claimed-senders
keeps it fixed.
