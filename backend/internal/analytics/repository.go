package analytics

import (
	"time"

	"github.com/ayush/supportiq/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AnalyticsRepository encapsulates all raw database queries used by analytics.
// It reads from application tables (tickets, ai_replies, background_jobs,
// email_messages) and also owns the aggregated metrics tables.
type AnalyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository creates an AnalyticsRepository.
func NewAnalyticsRepository(db *gorm.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// ─── Live ticket counts ─────────────────────────────────────────────────────

type StatusCount struct {
	Status string
	Count  int64
}

type LabelCount struct {
	Label string
	Count int64
}

// CountTicketsByStatus returns open/in-progress/resolved/closed ticket counts.
func (r *AnalyticsRepository) CountTicketsByStatus() ([]StatusCount, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.Model(&models.Ticket{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]StatusCount, len(rows))
	for i, row := range rows {
		out[i] = StatusCount{Status: row.Status, Count: row.Count}
	}
	return out, nil
}

// CountTicketsByPriority returns ticket counts grouped by priority.
func (r *AnalyticsRepository) CountTicketsByPriority(start, end time.Time) ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("priority as label, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("priority").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// CountTicketsByCategory returns ticket counts grouped by category.
func (r *AnalyticsRepository) CountTicketsByCategory(start, end time.Time) ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("COALESCE(NULLIF(ai_category,''), category) as label, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("label").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// CountTicketsBySource returns ticket counts grouped by source channel.
func (r *AnalyticsRepository) CountTicketsBySource(start, end time.Time) ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("source as label, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("source").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// CountTicketsByHour returns ticket creation count per hour-of-day for the
// given date (useful for today's heat-map).
func (r *AnalyticsRepository) CountTicketsByHour(date time.Time) ([]LabelCount, error) {
	dayStart := truncateDay(date)
	dayEnd := dayStart.Add(24 * time.Hour)
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("TO_CHAR(created_at, 'HH24') as label, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", dayStart, dayEnd).
		Group("label").
		Order("label ASC").
		Find(&rows).Error
	return rows, err
}

// CountTicketsInRange returns total tickets created in a date range.
func (r *AnalyticsRepository) CountTicketsInRange(start, end time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Ticket{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Count(&count).Error
	return count, err
}

// CountTicketsCreatedToday returns tickets created today (midnight to now).
func (r *AnalyticsRepository) CountTicketsCreatedToday() (int64, error) {
	return r.CountTicketsInRange(truncateDay(time.Now()), time.Now())
}

// CountTicketsResolvedToday returns tickets moved to RESOLVED/CLOSED today.
func (r *AnalyticsRepository) CountTicketsResolvedToday() (int64, error) {
	today := truncateDay(time.Now())
	var count int64
	err := r.db.Model(&models.Ticket{}).
		Where("status IN ('RESOLVED','CLOSED') AND updated_at >= ?", today).
		Count(&count).Error
	return count, err
}

// AvgResolutionHours returns average resolution time in hours for tickets
// closed/resolved in the given window.
func (r *AnalyticsRepository) AvgResolutionHours(start, end time.Time) (float64, error) {
	var result struct{ Avg float64 }
	err := r.db.Model(&models.Ticket{}).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) / 3600), 0) as avg").
		Where("status IN ('RESOLVED','CLOSED') AND updated_at BETWEEN ? AND ?", start, end).
		Find(&result).Error
	return result.Avg, err
}

// CountTicketsByStatusSnapshot returns total counts per status at the current moment.
func (r *AnalyticsRepository) CountTicketsByStatusSnapshot() (map[string]int64, error) {
	rows, err := r.CountTicketsByStatus()
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, row := range rows {
		m[row.Status] = row.Count
	}
	return m, nil
}

// ─── Live AI counts ──────────────────────────────────────────────────────────

type AIReplySummary struct {
	Total     int64
	Approved  int64
	Rejected  int64
	Edited    int64
	Retried   int64
	AvgConf   float64
	AvgGenMs  float64
}

// SummariseAIReplies returns aggregate AI reply statistics for the window.
func (r *AnalyticsRepository) SummariseAIReplies(start, end time.Time) (AIReplySummary, error) {
	var row struct {
		Total    int64
		Approved int64
		Rejected int64
		Edited   int64
		Retried  int64
		AvgConf  float64
		AvgGenMs float64
	}
	err := r.db.Model(&models.AIReply{}).
		Select(`
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'APPROVED')    as approved,
			COUNT(*) FILTER (WHERE status = 'REJECTED')    as rejected,
			COUNT(*) FILTER (WHERE status = 'APPROVED' AND edited_reply != '') as edited,
			COUNT(*) FILTER (WHERE status = 'REGENERATED') as retried,
			COALESCE(AVG(confidence), 0)                   as avg_conf,
			COALESCE(AVG(generation_time), 0)              as avg_gen_ms
		`).
		Where("created_at BETWEEN ? AND ?", start, end).
		Find(&row).Error
	if err != nil {
		return AIReplySummary{}, err
	}
	return AIReplySummary{
		Total:    row.Total,
		Approved: row.Approved,
		Rejected: row.Rejected,
		Edited:   row.Edited,
		Retried:  row.Retried,
		AvgConf:  row.AvgConf,
		AvgGenMs: row.AvgGenMs,
	}, nil
}

// AvgAIConfidence returns the average ai_confidence across all tickets with a
// non-null confidence value created within the window.
func (r *AnalyticsRepository) AvgAIConfidence(start, end time.Time) (float64, error) {
	var result struct{ Avg float64 }
	err := r.db.Model(&models.Ticket{}).
		Select("COALESCE(AVG(ai_confidence), 0) as avg").
		Where("ai_confidence IS NOT NULL AND created_at BETWEEN ? AND ?", start, end).
		Find(&result).Error
	return result.Avg, err
}

// CountAIAnalysesCompleted returns the number of tickets with AI analysis
// completed in the given window.
func (r *AnalyticsRepository) CountAIAnalysesCompleted(start, end time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Ticket{}).
		Where("ai_processing_status = 'COMPLETED' AND processed_at BETWEEN ? AND ?", start, end).
		Count(&count).Error
	return count, err
}

// CountAIFailures returns jobs with type containing AI that have FAILED status.
func (r *AnalyticsRepository) CountAIFailures(start, end time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.BackgroundJob{}).
		Where("job_type IN ('AI_ANALYSIS','GENERATE_REPLY','RETRY_AI','RETRY_REPLY') AND status = 'FAILED' AND created_at BETWEEN ? AND ?", start, end).
		Count(&count).Error
	return count, err
}

// TopAICategories returns the most-assigned AI categories in the window.
func (r *AnalyticsRepository) TopAICategories(start, end time.Time, limit int) ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("ai_category as label, COUNT(*) as count").
		Where("ai_category != '' AND created_at BETWEEN ? AND ?", start, end).
		Group("ai_category").
		Order("count DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// TopAISentiments returns the most common AI-assigned sentiments.
func (r *AnalyticsRepository) TopAISentiments(start, end time.Time) ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.Ticket{}).
		Select("ai_sentiment as label, COUNT(*) as count").
		Where("ai_sentiment != '' AND created_at BETWEEN ? AND ?", start, end).
		Group("ai_sentiment").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// ─── Queue counts ────────────────────────────────────────────────────────────

type QueueSnapshot struct {
	Queued     int64
	Processing int64
	Completed  int64
	Failed     int64
	Dead       int64
	Retrying   int64
	AvgQueueSec float64
}

// SnapshotQueue returns a live queue health snapshot.
func (r *AnalyticsRepository) SnapshotQueue() (QueueSnapshot, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.Model(&models.BackgroundJob{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&rows).Error
	if err != nil {
		return QueueSnapshot{}, err
	}
	snap := QueueSnapshot{}
	for _, row := range rows {
		switch row.Status {
		case "QUEUED":
			snap.Queued = row.Count
		case "PROCESSING":
			snap.Processing = row.Count
		case "COMPLETED":
			snap.Completed = row.Count
		case "FAILED":
			snap.Failed = row.Count
		case "DEAD":
			snap.Dead = row.Count
		case "RETRYING":
			snap.Retrying = row.Count
		}
	}

	// Average queue wait for jobs that have started in the last 24 h
	var avgRow struct{ Avg float64 }
	_ = r.db.Model(&models.BackgroundJob{}).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(started_at, NOW()) - created_at))), 0) as avg").
		Where("created_at > NOW() - INTERVAL '24 hours' AND status IN ('QUEUED','PROCESSING','COMPLETED')").
		Find(&avgRow).Error
	snap.AvgQueueSec = avgRow.Avg

	return snap, nil
}

