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

	"gopkg.in/yaml.v2"
)

const ConfigDir = "/_config"

const ErrorPage = "/error.html"

type Site struct {
	dir         string
	compress    bool
	images      map[string]image.Config
	deployPaths map[string]string
}

type front struct {
	Template string
	Data     interface{}
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
	s := &Site{dir: dir, deployPaths: make(map[string]string)}
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

func (s *Site) Paths() ([]string, error) {
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
		path := filepath.ToSlash(fname[len(s.dir):])
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

func (s *Site) SetDeployPath(from, to string) {
	s.deployPaths[from] = to
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
	if !strings.HasPrefix(path, "/") {
		return nil, nil, errors.New("path must start with '/'")
	}
	fpath := filepath.Join(s.dir, filepath.FromSlash(path[1:]))
	front, body, err := s.readResource(fpath)
	if err != nil {
		return nil, nil, err
	}

	// Resource type

	mt := mime.TypeByExtension(filepath.Ext(fpath))
	if mt == "" {
		mt = "text/html; charset=utf-8"
	}

	// Execute template

	if front.Template != "NONE" {
		ctx := &templateContext{path: path, s: s}
		var files []string
		if front.Template != "" {
			files = append(files, ctx.filePath(front.Template))
		}
		files = append(files, fpath)

		var tmpl interface {
			ExecuteTemplate(wr io.Writer, name string, data interface{}) error
		}
		if typeSubtype(mt) == "text/html" {
			tmpl, err = htemp.New("").Funcs(ctx.funcMap(path, s)).ParseFiles(files...)
		} else {
			tmpl, err = ttemp.New("").Funcs(ctx.funcMap(path, s)).ParseFiles(files...)
		}
		if err != nil {
			return nil, nil, err
		}

		var buf bytes.Buffer
		err = tmpl.ExecuteTemplate(&buf, "ROOT", front.Data)
		if err != nil {
			return nil, nil, err
		}
		body = buf.Bytes()
	}

	// HTTP headers.

	encoding := "identity"
	if s.compress && compressTypes[typeSubtype(mt)] {
		var buf bytes.Buffer
		gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		gzw.Write(body)
		gzw.Close()
		body = buf.Bytes()
		encoding = "gzip"
	}

	header := http.Header{
		"Content-Type":     {mt},
		"Content-Encoding": {encoding},
		"Content-Length":   {strconv.Itoa(len(body))},
	}

	return body, header, nil
}

func typeSubtype(mt string) string {
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	return mt
}

// resource returns the parsed front matter (if any) and the file's contents.
func (s *Site) readResource(fpath string) (*front, []byte, error) {
	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			err = NotFoundError{err}
		}
		return nil, nil, err
	}

	front := &front{Template: "NONE"}
	if bytes.HasPrefix(data, frontStart) {
		if i := bytes.Index(data, frontEnd); i >= 0 {
			front.Template = ""
			err = yaml.Unmarshal(data[len(frontStart):i+1], &front)
			if err != nil {
				return front, data, err
			}
		}
	}
	return front, data, err
}
