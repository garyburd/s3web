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
)

func Walk(dir string, fn func(*Resource) error) error {
	site := newSite(dir)
	reportedErrors := make(map[string]struct{})
	err := filepath.Walk(site.dir, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		_, n := filepath.Split(fpath)
		if n[0] == '_' ||
			(n[0] == '.' && n != ".well-known" && n != "." && n != "..") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		r, err := site.readResource(fpath, info)
		if err != nil {
			m := err.Error()
			if _, ok := reportedErrors[m]; !ok {
				reportedErrors[m] = struct{}{}
				fmt.Fprintln(os.Stderr, m)
			}
			return nil
		}

		return fn(r)
	})
	if err != nil {
		return err
	}
	if len(reportedErrors) > 0 {
		return errors.New("errors reported")
	}
	return nil
}
