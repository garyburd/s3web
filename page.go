// Copyright 2013 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"bytes"
	"fmt"
	"go/build"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

var (
	commentStart = []byte("{{/*\n")
	commentEnd   = []byte("*/}}\n")
	nameValuePat = regexp.MustCompile(`^([-_a-zA-Z]+):\s+([^\n]+)\n`)
)

func parseFrontMatter(p []byte) map[string]string {
	if !bytes.HasPrefix(p, commentStart) {
		return nil
	}
	p = p[len(commentStart):]
	fm := make(map[string]string)
	for {
		if bytes.HasPrefix(p, commentEnd) {
			return fm
		}
		m := nameValuePat.FindSubmatch(p)
		if m == nil {
			return nil
		}
		fm[string(m[1])] = string(m[2])
		p = p[len(m[0]):]
	}
	return nil
}

func evalPage(fname string, p []byte) ([]byte, error) {
	page := parseFrontMatter(p)
	if page == nil {
		return p, nil
	}
	t, err := template.New("").Parse(string(p))
	if err != nil {
		return nil, err
	}
	ctx := &pageContext{
		rootDir: rootDir,
		fileDir: filepath.Dir(fname),
		Page:    page,
	}

	tname := page["Template"]
	if tname != "" {
		tname := ctx.resolvePath(tname)
		if _, err := t.ParseFiles(tname); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	err = t.ExecuteTemplate(&buf, "ROOT", ctx)
	return buf.Bytes(), err
}

type pageContext struct {
	rootDir string
	fileDir string
	Page    map[string]string
}

func (ctx *pageContext) resolvePath(fname string) string {
	switch {
	case strings.HasPrefix(fname, "go://"):
		d, f := path.Split(fname[len("go://"):])
		p, err := build.Default.Import(d, "", build.FindOnly)
		if err == nil {
			fname = filepath.Join(p.Dir, f)
		}
	case strings.HasPrefix(fname, "/"):
		fname = filepath.Join(ctx.rootDir, filepath.FromSlash(fname[1:]))
	default:
		fname = filepath.Join(ctx.fileDir, filepath.FromSlash(fname))
	}
	return fname
}

func (ctx *pageContext) Image(fname string, attrs ...string) (string, error) {
	rc, err := os.Open(ctx.resolvePath(fname))
	if err != nil {
		return "", err
	}
	defer rc.Close()
	config, _, err := image.DecodeConfig(rc)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<img src="%s" width="%d" height="%d"`, fname, config.Width, config.Height)
	for _, attr := range attrs {
		buf.WriteByte(' ')
		buf.WriteString(attr)
	}
	buf.WriteByte('>')
	return buf.String(), nil
}
