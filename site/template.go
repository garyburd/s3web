// Copyright 2011 Gary Burd
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

package site

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	htemp "html/template"
	"image"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type templateContext struct {
	s    *Site
	path string
}

func (ctx *templateContext) funcMap(path string, s *Site) map[string]interface{} {
	return map[string]interface{}{
		"code":    ctx.code,
		"data":    ctx.data,
		"image":   ctx.image,
		"include": ctx.include,
		"path":    ctx.deployPath,
	}
}

func (ctx *templateContext) filePath(p string) string {
	if strings.HasPrefix(p, "go://") {
		d, f := path.Split(p[len("go://"):])
		pkg, err := build.Default.Import(d, "", build.FindOnly)
		if err != nil {
			return p
		}
		return filepath.Join(pkg.Dir, f)
	}
	if strings.HasSuffix(p, "/") {
		p += "index.html"
	}
	if !strings.HasPrefix(p, "/") {
		p = path.Join(path.Dir(ctx.path), p)
	}
	return filepath.Join(ctx.s.dir, filepath.FromSlash(p))
}

func (ctx *templateContext) data(p string) (interface{}, error) {
	front, _, err := ctx.s.readResource(ctx.filePath(p))
	if err != nil {
		return nil, err
	}
	if front.Data == nil {
		return nil, fmt.Errorf("front data for %s not set", p)
	}
	return front.Data, nil
}

func (ctx *templateContext) include(path string) (string, error) {
	p, err := ioutil.ReadFile(ctx.filePath(path))
	return string(p), err
}

func (ctx *templateContext) imageConfig(path string) (image.Config, error) {
	fpath := ctx.filePath(path)
	if ctx.s.images == nil {
		ctx.s.images = make(map[string]image.Config)
	}
	if c, ok := ctx.s.images[fpath]; ok {
		return c, nil
	}
	f, err := os.Open(fpath)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, fmt.Errorf("error reading %s: %v", fpath, err)
	}
	ctx.s.images[fpath] = config
	return config, nil
}

func (ctx *templateContext) image(path string, attrs ...string) (htemp.HTML, error) {
	c, err := ctx.imageConfig(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<img src="%s" width="%d" height="%d"`, ctx.deployPath(path), c.Width, c.Height)
	for _, attr := range attrs {
		buf.WriteByte(' ')
		buf.WriteString(attr)
	}
	buf.WriteByte('>')
	return htemp.HTML(buf.String()), nil
}

func (ctx *templateContext) deployPath(p string) string {
	absPath := p
	if !strings.HasPrefix(p, "/") {
		absPath = path.Join(path.Dir(ctx.path), p)
	}
	if p, ok := ctx.s.deployPaths[absPath]; ok {
		return p
	}
	return p
}

var (
	nl         = []byte{'\n'}
	tab        = []byte{'\t'}
	fourSpaces = []byte{' ', ' ', ' ', ' '}
	omitNl     = []byte("OMIT\n")
)

func (ctx *templateContext) code(path string, patterns ...string) (string, error) {
	p, err := ioutil.ReadFile(ctx.filePath(path))
	if err == nil {
		switch len(patterns) {
		case 0:
			// nothing to do
		case 1:
			p, err = oneLine(p, patterns[0])
		case 2:
			p, err = multipleLines(p, patterns[0], patterns[1])
		default:
			err = errors.New("> 2 arguments")
		}
	}
	if err != nil {
		return "", fmt.Errorf("Code %q %v, %v", path, patterns, err)
	}
	p = bytes.TrimSuffix(p, nl)
	p = bytes.Replace(p, tab, fourSpaces, -1)
	return string(p), nil
}

func oneLine(p []byte, pattern string) ([]byte, error) {
	lines := bytes.SplitAfter(p, nl)
	i, err := match(lines, pattern)
	if err != nil {
		return nil, err
	}
	return lines[i], nil
}

func multipleLines(p []byte, pattern1, pattern2 string) ([]byte, error) {
	lines := bytes.SplitAfter(p, nl)
	line1, err := match(lines, pattern1)
	if err != nil {
		return nil, err
	}
	line2, err := match(lines[line1:], pattern2)
	if err != nil {
		return nil, err
	}
	line2 += line1
	for i := line1; i <= line2; i++ {
		if bytes.HasSuffix(lines[i], omitNl) {
			lines[i] = []byte{}
		}
	}
	return bytes.Join(lines[line1:line2+1], []byte{}), nil
}

func match(lines [][]byte, pattern string) (int, error) {
	if len(pattern) < 2 || pattern[0] != '/' || pattern[len(pattern)-1] != '/' {
		return 0, fmt.Errorf("invalid pattern %q", pattern)
	}
	re, err := regexp.Compile(pattern[1 : len(pattern)-1])
	if err != nil {
		return 0, err
	}
	for i, line := range lines {
		if re.Match(line) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("pattern %q not found", pattern)
}
