# Benchmarks linux-amd64 (D67)

20 fresh processes per row; P50 / P95 (min-max). Host: Linux-7.0.0-1017-azure-x86_64-with-glibc2.39, 4 CPUs.
S-Go: `sgo devel-807605b9c3c0 spec 0.6.0 admission a1bd4853d4c541d8 upstream 2aae1000ea1d4b28923b96db28acf4e3b99d6aff (host-built)`. Go: `go version go1.27.3 linux/amd64` (isolated reference build).
Lab rows run the driver (`sgo run` vs `sgo lab run`), so they include its build with a warm cache.
Not measured: allocation counts, virtual size. Driver rows' CPU and RSS are the driver process's
(including the children it waited for). The max of a row is usually its first, cold-cache run.
Compile: S-Go is `sgo build` with the TCB cached (the user package and link only); Go is
`go build -a` (the whole standard-library closure rebuilt): not comparable, recorded for completeness.

| benchmark | variant | wall ms | CPU ms | peak RSS MiB | compile s (see above) |
|---|---|---|---|---|---|
| buffered | sgo | 7.6 / 7.9 (7.3-9.1) | 9.2 / 9.6 | 13.7 / 13.7 | 0.39 |
| buffered | go | 4.2 / 4.6 (4.0-4.7) | 4.9 / 5.6 | 13.7 / 13.7 | 2.35 |
| cancel | sgo | 14.5 / 14.7 (14.3-14.8) | 8.4 / 8.6 | 13.7 / 13.7 | 0.39 |
| cancel | go | 14.2 / 14.4 (14.1-14.6) | 8.4 / 8.8 | 13.7 / 13.7 | 2.97 |
| emptynursery | sgo | 10.8 / 11.1 (10.2-14.4) | 13.1 / 13.8 | 13.7 / 13.7 | 0.42 |
| livetasks | sgo | 79.4 / 80.2 (77.7-80.7) | 65.3 / 67.5 | 31.5 / 31.8 | 0.38 |
| livetasks | go | 71.4 / 72.0 (71.0-72.0) | 40.0 / 41.0 | 27.8 / 27.8 | 2.88 |
| rendezvous | sgo | 24.1 / 24.7 (23.3-25.4) | 28.6 / 29.3 | 13.7 / 13.7 | 0.38 |
| rendezvous | go | 14.0 / 14.4 (13.7-14.4) | 16.3 / 16.8 | 13.7 / 13.7 | 2.48 |
| rendezvous | sgo_run_lab_off | 414.6 / 420.2 (407.0-420.5) | 463.4 / 471.1 | 68.2 / 68.6 |  |
| rendezvous | sgo_lab_run | 1008.7 / 1028.8 (988.6-1030.7) | 1219.5 / 1252.0 | 110.4 / 110.7 |  |
| spawn | sgo | 6.4 / 6.8 (6.1-6.9) | 11.8 / 12.3 | 13.7 / 13.7 | 0.38 |
| spawn | go | 3.4 / 3.6 (3.1-3.7) | 6.2 / 6.6 | 13.7 / 13.7 | 2.30 |
| spawn | sgo_run_lab_off | 390.9 / 412.6 (386.6-423.7) | 436.7 / 463.7 | 68.2 / 68.7 |  |
| spawn | sgo_lab_run | 653.6 / 662.2 (648.7-662.6) | 766.1 / 778.4 | 68.4 / 69.7 |  |
