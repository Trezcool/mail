package mail

import (
	"net/mail"
	"sync"

	"github.com/pkg/errors"
	"github.com/sourcegraph/conc/pool"
)

const (
	contentTypeText = "text/plain"
	contentTypeHTML = "text/html"

	defaultMaxWorkers = 3
)

// BaseProvider is a base implementation of the Provider interface
type BaseProvider struct {
	from       mail.Address
	subjPrefix string
	maxWorkers int
	queue      chan Message
	watchOnce  sync.Once
}

func NewBaseProvider(
	from mail.Address,
	opts ...Option,
) (*BaseProvider, <-chan error) {
	config := providerConfig{
		subjPrefix: "",
		maxWorkers: defaultMaxWorkers,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	p := &BaseProvider{
		from:       from,
		subjPrefix: config.subjPrefix,
		maxWorkers: config.maxWorkers,
	}
	return p, p.init()
}

func (p *BaseProvider) init() <-chan error {
	errC := make(chan error)

	p.watchOnce.Do(func() {
		p.queue = make(chan Message, p.maxWorkers)

		go p.watch(errC)
	})

	return errC
}

func (p *BaseProvider) watch(errC chan<- error) {
	wp := pool.
		New().
		WithMaxGoroutines(p.maxWorkers)

	for message := range p.queue {
		msg := &message
		wp.Go(func() {
			if err := p.render(msg); err != nil {
				errC <- err
				return
			}

			if err := p.send(*msg); err != nil {
				errC <- errors.Wrapf(err, "sending email %s", msg)
			}
		})
	}

	wp.Wait()
	close(errC)
}

func (p *BaseProvider) render(msg *Message) error {
	if !msg.HasRecipients() {
		return errors.Errorf("no recipients for email %s", msg)
	}

	if err := msg.Render(); err != nil {
		return errors.Wrapf(err, "rendering email %s", msg)
	}

	if !(msg.HasContent() || msg.HasAttachments()) {
		return errors.Errorf("no content or attachments for email %s", msg)
	}

	return nil
}

func (p *BaseProvider) send(msg Message) error {
	panic("implement me")
}

// SendMessage pushes messages to the queue
func (p *BaseProvider) SendMessage(messages ...Message) {
	for _, msg := range messages {
		p.queue <- msg
	}
}

// Close closes the queue
func (p *BaseProvider) Close() {
	close(p.queue)
}

type providerConfig struct {
	subjPrefix string
	maxWorkers int
}

// Option is a configuration option for the provider
type Option func(*providerConfig)

// WithSubjectPrefix sets the default subject prefix for all messages
func WithSubjectPrefix(prefix string) Option {
	return func(o *providerConfig) {
		o.subjPrefix = prefix
	}
}

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(max uint) Option {
	if max == 0 {
		return nil
	}
	return func(o *providerConfig) {
		o.maxWorkers = int(max)
	}
}
