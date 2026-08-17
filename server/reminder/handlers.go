package reminder

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupAuthRoutes(api *gin.RouterGroup, db *gorm.DB) {
	group := api.Group("/reminder")

	group.GET("/summary", handleSummary(db))
	group.GET("/events", handleRealtimeEvents())
	group.GET("/lists", handleListLists(db))
	group.POST("/lists", handleCreateList(db))
	group.PATCH("/lists/:id", handleUpdateList(db))
	group.DELETE("/lists/:id", handleDeleteList(db))

	group.GET("/items", handleListReminders(db))
	group.POST("/items", handleCreateReminder(db))
	group.GET("/items/:id", handleGetReminder(db))
	group.PUT("/items/:id", handleUpdateReminder(db))
	group.DELETE("/items/:id", handleDeleteReminder(db))
	group.POST("/items/:id/complete", handleCompleteReminder(db))
	group.POST("/items/:id/restore", handleRestoreReminder(db))
	group.POST("/items/:id/snooze", handleSnoozeReminder(db))

	group.GET("/notifications", handleNotifications(db))
	group.GET("/notifications/unread-count", handleUnreadCount(db))
	group.POST("/notifications/:id/read", handleReadNotification(db))
	group.POST("/notifications/read-all", handleReadAllNotifications(db))

	group.GET("/channels", handleChannelStatuses(db))
	group.PUT("/channels/:channel", handleBindChannel(db))
	group.PATCH("/channels/:channel", handleToggleChannel(db))
	group.DELETE("/channels/:channel", handleUnbindChannel(db))
	group.POST("/channels/:channel/test", handleTestChannel(db))
	group.POST("/channels/qq/bind-code", handleCreateQQBindCode(db))
}

func setupAdminRoutes(api *gin.RouterGroup, db *gorm.DB) {
	api.GET("/reminder/admin/providers", handleProviderStatuses(db))
	api.GET("/reminder/admin/notification-brand", handleNotificationBrand(db))
	api.PUT("/reminder/admin/notification-brand", handleNotificationBrand(db))
	api.PUT("/reminder/admin/providers/feishu", handleSaveFeishuProvider(db))
	api.PUT("/reminder/admin/providers/:provider", handleSaveProvider(db))
	api.GET("/reminder/admin/deliveries", func(c *gin.Context) {
		var jobs []DeliveryJob
		if err := db.Order("id DESC").Limit(200).Find(&jobs).Error; err != nil {
			response.ErrorInternal(c, "读取投递记录失败")
			return
		}
		response.Success(c, jobs)
	})
}

func currentUserID(c *gin.Context) uint {
	v, _ := c.Get("userID")
	switch id := v.(type) {
	case uint:
		return id
	case int:
		return uint(id)
	case float64:
		return uint(id)
	default:
		return 0
	}
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.ErrorBadRequest(c, "无效的资源 ID")
		return 0, false
	}
	return uint(n), true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "提醒或清单不存在")
	case errors.Is(err, errVersionConflict):
		response.Error(c, http.StatusConflict, 409, err.Error())
	default:
		response.ErrorBadRequest(c, err.Error())
	}
}

func handleListLists(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := currentUserID(c)
		if _, err := ensureDefaultList(db, userID); err != nil {
			response.ErrorInternal(c, "初始化默认清单失败")
			return
		}
		var lists []List
		if err := db.Where("user_id = ? AND deleted_at IS NULL", userID).Order("is_default DESC, position ASC, id ASC").Find(&lists).Error; err != nil {
			response.ErrorInternal(c, "读取清单失败")
			return
		}
		type openCountRow struct {
			ListID    uint
			OpenCount int64
		}
		var countRows []openCountRow
		if err := db.Model(&Reminder{}).
			Select("list_id, COUNT(*) AS open_count").
			Where("user_id = ? AND completed_at IS NULL AND deleted_at IS NULL", userID).
			Group("list_id").Scan(&countRows).Error; err != nil {
			response.ErrorInternal(c, "统计清单失败")
			return
		}
		openCounts := make(map[uint]int64, len(countRows))
		for _, row := range countRows {
			openCounts[row.ListID] = row.OpenCount
		}
		result := make([]ListDTO, 0, len(lists))
		for _, list := range lists {
			result = append(result, ListDTO{List: list, OpenCount: openCounts[list.ID]})
		}
		response.Success(c, result)
	}
}

func handleCreateList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Name  string `json:"name"`
			Color string `json:"color"`
			Icon  string `json:"icon"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "清单参数无效")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || len([]rune(in.Name)) > 40 {
			response.ErrorBadRequest(c, "清单名称应为 1–40 个字符")
			return
		}
		if in.Color == "" {
			in.Color = "blue"
		}
		if in.Icon == "" {
			in.Icon = "list"
		}
		var maxPosition int
		_ = db.Model(&List{}).Where("user_id = ?", currentUserID(c)).Select("COALESCE(MAX(position), 0)").Scan(&maxPosition).Error
		list := List{UserID: currentUserID(c), Name: in.Name, Color: in.Color, Icon: in.Icon, Position: maxPosition + 1}
		if err := db.Create(&list).Error; err != nil {
			response.ErrorInternal(c, "创建清单失败")
			return
		}
		response.Success(c, list)
	}
}

func handleUpdateList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var in struct {
			Name     string `json:"name"`
			Color    string `json:"color"`
			Icon     string `json:"icon"`
			Position *int   `json:"position"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "清单参数无效")
			return
		}
		var list List
		if err := db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, currentUserID(c)).First(&list).Error; err != nil {
			writeServiceError(c, err)
			return
		}
		updates := map[string]interface{}{}
		if name := strings.TrimSpace(in.Name); name != "" {
			updates["name"] = name
		}
		if in.Color != "" {
			updates["color"] = in.Color
		}
		if in.Icon != "" {
			updates["icon"] = in.Icon
		}
		if in.Position != nil {
			updates["position"] = *in.Position
		}
		if err := db.Model(&list).Updates(updates).Error; err != nil {
			response.ErrorInternal(c, "更新清单失败")
			return
		}
		db.First(&list, id)
		response.Success(c, list)
	}
}

