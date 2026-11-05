# httpserver

A small HTTP/1.1 server (W23). `server.Run` listens on `unix:http.sock` and serves one task per
connection inside the server's nursery while a client function runs, then shuts the server down
with one `Cancel` of that nursery. The demo client sends garbage on one connection, leaves a
second idle, and makes keep-alive requests on a third. The keep-alive connection is still open at
shutdown. Nothing the server started can outlive it.

    sgo run .                 # prints expected.txt
    sgo test ./...            # parser on an in-memory stream; refusals; keep-alive, deadline,
                              # bad client, oversized body, chunked refused, idle shutdown, restart
    sgo lab sweep -seeds 0..63 -expect expected.txt .

What it shows:

- **A connection's failure is that connection's, not the server's.** A child task that returns an
  error cancels its siblings (SGO-SPAWN-004): that is what makes a nursery fail as a unit. A server
  wants the opposite for client errors, so `serveConn` handles them itself (`hangUp`). It answers a
  refused request with the status it calls for (400, 413), closes the connection, and returns nil.
  Only an error meaning the server itself is broken should leave a connection task; this example
  has none.
- **One parser, any stream (D176).** Package `http` is written against `std/io` resource
  interfaces (`io.LineReader`, `io.Reader`, `io.Writer`). The server hands it a `*sgo.Conn`, and
  the tests hand it an in-memory value. The compiler ties the interface value to the scope that
  owns the connection, so a parsed stream cannot be kept past its connection.
- **Deadlines as scopes.** Each request read runs in a nursery with `CancelAfter(ReadLimitMs)`, so
  an idle or trickling client is closed while the others are served. `/slow` runs its handler the
  same way and answers `504` when its deadline passes.
- **Shutdown is scope exit.** The server keeps no list of connections to close: each accepted
  connection belongs to the server's nursery, which releases them when it ends.

Protocol subset: keep-alive connections and bodies framed by one `Content-Length` of at most
`http.MaxBody` (1 MiB). `Transfer-Encoding`, a repeated or negative `Content-Length`, or a
malformed line is refused with `400`, and a larger body with `413`, never misparsed.
