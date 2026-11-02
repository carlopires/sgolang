# Benchmarks linux-arm64 (D67)

20 fresh processes per row; P50 / P95 (min-max). Host: Linux-7.0.0-1017-azure-aarch64-with-glibc2.39, 4 CPUs.
S-Go: `sgo devel-4c1d0e19f106 spec 0.6.0 admission a1bd4853d4c541d8 upstream 2aae1000ea1d4b28923b96db28acf4e3b99d6aff (host-built)`. Go: `go version go1.27.3 linux/arm64` (isolated reference build).
Lab rows run the driver (`sgo run` vs `sgo lab run`), so they include its build with a warm cache.
Not measured: allocation counts, virtual size. Driver rows' CPU and RSS are the driver process's
(including the children it waited for). The max of a row is usually its first, cold-cache run.
Compile: S-Go is `sgo build` with the TCB cached (the user package and link only); Go is
`go build -a` (the whole standard-library closure rebuilt): not comparable, recorded for completeness.

| benchmark | variant | wall ms | CPU ms | peak RSS MiB | compile s (see above) |
|---|---|---|---|---|---|
| buffered | sgo | 9.2 / 9.4 (9.0-10.1) | 10.1 / 10.5 | 12.7 / 12.7 | 0.35 |
| buffered | go | 6.8 / 7.0 (6.7-7.1) | 7.5 / 7.8 | 12.7 / 12.7 | 2.21 |
| cancel | sgo | 14.3 / 14.5 (14.1-14.5) | 8.0 / 8.3 | 12.7 / 12.7 | 0.35 |
| cancel | go | 14.3 / 14.4 (14.1-14.4) | 7.8 / 8.1 | 12.7 / 12.7 | 2.73 |
| emptynursery | sgo | 14.5 / 15.3 (13.2-15.5) | 16.6 / 17.5 | 12.7 / 12.7 | 0.35 |
| livetasks | sgo | 76.7 / 79.4 (74.9-81.3) | 74.9 / 76.8 | 32.2 / 32.3 | 0.35 |
| livetasks | go | 69.3 / 70.3 (68.2-70.7) | 44.5 / 45.6 | 27.8 / 27.9 | 2.77 |
| rendezvous | sgo | 33.0 / 33.2 (32.6-33.3) | 37.1 / 37.6 | 12.7 / 12.7 | 0.35 |
| rendezvous | go | 18.9 / 19.0 (18.8-19.1) | 20.9 / 21.1 | 12.7 / 12.7 | 2.22 |
| rendezvous | sgo_run_lab_off | 381.6 / 383.4 (379.5-389.4) | 417.3 / 422.1 | 64.8 / 65.2 |  |
| rendezvous | sgo_lab_run | 845.4 / 855.5 (838.0-869.9) | 986.5 / 1004.4 | 110.3 / 110.7 |  |
| spawn | sgo | 8.6 / 8.8 (8.0-8.8) | 18.0 / 18.8 | 12.7 / 12.7 | 0.35 |
| spawn | go | 5.5 / 5.6 (5.2-5.8) | 9.5 / 10.1 | 12.8 / 12.8 | 2.41 |
| spawn | sgo_run_lab_off | 358.9 / 363.0 (356.2-363.3) | 399.5 / 407.1 | 65.0 / 65.3 |  |
| spawn | sgo_lab_run | 958.0 / 1046.5 (912.4-1062.4) | 1067.7 / 1160.1 | 64.8 / 65.1 |  |
