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
	"path"
	"path/filepath"
	"strings"

	"github.com/garyburd/staticsite/template"
)

type site struct {
	// File system directory for the site.
	dir string

	// Template loader.
	loader *template.Loader

	// Key is text of reported error messages.  Used to filter duplicate messages.
	reportedErrors map[string]struct{}

	// Visit function for walk.
	visitFn func(*Resource) error // Visit function for walk.

	// Previously loaded pages.
	pages map[string]*Page // Loaded pages.
}

func newSite(dir string, visitFn func(*Resource) error) *site {
	if dir == "" {
		dir = "."
	}
	s := &site{
		dir:            filepath.Clean(dir),
		visitFn:        visitFn,
		reportedErrors: make(map[string]struct{}),
		pages:          make(map[string]*Page),
	}
	var err error
	s.loader, err = template.NewLoader(filepath.Join(s.dir, LayoutDir), s.templateFuncs())
	if err != nil {
		panic(err)
	}
	return s
}

func (s *site) filePath(fdir string, udir string, upath string) string {
	if !strings.HasPrefix(upath, "/") {
		upath = path.Join(udir, upath)
	}
	return filepath.Join(s.dir, fdir, filepath.FromSlash(upath))
}

func (s *site) fileGlob(fdir string, udir string, upattern string) (fpaths []string, upaths []string, err error) {
	fpattern := s.filePath(fdir, udir, upattern)
	if err != nil {
		return nil, nil, err
	}

	fpaths, err = filepath.Glob(fpattern)
	if err != nil {
		return nil, nil, err
	}

	upaths = make([]string, len(fpaths))
	base := filepath.Join(s.dir, fdir)
	for i, fpath := range fpaths {
		p, err := filepath.Rel(base, fpath)
		if err != nil {
			return nil, nil, err
		}
		upaths[i] = shortPath(udir, "/"+filepath.ToSlash(p))
	}
	return fpaths, upaths, nil
}

func shortPath(udir string, p string) string {
	if strings.HasSuffix(p, "/index.html") {
		p = p[:len(p)-len("index.html")]
	}
	if udir == "" {
		return p
	}
	if udir == "/" {
		p = strings.TrimPrefix(p, "/")
	} else if len(p) > len(udir) &&
		p[:len(udir)] == udir &&
		p[len(udir)] == '/' {
		p = p[len(udir)+1:]
	}
	return p
}
