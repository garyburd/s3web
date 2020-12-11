package template

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

var templateTests = []struct {
	name           string
	wantParseError bool
}{
	{name: "example.html"},
	{name: "cycle1.html", wantParseError: true},
}

var data = map[string]interface{}{
	"Hello": "World",
}

func TestTemplate(t *testing.T) {
	l, err := NewLoader("testdata/in", nil)
	if err != nil {
		t.Fatalf("NewManager returned error %v", err)
	}

	for _, tt := range templateTests {
		templ, err := l.Get(tt.name)
		if (err != nil) != tt.wantParseError {
			t.Errorf("Parse(%q) returned error %v, want error = %v", tt.name, err, tt.wantParseError)
			continue
		}

		if tt.wantParseError {
			continue
		}

		var got bytes.Buffer
		err = templ.Execute(&got, data)
		if err != nil {
			t.Errorf("Execute(%q, nil) returned error %v", tt.name, err)
			continue
		}
		want, err := ioutil.ReadFile(filepath.Join("testdata/out", tt.name))
		if err != nil {
			t.Errorf("ReadFile returned error %v", err)
			continue
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("Execute(%q) got:\n%s\nwant:\n%s", tt.name, got.Bytes(), want)
		}
	}
}
