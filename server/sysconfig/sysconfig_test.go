package sysconfig

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"smallgo/server/database"
)

// setupTestDB opens a fresh temp-dir SQLite database with all models migrated.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func findDef(defs []ConfigDef, key string) (ConfigDef, bool) {
	for _, d := range defs {
		if d.Key == key {
			return d, true
		}
	}
	return ConfigDef{}, false
}

// TestRegistryContainsBuiltins locks the six built-in config definitions:
// five system-scope (site_title, site_description, allow_register,
// require_login, jwt_secret) and one user-scope (theme_mode).
func TestRegistryContainsBuiltins(t *testing.T) {
	// Given: the built-in registrations from init()
	// When: listing each scope
	system := ListConfigs(ScopeSystem)
	user := ListConfigs(ScopeUser)

	// Then: all six built-ins are present in their scope
	for _, key := range []string{"site_title", "site_description", "allow_register", "require_login", "jwt_secret"} {
		if _, ok := findDef(system, key); !ok {
			t.Fatalf("system registry missing built-in %q", key)
		}
	}
	if _, ok := findDef(user, "theme_mode"); !ok {
		t.Fatalf("user registry missing built-in %q", "theme_mode")
	}

	// Then: spot-check metadata of the most safety-relevant defs
	allowReg, _ := findDef(system, "allow_register")
	if allowReg.Type != TypeBool || !allowReg.Public || allowReg.Default != "true" || allowReg.Group != "access" {
		t.Fatalf("allow_register def wrong: %+v", allowReg)
	}
	secret, _ := findDef(system, "jwt_secret")
	if !secret.Internal {
		t.Fatalf("jwt_secret must be internal: %+v", secret)
	}
	theme, _ := findDef(user, "theme_mode")
	if theme.Type != TypeSelect || theme.Default != "system" || theme.Group != "appearance" {
		t.Fatalf("theme_mode def wrong: %+v", theme)
	}
	if len(theme.Options) != 3 {
		t.Fatalf("theme_mode options = %v, want 3 entries", theme.Options)
	}
}

// TestRegisterConfigDuplicatePanics ensures the registry refuses ambiguous
// double-registration of the same key.
func TestRegisterConfigDuplicatePanics(t *testing.T) {
	// Given: a key that is already registered (uses a test-only scope so the
	// extra entry cannot affect built-in listings)
	RegisterConfig(ConfigDef{Key: "test_duplicate_key", Scope: Scope("test"), Type: TypeString})

	// When/Then: registering the same key again panics
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate RegisterConfig")
		}
	}()
	RegisterConfig(ConfigDef{Key: "test_duplicate_key", Scope: Scope("test"), Type: TypeString})
}

// TestInitDefaultConfigsSeedsSystemConfigs verifies a fresh database gets one
// non-empty row per registered system config.
func TestInitDefaultConfigsSeedsSystemConfigs(t *testing.T) {
	// Given: an empty migrated database
	db := setupTestDB(t)

	// When: seeding defaults
	if err := InitDefaultConfigs(db); err != nil {
		t.Fatalf("init defaults: %v", err)
	}

	// Then: every registered system config exists with a non-empty value
	for _, key := range []string{"site_title", "site_description", "allow_register", "require_login", "jwt_secret"} {
		value, err := GetConfig(db, key, 0)
		if err != nil {
			t.Fatalf("seeded config %q missing: %v", key, err)
		}
		if value == "" {
			t.Fatalf("seeded config %q is empty", key)
		}
	}
}

