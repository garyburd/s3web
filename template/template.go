package template

import (
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"path/filepath"
	"sync"
	ttemplate "text/template"
	"text/template/parse"
)

const (
	mainName = "__main__"
	metaName = "_"
)

type Loader struct {
	dir      string
	funcs    map[string]interface{}
	template *htemplate.Template

	treesMu    sync.Mutex
	treesCache map[string]*treesCacheEntry

	templateMu    sync.Mutex
	templateCache map[string]*templateCacheEntry
}

type treesCacheEntry struct {
	once  sync.Once
	trees map[string]*parse.Tree
	err   error
}

type templateCacheEntry struct {
	once     sync.Once
	template *htemplate.Template
	err      error
}

func NewLoader(dir string, funcs map[string]interface{}) (*Loader, error) {
	l := Loader{
		dir:           dir,
		funcs:         make(map[string]interface{}),
		treesCache:    make(map[string]*treesCacheEntry),
		templateCache: make(map[string]*templateCacheEntry),
		template:      htemplate.New(mainName).Funcs(funcs),
	}

	// Create funcs for tree parse.
	for _, name := range textTemplateBuiltinFuncs {
		name := name
		l.funcs[name] = func(args ...interface{}) (string, error) {
			return "", fmt.Errorf("unexpected call to %q", name)
		}
	}
	for k, v := range funcs {
		l.funcs[k] = v
	}

	return &l, nil
}

// Built-in functions from text/template package.
// Copied from builtins() in $GOROOT/src/text/template/funcs.go.
var textTemplateBuiltinFuncs = []string{
	"and",
	"call",
	"html",
	"index",
	"slice",
	"js",
	"len",
	"not",
	"or",
	"print",
	"printf",
	"println",
	"urlquery",
	"eq",
	"ge",
	"gt",
	"le",
	"lt",
	"ne",
}

func (l *Loader) Get(path string) (*htemplate.Template, error) {
	fpath := filepath.Join(l.dir, filepath.FromSlash(path))

	l.templateMu.Lock()
	e := l.templateCache[fpath]
	if e == nil {
		e = &templateCacheEntry{}
		l.templateCache[fpath] = e
	}
	l.templateMu.Unlock()

	e.once.Do(func() {
		e.template, e.err = l.loadTemplate(fpath)
	})

	return e.template, e.err
}

func (l *Loader) loadTemplate(fpath string) (*htemplate.Template, error) {

	// Optimize for the case where the trees in fpath are not used in other
	// templates.
	//  - Don't cache the trees for fpath.
	//  - To allow direct use of the trees in the compiled template, do
	//    copy imported trees into the trees for this path.

	trees, err := l.loadTrees(fpath, true, map[string]struct{}{})
	if err != nil {
		return nil, err
	}

	t := htemplate.Must(l.template.Clone())
	for name, tree := range trees {
		if _, err := t.AddParseTree(name, tree); err != nil {
			return nil, err
		}
	}
	return t.Lookup(mainName), nil
}

func (l *Loader) getTrees(fpath string, inflight map[string]struct{}) (map[string]*parse.Tree, error) {
	l.treesMu.Lock()
	e := l.treesCache[fpath]
	if e == nil {
		e = &treesCacheEntry{}
		l.treesCache[fpath] = e
	}
	l.treesMu.Unlock()

	e.once.Do(func() {
		e.trees, e.err = l.loadTrees(fpath, false, inflight)
	})

	return e.trees, e.err
}

func (l *Loader) loadTrees(fpath string, copyImports bool, inflight map[string]struct{}) (map[string]*parse.Tree, error) {
	if _, ok := inflight[fpath]; ok {
		return nil, fmt.Errorf("template import cycle: %s", fpath)
	}
	inflight[fpath] = struct{}{}

	p, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	trees, err := parse.Parse(fpath, string(p), "", "", l.funcs)
	if err != nil {
		return nil, err
	}

	main := trees[fpath]
	delete(trees, fpath)
	if !parse.IsEmptyTree(main.Root) {
		trees[mainName] = main
	}

	if tree, ok := trees[metaName]; ok {
		delete(trees, metaName)
		m := &meta{
			trees:       trees,
			loader:      l,
			inflight:    inflight,
			copyImports: copyImports,
		}
		t, err := ttemplate.New(metaName).AddParseTree(metaName, tree)
		if err != nil {
			return nil, err
		}
		err = t.Execute(ioutil.Discard, m)
		if err != nil {
			if m.err != nil {
				// Jump over layers of template execution errors.
				err = m.err
			}
			return nil, err
		}
	}

	return trees, err
}

type meta struct {
	trees       map[string]*parse.Tree
	loader      *Loader
	inflight    map[string]struct{}
	copyImports bool

	err error
}

func (m *meta) Import(path string) (string, error) {
	fpath := filepath.Join(m.loader.dir, filepath.FromSlash(path))
	trees, err := m.loader.getTrees(fpath, m.inflight)
	if err != nil {
		m.err = err
		return "", err
	}
	for name, tree := range trees {
		if _, ok := m.trees[name]; ok {
			continue
		}
		if m.copyImports {
			tree = tree.Copy()
		}
		m.trees[name] = tree
	}
	return "", nil
}

func (m *meta) Include(name string, path string) (string, error) {
	fpath := filepath.Join(m.loader.dir, filepath.FromSlash(path))
	p, err := ioutil.ReadFile(fpath)
	if err != nil {
		return "", err
	}

	if i := len(p) - 1; i >= 0 && p[i] == '\n' {
		p = p[:i]
		i--
		if i >= 0 && p[i] == '\r' {
			p = p[:i]
		}
	}

	tree := &parse.Tree{
		Name:      name,
		ParseName: fpath,
		Root: &parse.ListNode{
			NodeType: parse.NodeList,
			Nodes: []parse.Node{
				&parse.TextNode{
					NodeType: parse.NodeText,
					Text:     p,
				},
			},
		},
	}

	m.trees[name] = tree
	return "", nil
}
