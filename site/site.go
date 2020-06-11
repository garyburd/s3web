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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type site struct {
	// dir is the file system directory for the site
	dir string

	// templates is a cache of parsed templates. The key is the file path of
	// the template.
	templates map[string]*template
}

func newSite(dir string) *site {
	if dir == "" {
		dir = "."
	}
	return &site{
		dir:       dir,
		templates: make(map[string]*template),
	}
}

// toFilePath converts an absolute URL path to a file path.
func (s *site) toFilePath(upath string) string {
	return filepath.Join(s.dir, filepath.FromSlash(upath[1:]))
}

var frontMatterExts = map[string]bool{
	".html": true,
	".htm":  true,
}

func (s *site) readResource(fpath string, info os.FileInfo) (*Resource, error) {

	r := &Resource{ModTime: info.ModTime()}
	if s.dir == "." {
		r.Path = "/" + filepath.ToSlash(fpath)
	} else {
		r.Path = filepath.ToSlash(fpath[len(s.dir):])
	}

	if _, ok := frontMatterExts[filepath.Ext(fpath)]; !ok {
		r.FilePath = fpath
		r.Size = info.Size()
		return r, nil
	}

	p := &Page{Path: r.Path}
	if strings.HasSuffix(p.Path, "/index.html") {
		p.Path = p.Path[:len(p.Path)-len("index.html")]
	}

	data, hasFrontMatter, err := readFileWithFrontMatter(fpath, p)
	if err != nil {
		return nil, err
	}

	if !hasFrontMatter {
		r.Data = data
		r.Size = int64(len(r.Data))
		r.FilePath = fpath
		return r, nil
	}

	if p.Render == false && p.Layout == "" {
		r.Data = append(bytes.TrimSpace(data), '\n')
		r.Size = int64(len(r.Data))
		return r, nil
	}

	t, err := s.readTemplate(r.Path, p.Layout)
	if err != nil {
		return nil, err
	}
	r.ModTime = maxTime(r.ModTime, t.modTime)

	if !p.Render {
		p.content = data
	} else {
		t, err = t.parse(fpath, time.Time{}, data)
		if err != nil {
			return nil, err
		}
		t.exec = t.clone // no need to clone leaf template
	}

	data, modTime, err := t.execute(s, p)
	if err != nil {
		return nil, err
	}

	r.Data = append(bytes.TrimSpace(data), '\n')
	r.Size = int64(len(r.Data))
	r.ModTime = maxTime(r.ModTime, modTime)
	return r, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// layout is the front matter for template files.
type layout struct {
	// Layout is the path to the parent layout.
	Layout string
}

func (s *site) readTemplate(rpath, upath string) (*template, error) {
	if upath == "" {
		return baseTemplate, nil
	}

	if !strings.HasPrefix(upath, "/") {
		upath = path.Join(path.Dir(rpath), upath)
	}
	fpath := s.toFilePath(upath)

	t, ok := s.templates[fpath]
	if ok {
		if t == nil {
			return nil, errors.New("recursive layouts")
		}
		return t, nil
	}

	info, err := os.Stat(fpath)
	if err != nil {
		return nil, fmt.Errorf("%s:1: %w", s.toFilePath(rpath), err)
	}

	var l layout
	data, _, err := readFileWithFrontMatter(fpath, &l)
	if err != nil {
		return nil, err
	}

	s.templates[fpath] = nil // for detecting recursion
	t, err = s.readTemplate(upath, l.Layout)
	delete(s.templates, fpath) // undo recursion check

	if err != nil {
		return t, err
	}

	t, err = t.parse(fpath, info.ModTime(), data)
	if err != nil {
		return nil, err
	}

	s.templates[fpath] = t
	return t, nil
}

var (
	frontStart = regexp.MustCompile(`(?m)\A{\s*`)
	frontEnd   = regexp.MustCompile(`(?m)^}\s*$`)
)

func readFileWithFrontMatter(fpath string, fm interface{}) ([]byte, bool, error) {

	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, false, err
	}

	if m := frontStart.FindIndex(data); m == nil {
		return data, false, nil
	}
	m := frontEnd.FindIndex(data)
	if m == nil {
		return data, false, nil
	}

	err = DecodeConfig(fpath, data[:m[1]], fm)
	if err != nil {
		return nil, false, err
	}

	// Overwrite front matter with spaces.
	for i := range data[:m[1]] {
		if data[i] != '\n' {
			data[i] = ' '
		}
	}

	return data, true, nil
}
