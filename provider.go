package mail

import (
	"sync"

	"github.com/pkg/errors"
)

const (
	contentTypeText = "text/plain"
	contentTypeHTML = "text/html"
)

type BaseProvider struct{}

func (p BaseProvider) SendMessages(messages ...*Message) <-chan error {
	var (
		errs = make(chan error)
		wg   sync.WaitGroup
	)

	for _, msg := range messages { // TODO: use a worker pool
		wg.Add(1)
		msg := msg
		go func() {
			defer wg.Done()

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

func (p BaseProvider) send(msg Message) error {
	panic("implement me")
}
