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
	fmtcmd "github.com/garyburd/staticsite/fmt"
	"github.com/garyburd/staticsite/s3"
	"github.com/garyburd/staticsite/serve"
	"github.com/garyburd/staticsite/site"
)

var tools = []*site.Tool{
	serve.Tool,
	s3.Tool,
	check.Tool,
	fmtcmd.Tool,
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
	for _, t := range tools {
		if args[0] == t.Name {
			t.FlagSet.Usage = func() {
				if t.Help != "" {
					log.Print(strings.TrimSpace(t.Help))
					log.Print("\n\n")
				}
				log.Println(t.Usage)
				t.FlagSet.PrintDefaults()
				os.Exit(2)
			}
			t.FlagSet.Parse(args[1:])
			t.Run()
			return
		}
	}
	flag.Usage()
}

func printUsage() {
	var names []string
	for _, t := range tools {
		names = append(names, t.Name)
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", os.Args[0], strings.Join(names, " | "))
	flag.PrintDefaults()
}
