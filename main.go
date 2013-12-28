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

package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kr/s3"
)

var (
	rootDir     string
	httpAddr    = flag.String("http", ":8080", "serve locally at this address")
	dryRun      = flag.Bool("n", false, "Dry run.  Don't upload or delete during deploy")
	errNotFound = errors.New("s3web: file ignored")
	errorKey    string
	bucket      string
	accessKey   string
)

func readConfig() {
	p, err := ioutil.ReadFile(filepath.Join(rootDir, "_config.txt"))
	if err != nil {
		log.Fatalf("Error reading configuration, %v", err)
	}
	fm := parseFrontMatter(p)
	if fm == nil {
		log.Fatalf("Error reading configuration file")
	}
	for k, v := range fm {
		switch k {
		case "error":
			errorKey = v
		case "bucket":
			bucket = v
			if !strings.HasSuffix(bucket, "/") {
				bucket += "/"
			}
		case "accessKey":
			accessKey = v
		default:
			log.Fatal("Unknown configuration key, %s", k)
		}
	}
}

var passRegexp = regexp.MustCompile(`password: "([^"]+)"`)

func getSecret() (string, error) {
	c := exec.Command("/usr/bin/security",
		"find-generic-password",
		"-g",
		"-s", "aws",
		"-a", accessKey)
	var b bytes.Buffer
	c.Stderr = &b
	err := c.Run()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok && !err.Success() {
			return "", errors.New(b.String())
		}
		return "", err
	}
	match := passRegexp.FindSubmatch(b.Bytes())
	if match == nil {
		return "", errors.New("Password not found")
	}
	return string(match[1]), nil
}

func setSecret(secret string) error {
	c := exec.Command("/usr/bin/security",
		"add-generic-password",
		"-U",
		"-s", "aws",
		"-a", accessKey,
		"-p", secret)
	var b bytes.Buffer
	c.Stderr = &b
	err := c.Run()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok && !err.Success() {
			return errors.New(b.String())
		}
		return err
	}
	return nil
}

func runSecret() {
	io.WriteString(os.Stdout, "Secret: ")
	r := bufio.NewReader(os.Stdin)
	b, isPrefix, err := r.ReadLine()
	if err != nil {
		log.Fatal(err)
	}
	if isPrefix {
		log.Fatal("Long line")
	}
	err = setSecret(string(b))
	if err != nil {
		log.Fatalf("Error saving password, %v", err)
	}
}

type resource struct {
	data   []byte
	header http.Header
}

var compressTypes = map[string]bool{
	"application/javascript": true,
	"text/css":               true,
}

func readResource(key string) (*resource, error) {
	fname := filepath.Join(rootDir, filepath.FromSlash(key))
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, errNotFound
	}
	data, err = evalPage(fname, data)
	if err != nil {
		return nil, err
	}

	mimeType := mime.TypeByExtension(path.Ext(fname))
	if mimeType == "" {
		mimeType = "text/html"
	}

	encoding := "identity"
	if compressTypes[strings.Split(mimeType, ";")[0]] {
		var buf bytes.Buffer
		gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		gzw.Write(data)
		gzw.Close()
		data = buf.Bytes()
		encoding = "gzip"
	}

	return &resource{data: data,
		header: http.Header{
			"Content-Type":     {mimeType},
			"Content-Length":   {strconv.Itoa(len(data))},
			"Content-Encoding": {encoding},
		}}, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	switch {
	case key == "":
		key = "index.html"
	case key[len(key)-1] == '/':
		key += "index.html"
	}
	status := 200
	rsrc, err := readResource(key)
	if err == errNotFound && errorKey != "" {
		status = 404
		rsrc, err = readResource(errorKey)
	}
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	for k, v := range rsrc.header {
		w.Header()[k] = v
	}
	w.WriteHeader(status)
	w.Write(rsrc.data)
}

func runTest() {
	s := http.Server{
		Addr:    *httpAddr,
		Handler: http.HandlerFunc(handler),
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Server return error %v", err)
	}
}

func runPush() {
	secret, err := getSecret()
	if err != nil {
		log.Fatalf("Error reading secret, %v", err)
	}
	keys := s3.Keys{AccessKey: accessKey, SecretKey: secret}

	// Get bucket list.

	req, _ := http.NewRequest("GET", bucket, nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	s3.Sign(req, keys)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

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

	err = filepath.Walk(rootDir, func(fname string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if _, n := filepath.Split(fname); n[0] == '.' || n[0] == '_' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		key := filepath.ToSlash(fname[1+len(rootDir):])
		rsrc, err := readResource(key)
		if err != nil {
			if err == errNotFound {
				delete(sums, key)
			} else {
				log.Println(err)
			}
			return nil
		}

		h := md5.New()
		h.Write(rsrc.data)
		sum := hex.EncodeToString(h.Sum(nil))

		if sums[key] != sum {
			log.Printf("Uploading %s %v", key, rsrc.header)
			if !*dryRun {
				req, _ := http.NewRequest("PUT", bucket+key, bytes.NewReader(rsrc.data))
				req.ContentLength = int64(len(rsrc.data))
				req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
				req.Header.Set("X-Amz-Acl", "public-read")
				for k, v := range rsrc.header {
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
			log.Printf("Skipping  %s", key)
		}
		delete(sums, key)
		return nil
	})
	if err != nil {
		log.Fatalf("Error uploading, %v", err)
	}
	for key := range sums {
		log.Printf("Deleteing %s", key)
		if !*dryRun {
			req, _ := http.NewRequest("DELETE", bucket+key, nil)
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

func usage() {
	fmt.Fprintf(os.Stderr, "usage: s3web dir test|push|secret\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 2 {
		usage()
	}
	var err error
	rootDir, err = filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	rootDir = filepath.Clean(rootDir)
	readConfig()
	switch flag.Arg(1) {
	case "secret":
		runSecret()
	case "test":
		runTest()
	case "push":
		runPush()
	default:
		usage()
	}
}
