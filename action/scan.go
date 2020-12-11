package action

import (
	"bytes"
	"fmt"
	"html"
	"io/ioutil"
	"unicode"
	"unicode/utf8"
)

const (
	defaultLeftDelim  = "<%"
	defaultRightDelim = "%>"
)

func Parse(input []byte, fpath string) ([]*Action, *LocationContext, error) {
	actions, err := newScanner(input, fpath, "", "").scan()
	return actions, &LocationContext{fpath: fpath, input: input}, err
}

func ParseFile(fpath string) ([]*Action, *LocationContext, error) {
	input, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, nil, err
	}
	return Parse(input, fpath)
}

type scanner struct {
	fpath string
	input []byte
	pos   int

	// action delimiters
	leftDelim  []byte
	rightDelim []byte

	unquoteTerminators []byte
}

func newScanner(input []byte, fpath string, leftDelim, rightDelim string) *scanner {
	if leftDelim == "" {
		leftDelim = defaultLeftDelim
	}
	if rightDelim == "" {
		rightDelim = defaultRightDelim
	}
	unquoteTerminators := []byte(" \t\r\n\"'`=")
	for _, r := range leftDelim {
		if !bytes.ContainsRune(unquoteTerminators, r) {
			unquoteTerminators = append(unquoteTerminators, string(r)...)
		}
	}
	for _, r := range rightDelim {
		if !bytes.ContainsRune(unquoteTerminators, r) {
			unquoteTerminators = append(unquoteTerminators, string(r)...)
		}
	}
	return &scanner{
		fpath:              fpath,
		input:              input,
		leftDelim:          []byte(leftDelim),
		rightDelim:         []byte(rightDelim),
		unquoteTerminators: unquoteTerminators,
	}
}

func (s *scanner) loc(pos int) string {
	return loc(s.fpath, s.input, pos)
}

func (s *scanner) scan() ([]*Action, error) {

	var result []*Action
	for {
		pos := s.pos
		text, more := s.scanText()
		if len(text) > 0 {
			result = append(result, &Action{Name: TextAction, pos: pos, Text: text})
		}
		if !more {
			break
		}
		a, err := s.scanAction()
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

// scanText scans to text to the next action or EOF.
func (s *scanner) scanText() ([]byte, bool) {

	i := bytes.Index(s.input[s.pos:], s.leftDelim)
	if i < 0 {
		val := s.input[s.pos:]
		s.pos = len(s.input)
		return val, false
	}

	val := s.input[s.pos : s.pos+i]
	s.pos += i + len(s.leftDelim)
	return val, true
}

// skipSpace skips ASCII whitespace and returns whether any was skipped.
func (s *scanner) skipSpace() bool {
	ok := false
	for i, b := range s.input[s.pos:] {
		switch b {
		case '\r', '\n', ' ', '\t':
			ok = true
		default:
			s.pos += i
			return ok
		}
	}
	s.pos = len(s.input)
	return ok
}

func (s *scanner) scanAction() (*Action, error) {

	s.skipSpace()
	pos := s.pos

	name, err := s.scanActionName()
	if err != nil {
		return nil, err
	}

	a := &Action{
		pos:  pos,
		Name: name,
		Args: make(map[string]Value),
	}

	for {
		name, done, err := s.scanArgumentName()
		if err != nil {
			return nil, err
		} else if done {
			break
		}

		done, err = s.scanEqual()
		if err != nil {
			return nil, err
		} else if done {
			break
		}

		pos = s.pos

		text, err := s.scanArgumentValue()
		if err != nil {
			return nil, err
		}

		a.Args[name] = Value{Text: text, pos: pos}
	}
	return a, nil
}

func (s *scanner) scanActionName() (string, error) {

	r, w := utf8.DecodeRune(s.input[s.pos:])
	if !isActionNameStart(r) {
		return "", fmt.Errorf("%s: expected start of action name, found %c", s.loc(s.pos), r)
	}

	i := w + s.pos
	for {
		r, w = utf8.DecodeRune(s.input[i:])
		if !isActionName(r) {
			name := s.input[s.pos:i]
			s.pos = i
			return string(name), nil
		}
		i += w
	}
}

func (s *scanner) scanArgumentName() (string, bool, error) {

	pos := s.pos
	skipped := s.skipSpace()

	if bytes.HasPrefix(s.input[s.pos:], s.rightDelim) {
		s.pos += len(s.rightDelim)
		return "", true, nil
	}

	if s.pos >= len(s.input) {
		return "", false, fmt.Errorf("%s: reached EOF looking for argument name", s.loc(pos))
	}

	if !skipped {
		return "", false, fmt.Errorf("%s: expected space before start of argument name", s.loc(pos))
	}

	r, w := utf8.DecodeRune(s.input[s.pos:])
	if !isArgumentNameStart(r) {
		return "", false, fmt.Errorf("%s: expected start of argument name, found %c", s.loc(pos), r)
	}

	i := w + s.pos
	for {
		r, w = utf8.DecodeRune(s.input[i:])
		if !isArgumentName(r) {
			name := s.input[s.pos:i]
			s.pos = i
			return string(name), false, nil
		}
		i += w
	}
}

func (s *scanner) scanEqual() (bool, error) {
	pos := s.pos
	s.skipSpace()

	if bytes.HasPrefix(s.input[s.pos:], s.rightDelim) {
		s.pos += len(s.rightDelim)
		return true, nil
	}

	if s.pos >= len(s.input) {
		return false, fmt.Errorf("%s: reached EOF looking for =", s.loc(pos))
	}

	r, _ := utf8.DecodeRune(s.input[s.pos:])
	if r != '=' {
		return false, fmt.Errorf("%s: expected =, found %c", s.loc(pos), r)
	}

	s.pos += len("=")
	return false, nil
}

func (s *scanner) scanArgumentValue() (string, error) {
	pos := s.pos
	s.skipSpace()

	if s.pos >= len(s.input) {
		return "", fmt.Errorf("%s: reached EOF looking for argument value", s.loc(pos))
	}

	fn := s.scanUnquotedValue
	if b := s.input[s.pos]; b == '\'' || b == '"' {
		fn = s.scanQuotedValue
	}

	val, err := fn()
	return html.UnescapeString(val), err
}

func (s *scanner) scanQuotedValue() (string, error) {
	pos := s.pos

	q := s.input[s.pos]
	s.pos++

	i := bytes.Index(s.input[s.pos:], []byte{q})
	if i < 0 {
		return "", fmt.Errorf("%s: reached EOF looking for close quote %c", s.loc(pos), q)
	}

	val := s.input[s.pos : s.pos+i]
	s.pos += i + 1
	return string(val), nil
}

func (s *scanner) scanUnquotedValue() (string, error) {
	i := s.pos
	for i < len(s.input) {
		r, w := utf8.DecodeRune(s.input[i:])
		if bytes.ContainsRune(s.unquoteTerminators, r) {
			val := s.input[s.pos:i]
			if len(val) == 0 {
				return "", fmt.Errorf("%s: expected value following =, found %c", s.loc(s.pos), r)
			}
			s.pos = i
			return string(val), nil
		}
		i += w
	}
	return "", fmt.Errorf("%s: reached EOF looking for end of value", s.loc(s.pos))
}

func isActionNameStart(r rune) bool {
	return unicode.IsLetter(r)
}

func isActionName(r rune) bool {
	return r == '-' || r == ':' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isArgumentNameStart(r rune) bool {
	return unicode.IsLetter(r)
}

func isArgumentName(r rune) bool {
	return r == '-' || r == ':' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
