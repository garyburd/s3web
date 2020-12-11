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
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// Sort for consistent output. The index page is moved to the end so the
	// index page can efficienlty query other pages in the same directory.
	sort.Slice(names, func(i, j int) bool {
		ni := names[i]
		nj := names[j]
		if ni == "index.html" {
			return false
		} else if nj == "index.html" {
			return true
		} else {
			return ni < nj
		}
	})

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
			Path:     upath + "/" + name,
			ModTime:  fileInfo.ModTime(),
			Size:     fileInfo.Size(),
		}

		// Hold pages until after child directories are visited so that pages
		// in parent directories can efficiently query pages in child
		// directories.
		if isPageDir && isPageExt(name) {
			// TODO: warn about ignored file?
			pages = append(pages, r)
			continue
		}

		if err := s.visitFile(r); err != nil {
			return err
		}
	}

	for _, r := range pages {
		err := s.processPage(r)
		if err != nil {
			m := strings.TrimPrefix(err.Error(), "template: ")
			if _, ok := s.reportedErrors[m]; !ok {
				s.reportedErrors[m] = struct{}{}
				fmt.Fprintln(os.Stderr, m)
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
	if Verbose {
		fmt.Printf("File %s -> %s\n", r.FilePath, r.Path)
	}
	return s.visitFn(r)
}

func Visit(dir string, fn func(*Resource) error) error {
	s := newSite(dir, fn)
	err := s.visitDirectory(filepath.Join(s.dir, StaticDir), "", false)
	if err != nil {
		return err
	}
	err = s.visitDirectory(filepath.Join(s.dir, PageDir), "", true)
	if err != nil {
		return err
	}
	if len(s.reportedErrors) > 0 {
		return errors.New("errors reported")
	}
	return nil
}
