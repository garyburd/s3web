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

	"github.com/garyburd/staticsite/action"
	"github.com/garyburd/staticsite/html"
)

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

	// The page's directory. Use for resolving relative paths.
	Dir string

	Content htemplate.HTML
}

type actionContext struct {
	// The page's directory. Use for resolving relative paths.
	Dir string

	// Used to jump over template execution errors.
	err error

	lc *action.LocationContext

	action *action.Action
}

func (ac *actionContext) fatal(err error) error {
	ac.err = err
	return err
}

func (ac *actionContext) requiredAgumentNotFound(name string) error {
	return ac.fatal(fmt.Errorf("%s: required argument %q not found", ac.action.Location(ac.lc), name))
}

func (ac *actionContext) String(name string, def ...string) (string, error) {
	v, ok := ac.action.Args[name]
	if !ok {
		if len(def) == 0 {
			return "", ac.requiredAgumentNotFound(name)
		}
		return def[0], nil
	}
	return v.Text, nil
}

func (ac *actionContext) Int(name string, def ...int) (int, error) {
	v, ok := ac.action.Args[name]
	if !ok {
		if len(def) == 0 {
			return 0, ac.requiredAgumentNotFound(name)
		}
		return def[0], nil
	}
	i, err := strconv.Atoi(v.Text)
	if err != nil {
		return 0, ac.fatal(fmt.Errorf("%s: %w", v.Location(ac.lc), err))
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
		case "layout":
			// handled in caller.
		default:
			return fmt.Errorf("%s: unknown argument %q", v.Location(lc), k)
		}
	}
	return nil
}

func (s *site) processPage(r *Resource) error {
	actions, lc, err := action.ParseFile(r.FilePath)
	if err != nil {
		return err
	}

	p := &Page{
		Path:  r.Path,
		Dir:   path.Dir(r.Path),
		Title: path.Base(strings.TrimSuffix(r.Path, "/index.html")),
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
				layout, err = s.loader.Get(v.Text)
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
			ac := actionContext{
				Dir:    p.Dir,
				action: a,
				lc:     lc,
			}
			if err := t.Execute(&body, &ac); err != nil {
				if ac.err != nil {
					return ac.err
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

	data, err := html.Minify(buf.Bytes())
	if err != nil {
		return fmt.Errorf("%s:1 %v", r.FilePath, err)
	}

	s.pages[p.Path] = p
	r.Data = data
	r.Size = int64(len(r.Data))
	r.ModTime = time.Time{}
	return nil
}
