package site

import (
	"encoding/json"
	"errors"
	"fmt"
	htemplate "html/template"
	"image"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// functionContext is the context for template functions.
type functionContext struct {
	// The site.
	site *site

	// modTime is maxiumum modification time of any referenced file.
	modTime time.Time

	// Current page URL directory.
	udir string

	// Current page file directory.
	fdir string

	// Current page file name.
	fname string
}

func (fc *functionContext) funcs() htemplate.FuncMap {
	return htemplate.FuncMap{
		"makeSlice": func(v ...interface{}) []interface{} { return v },
		"dict":      dict,

		"pathBase": path.Base,
		"pathDir":  path.Dir,
		"pathJoin": path.Join,

		"stringTrimPrefix": strings.TrimPrefix,
		"stringTrimSuffix": strings.TrimSuffix,
		"stringTrimSpace":  strings.TrimSpace,
		"stringReplaceAll": strings.ReplaceAll,

		"timeNow": time.Now,

		"glob":            fc.glob,
		"include":         fc.include,
		"includeCSS":      fc.includeCSS,
		"includeHTML":     fc.includeHTML,
		"includeHTMLAttr": fc.includeHTMLAttr,
		"includeJS":       fc.includeJS,
		"includeJSStr":    fc.includeJSStr,
		"readJSON":        fc.readJSON,
		"readPage":        fc.readPage,
		"readPages":       fc.readPages,
		"readImage":       fc.readImage,
		"readImageSrcSet": fc.readImageSrcSet,
	}
}

func dict(values ...interface{}) (map[string]interface{}, error) {
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

func (fc *functionContext) updateModTime(fpath string) error {
	fi, err := os.Stat(fpath)
	if err != nil {
		return err
	}
	fc.modTime = maxTime(fc.modTime, fi.ModTime())
	return nil
}

func (fc *functionContext) toFilePath(upath string) string {
	if !strings.HasPrefix(upath, "/") {
		upath = path.Join(fc.udir, upath)
	}
	return fc.site.toFilePath(upath)
}

func (fc *functionContext) toURLPath(abs bool, fpath string) (string, error) {
	if abs {
		p, err := filepath.Rel(fc.site.dir, fpath)
		if err != nil {
			return "", err
		}
		return "/" + filepath.ToSlash(p), nil
	}

	p, err := filepath.Rel(fc.fdir, fpath)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(p), nil
}

func (fc *functionContext) globInternal(uglob string) (fpaths []string, upaths []string, err error) {
	fglob := fc.toFilePath(uglob)
	fpaths, err = filepath.Glob(fglob)
	if err != nil {
		return nil, nil, err
	}

	upaths = make([]string, len(fpaths))
	abs := strings.HasPrefix(uglob, "/")
	for i, fpath := range fpaths {
		upaths[i], err = fc.toURLPath(abs, fpath)
		if err != nil {
			return nil, nil, err
		}
	}

	return fpaths, upaths, nil
}

func (fc *functionContext) glob(uglob string) ([]string, error) {
	_, upaths, err := fc.globInternal(uglob)
	return upaths, err
}

func (fc *functionContext) include(upath string) (string, error) {
	fpath := fc.toFilePath(upath)
	fc.updateModTime(fpath)
	b, err := ioutil.ReadFile(fpath)
	return string(b), err
}

func (fc *functionContext) includeCSS(upath string) (htemplate.CSS, error) {
	s, err := fc.include(upath)
	return htemplate.CSS(s), err
}

func (fc *functionContext) includeHTML(upath string) (htemplate.HTML, error) {
	s, err := fc.include(upath)
	return htemplate.HTML(s), err
}

func (fc *functionContext) includeHTMLAttr(upath string) (htemplate.HTMLAttr, error) {
	s, err := fc.include(upath)
	return htemplate.HTMLAttr(s), err
}

func (fc *functionContext) includeJS(upath string) (htemplate.JS, error) {
	s, err := fc.include(upath)
	return htemplate.JS(s), err
}

func (fc *functionContext) includeJSStr(upath string) (htemplate.JSStr, error) {
	s, err := fc.include(upath)
	return htemplate.JSStr(s), err
}

func (fc *functionContext) readJSON(upath string) (interface{}, error) {
	fpath := fc.toFilePath(upath)
	fc.updateModTime(fpath)
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var v interface{}
	err = json.NewDecoder(f).Decode(&v)
	return v, err
}

func (fc *functionContext) readPage(upath string) (*Page, error) {
	if strings.HasSuffix(upath, "/") {
		upath += "index.html"
	}

	fpath := fc.toFilePath(upath)
	fc.updateModTime(fpath)

	if strings.HasSuffix(upath, "/index.html") {
		upath = upath[:len(upath)-len("index.html")]
	}

	p := &Page{Path: upath}
	_, _, err := readFileWithFrontMatter(fpath, p)
	return p, err
}

func (fc *functionContext) readPages(uglob string, options ...string) ([]*Page, error) {
	if strings.HasSuffix(uglob, "/") {
		uglob += "index.html"
	}

	fpaths, upaths, err := fc.globInternal(uglob)
	if err != nil {
		return nil, err
	}

	var pages []*Page
	for i, fpath := range fpaths {
		upath := upaths[i]

		fc.updateModTime(fpath)
		if strings.HasSuffix(upath, "/index.html") {
			upath = upath[:len(upath)-len("index.html")]
		}

		page := &Page{Path: upath}
		_, _, err := readFileWithFrontMatter(fpath, page)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}

	for _, option := range options {
		switch {
		case option == "sort:-Created":
			sort.Slice(pages, func(i, j int) bool {
				return pages[j].Created.Before(pages[i].Created)
			})
		case strings.HasPrefix(option, "limit:"):
			s := option[len("limit:"):]
			n, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("readPages: invalid limit %q", s)
			}
			if n < len(pages) {
				pages = pages[:n]
			}
		default:
			return nil, fmt.Errorf("readPages: invalid option %q", option)
		}
	}

	return pages, nil
}

type Image struct {
	Width  int
	Height int
	Src    string
}

func (img *Image) SrcWidthHeight() htemplate.HTMLAttr {
	return htemplate.HTMLAttr(fmt.Sprintf(`src="%s" width="%d" height="%d"`, img.Src, img.Width, img.Height))
}

func (fc *functionContext) readImage(upath string) (*Image, error) {
	config, err := readImageConfig(fc.toFilePath(upath))
	return &Image{Src: upath, Width: config.Width, Height: config.Height}, err
}

type ImageSrcSet struct {
	Image
	SrcSet string
}

func (fc *functionContext) readImageSrcSet(uglob string, maxWidth int, maxHeight int) (*ImageSrcSet, error) {

	fpaths, upaths, err := fc.globInternal(uglob)
	if err != nil {
		return nil, err
	}

	if len(fpaths) == 0 {
		return nil, fmt.Errorf("%s - no images found for %s", fc.fname, uglob)
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
