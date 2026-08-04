package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"0.10.0", "0.9.0", 1}, // numeric, not lexical
		{"0.9.0", "0.10.0", -1},
		{"2.0", "2.0.0", 0},
		{"1.2.3", "1.2", 1},
		{"dev", "dev", 0}, // non-numeric treated as 0
		{"1.0", "dev", 1}, // 1 > 0
		// Leading "v" prefix (Makefile injects VERSION file verbatim) must NOT
		// collapse to 0 — otherwise documented plain-numeric upgrades like
		// "0.0.1" appear newer than a "v1.0.0" binary and silently break the
		// upgrade ladder and downgrade-protection guard.
		{"v1.0.0", "1.0.0", 0},
		{"V1.0.0", "1.0.0", 0},
		{"0.0.1", "v1.0.0", -1}, // would be +1 without strip → fatal trap
		{"v0.0.1", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func freshDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&UpgradeRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRunUpgradesRejectsDowngrade(t *testing.T) {
	db := freshDB(t)
	// Pretend a previous binary already migrated us to 1.0.0.
	db.Create(&UpgradeRecord{Version: "1.0.0"})

	err := RunUpgrades(db, "0.5.0", []Upgrade{})
	if err == nil {
		t.Fatalf("expected downgrade error, got nil")
	}
}

func TestRunUpgradesAppliesPendingAndSkipsApplied(t *testing.T) {
	db := freshDB(t)
	db.Create(&UpgradeRecord{Version: "0.1.0"})

	called := false
	upgrades := []Upgrade{
		{Version: "0.1.0", Upgrade: func(db *gorm.DB) error {
			called = true
			return nil
		}},
		{Version: "0.2.0", Name: "add_x", Upgrade: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE IF NOT EXISTS x (id INTEGER)").Error
		}},
	}
	if err := RunUpgrades(db, "0.2.0", upgrades); err != nil {
		t.Fatalf("RunUpgrades: %v", err)
	}
	if called {
		t.Fatalf("already-applied 0.1.0 ran again")
	}

	var rec UpgradeRecord
	if err := db.Where("version = ?", "0.2.0").First(&rec).Error; err != nil {
		t.Fatalf("0.2.0 not recorded: %v", err)
	}
	if rec.Name != "add_x" {
		t.Fatalf("name not persisted, got %q", rec.Name)
	}
}

func TestRunUpgradesNoTx(t *testing.T) {
	db := freshDB(t)
	upgrades := []Upgrade{
		{
			Version: "0.0.1",
			Name:    "no_tx_test",
			NoTx:    true,
			Upgrade: func(db *gorm.DB) error {
				return db.Exec("CREATE TABLE IF NOT EXISTS no_tx_marker (id INTEGER)").Error
			},
		},
	}
	if err := RunUpgrades(db, "0.1.0", upgrades); err != nil {
		t.Fatalf("RunUpgrades NoTx: %v", err)
	}

	var rec UpgradeRecord
	if err := db.Where("version = ?", "0.0.1").First(&rec).Error; err != nil {
		t.Fatalf("NoTx upgrade not recorded: %v", err)
	}
	if rec.Name != "no_tx_test" {
		t.Fatalf("Name not persisted for NoTx upgrade: got %q", rec.Name)
	}

	// The upgrade side effect must actually have happened outside the txn.
	var count int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='no_tx_marker'").Scan(&count)
	if count != 1 {
		t.Fatalf("NoTx upgrade did not create table (count=%d)", count)
	}
}
