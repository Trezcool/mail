package mail

import (
	htmltmpl "html/template"
	"io/fs"
	"path/filepath"
	"strings"
	texttmpl "text/template"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const (
	extText = ".txt"
	extHTML = ".gohtml"
)

type tmplCache struct {
	textCache map[string]*texttmpl.Template
	htmlCache map[string]*htmltmpl.Template
}

func newTmplCache() *tmplCache {
	return &tmplCache{
		textCache: make(map[string]*texttmpl.Template),
		htmlCache: make(map[string]*htmltmpl.Template),
	}
}

func (c *tmplCache) contains(name string) bool {
	_, ok := c.textCache[name]
	if ok {
		return ok
	}
	_, ok = c.htmlCache[name]
	return ok
}
func (c *tmplCache) getText(name string) (*texttmpl.Template, bool) {
	tmpl, ok := c.textCache[name]
	return tmpl, ok
}
func (c *tmplCache) getHTML(name string) (*htmltmpl.Template, bool) {
	tmpl, ok := c.htmlCache[name]
	return tmpl, ok
}

var templates *tmplCache

// ParseTemplates parses all templates in the given rootpath and stores them in the global templates cache.
//
// Must be called upon app initialization & before sending any messages.
func ParseTemplates(fsys fs.FS, rootpath string, baseTmplName ...string) error {
	if templates == nil {
		templates = newTmplCache()
	}

	rootpath = filepath.Clean(rootpath) + "/"

	// TODO: support multiple base templates ?
	hasBase := len(baseTmplName) > 0
	baseTmpl := lo.Ternary(hasBase, baseTmplName[0], "")

	paths, err := fs.Glob(fsys, rootpath+"*") // TODO: walk instead ?
	if err != nil {
		return errors.Wrapf(err, "globbing %s", rootpath)
	}

	for _, path := range paths {
		filename := filepath.Base(path)
		ext := filepath.Ext(filename)
		isBase := lo.Ternary(hasBase, strings.HasPrefix(filename, baseTmpl), false)
		if isBase || !(ext == extText || ext == extHTML) {
			continue
		}

		name := filename[:strings.LastIndex(filename, ".")]

		tmplPaths := lo.Ternary(
			hasBase,
			[]string{filepath.Join(rootpath, baseTmpl+ext), path},
			[]string{path},
		)

		switch ext {
		case extText:
			tmpl, parseErr := texttmpl.ParseFS(fsys, tmplPaths...)
			if parseErr != nil {
				return errors.Wrapf(parseErr, "parsing %s files %v", ext, tmplPaths)
			}
			templates.textCache[name] = tmpl
		case extHTML:
			tmpl, parseErr := htmltmpl.ParseFS(fsys, tmplPaths...)
			if parseErr != nil {
				return errors.Wrapf(parseErr, "parsing %s files %v", ext, tmplPaths)
			}
			templates.htmlCache[name] = tmpl
		}
	}

	return nil
}
