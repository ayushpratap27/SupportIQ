package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	emailcrypto "github.com/ayush/supportiq/internal/email/crypto"
	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/integrations"
	"github.com/ayush/supportiq/internal/integrations/provider"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IntegrationService handles business logic for integration management.
type IntegrationService struct {
	repo          *repositories.IntegrationRepository
	ticketRepo    *repositories.TicketRepository
	registry      *integrations.Registry
	encryptionKey string
}

// NewIntegrationService creates a new IntegrationService.
func NewIntegrationService(
	repo *repositories.IntegrationRepository,
	ticketRepo *repositories.TicketRepository,
	registry *integrations.Registry,
	encryptionKey string,
) *IntegrationService {
	return &IntegrationService{
		repo:          repo,
		ticketRepo:    ticketRepo,
		registry:      registry,
		encryptionKey: encryptionKey,
	}
}

// List returns all configured integrations.
func (s *IntegrationService) List() ([]dto.IntegrationResponse, error) {
	integrationList, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	resp := make([]dto.IntegrationResponse, 0, len(integrationList))
	for _, intg := range integrationList {
		resp = append(resp, toIntegrationResponse(intg))
	}
	return resp, nil
}

// Create validates, encrypts the config, and stores a new integration.
func (s *IntegrationService) Create(req dto.CreateIntegrationRequest, userID uint) (*dto.IntegrationResponse, error) {
	// Build provider to validate config before persisting
	prov, err := s.registry.Build(req.Provider, req.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	_ = prov // validation only at creation; TestConnection is explicit

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	encrypted, err := emailcrypto.Encrypt(s.encryptionKey, string(configJSON))
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}

	intg := &models.Integration{
		Provider:      models.IntegrationProvider(req.Provider),
		Name:          req.Name,
		Configuration: encrypted,
		Status:        models.IntegrationStatusInactive,
		Enabled:       req.Enabled,
		CreatedBy:     userID,
	}
	if err := s.repo.Create(intg); err != nil {
		return nil, err
	}
	resp := toIntegrationResponse(*intg)
	return &resp, nil
}

// Update applies changes to an existing integration.
func (s *IntegrationService) Update(id uint, req dto.UpdateIntegrationRequest) (*dto.IntegrationResponse, error) {
	intg, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		intg.Name = *req.Name
	}
	if req.Enabled != nil {
		intg.Enabled = *req.Enabled
	}
	if req.Config != nil {
		// Validate new config
		if _, err := s.registry.Build(string(intg.Provider), req.Config); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
		configJSON, _ := json.Marshal(req.Config)
		encrypted, err := emailcrypto.Encrypt(s.encryptionKey, string(configJSON))
		if err != nil {
			return nil, fmt.Errorf("encrypt config: %w", err)
		}
		intg.Configuration = encrypted
		intg.Status = models.IntegrationStatusInactive // re-test required
	}

	if err := s.repo.Update(intg); err != nil {
		return nil, err
	}
	resp := toIntegrationResponse(*intg)
	return &resp, nil
}

// Delete removes an integration by ID.
func (s *IntegrationService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// TestConnection attempts a live connection test for the integration.
func (s *IntegrationService) TestConnection(ctx context.Context, id uint) error {
	intg, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	prov, err := s.buildProvider(*intg)
	if err != nil {
		s.setError(intg, err.Error())
		return err
	}

	if err := prov.TestConnection(ctx); err != nil {
		s.setError(intg, err.Error())
		return err
	}

	// Mark active
	now := time.Now()
	intg.Status = models.IntegrationStatusActive
	intg.ErrorMessage = ""
	intg.LastSyncAt = &now
	_ = s.repo.Update(intg)
	return nil
}

// GetTicketIntegrations returns all external issue links for a ticket.
func (s *IntegrationService) GetTicketIntegrations(ticketID uuid.UUID) ([]dto.TicketIntegrationResponse, error) {
	items, err := s.repo.FindTicketIntegrations(ticketID)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.TicketIntegrationResponse, 0, len(items))
	for _, item := range items {
		r := dto.TicketIntegrationResponse{
			ID:            item.ID,
			IntegrationID: item.IntegrationID,
			ExternalKey:   item.ExternalKey,
			ExternalURL:   item.ExternalURL,
			SyncedAt:      item.SyncedAt,
			CreatedAt:     item.CreatedAt,
		}
		if item.Integration != nil {
			r.Provider = string(item.Integration.Provider)
			r.Name = item.Integration.Name
		}
		resp = append(resp, r)
	}
	return resp, nil
}

