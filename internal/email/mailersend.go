package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

const mailerSendAPIURL = "https://api.mailersend.com/v1/email"

// Кароче это минимальный интерфейс для отправки электронных писем для входа в систему OTP
// Он намеренно отделен от существующего интерфейса Sender, поэтому доставка OTP может использовать MailerSend независимо от SendGrid
type OTPSender interface {
	SendLoginOTP(toEmail, code string) error
}

// Он отправляет электронные письма OTP через REST API MailerSend
type MailerSendOTPSender struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
	logger    *slog.Logger
}

// Он выводит OTP на стандартный вывод
type ConsoleOTPSender struct {
	logger *slog.Logger
}

// Функция создает OTPSender. Возвращается к выводу консоли когда apiKey пуст будет
func NewOTPSender(apiKey, fromEmail, fromName string, logger *slog.Logger) OTPSender {
	if apiKey == "" {
		logger.Warn("MAILERSEND_API_KEY not set so OTP emails will be printed to console")
		return &ConsoleOTPSender{logger: logger}
	}
	logger.Info("MailerSend OTP sender configured", "from_email", fromEmail)
	return &MailerSendOTPSender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client:    &http.Client{},
		logger:    logger,
	}
}

// mailerSendPayload отражает тело запроса MailerSend /v1/email.
type mailerSendPayload struct {
	From    mailerSendAddress   `json:"from"`
	To      []mailerSendAddress `json:"to"`
	Subject string              `json:"subject"`
	Text    string              `json:"text"`
	HTML    string              `json:"html"`
}

type mailerSendAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func (s *MailerSendOTPSender) SendLoginOTP(toEmail, code string) error {
	plainText := fmt.Sprintf(
		"Your DrivaBai login code is: %s\n\nThis code expires in 10 minutes.\n\nIf you did not request this, you can safely ignore this email.\n\nThe DrivaBai Team",
		code,
	)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8">
<style>
  body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;line-height:1.6;color:#333}
  .container{max-width:600px;margin:0 auto;padding:20px}
  .code{font-size:36px;font-weight:bold;letter-spacing:10px;color:#4ECDC4;text-align:center;padding:24px;background:#f5f5f5;border-radius:8px;margin:24px 0}
  .footer{margin-top:30px;font-size:12px;color:#666}
</style>
</head>
<body>
<div class="container">
  <h2>Your DrivaBai login code</h2>
  <p>Use the code below to sign in. It expires in <strong>10 minutes</strong>.</p>
  <div class="code">%s</div>
  <p>If you did not request this code, you can safely ignore this email.</p>
  <div class="footer"><p>The DrivaBai Team</p></div>
</div>
</body>
</html>`, code)

	payload := mailerSendPayload{
		From:    mailerSendAddress{Email: s.fromEmail, Name: s.fromName},
		To:      []mailerSendAddress{{Email: toEmail}},
		Subject: "Your DrivaBai login code",
		Text:    plainText,
		HTML:    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailersend: marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, mailerSendAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailersend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("mailersend: request failed", "error", err, "to", toEmail)
		return fmt.Errorf("mailersend: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.logger.Error("mailersend: non-2xx response", "status", resp.StatusCode, "to", toEmail)
		return fmt.Errorf("mailersend: API returned status %d", resp.StatusCode)
	}

	s.logger.Info("OTP email sent via MailerSend", "to", toEmail, "status", resp.StatusCode)
	return nil
}

func (s *ConsoleOTPSender) SendLoginOTP(toEmail, code string) error {
	// VAZHNOOO. Регистрация кода OTP предусмотрена только в режиме разработки
	// В производстве всегда используется MailerSendOTPSender который никогда не регистрирует код
	s.logger.Info("OTP EMAIL (dev mode — MailerSend not configured)",
		"to", toEmail,
	)
	fmt.Printf("\n"+
		"╔══════════════════════════════════════════════════════════╗\n"+
		"║  📧 LOGIN OTP EMAIL (MailerSend not configured)          ║\n"+
		"╠══════════════════════════════════════════════════════════╣\n"+
		"║  To:   %-50s ║\n"+
		"║  Code: %-50s ║\n"+
		"╚══════════════════════════════════════════════════════════╝\n\n",
		toEmail, code)
	return nil
}