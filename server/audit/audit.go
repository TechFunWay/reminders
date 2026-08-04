// Package audit provides a database-backed operation log for security- and
// admin-relevant actions, plus admin endpoints to browse and export it.
//
// Record an action from any handler:
//
//	audit.Log(db, c, "user_delete", "user", targetID, "deleted user bob")
//
// The current user and client IP are read from the gin context (populated by
// the auth middleware), and the entry is mirrored to the audit log file.
package audit

import (
	"encoding/csv"
	"strconv"
	"strings"
	"sync"
	"time"

	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/response"
	"smallgo/server/scheduler"
	"smallgo/server/sysconfig"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Log writes an audit entry. It never blocks the request on failure — a write
// error is logged but not returned, since auditing must not break the action.
func Log(db *gorm.DB, c *gin.Context, action, targetType string, targetID uint, detail string) {
	// Mark the request so MutationLogger does not create a second generic row
	// for handlers that already record a more descriptive operation.
	c.Set("audit_logged", true)
	entry := database.AuditLog{
		UserID:     c.GetUint("userID"),
		Username:   c.GetString("username"),
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         c.ClientIP(),
		CreatedAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		logger.Error("audit: failed to write entry for action %q: %v", action, err)
		return
	}
	logger.Audit("user=%d(%s) action=%s target=%s/%d ip=%s detail=%s",
		entry.UserID, entry.Username, action, targetType, targetID, entry.IP, detail)
}

// MutationLogger records every successful authenticated write request. It
// complements explicit audit rows in user administration handlers and ensures
// reminder, channel, configuration, profile and security changes are visible.
func MutationLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.GetBool("audit_logged") || c.GetUint("userID") == 0 || c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
			return
		}
		switch c.Request.Method {
		case "POST", "PUT", "PATCH", "DELETE":
		default:
			return
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		action := strings.ToLower(c.Request.Method) + "_" + strings.Trim(strings.ReplaceAll(path, "/", "_"), "_")
		Log(db, c, action, "api", 0, c.Request.Method+" "+path)
	}
}

var cleanupOnce sync.Once

// RegisterCleanup removes old database audit rows once a day. The same
// log_retention_days setting controls both runtime logs and operation logs;
// 0 explicitly keeps history forever.
func RegisterCleanup(db *gorm.DB) {
	cleanupOnce.Do(func() {
		scheduler.Register(scheduler.Job{
			Name: "audit-log-cleanup", Daily: "03:15", RunAtStart: true,
			Run: func() { cleanup(db) },
		})
	})
}

func cleanup(db *gorm.DB) {
	raw, err := sysconfig.GetConfig(db, "log_retention_days", 0)
	if err != nil {
		logger.Error("audit: read retention setting: %v", err)
		return
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	if result := db.Where("created_at < ?", cutoff).Delete(&database.AuditLog{}); result.Error != nil {
		logger.Error("audit: cleanup failed: %v", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("audit: removed %d entries older than %d days", result.RowsAffected, days)
	}
}

type entryView struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   uint   `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	CreatedAt  string `json:"created_at"`
}

// buildQuery applies the shared filters (user_id, action, username) used by both
// the list and export endpoints.
func buildQuery(db *gorm.DB, c *gin.Context) *gorm.DB {
	q := db.Model(&database.AuditLog{})
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			q = q.Where("user_id = ?", id)
		}
	}
	if v := c.Query("action"); v != "" {
		q = q.Where("action = ?", v)
	}
	if v := c.Query("username"); v != "" {
		q = q.Where("username LIKE ?", "%"+v+"%")
	}
	return q
}

func toViews(logs []database.AuditLog) []entryView {
	views := make([]entryView, 0, len(logs))
	for _, l := range logs {
		views = append(views, entryView{
			ID: l.ID, UserID: l.UserID, Username: l.Username, Action: l.Action,
			TargetType: l.TargetType, TargetID: l.TargetID, Detail: l.Detail, IP: l.IP,
			CreatedAt: l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return views
}

func handleList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := utils.NormalizePage(
			utils.Atoi(c.Query("page"), 1),
			utils.Atoi(c.Query("pageSize"), utils.DefaultPageSize),
		)

		q := buildQuery(db, c)
		var total int64
		if err := q.Count(&total).Error; err != nil {
			response.ErrorInternal(c, "获取审计日志失败")
			return
		}

		var logs []database.AuditLog
		if err := q.Order("id DESC").Offset(utils.Offset(page, pageSize)).Limit(pageSize).Find(&logs).Error; err != nil {
			response.ErrorInternal(c, "获取审计日志失败")
			return
		}

		response.SuccessPage(c, toViews(logs), total, page, pageSize)
	}
}

func handleExport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var logs []database.AuditLog
		if err := buildQuery(db, c).Order("id DESC").Limit(10000).Find(&logs).Error; err != nil {
			response.ErrorInternal(c, "导出审计日志失败")
			return
		}

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="audit-log.csv"`)
		c.Writer.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel

		w := csv.NewWriter(c.Writer)
		defer w.Flush()
		w.Write([]string{"id", "user_id", "username", "action", "target_type", "target_id", "detail", "ip", "created_at"})
		for _, v := range toViews(logs) {
			w.Write([]string{
				strconv.FormatUint(uint64(v.ID), 10),
				strconv.FormatUint(uint64(v.UserID), 10),
				v.Username, v.Action, v.TargetType,
				strconv.FormatUint(uint64(v.TargetID), 10),
				v.Detail, v.IP, v.CreatedAt,
			})
		}
	}
}

// RegisterRoutes mounts the admin-only audit endpoints.
func RegisterRoutes(admin *gin.RouterGroup, db *gorm.DB) {
	admin.GET("/audit-logs", handleList(db))
	admin.GET("/audit-logs/export", handleExport(db))
}
