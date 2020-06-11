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
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/garyburd/staticsite/site"
)

var (
	flagSet  = flag.NewFlagSet("serve", flag.ExitOnError)
	httpAddr = flagSet.String("addr", "127.0.0.1:8080", "serve site at `address`")
	Command  = &site.Command{
		Name:    "serve",
		Usage:   "serve [directoy]",
		FlagSet: flagSet,
		Run:     run,
	}
)

func run() {
	h := make(handler)
	err := site.Walk(flagSet.Arg(0), func(r *site.Resource) error {
		h[r.Path] = r
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Serving %d resources at %s.", len(h), *httpAddr)
	s := http.Server{Addr: *httpAddr, Handler: h}
	log.Fatal(s.ListenAndServe())
}

type handler map[string]*site.Resource

func (h handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	switch {
	case path == "":
		path = "/index.html"
	case strings.HasSuffix(path, "/"):
		path += "index.html"
	}
	r := h[path]
	if r == nil {
		http.Error(resp, "Not Found", http.StatusNotFound)
		return
	}
	f, ct, err := r.Open()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	defer f.Close()

	resp.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	resp.Header().Set("Content-Type", ct)

	if r.Redirect != "" {
		resp.Header().Set("Location", r.Redirect)
		resp.WriteHeader(http.StatusFound)
		io.Copy(resp, f)
		return
	}

	http.ServeContent(resp, req, r.Path, r.ModTime, f)
}
