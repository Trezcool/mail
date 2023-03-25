package mail

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

var (
	SentMessages = make([]Message, 0)
	smMu         sync.Mutex
)

// ConsoleProvider prints emails to stdout in compliance with RFC 2046
type ConsoleProvider struct {
	fromEmail  mail.Address
	subjPrefix string
	quiet      bool
}

func NewConsoleProvider(fromEmail mail.Address, subjPrefix string, quiet ...bool) *ConsoleProvider {
	return &ConsoleProvider{
		fromEmail:  fromEmail,
		subjPrefix: subjPrefix,
		quiet:      len(quiet) > 0 && quiet[0],
	}
}

func (p ConsoleProvider) SendMessages(messages ...*Message) <-chan error {
	var errs []error

	for _, msg := range messages {
		if err := p.sendMessage(msg); err != nil {
			errs = append(errs, err)
		}
	}

	errsC := make(chan error, len(errs))
	for _, err := range errs {
		errsC <- err
	}
	close(errsC)
	return errsC
}

func (p ConsoleProvider) sendMessage(msg *Message) error {
	if !msg.HasRecipients() {
		return errors.Errorf("no recipients for email %s", msg.Subject)
	}

	if err := msg.Render(); err != nil {
		return errors.Wrapf(err, "rendering email %s", msg.Subject)
	}

	if !(msg.HasContent() || msg.HasAttachments()) {
		return errors.Errorf("no content or attachments for email %s", msg.Subject)
	}

	if err := p.send(*msg); err != nil {
		return errors.Wrapf(err, "sending email %s", msg.Subject)
	}

	smMu.Lock()
	SentMessages = append(SentMessages, *msg)
	smMu.Unlock()
	return nil
}

func (p ConsoleProvider) send(msg Message) (err error) {
	body := new(strings.Builder)

	// Write mail header
	_, _ = fmt.Fprintf(body, "From: %s\r\n", p.fromEmail.String())
	_, _ = fmt.Fprint(body, "MIME-Version: 1.0\r\n")
	_, _ = fmt.Fprintf(body, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(body, "Subject: %s\r\n", p.subjPrefix+msg.Subject)
	_, _ = fmt.Fprintf(body, "To: %s\r\n", joinAddresses(msg.To))
	_, _ = fmt.Fprintf(body, "CC: %s\r\n", joinAddresses(msg.Cc))
	_, _ = fmt.Fprintf(body, "BCC: %s\r\n", joinAddresses(msg.Bcc))

	var mixedW *multipart.Writer
	altW := multipart.NewWriter(body)
	defer func() { _ = altW.Close() }()

	if msg.HasAttachments() {
		mixedW = multipart.NewWriter(body)
		defer func() { _ = mixedW.Close() }()
		_, _ = fmt.Fprintf(body, "Content-Type: multipart/mixed\r\n")
		_, _ = fmt.Fprintf(body, "Content-Type: boundary=%s\r\n", mixedW.Boundary())
	} else {
		_, _ = fmt.Fprintf(body, "Content-Type: multipart/alternative\r\n")
		_, _ = fmt.Fprintf(body, "Content-Type: boundary=%s\r\n", altW.Boundary())
	}
	_, _ = fmt.Fprint(body, "\r\n")

	if mixedW != nil {
		if _, err = mixedW.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"multipart/alternative", "boundary=" + altW.Boundary()},
		}); err != nil {
			return errors.Wrap(err, "creating multipart/alternative part")
		}
	}

	var w io.Writer
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

	if mixedW != nil {
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
	}

	if !p.quiet {
		log.Println(body.String())
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
