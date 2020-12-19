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
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"time"
)

// Resource represents a resource on the site.
type Resource struct {
	// URI with leading slash and no trailing index file name.
	Path string

	// Modification time
	ModTime time.Time

	// Size in bytes
	Size int64

	// FilePath is path to resource on disk.
	FilePath string

	// Redirect requests to this URL.
	Redirect string

	// Data is the page data. If set, data overrides the resource data
	// stored on disk.
	Data []byte

	// For use by commands.
	UpdateReason string
}

type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type readSeekNopClose struct{ io.ReadSeeker }

func (readSeekNopClose) Close() error { return nil }

// Open opens an reader on the resource's data.
func (r *Resource) Open() (reader ReadSeekCloser, contentType string, err error) {
	if r.Data != nil {
		// Assume that MIME type of data loaded from the content directory is text/html.
		ct := "text/html; charset=utf-8"
		return readSeekNopClose{bytes.NewReader(r.Data)}, ct, nil
	}

	f, err := os.Open(r.FilePath)
	if err != nil {
		return nil, "", err
	}

	ct := mime.TypeByExtension(path.Ext(r.Path))
	if ct == "" {
		var buf [512]byte
		n, _ := io.ReadFull(f, buf[:])
		ct = http.DetectContentType(buf[:n])
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			f.Close()
			return nil, "", err
		}
	}

	return f, ct, err
}
