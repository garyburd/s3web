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

package serve

import (
	"flag"
	"log"
	"net/http"

	"github.com/garyburd/s3web/site"
)

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/index.html"
	}
	status := http.StatusOK

	s, err := site.New(dir, site.WithCompression(true))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, h, err := s.Resource(path)
	if _, ok := err.(site.NotFoundError); ok {
		status = http.StatusNotFound
		if pe, he, erre := s.Resource(site.ErrorPage); erre == nil {
			p = pe
			h = he
		} else {
			p = []byte(err.Error())
			h = http.Header{"Content-Type": {"text/plain"}}
		}
	} else if err != nil {
		status = http.StatusInternalServerError
		p = []byte(err.Error())
		h = http.Header{"Content-Type": {"text/plain"}}
	}
	for k, v := range h {
		w.Header()[k] = v
	}
	w.WriteHeader(status)
	w.Write(p)
}

var (
	FlagSet  = flag.NewFlagSet("serve", flag.ExitOnError)
	Usage    = "serve dir"
	httpAddr = FlagSet.String("addr", ":8080", "serve locally at this address")
	dir      string
)

func Run() {
	if len(FlagSet.Args()) != 1 {
		FlagSet.Usage()
	}
	dir = FlagSet.Arg(0)
	s := http.Server{Addr: *httpAddr, Handler: http.HandlerFunc(handler)}
	log.Fatal(s.ListenAndServe())
}
