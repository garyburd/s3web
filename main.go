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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/garyburd/staticsite/check"
	"github.com/garyburd/staticsite/s3"
	"github.com/garyburd/staticsite/serve"
	"github.com/garyburd/staticsite/site"
)

var commands = []*site.Command{
	serve.Command,
	s3.Command,
	check.Command,
}

func main() {
	log.SetFlags(0)
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return
	}
	for _, c := range commands {
		if args[0] == c.Name {
			c.FlagSet.Usage = func() {
				log.Println(c.Usage)
				c.FlagSet.PrintDefaults()
				os.Exit(2)
			}
			c.FlagSet.Parse(args[1:])
			c.Run()
			return
		}
	}
	flag.Usage()
}

func printUsage() {
	var names []string
	for _, c := range commands {
		names = append(names, c.Name)
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", os.Args[0], strings.Join(names, " | "))
	flag.PrintDefaults()
}
