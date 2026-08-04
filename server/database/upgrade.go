package database

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UpgradeFunc func(db *gorm.DB) error

// Upgrade describes a one-off data migration that runs after AutoMigrate.
// Each Upgrade runs at most once (tracked in upgrade_records by Version).
type Upgrade struct {
	Version string
	Name    string // optional human-readable label, stored on UpgradeRecord
	Upgrade UpgradeFunc
	NoTx    bool // run outside a transaction (rare: SQLite DDL that cannot run in one)
}

// Upgrades holds versioned, one-off data migrations that run after AutoMigrate.
// Each upgrade runs at most once (tracked in the upgrade_records table). Append
// to this slice from an init() in your own package, or edit it directly:
//
//	database.Upgrades = append(database.Upgrades, database.Upgrade{
//	    Version: "0.0.2",
//	    Upgrade: func(db *gorm.DB) error { return db.Exec("...").Error },
//	})
var Upgrades []Upgrade

// RunUpgrades applies every upgrade whose Version has not yet been recorded,
// in ascending version order, each inside its own transaction (unless NoTx
// is set). It refuses to start if an already-applied DB version is higher
// than the supplied appVersion, which protects against running an older
// binary against a newer schema.
//
// It is safe to call on every startup and is idempotent.
func RunUpgrades(db *gorm.DB, appVersion string, upgrades []Upgrade) error {
	var records []UpgradeRecord
	if err := db.Find(&records).Error; err != nil {
		return err
	}
	applied := make(map[string]bool, len(records))
	for _, r := range records {
		applied[r.Version] = true
		if compareVersions(r.Version, appVersion) > 0 {
			return fmt.Errorf("database schema (%s) is newer than this binary (%s); refusing to start to avoid corruption", r.Version, appVersion)
		}
	}

	pending := make([]Upgrade, 0, len(upgrades))
	for _, u := range upgrades {
		if !applied[u.Version] && compareVersions(u.Version, appVersion) <= 0 {
			pending = append(pending, u)
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return compareVersions(pending[i].Version, pending[j].Version) < 0
	})

	for _, u := range pending {
		if err := applyUpgrade(db, u); err != nil {
			return fmt.Errorf("upgrade %s: %w", u.Version, err)
		}
	}
	return nil
}

func applyUpgrade(db *gorm.DB, u Upgrade) error {
	rec := UpgradeRecord{Version: u.Version, Name: u.Name, UpgradedAt: time.Now()}
	if u.NoTx {
		if err := u.Upgrade(db); err != nil {
			return err
		}
		return db.Create(&rec).Error
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := u.Upgrade(tx); err != nil {
			return err
		}
		return tx.Create(&rec).Error
	})
}

// compareVersions compares dotted version strings numerically segment by
// segment (e.g. "0.10.0" > "0.9.0"). Non-numeric segments compare as 0, so
// arbitrary labels sort consistently without bricking startup. A leading
// "v" or "V" prefix is stripped so project VERSION files like "v1.0.0"
// compare equal to plain "1.0.0".
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	a = strings.TrimPrefix(a, "V")
	b = strings.TrimPrefix(b, "v")
	b = strings.TrimPrefix(b, "V")
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}
