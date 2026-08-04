package sysconfig

import (
	"fmt"
	"sort"
	"strconv"
)

// Scope identifies who a config belongs to: the whole installation (system,
// admin-managed) or an individual user.
type Scope string

const (
	ScopeSystem Scope = "system"
	ScopeUser   Scope = "user"
)

// ValueType describes the syntax of a config value for validation and UI
// rendering.
type ValueType string

const (
	TypeString ValueType = "string"
	TypeBool   ValueType = "bool"
	TypeInt    ValueType = "int"
	TypeSelect ValueType = "select"
)

// ConfigDef is the static definition of a registered config key.
type ConfigDef struct {
	Key         string
	Scope       Scope
	Type        ValueType
	Default     string
	Public      bool
	Internal    bool
	Group       string
	Label       string
	Description string
	Options     []string
}

var configRegistry = map[string]ConfigDef{}

// RegisterConfig adds a config definition. It panics on duplicate keys —
// registration happens in init() so a duplicate is a programming error that
// must surface immediately.
func RegisterConfig(def ConfigDef) {
	if _, exists := configRegistry[def.Key]; exists {
		panic(fmt.Sprintf("sysconfig: duplicate config key %q", def.Key))
	}
	configRegistry[def.Key] = def
}

// ListConfigs returns all registered definitions for a scope, sorted by key
// for deterministic output.
func ListConfigs(scope Scope) []ConfigDef {
	defs := make([]ConfigDef, 0)
	for _, def := range configRegistry {
		if def.Scope == scope {
			defs = append(defs, def)
		}
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Key < defs[j].Key })
	return defs
}

// GetConfigDef returns the registered definition for a key.
func GetConfigDef(key string) (ConfigDef, bool) {
	def, ok := configRegistry[key]
	return def, ok
}

// isInternalKey reports whether a key is registered as internal-only (never
// exposed through any API listing, never writable via the config API).
func isInternalKey(key string) bool {
	def, ok := configRegistry[key]
	return ok && def.Internal
}

// Validate checks a raw value against the definition's type. Booleans accept
// only "true"/"false" (consumers compare against those exact strings);
// selects accept only one of the declared options.
func Validate(def ConfigDef, value string) error {
	switch def.Type {
	case TypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("config %q must be \"true\" or \"false\"", def.Key)
		}
	case TypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("config %q must be an integer", def.Key)
		}
	case TypeSelect:
		for _, opt := range def.Options {
			if value == opt {
				return nil
			}
		}
		return fmt.Errorf("config %q must be one of %v", def.Key, def.Options)
	}
	return nil
}

func init() {
	RegisterConfig(ConfigDef{
		Key: "site_title", Scope: ScopeSystem, Type: TypeString,
		Default: "提醒事项", Public: true,
		Group: "general", Label: "站点标题",
		Description: "显示在浏览器标题和页面头部的站点名称",
	})
	RegisterConfig(ConfigDef{
		Key: "site_description", Scope: ScopeSystem, Type: TypeString,
		Default: "简洁可靠的多渠道提醒应用", Public: true,
		Group: "general", Label: "站点描述",
		Description: "站点的简短介绍",
	})
	RegisterConfig(ConfigDef{
		Key: "allow_register", Scope: ScopeSystem, Type: TypeBool,
		Default: "true", Public: true,
		Group: "access", Label: "允许注册",
		Description: "允许新用户创建账号",
	})
	RegisterConfig(ConfigDef{
		Key: "require_login", Scope: ScopeSystem, Type: TypeBool,
		Default: "true", Public: true,
		Group: "access", Label: "需要登录",
		Description: "要求访客登录后才能使用应用",
	})
	RegisterConfig(ConfigDef{
		Key: "jwt_secret", Scope: ScopeSystem, Type: TypeString,
		Internal: true,
		Group:    "internal", Label: "JWT Secret",
		Description: "Secret used to sign session tokens (auto-generated)",
	})
	RegisterConfig(ConfigDef{
		Key: "theme_mode", Scope: ScopeUser, Type: TypeSelect,
		Default: "system", Options: []string{"system", "light", "dark"},
		Group: "appearance", Label: "主题模式",
		Description: "界面配色方案",
	})
	RegisterConfig(ConfigDef{
		Key: "log_retention_days", Scope: ScopeSystem, Type: TypeInt,
		Default: "30",
		Group:   "system", Label: "日志与操作记录保留天数",
		Description: "运行日志与操作日志自动清理的最大保留天数，0 表示永久保留",
	})
}
