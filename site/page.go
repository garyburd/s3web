package site

import (
	"bytes"
	"fmt"
	htemplate "html/template"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/staticsite/common/action"
	"github.com/garyburd/staticsite/site/html"
	"github.com/garyburd/staticsite/site/scratch"
)

// Page represents the meta data for a page.
type Page struct {
	// Title is the page's title.
	Title string

	// Subtitle is the page's subtitle.
	Subtitle string

	// Created is the page creation time.
	Created time.Time

	// Updated is the time that the page was updated.
	Updated time.Time

	// Page path.
	Path string

	// Scratch data with page scope.
	Scratch *scratch.Scratch

	Content htemplate.HTML
}

// templateActionData is the data for executing template actions.
type templateActionData struct {
	// Path of the current page.
	Path string

	// Scratch data with page scope.
	Scratch *scratch.Scratch

	// Used to jump over template execution errors.
	err error

	// Location context for the page.
	lc *action.LocationContext

	// The current action.
	action *action.Action
}

func (ad *templateActionData) fatal(err error) error {
	ad.err = err
	return err
}

func (ad *templateActionData) requiredAgumentNotFound(name string) error {
	return ad.fatal(fmt.Errorf("%s: required argument %q not found", ad.action.Location(ad.lc), name))
}

// String returns argument with given name as a string. If the argument is
// missing, String return an error or def if specified.
func (ad *templateActionData) String(name string, def ...string) (string, error) {
	v, ok := ad.action.Args[name]
	if !ok {
		if len(def) == 0 {
			return "", ad.requiredAgumentNotFound(name)
		}
		return def[0], nil
	}
	return v.Text, nil
}

// String returns argument with given name as an int. If the argument is
// missing, Int return an error or def if specified.
func (ad *templateActionData) Int(name string, def ...int) (int, error) {
	v, ok := ad.action.Args[name]
	if !ok {
		if len(def) == 0 {
			return 0, ad.requiredAgumentNotFound(name)
		}
		return def[0], nil
	}
	i, err := strconv.Atoi(v.Text)
	if err != nil {
		return 0, ad.fatal(fmt.Errorf("%s: %w", v.Location(ad.lc), err))
	}
	return i, nil
}

func (p *Page) set(a *action.Action, lc *action.LocationContext) error {
	for k, v := range a.Args {
		switch k {
		case "title":
			p.Title = v.Text
		case "subtitle":
			p.Subtitle = v.Text
		case "created":
			var err error
			p.Created, err = time.Parse(time.RFC3339, v.Text)
			if err != nil {
				return fmt.Errorf("%s: %w", v.Location(lc), err)
			}
		case "updated":
			var err error
			p.Created, err = time.Parse(time.RFC3339, v.Text)
			if err != nil {
				return fmt.Errorf("%s: %w", v.Location(lc), err)
			}
		case "path":
			if !strings.HasPrefix(v.Text, "/") {
				return fmt.Errorf(`%s: page path must start with "/"`, v.Location(lc))
			}
			p.Path = v.Text
		case "layout":
			// handled in caller.
		default:
			return fmt.Errorf("%s: unknown argument %q", v.Location(lc), k)
		}
	}
	return nil
}

func (s *site) processPage(r *Resource) error {

	scratch := scratch.New()

	p := &Page{
		Path:    r.Path,
		Title:   path.Base(r.Path),
		Scratch: scratch,
	}

	actions, lc, err := action.ParseFile(r.FilePath)
	if err != nil {
		return err
	}

	var layout *htemplate.Template
	var body strings.Builder

	for _, a := range actions {
		switch {
		case a.Name == action.TextAction:
			body.Write(a.Text)
		case a.Name == "set":
			if err := p.set(a, lc); err != nil {
				return err
			}
			if v, ok := a.Args["layout"]; ok {
				layout, err = s.loader.Load(v.Text)
				if err != nil {
					if os.IsNotExist(err) {
						err = fmt.Errorf("%s: %w", v.Location(lc), err)
					}
					return err
				}
			}
		case strings.HasPrefix(a.Name, "t:"):
			if layout == nil {
				return fmt.Errorf("%s: specify layout with set command before calling templates",
					a.Location(lc))
			}
			name := a.Name[len("t:"):]
			t := layout.Lookup(name)
			if t == nil {
				return fmt.Errorf("%s: template with name %q not found in layout",
					a.Location(lc), name)
			}
			ad := templateActionData{
				Path:    p.Path,
				Scratch: scratch,
				lc:      lc,
				action:  a,
			}
			if err := t.Execute(&body, &ad); err != nil {
				if ad.err != nil {
					return ad.err
				}
				return fmt.Errorf("%s: %w", a.Location(lc), err)
			}
		default:
			return fmt.Errorf("%s: unknown command %q", a.Location(lc), a.Name)
		}
	}

	var buf bytes.Buffer
	if layout == nil {
		buf.WriteString(body.String())
	} else {
		p.Content = htemplate.HTML(body.String())
		err = layout.Execute(&buf, p)
		if err != nil {
			return err
		}
	}

	p.Scratch = nil

	data, err := html.Minify(buf.Bytes())
	if err != nil {
		return fmt.Errorf("%s:1 %v", r.FilePath, err)
	}

	r.Data = data
	r.Size = int64(len(r.Data))
	r.ModTime = time.Time{}

	// The 'set' action can override the page's path. Use the original path in
	// page queries.
	queryPath := r.Path
	r.Path = p.Path
	s.addPage(queryPath, p)

	return nil
}
