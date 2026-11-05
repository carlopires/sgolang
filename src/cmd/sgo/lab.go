// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// sgo lab (D41, D46, D48): lab binaries run under a manifest on fd 3 and write their canonical
// event lines to fd 4 (src/runtime/sgo_lab_boot.go). The driver adds the trace header and footer
// and a sidecar with host data, which never enters the canonical lines.
//
//	sgo lab run [-seed N] [-sched random|fifo] [-k N] [-quota N] [-epoch MS] [-trace FILE] [-timeout S] <main package>
//	sgo lab replay [-timeout S] <trace> <main package>
//	sgo lab sweep -seeds A..B [-sched ...] [-k N] [-quota N] [-expect FILE] [-save FILE] [-timeout S] <main package>

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// labFlags are the flags of the sgo lab commands.
type labFlags struct {
	seed, seedTo uint64
	haveSeeds    bool
	policy       string
	k, quota     int64
	epoch        int64 // WallClock at virtual time 0, UTC ms (D45)
	trace        string
	expect, save string
	timeout      time.Duration
	args         []string
	progArgs     []string // the program's arguments, after "--" (D178)
	stdin        string   // the run's standard input, from -stdin FILE (D178)
	interrupt    []int64  // virtual times of injected interrupts, from -interrupt MS[,MS] (D179)
	haveProgArgs bool
}

// The manifest defaults: K of D43, a quota that bounds any run, the watchdog of D47.
const (
	labDefaultK       = 1024
	labDefaultQuota   = 100_000_000
	labDefaultTimeout = 60 * time.Second
)

