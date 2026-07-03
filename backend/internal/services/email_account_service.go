package services

import (
	"fmt"
	"net/http"

	emailcrypto "github.com/ayush/supportiq/internal/email/crypto"
	emailproviders "github.com/ayush/supportiq/internal/email/providers"
	smtpprovider "github.com/ayush/supportiq/internal/email/providers/smtp"
	imapprovider "github.com/ayush/supportiq/internal/email/providers/imap"
	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/utils"
)

// EmailAccountService manages CRUD and connection testing for email accounts.
type EmailAccountService struct {
	repo          *repositories.EmailAccountRepository
	encryptionKey string // JWT_ACCESS_SECRET — used to derive the AES key
}

func NewEmailAccountService(repo *repositories.EmailAccountRepository, encryptionKey string) *EmailAccountService {
	return &EmailAccountService{repo: repo, encryptionKey: encryptionKey}
}

// List returns all accounts (credentials stripped from response).
func (s *EmailAccountService) List() ([]dto.EmailAccountResponse, int, error) {
	accounts, err := s.repo.FindAll()
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to list email accounts")
	}
	resp := make([]dto.EmailAccountResponse, len(accounts))
	for i := range accounts {
		resp[i] = dto.ToEmailAccountResponse(&accounts[i])
	}
	return resp, http.StatusOK, nil
}

// Create persists a new email account after encrypting the password.
func (s *EmailAccountService) Create(req *dto.CreateEmailAccountRequest) (*dto.EmailAccountResponse, int, error) {
	encrypted, err := emailcrypto.Encrypt(s.encryptionKey, req.Password)
	if err != nil {
		utils.Logger.WithError(err).Error("EmailAccount: password encryption failed")
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to secure credentials")
	}

	imapPort := req.IMAPPort
	if imapPort == 0 {
		imapPort = 993
	}
	smtpPort := req.SMTPPort
	if smtpPort == 0 {
		smtpPort = 587
	}

	account := &models.EmailAccount{
		Provider:          models.EmailProvider(req.Provider),
		EmailAddress:      req.EmailAddress,
		DisplayName:       req.DisplayName,
		IMAPHost:          req.IMAPHost,
		IMAPPort:          imapPort,
		IMAPUseTLS:        req.IMAPUseTLS,
		SMTPHost:          req.SMTPHost,
		SMTPPort:          smtpPort,
		SMTPImplicitTLS:   req.SMTPImplicitTLS,
		Username:          req.Username,
		EncryptedPassword: encrypted,
		IsActive:          req.IsActive,
	}
	if !req.IsActive {
		account.IsActive = true // default to active
	}

	if err := s.repo.Create(account); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create email account")
	}

	resp := dto.ToEmailAccountResponse(account)
	return &resp, http.StatusCreated, nil
}

// Update applies partial updates to an existing account.
func (s *EmailAccountService) Update(id uint, req *dto.UpdateEmailAccountRequest) (*dto.EmailAccountResponse, int, error) {
	account, err := s.repo.FindByID(id)
	if err != nil {
		return nil, http.StatusNotFound, fmt.Errorf("email account not found")
	}

	if req.DisplayName != nil {
		account.DisplayName = *req.DisplayName
	}
	if req.IMAPHost != nil {
		account.IMAPHost = *req.IMAPHost
	}
	if req.IMAPPort != nil {
		account.IMAPPort = *req.IMAPPort
	}
	if req.IMAPUseTLS != nil {
		account.IMAPUseTLS = *req.IMAPUseTLS
	}
	if req.SMTPHost != nil {
		account.SMTPHost = *req.SMTPHost
	}
	if req.SMTPPort != nil {
		account.SMTPPort = *req.SMTPPort
	}
	if req.SMTPImplicitTLS != nil {
		account.SMTPImplicitTLS = *req.SMTPImplicitTLS
	}
	if req.Username != nil {
		account.Username = *req.Username
	}
	if req.Password != nil && *req.Password != "" {
		encrypted, err := emailcrypto.Encrypt(s.encryptionKey, *req.Password)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to secure credentials")
		}
		account.EncryptedPassword = encrypted
	}
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}

	if err := s.repo.Update(account); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update email account")
	}

	resp := dto.ToEmailAccountResponse(account)
	return &resp, http.StatusOK, nil
}

// Delete removes an email account by ID.
func (s *EmailAccountService) Delete(id uint) (int, error) {
	if _, err := s.repo.FindByID(id); err != nil {
		return http.StatusNotFound, fmt.Errorf("email account not found")
	}
	if err := s.repo.Delete(id); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete email account")
	}
	return http.StatusOK, nil
}

// TestSMTP tests the SMTP connection for an account.
func (s *EmailAccountService) TestSMTP(id uint) error {
	account, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("email account not found")
	}
	if account.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}

	pass, err := emailcrypto.Decrypt(s.encryptionKey, account.EncryptedPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials")
	}

	client := smtpprovider.New(
		account.SMTPHost, account.SMTPPort,
		account.Username, pass,
		account.EmailAddress, account.DisplayName,
		account.SMTPImplicitTLS,
	)
	return client.TestConnection(nil)
}

// TestIMAP tests the IMAP connection for an account.
func (s *EmailAccountService) TestIMAP(id uint) error {
	account, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("email account not found")
	}
	if account.IMAPHost == "" {
		return fmt.Errorf("IMAP host not configured")
	}

	pass, err := emailcrypto.Decrypt(s.encryptionKey, account.EncryptedPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials")
	}

	client := imapprovider.New(account.IMAPHost, account.IMAPPort, account.Username, pass, account.IMAPUseTLS)
	return client.TestConnection(nil)
}

// BuildSender decrypts credentials and returns a ready-to-use Sender.
func (s *EmailAccountService) BuildSender(account *models.EmailAccount) (emailproviders.Sender, error) {
	pass, err := emailcrypto.Decrypt(s.encryptionKey, account.EncryptedPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt SMTP credentials")
	}
	return smtpprovider.New(
		account.SMTPHost, account.SMTPPort,
		account.Username, pass,
		account.EmailAddress, account.DisplayName,
		account.SMTPImplicitTLS,
	), nil
}

// BuildReceiver decrypts credentials and returns a ready-to-use Receiver.
func (s *EmailAccountService) BuildReceiver(account *models.EmailAccount) (emailproviders.Receiver, error) {
	pass, err := emailcrypto.Decrypt(s.encryptionKey, account.EncryptedPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt IMAP credentials")
	}
	return imapprovider.New(account.IMAPHost, account.IMAPPort, account.Username, pass, account.IMAPUseTLS), nil
}
