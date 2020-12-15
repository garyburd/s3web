package action

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var parserTests = []struct {
	doc     string
	actions []string
}{
	{
		`
        this is the first line
        and the second line
        <%a 
        foo="bar"%>
        <% b quux="b" quux+="a" quux += "z"  %>
        and the fourth line
        `,
		[]string{
			`"this is the first line\nand the second line\n"`,
			`x:3:3:a x:4:5:foo="bar"`,
			`"\n"`,
			`x:5:4:b x:5:11:quux="baz"`,
			`"\nand the fourth line\n"`,
		},
	},
	{
		`
        <%a
        e=0
        `,
		nil,
	},
}

var leadingWSPat = regexp.MustCompile(`^\s*`)

func cleanDoc(s string) []byte {
	if s[0] == '\n' {
		s = s[1:]
	}
	prefix := leadingWSPat.FindString(s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, prefix)
	}
	return []byte(strings.Join(lines, "\n"))
}

func printAction(lc *LocationContext, a *Action) string {
	if a.Name == TextAction {
		return fmt.Sprintf("%q", a.Text)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s:%s", a.Location(lc), a.Name)

	var names []string
	for name := range a.Args {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		v := a.Args[name]
		fmt.Fprintf(&buf, " %s:%s=%q", v.Location(lc), name, v.Text)
	}
	return buf.String()
}

func TestParse(t *testing.T) {
	for i, tt := range parserTests {
		doc := cleanDoc(tt.doc)
		actions, lc, err := Parse(doc, "x")

		if (tt.actions == nil) != (err != nil) {
			t.Errorf("test %d, got err %v, want err %v", i, err, tt.actions == nil)
			continue
		}

		if err != nil {
			continue
		}

		n := len(tt.actions)
		if len(actions) != len(tt.actions) {
			t.Errorf("test %d -  got %d, expected %d", i, len(actions), len(tt.actions))
			if len(actions) > n {
				n = len(actions)
			}
		}

		for j := 0; j < n; j++ {
			var got, want string
			if j < len(actions) {
				got = printAction(lc, actions[j])
			}
			if j < len(tt.actions) {
				want = tt.actions[j]
			}
			if got != want {
				t.Errorf("test %d action %d\n got %s\nwant %s", i, j, got, want)
			}
		}
	}
}
