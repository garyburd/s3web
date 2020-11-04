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

package check

import (
	"flag"
	"log"

	"github.com/garyburd/staticsite/site"
)

var (
	flagSet = flag.NewFlagSet("check", flag.ExitOnError)
	Tool    = &site.Tool{
		Name:    "check",
		Usage:   "check [directory]",
		FlagSet: flagSet,
		Run:     run,
	}
)

func run() {
	err := site.Walk(flagSet.Arg(0), func(r *site.Resource) error { return nil })
	if err != nil {
		log.Fatal(err)
	}
}
