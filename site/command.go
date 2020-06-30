// Copyright 2019 Gary Burd
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

package site

// Utilities for commands.

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"regexp"
)

const ConfigDir = "/_config"

type Command struct {
	Name    string
	FlagSet *flag.FlagSet
	Usage   string
	Run     func()
	Help    string
}

func DecodeConfig(fpath string, data []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	err := d.Decode(v)
	var se *json.SyntaxError
	if errors.As(err, &se) {
		offset := int(se.Offset)
		return fmt.Errorf("%s:%d: %w", fpath, bytes.Count(data[:offset+1], []byte("\n"))+1, err)
	} else if err != nil {
		return fmt.Errorf("%s:1: %w", fpath, err)
	}
	return nil
}

var (
	frontStart = regexp.MustCompile(`(?m)\A{\s*`)
	frontEnd   = regexp.MustCompile(`(?m)^}\s*$`)
)

func FrontMatterEnd(data []byte) int {
	if m := frontStart.FindIndex(data); m == nil {
		return -1
	}
	m := frontEnd.FindIndex(data)
	if m == nil {
		return -1
	}
	return m[1]
}
