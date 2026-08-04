package qrcode

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
	"smallgo/server/apps"
	"smallgo/server/database"
	"smallgo/server/response"
)

// QRCodeStat is a GORM model contributed by this app to AutoMigrate. It
// demonstrates the database.RegisterModels extension point: app-owned
// tables land on the same startup pass as the framework's core models.
type QRCodeStat struct {
	ID        uint   `gorm:"primarykey"`
	Content   string `gorm:"index;not null"`
	Size      int
	CreatedAt time.Time `gorm:"index"`
}

func init() {
	database.RegisterModels(&QRCodeStat{})

	// Contribute a one-off data migration tracked in upgrade_records.
	// Plain dotted-numeric Version matches the README/AGENTS examples;
	// RunUpgrades strips a leading "v" before comparing.
	database.Upgrades = append(database.Upgrades, database.Upgrade{
		Version: "0.0.1",
		Name:    "qrcode_seed_default_stat",
		Upgrade: func(db *gorm.DB) error {
			var count int64
			if err := db.Model(&QRCodeStat{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			return db.Create(&QRCodeStat{
				Content:   "https://techfunway.wycto.cn",
				Size:      256,
				CreatedAt: time.Now(),
			}).Error
		},
	})

	apps.Register(apps.App{
		Name:        "qrcode",
		DisplayName: "二维码",
		Icon:        "qr",
		RoutePrefix: "/qrcode",
		NavPosition: 200,
		Setup:       setupRoutes,
	})
}

func setupRoutes(api *gin.RouterGroup, db *gorm.DB) {
	api.GET("/qrcode", HandleQRCode(db))
}

// HandleQRCode returns a PNG QR code for the `content` query param. The
// `size` parameter is capped at maxSizePx so a hostile client can't pin
// the CPU/memory by requesting `?size=100000`.
func HandleQRCode(db *gorm.DB) gin.HandlerFunc {
	const (
		defaultSizePx   = 256
		maxSizePx       = 1024
		maxContentBytes = 2048
	)
	return func(c *gin.Context) {
		content := c.Query("content")
		if content == "" {
			response.ErrorBadRequest(c, "请输入内容")
			return
		}
		if len(content) > maxContentBytes {
			response.ErrorBadRequest(c, "内容不能超过 2048 字节")
			return
		}

		size := defaultSizePx
		if s, err := strconv.Atoi(c.Query("size")); err == nil {
			switch {
			case s <= 0:
			case s > maxSizePx:
				size = maxSizePx
			default:
				size = s
			}
		}

		png, err := qrcode.Encode(content, qrcode.Medium, size)
		if err != nil {
			response.ErrorInternal(c, "生成二维码失败")
			return
		}

		// Best-effort stat row — a failed insert must NOT fail the request.
		_ = db.Create(&QRCodeStat{
			Content:   content,
			Size:      size,
			CreatedAt: time.Now(),
		}).Error

		c.Data(http.StatusOK, "image/png", png)
	}
}
