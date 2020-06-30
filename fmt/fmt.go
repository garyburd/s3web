// Copyright 2020 Gary Burd
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

package fmt

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/garyburd/staticsite/site"
)

const help = `
fmt formats front matter.

Without an explicit path, it processes the standard input. Given a file, it
operates on that file. By default, fmt prints the reformatted sources to
standard output.
`

var (
	flagSet = flag.NewFlagSet("fmt", flag.ExitOnError)
	write   = flagSet.Bool("w", false, `Do not print reformatted sources `+
		`to standard output. If a file's formatting is different from `+
		`fmt's, overwrite it with fmt's version.`)
	Command = &site.Command{
		Name:    "fmt",
		Usage:   "fmt [path...]",
		FlagSet: flagSet,
		Run:     run,
		Help:    help,
	}
)

func run() {
	if flagSet.NArg() == 0 {
		src, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(format(src))
	} else {
		for _, arg := range flagSet.Args() {
			src, perm, err := readFile(arg)
			if err != nil {
				log.Print(err)
				continue
			}
			dst := format(src)
			if *write {
				if !bytes.Equal(src, dst) {
					err := ioutil.WriteFile(arg, dst, perm)
					if err != nil {
						log.Println(err)
					}
				}
			} else {
				os.Stdout.Write(dst)
			}
		}
	}
}

func format(src []byte) []byte {
	i := site.FrontMatterEnd(src)
	if i < 0 {
		return src
	}
	var dst bytes.Buffer
	if err := json.Indent(&dst, src[:i], "", "  "); err != nil {
		return src
	}
	dst.Write(src[i:])
	return dst.Bytes()
}

func readFile(name string) ([]byte, os.FileMode, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	perm := fi.Mode().Perm()

	n := fi.Size()
	if n == 0 {
		return nil, perm, nil
	}

	var buf bytes.Buffer
	if int64(int(n)) == n {
		buf.Grow(int(n))
	}
	_, err = buf.ReadFrom(f)
	return buf.Bytes(), perm, err
}
