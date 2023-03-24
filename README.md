# mail

This is a simple and easy-to-use email library for Go that allows you to send emails from your Go applications.

### Features
- Send emails with plain text or HTML content
- Attach files to your emails
- Support for CC and BCC recipients
- Support for different providers:
  - **Console**: print emails to stdout in compliance with RFC 2046 (for development & testing purposes)
  - **Sendgrid**: send emails using the Sendgrid API
  - **Mailchimp**: send emails using the Mailchimp API (coming soon...)
  - **Mailgun**: send emails using the Mailgun API (coming soon...)

### Installation
```bash
go get github.com/Trezcool/mail
```
