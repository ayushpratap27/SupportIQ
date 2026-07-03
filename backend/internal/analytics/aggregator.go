package analytics

import (
	"time"

	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/utils"
)

// Aggregator computes and persists pre-aggregated metrics rows.
// It is called by the Scheduler on a regular interval and by the manual
// POST /analytics/aggregate endpoint.
type Aggregator struct {
	repo *AnalyticsRepository
}

// NewAggregator creates an Aggregator.
func NewAggregator(repo *AnalyticsRepository) *Aggregator {
	return &Aggregator{repo: repo}
}

// RunAll aggregates tickets, AI, and agent metrics for today and yesterday.
func (a *Aggregator) RunAll() {
	today := truncateDay(time.Now())
	yesterday := today.AddDate(0, 0, -1)

	a.AggregateDailyTickets(today)
	a.AggregateDailyTickets(yesterday)
	a.AggregateAIMetrics(today)
	a.AggregateAIMetrics(yesterday)
	a.AggregateAllAgents()
}

// AggregateDailyTickets computes and upserts DailyTicketMetrics for the given date.
func (a *Aggregator) AggregateDailyTickets(date time.Time) {
	m, err := a.repo.ComputeDailyTickets(date)
	if err != nil {
		utils.Logger.WithError(err).WithField("date", date).
			Error("Aggregator: failed to compute daily ticket metrics")
		return
	}
	if err := a.repo.UpsertDailyTicketMetrics(&m); err != nil {
		utils.Logger.WithError(err).WithField("date", date).
			Error("Aggregator: failed to upsert daily ticket metrics")
	}
}

// AggregateAIMetrics computes and upserts AIMetrics for the given date.
func (a *Aggregator) AggregateAIMetrics(date time.Time) {
	m, err := a.repo.ComputeAIMetrics(date)
	if err != nil {
		utils.Logger.WithError(err).WithField("date", date).
			Error("Aggregator: failed to compute AI metrics")
		return
	}
	if err := a.repo.UpsertAIMetrics(&m); err != nil {
		utils.Logger.WithError(err).WithField("date", date).
			Error("Aggregator: failed to upsert AI metrics")
	}
}

// AggregateAllAgents recalculates metrics for every active agent.
func (a *Aggregator) AggregateAllAgents() {
	agents, err := a.repo.AllSupportAgents()
	if err != nil {
		utils.Logger.WithError(err).Error("Aggregator: failed to fetch agents")
		return
	}
	for _, agent := range agents {
		a.aggregateAgent(agent)
	}
}

func (a *Aggregator) aggregateAgent(agent models.User) {
	raw, err := a.repo.ComputeAgentMetrics(agent.ID)
	if err != nil {
		utils.Logger.WithError(err).WithField("userID", agent.ID).
			Error("Aggregator: failed to compute agent metrics")
		return
	}
	m := &models.AgentMetrics{
		UserID:                agent.ID,
		TicketsAssigned:       int(raw.Assigned),
		TicketsResolved:       int(raw.Resolved),
		AverageResolutionTime: raw.AvgResolutionH,
		AverageReplyTime:      raw.AvgReplyH,
		LastCalculated:        time.Now().UTC(),
	}
	if err := a.repo.UpsertAgentMetrics(m); err != nil {
		utils.Logger.WithError(err).WithField("userID", agent.ID).
			Error("Aggregator: failed to upsert agent metrics")
	}
}
