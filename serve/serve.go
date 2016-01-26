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
	"strings"

	"github.com/garyburd/s3web/site"
)

func handler(resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	switch {
	case path == "":
		path = "/index.html"
	case strings.HasSuffix(path, "/"):
		path += "index.html"
	}
	status := http.StatusOK

	s, err := site.New(dir, site.WithCompression(true))
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
	body, header, err := s.Resource(path)
	if _, ok := err.(site.NotFoundError); ok {
		status = http.StatusNotFound
		if b, h, erre := s.Resource(site.ErrorPage); erre == nil {
			body = b
			header = h
		} else {
			body = []byte(err.Error())
			header = http.Header{"Content-Type": {"text/plain"}}
		}
	} else if err != nil {
		status = http.StatusInternalServerError
		body = []byte(err.Error())
		header = http.Header{"Content-Type": {"text/plain"}}
	}
	for k, v := range header {
		resp.Header()[k] = v
	}
	resp.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	resp.WriteHeader(status)
	resp.Write(body)
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
