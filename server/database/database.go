package database

import (
	"log"
	"time"

	"smallgo/server/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// coreModels are the framework's own tables, always migrated on startup.
var coreModels = []interface{}{
	&User{},
	&SystemConfig{},
	&UpgradeRecord{},
	&SecurityQuestion{},
	&AuditLog{},
}

// appModels holds models contributed by registered apps via RegisterModels.
// Apps add to this slice from init(); AutoMigrate iterates both coreModels
// and appModels together so an app's schema lands on the same startup pass
// as the framework's own tables.
var appModels []interface{}

// RegisterModels adds GORM-managed models to the migration roster. Call
// from an app's init(); it is safe to call multiple times across packages.
//
//	func init() {
//	    database.RegisterModels(&Note{}, &Tag{})
//	}
func RegisterModels(models ...interface{}) {
	appModels = append(appModels, models...)
}

func InitDB(dbPath string) (*gorm.DB, error) {
	// Keep GORM quiet about expected "record not found" lookups (used heavily
	// for upsert-style config init) while still surfacing slow queries/errors.
	gormLog := gormlogger.New(log.New(logger.NewWriter("WARN"), "", 0), gormlogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
	})

	// Embed pragmas in the DSN so every pooled connection inherits them,
	// not only the connection that happened to run the PRAGMA exec below.
	dsn := dbPath +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(ON)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SQLite serializes writes through a single connection to avoid
	// SQLITE_BUSY under concurrent goroutines.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return db, nil
}

// AutoMigrate runs GORM's AutoMigrate across all framework models and any
// additional models contributed by apps via RegisterModels.
func AutoMigrate(db *gorm.DB) error {
	models := make([]interface{}, 0, len(coreModels)+len(appModels))
	models = append(models, coreModels...)
	models = append(models, appModels...)
	return db.AutoMigrate(models...)
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
