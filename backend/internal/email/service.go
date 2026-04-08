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
	username string // SMTP auth username (defaults to from address; Resend requires "resend")
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
	// SMTP_USERNAME allows providers like Resend that require a fixed username
	// ("resend") independent of the from address. Defaults to the from address
	// for standard SMTP providers (Gmail, SES direct, etc.).
	username := os.Getenv("SMTP_USERNAME")
	if username == "" {
		username = from
	}

	enabled := host != "" && from != ""
	if !enabled {
		log.Println("WARNING: Email service not configured (set SMTP_HOST, SMTP_FROM)")
	}

	return &Service{
		smtpHost: host,
		smtpPort: port,
		username: username,
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

	auth := smtp.PlainAuth("", s.username, s.password, s.smtpHost)
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
<p><a href="https://apex-build.dev/settings" style="background: #ef4444; color: white; padding: 10px 20px; text-decoration: none; border-radius: 6px; display: inline-block;">Update Payment Method</a></p>
<p>If your payment method is not updated within 7 days, your account will be downgraded to the Free tier.</p>
<p>-- The APEX.BUILD Team</p>
</body></html>`, username, invoiceID)

	return s.Send(to, subject, body)
}

// SendVerificationCode sends a 6-digit email verification code to the user.
func (s *Service) SendVerificationCode(to, username, code string) error {
	subject := "APEX.BUILD — Verify your email"
	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#0f172a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#0f172a;min-height:100vh;">
    <tr><td align="center" style="padding:40px 20px;">
      <table width="560" cellpadding="0" cellspacing="0" style="background:#1e293b;border-radius:12px;border:1px solid #334155;overflow:hidden;">
        <tr>
          <td style="padding:32px 40px;border-bottom:1px solid #334155;text-align:center;">
            <span style="font-size:24px;font-weight:700;color:#f8fafc;letter-spacing:-0.5px;">APEX<span style="color:#6366f1;">.BUILD</span></span>
          </td>
        </tr>
        <tr>
          <td style="padding:40px;">
            <h1 style="margin:0 0 8px;font-size:22px;font-weight:600;color:#f8fafc;">Verify your email</h1>
            <p style="margin:0 0 32px;color:#94a3b8;font-size:15px;line-height:1.6;">
              Hey %s! Enter the code below in the APEX.BUILD app to verify your email address.
            </p>
            <div style="background:#0f172a;border:1px solid #6366f1;border-radius:10px;padding:28px;text-align:center;margin-bottom:32px;">
              <div style="font-size:40px;font-weight:700;letter-spacing:12px;color:#f8fafc;font-variant-numeric:tabular-nums;">%s</div>
            </div>
            <p style="margin:0 0 8px;color:#64748b;font-size:13px;text-align:center;">
              This code expires in <strong style="color:#94a3b8;">15 minutes</strong>.
            </p>
            <p style="margin:0;color:#64748b;font-size:13px;text-align:center;">
              If you didn't create an account, you can safely ignore this email.
            </p>
          </td>
        </tr>
        <tr>
          <td style="padding:20px 40px;border-top:1px solid #334155;text-align:center;">
            <p style="margin:0;color:#475569;font-size:12px;">APEX.BUILD — AI-powered app building</p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, username, code)

	return s.Send(to, subject, body)
}

// IsEnabled returns whether the email service is configured
func (s *Service) IsEnabled() bool {
	return s.enabled
}
