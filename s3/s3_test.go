// Copyright 2020 Gary Burd
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

package s3

import (
	"testing"

	"github.com/garyburd/staticsite/site"
)

var (
	sortInput = []*site.Resource{
		{Path: "/a.css"},
		{Path: "/a.html"},

		{Path: "/b.jpg"},
		{Path: "/b.png"},

		{Path: "/c.html", UpdateReason: updateNew},
		{Path: "/c.jpg", UpdateReason: updateNew},
	}

	sortOutput = []string{
		"/c.jpg",
		"/c.html",
		"/b.jpg",
		"/b.png",
		"/a.css",
		"/a.html",
	}
)

func TestSortUploads(t *testing.T) {
	if len(sortInput) != len(sortOutput) {
		t.Fatal("length of test cases and expected output not equal")
	}
	sortUploads(sortInput)
	for i := range sortInput {
		if sortInput[i].Path != sortOutput[i] {
			t.Errorf("%d: got %s, want %s", i, sortInput[i].Path, sortOutput[i])
		}
	}
}