// CreateJiraIssue creates a Jira issue for a ticket.
func (s *IntegrationService) CreateJiraIssue(ctx context.Context, ticketID uuid.UUID) (*dto.TicketIntegrationResponse, error) {
	return s.createIssue(ctx, ticketID, "jira")
}

// CreateLinearIssue creates a Linear issue for a ticket.
func (s *IntegrationService) CreateLinearIssue(ctx context.Context, ticketID uuid.UUID) (*dto.TicketIntegrationResponse, error) {
	return s.createIssue(ctx, ticketID, "linear")
}

// CreateGitHubIssue creates a GitHub issue for a ticket.
func (s *IntegrationService) CreateGitHubIssue(ctx context.Context, ticketID uuid.UUID) (*dto.TicketIntegrationResponse, error) {
	return s.createIssue(ctx, ticketID, "github")
}

func (s *IntegrationService) createIssue(ctx context.Context, ticketID uuid.UUID, providerType string) (*dto.TicketIntegrationResponse, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	providerIntgs, err := s.repo.FindByProvider(models.IntegrationProvider(providerType))
	if err != nil || len(providerIntgs) == 0 {
		return nil, fmt.Errorf("%s integration not configured", providerType)
	}
	intg := providerIntgs[0]

	prov, err := s.buildProvider(intg)
	if err != nil {
		return nil, err
	}

	issueProv, ok := prov.(provider.IssueProvider)
	if !ok {
		return nil, fmt.Errorf("%s does not support issue creation", providerType)
	}

	ref, err := issueProv.CreateIssue(ctx, ticket)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	now := time.Now()
	ti := &models.TicketIntegration{
		TicketID:      ticket.ID,
		IntegrationID: intg.ID,
		ExternalID:    ref.ExternalID,
		ExternalKey:   ref.ExternalKey,
		ExternalURL:   ref.ExternalURL,
		SyncedAt:      &now,
	}
	if err := s.repo.CreateTicketIntegration(ti); err != nil {
		// Non-fatal: issue was created, just couldn't save the link
		_ = err
	}

	return &dto.TicketIntegrationResponse{
		ID:            ti.ID,
		IntegrationID: intg.ID,
		Provider:      string(intg.Provider),
		Name:          intg.Name,
		ExternalKey:   ref.ExternalKey,
		ExternalURL:   ref.ExternalURL,
		SyncedAt:      &now,
		CreatedAt:     ti.CreatedAt,
	}, nil
}

// ListEvents returns the most recent events for an integration.
func (s *IntegrationService) ListEvents(integrationID uint) ([]dto.IntegrationEventResponse, error) {
	// Verify the integration exists
	if _, err := s.repo.FindByID(integrationID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, err
	}
	events, err := s.repo.FindPendingEvents(100)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.IntegrationEventResponse, 0, len(events))
	for _, evt := range events {
		if evt.IntegrationID != integrationID {
			continue
		}
		resp = append(resp, dto.IntegrationEventResponse{
			ID:            evt.ID,
			IntegrationID: evt.IntegrationID,
			EventType:     evt.EventType,
			Status:        string(evt.Status),
			RetryCount:    evt.RetryCount,
			ErrorMessage:  evt.ErrorMessage,
			CreatedAt:     evt.CreatedAt,
			ProcessedAt:   evt.ProcessedAt,
		})
	}
	return resp, nil
}

func (s *IntegrationService) buildProvider(intg models.Integration) (provider.Provider, error) {
	plaintext, err := emailcrypto.Decrypt(s.encryptionKey, intg.Configuration)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(plaintext), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return s.registry.Build(string(intg.Provider), cfg)
}

func (s *IntegrationService) setError(intg *models.Integration, msg string) {
	intg.Status = models.IntegrationStatusError
	intg.ErrorMessage = msg
	_ = s.repo.Update(intg)
}

func toIntegrationResponse(intg models.Integration) dto.IntegrationResponse {
	return dto.IntegrationResponse{
		ID:           intg.ID,
		Provider:     string(intg.Provider),
		Name:         intg.Name,
		Status:       string(intg.Status),
		Enabled:      intg.Enabled,
		CreatedBy:    intg.CreatedBy,
		LastSyncAt:   intg.LastSyncAt,
		ErrorMessage: intg.ErrorMessage,
		CreatedAt:    intg.CreatedAt,
		UpdatedAt:    intg.UpdatedAt,
	}
}
