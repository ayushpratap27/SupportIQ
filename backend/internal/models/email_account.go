package models

import "time"

// EmailProvider identifies the email service provider.
type EmailProvider string

const (
	EmailProviderSMTPIMAP EmailProvider = "SMTP_IMAP"
	EmailProviderGmail    EmailProvider = "GMAIL"
	EmailProviderOutlook  EmailProvider = "OUTLOOK"
	EmailProviderSendGrid EmailProvider = "SENDGRID"
	EmailProviderSES      EmailProvider = "SES"
	EmailProviderMailgun  EmailProvider = "MAILGUN"
)

// EmailAccount holds IMAP/SMTP credentials for a configured mailbox.
// Passwords are stored AES-256-GCM encrypted and never returned through APIs.
type EmailAccount struct {
	ID                uint          `gorm:"primarykey;autoIncrement"            json:"id"`
	Provider          EmailProvider `gorm:"type:varchar(50);not null"           json:"provider"`
	EmailAddress      string        `gorm:"type:varchar(255);uniqueIndex;not null" json:"email_address"`
	DisplayName       string        `gorm:"type:varchar(100)"                   json:"display_name"`
	IMAPHost          string        `gorm:"type:varchar(255)"                   json:"imap_host"`
	IMAPPort          int           `gorm:"default:993"                         json:"imap_port"`
	IMAPUseTLS        bool          `gorm:"not null;default:true"               json:"imap_use_tls"`
	SMTPHost          string        `gorm:"type:varchar(255)"                   json:"smtp_host"`
	SMTPPort          int           `gorm:"default:587"                         json:"smtp_port"`
	SMTPImplicitTLS   bool          `gorm:"not null;default:false"              json:"smtp_implicit_tls"`
	Username          string        `gorm:"type:varchar(255);not null"          json:"username"`
	EncryptedPassword string        `gorm:"type:text;not null"                  json:"-"` // never serialised
	IsActive          bool          `gorm:"not null;default:true;index"         json:"is_active"`
	LastSyncAt        *time.Time    `                                            json:"last_sync_at,omitempty"`
	CreatedAt         time.Time     `                                            json:"created_at"`
	UpdatedAt         time.Time     `                                            json:"updated_at"`
}