func handleDeleteList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		userID := currentUserID(c)
		err := db.Transaction(func(tx *gorm.DB) error {
			var list List
			if err := tx.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).First(&list).Error; err != nil {
				return err
			}
			if list.IsDefault {
				return errors.New("默认清单不能删除")
			}
			fallback, err := ensureDefaultList(tx, userID)
			if err != nil {
				return err
			}
			if err := tx.Model(&Reminder{}).Where("user_id = ? AND list_id = ? AND deleted_at IS NULL", userID, id).Update("list_id", fallback.ID).Error; err != nil {
				return err
			}
			now := time.Now()
			return tx.Model(&list).Update("deleted_at", &now).Error
		})
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, gin.H{"deleted": true})
	}
}

func handleListReminders(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var listID uint
		if raw := c.Query("list_id"); raw != "" {
			if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
				listID = uint(n)
			}
		}
		items, err := listReminders(db, currentUserID(c), c.DefaultQuery("view", "today"), c.Query("q"), listID)
		if err != nil {
			response.ErrorInternal(c, "读取提醒失败")
			return
		}
		response.Success(c, items)
	}
}

func handleGetReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		item, err := getReminder(db, currentUserID(c), id)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleCreateReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in SaveReminderInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "提醒参数无效")
			return
		}
		item, err := createReminder(db, currentUserID(c), in)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleUpdateReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var in SaveReminderInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "提醒参数无效")
			return
		}
		item, err := updateReminder(db, currentUserID(c), id, in)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleDeleteReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		if err := deleteReminder(db, currentUserID(c), id); err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, gin.H{"deleted": true})
	}
}

func handleCompleteReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		item, err := completeReminder(db, currentUserID(c), id)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleRestoreReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		item, err := restoreReminder(db, currentUserID(c), id)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleSnoozeReminder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var in struct {
			Until time.Time `json:"until"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || in.Until.IsZero() {
			response.ErrorBadRequest(c, "请选择稍后提醒时间")
			return
		}
		item, err := snoozeReminder(db, currentUserID(c), id, in.Until)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		response.Success(c, item)
	}
}

func handleSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := currentUserID(c)
		loc, _ := time.LoadLocation("Asia/Shanghai")
		now := time.Now()
		local := now.In(loc)
		endToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
		var counts struct {
			Today     int64
			Planned   int64
			OpenTotal int64
			Completed int64
			Overdue   int64
		}
		if err := db.Model(&Reminder{}).
			Select(`
				COALESCE(SUM(CASE WHEN completed_at IS NULL AND due_at IS NOT NULL AND due_at < ? THEN 1 ELSE 0 END), 0) AS today,
				COALESCE(SUM(CASE WHEN completed_at IS NULL AND due_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS planned,
				COALESCE(SUM(CASE WHEN completed_at IS NULL THEN 1 ELSE 0 END), 0) AS open_total,
				COALESCE(SUM(CASE WHEN completed_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS completed,
				COALESCE(SUM(CASE WHEN completed_at IS NULL AND due_at IS NOT NULL AND due_at < ? THEN 1 ELSE 0 END), 0) AS overdue`, endToday, now).
			Where("user_id = ? AND deleted_at IS NULL", userID).
			Scan(&counts).Error; err != nil {
			response.ErrorInternal(c, "统计提醒失败")
			return
		}
		var unread int64
		if err := db.Model(&Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&unread).Error; err != nil {
			response.ErrorInternal(c, "统计通知失败")
			return
		}
		response.Success(c, gin.H{
			"today":     counts.Today,
			"planned":   counts.Planned,
			"all":       counts.OpenTotal,
			"completed": counts.Completed,
			"overdue":   counts.Overdue,
			"unread":    unread,
		})
	}
}

func handleNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rows []Notification
		if err := db.Where("user_id = ?", currentUserID(c)).Order("id DESC").Limit(100).Find(&rows).Error; err != nil {
			response.ErrorInternal(c, "读取通知失败")
			return
		}
		response.Success(c, rows)
	}
}

func handleUnreadCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64
		if err := db.Model(&Notification{}).Where("user_id = ? AND read_at IS NULL", currentUserID(c)).Count(&count).Error; err != nil {
			response.ErrorInternal(c, "读取未读数量失败")
			return
		}
		response.Success(c, gin.H{"count": count})
	}
}

func handleReadNotification(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		now := time.Now()
		result := db.Model(&Notification{}).Where("id = ? AND user_id = ?", id, currentUserID(c)).Update("read_at", &now)
		if result.Error != nil {
			response.ErrorInternal(c, "更新通知失败")
			return
		}
		if result.RowsAffected == 0 {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "通知不存在")
			return
		}
		response.Success(c, gin.H{"read": true})
	}
}

func handleReadAllNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		if err := db.Model(&Notification{}).Where("user_id = ? AND read_at IS NULL", currentUserID(c)).Update("read_at", &now).Error; err != nil {
			response.ErrorInternal(c, "更新通知失败")
			return
		}
		response.Success(c, gin.H{"read": true})
	}
}
