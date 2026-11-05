# httpserver

A small HTTP/1.1 server (W23), in three packages:

| Package | Role |
|---|---|
| `http` | reads and writes HTTP/1.1 messages over any `std/io` stream |
| `server` | `server.Serve(task, l, ready)`: one task per connection, until the scope is cancelled |
| `client` | `client.Get`, `client.Transcript` over a keep-alive connection, and `client.Demo` |

`main` wires them together. It listens on `unix:http.sock`, starts `server.Serve` in a nursery,
and runs `client.Demo` against it: one client sends garbage, one stays idle, and one makes
keep-alive requests. Then it shuts the server down with one `Cancel` of the nursery while the
keep-alive connection is still open. Nothing the server started can outlive it.

S-Go programs have no command-line arguments yet (SGO-PROC-004), so `main` runs both sides.
Once arguments are admitted (proposed D178), `main` becomes a dispatcher: `serve` runs
`server.Serve` until interrupted, and `get PATH` makes one `client.Get`.

    sgo run .                 # prints expected.txt
    sgo test ./...            # parser on an in-memory stream; refusals; keep-alive, deadline,
                              # bad client, oversized body, chunked refused, idle shutdown, restart
    sgo lab sweep -seeds 0..63 -expect expected.txt .

What it shows:

- **A connection's failure is that connection's, not the server's.** A child task that returns an
  error cancels its siblings (SGO-SPAWN-004): that is what makes a nursery fail as a unit. A server
  wants the opposite for client errors, so the server's `serveConn` handles them itself (`hangUp`). It answers a
  refused request with the status it calls for (400, 413), closes the connection, and returns nil.
  Only an error meaning the server itself is broken should leave a connection task; this example
  has none.
- **One parser, any stream (D176).** Package `http` is written against `std/io` resource
  interfaces (`io.LineReader`, `io.Reader`, `io.Writer`). The server and the client hand it a
  `*sgo.Conn`, and the tests hand it an in-memory value. The compiler ties the interface value to the scope that
  owns the connection, so a parsed stream cannot be kept past its connection.
- **Deadlines as scopes.** Each request read runs in a nursery with `CancelAfter(ReadLimitMs)`, so
  an idle or trickling client is closed while the others are served. `/slow` runs its handler the
  same way and answers `504` when its deadline passes.
- **Shutdown is scope exit.** The server keeps no list of connections to close: each accepted
  connection belongs to the server's nursery, which releases them when it ends.

Protocol subset: keep-alive connections and bodies framed by one `Content-Length` of at most
`http.MaxBody` (1 MiB). `Transfer-Encoding`, a repeated or negative `Content-Length`, or a
malformed line is refused with `400`, and a larger body with `413`, never misparsed.
