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

package deploys3

import (
	"bytes"
	"crypto/md5"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/garyburd/s3web/site"
	"github.com/kr/s3"
	"gopkg.in/yaml.v1"
)

var (
	FlagSet = flag.NewFlagSet("deploys3", flag.ExitOnError)
	Usage   = "deploys3 dir"
	dryRun  = FlagSet.Bool("n", false, "Dry run")
)

type config struct {
	Bucket  string
	Profile string
}

type credentials struct {
	Key    string
	Secret string
}

func Run() {
	if len(FlagSet.Args()) != 1 {
		FlagSet.Usage()
	}
	dir, err := filepath.Abs(FlagSet.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	p, err := ioutil.ReadFile(filepath.Join(dir, filepath.FromSlash(site.ConfigDir), "s3.yml"))
	if err != nil {
		log.Fatal(err)
	}

	var config config
	err = yaml.Unmarshal(p, &config)
	if err != nil {
		log.Fatal(err)
	}

	p, err = ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), ".aws", config.Profile+".yml"))
	if err != nil {
		log.Fatal(err)
	}

	var keys s3.Keys
	err = yaml.Unmarshal(p, &keys)
	if err != nil {
		log.Fatal(err)
	}

	req, _ := http.NewRequest("GET", config.Bucket, nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	s3.Sign(req, keys)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatal("status from list bucket ", resp.StatusCode)
	}

	var contents struct {
		IsTruncated bool
		Contents    []struct {
			Key  string
			ETag string
		}
	}
	if err := xml.NewDecoder(resp.Body).Decode(&contents); err != nil {
		log.Fatalf("Error reading bucket, %v", err)
	}
	if contents.IsTruncated {
		log.Fatalf("Bucket contents truncated")
	}

	sums := make(map[string]string, len(contents.Contents))
	for _, item := range contents.Contents {
		sums[item.Key] = item.ETag[1 : len(item.ETag)-1]
	}

	s, err := site.New(dir, site.WithCompression(true))
	if err != nil {
		log.Fatal(err)
	}
	paths, err := s.ResourcePaths()
	if err != nil {
		log.Fatal(err)
	}

	for _, path := range paths {
		p, h, err := s.Resource(path)
		if err != nil {
			if _, ok := err.(site.NotFoundError); ok {
				delete(sums, path[1:])
			} else {
				log.Println(err)
			}
			continue
		}

		sum := fmt.Sprintf("%x", md5.Sum(p))

		if sums[path[1:]] != sum {
			log.Printf("Uploading %s %v", path, h)
			if !*dryRun {
				req, _ := http.NewRequest("PUT", config.Bucket+path, bytes.NewReader(p))
				req.ContentLength = int64(len(p))
				req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
				req.Header.Set("X-Amz-Acl", "public-read")
				for k, v := range h {
					req.Header[k] = v
				}
				s3.Sign(req, keys)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					log.Fatal(err)
				}
				resp.Body.Close()
			}
		} else {
			log.Printf("Skipping  %s", path)
		}
		delete(sums, path[1:])
	}

	for key := range sums {
		log.Printf("Deleteing %s", key)
		if !*dryRun {
			req, _ := http.NewRequest("DELETE", config.Bucket+"/"+key, nil)
			req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
			s3.Sign(req, keys)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Fatal(err)
			}
			resp.Body.Close()
		}
	}
}
