package mail

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ConsoleProvider prints emails to stdout in compliance with RFC 2046
type ConsoleProvider struct {
	*BaseProvider
}

func NewConsoleProvider(
	from mail.Address,
	opts ...Option,
) (*ConsoleProvider, <-chan error) {
	bp, errC := NewBaseProvider(from, opts...)

	p := &ConsoleProvider{
		BaseProvider: bp,
	}
	return p, errC
}

// TODO: test
func (p *ConsoleProvider) send(msg Message) (err error) {
	buffer := new(strings.Builder)

	p.writeHeader(buffer, msg)

	var mixedW *multipart.Writer
	altW := multipart.NewWriter(buffer) // todo: mock random boundary in multipart writer for tests
	defer func() { _ = altW.Close() }()

	if err = p.writeMultipartContentType(altW, mixedW, buffer, msg); err != nil {
		return err
	}

	if mixedW != nil {
		defer func() { _ = mixedW.Close() }()
	}

	var w io.Writer
	if err = p.writeBody(altW, w, msg); err != nil {
		return err
	}

	if err = p.writeAttachments(mixedW, w, msg); err != nil {
		return err
	}

	log.Println(buffer.String()) // todo: mock logger for tests
	return nil
}

func (p *ConsoleProvider) writeHeader(w io.Writer, msg Message) {
	_, _ = fmt.Fprintf(w, "From: %s\r\n", p.from.String())
	_, _ = fmt.Fprint(w, "MIME-Version: 1.0\r\n")
	_, _ = fmt.Fprintf(w, "Date: %s\r\n", time.Now().Format(time.RFC1123Z)) // todo: mock time for tests
	_, _ = fmt.Fprintf(w, "Subject: %s\r\n", p.subjPrefix+msg.Subject)
	_, _ = fmt.Fprintf(w, "To: %s\r\n", joinAddresses(msg.To))
	_, _ = fmt.Fprintf(w, "CC: %s\r\n", joinAddresses(msg.Cc))
	_, _ = fmt.Fprintf(w, "BCC: %s\r\n", joinAddresses(msg.Bcc))
}

func (p *ConsoleProvider) writeMultipartContentType(
	altW, mixedW *multipart.Writer,
	w io.Writer,
	msg Message,
) error {
	if msg.HasAttachments() {
		if mixedW == nil {
			mixedW = multipart.NewWriter(w)
		}
		_, _ = fmt.Fprintf(w, "Content-Type: multipart/mixed\r\n")
		_, _ = fmt.Fprintf(w, "Content-Type: boundary=%s\r\n", mixedW.Boundary())
	} else {
		_, _ = fmt.Fprintf(w, "Content-Type: multipart/alternative\r\n")
		_, _ = fmt.Fprintf(w, "Content-Type: boundary=%s\r\n", altW.Boundary())
	}
	_, _ = fmt.Fprint(w, "\r\n")

	if mixedW != nil {
		if _, err := mixedW.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"multipart/alternative", "boundary=" + altW.Boundary()},
		}); err != nil {
			return errors.Wrap(err, "creating multipart/alternative part")
		}
	}

	return nil
}

func (p *ConsoleProvider) writeBody(
	altW *multipart.Writer,
	w io.Writer,
	msg Message,
) (err error) {
	if msg.TextContent != "" {
		w, err = altW.CreatePart(textproto.MIMEHeader{"Content-Type": {contentTypeText}})
		if err != nil {
			return errors.Wrap(err, "creating text/plain part")
		}
		_, _ = fmt.Fprintf(w, "%s\r\n", msg.TextContent)
	}

	if msg.HTMLContent != "" {
		w, err = altW.CreatePart(textproto.MIMEHeader{"Content-Type": {contentTypeHTML}})
		if err != nil {
			return errors.Wrap(err, "creating text/html part")
		}
		_, _ = fmt.Fprintf(w, "%s\r\n", msg.HTMLContent)
	}

	return nil
}

func (p *ConsoleProvider) writeAttachments(
	mixedW *multipart.Writer,
	w io.Writer,
	msg Message,
) (err error) {
	if mixedW == nil {
		return nil
	}

	for _, at := range msg.Attachments {
		w, err = mixedW.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {at.ContentType},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {"attachment; filename=" + at.Filename},
		})
		if err != nil {
			return errors.Wrap(err, "creating "+at.ContentType+" part")
		}
		_, _ = fmt.Fprintf(w, "%s\r\n", at.Content.String())
	}

	return nil
}

func joinAddresses(addrs []mail.Address) string {
	return lo.Reduce(
		addrs,
		func(joined string, addr mail.Address, _ int) string {
			return strings.Join([]string{joined, addr.String()}, ", ")
		},
		"",
	)
}
