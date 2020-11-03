// Copyright 2019 Gary Burd
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
	htemplate "html/template"
	"path"
	"strings"
	ttemplate "text/template"
	tparse "text/template/parse"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// template represents a parsed template.
//
// The clone field is set when the template is parsed and is never executed so
// that it can be later cloned (an html/template template cannot be cloned
// after the template is executed). The exec field is set on first execution of
// the template.
//
// modTime is the maximum file modification time of this template and any
// templates that this template depends on.
type template struct {
	clone   *htemplate.Template
	exec    *htemplate.Template
	modTime time.Time
}

func (t *template) parse(fpath string, modTime time.Time, data []byte) (*template, error) {
	t0 := htemplate.Must(t.clone.Clone())
	t1, err := t0.New(fpath).Parse(string(data))
	if err != nil {
		s := err.Error()
		if ts := strings.TrimPrefix(s, "template: "); ts != s {
			err = errors.New(ts)
		}
		return nil, err
	}

	result := &template{modTime: maxTime(modTime, t.modTime), clone: t0}

	// Return the new template if it's not empty.
	if t1.Tree != nil && t1.Tree.Root != nil {
		for _, n := range t1.Tree.Root.Nodes {
			tn, ok := n.(*tparse.TextNode)
			if !ok || len(bytes.TrimSpace(tn.Text)) > 0 {
				result.clone = t1
				break
			}
		}
	}
	return result, nil
}

func (t *template) execute(s *site, p *Page) ([]byte, time.Time, error) {
	if t.exec == nil {
		t.exec = htemplate.Must(t.clone.Clone())
	}

	udir := path.Dir(p.Path)
	fdir := s.toFilePath(udir)

	fc := functionContext{
		site:  s,
		udir:  udir,
		fdir:  fdir,
		fname: p.Path,
	}
	t.exec.Funcs(fc.funcs())

	var buf bytes.Buffer

	err := t.exec.Execute(&buf, p)
	if ee, ok := err.(ttemplate.ExecError); ok {
		s := ee.Err.Error()
		if ts := strings.TrimPrefix(s, "template: "); ts != s {
			err = errors.New(ts)
		}
	}
	return buf.Bytes(), fc.modTime, err
}

var baseTemplate = &template{
	clone: htemplate.Must(htemplate.New("_").Funcs((&functionContext{}).funcs()).Parse("")),
}
