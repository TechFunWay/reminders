package reminder

import "time"

const (
	ChannelInApp    = "inapp"
	ChannelEmail    = "email"
	ChannelSMS      = "sms"
	ChannelFeishu   = "feishu"
	ChannelQQ       = "qq"
	ChannelDingTalk = "dingtalk"
)

var supportedChannels = map[string]bool{
	ChannelInApp: true, ChannelEmail: true, ChannelSMS: true,
	ChannelFeishu: true, ChannelQQ: true, ChannelDingTalk: true,
}

type List struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	UserID    uint       `gorm:"not null;index:idx_reminder_lists_user_position" json:"-"`
	Name      string     `gorm:"size:40;not null" json:"name"`
	Color     string     `gorm:"size:16;not null;default:blue" json:"color"`
	Icon      string     `gorm:"size:32;not null;default:list" json:"icon"`
	Position  int        `gorm:"not null;default:0;index:idx_reminder_lists_user_position" json:"position"`
	IsDefault bool       `gorm:"not null;default:false" json:"is_default"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

type Reminder struct {
	ID           uint              `gorm:"primarykey" json:"id"`
	UserID       uint              `gorm:"not null;index:idx_reminders_user_due;index:idx_reminders_user_completed" json:"-"`
	ListID       uint              `gorm:"not null;index" json:"list_id"`
	Title        string            `gorm:"size:200;not null" json:"title"`
	Notes        string            `gorm:"type:text" json:"notes"`
	Priority     int               `gorm:"not null;default:0" json:"priority"`
	DueAt        *time.Time        `gorm:"index:idx_reminders_user_due" json:"due_at"`
	AllDay       bool              `gorm:"not null;default:false" json:"all_day"`
	RepeatRule   string            `gorm:"size:16;not null;default:none" json:"repeat_rule"`
	CompletedAt  *time.Time        `gorm:"index:idx_reminders_user_completed" json:"completed_at"`
	SnoozedUntil *time.Time        `json:"snoozed_until"`
	Position     int               `gorm:"not null;default:0" json:"position"`
	Version      uint              `gorm:"not null;default:1" json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	DeletedAt    *time.Time        `gorm:"index" json:"-"`
	Channels     []ReminderChannel `gorm:"foreignKey:ReminderID" json:"-"`
}

type ReminderChannel struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ReminderID uint      `gorm:"not null;uniqueIndex:idx_reminder_channel" json:"reminder_id"`
	UserID     uint      `gorm:"not null;index" json:"-"`
	Channel    string    `gorm:"size:16;not null;uniqueIndex:idx_reminder_channel" json:"channel"`
	Enabled    bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Completion struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	ReminderID    uint       `gorm:"not null;index" json:"reminder_id"`
	UserID        uint       `gorm:"not null;index" json:"-"`
	ScheduledFor  *time.Time `json:"scheduled_for"`
	CompletedAt   time.Time  `gorm:"index" json:"completed_at"`
	TitleSnapshot string     `gorm:"size:200;not null" json:"title"`
}

type ChannelBinding struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	UserID        uint       `gorm:"not null;uniqueIndex:idx_user_channel_binding" json:"-"`
	Channel       string     `gorm:"size:16;not null;uniqueIndex:idx_user_channel_binding" json:"channel"`
	Target        string     `gorm:"type:text;not null" json:"-"`
	TargetMasked  string     `gorm:"size:120;not null" json:"target_masked"`
	Status        string     `gorm:"size:16;not null;default:active;index" json:"status"`
	VerifiedAt    *time.Time `json:"verified_at"`
	LastTestedAt  *time.Time `json:"last_tested_at"`
	LastErrorCode string     `gorm:"size:64" json:"last_error_code,omitempty"`
	LastErrorAt   *time.Time `json:"last_error_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ProviderConfig stores administrator-managed provider credentials. The secret
// is encrypted with the installation key and never returned by APIs.
type ProviderConfig struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Provider  string    `gorm:"size:32;not null;uniqueIndex" json:"provider"`
	Secret    string    `gorm:"type:text;not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QQBindCode links a signed-in Reminder user to a QQ C2C conversation without
// ever asking that person to find a platform OpenID.
type QQBindCode struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"-"`
	Code      string    `gorm:"size:12;not null;uniqueIndex" json:"code"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type DeliveryJob struct {
	ID                uint       `gorm:"primarykey" json:"id"`
	UserID            uint       `gorm:"not null;index" json:"-"`
	ReminderID        uint       `gorm:"not null;uniqueIndex:idx_delivery_occurrence" json:"reminder_id"`
	Channel           string     `gorm:"size:16;not null;uniqueIndex:idx_delivery_occurrence;index" json:"channel"`
	ScheduledFor      time.Time  `gorm:"not null;uniqueIndex:idx_delivery_occurrence;index" json:"scheduled_for"`
	RunAt             time.Time  `gorm:"not null;index:idx_delivery_due" json:"run_at"`
	Status            string     `gorm:"size:16;not null;default:pending;index:idx_delivery_due" json:"status"`
	AttemptCount      int        `gorm:"not null;default:0" json:"attempt_count"`
	NextAttemptAt     *time.Time `gorm:"index" json:"next_attempt_at,omitempty"`
	LockedAt          *time.Time `json:"locked_at,omitempty"`
	WorkerID          string     `gorm:"size:64" json:"worker_id,omitempty"`
	IdempotencyKey    string     `gorm:"size:64;not null;uniqueIndex" json:"idempotency_key"`
	LastErrorCode     string     `gorm:"size:64" json:"last_error_code,omitempty"`
	LastErrorMessage  string     `gorm:"size:300" json:"last_error_message,omitempty"`
	ExternalMessageID string     `gorm:"size:200" json:"external_message_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type DeliveryAttempt struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	JobID           uint      `gorm:"not null;index" json:"job_id"`
	AttemptNo       int       `gorm:"not null" json:"attempt_no"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	Result          string    `gorm:"size:16;not null" json:"result"`
	ProviderCode    string    `gorm:"size:100" json:"provider_code,omitempty"`
	LatencyMS       int64     `json:"latency_ms"`
	ResponseSummary string    `gorm:"size:300" json:"response_summary,omitempty"`
}

type Notification struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	UserID        uint       `gorm:"not null;index:idx_notifications_user_read" json:"-"`
	ReminderID    *uint      `gorm:"index" json:"reminder_id,omitempty"`
	DeliveryJobID *uint      `gorm:"uniqueIndex" json:"delivery_job_id,omitempty"`
	Type          string     `gorm:"size:32;not null" json:"type"`
	Title         string     `gorm:"size:200;not null" json:"title"`
	Body          string     `gorm:"size:500" json:"body"`
	ReadAt        *time.Time `gorm:"index:idx_notifications_user_read" json:"read_at,omitempty"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
}

type ReminderDTO struct {
	Reminder
	Channels []string `json:"channels"`
	ListName string   `json:"list_name"`
}

type ListDTO struct {
	List
	OpenCount int64 `json:"open_count"`
}

type ChannelStatus struct {
	Channel      string `json:"channel"`
	Label        string `json:"label"`
	Configured   bool   `json:"configured"`
	Bound        bool   `json:"bound"`
	Status       string `json:"status"`
	TargetMasked string `json:"target_masked,omitempty"`
	BotLink      string `json:"bot_link,omitempty"`
	Description  string `json:"description"`
}
