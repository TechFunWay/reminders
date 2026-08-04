package sysconfig

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/response"
)

func handleGetPublicConfigs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		configs, err := GetPublicConfigs(db)
		if err != nil {
			response.ErrorInternal(c, "获取公开配置失败")
			return
		}
		response.Success(c, configs)
	}
}

func handleGetSystemConfigs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		configs, err := GetSystemConfigs(db)
		if err != nil {
			response.ErrorInternal(c, "获取系统配置失败")
			return
		}
		response.Success(c, configs)
	}
}

// handleGetSystemConfigMeta returns metadata for every non-internal system
// config, with current values read from the database. Admin only.
func handleGetSystemConfigMeta(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		metas := make([]ConfigMeta, 0)
		for _, def := range ListConfigs(ScopeSystem) {
			if def.Internal {
				continue
			}
			value := def.Default
			if v, err := GetConfig(db, def.Key, 0); err == nil {
				value = v
			}
			metas = append(metas, buildConfigMeta(def, value))
		}
		response.Success(c, metas)
	}
}

// handleGetUserConfigMeta returns metadata for user-scope configs. The value
// is the caller's own override, falling back to the registered default.
func handleGetUserConfigMeta(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		metas := make([]ConfigMeta, 0)
		for _, def := range ListConfigs(ScopeUser) {
			value := def.Default
			if v, err := GetConfig(db, def.Key, userID); err == nil {
				value = v
			}
			metas = append(metas, buildConfigMeta(def, value))
		}
		response.Success(c, metas)
	}
}

// handleGetAllConfigs merges the system configs visible to the caller with
// their own per-user configs. Admins see all system configs; regular users
// only the public ones.
func handleGetAllConfigs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		isAdmin := c.GetString("role") == "admin"
		var systemConfigs map[string]string
		var err error
		if isAdmin {
			systemConfigs, err = GetSystemConfigs(db)
		} else {
			systemConfigs, err = GetPublicConfigs(db)
		}
		if err != nil {
			response.ErrorInternal(c, "获取系统配置失败")
			return
		}
		userConfigs, err := GetUserConfigs(db, userID)
		if err != nil {
			response.ErrorInternal(c, "获取用户配置失败")
			return
		}
		result := make(map[string]string)
		for k, v := range systemConfigs {
			result[k] = v
		}
		for k, v := range userConfigs {
			result[k] = v
		}
		response.Success(c, result)
	}
}

// handleUpdateConfig is registry-driven: internal keys are read-only,
// system-scope keys are admin-only and type-validated, user-scope keys are
// type-validated and stored per caller. Unregistered keys keep the legacy
// ad-hoc per-user behavior.
func handleUpdateConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		isAdmin := c.GetString("role") == "admin"
		var req struct {
			Key   string `json:"key" binding:"required"`
			Value string `json:"value" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请求无效")
			return
		}
		def, registered := GetConfigDef(req.Key)
		switch {
		case registered && def.Internal:
			response.ErrorForbidden(c, "该配置项为只读")
			return
		case registered && def.Scope == ScopeSystem:
			if !isAdmin {
				response.ErrorForbidden(c, "系统配置需要管理员权限")
				return
			}
			if err := Validate(def, req.Value); err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
			if err := UpdateConfig(db, req.Key, req.Value, 0); err != nil {
				response.ErrorInternal(c, "更新配置失败")
				return
			}
		case registered && def.Scope == ScopeUser:
			if err := Validate(def, req.Value); err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
			if err := UpdateConfig(db, req.Key, req.Value, userID); err != nil {
				response.ErrorInternal(c, "更新配置失败")
				return
			}
		default:
			if err := UpdateConfig(db, req.Key, req.Value, userID); err != nil {
				response.ErrorInternal(c, "更新配置失败")
				return
			}
		}
		response.Success(c, nil)
	}
}

// RegisterRoutes wires config endpoints. The public group exposes read-only
// public config; the auth group covers the merged list, per-user writes and
// user metadata; the admin group guards system config reads and metadata.
func RegisterRoutes(public *gin.RouterGroup, auth *gin.RouterGroup, admin *gin.RouterGroup, db *gorm.DB) {
	public.GET("/configs/public", handleGetPublicConfigs(db))
	admin.GET("/configs/system", handleGetSystemConfigs(db))
	admin.GET("/configs/meta", handleGetSystemConfigMeta(db))
	auth.GET("/configs/user/meta", handleGetUserConfigMeta(db))
	auth.GET("/configs", handleGetAllConfigs(db))
	auth.PUT("/configs", handleUpdateConfig(db))
}
