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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/garyburd/staticsite/common"
)

func isPageExt(name string) bool {
	return strings.HasSuffix(name, ".html")
}

func (s *site) visitDirectory(fpath string, upath string, isPageDir bool) error {
	d, err := os.Open(fpath)
	if err != nil {
		return err
	}
	names, err := d.Readdirnames(-1)
	d.Close()
	if err != nil {
		return err
	}

	var indexPage *Resource
	var indexPages []*Resource
	var pages []*Resource

	for _, name := range names {
		if name == ".DS_Store" {
			continue
		}

		filePath := fpath + string(filepath.Separator) + name
		fileInfo, err := os.Lstat(filePath)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			if err := s.visitDirectory(filePath, upath+"/"+name, isPageDir); err != nil {
				return err
			}
			continue
		}

		r := &Resource{
			FilePath: filePath,
			ModTime:  fileInfo.ModTime(),
			Size:     fileInfo.Size(),
		}

		if !isPageDir {
			if name == "index.html" {
				r.Path = upath + "/"
			} else {
				r.Path = upath + "/" + name
			}
			if err := s.visitFile(r); err != nil {
				return err
			}
			continue
		}

		// Hold pages until after child directories are visited so that pages
		// in parent directories can query pages in child directories.
		//
		// Apply page rename rules.
		//
		// Group according to allowed page queries.

		if name == "index.html" {
			r.Path = upath + "/"
			indexPage = r
		} else if strings.HasSuffix(name, ".index.html") {
			r.Path = upath + "/" + name[:len(name)-len(".index.html")]
			indexPages = append(indexPages, r)
		} else if strings.HasSuffix(name, ".html") {
			r.Path = upath + "/" + name[:len(name)-len(".html")] + "/"
			pages = append(pages, r)
		}
	}

	pages = append(pages, indexPages...)
	if indexPage != nil {
		pages = append(pages, indexPage)
	}

	for _, r := range pages {
		err := s.processPage(r)
		if err != nil {
			m := strings.TrimPrefix(err.Error(), "template: ")
			if _, ok := s.reportedErrors[m]; !ok {
				s.reportedErrors[m] = struct{}{}
				fmt.Fprintln(s.errOut, m)
			}
			return nil
		}
		if err := s.visitFile(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *site) visitFile(r *Resource) error {
	if common.Verbose {
		fmt.Printf("File %s -> %s\n", r.FilePath, r.Path)
	}
	return s.visitFn(r)
}

func Visit(dir string, errOut io.Writer, fn func(*Resource) error) error {
	s := newSite(dir, errOut, fn)
	err := s.visitDirectory(filepath.Join(s.dir, common.StaticDir), "", false)
	if err != nil {
		return err
	}
	err = s.visitDirectory(filepath.Join(s.dir, common.PageDir), "", true)
	if err != nil {
		return err
	}
	if len(s.reportedErrors) > 0 {
		return errors.New("errors reported")
	}
	return nil
}
