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

package site

import (
	"bytes"
	"compress/gzip"
	"errors"
	htemp "html/template"
	"image"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	ttemp "text/template"

	"gopkg.in/yaml.v1"
)

const ConfigDir = "/_config"

const ErrorPage = "/error.html"

type Site struct {
	dir      string
	compress bool
	fronts   map[string]map[string]interface{}
	images   map[string]image.Config
}

type NotFoundError struct {
	err error
}

func (nf NotFoundError) Error() string {
	return nf.err.Error()
}

func New(dir string, options ...Option) (*Site, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Site{dir: dir}
	for _, o := range options {
		o.f(s)
	}
	return s, nil
}

type Option struct{ f func(*Site) }

func WithCompression(compress bool) Option {
	return Option{func(s *Site) {
		s.compress = true
	}}
}

// ResourcePaths returns the paths for all resources on the site.
func (s *Site) ResourcePaths() ([]string, error) {
	var paths []string
	err := filepath.Walk(s.dir, func(fname string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if _, n := filepath.Split(fname); n != ".well-known" && (n[0] == '.' || n[0] == '_') {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		paths = append(paths, filepath.ToSlash(fname[len(s.dir):]))
		return nil
	})
	return paths, err
}

var (
	frontStart    = []byte("{{/*\n")
	frontEnd      = []byte("\n*/}}\n")
	compressTypes = map[string]bool{
		"application/javascript": true,
		"text/css":               true,
		"text/html":              true,
	}
)

// Resource returns the entity for the given path.
func (s *Site) Resource(path string) ([]byte, http.Header, error) {
	fpath, front, data, err := s.resource(path)
	if err != nil {
		return nil, nil, err
	}

	// Resource type

	mt := mime.TypeByExtension(filepath.Ext(fpath))
	if mt == "" {
		mt = "text/html"
	}

	// If resource has front matter, then execute template.

	if front != nil {
		td := &templateData{path: path, s: s, front: front}
		var files []string
		if path, _ := front["Template"].(string); path != "" {
			files = append(files, td.resolvePath(path))
		}
		files = append(files, fpath)

		var tmpl interface {
			ExecuteTemplate(wr io.Writer, name string, data interface{}) error
		}
		if typeSubtype(mt) == "text/html" {
			tmpl, err = htemp.ParseFiles(files...)
		} else {
			tmpl, err = ttemp.ParseFiles(files...)
		}
		if err != nil {
			return nil, nil, err
		}

		var buf bytes.Buffer
		err = tmpl.ExecuteTemplate(&buf, "ROOT", td)
		if err != nil {
			return nil, nil, err
		}
		data = buf.Bytes()
	}

	// HTTP headers.

	encoding := "identity"
	if s.compress && compressTypes[typeSubtype(mt)] {
		var buf bytes.Buffer
		gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		gzw.Write(data)
		gzw.Close()
		data = buf.Bytes()
		encoding = "gzip"
	}

	header := http.Header{
		"Content-Type":     {mt},
		"Content-Encoding": {encoding},
		"Content-Length":   {strconv.Itoa(len(data))},
	}

	return data, header, nil
}

func typeSubtype(mt string) string {
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	return mt
}

// resource returns the parsed front matter (if any) and the file's contents.
func (s *Site) resource(path string) (string, map[string]interface{}, []byte, error) {
	if !strings.HasPrefix(path, "/") {
		return "", nil, nil, errors.New("path must start with '/'")
	}
	fpath := filepath.Join(s.dir, filepath.FromSlash(path[1:]))

	// Add/remove "index.html" as needed.
	if strings.HasSuffix(path, "/") {
		fpath = filepath.Join(fpath, "index.html")
	} else if strings.HasSuffix(path, "/index.html") {
		path = path[:len(path)-len("index.html")]
	}

	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			err = NotFoundError{err}
		}
		return fpath, nil, nil, err
	}

	var front map[string]interface{}
	if bytes.HasPrefix(data, frontStart) {
		if i := bytes.Index(data, frontEnd); i >= 0 {
			front = map[string]interface{}{
				"Path": path,
			}
			err = yaml.Unmarshal(data[len(frontStart):i+1], &front)
			if err != nil {
				return fpath, nil, data, err
			}
		}
	}
	return fpath, front, data, err
}
