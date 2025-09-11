package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/danglnh07/TaskManagement/util"
)

type EmailService struct {
	config *util.Config
	Auth   smtp.Auth
}

func NewEmailService(config *util.Config) *EmailService {
	// Try simple authentication
	smtpAuth := smtp.PlainAuth("", config.Email, config.AppPassword, config.SMTPHost)

	return &EmailService{
		config: config,
		Auth:   smtpAuth,
	}
}

func (service *EmailService) SendEmail(to, subject, body string) error {
	// Set email headers with MIME version and content type
	headers := make(map[string]string)
	headers["From"] = service.config.Email
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Build the message with headers
	var message strings.Builder
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	addr := fmt.Sprintf("%s:%s", service.config.SMTPHost, service.config.SMTPPort)
	return smtp.SendMail(
		addr,
		service.Auth,
		service.config.Email,
		[]string{to},
		[]byte(message.String()),
	)
}
