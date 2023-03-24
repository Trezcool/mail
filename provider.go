package mail

import (
	"sync"

	"github.com/pkg/errors"
)

const (
	contentTypeText = "text/plain"
	contentTypeHTML = "text/html"
)

// Provider is any service that can send emails
type Provider interface {
	SendMessages(messages ...*Message) <-chan error
}

type baseProvider struct{}

func (p baseProvider) SendMessages(messages ...*Message) <-chan error {
	var (
		errs = make(chan error)
		wg   sync.WaitGroup
	)

	for _, msg := range messages { // TODO: use a worker pool
		wg.Add(1)
		msg := msg
		go func() {
			if !msg.HasRecipients() {
				errs <- errors.Errorf("no recipients for email %s", msg.Subject)
				return
			}

			if err := msg.Render(); err != nil {
				errs <- errors.Wrapf(err, "rendering email %s", msg.Subject)
				return
			}

			if !(msg.HasContent() || msg.HasAttachments()) {
				errs <- errors.Errorf("no content or attachments for email %s", msg.Subject)
				return
			}

			if err := p.send(*msg); err != nil {
				errs <- errors.Wrapf(err, "sending email %s", msg.Subject)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	return errs
}

func (p baseProvider) send(msg Message) error {
	panic("implement me")
}