func parseLabFlags(cmd string, args []string) (*labFlags, error) {
	f := &labFlags{policy: "random", k: labDefaultK, quota: labDefaultQuota, timeout: labDefaultTimeout}
	for i, a := range args {
		if a == "--" {
			args, f.progArgs, f.haveProgArgs = args[:i], args[i+1:], true
			break
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			f.args = args[i:]
			break
		}
		name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !hasVal {
			if i+1 >= len(args) {
				return nil, usage("flag -%s needs a value", name)
			}
			i++
			val = args[i]
		}
		num := func() (int64, error) {
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil || v < 0 {
				return 0, usage("-%s must be a non-negative integer, not %q", name, val)
			}
			return v, nil
		}
		var err error
		switch {
		case name == "seed" && cmd == "run":
			var v int64
			v, err = num()
			f.seed = uint64(v)
		case name == "seeds" && cmd == "sweep":
			from, to, ok := strings.Cut(val, "..")
			a, e1 := strconv.ParseUint(from, 10, 64)
			b, e2 := strconv.ParseUint(to, 10, 64)
			if !ok || e1 != nil || e2 != nil || b < a {
				return nil, usage("-seeds must be A..B with A <= B, not %q (D48)", val)
			}
			f.seed, f.seedTo, f.haveSeeds = a, b, true
		case name == "sched" && cmd != "replay":
			if val != "random" && val != "fifo" {
				return nil, usage("-sched must be random or fifo, not %q (D49)", val)
			}
			f.policy = val
		case name == "k" && cmd != "replay":
			f.k, err = num()
			if err == nil && f.k == 0 {
				err = usage("-k must be positive")
			}
		case name == "quota" && cmd != "replay":
			f.quota, err = num()
		case name == "epoch" && cmd != "replay":
			f.epoch, err = num()
		case name == "trace" && cmd == "run":
			f.trace = val
		case name == "stdin" && cmd != "replay":
			data, rerr := os.ReadFile(val)
			if rerr != nil {
				return nil, usage("cannot read -stdin file %s", val)
			}
			f.stdin = string(data)
		case name == "interrupt" && cmd != "replay":
			f.interrupt = nil
			for _, part := range strings.Split(val, ",") {
				v, perr := strconv.ParseInt(part, 10, 64)
				if perr != nil || v < 0 || (len(f.interrupt) > 0 && v < f.interrupt[len(f.interrupt)-1]) {
					return nil, usage("-interrupt must be one or two non-decreasing virtual times in milliseconds, not %q (D179)", val)
				}
				f.interrupt = append(f.interrupt, v)
			}
			if len(f.interrupt) > 2 {
				return nil, usage("-interrupt takes at most two times: the second signal ends the run (D179)")
			}
		case name == "expect" && cmd == "sweep":
			f.expect = val
		case name == "save" && cmd == "sweep":
			f.save = val
		case name == "timeout":
			var v int64
			v, err = num()
			if err == nil && v == 0 {
				err = usage("-timeout must be positive")
			}
			f.timeout = time.Duration(v) * time.Second
		default:
			return nil, usage("unknown flag %s for sgo lab %s", a, cmd)
		}
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

func cmdLab(args []string) error {
	if len(args) == 0 {
		return usage("missing lab command (sgo lab run | replay | sweep)")
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "run":
		return cmdLabRun(rest)
	case "replay":
		return cmdLabReplay(rest)
	case "sweep":
		return cmdLabSweep(rest)
	default:
		return usage("unknown lab command %q", cmd)
	}
}

// labManifest is the manifest of one run (D46); the header of its trace.
type labManifest struct {
	Format  string   `json:"sgolab"`
	Build   string   `json:"build"`
	Seed    uint64   `json:"seed"`
	Policy  string   `json:"policy"`
	K       int64    `json:"k"`
	Quota   int64    `json:"quota"`
	Epoch   int64    `json:"epoch"`
	Replay  bool     `json:"replay,omitempty"`
	Net     []string `json:"net,omitempty"`       // the lab network table, from the package's lab.net (D146)
	Args    []string `json:"args,omitempty"`      // the program's command-line arguments (D178)
	Stdin   string   `json:"stdin,omitempty"`     // the program's standard input (D178)
	Intr    []int64  `json:"interrupt,omitempty"` // virtual times of injected interrupts (D179)
	Picks   []int    `json:"-"`
	program string   // the binary
}

const labFormat = "1"

// labResult is one run: its canonical lines, exit status, output, and host data.
type labResult struct {
	lines          []string
	status         int
	stdout, stderr []byte
	hang           bool
	wall           time.Duration
}

// labBuild builds the lab binary of the main package in dir into tmp.
func labBuild(dir, tmp string) (string, string, error) {
	exe, err := buildProgram(dir, "", 0, true, func(*module, *pkg) string { return filepath.Join(tmp, "prog") })
	return exe, lastBuildID, err
}

// labExec runs the lab binary once under m: the manifest on fd 3, the event lines from fd 4.
// stdout and stderr are copied to the given writers (and kept). A run that does not end within
// timeout is killed and reported as a host hang (D47), never as a managed outcome.
func labExec(m *labManifest, timeout time.Duration, stdout, stderr io.Writer) (*labResult, error) {
	tmp, err := os.MkdirTemp("", "sgo-lab-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	var man bytes.Buffer
	fmt.Fprintf(&man, "seed %d\npolicy %s\nk %d\nquota %d\nepoch %d\n", m.Seed, m.Policy, m.K, m.Quota, m.Epoch)
	for _, l := range m.Net {
		fmt.Fprintf(&man, "net %s\n", l)
	}
	for in := m.Stdin; in != ""; {
		n := min(len(in), 4096)
		fmt.Fprintf(&man, "stdin %x\n", in[:n])
		in = in[n:]
	}
	for _, ms := range m.Intr {
		fmt.Fprintf(&man, "interrupt %d\n", ms)
	}
	if m.Replay {
		s := make([]string, len(m.Picks))
		for i, p := range m.Picks {
			s[i] = strconv.Itoa(p)
		}
		fmt.Fprintf(&man, "replay %s\n", strings.Join(s, ","))
	}
	manPath, tracePath := filepath.Join(tmp, "manifest"), filepath.Join(tmp, "trace")
	if err := os.WriteFile(manPath, man.Bytes(), 0o600); err != nil {
		return nil, err
	}
	mf, err := os.Open(manPath)
	if err != nil {
		return nil, err
	}
	defer mf.Close()
	tf, err := os.Create(tracePath)
	if err != nil {
		return nil, err
	}
	defer tf.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.program, m.Args...)
	var out, errb bytes.Buffer
	cmd.Stdin = nil
	cmd.Stdout = io.MultiWriter(&out, stdout)
	cmd.Stderr = io.MultiWriter(&errb, stderr)
	cmd.ExtraFiles = []*os.File{mf, tf} // fd 3, fd 4
	start := time.Now()
	err = cmd.Run()
	r := &labResult{stdout: out.Bytes(), stderr: errb.Bytes(), wall: time.Since(start)}
	if ctx.Err() != nil {
		r.hang, r.status = true, -1
		return r, nil
	}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		r.status = ee.ExitCode()
	default:
		return nil, err
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if l != "" {
			r.lines = append(r.lines, l)
		}
	}
	return r, nil
}

// labDigest is the SHA-256 of the canonical lines, each ended by a newline (D46).
func labDigest(lines []string) string {
	h := sha256.New()
	for _, l := range lines {
		io.WriteString(h, l+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeLabTrace writes a trace: the manifest header, the canonical lines, and the footer (D46);
// host data go to the sidecar path+".host.json".
func writeLabTrace(path string, m *labManifest, r *labResult) error {
	var b bytes.Buffer
	hdr, _ := json.Marshal(m)
	b.Write(hdr)
	b.WriteByte('\n')
	for _, l := range r.lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	foot, _ := json.Marshal(struct {
		End    bool   `json:"end"`
		Events int    `json:"events"`
		Status int    `json:"status"`
		SHA256 string `json:"sha256"`
	}{true, len(r.lines), r.status, labDigest(r.lines)})
	b.Write(foot)
	b.WriteByte('\n')
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		return err
	}
	host, _ := json.MarshalIndent(struct {
		Hang   bool    `json:"hang"`
		Status int     `json:"status"`
		WallMS float64 `json:"wall_ms"`
	}{r.hang, r.status, float64(r.wall.Microseconds()) / 1000}, "", "  ")
	return os.WriteFile(path+".host.json", append(host, '\n'), 0o644)
}

// readLabTrace reads a trace written by writeLabTrace and checks its footer digest.
func readLabTrace(path string) (*labManifest, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, usage("cannot read trace %s", path)
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) < 2 {
		return nil, nil, labErr("SGO_E_LAB_TRACE", "trace %s has no header and footer", path)
	}
	var m labManifest
	if err := json.Unmarshal([]byte(all[0]), &m); err != nil || m.Format != labFormat {
		return nil, nil, labErr("SGO_E_LAB_TRACE", "trace %s has no sgolab %s header", path, labFormat)
	}
	var foot struct {
		End    bool   `json:"end"`
		Events int    `json:"events"`
		SHA256 string `json:"sha256"`
	}
	lines := all[1 : len(all)-1]
	if err := json.Unmarshal([]byte(all[len(all)-1]), &foot); err != nil || !foot.End ||
		foot.Events != len(lines) || foot.SHA256 != labDigest(lines) {
		return nil, nil, labErr("SGO_E_LAB_TRACE", "trace %s is incomplete or altered (footer does not match its events)", path)
	}
	return &m, lines, nil
}

func labErr(code, format string, args ...any) *toolErr {
	return &toolErr{"sgo:1:1", code, fmt.Sprintf(format, args...), 1}
}

// readLabNet reads the lab network table of the package in dir (D146): its lab.net file, if any.
// Each line, after '#' comments are removed and space runs folded, is one of
//
//	resolve HOST IP [MS]
//	peer IP:PORT [MS]
//	blackhole IP:PORT
//
// with HOST a canonical host name, IP and IP:PORT canonical literals, and MS a delay in virtual
// milliseconds; an endpoint is a peer or a blackhole at most once.
func readLabNet(dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "lab.net"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		if j := strings.IndexByte(line, '#'); j >= 0 {
			line = line[:j]
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		bad := func(why string) error {
			return &toolErr{fmt.Sprintf("%s:%d:1", filepath.Join(dir, "lab.net"), i+1), "SGO_E_USAGE", fmt.Sprintf("%q: %s (D146)", strings.Join(f, " "), why), 2}
		}
		ms := func(s string) bool {
			n, err := strconv.ParseInt(s, 10, 64)
			return err == nil && n >= 0 && strconv.FormatInt(n, 10) == s
		}
		switch {
		case f[0] == "resolve" && (len(f) == 3 || len(f) == 4):
			if h, _, ok := hostPort(f[1] + ":1"); !ok || h != f[1] {
				return nil, bad("not a canonical host name")
			}
			if a, err := netip.ParseAddr(f[2]); err != nil || a.String() != f[2] || a.Zone() != "" || a.Is4In6() {
				return nil, bad("not a canonical IP literal")
			}
			if len(f) == 4 && !ms(f[3]) {
				return nil, bad("the delay is not a non-negative integer of milliseconds")
			}
		case (f[0] == "peer" && (len(f) == 2 || len(f) == 3)) || (f[0] == "blackhole" && len(f) == 2):
			ap, err := netip.ParseAddrPort(f[1])
			if err != nil || ap.String() != f[1] || ap.Port() == 0 || ap.Addr().Zone() != "" || ap.Addr().Is4In6() {
				return nil, bad("not a canonical IP:PORT endpoint")
			}
			if seen[f[1]] {
				return nil, bad("the endpoint is already a peer or blackhole")
			}
			seen[f[1]] = true
			if len(f) == 3 && !ms(f[2]) {
				return nil, bad("the delay is not a non-negative integer of milliseconds")
			}
		default:
			return nil, bad("a line is \"resolve HOST IP [MS]\", \"peer IP:PORT [MS]\", or \"blackhole IP:PORT\"")
		}
		out = append(out, strings.Join(f, " "))
	}
	return out, nil
}
