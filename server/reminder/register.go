package reminder

import (
	"time"

	"smallgo/server/apps"
	"smallgo/server/database"
	"smallgo/server/scheduler"
	"smallgo/server/sysconfig"

	"gorm.io/gorm"
)

var appDB *gorm.DB

func init() {
	database.Upgrades = append(database.Upgrades, database.Upgrade{
		Version: "0.1.2", Name: "rename_default_site_title",
		Upgrade: func(db *gorm.DB) error {
			// Preserve administrator customisations; only migrate the old default.
			return db.Model(&database.SystemConfig{}).
				Where("user_id = ? AND key = ? AND value = ?", 0, "site_title", "Reminder").
				Update("value", "提醒事项").Error
		},
	})

	database.RegisterModels(
		&List{}, &Reminder{}, &ReminderChannel{}, &Completion{},
		&ChannelBinding{}, &ProviderConfig{}, &QQBindCode{}, &DeliveryJob{}, &DeliveryAttempt{}, &Notification{},
	)

	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key: "reminder_default_channels", Scope: sysconfig.ScopeUser, Type: sysconfig.TypeString,
		Default: "inapp", Group: "notification", Label: "默认提醒渠道",
		Description: "新提醒默认使用的渠道，多个渠道以英文逗号分隔",
	})
	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key: "reminder_all_day_time", Scope: sysconfig.ScopeUser, Type: sysconfig.TypeString,
		Default: "09:00", Group: "notification", Label: "全天提醒时间",
		Description: "只有日期的提醒在当天何时通知",
	})

	apps.Register(apps.App{
		Name:        "reminder",
		DisplayName: "提醒事项",
		Icon:        "check-circle",
		RoutePrefix: "/reminders",
		NavPosition: 10,
		SetupAuth:   setupAuthRoutes,
		SetupAdmin:  setupAdminRoutes,
		Migrate: func(db *gorm.DB) error {
			appDB = db
			return nil
		},
	})

	scheduler.Register(scheduler.Job{
		Name:       "reminder-delivery",
		Interval:   time.Second,
		RunAtStart: true,
		Run: func() {
			if appDB != nil {
				dispatchDueJobs(appDB)
			}
		},
	})
	scheduler.Register(scheduler.Job{
		Name: "reminder-qq-gateway", Interval: 5 * time.Second,
		Run: func() {
			if appDB != nil {
				ensureQQGateway(appDB)
			}
		},
	})
}
