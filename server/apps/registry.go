// Package apps provides a registry for modular application components.
//
// Each app registers itself via init() and receives the API router group plus
// the database connection during server startup. Apps can additionally
// contribute GORM models to AutoMigrate, contribute one-off data migrations,
// and run teardown code on shutdown. This keeps the core server.go clean
// and lets each app manage its own routes, schema, and lifecycle.
//
// Usage:
//
//	func init() {
//	    apps.Register(apps.App{
//	        Name:        "myapp",
//	        DisplayName: "My App",
//	        Icon:        "sparkles",
//	        RoutePrefix: "/myapp",
//	        NavPosition: 100,
//	        Setup: func(api *gin.RouterGroup, db *gorm.DB) {
//	            api.GET("/myapp/hello", handleHello)
//	        },
//	    })
//	}
//
// All optional fields are zero-value safe; an app that only needs Setup is
// fine. The dispatcher skips nil callbacks.
package apps

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// App describes a self-registering module that extends the base server.
// To contribute GORM models to automigrate, call database.RegisterModels
// from the same init(); to contribute one-off data migrations, append to
// database.Upgrades.
type App struct {
	Name        string
	DisplayName string                                  // sidebar label; falls back to Name when empty
	Icon        string                                  // sidebar icon (SVG name or emoji)
	RoutePrefix string                                  // URL prefix the app owns, e.g. "/qrcode"
	NavPosition int                                     // sidebar ordering; 0 means "off the bottom"
	Setup       func(api *gin.RouterGroup, db *gorm.DB) // route registration
	SetupAuth   func(api *gin.RouterGroup, db *gorm.DB) // authenticated route registration
	SetupAdmin  func(api *gin.RouterGroup, db *gorm.DB) // administrator route registration
	Migrate     func(db *gorm.DB) error                 // optional one-off data migration (runs after AutoMigrate)
	Shutdown    func()                                  // optional cleanup at process exit
}

var registered []App

func Register(a App) {
	registered = append(registered, a)
}

func All() []App {
	return registered
}

// SetupAll wires every registered app's routes into the given group. Apps
// without a Setup callback are skipped, so partially registered apps remain
// valid during incremental migration.
func SetupAll(api *gin.RouterGroup, db *gorm.DB) {
	for _, a := range registered {
		if a.Setup != nil {
			a.Setup(api, db)
		}
	}
}

// SetupProtectedAll wires authenticated and administrator-only routes after
// the framework has attached the corresponding middleware. Keeping this
// separate preserves the original public app extension point.
func SetupProtectedAll(auth, admin *gin.RouterGroup, db *gorm.DB) {
	for _, a := range registered {
		if a.SetupAuth != nil {
			a.SetupAuth(auth, db)
		}
		if a.SetupAdmin != nil {
			a.SetupAdmin(admin, db)
		}
	}
}

// MigrateAll invokes each app's optional Migrate callback (for one-off data
// migrations distinct from schema migration). Returns the first error; later
// apps are not run.
func MigrateAll(db *gorm.DB) error {
	for _, a := range registered {
		if a.Migrate != nil {
			if err := a.Migrate(db); err != nil {
				return err
			}
		}
	}
	return nil
}

// ShutdownAll invokes each app's optional Shutdown callback in reverse
// registration order so dependencies that registered later clean up first.
func ShutdownAll() {
	for i := len(registered) - 1; i >= 0; i-- {
		if registered[i].Shutdown != nil {
			registered[i].Shutdown()
		}
	}
}
