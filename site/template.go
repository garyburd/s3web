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
		"args":            func(v ...interface{}) []interface{} { return v },
		"data":            ctx.data,
		"image":           ctx.image,
		"imageSrcset":     ctx.imageSrcset,
		"imageSrcWH":      ctx.imageSrcWH,
		"include":         ctx.include,
		"includeCSS":      ctx.includeCSS,
		"includeHTML":     ctx.includeHTML,
		"includeHTMLAttr": ctx.includeHTMLAttr,
		"includeJS":       ctx.includeJS,
		"includeJSStr":    ctx.includeJSStr,
	}
}

func (ctx *templateContext) filePath(p string) string {
	if strings.HasSuffix(p, "/") {
		p += "index.html"
	}
	if !strings.HasPrefix(p, "/") {
		p = path.Join(path.Dir(ctx.path), p)
	}
	p = filepath.Join(ctx.s.dir, filepath.FromSlash(p))
	return p
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

func (ctx *templateContext) includeCSS(path string) (htemp.CSS, error) {
	s, err := ctx.include(path)
	return htemp.CSS(s), err
}

func (ctx *templateContext) includeHTML(path string) (htemp.HTML, error) {
	s, err := ctx.include(path)
	return htemp.HTML(s), err
}

func (ctx *templateContext) includeHTMLAttr(path string) (htemp.HTMLAttr, error) {
	s, err := ctx.include(path)
	return htemp.HTMLAttr(s), err
}

func (ctx *templateContext) includeJS(path string) (htemp.JS, error) {
	s, err := ctx.include(path)
	return htemp.JS(s), err
}

func (ctx *templateContext) includeJSStr(path string) (htemp.JSStr, error) {
	s, err := ctx.include(path)
	return htemp.JSStr(s), err
}

func (ctx *templateContext) imageConfig(fpath string) (image.Config, error) {
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

func (ctx *templateContext) imageSrcset(p string) (htemp.HTMLAttr, error) {
	dir := path.Dir(p)
	fp := ctx.filePath(p)

	fpaths, err := filepath.Glob(fp)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `srcset="`)
	for i, fpath := range fpaths {
		c, err := ctx.imageConfig(fpath)
		if err != nil {
			return "", err
		}
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%s %dw", path.Join(dir, filepath.Base(fpath)), c.Width)
	}
	buf.WriteString(`"`)
	return htemp.HTMLAttr(buf.String()), nil
}

func (ctx *templateContext) imageSrcWH(path string) (htemp.HTMLAttr, error) {
	c, err := ctx.imageConfig(ctx.filePath(path))
	if err != nil {
		return "", err
	}
	return htemp.HTMLAttr(fmt.Sprintf(`src="%s" width="%d" height="%d"`, path, c.Width, c.Height)), nil
}

func (ctx *templateContext) image(path string, attrs ...string) (htemp.HTML, error) {
	c, err := ctx.imageConfig(ctx.filePath(path))
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
