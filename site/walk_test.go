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

package site_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/garyburd/staticsite/site"
)

func TestWalk(t *testing.T) {
	err := site.Walk("testdata/site", func(r *site.Resource) error {
		fpath := filepath.Join("testdata/output", filepath.FromSlash(r.Path))
		expected, err := ioutil.ReadFile(fpath)
		if err != nil {
			t.Errorf("ioutil.ReadFile(%q) returned error: %v", fpath, err)
			return nil
		}
		actual := r.Data
		if r.FilePath != "" {
			actual, err = ioutil.ReadFile(r.FilePath)
			if err != nil {
				t.Errorf("ioutil.ReadFile(%q) returned error: %v", r.FilePath, err)
				return nil
			}
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("%s\n\t got: %q\n\twant: %q", r.Path, actual, expected)
			return nil
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
