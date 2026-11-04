# httpserver

A small HTTP/1.1 server (W23). `server.Run` listens on `unix:http.sock` and serves one task per
connection inside the server's nursery. A client sends several requests over one keep-alive
connection. Then the server is shut down with that connection still open: one `Cancel` of the
nursery ends the `Accept` loop and the connection task blocked reading the next request. Nothing
the server started can outlive it.

    sgo run .                 # prints expected.txt
    sgo test ./...            # parser on an in-memory stream; keep-alive, deadline, idle shutdown, restart
    sgo lab sweep -seeds 0..63 -expect expected.txt .

What it shows:

- **One parser, any stream (D176).** Package `http` is written against `std/io` resource
  interfaces (`io.LineReader`, `io.Reader`, `io.Writer`). The server hands it a `*sgo.Conn`, and the
  tests hand it an in-memory value. The compiler ties the interface value to the scope that owns
  the connection, so a parsed stream cannot be kept past its connection.
- **A deadline per request.** `/slow` runs its handler in a nursery with `CancelAfter`. When the
  deadline passes, the handler's wait reports `IsDeadline` and the server answers `504`. The
  connection and the server keep going.
- **Shutdown is scope exit.** The server keeps no list of connections to close: each accepted
  connection belongs to the server's nursery, and the nursery releases them when it ends.
