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

package site_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/garyburd/s3web/site"
)

func TestSite(t *testing.T) {
	s, err := site.New("testdata/site")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := s.Paths()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		fpath := filepath.Join("testdata/output", filepath.FromSlash(path))
		expected, err := ioutil.ReadFile(fpath)
		if err != nil {
			t.Errorf("ioutil.ReadFile(%q) returned error: %v", fpath, err)
			continue
		}
		body, _, err := s.Resource(path)
		if err != nil {
			t.Errorf("s.Resource(%q) returned error: %v", path, err)
		}
		if !bytes.Equal(expected, body) {
			t.Errorf("%s\n\t got: %q\n\twant: %q", path, body, expected)
			continue
		}
	}
}
