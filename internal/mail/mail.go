package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type SMTP struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	Plain    bool
	Timeout  time.Duration
}

func (m SMTP) Send(ctx context.Context, to, subject, body string) error {
	timeout := m.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(m.Host, strconv.Itoa(m.Port)))
	if err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	defer c.Close()
	if !m.Plain {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("mail: server offers no STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if m.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	if _, err := w.Write(message(m.From, to, subject, body, time.Now())); err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	return c.Quit()
}

func message(from, to, subject, body string, now time.Time) []byte {
	var id [16]byte
	rand.Read(id[:])
	domain := from[strings.LastIndex(from, "@")+1:]
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from, to, subject)
	fmt.Fprintf(&b, "Date: %s\r\nMessage-ID: <%s@%s>\r\n", now.Format(time.RFC1123Z), hex.EncodeToString(id[:]), domain)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}
