# SPEC-NET — Network

Ambient network access stays denied (SGO-NET-001). Spec 0.5.0 admits exact TCP and Unix domain socket endpoint grants with owned connection and listener handles (SGO-NET-002; D52, D54-D58, D60).

### SGO-NET-001 — Network are denied in subset 0
- **version:** 2
- **status:** superseded
- **reference_closure:** SGO-ERR-001, SGO-NET-002; plan §8.3.
- **grammar_and_types:** Superseded in spec 0.5.0 by SGO-NET-002. Still denied: every network access outside the `sgo.Conn` and `sgo.Listener` API of that clause: ambient packages (`net`, `net/http`, `syscall`), DNS and host names, UDP, Unix datagram and seqpacket sockets, abstract socket names, descriptor or credential passing, socket options, and deadlines. No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and this clause.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause; admission is planned for W09 (plan §8.3).
- **negative_tests:** S13; fixture:net-dial.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-NET-002 — Endpoint grants: TCP and Unix domain sockets (subset 3)
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-NET-001, SGO-NET-003, SGO-FILE-003, SGO-AUTH-002, SGO-TYPE-005, SGO-CANCEL-002, SGO-BUILD-001, SGO-LAB-002, SGO-LAB-003; D52, D54, D55, D56, D57, D58, D60.
- **grammar_and_types:** In the main module's `sgo.mod`: `grant connect IP:PORT` and `grant listen IP:PORT`, with IP an IPv4 literal or a bracketed IPv6 literal (not IPv4-mapped, no zone) and PORT in 1–65535 (a host name is admitted only in a connect grant, for a dialer: SGO-NET-003); and `grant connect unix:PATH` and `grant listen unix:PATH` for a Unix domain stream socket at PATH, absolute or relative to the working directory at start, of 1 to 103 bytes. API: `root.Dial(addr string) (*sgo.Conn, error)`, `root.Listen(addr string) (*sgo.Listener, error)`; on `c *sgo.Conn`: `Read(max int) (string, error)`, `Write(s string) (int, error)`, `PeerUID() (int, error)`, `Close() error`, `LendTo(n *sgo.Nursery) *sgo.Conn`; on `l *sgo.Listener`: `Accept() (*sgo.Conn, error)`, `Close() error`, `LendTo(n *sgo.Nursery) *sgo.Listener`. `*sgo.Conn` and `*sgo.Listener` are capability handle types (SGO-TYPE-005). `addr` is `IP:PORT` or `unix:PATH`. No DNS outside `Dialer.Resolve` (SGO-NET-003), no UDP, Unix datagram or seqpacket sockets, Linux abstract socket names, descriptor or credential passing, socket options, or deadlines are admitted.
- **preconditions:** `Dial` and `Listen` are called by the root task (children use lent handles) with an address that a grant of the same direction names exactly (IP endpoints compared as IP and port; Unix endpoints as the exact path text).
- **evaluation_and_value_semantics:** `Dial` connects a TCP stream to the endpoint; `Listen` binds and listens on it; `Accept` waits for and returns one connection. `Read` and `Write` follow SGO-FILE-003; `Read` returns end of stream (`sgo.IsEOF`) after the peer has closed its side and its data has been read. A Unix PATH is given to the kernel as written: it names an endpoint, not a tree, and the kernel follows symlinks in it (unlike the confined paths of SGO-FILE-002). `Listen` on a Unix PATH first removes an existing file there only if it is a socket (a stale one); any other file is `AddressInUse` and is left alone. `PeerUID` returns the user ID of the process at the other end of a Unix connection (SO_PEERCRED on linux, getpeereid on darwin).
- **ownership_and_lifecycle:** As SGO-FILE-003: the dialing or accepting task owns a connection; the listening task owns a listener; an accepted connection carries no authority beyond itself. Closing a Unix listener removes its socket file if PATH still names the socket it created.
- **effect_and_capability_requirements:** Only declared endpoints exist; dependencies cannot add one. Every `Dial` is checked, so a reconnect is checked again (I02). The grants are part of the build identity (`io=` in the build ID).
- **concurrency_and_memory_order:** Bytes written by one `Write` are received in order; S-Go adds no ordering between tasks beyond the stream itself.
- **allowed_nondeterminism:** Short reads; the timing and splitting of data on the stream; connection failures from the environment.
- **failure_and_partial_progress:** Errors are returned values with a kind and progress (SGO-FILE-003): `ConnectionRefused`, `ConnectionReset`, `AddressInUse`, `Unreachable`, `TimedOut`, `IOError`; `PeerUID` on a TCP connection returns `Unsupported`. An address no grant names, a host-name address given to `root.Dial`, or `Dial`/`Listen` from a child, fails the task with `MissingCapability` before any effect.
- **cancellation_and_cleanup:** Sockets are non-blocking on the runtime's network poller: a waiting `Dial`, `Accept`, `Read`, or `Write` is a cancellation point (SGO-CANCEL-002); cancellation ends the wait at once and returns the cancellation error with the progress so far, and the handle stays usable by its other users. Closing a handle closes the socket.
- **compile_link_checks:** `SGO_E_USAGE` for malformed, non-literal, or duplicate endpoint grants and Unix paths that are empty or longer than 103 bytes; the linker accepts only the driver's canonical grant text. `SGO_E_CAPABILITY_MISUSE` for nil or comparison; `SGO_E_DISCARDED_ERROR` for unused errors; `SGO_E_UNADMITTED` for other members.
- **runtime_guards:** Exact endpoint check before any effect; root-task check; ownership and lend checks (SGO-FILE-003); cancellation registry of socket waiters.
- **tcb_assumptions:** The runtime's network poller (netpoll, kqueue/epoll) and the host's socket calls (including `AF_UNIX`, SO_PEERCRED, and getpeereid) behave as documented; `src/runtime/sgo_net.go`, `sgo_net_sys_*`, `sgo_iogrant.go`.
- **positive_tests:** I02, I06, I07; fixture:net-tcp-echo, fixture:lab-net-tcp-echo, fixture:net-unix-echo, fixture:lab-net-unix-echo, fixture:net-cancel-progress; TestSGoNet, TestSGoNetUnix, TestSGoParseAddr.
- **negative_tests:** S13, S17; fixture:net-undeclared-endpoint, fixture:net-dial-from-child, fixture:net-unix-over-file, fixture:net-grant-hostname, fixture:net-unix-path-too-long, fixture:net-dial, fixture:native-syscall-import.
- **implementation_coverage:** W09: runtime `src/runtime/sgo_net.go`, `sgo_netfake.go`, `sgo_iogrant.go`; API `src/sgo/io.go`; driver `src/cmd/sgo/iogrant.go`; linker `-sgoiogrants`; admission lists.
- **executed_evidence:** `sgo/evidence/w09/README.md` (model path and handle scenarios; runtime tests; the conformance cases above on darwin/arm64, linux/amd64, linux/arm64).
- **supported_execution_configurations:** production; lab (a loopback network inside the run connects granted dials to the run's listeners on the same endpoint, else `ConnectionRefused`; no host network is reached; waits are channel waits, so lab deadlock detection covers them, D58; a Unix endpoint is keyed by its path and creates no file, and `PeerUID` on a Unix connection returns the fixed value 1000, D60).

### SGO-NET-003 — Dialers and host-name resolution (subset 3)
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-NET-002, SGO-NET-004, SGO-FILE-003, SGO-TYPE-005, SGO-CANCEL-002, SGO-LAB-002; D143, D145, D146.
- **grammar_and_types:** In `sgo.mod`, `grant connect HOST:PORT` with HOST a canonical host name: dot-separated labels of `[a-z0-9-]`, 1 to 63 bytes each, not starting or ending with `-`, 253 bytes in all, with at least one letter or `-`. Listen grants never name a host. API: `root.Dialer(endpoints []string) *sgo.Dialer`; on `d *sgo.Dialer`: `Resolve(host, family string) ([]string, error)` with family `"ip4"` or `"ip6"`, `Dial(addr string) (*sgo.Conn, error)`, `LendTo(n *sgo.Nursery) *sgo.Dialer`, `Close() error`. `*sgo.Dialer` is a capability handle type (SGO-TYPE-005).
- **preconditions:** `Dialer` is called by the root task, and each endpoint is named exactly by a connect grant (IP, Unix, or host name). `Resolve` takes a host that one of the dialer's endpoints names. `Dial` takes one of the dialer's IP or Unix endpoints, or an address that this dialer's `Resolve` returned. The caller of `Resolve` and `Dial` owns the dialer or runs in a nursery it is lent to.
- **evaluation_and_value_semantics:** `Resolve` looks the host up in `/etc/hosts`; if the file lists it, the answer is the file's addresses of the family. Otherwise it sends one query (A for `ip4`, AAAA for `ip6`) to each name server of `/etc/resolv.conf` (`127.0.0.1` if none) over UDP, in two rounds with a one-second wait per try. A response counts only if it carries the query id and echoes the one question (the host, the record type, class IN); an answer record counts only if its owner is the host or the target of a CNAME record of the same response whose owner counts. An unspecified address (`0.0.0.0`, `::`) or an IPv4-mapped IPv6 address is never a result, from either source. It returns `IP:PORT` in canonical text for each address and each port with which the dialer names the host, in answer order. None is not an error. From then on, the dialer may dial those addresses; resolving again adds no address and port twice. `Dial` connects as `root.Dial` does (SGO-NET-002), and the connection belongs to the dialing task.
- **ownership_and_lifecycle:** The dialer is owned by the root task and lent like any handle. Resolved addresses extend that dialer only, never the grants or another dialer.
- **effect_and_capability_requirements:** A dialer carries at most the authority of the grants it was made from. A host name grants exactly the addresses it resolves to at run time, through `Resolve`. `root.Dial` refuses host names. Host-name grants are part of the build identity (`io=`).
- **concurrency_and_memory_order:** Tasks sharing a lent dialer may resolve and dial concurrently. A `Dial` sees every address that a `Resolve` returned before it.
- **allowed_nondeterminism:** DNS answers, their order and timing, and resolution failures.
- **failure_and_partial_progress:** A failed resolution returns `IOError`. Each `Dial` fails as in SGO-NET-002. An endpoint that no grant names, an address this dialer neither names nor resolved, a host the dialer does not name, or a `Dialer` call from a child, fails the task with `MissingCapability` before any effect. A family other than `ip4` or `ip6` fails it with `InvalidArgument`.
- **cancellation_and_cleanup:** Waits in `Resolve` and `Dial` are cancellation points (SGO-CANCEL-002). Cancellation is the connection timeout: there is no other.
- **compile_link_checks:** `SGO_E_USAGE` for a host-name listen grant or a malformed host name; the linker accepts only canonical grant text. `SGO_E_CAPABILITY_MISUSE` for nil or comparison of a dialer; `SGO_E_DISCARDED_ERROR` for unused errors.
- **runtime_guards:** Root check in `Dialer`. Per-call checks of the endpoint against the dialer's endpoints and resolved addresses. Owner and lend checks.
- **tcb_assumptions:** `/etc/hosts`, `/etc/resolv.conf`, and the configured name servers are trusted for the answers they give. `src/runtime/sgo_netresolve.go`, `sgo_netdial.go`, `sgo_iogrant.go`. Not implemented: TCP retry of truncated answers, search domains, address sorting (RFC 6724).
- **positive_tests:** fixture:net-dialer-race; TestSGoResolveDNSAnswer, TestSGoResolveNoGrowth, FuzzSGoDNSAnswer (runtime lane); examples/happyeyeballs (real run and the lab scenarios silent-v6, slow-aaaa, prefer-v6, slow-v6, all-refused).
- **negative_tests:** fixture:net-dialer-child-create, fixture:net-dialer-other-endpoint, fixture:net-dialer-not-lent, fixture:net-grant-hostname; TestSGoResolveResolvable (runtime lane).
- **implementation_coverage:** W18: runtime `src/runtime/sgo_netdial.go`, `sgo_netresolve.go`, `sgo_netfake.go`; API `src/sgo/io.go`; driver `src/cmd/sgo/iogrant.go` and `lab.go`; linker `-sgoiogrants`.
- **executed_evidence:** `sgo/evidence/w18/README.md`.
- **supported_execution_configurations:** production; lab. In a lab run, `Resolve` and `Dial` consult the run's network table, from the package's `lab.net` (D146): `resolve HOST IP [MS]` answers after MS virtual ms (an IP that may not be a result makes the table malformed); `peer IP:PORT [MS]` accepts after MS ms, sends `peer IP:PORT\n`, then end of stream; `blackhole IP:PORT` never answers, and the connect fails after 75 virtual seconds. The table is part of the trace header, and replay refuses a different table.

### SGO-NET-004 — Connection slots (subset 3)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NET-002, SGO-NET-003, SGO-FILE-003, SGO-TYPE-005; D144; model `sgo/model/handoff.go`.
- **grammar_and_types:** `c.NewConnSlot() *sgo.ConnSlot`; on `s *sgo.ConnSlot`: `Put(conn *sgo.Conn) error`, `Take() (*sgo.Conn, bool)`, `LendTo(n *sgo.Nursery) *sgo.ConnSlot`, `Close() error`. `*sgo.ConnSlot` is a capability handle type (SGO-TYPE-005).
- **preconditions:** `Put` is called by the connection's owner, which owns the slot or runs in a nursery it is lent to. `Take` is called by the slot's owner.
- **evaluation_and_value_semantics:** The first `Put` that succeeds moves the connection into the slot; every later `Put` returns `AlreadySet` and leaves its connection with its owner. `Take` returns the held connection and true, then false ever after.
- **ownership_and_lifecycle:** `Put` takes the connection out of the putter's handles; `Take` makes the taker its owner, in the taker's current scope. A connection still in the slot is closed when the slot is closed.
- **effect_and_capability_requirements:** A slot moves one connection between tasks without widening any authority.
- **concurrency_and_memory_order:** `Put` is one critical section: check, unlink, and publish happen together, so exactly one of any concurrent `Put`s wins (checked by the model in `sgo/model/handoff.go`). `Take` sees the winner's connection.
- **allowed_nondeterminism:** Which concurrent `Put` wins.
- **failure_and_partial_progress:** `AlreadySet` for a later `Put`; `HandleLent` for a connection lent to an open nursery; `HandleClosed` for a closed connection. A `Put` by anyone but the connection's owner, or a `Take` by anyone but the slot's owner, fails the task with `MissingCapability`.
- **cancellation_and_cleanup:** `Put` and `Take` do not wait.
- **compile_link_checks:** `SGO_E_CAPABILITY_MISUSE` for nil or comparison of a slot; `SGO_E_DISCARDED_ERROR` for an unused `Put` or `Close` error.
- **runtime_guards:** Owner checks; the slot lock around `Put`; closing a slot closes the connection it holds.
- **tcb_assumptions:** `src/runtime/sgo_netdial.go`.
- **positive_tests:** fixture:net-dialer-race, fixture:net-slot-second-put, fixture:net-slot-close-closes; TestHandoff (model).
- **negative_tests:** fixture:net-slot-take-not-owner, fixture:net-slot-put-not-owner.
- **implementation_coverage:** W18: runtime `src/runtime/sgo_netdial.go`; API `src/sgo/io.go`; admission lists.
- **executed_evidence:** `sgo/evidence/w18/README.md`.
- **supported_execution_configurations:** production; lab.
