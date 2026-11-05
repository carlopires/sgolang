// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Per-user application directories (D182). "grant dir config|state|cache ro|rw" in sgo.mod reaches
// the runtime as a directory grant whose path is "@KIND/APP"; the runtime resolves it to the
// application's own directory under the platform convention:
//
//	linux:  $XDG_CONFIG_HOME/APP (default ~/.config/APP), $XDG_STATE_HOME/APP (~/.local/state/APP),
//	        $XDG_CACHE_HOME/APP (~/.cache/APP), $XDG_RUNTIME_DIR/APP (no default)
//	darwin: ~/Library/Application Support/APP, ~/Library/Application Support/APP/State,
//	        ~/Library/Caches/APP, $TMPDIR/APP (the per-user temporary directory)
//
// An XDG variable that is not an absolute path is ignored, as the XDG specification says. Only
// these variables and HOME are read, only for a granted kind, and only by the runtime: the program
// never sees them. A directory that does not exist is created with mode 0700 when the program
// first uses the grant, not at start.

package runtime

import "internal/goos"

var sgoAppDirLock mutex // serializes the creation of a granted application directory

// sgoRawGetenv reads the environment for the runtime's own resolution of application directories;
// gogetenv returns nothing in an S-Go program (SGO-PROC-004).
func sgoRawGetenv(key string) string {
	for _, s := range environ() {
		if len(s) > len(key) && s[len(key)] == '=' && s[:len(key)] == key {
			return s[len(key)+1:]
		}
	}
	return ""
}

// sgoAppBase is the platform directory of kind (config, state, cache, runtime), or "" if it
// cannot be determined (no HOME, or no runtime directory).
func sgoAppBase(kind string) string {
	xdg := func(name, def string) string {
		if v := sgoRawGetenv(name); len(v) > 0 && v[0] == '/' {
			return v
		}
		if def == "" {
			return ""
		}
		if home := sgoRawGetenv("HOME"); len(home) > 0 && home[0] == '/' {
			return home + def
		}
		return ""
	}
	if goos.IsDarwin != 0 {
		switch kind {
		case "config", "state":
			return xdg("", "/Library/Application Support")
		case "cache":
			return xdg("", "/Library/Caches")
		case "runtime":
			return xdg("TMPDIR", "")
		}
		return ""
	}
	switch kind {
	case "config":
		return xdg("XDG_CONFIG_HOME", "/.config")
	case "state":
		return xdg("XDG_STATE_HOME", "/.local/state")
	case "cache":
		return xdg("XDG_CACHE_HOME", "/.cache")
	case "runtime":
		return xdg("XDG_RUNTIME_DIR", "")
	}
	return ""
}

// sgoAppPath resolves "@KIND/APP[/REST]" to an absolute path, or reports false.
func sgoAppPath(p string) (string, bool) {
	if len(p) < 2 || p[0] != '@' {
		return p, true
	}
	kind, rest := p[1:], ""
	if i := sgoIndexByte(kind, '/'); i >= 0 {
		kind, rest = kind[:i], kind[i:]
	}
	base := sgoAppBase(kind)
	if base == "" || rest == "" {
		return "", false
	}
	for len(base) > 1 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	if goos.IsDarwin != 0 && kind == "state" { // state beside config, not in it
		if i := sgoIndexByte(rest[1:], '/'); i >= 0 {
			rest = rest[:i+1] + "/State" + rest[i+1:]
		} else {
			rest += "/State"
		}
	}
	return base + rest, true
}

// sgoAppDirPrepare is called at start for an application directory grant g whose directory could
// not be opened because it does not exist: it records, as C strings, every prefix of its path
// for the creation on first use.
func sgoAppDirPrepare(g *sgoDirGrant) {
	p := g.path
	for i := 1; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			g.mk = append(g.mk, sgoCString(p[:i]))
		}
	}
}

// sgoAppDirCreate creates and opens the directory of grant g on its first use. It does not
// allocate: the paths were prepared at start.
func sgoAppDirCreate(g *sgoDirGrant) {
	lock(&sgoAppDirLock)
	if g.fd < 0 && g.mk != nil {
		for _, c := range g.mk {
			sgoMkdirat(sgoAT_FDCWD, c, 0o700) // an existing component is fine
		}
		fd := sgoOpenat(sgoAT_FDCWD, g.mk[len(g.mk)-1], sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
		if fd < 0 {
			g.err = sgoErrKind(fd)
		} else {
			g.fd, g.err = fd, ""
		}
		g.mk = nil
	}
	unlock(&sgoAppDirLock)
}
