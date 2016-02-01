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
	"crypto/md5"
	"encoding/xml"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/garyburd/s3web/site"
	"github.com/kr/s3"
	"gopkg.in/yaml.v1"
)

var (
	FlagSet            = flag.NewFlagSet("deploy", flag.ExitOnError)
	Usage              = "deploy dir"
	dryRun             = FlagSet.Bool("n", false, "Dry run")
	force              = FlagSet.Bool("f", false, "Force upload of all files")
	versionPat         = regexp.MustCompile(`-X\.[^/]*$`)
	deployedVersionPat = regexp.MustCompile(`-[0-9A-F]{8}\.[^/]*$`)
	castagnoliTable    = crc32.MakeTable(crc32.Castagnoli)
)

type config struct {
	Bucket          string
	MaxAge          int
	MaxAgeVersioned int
}

// object represent an S3 object
type object struct {
	Key          string
	ETag         string
	LastModified time.Time

	keep bool
}

type byUploadOrder []string

func (p byUploadOrder) Len() int      { return len(p) }
func (p byUploadOrder) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byUploadOrder) Less(i, j int) bool {
	mi := versionPat.MatchString(p[i])
	mj := versionPat.MatchString(p[j])
	if mi != mj {
		return mi
	}
	ci := strings.Count(p[i], "/")
	cj := strings.Count(p[j], "/")
	if ci != cj {
		return ci > cj
	}
	return p[i] < p[j]
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

	markKeepers(objects, config.MaxAge)

	s, err := site.New(dir, site.WithCompression(true))
	if err != nil {
		log.Fatal(err)
	}
	paths, err := s.Paths()
	if err != nil {
		log.Fatal(err)
	}
	sort.Sort(byUploadOrder(paths))

	for _, path := range paths {
		body, header, err := s.Resource(path)
		if err != nil {
			log.Fatal(err)
		}
		deployPath := path
		maxAge := config.MaxAge
		if m := versionPat.FindStringIndex(path); m != nil {
			sum := crc32.Checksum(body, castagnoliTable)
			deployPath = fmt.Sprintf("%s-%08X.%s", path[:m[0]], sum, path[m[0]+3:])
			s.SetDeployPath(path, deployPath)
			maxAge = config.MaxAgeVersioned
		}
		if o := objects[deployPath[1:]]; o != nil {
			delete(objects, o.Key)
			if !*force && o.ETag == fmt.Sprintf(`"%x"`, md5.Sum(body)) {
				log.Printf("OK     %s", deployPath)
				continue
			}
		}
		log.Printf("UPLOAD %s", deployPath)
		if *dryRun {
			continue
		}
		if l := header.Get("Location"); l != "" {
			header.Del("Location")
			header.Set("X-Amz-Website-Redirect-Location", l)
		}
		header.Set("X-Amz-Acl", "public-read")
		header.Set("Cache-Control", fmt.Sprintf("max-age=%d", maxAge))
		if err := put(keys, config.Bucket+deployPath, body, header); err != nil {
			log.Fatal(err)
		}
	}

	for _, o := range objects {
		if o.keep {
			log.Printf("SAVE   /%s", o.Key)
			continue
		}
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

func put(keys s3.Keys, path string, body []byte, header http.Header) error {
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
	rc, err := get(keys, bucket)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var contents struct {
		IsTruncated bool
		Contents    []*object
	}
	if err := xml.NewDecoder(rc).Decode(&contents); err != nil {
		return nil, fmt.Errorf("error reading bucket, %v", err)
	}
	if contents.IsTruncated {
		return nil, fmt.Errorf("bucket contents truncated")
	}
	objects := make(map[string]*object, len(contents.Contents))
	for _, o := range contents.Contents {
		objects[o.Key] = o
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
	if config.MaxAgeVersioned == 0 {
		config.MaxAgeVersioned = 24 * 60 * 60
	}
	return &config, nil
}

func markKeepers(objects map[string]*object, maxAge int) {
	keepers := make(map[string]*object)
	cutoff := time.Now().Add(-2 * time.Duration(maxAge) * time.Second)
	for _, o := range objects {
		m := deployedVersionPat.FindStringIndex(o.Key)
		if m == nil {
			continue
		}
		if o.LastModified.After(cutoff) {
			o.keep = true
			continue
		}
		key := o.Key[:m[0]] + o.Key[m[0]+9:]
		if oo := keepers[key]; oo == nil || oo.LastModified.Before(o.LastModified) {
			keepers[key] = o
		}
	}
	for _, o := range keepers {
		o.keep = true
	}
}
