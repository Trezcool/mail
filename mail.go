package mail

import (
	"bytes"
	"encoding/base64"
	htmltmpl "html/template"
	"io"
	"io/fs"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	texttmpl "text/template"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

var templates tmplCache

const (
	extText = ".txt"
	extHTML = ".gohtml"
)

type (
	tmplCacheEntry map[string]interface{}    // {ext: *Template}
	tmplCache      map[string]tmplCacheEntry // {name: {tmplCacheEntry}}

	// Message holds all the information needed to send an email
	Message struct {
		To          []mail.Address
		Cc          []mail.Address
		Bcc         []mail.Address
		Subject     string
		BodyStr     string // simple text/plain, non-templated content
		Attachments []Attachment

		// templated contents
		TemplateName string // without ext
		TemplateData interface{}
		TextContent  string
		HTMLContent  string
	}

	ContextData struct {
		Data interface{}
	}

	Attachment struct {
		Content     *bytes.Buffer
		ContentType string
		Filename    string
	}
)

func (m *Message) getContextData() ContextData {
	return ContextData{
		Data: m.TemplateData,
	}
}

func (m *Message) getTemplate(ext string) (interface{}, bool) {
	cache, ok := templates[m.TemplateName]
	if !ok {
		return nil, ok
	}
	tmplEntry, ok := cache[ext]
	return tmplEntry, ok
}

func (m *Message) renderText() error {
	if m.BodyStr != "" {
		m.TextContent = m.BodyStr
		return nil
	} else if m.TemplateName == "" {
		return nil
	}

	tmplEntry, ok := m.getTemplate(extText)
	if !ok {
		return nil
	}
	tmpl, ok := tmplEntry.(*texttmpl.Template)
	if !ok {
		return nil
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, m.getContextData()); err != nil {
		return errors.Wrap(err, "executing template")
	}

	m.TextContent = buff.String()
	return nil
}

func (m *Message) renderHTML() error {
	if m.TemplateName == "" {
		return nil
	}

	tmplEntry, ok := m.getTemplate(extHTML)
	if !ok {
		return nil
	}
	tmpl, ok := tmplEntry.(*htmltmpl.Template)
	if !ok {
		return nil
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, m.getContextData()); err != nil {
		return errors.Wrap(err, "executing template")
	}

	m.HTMLContent = buff.String()
	return nil
}

func (m *Message) Render() error {
	if err := m.renderText(); err != nil {
		return errors.Wrap(err, "rendering text template")
	}

	return errors.Wrap(
		m.renderHTML(),
		"rendering html template",
	)
}

func (m *Message) Attach(r io.Reader, filename string, contentType ...string) error {
	attachment := Attachment{Filename: filename}

	// read & attach content
	content, err := io.ReadAll(r)
	if err != nil {
		return errors.Wrapf(err, "reading content of %s", filename)
	}
	// base64 encode content
	encoder := base64.NewEncoder(base64.StdEncoding, attachment.Content)
	if _, err := encoder.Write(content); err != nil {
		return errors.Wrapf(err, "encoding content of %s", filename)
	}
	_ = encoder.Close()

	// set content type
	if len(contentType) > 0 {
		attachment.ContentType = contentType[0]
	} else {
		attachment.ContentType = http.DetectContentType(content)
	}

	m.Attachments = append(m.Attachments, attachment)
	return nil
}

func (m *Message) AttachFile(path string, contentType ...string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "opening file %s", path)
	}
	defer func() { _ = file.Close() }()

	return errors.Wrapf(
		m.Attach(file, filepath.Base(path), contentType...),
		"attaching file %s", path,
	)
}

func (m *Message) HasRecipients() bool  { return len(m.To) > 0 }
func (m *Message) HasContent() bool     { return (m.TextContent != "") || (m.HTMLContent != "") }
func (m *Message) HasAttachments() bool { return len(m.Attachments) > 0 }

func (m *Message) String() string {
	var buff bytes.Buffer

	buff.WriteString("To: ")
	for _, to := range m.To {
		buff.WriteString(to.String())
		buff.WriteString(", ")
	}
	buff.WriteString("\n")

	if len(m.Cc) > 0 {
		buff.WriteString("CC: ")
		for _, cc := range m.Cc {
			buff.WriteString(cc.String())
			buff.WriteString(", ")
		}
		buff.WriteString("\n")
	}

	if len(m.Bcc) > 0 {
		buff.WriteString("BCC: ")
		for _, bcc := range m.Bcc {
			buff.WriteString(bcc.String())
			buff.WriteString(", ")
		}
		buff.WriteString("\n")
	}

	buff.WriteString("Subject: ")
	buff.WriteString(m.Subject)

	return buff.String()
}

// ParseTemplates parses all templates in the given rootpath and stores them in the global templates cache.
func ParseTemplates(fsys fs.FS, rootpath string, baseTmplName ...string) error {
	if templates == nil {
		templates = make(tmplCache)
	}

	// TODO: support multiple base templates ?
	baseTmpl := lo.Ternary(len(baseTmplName) > 0, baseTmplName[0], "")
	hasBase := baseTmpl != ""

	paths, err := fs.Glob(fsys, rootpath+"*") // TODO: walk instead of glob ?
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
		entry, ok := templates[name]
		if !ok {
			entry = make(tmplCacheEntry)
			templates[name] = entry
		}

		tmplPaths := lo.Ternary(
			hasBase,
			[]string{rootpath + baseTmpl + ext, path},
			[]string{path},
		)

		switch ext {
		case extText:
			tmpl, parseErr := texttmpl.ParseFS(fsys, tmplPaths...)
			if parseErr != nil {
				return errors.Wrapf(parseErr, "parsing %s files %v", ext, tmplPaths)
			}
			entry[ext] = tmpl
		case extHTML:
			tmpl, parseErr := htmltmpl.ParseFS(fsys, tmplPaths...)
			if parseErr != nil {
				return errors.Wrapf(parseErr, "parsing %s files %v", ext, tmplPaths)
			}
			entry[ext] = tmpl
		}
	}

	return nil
}
