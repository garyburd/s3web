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
	"go/build"
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

type templateData struct {
	s     *Site
	front map[string]interface{}
	dir   string
	path  string
}

func (td *templateData) absolutePath(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return path.Join(path.Dir(td.path), p)
}

func (td *templateData) resolvePath(p string) string {
	if strings.HasPrefix(p, "go://") {
		d, f := path.Split(p[len("go://"):])
		pkg, err := build.Default.Import(d, "", build.FindOnly)
		if err != nil {
			return p
		}
		return filepath.Join(pkg.Dir, f)
	}
	return filepath.Join(td.s.dir, filepath.FromSlash(td.absolutePath(p)))
}

func (td *templateData) Page(p ...string) (map[string]interface{}, error) {
	if len(p) == 0 {
		return td.front, nil
	}
	_, front, _, err := td.s.resource(td.absolutePath(p[0]))
	return front, err
}

func (td *templateData) Include(path string) (string, error) {
	p, err := ioutil.ReadFile(td.resolvePath(path))
	return string(p), err
}

func (td *templateData) image(path string) (image.Config, error) {
	fpath := td.resolvePath(path)
	if td.s.images == nil {
		td.s.images = make(map[string]image.Config)
	}
	if c, ok := td.s.images[fpath]; ok {
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
	td.s.images[fpath] = config
	return config, nil
}

func (td *templateData) Image(path string, attrs ...string) (htemp.HTML, error) {
	c, err := td.image(path)
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
