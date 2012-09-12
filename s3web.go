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
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/garyburd/indigo/server"
	"github.com/garyburd/indigo/web"
	"io"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"mime"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	rootDir      string
	httpAddr     = flag.String("http", ":8080", "serve locally at this address")
	dryRun       = flag.Bool("n", false, "Dry run.  Don't upload or delete during deploy")
	ignoredError = errors.New("s3web: file ignored")
)

var config = struct {
	Index       string
	Error       string
	Bucket      string
	AccessKey   string
	DefaultType string
}{
	DefaultType: "text/html; charset=utf-8",
}

func readConfig() {
	b, err := ioutil.ReadFile(filepath.Join(rootDir, "_config.json"))
	if err != nil {
		log.Fatalf("Error reading configuration, %v", err)
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("Error reading configuration, %v", err)
	}
}

var passRegexp = regexp.MustCompile(`password: "([^"]+)"`)

func getSecret() (string, error) {
	c := exec.Command("/usr/bin/security",
		"find-generic-password",
		"-g",
		"-s", "aws",
		"-a", config.AccessKey)
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
		"-a", config.AccessKey,
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

func readKey(key string) ([]byte, string, error) {
	_, name := path.Split(key)
	if name[0] == '.' || name[0] == '_' {
		return nil, "", ignoredError
	}
	fname := filepath.Join(rootDir, filepath.FromSlash(key))
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, "", err
	}
	mimeType := mime.TypeByExtension(path.Ext(fname))
	if mimeType == "" {
		mimeType = config.DefaultType
	}
	return b, mimeType, nil
}

func rootHandler(resp web.Response, req *web.Request) error {

	key := req.URL.Path[1:]
	if key == "" {
		key = config.Index
	}

	var status = web.StatusOK
	b, mimeType, err := readKey(key)
	if err != nil && config.Error != "" {
		status = web.StatusNotFound
		b, mimeType, err = readKey(config.Error)
	}
	if err != nil {
		return &web.Error{Status: web.StatusNotFound, Reason: err}
	}

	w := resp.Start(status, web.Header{
		web.HeaderContentLength: {strconv.Itoa(len(b))},
		web.HeaderContentType:   {mimeType},
	})
	_, err = w.Write(b)
	return err
}

func runTest() {
	l, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatalf("Could not listen on %s, %v", *httpAddr, err)
		return
	}
	defer l.Close()

	log.Printf("Listening on %s", l.Addr())

	err = (&server.Server{
		Logger:   server.LoggerFunc(server.ShortLogger),
		Listener: l,
		Handler:  web.HandlerFunc(rootHandler)}).Serve()
	if err != nil {
		log.Fatalf("Server return error %v", err)
	}
}

func runPush() {
	secret, err := getSecret()
	if err != nil {
		log.Fatalf("Error reading secret, %v", err)
	}
	bucket := s3.New(aws.Auth{config.AccessKey, secret}, aws.USEast).Bucket(config.Bucket)

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

		if _, name := filepath.Split(fname); name[0] == '.' || name[0] == '_' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		key := filepath.ToSlash(fname[1+len(rootDir):])
		data, mimeType, err := readKey(key)
		if err != nil {
			if err != ignoredError {
				log.Println(err)
			}
			delete(sums, key)
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