// CountJobsByType returns job counts grouped by job_type.
func (r *AnalyticsRepository) CountJobsByType() ([]LabelCount, error) {
	var rows []LabelCount
	err := r.db.Model(&models.BackgroundJob{}).
		Select("job_type as label, COUNT(*) as count").
		Group("job_type").
		Order("count DESC").
		Find(&rows).Error
	return rows, err
}

// ─── Email counts ─────────────────────────────────────────────────────────────

type EmailSnapshot struct {
	Received     int64
	Sent         int64
	Failed       int64
	Queued       int64
	AvgDelivSec  float64
}

// SnapshotEmail returns email statistics for the given window.
func (r *AnalyticsRepository) SnapshotEmail(start, end time.Time) (EmailSnapshot, error) {
	var rows []struct {
		Direction string
		Status    string
		Count     int64
	}
	err := r.db.Model(&models.EmailMessage{}).
		Select("direction, status, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("direction, status").
		Find(&rows).Error
	if err != nil {
		return EmailSnapshot{}, err
	}

	snap := EmailSnapshot{}
	for _, row := range rows {
		switch {
		case row.Direction == "INBOUND" && row.Status == "RECEIVED":
			snap.Received += row.Count
		case row.Direction == "OUTBOUND" && row.Status == "SENT":
			snap.Sent += row.Count
		case row.Direction == "OUTBOUND" && row.Status == "FAILED":
			snap.Failed += row.Count
		case row.Direction == "OUTBOUND" && row.Status == "QUEUED":
			snap.Queued += row.Count
		}
	}

	var avgRow struct{ Avg float64 }
	_ = r.db.Model(&models.EmailMessage{}).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (sent_at - created_at))), 0) as avg").
		Where("direction = 'OUTBOUND' AND status = 'SENT' AND sent_at BETWEEN ? AND ?", start, end).
		Find(&avgRow).Error
	snap.AvgDelivSec = avgRow.Avg

	return snap, nil
}

