package site

// Utilities for tools.

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
)

const (
	ConfigDir = "config"
	LayoutDir = "layout"
	PageDir   = "content"
	StaticDir = "static"
)

type Tool struct {
	Name    string
	FlagSet *flag.FlagSet
	Usage   string
	Run     func()
	Help    string
}

var Verbose bool

func DecodeConfigFile(fpath string, v interface{}) error {
	p, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(p))
	d.DisallowUnknownFields()
	err = d.Decode(v)
	var se *json.SyntaxError
	if errors.As(err, &se) {
		offset := int(se.Offset)
		return fmt.Errorf("%s:%d: %w", fpath, bytes.Count(p[:offset+1], []byte("\n"))+1, err)
	} else if err != nil {
		return fmt.Errorf("%s:1: %w", fpath, err)
	}
	return nil
}
