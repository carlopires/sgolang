# Benchmarks darwin-arm64 (D67)

20 fresh processes per row; P50 / P95 (min-max). Host: macOS-15.3-arm64-arm-64bit-Mach-O, 12 CPUs.
S-Go: `sgo devel-2a7044fd2d23 spec 0.6.0 admission a1bd4853d4c541d8 upstream 2aae1000ea1d4b28923b96db28acf4e3b99d6aff (host-built)`. Go: `go version go1.27.3 darwin/arm64` (isolated reference build).
Lab rows run the driver (`sgo run` vs `sgo lab run`), so they include its build with a warm cache.
Not measured: allocation counts, virtual size. Driver rows' CPU and RSS are the driver process's
(including the children it waited for). The max of a row is usually its first, cold-cache run.
Compile: S-Go is `sgo build` with the TCB cached (the user package and link only); Go is
`go build -a` (the whole standard-library closure rebuilt): not comparable, recorded for completeness.

| benchmark | variant | wall ms | CPU ms | peak RSS MiB | compile s (see above) |
|---|---|---|---|---|---|
| buffered | sgo | 7.2 / 9.3 (6.2-153.4) | 8.8 / 12.1 | 6.6 / 6.7 | 0.23 |
| buffered | go | 5.1 / 6.5 (4.8-145.8) | 5.7 / 7.5 | 3.3 / 3.4 | 0.98 |
| cancel | sgo | 15.6 / 16.4 (15.1-157.0) | 8.2 / 9.1 | 7.7 / 8.0 | 0.23 |
| cancel | go | 15.9 / 16.5 (14.8-162.9) | 8.0 / 9.3 | 8.7 / 8.8 | 1.27 |
| emptynursery | sgo | 7.0 / 7.8 (6.7-151.6) | 7.1 / 7.7 | 8.8 / 9.3 | 0.24 |
| livetasks | sgo | 67.9 / 71.9 (64.8-197.3) | 79.2 / 87.3 | 36.3 / 37.4 | 0.24 |
| livetasks | go | 66.5 / 68.2 (64.5-210.7) | 55.1 / 62.4 | 30.7 / 30.8 | 1.32 |
| rendezvous | sgo | 19.3 / 19.8 (18.9-166.3) | 26.6 / 27.5 | 9.2 / 9.7 | 0.23 |
| rendezvous | go | 12.9 / 13.5 (12.4-150.7) | 17.4 / 18.4 | 3.3 / 3.4 | 0.96 |
| rendezvous | sgo_run_lab_off | 391.3 / 405.5 (378.7-421.8) | 282.3 / 294.1 | 68.9 / 69.3 |  |
| rendezvous | sgo_lab_run | 855.6 / 876.6 (833.1-914.1) | 800.8 / 807.3 | 131.8 / 132.3 |  |
| spawn | sgo | 5.0 / 7.1 (4.8-149.0) | 11.2 / 15.9 | 8.8 / 9.0 | 0.23 |
| spawn | go | 3.8 / 5.2 (3.5-146.1) | 7.2 / 10.2 | 4.6 / 4.8 | 0.98 |
| spawn | sgo_run_lab_off | 380.3 / 387.6 (370.7-389.2) | 271.0 / 275.0 | 69.0 / 69.5 |  |
| spawn | sgo_lab_run | 594.8 / 614.3 (580.3-649.9) | 550.2 / 562.3 | 69.1 / 69.4 |  |
