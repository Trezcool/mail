package mail

import (
	"fmt"
	"net/http"
	"net/mail"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sendgrid/sendgrid-go"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
)

const (
	sgHost     = "https://api.sendgrid.com"
	sgEndpoint = "/v3/mail/send"

	dispositionAttachment = "attachment"
)

// SendgridProvider sends emails using the Sendgrid API
type SendgridProvider struct {
	baseProvider
	key        string
	from       *sgmail.Email
	subjPrefix string
}

var _ Provider = (*SendgridProvider)(nil)

func NewSendgridProvider(fromEmail mail.Address, subjPrefix, apiKey string) *SendgridProvider {
	return &SendgridProvider{
		key:        apiKey,
		from:       sgmail.NewEmail(fromEmail.Name, fromEmail.Address),
		subjPrefix: subjPrefix,
	}
}

func (p SendgridProvider) send(msg Message) error {
	req := sendgrid.GetRequest(p.key, sgEndpoint, sgHost)
	req.Method = http.MethodPost
	req.Body = sgmail.GetRequestBody(p.prepare(msg))

	res, err := sendgrid.API(req)
	if err != nil {
		return errors.Wrap(err, "sending request")
	}
	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("unexpected status code: %d - Body: %s", res.StatusCode, res.Body)
	}
	return nil
}

func (p SendgridProvider) prepare(msg Message) *sgmail.SGMailV3 {
	perso := sgmail.NewPersonalization()
	perso.Subject = p.subjPrefix + msg.Subject

	perso.AddTos(addressesToSGEmails(msg.To)...)
	perso.AddCCs(addressesToSGEmails(msg.Cc)...)
	perso.AddBCCs(addressesToSGEmails(msg.Bcc)...)

	m := sgmail.NewV3Mail()
	m.SetFrom(p.from)
	m.AddPersonalizations(perso)

	m.AddContent(
		sgmail.NewContent(contentTypeText, msg.TextContent),
		sgmail.NewContent(contentTypeHTML, msg.HTMLContent),
	)

	m.AddAttachment(attachmentsToSGAttachments(msg.Attachments)...)

	return m
}

func addressesToSGEmails(addrs []mail.Address) []*sgmail.Email {
	return lo.Map(addrs, func(addr mail.Address, _ int) *sgmail.Email {
		return sgmail.NewEmail(addr.Name, addr.Address)
	})
}

func attachmentsToSGAttachments(attachments []Attachment) []*sgmail.Attachment {
	return lo.Map(attachments, func(at Attachment, _ int) *sgmail.Attachment {
		return &sgmail.Attachment{
			Content:     at.Content.String(),
			Type:        at.ContentType,
			Filename:    at.Filename,
			Disposition: dispositionAttachment,
		}
	})
}
