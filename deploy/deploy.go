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

package deploy

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/s3web/site"
	"github.com/kr/s3"
	"gopkg.in/yaml.v2"
)

var (
	FlagSet       = flag.NewFlagSet("deploy", flag.ExitOnError)
	Usage         = "deploy dir"
	dryRun        = FlagSet.Bool("n", false, "Dry run")
	force         = FlagSet.Bool("f", false, "Force upload of all files")
	compressTypes = map[string]bool{
		"application/javascript": true,
		"text/css":               true,
		"text/html":              true,
	}
)

type config struct {
	Bucket string
	MaxAge int
}

// object represent an S3 object
type object struct {
	Key          string
	ETag         string
	LastModified time.Time
}

// putRequest represents the data of an object to put.
type putRequest struct {
	path   string
	header http.Header
	body   []byte
}

func Run() {
	if len(FlagSet.Args()) != 1 {
		FlagSet.Usage()
	}
	dir := FlagSet.Arg(0)

	config, err := readConfig(dir)
	if err != nil {
		log.Fatal(err)
	}

	keys := s3.Keys{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}

	if keys.AccessKey == "" || keys.SecretKey == "" {
		log.Fatal("Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables with AWS keys")
	}

	objects, err := fetchObjects(keys, config.Bucket)
	if err != nil {
		log.Fatal(err)
	}

	var (
		wg sync.WaitGroup
		ch = make(chan putRequest, 1)
	)
	wg.Add(1)
	go func() {
		for pr := range ch {
			if err := put(keys, config.Bucket+pr.path, pr.header, pr.body); err != nil {
				log.Fatal(err)
			}
		}
		wg.Done()
	}()

	s, err := site.New(dir)
	if err != nil {
		log.Fatal(err)
	}

	err = s.Walk(func(path string, header http.Header, body []byte) error {
		if compressTypes[typeSubtype(header.Get("Content-Type"))] {
			var buf bytes.Buffer
			gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			gzw.Write(body)
			gzw.Close()
			body = buf.Bytes()
			header.Set("Content-Encoding", "gzip")
		}

		if o := objects[path[1:]]; o != nil {
			delete(objects, o.Key)
			if !*force && o.ETag == fmt.Sprintf(`"%x"`, md5.Sum(body)) {
				log.Printf("OK     %s", path)
				return nil
			}
		}
		log.Printf("UPLOAD %s", path)
		if *dryRun {
			return nil
		}
		if l := header.Get("Location"); l != "" {
			header.Del("Location")
			header.Set("X-Amz-Website-Redirect-Location", l)
		}
		header.Set("X-Amz-Acl", "public-read")
		header.Set("Cache-Control", fmt.Sprintf("max-age=%d", config.MaxAge))
		ch <- putRequest{path, header, body}
		return nil
	})

	close(ch)
	wg.Wait()

	for _, o := range objects {
		log.Printf("DELETE /%s", o.Key)
		if *dryRun {
			continue
		}
		if err := del(keys, config.Bucket+"/"+o.Key); err != nil {
			log.Fatal(err)
		}
	}
}

func get(keys s3.Keys, path string) (io.ReadCloser, error) {
	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	s3.Sign(req, keys)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("get %s returned status %d", path, resp.StatusCode)
	}
	return resp.Body, nil
}

func put(keys s3.Keys, path string, header http.Header, body []byte) error {
	req, _ := http.NewRequest("PUT", path, bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	for k, v := range header {
		req.Header[k] = v
	}
	s3.Sign(req, keys)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("put %s returned status %d", path, resp.StatusCode)
	}
	return nil
}

func del(keys s3.Keys, path string) error {
	req, _ := http.NewRequest("DELETE", path, nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	s3.Sign(req, keys)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete %s returned status %d", path, resp.StatusCode)
	}
	return nil
}

func fetchObjects(keys s3.Keys, bucket string) (map[string]*object, error) {
	objects := make(map[string]*object)
	h := url.Values{"list-type": {"2"}}
	for {
		rc, err := get(keys, bucket+"?"+h.Encode())
		if err != nil {
			return nil, err
		}
		var data struct {
			NextContinuationToken string
			Contents              []*object
		}
		err = xml.NewDecoder(rc).Decode(&data)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("error reading bucket, %v", err)
		}
		for _, o := range data.Contents {
			objects[o.Key] = o
		}
		if data.NextContinuationToken == "" {
			break
		}
		h.Set("continuation-token", data.NextContinuationToken)
	}
	return objects, nil
}

func readConfig(dir string) (*config, error) {
	p, err := ioutil.ReadFile(filepath.Join(dir, filepath.FromSlash(site.ConfigDir), "s3.yml"))
	if err != nil {
		return nil, err
	}

	var config config
	err = yaml.Unmarshal(p, &config)
	if err != nil {
		return nil, err
	}
	if config.MaxAge == 0 {
		config.MaxAge = 60 * 60
	}
	return &config, nil
}

func typeSubtype(mt string) string {
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	return mt
}
