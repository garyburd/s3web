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
	"fmt"
	htemp "html/template"
	"image"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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
		"data":    ctx.data,
		"image":   ctx.image,
		"include": ctx.include,
	}
}

func (ctx *templateContext) filePath(p string) string {
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
	fmt.Fprintf(&buf, `<img src="%s" width="%d" height="%d"`, path, c.Width, c.Height)
	for _, attr := range attrs {
		buf.WriteByte(' ')
		buf.WriteString(attr)
	}
	buf.WriteByte('>')
	return htemp.HTML(buf.String()), nil
}