// TestInitDefaultConfigsPreservesAdminEdits runs the seeder twice: values an
// admin changed between runs must survive, and no duplicate rows may appear.
func TestInitDefaultConfigsPreservesAdminEdits(t *testing.T) {
	// Given: a seeded database where the admin edited site_title
	db := setupTestDB(t)
	if err := InitDefaultConfigs(db); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := UpdateConfig(db, "site_title", "Edited Title", 0); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// When: the seeder runs again (restart)
	if err := InitDefaultConfigs(db); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	// Then: the admin edit survives and no rows are duplicated
	value, err := GetConfig(db, "site_title", 0)
	if err != nil {
		t.Fatal(err)
	}
	if value != "Edited Title" {
		t.Fatalf("admin edit overwritten, site_title = %q", value)
	}
	var count int64
	if err := db.Model(&database.SystemConfig{}).Where("user_id = 0").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("system config rows = %d, want 6 (no duplicates)", count)
	}
}

// TestInitDefaultConfigsRandomSeedsJWTSecret proves two independent installs
// get distinct JWT secrets.
func TestInitDefaultConfigsRandomSeedsJWTSecret(t *testing.T) {
	// Given: two independent fresh installs
	db1 := setupTestDB(t)
	db2 := setupTestDB(t)

	// When: seeding both
	if err := InitDefaultConfigs(db1); err != nil {
		t.Fatal(err)
	}
	if err := InitDefaultConfigs(db2); err != nil {
		t.Fatal(err)
	}

	// Then: each gets a non-empty, distinct jwt_secret
	s1, err1 := GetConfig(db1, "jwt_secret", 0)
	s2, err2 := GetConfig(db2, "jwt_secret", 0)
	if err1 != nil || err2 != nil {
		t.Fatalf("jwt_secret not seeded: %v / %v", err1, err2)
	}
	if s1 == "" || s2 == "" {
		t.Fatal("jwt_secret seeded empty")
	}
	if s1 == s2 {
		t.Fatalf("jwt_secret not random: both %q", s1)
	}
}

// TestValidateRejectsInvalidValues covers bool/int/select mismatches.
func TestValidateRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		def   ConfigDef
		value string
	}{
		{"bool rejects maybe", ConfigDef{Key: "b", Type: TypeBool}, "maybe"},
		{"bool rejects 1", ConfigDef{Key: "b", Type: TypeBool}, "1"},
		{"int rejects abc", ConfigDef{Key: "i", Type: TypeInt}, "abc"},
		{"int rejects 1.5", ConfigDef{Key: "i", Type: TypeInt}, "1.5"},
		{"select rejects unknown option", ConfigDef{Key: "s", Type: TypeSelect, Options: []string{"a", "b"}}, "c"},
		{"select rejects empty", ConfigDef{Key: "s", Type: TypeSelect, Options: []string{"a", "b"}}, ""},
	}
	for _, tc := range cases {
		// When/Then: an ill-typed value is rejected
		if err := Validate(tc.def, tc.value); err == nil {
			t.Fatalf("%s: Validate(%q) accepted", tc.name, tc.value)
		}
	}
}

// TestValidateAcceptsValidValues covers the happy paths for every type.
func TestValidateAcceptsValidValues(t *testing.T) {
	cases := []struct {
		name  string
		def   ConfigDef
		value string
	}{
		{"string accepts anything", ConfigDef{Key: "s", Type: TypeString}, "whatever"},
		{"string accepts empty", ConfigDef{Key: "s", Type: TypeString}, ""},
		{"bool accepts true", ConfigDef{Key: "b", Type: TypeBool}, "true"},
		{"bool accepts false", ConfigDef{Key: "b", Type: TypeBool}, "false"},
		{"int accepts positive", ConfigDef{Key: "i", Type: TypeInt}, "42"},
		{"int accepts negative", ConfigDef{Key: "i", Type: TypeInt}, "-7"},
		{"select accepts listed option", ConfigDef{Key: "s", Type: TypeSelect, Options: []string{"a", "b"}}, "a"},
	}
	for _, tc := range cases {
		// When/Then: a well-typed value passes
		if err := Validate(tc.def, tc.value); err != nil {
			t.Fatalf("%s: Validate(%q) = %v", tc.name, tc.value, err)
		}
	}
}
