// Package mail implementa el envío de emails transaccionales. El flujo de
// negocio (service.PasswordResetService) solo conoce la interfaz Mailer.
package mail

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/smtp"
	"time"

	"relojeria-yampier/internal/config"
)

// Mailer envía un email de texto.
type Mailer interface {
	Send(to, subject, body string) error
}

// New elige la implementación según la configuración (ya validada en
// config.Load: MAIL_DRIVER=log nunca llega acá en producción).
func New(cfg config.MailConfig, log *slog.Logger) Mailer {
	switch cfg.Driver {
	case "smtp":
		return &SMTP{host: cfg.Host, port: cfg.Port, user: cfg.Username, pass: cfg.Password, from: cfg.From}
	case "log":
		return Log{log: log}
	default:
		return Disabled{log: log}
	}
}

// Disabled no envía nada (el flujo responde igual; queda un aviso sin datos).
type Disabled struct{ log *slog.Logger }

func (d Disabled) Send(_, subject, _ string) error {
	d.log.Warn("email no enviado (MAIL_DRIVER sin configurar)", "subject", subject)
	return nil
}

// Log escribe el email en el log. SOLO desarrollo: incluye el enlace de
// recuperación, por eso config.Load lo rechaza en producción.
type Log struct{ log *slog.Logger }

func (l Log) Send(to, subject, body string) error {
	l.log.Info("[MAIL_DRIVER=log · solo desarrollo] email", "to", to, "subject", subject, "body", body)
	return nil
}

// SMTP envía con net/smtp, que exige TLS (STARTTLS) para autenticarse salvo
// contra localhost: las credenciales nunca viajan en claro.
type SMTP struct{ host, port, user, pass, from string }

func (m *SMTP) Send(to, subject, body string) error {
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.from, to, mimeHeader(subject), time.Now().Format(time.RFC1123Z), body)
	return smtp.SendMail(m.host+":"+m.port, auth, m.from, []string{to}, []byte(msg))
}

// mimeHeader codifica el asunto si tiene caracteres no ASCII (tildes).
func mimeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}
