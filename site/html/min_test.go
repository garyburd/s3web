package html

import (
	"strings"
	"testing"
)

var src1 = `
<!doctype html>
<!-- delete me -->
<head>
  <meta charset="utf-8">
  <meta name='description' content='this is a quote: "'>

  <title>Sample document</title>
  <style>
  body {

    /* No change expected here. */

  }
  </style>
  <script>

    // Don't touch anything here.

  </script>
  <SCRIPT>

    Another script

  </script
</head>
<body>
   <!-- another comment to delete -->

    <p>
    No is the time
        for <b>all</b> good men.

    </p>

    <p><code>  x  </code>
    <pre>
        Preformated

        Text
    </pre>

   <img alt="" width="100" src="foo&amp;bar.html"
   <footer>
	   Copyright &copy;  Author
   </footer>
</body>
</html>`

var want1 = `<!doctype html>
<head>
<meta charset=utf-8>
<meta name=description content="this is a quote: &#34;">
<title>Sample document</title>
<style>
  body {

    /* No change expected here. */

  }
  </style>
<script>

    // Don't touch anything here.

  </script>
<script>

    Another script

  </script>
<body>
<p>
No is the time
for <b>all</b> good men.
</p>
<p><code>  x  </code>
<pre>
        Preformated

        Text
    </pre>
<img alt="" width=100 src=foo&bar.html <footer="">
Copyright &copy; Author
</footer>
</body>
</html>
`

var minTests = []struct {
	src, want string
}{
	{src1, want1},
}

func TestMin(t *testing.T) {
	for i, tt := range minTests {
		got, err := Minify([]byte(tt.src))
		if err != nil {
			t.Errorf("%d: err = %v", i, err)
			continue
		}

		wantLines := strings.Split(tt.want, "\n")
		gotLines := strings.Split(string(got), "\n")

		n := len(wantLines)
		if len(gotLines) > n {
			n = len(gotLines)
		}

		for j := 0; j < n; j++ {
			var gotLine, wantLine string
			if j < len(gotLines) {
				gotLine = gotLines[j]
			}
			if j < len(wantLines) {
				wantLine = wantLines[j]
			}
			if gotLine != wantLine {
				t.Errorf("test %d, line %d\n got=%q\nwant=%q\n\n%s", i, j, gotLine, wantLine, got)
			}
		}
	}
}
