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
	htemplate "html/template"
	"time"
)

type Page struct {
	// Layout is the path to parent template or the empty string if there is no
	// parent.
	Layout string

	// Set render to true to render this page as a template.
	Render bool

	// Title is the page's title.
	Title string

	// Subtitle is the page's subtitle.
	Subtitle string

	// Created is the page creation time.
	Created time.Time

	// Updated is the time that the page was updated.
	Updated time.Time

	// Params contains arbitrary data for use by the site.
	Params map[string]interface{}

	// Path used to load the page.
	Path string `json:"-"`

	content []byte
}

// Content is the content of page for use when Render is false.
func (p *Page) Content() htemplate.HTML {
	return htemplate.HTML(p.content)
}
