package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// SMTP credentials are read only by the server. Insecure transport is restricted to loopback test sinks.
type SMTP struct {
	Host, Port, Username, Password, From string
	Local                                bool
}

func SMTPFromEnv() *SMTP {
	port := os.Getenv("AUTH_SMTP_PORT")
	if port == "" {
		port = "587"
	}
	return &SMTP{Host: os.Getenv("AUTH_SMTP_HOST"), Port: port, Username: os.Getenv("AUTH_SMTP_USERNAME"), Password: os.Getenv("AUTH_SMTP_PASSWORD"), From: os.Getenv("AUTH_SMTP_FROM"), Local: os.Getenv("AUTH_SMTP_LOCAL_TEST") == "true"}
}
func (s *SMTP) Ready() bool { _, e := mail.ParseAddress(s.From); return s.Host != "" && e == nil }
func (s *SMTP) Send(ctx context.Context, to, code, purpose string) error {
	if !s.Ready() {
		return fmt.Errorf("邮件服务尚未配置")
	}
	from, e := mail.ParseAddress(s.From)
	if e != nil {
		return e
	}
	recipient, e := mail.ParseAddress(to)
	if e != nil || recipient.Address != to || strings.ContainsAny(to+s.From, "\r\n") {
		return fmt.Errorf("邮箱格式不正确")
	}
	conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Port))
	if e != nil {
		return fmt.Errorf("邮件发送失败，请稍后重试")
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if s.Port == "465" {
		t := tls.Client(conn, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
		if e = t.HandshakeContext(ctx); e != nil {
			return e
		}
		conn = t
	}
	c, e := smtp.NewClient(conn, s.Host)
	if e != nil {
		return e
	}
	defer c.Close()
	loop := s.Host == "localhost" || net.ParseIP(s.Host) != nil && net.ParseIP(s.Host).IsLoopback()
	if s.Port != "465" && !(s.Local && loop) {
		if e = c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); e != nil {
			return fmt.Errorf("邮件服务需要 TLS")
		}
	}
	if s.Username != "" {
		if e = c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); e != nil {
			return fmt.Errorf("邮件服务认证失败")
		}
	}
	if e = c.Mail(from.Address); e != nil {
		return e
	}
	if e = c.Rcpt(to); e != nil {
		return e
	}
	w, e := c.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: AskXuan verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n问玄东方：您的%s验证码为 %s，5 分钟内有效。请勿向他人提供验证码。如非本人操作，请忽略。\r\n", from.String(), to, purpose, code)
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return c.Quit()
}
