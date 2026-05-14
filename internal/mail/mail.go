package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	FromName     string `yaml:"from_name"`
	FromAddress  string `yaml:"from_address"`
	TLS          bool   `yaml:"tls"`
	TemplatesDir string `yaml:"templates_dir"`
}

type Message struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	HTML        string
	Attachments []Attachment
}

type Attachment struct {
	Filename string
	Data     []byte
	MimeType string
}

type Mailer struct {
	config    Config
	templates map[string]*template.Template
}

func New(cfg Config) (*Mailer, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("mail: host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("mail: from_address is required")
	}
	if cfg.TemplatesDir == "" {
		cfg.TemplatesDir = "templates/email"
	}

	m := &Mailer{
		config:    cfg,
		templates: make(map[string]*template.Template),
	}

	m.loadTemplates()
	return m, nil
}

func (m *Mailer) Send(msg *Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("mail: no recipients")
	}

	body := m.buildMessage(msg)
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)

	var auth smtp.Auth
	if m.config.Username != "" {
		auth = smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)
	}

	allRecipients := append(append(msg.To, msg.CC...), msg.BCC...)

	if m.config.TLS {
		return m.sendTLS(addr, auth, m.config.FromAddress, allRecipients, body)
	}

	return smtp.SendMail(addr, auth, m.config.FromAddress, allRecipients, body)
}

func (m *Mailer) SendTemplate(to []string, subject, templateName string, data interface{}) error {
	tmpl, ok := m.templates[templateName]
	if !ok {
		return fmt.Errorf("mail: template %q not found", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("mail: template execution failed: %w", err)
	}

	return m.Send(&Message{
		To:      to,
		Subject: subject,
		HTML:    buf.String(),
	})
}

func (m *Mailer) sendTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	host, _, _ := net.SplitHostPort(addr)

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		addr,
		&tls.Config{ServerName: host},
	)
	if err != nil {
		return fmt.Errorf("mail: tls dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("mail: smtp client failed: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mail: auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail: mail from failed: %w", err)
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("mail: rcpt to %s failed: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mail: write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: close failed: %w", err)
	}

	return client.Quit()
}

func (m *Mailer) buildMessage(msg *Message) []byte {
	var buf bytes.Buffer
	boundary := "nguyen-boundary-" + fmt.Sprintf("%d", time.Now().UnixNano())

	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", m.config.FromName, m.config.FromAddress))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	if len(msg.CC) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.CC, ", ")))
	}
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")

	hasAttachments := len(msg.Attachments) > 0
	hasHTML := msg.HTML != ""
	hasText := msg.Body != ""

	if hasAttachments {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))

		if hasHTML {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
			buf.WriteString(msg.HTML)
			buf.WriteString("\r\n")
		} else if hasText {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			buf.WriteString(msg.Body)
			buf.WriteString("\r\n")
		}

		for _, att := range msg.Attachments {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			mimeType := att.MimeType
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			buf.WriteString(fmt.Sprintf("Content-Type: %s; name=%q\r\n", mimeType, att.Filename))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%q\r\n\r\n", att.Filename))
			buf.WriteString(base64.StdEncoding.EncodeToString(att.Data))
			buf.WriteString("\r\n")
		}
		buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else if hasHTML {
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		buf.WriteString(msg.HTML)
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		buf.WriteString(msg.Body)
	}

	return buf.Bytes()
}

func (m *Mailer) loadTemplates() {
	dir := m.config.TemplatesDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".html") {
			continue
		}

		tmplName := strings.TrimSuffix(name, ".html")
		tmpl, err := template.ParseFiles(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		m.templates[tmplName] = tmpl
	}
}
