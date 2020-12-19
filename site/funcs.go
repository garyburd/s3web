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
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/garyburd/staticsite/common"
)

func (site *site) templateFuncs() map[string]interface{} {
	static := staticFuncs{site}
	page := pageFuncs{site}
	time := timeFuncs{time.Now()} // snap time once for consitency across pages.
	return map[string]interface{}{
		"static":  func() staticFuncs { return static },
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

func (utilFuncs) SliceValues(slice interface{}) ([]*sliceElement, error) {
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice, got %T", slice)
	}
	result := make([]*sliceElement, v.Len())
	for i := range result {
		result[i] = &sliceElement{v, i}
	}
	return result, nil
}

type sliceElement struct {
	v reflect.Value
	i int
}

func (se *sliceElement) Value() interface{} {
	return se.v.Index(se.i).Interface()
}

func (se *sliceElement) Index() int {
	return se.i
}

func (se *sliceElement) First() bool {
	return se.i == 0
}

func (se *sliceElement) Last() bool {
	return se.i == se.v.Len()-1
}

func (se *sliceElement) Even() bool {
	return se.i%2 == 0
}

func (se *sliceElement) Odd() bool {
	return se.i%2 != 0
}

func (se *sliceElement) Previous() interface{} {
	if se.i <= 0 {
		return nil
	}
	return se.v.Index(se.i - 1).Interface()
}

func (se *sliceElement) Next() interface{} {
	if se.i+1 >= se.v.Len() {
		return nil
	}
	return se.v.Index(se.i + 1).Interface()
}

type staticFuncs struct{ site *site }

func (sf staticFuncs) VersionedPath(pageDir string, upath string) (string, error) {
	fpath := sf.site.filePath(common.StaticDir, absPath(upath, upath))
	h, err := sf.site.getFileHash(fpath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s?v=%s", upath, h), nil
}

type Image struct {
	Width  int
	Height int
	Src    string
}

func (img *Image) SrcWidthHeight() htemplate.HTMLAttr {
	return htemplate.HTMLAttr(fmt.Sprintf(`src="%s" width="%d" height="%d"`, img.Src, img.Width, img.Height))
}

func (sf staticFuncs) ReadImage(upage string, upath string) (*Image, error) {
	fpath := sf.site.filePath(common.StaticDir, absPath(upage, upath))
	config, err := readImageConfig(fpath)
	return &Image{Src: upath, Width: config.Width, Height: config.Height}, err
}

type ImageSrcSet struct {
	Image
	SrcSet string
}

func (sf staticFuncs) ReadImageSrcSet(upage string, upattern string, maxWidth int, maxHeight int) (*ImageSrcSet, error) {
	fpaths, upaths, err := sf.site.fileGlob(common.StaticDir, absPath(upage, upattern))
	if err != nil {
		return nil, err
	}

	if len(fpaths) == 0 {
		return nil, fmt.Errorf("no images found for %s (%s)", upattern, sf.site.filePath(common.StaticDir, absPath(upage, upattern)))
	}

	configs := make([]image.Config, len(fpaths))
	for i, fpath := range fpaths {
		config, err := readImageConfig(fpath)
		if err != nil {
			return nil, err
		}
		configs[i] = config
		upaths[i] = shortPath(upage, upaths[i])
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

type tempPage struct {
	*Page
	Path string
}

func (pf pageFuncs) Read(upage string, upath string) (*tempPage, error) {
	// TODO: check for valid upaath

	p := pf.site.getPage(absPath(upage, upath))
	if p == nil {
		return nil, fmt.Errorf("page %q not found", upath)
	}

	return &tempPage{Page: p, Path: upath}, nil
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

func (pf pageFuncs) Glob(upage string, upattern string, options ...pageOption) ([]*tempPage, error) {
	// TODO: check for valid pattern.

	var o pageOptions
	for _, fn := range options {
		if fn != nil {
			fn(&o)
		}
	}

	upattern = absPath(upage, upattern)
	pages, err := pf.site.globPages(upattern)
	if err != nil {
		return nil, err
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

	result := make([]*tempPage, len(pages))
	for i, p := range pages {
		result[i] = &tempPage{Page: p, Path: shortPath(upage, p.Path)}
	}

	return result, nil
}

func absPath(upage string, upath string) string {
	if strings.HasPrefix(upath, "/") {
		return upath
	}
	slash := strings.HasSuffix(upath, "/")
	upath = path.Join(path.Dir(upage), upath)
	if slash {
		upath += "/"
	}
	return upath
}
