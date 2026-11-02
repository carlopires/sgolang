# service

A Unix-socket service (D66). The root listens on `unix:svc.sock` and lends the listener to a
nursery; `n.Start` returns only when the service is ready, that is when its workers are accepting.
Five clients each send a request and check the reply. Then the service is shut down by cancelling
the nursery while one more client holds an idle connection: every worker's `Accept` or `Read`
reports cancellation, each worker closes the connections it accepted, and the root closes the ones
it dialed.

    sgo run .                 # prints expected.txt
    sgo test ./...            # serving, one worker, more workers than clients, restart; quiescence (D64)
    sgo lab sweep -seeds 0..127 -expect expected.txt .

Ownership shapes the code: only the root holds the network grants, so it dials every client
connection and owns it; a lent handle cannot be closed by the task borrowing it. `runClients`
therefore runs each client in a nursery of its own, nested in the previous one, so all clients run
at once and the root closes each connection after its client has ended. Without that the tests
fail with `LeakedWork` (the runtime would close the connections, recording `HandleAbandoned`).

Its tests found a runtime bug, fixed in 98d1d5a6e5: two tasks waiting on one socket handle (workers
accepting on one listener) ended the program with `runtime: double wait`.