// CountEmailsProcessedToday returns inbound + outbound emails received/sent today.
func (r *AnalyticsRepository) CountEmailsProcessedToday() (int64, error) {
	today := truncateDay(time.Now())
	var count int64
	err := r.db.Model(&models.EmailMessage{}).
		Where("created_at >= ?", today).
		Count(&count).Error
	return count, err
}

// ─── Aggregated metrics table CRUD ──────────────────────────────────────────

// UpsertDailyTicketMetrics inserts or updates a DailyTicketMetrics row.
func (r *AnalyticsRepository) UpsertDailyTicketMetrics(m *models.DailyTicketMetrics) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tickets_created", "tickets_closed", "tickets_resolved",
			"tickets_reopened", "average_resolution_time",
			"average_first_response_time", "average_ai_processing_time",
		}),
	}).Create(m).Error
}

// GetDailyTicketMetrics returns aggregated daily metrics for a date range.
func (r *AnalyticsRepository) GetDailyTicketMetrics(start, end time.Time) ([]models.DailyTicketMetrics, error) {
	var rows []models.DailyTicketMetrics
	err := r.db.Where("date BETWEEN ? AND ?", start, end).
		Order("date ASC").
		Find(&rows).Error
	return rows, err
}

// UpsertAIMetrics inserts or updates an AIMetrics row.
func (r *AnalyticsRepository) UpsertAIMetrics(m *models.AIMetrics) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"analysis_generated", "replies_generated", "average_confidence",
			"average_generation_time", "approval_rate", "edit_rate",
			"rejection_rate", "retry_rate",
		}),
	}).Create(m).Error
}

// GetAIMetrics returns aggregated AI metrics for a date range.
func (r *AnalyticsRepository) GetAIMetrics(start, end time.Time) ([]models.AIMetrics, error) {
	var rows []models.AIMetrics
	err := r.db.Where("date BETWEEN ? AND ?", start, end).
		Order("date ASC").
		Find(&rows).Error
	return rows, err
}

// UpsertAgentMetrics inserts or updates an AgentMetrics row.
func (r *AnalyticsRepository) UpsertAgentMetrics(m *models.AgentMetrics) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tickets_assigned", "tickets_resolved",
			"average_resolution_time", "average_reply_time",
			"average_customer_rating", "last_calculated",
		}),
	}).Create(m).Error
}

