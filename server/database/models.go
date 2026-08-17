package database

import "time"

type User struct {
	ID           uint   `gorm:"primarykey"`
	Username     string `gorm:"uniqueIndex;not null"`
	Password     string `gorm:"not null"`
	Role         string `gorm:"default:user"`
	Status       int    `gorm:"default:1"`
	APIKey       string
	AuthVersion  uint  `gorm:"not null;default:1"`
	FnOSUserID   *uint `gorm:"uniqueIndex"`
	FnOSUsername string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SystemConfig struct {
	ID        uint   `gorm:"primarykey"`
	UserID    uint   `gorm:"default:0;uniqueIndex:idx_user_key"`
	Key       string `gorm:"not null;uniqueIndex:idx_user_key"`
	Value     string
	Public    bool `gorm:"default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UpgradeRecord struct {
	ID         uint   `gorm:"primarykey"`
	Version    string `gorm:"not null;index"`
	Name       string
	UpgradedAt time.Time
}

// AuditLog records a security- or admin-relevant action for later review.
type AuditLog struct {
	ID         uint   `gorm:"primarykey"`
	UserID     uint   `gorm:"index"`
	Username   string `gorm:"index"`
	Action     string `gorm:"index;not null"`
	TargetType string `gorm:"index"`
	TargetID   uint
	Detail     string
	IP         string
	CreatedAt  time.Time `gorm:"index"`
}

type SecurityQuestion struct {
	ID        uint `gorm:"primarykey"`
	UserID    uint `gorm:"not null;uniqueIndex"`
	Question1 string
	Answer1   string
	Question2 string
	Answer2   string
	Question3 string
	Answer3   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
