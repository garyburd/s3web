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

	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
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

func readResource(key string) ([]byte, string, error) {
	fname := filepath.Join(rootDir, filepath.FromSlash(key))
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, "", errNotFound
	}
	b, err = evalPage(fname, b)
	if err != nil {
		return nil, "", err
	}
	mimeType := mime.TypeByExtension(path.Ext(fname))
	if mimeType == "" {
		mimeType = "text/html; charset=utf-8"
	}
	return b, mimeType, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	if key == "" {
		key = "index.html"
	}
	status := 200
	b, mimeType, err := readResource(key)
	if err == errNotFound && errorKey != "" {
		status = 404
		b, mimeType, err = readResource(errorKey)
	}
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	w.Write(b)
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
	bucket := s3.New(aws.Auth{accessKey, secret}, aws.USEast).Bucket(bucket)

	// Get etags of items in bucket

	r, err := bucket.GetReader("/")
	if err != nil {
		log.Fatalf("Error reading bucket, %v", err)
	}
	defer r.Close()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatalf("Error reading bucket, %v", err)
	}
	var contents struct {
		Contents []struct {
			Key  string
			ETag string
		}
	}
	err = xml.Unmarshal(data, &contents)
	if err != nil {
		log.Fatalf("Error reading bucket, %v", err)
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
		data, mimeType, err := readResource(key)
		if err != nil {
			if err == errNotFound {
				delete(sums, key)
			} else {
				log.Println(err)
			}
			return nil
		}

		h := md5.New()
		h.Write(data)
		sum := hex.EncodeToString(h.Sum(nil))

		if sums[key] != sum {
			log.Printf("Uploading %s %s %d", key, mimeType, len(data))
			if !*dryRun {
				err = bucket.Put("/"+key, data, mimeType, s3.PublicRead)
				if err != nil {
					return err
				}
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
			err = bucket.Del("/" + key)
			if err != nil {
				log.Fatalf("Error deleting, %v", err)
			}
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
