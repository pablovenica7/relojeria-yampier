package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Mailer envía emails transaccionales (hoy: recuperación de contraseña).
// Está desacoplado: el flujo de negocio no sabe si se envía por SMTP o si
// solo se muestra en el log de desarrollo.
type Mailer interface {
	Send(to, subject, body string) error
}

// MAIL_DRIVER:
//   - "smtp": envía con SMTP_HOST/SMTP_PORT/SMTP_USERNAME/SMTP_PASSWORD/MAIL_FROM.
//   - "log":  SOLO desarrollo. Escribe el email (con el enlace) en el log del
//     backend para poder probar el flujo sin servidor de correo. Se rechaza
//     con APP_ENV=production, porque el log quedaría con enlaces válidos.
//   - vacío:  no se envían emails (el flujo responde igual, sin revelar nada).
func newMailerFromEnv() Mailer {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_DRIVER"))) {
	case "smtp":
		m := &smtpMailer{
			host: os.Getenv("SMTP_HOST"), port: os.Getenv("SMTP_PORT"),
			user: os.Getenv("SMTP_USERNAME"), pass: os.Getenv("SMTP_PASSWORD"),
			from: os.Getenv("MAIL_FROM"),
		}
		if m.port == "" {
			m.port = "587"
		}
		if m.host == "" || m.from == "" {
			log.Println("MAIL_DRIVER=smtp sin SMTP_HOST o MAIL_FROM: no se enviarán emails")
			return disabledMailer{}
		}
		return m
	case "log":
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			log.Println("MAIL_DRIVER=log no se permite con APP_ENV=production: no se enviarán emails")
			return disabledMailer{}
		}
		return logMailer{}
	default:
		return disabledMailer{}
	}
}

type disabledMailer struct{}

func (disabledMailer) Send(_, subject, _ string) error {
	log.Printf("Email no enviado (MAIL_DRIVER sin configurar): %q", subject)
	return nil
}

type logMailer struct{}

func (logMailer) Send(to, subject, body string) error {
	log.Printf("[MAIL_DRIVER=log · solo desarrollo] Para: %s | Asunto: %s\n%s", to, subject, body)
	return nil
}

type smtpMailer struct{ host, port, user, pass, from string }

// Send usa net/smtp, que exige TLS (STARTTLS) para autenticarse salvo contra
// localhost: las credenciales nunca viajan en claro.
func (m *smtpMailer) Send(to, subject, body string) error {
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
			return "=?UTF-8?B?" + base64Std(s) + "?="
		}
	}
	return s
}
