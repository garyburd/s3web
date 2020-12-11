package site

import (
	"errors"
	"fmt"
	htemplate "html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

func (site *site) templateFuncs() map[string]interface{} {
	image := imageFuncs{site}
	page := pageFuncs{site}
	time := timeFuncs{time.Now()} // snap time once for consitency across pages.
	return map[string]interface{}{
		"image":   func() imageFuncs { return image },
		"page":    func() pageFuncs { return page },
		"path":    func() pathFuncs { return pathFuncs{} },
		"strings": func() stringFuncs { return stringFuncs{} },
		"time":    func() timeFuncs { return time },
		"util":    func() utilFuncs { return utilFuncs{} },
	}
}

type stringFuncs struct{}

func (stringFuncs) TrimPrefix(s, prefix string) string   { return strings.TrimPrefix(s, prefix) }
func (stringFuncs) TrimSuffix(s, suffix string) string   { return strings.TrimPrefix(s, suffix) }
func (stringFuncs) TrimSpace(s string) string            { return strings.TrimSpace(s) }
func (stringFuncs) ReplaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

type pathFuncs struct{}

func (pathFuncs) Base(p string) string        { return path.Base(p) }
func (pathFuncs) Dir(p string) string         { return path.Dir(p) }
func (pathFuncs) Join(elems ...string) string { return path.Join(elems...) }

type timeFuncs struct{ now time.Time }

func (tf timeFuncs) Now() time.Time { return tf.now }

type utilFuncs struct{}

func (utilFuncs) Slice(values ...interface{}) []interface{} { return values }

func (utilFuncs) Map(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: must have even number of arguments")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key %v is not a string", values[i])
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}

type imageFuncs struct{ site *site }

type Image struct {
	Width  int
	Height int
	Src    string
}

func (img *Image) SrcWidthHeight() htemplate.HTMLAttr {
	return htemplate.HTMLAttr(fmt.Sprintf(`src="%s" width="%d" height="%d"`, img.Src, img.Width, img.Height))
}

func (f imageFuncs) Read(pageDir string, upath string) (*Image, error) {
	fpath := f.site.filePath(StaticDir, pageDir, upath)
	config, err := readImageConfig(fpath)
	return &Image{Src: upath, Width: config.Width, Height: config.Height}, err
}

type ImageSrcSet struct {
	Image
	SrcSet string
}

func (f imageFuncs) ReadSrcSet(pageDir string, upattern string, maxWidth int, maxHeight int) (*ImageSrcSet, error) {
	fpaths, upaths, err := f.site.fileGlob(StaticDir, pageDir, upattern)
	if err != nil {
		return nil, err
	}

	if len(fpaths) == 0 {
		return nil, fmt.Errorf("no images found for %s (%s)", upattern, f.site.filePath(StaticDir, pageDir, upattern))
	}

	configs := make([]image.Config, len(fpaths))
	for i, fpath := range fpaths {
		config, err := readImageConfig(fpath)
		if err != nil {
			return nil, err
		}
		configs[i] = config
	}

	return computeSrcSet(fpaths, upaths, configs, maxWidth, maxHeight)
}

func computeSrcSet(fpaths []string, upaths []string, configs []image.Config, maxWidth int, maxHeight int) (*ImageSrcSet, error) {

	if maxWidth <= 0 {
		return nil, errors.New("imageSrcSet: maxWidth must be greater than zero")
	}

	bestIndex := 0
	bestScale := math.MaxFloat64
	var buf strings.Builder

	for i, config := range configs {
		if config.Width <= 0 || config.Height <= 0 {
			return nil, fmt.Errorf("imageSrcSet: image %s no width or height", fpaths[i])
		}

		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%s %dw", upaths[i], config.Width)

		scale := float64(maxWidth) / float64(config.Width)
		if maxHeight > 0 {
			scaleH := float64(maxHeight) / float64(config.Height)
			if scaleH <= 1 && scaleH < scale {
				scale = scaleH
			}
		}

		if (bestScale > 1 && scale < bestScale) ||
			(bestScale < 1 && scale > bestScale && scale <= 1) {
			bestIndex = i
			bestScale = scale
		}
	}

	// Don't stretch undersize image.
	if bestScale > 1 {
		bestScale = 1
	}

	return &ImageSrcSet{SrcSet: buf.String(),
		Image: Image{
			Src:    upaths[bestIndex],
			Width:  int(float64(configs[bestIndex].Width) * bestScale),
			Height: int(float64(configs[bestIndex].Height) * bestScale),
		}}, nil
}

func readImageConfig(fpath string) (image.Config, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		err = fmt.Errorf("error reading %s: %w", fpath, err)
	}
	return config, err
}

type pageFuncs struct{ site *site }
type pageOption func(*pageOptions)
type pageOptions struct {
	lessFn  func(a, b *Page) bool
	reverse bool
	limit   int
}

func (pf pageFuncs) Read(pageDir string, upath string) (*Page, error) {
	// TODO: require current page path as prefox of upath.
	if strings.HasSuffix(upath, "/") {
		upath += "index.html"
	}

	p := pf.site.pages[upath]
	if p == nil {
		return nil, fmt.Errorf("page %q not found", upath)
	}

	pCopy := *p
	pCopy.Path = upath
	return &pCopy, nil
}

var pageLessFuncs = map[string]func(a, b *Page) bool{
	"created": func(a, b *Page) bool { return a.Created.Before(b.Created) },
}

func (pf pageFuncs) Limit(n int) pageOption {
	return func(o *pageOptions) { o.limit = n }

}
func (pf pageFuncs) Sort(field string) (pageOption, error) {
	if field == "" {
		return nil, nil
	}

	reverse := false
	if strings.HasPrefix(field, "-") {
		reverse = true
		field = field[1:]
	}
	fn := pageLessFuncs[field]
	if fn == nil {
		return nil, fmt.Errorf("sort by %q not supported", field)
	}
	return func(o *pageOptions) {
		o.lessFn = fn
		o.reverse = reverse
	}, nil
}

func (pf pageFuncs) Glob(pageDir string, upattern string, options ...pageOption) ([]*Page, error) {

	var o pageOptions
	for _, fn := range options {
		if fn != nil {
			fn(&o)
		}
	}

	if strings.HasSuffix(upattern, "/") {
		upattern += "index.html"
	}

	if !strings.HasPrefix(upattern, "/") {
		upattern = path.Join(pageDir, upattern)
	}

	// TODO: require current page directory as prefix of upattern.

	var pages []*Page
	for upath, page := range pf.site.pages {
		matched, err := path.Match(upattern, upath)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		pages = append(pages, page)
	}

	if o.lessFn != nil {
		var lessFn func(a, b int) bool
		if o.reverse {
			lessFn = func(a, b int) bool { return o.lessFn(pages[b], pages[a]) }
		} else {
			lessFn = func(a, b int) bool { return o.lessFn(pages[a], pages[b]) }
		}
		sort.Slice(pages, lessFn)
	}

	if o.limit > 0 {
		if o.limit < len(pages) {
			pages = pages[:o.limit]
		}
	} else if o.limit < 0 {
		if o.limit < len(pages) {
			pages = pages[len(pages)-o.limit:]
		}
	}

	// Copy pages so we can scribble on the page path.
	pageCopies := make([]Page, len(pages))
	for i, p := range pages {
		pageCopies[i] = *p
		pageCopies[i].Path = shortPath(pageDir, p.Path)
		pages[i] = &pageCopies[i]
	}

	return pages, nil
}
