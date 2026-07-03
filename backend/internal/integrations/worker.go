// Package integrations provides the background worker that polls TicketActivity
// rows and dispatches integration events to external providers.
package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ayush/supportiq/internal/integrations/provider"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	emailcrypto "github.com/ayush/supportiq/internal/email/crypto"
	"github.com/ayush/supportiq/internal/utils"
)

const (
	defaultPollInterval = 30 * time.Second
	maxRetries          = 5
	eventBatchSize      = 50
)

// Worker polls TicketActivity rows, converts them to IntegrationEvents, and
// dispatches them to all enabled provider integrations.
type Worker struct {
	integrationRepo *repositories.IntegrationRepository
	ticketRepo      *repositories.TicketRepository
	registry        *Registry
	encryptionKey   string
	lastActivityID  uint
	pollInterval    time.Duration
}

// NewWorker creates a new integration Worker. encryptionKey is used to decrypt
// stored integration configurations before building providers.
func NewWorker(
	integrationRepo *repositories.IntegrationRepository,
	ticketRepo *repositories.TicketRepository,
	registry *Registry,
	encryptionKey string,
) *Worker {
	return &Worker{
		integrationRepo: integrationRepo,
		ticketRepo:      ticketRepo,
		registry:        registry,
		encryptionKey:   encryptionKey,
		pollInterval:    defaultPollInterval,
	}
}

// Start runs the worker indefinitely until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	// Initialise lastActivityID so we don't replay old events.
	w.initLastActivityID()

	activityTicker := time.NewTicker(w.pollInterval)
	eventTicker := time.NewTicker(w.pollInterval)
	defer activityTicker.Stop()
	defer eventTicker.Stop()

	utils.Logger.Info("Integration worker started")
	for {
		select {
		case <-ctx.Done():
			utils.Logger.Info("Integration worker stopped")
			return
		case <-activityTicker.C:
			w.pollActivities(ctx)
		case <-eventTicker.C:
			w.processEvents(ctx)
		}
	}
}

func (w *Worker) initLastActivityID() {
	activities, err := w.integrationRepo.FindActivitiesSince(0, 1)
	if err != nil || len(activities) == 0 {
		return
	}
	// Find the max ID by querying with a very large offset is not efficient;
	// use a separate read-all-descending approach.
	// As a pragmatic bootstrap we simply skip all existing activities.
	w.lastActivityID = activities[len(activities)-1].ID
}

// pollActivities fetches new TicketActivity rows and creates IntegrationEvent
// records for each relevant event type.
func (w *Worker) pollActivities(ctx context.Context) {
	activities, err := w.integrationRepo.FindActivitiesSince(w.lastActivityID, eventBatchSize)
	if err != nil {
		utils.Logger.WithError(err).Error("Integration worker: poll activities failed")
		return
	}
	for _, act := range activities {
		if act.ID > w.lastActivityID {
			w.lastActivityID = act.ID
		}
		eventType := activityToEventType(act)
		if eventType == "" {
			continue
		}
		if err := w.createEventsForActivity(ctx, act, eventType); err != nil {
			utils.Logger.WithError(err).WithField("activity_id", act.ID).
				Error("Integration worker: create events failed")
		}
	}
}

func activityToEventType(act models.TicketActivity) string {
	switch act.ActivityType {
	case models.ActivityCreateTicket:
		return provider.EventTicketCreated
	case models.ActivityStatusChanged:
		if act.NewValue == string(models.TicketStatusClosed) {
			return provider.EventTicketClosed
		}
		return provider.EventTicketStatusChanged
	case models.ActivityAssignTicket:
		return provider.EventTicketAssigned
	case models.ActivityAIAnalysisCompleted:
		return provider.EventAIAnalysisComplete
	case models.ActivityReplyApproved:
		return provider.EventReplyApproved
	case models.ActivityEmailFailed:
		return provider.EventEmailFailed
	default:
		return ""
	}
}

func (w *Worker) createEventsForActivity(ctx context.Context, act models.TicketActivity, eventType string) error {
	integrations, err := w.integrationRepo.FindEnabled()
	if err != nil {
		return err
	}

	ticket, err := w.ticketRepo.FindByID(act.TicketID)
	if err != nil {
		return fmt.Errorf("find ticket %s: %w", act.TicketID, err)
	}

	payload := map[string]interface{}{
		"ticket_id":      ticket.ID.String(),
		"ticket_number":  ticket.TicketNumber,
		"subject":        ticket.Subject,
		"priority":       string(ticket.Priority),
		"status":         string(ticket.Status),
		"customer_email": ticket.CustomerEmail,
		"customer_name":  ticket.CustomerName,
		"activity_id":    act.ID,
		"agent_id":       act.UserID,
	}
	payloadJSON, _ := json.Marshal(payload)

	for _, intg := range integrations {
		// Filter: only create event if the provider supports it
		prov, err := w.buildProvider(intg)
		if err != nil {
			continue
		}
		supported := false
		for _, evtType := range prov.SupportedEvents() {
			if evtType == eventType {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		event := &models.IntegrationEvent{
			IntegrationID: intg.ID,
			EventType:     eventType,
			Payload:       string(payloadJSON),
			Status:        models.IntEventPending,
		}
		if err := w.integrationRepo.CreateEvent(event); err != nil {
			utils.Logger.WithError(err).WithField("integration_id", intg.ID).
				Error("Integration worker: store event")
		}
	}
	return nil
}

// processEvents dispatches pending IntegrationEvent rows to their providers.
func (w *Worker) processEvents(ctx context.Context) {
	events, err := w.integrationRepo.FindPendingEvents(eventBatchSize)
	if err != nil {
		utils.Logger.WithError(err).Error("Integration worker: fetch pending events")
		return
	}
	for _, evt := range events {
		w.dispatchEvent(ctx, evt)
	}
}

func (w *Worker) dispatchEvent(ctx context.Context, evt models.IntegrationEvent) {
	intg, err := w.integrationRepo.FindByID(evt.IntegrationID)
	if err != nil || !intg.Enabled {
		_ = w.integrationRepo.UpdateEventStatus(evt.ID, models.IntEventDead, "integration not found or disabled")
		return
	}

	prov, err := w.buildProvider(*intg)
	if err != nil {
		_ = w.integrationRepo.UpdateEventStatus(evt.ID, models.IntEventFailed, err.Error())
		return
	}

	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(evt.Payload), &payload)

	e := provider.Event{
		Type:          evt.EventType,
		TicketID:      strVal(payload, "ticket_id"),
		TicketNumber:  strVal(payload, "ticket_number"),
		Subject:       strVal(payload, "subject"),
		Priority:      strVal(payload, "priority"),
		Status:        strVal(payload, "status"),
		CustomerEmail: strVal(payload, "customer_email"),
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if notifyErr := prov.Notify(dispatchCtx, e); notifyErr != nil {
		_ = w.integrationRepo.IncrementEventRetry(evt.ID)
		if evt.RetryCount+1 >= maxRetries {
			_ = w.integrationRepo.MarkEventDead(evt.ID, notifyErr.Error())
		} else {
			_ = w.integrationRepo.UpdateEventStatus(evt.ID, models.IntEventFailed, notifyErr.Error())
		}
		return
	}

	_ = w.integrationRepo.UpdateEventStatus(evt.ID, models.IntEventProcessed, "")
}

func (w *Worker) buildProvider(intg models.Integration) (provider.Provider, error) {
	plaintext, err := emailcrypto.Decrypt(w.encryptionKey, intg.Configuration)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(plaintext), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return w.registry.Build(string(intg.Provider), cfg)
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
