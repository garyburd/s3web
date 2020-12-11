// Package action parses action from text.
//
// A action consists of <%, optional whitespace, a action name, zero or more
// arguments, optional whitespace and a %>.
//
// A action name consists of a letter followed by zero or more letters,
// digits, hyphens or colons.
//
// An argument consists of whitespace, an argument name, and an optional
// argument value specification.
//
// An argument name consists of a letter or  _, followed by zero or more
// letters, digits, _, ., :, or -.
//
// An argument value specification consists of optional whitespace,a =
// character, optional whitespace, and an argument value.
//
// An argument value consists of an unquoted argument value, a single-quoted
// argument value,or a double-quoted argument value.
//
// An unquoted argument value is a nonempty string of characters not including
// whitespace, ", ', `, =, or any character in an action delimiter.
//
// A single-quoted argument value consists of ', zero or more characters not
// including ', and a final '.
//
// A double-quoted argument value consists of ", zero or more characters not
// including ", and a final ".
//
// Argument values are unescaped using HTML rules.
package action

import (
	"bytes"
	"fmt"
)

const TextAction = "__text__"

type Action struct {
	Name string
	Args map[string]Value
	Text []byte // set for TextAction
	pos  int
}

type Value struct {
	Text string
	pos  int
}

type LocationContext struct {
	fpath string
	input []byte
}

func (a *Action) Location(lc *LocationContext) string {
	return loc(lc.fpath, lc.input, a.pos)
}

func (v Value) Location(lc *LocationContext) string {
	return loc(lc.fpath, lc.input, v.pos)
}

func loc(fpath string, input []byte, pos int) string {
	s := input[:pos]

	col := bytes.LastIndex(s, []byte{'\n'})
	if col == -1 {
		col = pos // first line
	} else {
		col = pos - col
	}

	line := 1 + bytes.Count(s, []byte{'\n'})
	return fmt.Sprintf("%s:%d:%d", fpath, line, col)
}
