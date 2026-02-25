package email

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// Service handles sending transactional emails
type Service struct {
	smtpHost string
	smtpPort string
	from     string
	password string
	enabled  bool
}

// NewService creates an email service from environment config
func NewService() *Service {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	from := os.Getenv("SMTP_FROM")
	password := os.Getenv("SMTP_PASSWORD")

	enabled := host != "" && from != ""
	if !enabled {
		log.Println("WARNING: Email service not configured (set SMTP_HOST, SMTP_FROM)")
	}

	return &Service{
		smtpHost: host,
		smtpPort: port,
		from:     from,
		password: password,
		enabled:  enabled,
	}
}

// Send sends an email with the given subject and HTML body
func (s *Service) Send(to, subject, htmlBody string) error {
	if !s.enabled {
		log.Printf("Email not sent (disabled): to=%s subject=%s", to, subject)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, htmlBody)

	auth := smtp.PlainAuth("", s.from, s.password, s.smtpHost)
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent: to=%s subject=%s", to, subject)
	return nil
}

// SendPaymentFailed sends a payment failure notification
func (s *Service) SendPaymentFailed(to, username, invoiceID string) error {
	subject := "APEX.BUILD -- Payment Failed"
	body := fmt.Sprintf(`<!DOCTYPE html>
<html><body style="font-family: -apple-system, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
<h2 style="color: #ef4444;">Payment Failed</h2>
<p>Hi %s,</p>
<p>We were unable to process your latest payment (Invoice: %s).</p>
<p>Please update your payment method to continue using APEX.BUILD without interruption:</p>
<p><a href="https://apex.build/settings" style="background: #ef4444; color: white; padding: 10px 20px; text-decoration: none; border-radius: 6px; display: inline-block;">Update Payment Method</a></p>
<p>If your payment method is not updated within 7 days, your account will be downgraded to the Free tier.</p>
<p>-- The APEX.BUILD Team</p>
</body></html>`, username, invoiceID)

	return s.Send(to, subject, body)
}

// IsEnabled returns whether the email service is configured
func (s *Service) IsEnabled() bool {
	return s.enabled
}
