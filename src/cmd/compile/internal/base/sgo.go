// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"strings"

	"cmd/internal/objabi"
	"cmd/internal/src"
)

// SGoPos formats pos as "file:line:col" for S-Go runtime diagnostic records (SGO-ERR-002). The
// file is the name as written (line directives applied) with the -trimpath rewrites applied, so
// a driver that maps the module root away gets module-relative names.
func SGoPos(pos src.XPos) string {
	p := Ctxt.PosTable.Pos(pos)
	name := p.RelFilename()
	if Flag.TrimPath != "" {
		if n, ok := objabi.ApplyRewrites(name, Flag.TrimPath); ok {
			name = n
		}
	}
	return fmt.Sprintf("%s:%d:%d", strings.TrimPrefix(name, "./"), p.RelLine(), p.RelCol())
}
