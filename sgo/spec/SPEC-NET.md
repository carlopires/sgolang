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
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NET-001, SGO-FILE-003, SGO-AUTH-002, SGO-TYPE-005, SGO-CANCEL-002, SGO-BUILD-001, SGO-LAB-002, SGO-LAB-003; D52, D54, D55, D56, D57, D58, D60.
- **grammar_and_types:** In the main module's `sgo.mod`: `grant connect IP:PORT` and `grant listen IP:PORT`, with IP an IPv4 literal or a bracketed IPv6 literal (not IPv4-mapped, no zone) and PORT in 1–65535, no host names; and `grant connect unix:PATH` and `grant listen unix:PATH` for a Unix domain stream socket at PATH, absolute or relative to the working directory at start, of 1 to 103 bytes. API: `root.Dial(addr string) (*sgo.Conn, error)`, `root.Listen(addr string) (*sgo.Listener, error)`; on `c *sgo.Conn`: `Read(max int) (string, error)`, `Write(s string) (int, error)`, `PeerUID() (int, error)`, `Close() error`, `LendTo(n *sgo.Nursery) *sgo.Conn`; on `l *sgo.Listener`: `Accept() (*sgo.Conn, error)`, `Close() error`, `LendTo(n *sgo.Nursery) *sgo.Listener`. `*sgo.Conn` and `*sgo.Listener` are capability handle types (SGO-TYPE-005). `addr` is `IP:PORT` or `unix:PATH`. No DNS, UDP, Unix datagram or seqpacket sockets, Linux abstract socket names, descriptor or credential passing, socket options, or deadlines are admitted.
- **preconditions:** `Dial` and `Listen` are called by the root task (children use lent handles) with an address that a grant of the same direction names exactly (IP endpoints compared as IP and port; Unix endpoints as the exact path text).
- **evaluation_and_value_semantics:** `Dial` connects a TCP stream to the endpoint; `Listen` binds and listens on it; `Accept` waits for and returns one connection. `Read` and `Write` follow SGO-FILE-003; `Read` returns end of stream (`sgo.IsEOF`) after the peer has closed its side and its data has been read. A Unix PATH is given to the kernel as written: it names an endpoint, not a tree, and the kernel follows symlinks in it (unlike the confined paths of SGO-FILE-002). `Listen` on a Unix PATH first removes an existing file there only if it is a socket (a stale one); any other file is `AddressInUse` and is left alone. `PeerUID` returns the user ID of the process at the other end of a Unix connection (SO_PEERCRED on linux, getpeereid on darwin).
- **ownership_and_lifecycle:** As SGO-FILE-003: the dialing or accepting task owns a connection; the listening task owns a listener; an accepted connection carries no authority beyond itself. Closing a Unix listener removes its socket file if PATH still names the socket it created.
- **effect_and_capability_requirements:** Only declared endpoints exist; dependencies cannot add one. Every `Dial` is checked, so a reconnect is checked again (I02). The grants are part of the build identity (`io=` in the build ID).
- **concurrency_and_memory_order:** Bytes written by one `Write` are received in order; S-Go adds no ordering between tasks beyond the stream itself.
- **allowed_nondeterminism:** Short reads; the timing and splitting of data on the stream; connection failures from the environment.
- **failure_and_partial_progress:** Errors are returned values with a kind and progress (SGO-FILE-003): `ConnectionRefused`, `ConnectionReset`, `AddressInUse`, `Unreachable`, `TimedOut`, `IOError`; `PeerUID` on a TCP connection returns `Unsupported`. An address no grant names, or `Dial`/`Listen` from a child, fails the task with `MissingCapability` before any effect.
- **cancellation_and_cleanup:** Sockets are non-blocking on the runtime's network poller: a waiting `Dial`, `Accept`, `Read`, or `Write` is a cancellation point (SGO-CANCEL-002); cancellation ends the wait at once and returns the cancellation error with the progress so far, and the handle stays usable by its other users. Closing a handle closes the socket.
- **compile_link_checks:** `SGO_E_USAGE` for malformed, non-literal, or duplicate endpoint grants and Unix paths that are empty or longer than 103 bytes; the linker accepts only the driver's canonical grant text. `SGO_E_CAPABILITY_MISUSE` for nil or comparison; `SGO_E_DISCARDED_ERROR` for unused errors; `SGO_E_UNADMITTED` for other members.
- **runtime_guards:** Exact endpoint check before any effect; root-task check; ownership and lend checks (SGO-FILE-003); cancellation registry of socket waiters.
- **tcb_assumptions:** The runtime's network poller (netpoll, kqueue/epoll) and the host's socket calls (including `AF_UNIX`, SO_PEERCRED, and getpeereid) behave as documented; `src/runtime/sgo_net.go`, `sgo_net_sys_*`, `sgo_iogrant.go`.
- **positive_tests:** I02, I06, I07; fixture:net-tcp-echo, fixture:lab-net-tcp-echo, fixture:net-unix-echo, fixture:lab-net-unix-echo, fixture:net-cancel-progress; TestSGoNet, TestSGoNetUnix, TestSGoParseAddr.
- **negative_tests:** S13, S17; fixture:net-undeclared-endpoint, fixture:net-dial-from-child, fixture:net-unix-over-file, fixture:net-grant-hostname, fixture:net-unix-path-too-long, fixture:net-dial, fixture:native-syscall-import.
- **implementation_coverage:** W09: runtime `src/runtime/sgo_net.go`, `sgo_netfake.go`, `sgo_iogrant.go`; API `src/sgo/io.go`; driver `src/cmd/sgo/iogrant.go`; linker `-sgoiogrants`; admission lists.
- **executed_evidence:** `sgo/evidence/w09/README.md` (model path and handle scenarios; runtime tests; the conformance cases above on darwin/arm64, linux/amd64, linux/arm64).
- **supported_execution_configurations:** production; lab (a loopback network inside the run connects granted dials to the run's listeners on the same endpoint, else `ConnectionRefused`; no host network is reached; waits are channel waits, so lab deadlock detection covers them, D58; a Unix endpoint is keyed by its path and creates no file, and `PeerUID` on a Unix connection returns the fixed value 1000, D60).
