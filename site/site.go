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
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/garyburd/staticsite/common"
	"github.com/garyburd/staticsite/site/template"
)

type site struct {
	// File system directory for the site.
	dir string

	// Template loader.
	loader *template.Loader

	// Key is text of reported error messages. Used to filter duplicate error messages.
	reportedErrors map[string]struct{}

	// Destination for error messages.
	errOut io.Writer

	// Visit function for walk.
	visitFn func(*Resource) error // Visit function for walk.

	// Previously loaded pages.
	pagesMu sync.RWMutex
	pages   map[string]*Page

	fileHashesMu sync.Mutex
	fileHashes   map[string]string
}

func newSite(dir string, errOut io.Writer, visitFn func(*Resource) error) *site {
	if dir == "" {
		dir = "."
	}
	s := &site{
		dir:            filepath.Clean(dir),
		visitFn:        visitFn,
		errOut:         errOut,
		reportedErrors: make(map[string]struct{}),
		pages:          make(map[string]*Page),
		fileHashes:     make(map[string]string),
	}
	var err error
	s.loader, err = template.NewLoader(filepath.Join(s.dir, common.LayoutDir), s.templateFuncs())
	if err != nil {
		panic(err)
	}
	return s
}

func (s *site) addPage(queryPath string, p *Page) {
	s.pagesMu.Lock()
	s.pages[queryPath] = p
	s.pagesMu.Unlock()
}

func (s *site) getPage(upath string) *Page {
	s.pagesMu.RLock()
	p := s.pages[upath]
	s.pagesMu.RUnlock()
	return p
}

func (s *site) globPages(upattern string) ([]*Page, error) {
	s.pagesMu.RLock()
	defer s.pagesMu.RUnlock()

	var pages []*Page
	for upath, page := range s.pages {
		matched, err := path.Match(upattern, upath)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func (s *site) getFileHash(fpath string) (string, error) {
	s.fileHashesMu.Lock()
	hash := s.fileHashes[fpath]
	s.fileHashesMu.Unlock()
	if hash != "" {
		return hash, nil
	}

	f, err := os.Open(fpath)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	hasher := md5.New()
	io.Copy(hasher, f)
	sum := hasher.Sum(nil)
	hash = fmt.Sprintf("%x", sum[:])

	s.fileHashesMu.Lock()
	s.fileHashes[fpath] = hash
	s.fileHashesMu.Unlock()

	return hash, nil
}

func (s *site) filePath(fdir string, upath string) string {
	return filepath.Join(s.dir, fdir, filepath.FromSlash(upath))
}

func (s *site) fileGlob(fdir string, upattern string) (fpaths []string, upaths []string, err error) {
	fpattern := s.filePath(fdir, upattern)
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
		upaths[i] = "/" + filepath.ToSlash(p)
	}
	return fpaths, upaths, nil
}

func shortPath(upage string, p string) string {
	if upage == "" {
		return p
	}
	if upage == "/" {
		p = strings.TrimPrefix(p, "/")
	} else {
		udir := path.Dir(upage)
		if len(p) > len(udir) &&
			p[:len(udir)] == udir &&
			p[len(udir)] == '/' {
			p = p[len(udir)+1:]
		}
	}
	return p
}
