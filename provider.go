package mail

const (
	contentTypeText = "text/plain"
	contentTypeHTML = "text/html"
)

// Provider is any service that can send emails
type Provider interface {
	SendMessages(messages ...*Message) <-chan error
}
