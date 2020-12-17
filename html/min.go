package html

import (
	"bytes"
	"io"

	"golang.org/x/net/html"
)

var rawTags = map[string]bool{
	"script":   true,
	"pre":      true,
	"style":    true,
	"code":     true,
	"textarea": true,
}

// Minify returns a mininfied version if the HTML in src.
func Minify(src []byte) ([]byte, error) {

	dst := make([]byte, 0, len(src))
	z := html.NewTokenizer(bytes.NewReader(src))
	raw := 0
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				err = nil
			}
			return dst, err
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			if rawTags[string(name)] {
				raw++
			}
			dst = append(dst, '<')
			dst = append(dst, name...)
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				dst = append(dst, ' ')
				dst = append(dst, k...)
				dst = append(dst, '=')
				if needsQuote(v) {
					dst = append(dst, '"')
					dst = append(dst, html.EscapeString(string(v))...)
					dst = append(dst, '"')
				} else {
					dst = append(dst, v...)
				}
			}
			dst = append(dst, '>')
		case html.EndTagToken:
			name, _ := z.TagName()
			dst = append(dst, "</"...)
			dst = append(dst, name...)
			dst = append(dst, '>')
			if rawTags[string(name)] {
				raw--
			}
		case html.CommentToken:
			// Skip
		case html.TextToken:
			p := z.Raw()
			if raw > 0 {
				dst = append(dst, p...)
			} else {
				dst = appendMinText(dst, p)
			}
		default:
			dst = append(dst, z.Raw()...)
		}
	}
}

func appendMinText(dst []byte, src []byte) []byte {
	emitNL, emitSpace := false, false

	// Backup over previous whitespace.
	if len(dst) > 0 {
		switch dst[len(dst)-1] {
		case '\n':
			emitNL = true
			dst = dst[:len(dst)-1]
		case ' ':
			emitSpace = true
			dst = dst[:len(dst)-1]
		}
	} else {
		// trim wS at beginning of file.
		src = bytes.TrimLeft(src, " \t\\n\r\n")
	}

	for _, b := range src {
		switch b {
		case '\n':
			emitNL = true
		case ' ', '\r', '\t':
			emitSpace = true
		default:
			if emitNL {
				dst = append(dst, '\n')
				emitNL, emitSpace = false, false
			} else if emitSpace {
				dst = append(dst, ' ')
				emitSpace = false
			}
			dst = append(dst, b)
		}
	}
	if emitNL {
		// Prefer \n over other whitespace.
		dst = append(dst, '\n')
	} else if emitSpace {
		dst = append(dst, ' ')
	}
	return dst
}

func needsQuote(v []byte) bool {
	return len(v) == 0 || bytes.IndexAny(v, "\"'`=<> \n\r\t\b") >= 0
}
