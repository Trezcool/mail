package mail

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// todo: ci
// todo: cd -> godoc with examples

type (
	// Message holds all the information needed to send an email
	// TODO: add support for provider's templated messages
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

func (m *Message) isSimple() bool {
	return m.BodyStr != ""
}

func (m *Message) getContextData() ContextData {
	return ContextData{
		Data: m.TemplateData,
	}
}

func (m *Message) renderText() error {
	if m.isSimple() {
		m.TextContent = m.BodyStr
		return nil
	}

	tmpl, ok := templates.getText(m.TemplateName)
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
	tmpl, ok := templates.getHTML(m.TemplateName)
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

// Render renders the different message contents.
//
// If the message is simple (BodyStr is not empty), it will only render the text
// otherwise it will render both text and html templates
func (m *Message) Render() error { // todo: test
	if m.isSimple() {
		return m.renderText()
	}

	if m.TemplateName == "" {
		return errors.New("template name is empty")
	}

	if !templates.contains(m.TemplateName) {
		return errors.Errorf("template %s not found", m.TemplateName)
	}

	if err := m.renderText(); err != nil {
		return errors.Wrap(err, "rendering text template")
	}

	return errors.Wrap(
		m.renderHTML(),
		"rendering html template",
	)
}

// Attach reads the content of the reader and attaches it to the message
func (m *Message) Attach(r io.Reader, filename string, contentType ...string) error {
	attachment := Attachment{Filename: filename}

	// read content
	content, err := io.ReadAll(r)
	if err != nil {
		return errors.Wrapf(err, "reading content of %s", filename)
	}

	// set content type
	if len(contentType) > 0 {
		attachment.ContentType = contentType[0]
	} else {
		attachment.ContentType = http.DetectContentType(content)
	}

	// base64 encode & attach content
	encoder := base64.NewEncoder(base64.StdEncoding, attachment.Content)
	defer func() { _ = encoder.Close() }()
	if _, err = encoder.Write(content); err != nil {
		return errors.Wrapf(err, "encoding content of %s", filename)
	}

	m.Attachments = append(m.Attachments, attachment)
	return nil
}

// AttachFile attaches the content of the file to the message todo: test
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
	var builder strings.Builder

	builder.WriteString("To: ")
	for _, to := range m.To {
		builder.WriteString(to.String())
		builder.WriteString(", ")
	}
	builder.WriteString("\n")

	if len(m.Cc) > 0 {
		builder.WriteString("CC: ")
		for _, cc := range m.Cc {
			builder.WriteString(cc.String())
			builder.WriteString(", ")
		}
		builder.WriteString("\n")
	}

	if len(m.Bcc) > 0 {
		builder.WriteString("BCC: ")
		for _, bcc := range m.Bcc {
			builder.WriteString(bcc.String())
			builder.WriteString(", ")
		}
		builder.WriteString("\n")
	}

	builder.WriteString("Subject: ")
	builder.WriteString(m.Subject)

	return builder.String()
}