// GetAllAgentMetrics returns all agent metrics rows with user details preloaded.
func (r *AnalyticsRepository) GetAllAgentMetrics() ([]models.AgentMetrics, error) {
	var rows []models.AgentMetrics
	err := r.db.Preload("User").
		Order("tickets_resolved DESC").
		Find(&rows).Error
	return rows, err
}

// GetAgentMetricsByUserID returns metrics for a single agent.
func (r *AnalyticsRepository) GetAgentMetricsByUserID(userID uint) (*models.AgentMetrics, error) {
	var m models.AgentMetrics
	err := r.db.Preload("User").
		Where("user_id = ?", userID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CountActiveTicketsForAgent returns currently open/in-progress tickets for an agent.
func (r *AnalyticsRepository) CountActiveTicketsForAgent(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Ticket{}).
		Where("assigned_to = ? AND status IN ('OPEN','IN_PROGRESS')", userID).
		Count(&count).Error
	return count, err
}

// AllSupportAgents returns all users with SupportAgent or Admin role.
func (r *AnalyticsRepository) AllSupportAgents() ([]models.User, error) {
	var users []models.User
	err := r.db.Where("is_active = true").
		Order("id ASC").
		Find(&users).Error
	return users, err
}

// ─── Aggregation raw queries ─────────────────────────────────────────────────

// These are used exclusively by the Aggregator to compute daily metrics.

type AgentRawMetrics struct {
	UserID         uint
	Assigned       int64
	Resolved       int64
	AvgResolutionH float64
	AvgReplyH      float64
}

// ComputeAgentMetrics runs the raw aggregation queries for a single user.
func (r *AnalyticsRepository) ComputeAgentMetrics(userID uint) (AgentRawMetrics, error) {
	var row struct {
		Assigned       int64
		Resolved       int64
		AvgResolutionH float64
	}
	err := r.db.Model(&models.Ticket{}).
		Select(`
			COUNT(*) as assigned,
			COUNT(*) FILTER (WHERE status IN ('RESOLVED','CLOSED')) as resolved,
			COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) / 3600)
			  FILTER (WHERE status IN ('RESOLVED','CLOSED')), 0) as avg_resolution_h
		`).
		Where("assigned_to = ?", userID).
		Find(&row).Error
	if err != nil {
		return AgentRawMetrics{}, err
	}

	// Proxy for reply time: avg time from ticket creation to first public comment
	var replyRow struct{ AvgH float64 }
	_ = r.db.Model(&models.TicketComment{}).
		Select(`
			COALESCE(AVG(EXTRACT(EPOCH FROM
			  (ticket_comments.created_at - t.created_at)) / 3600), 0) as avg_h
		`).
		Joins("JOIN tickets t ON t.id = ticket_comments.ticket_id").
		Where("ticket_comments.user_id = ? AND t.assigned_to = ? AND ticket_comments.comment_type = 'PUBLIC'",
			userID, userID).
		Find(&replyRow).Error

	return AgentRawMetrics{
		UserID:         userID,
		Assigned:       row.Assigned,
		Resolved:       row.Resolved,
		AvgResolutionH: row.AvgResolutionH,
		AvgReplyH:      replyRow.AvgH,
	}, nil
}

// ComputeDailyTickets computes raw counts for a specific calendar day.
func (r *AnalyticsRepository) ComputeDailyTickets(date time.Time) (models.DailyTicketMetrics, error) {
	dayStart := truncateDay(date)
	dayEnd := dayStart.Add(24 * time.Hour)

	var row struct {
		Created  int
		Closed   int
		Resolved int
		Reopened int
		AvgRes   float64
		AvgAI    float64
	}

	err := r.db.Model(&models.Ticket{}).
		Select(`
			COUNT(*) FILTER (WHERE created_at BETWEEN ? AND ?)  as created,
			COUNT(*) FILTER (WHERE status = 'CLOSED'   AND updated_at BETWEEN ? AND ?) as closed,
			COUNT(*) FILTER (WHERE status = 'RESOLVED' AND updated_at BETWEEN ? AND ?) as resolved,
			COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) / 3600)
			  FILTER (WHERE status IN ('RESOLVED','CLOSED') AND updated_at BETWEEN ? AND ?), 0) as avg_res,
			COALESCE(AVG(EXTRACT(EPOCH FROM (processed_at - created_at)))
			  FILTER (WHERE ai_processing_status = 'COMPLETED' AND processed_at BETWEEN ? AND ?), 0) as avg_ai
		`,
			dayStart, dayEnd,
			dayStart, dayEnd,
			dayStart, dayEnd,
			dayStart, dayEnd,
			dayStart, dayEnd,
		).Find(&row).Error
	if err != nil {
		return models.DailyTicketMetrics{}, err
	}

	// First response time: avg hours from ticket creation to first public comment
	var frRow struct{ AvgH float64 }
	_ = r.db.Model(&models.TicketComment{}).
		Select(`
			COALESCE(AVG(EXTRACT(EPOCH FROM
			  (tc.created_at - t.created_at)) / 3600), 0) as avg_h
		`).
		Table("ticket_comments tc").
		Joins("JOIN tickets t ON t.id = tc.ticket_id").
		Where(`tc.created_at BETWEEN ? AND ?
			AND tc.id = (
				SELECT MIN(tc2.id) FROM ticket_comments tc2
				WHERE tc2.ticket_id = tc.ticket_id AND tc2.comment_type = 'PUBLIC'
			)`, dayStart, dayEnd).
		Find(&frRow).Error

	return models.DailyTicketMetrics{
		Date:                     dayStart,
		TicketsCreated:           row.Created,
		TicketsClosed:            row.Closed,
		TicketsResolved:          row.Resolved,
		AverageResolutionTime:    row.AvgRes,
		AverageFirstResponseTime: frRow.AvgH,
		AverageAIProcessingTime:  row.AvgAI,
	}, nil
}

// ComputeAIMetrics computes AI performance metrics for a specific calendar day.
func (r *AnalyticsRepository) ComputeAIMetrics(date time.Time) (models.AIMetrics, error) {
	dayStart := truncateDay(date)
	dayEnd := dayStart.Add(24 * time.Hour)

	var analysisCount int64
	_ = r.db.Model(&models.Ticket{}).
		Where("ai_processing_status = 'COMPLETED' AND processed_at BETWEEN ? AND ?", dayStart, dayEnd).
		Count(&analysisCount).Error

	s, err := r.SummariseAIReplies(dayStart, dayEnd)
	if err != nil {
		return models.AIMetrics{}, err
	}

	approvalRate, editRate, rejectionRate, retryRate := calcRates(s)

	return models.AIMetrics{
		Date:                  dayStart,
		AnalysisGenerated:     int(analysisCount),
		RepliesGenerated:      int(s.Total),
		AverageConfidence:     s.AvgConf,
		AverageGenerationTime: s.AvgGenMs,
		ApprovalRate:          approvalRate,
		EditRate:              editRate,
		RejectionRate:         rejectionRate,
		RetryRate:             retryRate,
	}, nil
}

// ─── Report CRUD ─────────────────────────────────────────────────────────────

func (r *AnalyticsRepository) CreateReport(report *models.Report) error {
	return r.db.Create(report).Error
}

func (r *AnalyticsRepository) FindReport(id uint) (*models.Report, error) {
	var report models.Report
	err := r.db.Preload("Generator").First(&report, id).Error
	return &report, err
}

func (r *AnalyticsRepository) UpdateReport(report *models.Report) error {
	return r.db.Save(report).Error
}

func (r *AnalyticsRepository) ListReports(generatedBy *uint) ([]models.Report, error) {
	var reports []models.Report
	q := r.db.Order("created_at DESC")
	if generatedBy != nil {
		q = q.Where("generated_by = ?", *generatedBy)
	}
	err := q.Find(&reports).Error
	return reports, err
}

// DeleteOldReports removes report records (and their on-disk files) older than
// retentionDays. The caller is responsible for deleting the files themselves.
func (r *AnalyticsRepository) ListOldReports(retentionDays int) ([]models.Report, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	var reports []models.Report
	err := r.db.Where("created_at < ?", cutoff).Find(&reports).Error
	return reports, err
}

func (r *AnalyticsRepository) DeleteReport(id uint) error {
	return r.db.Delete(&models.Report{}, id).Error
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// truncateDay returns the UTC midnight of the given time.
func truncateDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func calcRates(s AIReplySummary) (approval, edit, rejection, retry float64) {
	if s.Total == 0 {
		return 0, 0, 0, 0
	}
	total := float64(s.Total)
	return round2(float64(s.Approved) / total * 100),
		round2(float64(s.Edited) / total * 100),
		round2(float64(s.Rejected) / total * 100),
		round2(float64(s.Retried) / total * 100)
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
