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
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/garyburd/staticsite/common"
	"github.com/garyburd/staticsite/site"
)

var (
	flagSet    = flag.NewFlagSet("serve", flag.ExitOnError)
	listenAddr = flagSet.String("addr", "127.0.0.1:8080", "serve site at `address`")
	live       = flagSet.Bool("live", true, "update page in browser on successful reload")
	Command    = &common.Command{
		Name:    "serve",
		Usage:   "serve [directoy]",
		FlagSet: flagSet,
		Run:     run,
	}

	reloadFlagSet = flag.NewFlagSet("reload", flag.ExitOnError)
	reloadAddr    = reloadFlagSet.String("addr", "127.0.0.1:8080", "reload site at `address`")
	ReloadCommand = &common.Command{
		Name:    "reload",
		Usage:   "reload",
		FlagSet: reloadFlagSet,
		Run:     runReload,
	}
)

const (
	// These paths are unlikely to collide with user content.
	waitPath   = "/0c1d1146cb5640e7adb189ab31c96c2a"
	reloadPath = "/1872d435a9544ac5914775a5daa0ecfe"
)

type server struct {
	live bool
	dir  string

	mu        sync.Mutex
	resources map[string]*site.Resource
	wait      chan struct{}
}

func run() {
	s := &server{
		live: *live,
		dir:  flagSet.Arg(0),
		wait: make(chan struct{}),
	}

	var err error
	s.resources, err = loadResources(s.dir, os.Stderr)
	if err != nil {
		log.Printf("Fix errors and run 'staticsite reload http://%s'", *listenAddr)
	} else {
		log.Printf("Loaded %d resources.", len(s.resources))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveResource)
	mux.HandleFunc(waitPath, s.serveWait)
	mux.HandleFunc(reloadPath, s.serveReload)

	log.Printf("Listening at %s.", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

func (s *server) serveResource(resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	s.mu.Lock()
	r := s.resources[path]
	s.mu.Unlock()

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

	if s.live && isTextHTML(ct) && req.Method != "HEAD" {
		io.Copy(resp, f)
		resp.Write(reloadScript)
		return
	}

	http.ServeContent(resp, req, r.Path, r.ModTime, f)
}

var reloadScript = []byte(`<script>
(() => {
    let wl = window.location;
    let sse = new EventSource(` + "`${wl.protocol}//${wl.host}" + waitPath + "`" + `);
    sse.addEventListener("message", () => wl.reload());
    console.log("HELLO");
})();
</script>`)

func (s *server) serveWait(resp http.ResponseWriter, req *http.Request) {
	flusher, ok := resp.(http.Flusher)
	if !ok {
		http.Error(resp, "wait not supported", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	flusher.Flush()

	s.mu.Lock()
	wait := s.wait
	s.mu.Unlock()

	select {
	case <-wait:
		io.WriteString(resp, "data: done\n\n")
	case <-req.Context().Done():
		// client gone
	}
}

func (s *server) serveReload(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain")
	resources, err := loadResources(s.dir, resp)
	if err != nil {
		log.Print(err)
		return
	}

	s.mu.Lock()
	s.resources = resources
	close(s.wait)
	s.wait = make(chan struct{})
	s.mu.Unlock()

	log.Printf("Reloaded %d resources", len(resources))
}

func loadResources(dir string, w io.Writer) (map[string]*site.Resource, error) {
	resources := make(map[string]*site.Resource)
	err := site.Visit(dir, w, func(r *site.Resource) error {
		resources[r.Path] = r
		return nil
	})
	return resources, err
}

func isTextHTML(ct string) bool {
	const th = "text/html"
	return strings.HasPrefix(ct, th) &&
		(len(ct) == len(th) || ct[len(th)] == ';')
}

func runReload() {
	r, err := http.Get(fmt.Sprintf("http://%s%s", *reloadAddr, reloadPath))
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	n, err := io.Copy(os.Stderr, r.Body)
	if n > 0 {
		os.Exit(1)
	}
}
