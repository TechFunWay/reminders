package sysconfig

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"gorm.io/gorm"

	"smallgo/server/database"
)

func generateJWTSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate JWT secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// InitDefaultConfigs seeds one row per registered system-scope config.
// Existing rows are never touched, so admin edits survive restarts.
func InitDefaultConfigs(db *gorm.DB) error {
	for _, def := range ListConfigs(ScopeSystem) {
		value := def.Default
		if def.Key == "jwt_secret" {
			var err error
			value, err = generateJWTSecret()
			if err != nil {
				return err
			}
		}
		var existing database.SystemConfig
		result := db.Where("user_id = ? AND key = ?", 0, def.Key).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			row := database.SystemConfig{UserID: 0, Key: def.Key, Value: value, Public: def.Public}
			if err := db.Create(&row).Error; err != nil {
				return err
			}
		} else if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

func GetPublicConfigs(db *gorm.DB) (map[string]string, error) {
	var configs []database.SystemConfig
	if err := db.Where("public = ?", true).Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, c := range configs {
		if isInternalKey(c.Key) {
			continue
		}
		result[c.Key] = c.Value
	}
	return result, nil
}

func GetSystemConfigs(db *gorm.DB) (map[string]string, error) {
	var configs []database.SystemConfig
	if err := db.Where("user_id = ?", 0).Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, c := range configs {
		if isInternalKey(c.Key) {
			continue
		}
		result[c.Key] = c.Value
	}
	return result, nil
}

func GetUserConfigs(db *gorm.DB, userID uint) (map[string]string, error) {
	var configs []database.SystemConfig
	if err := db.Where("user_id = ?", userID).Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, c := range configs {
		result[c.Key] = c.Value
	}
	return result, nil
}

func GetConfig(db *gorm.DB, key string, userID uint) (string, error) {
	var config database.SystemConfig
	if err := db.Where("user_id = ? AND key = ?", userID, key).First(&config).Error; err != nil {
		return "", err
	}
	return config.Value, nil
}

func UpdateConfig(db *gorm.DB, key string, value string, userID uint) error {
	var config database.SystemConfig
	result := db.Where("user_id = ? AND key = ?", userID, key).First(&config)
	if result.Error == gorm.ErrRecordNotFound {
		config = database.SystemConfig{
			UserID: userID,
			Key:    key,
			Value:  value,
		}
		return db.Create(&config).Error
	}
	if result.Error != nil {
		return result.Error
	}
	return db.Model(&config).Update("value", value).Error
}

// ConfigMeta is the API view of a registered config definition joined with
// its effective value.
type ConfigMeta struct {
	Key         string    `json:"key"`
	Scope       Scope     `json:"scope"`
	Type        ValueType `json:"type"`
	Value       string    `json:"value"`
	Default     string    `json:"default"`
	Public      bool      `json:"public"`
	Group       string    `json:"group"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Options     []string  `json:"options,omitempty"`
}

func buildConfigMeta(def ConfigDef, value string) ConfigMeta {
	return ConfigMeta{
		Key:         def.Key,
		Scope:       def.Scope,
		Type:        def.Type,
		Value:       value,
		Default:     def.Default,
		Public:      def.Public,
		Group:       def.Group,
		Label:       def.Label,
		Description: def.Description,
		Options:     def.Options,
	}
}
