package models

import "time"

// DailyTicketMetrics stores pre-aggregated per-day ticket statistics.
// One row per calendar date; upserted by the nightly aggregation job.
type DailyTicketMetrics struct {
	ID                       uint      `gorm:"primarykey;autoIncrement"              json:"id"`
	Date                     time.Time `gorm:"type:date;uniqueIndex;not null"        json:"date"`
	TicketsCreated           int       `gorm:"not null;default:0"                    json:"tickets_created"`
	TicketsClosed            int       `gorm:"not null;default:0"                    json:"tickets_closed"`
	TicketsResolved          int       `gorm:"not null;default:0"                    json:"tickets_resolved"`
	TicketsReopened          int       `gorm:"not null;default:0"                    json:"tickets_reopened"`
	AverageResolutionTime    float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_resolution_time"`    // hours
	AverageFirstResponseTime float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_first_response_time"` // hours
	AverageAIProcessingTime  float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_ai_processing_time"`  // seconds
	CreatedAt                time.Time `json:"created_at"`
}

// AgentMetrics stores per-agent performance snapshot, recalculated each aggregation cycle.
// One row per agent (UserID unique); updated in-place.
type AgentMetrics struct {
	ID                    uint      `gorm:"primarykey;autoIncrement"              json:"id"`
	UserID                uint      `gorm:"not null;uniqueIndex"                  json:"user_id"`
	TicketsAssigned       int       `gorm:"not null;default:0"                    json:"tickets_assigned"`
	TicketsResolved       int       `gorm:"not null;default:0"                    json:"tickets_resolved"`
	AverageResolutionTime float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_resolution_time"` // hours
	AverageReplyTime      float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_reply_time"`      // hours
	AverageCustomerRating float64   `gorm:"type:numeric(3,2);not null;default:0"  json:"average_customer_rating"`
	LastCalculated        time.Time `gorm:"not null"                              json:"last_calculated"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// AIMetrics stores pre-aggregated per-day AI performance statistics.
// One row per calendar date; upserted by the nightly aggregation job.
type AIMetrics struct {
	ID                    uint      `gorm:"primarykey;autoIncrement"              json:"id"`
	Date                  time.Time `gorm:"type:date;uniqueIndex;not null"        json:"date"`
	AnalysisGenerated     int       `gorm:"not null;default:0"                    json:"analysis_generated"`
	RepliesGenerated      int       `gorm:"not null;default:0"                    json:"replies_generated"`
	AverageConfidence     float64   `gorm:"type:numeric(5,2);not null;default:0"  json:"average_confidence"`
	AverageGenerationTime float64   `gorm:"type:numeric(10,2);not null;default:0" json:"average_generation_time"` // ms
	ApprovalRate          float64   `gorm:"type:numeric(5,2);not null;default:0"  json:"approval_rate"`
	EditRate              float64   `gorm:"type:numeric(5,2);not null;default:0"  json:"edit_rate"`
	RejectionRate         float64   `gorm:"type:numeric(5,2);not null;default:0"  json:"rejection_rate"`
	RetryRate             float64   `gorm:"type:numeric(5,2);not null;default:0"  json:"retry_rate"`
	CreatedAt             time.Time `json:"created_at"`
}

// ReportFormat is the output file format for an exported report.
type ReportFormat string

// ReportStatus is the generation lifecycle state of a report.
type ReportStatus string

const (
	ReportFormatCSV   ReportFormat = "CSV"
	ReportFormatExcel ReportFormat = "EXCEL"
	ReportFormatHTML  ReportFormat = "HTML"
)

const (
	ReportStatusPending   ReportStatus = "PENDING"
	ReportStatusCompleted ReportStatus = "COMPLETED"
	ReportStatusFailed    ReportStatus = "FAILED"
)

// Report tracks every generated analytics report so users can re-download them.
type Report struct {
	ID           uint         `gorm:"primarykey;autoIncrement"              json:"id"`
	Name         string       `gorm:"type:varchar(200);not null"            json:"name"`
	ReportType   string       `gorm:"type:varchar(50);not null"             json:"report_type"`
	Format       ReportFormat `gorm:"type:varchar(20);not null"             json:"format"`
	Status       ReportStatus `gorm:"type:varchar(20);not null;default:'PENDING'" json:"status"`
	FilePath     string       `gorm:"type:varchar(500)"                     json:"-"`
	FileSize     int64        `gorm:"not null;default:0"                    json:"file_size"`
	Filters      string       `gorm:"type:text"                             json:"filters"`
	GeneratedBy  uint         `gorm:"not null;index"                        json:"generated_by"`
	ErrorMessage string       `gorm:"type:text"                             json:"error_message,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`

	Generator *User `gorm:"foreignKey:GeneratedBy" json:"generator,omitempty"`
}
