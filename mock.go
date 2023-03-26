package mail

import (
	"net/mail"
)

var SentMessages = make([]Message, 0)

// MockProvider does not send emails, but stores them in memory. It is useful for testing.
type MockProvider struct {
	*BaseProvider
	errC chan error
}

func NewMockProvider(
	from mail.Address,
	opts ...Option,
) (*MockProvider, <-chan error) {
	bp, errC := NewBaseProvider(from, opts...)

	p := &MockProvider{
		BaseProvider: bp,
	}
	return p, errC
}

func (p *MockProvider) init() <-chan error {
	p.errC = make(chan error)
	return p.errC
}

func (p *MockProvider) SendMessage(messages ...Message) {
	for _, msg := range messages {
		if err := p.render(&msg); err != nil {
			p.errC <- err
			continue
		}
		SentMessages = append(SentMessages, msg)
	}
}

func (p *MockProvider) Close() {
	close(p.errC)
}
