// Copyright 2013 Gary Burd
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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"text/template"
)

var (
	nl         = []byte{'\n'}
	tab        = []byte{'\t'}
	fourSpaces = []byte{' ', ' ', ' ', ' '}
	omitNl     = []byte("OMIT\n")
)

func (ctx *pageContext) Code(fname string, patterns ...string) (string, error) {
	p, err := ioutil.ReadFile(ctx.resolvePath(fname))
	if err == nil {
		switch len(patterns) {
		case 0:
			// nothing to do
		case 1:
			p, err = oneLine(p, patterns[0])
		case 2:
			p, err = multipleLines(p, patterns[0], patterns[1])
		default:
			err = errors.New("> 2 arguments")
		}
	}
	if err != nil {
		return "", fmt.Errorf("Code %q %v, %v", fname, patterns, err)
	}
	p = bytes.TrimSuffix(p, nl)
	p = bytes.Replace(p, tab, fourSpaces, -1)
	var buf bytes.Buffer
	buf.WriteString("<pre>")
	template.HTMLEscape(&buf, p)
	buf.WriteString("</pre>")
	return buf.String(), nil
}

func oneLine(p []byte, pattern string) ([]byte, error) {
	lines := bytes.SplitAfter(p, nl)
	i, err := match(lines, pattern)
	if err != nil {
		return nil, err
	}
	return lines[i], nil
}

func multipleLines(p []byte, pattern1, pattern2 string) ([]byte, error) {
	lines := bytes.SplitAfter(p, nl)
	line1, err := match(lines, pattern1)
	if err != nil {
		return nil, err
	}
	line2, err := match(lines[line1:], pattern2)
	if err != nil {
		return nil, err
	}
	line2 += line1
	for i := line1; i <= line2; i++ {
		if bytes.HasSuffix(lines[i], omitNl) {
			lines[i] = []byte{}
		}
	}
	return bytes.Join(lines[line1:line2+1], []byte{}), nil
}

func match(lines [][]byte, pattern string) (int, error) {
	if len(pattern) < 2 || pattern[0] != '/' || pattern[len(pattern)-1] != '/' {
		return 0, fmt.Errorf("invalid pattern %q", pattern)
	}
	re, err := regexp.Compile(pattern[1 : len(pattern)-1])
	if err != nil {
		return 0, err
	}
	for i, line := range lines {
		if re.Match(line) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("pattern %q not found", pattern)
}
