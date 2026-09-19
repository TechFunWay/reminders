package stats

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"smallgo/server/database"
	"smallgo/server/sysconfig"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.InitDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := sysconfig.InitDefaultConfigs(db); err != nil {
		t.Fatalf("init configs: %v", err)
	}
	t.Cleanup(func() { database.CloseDB(db) })
	return db
}

func TestEndpointEnvOverride(t *testing.T) {
	t.Setenv("STATS_ENDPOINT", "http://127.0.0.1:1/fake")
	if got := Endpoint(); got != "http://127.0.0.1:1/fake" {
		t.Fatalf("Endpoint() = %q, want env override", got)
	}
	os.Unsetenv("STATS_ENDPOINT")
	if got := Endpoint(); got != "https://techfunway.wycto.cn/api/apps.online/refresh" {
		t.Fatalf("Endpoint() default = %q", got)
	}
}

func TestDeviceIDIsHashedAndStable(t *testing.T) {
	dir := t.TempDir()
	a := DeviceID("testapp", dir)
	b := DeviceID("testapp", dir)
	if a != b {
		t.Fatalf("DeviceID not stable: %q vs %q", a, b)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(a) {
		t.Fatalf("DeviceID = %q, want 32-hex md5", a)
	}
	if DeviceID("otherapp", dir) == a {
		t.Fatalf("different app names must not share device id")
	}
}

func TestPersistentDeviceIDReused(t *testing.T) {
	dir := t.TempDir()
	first := persistentDeviceID(dir)
	if first == "" {
		t.Fatal("persistentDeviceID returned empty")
	}
	if _, err := os.Stat(filepath.Join(dir, "device.id")); err != nil {
		t.Fatalf("device.id not persisted: %v", err)
	}
	if second := persistentDeviceID(dir); second != first {
		t.Fatalf("persistent id changed: %q vs %q", first, second)
	}
}

func TestDonateSupport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(t *testing.T, db *gorm.DB) *gin.Engine {
		r := gin.New()
		RegisterRoutes(r.Group("/api"), db)
		return r
	}

	t.Run("success", func(t *testing.T) {
		db := setupDB(t)
		var received map[string]any
		var calls atomic.Int32
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer fake.Close()
		t.Setenv("STATS_ENDPOINT", fake.URL)

		Init("testapp", "v1.0.0", "fnos", t.TempDir())
		r := newRouter(t, db)

		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/donate/support", nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if calls.Load() != 1 {
			t.Fatalf("upstream calls = %d, want 1", calls.Load())
		}
		if received["event"] != donateEvent {
			t.Fatalf("event = %v, want %q", received["event"], donateEvent)
		}
		if received["app_name"] != "testapp" {
			t.Fatalf("app_name = %v", received["app_name"])
		}
		// 未附带金额时不上报 amount 字段（omitempty）
		if _, ok := received["amount"]; ok {
			t.Fatalf("amount should be omitted when not provided, got %v", received["amount"])
		}
		// 隐私红线：不允许出现主机名字段
		if _, ok := received["hostname"]; ok {
			t.Fatal("hostname must never be reported")
		}
	})

	t.Run("amount forwarded", func(t *testing.T) {
		db := setupDB(t)
		var received map[string]any
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
		}))
		defer fake.Close()
		t.Setenv("STATS_ENDPOINT", fake.URL)

		Init("testapp", "v1.0.0", "", t.TempDir())
		r := newRouter(t, db)

		body := strings.NewReader(`{"amount": 6.6}`)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/donate/support", body)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if received["amount"] != 6.6 {
			t.Fatalf("amount = %v, want 6.6", received["amount"])
		}
	})

	t.Run("negative amount clamped to 0", func(t *testing.T) {
		db := setupDB(t)
		var received map[string]any
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
		}))
		defer fake.Close()
		t.Setenv("STATS_ENDPOINT", fake.URL)

		Init("testapp", "v1.0.0", "", t.TempDir())
		r := newRouter(t, db)

		body := strings.NewReader(`{"amount": -5}`)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/donate/support", body)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
		}
		// 负数金额被钳制为 0，0 值因 omitempty 不上报
		if _, ok := received["amount"]; ok {
			t.Fatalf("clamped amount should be omitted, got %v", received["amount"])
		}
	})

	t.Run("upstream 5xx fails", func(t *testing.T) {
		db := setupDB(t)
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer fake.Close()
		t.Setenv("STATS_ENDPOINT", fake.URL)

		Init("testapp", "v1.0.0", "", t.TempDir())
		r := newRouter(t, db)

		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/donate/support", nil))
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 on upstream failure", resp.Code)
		}
	})

	t.Run("disabled by admin config", func(t *testing.T) {
		db := setupDB(t)
		if err := sysconfig.UpdateConfig(db, "stats_enabled", "false", 0); err != nil {
			t.Fatalf("disable stats: %v", err)
		}
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer fake.Close()
		t.Setenv("STATS_ENDPOINT", fake.URL)

		Init("testapp", "v1.0.0", "", t.TempDir())
		r := newRouter(t, db)

		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/donate/support", nil))
		if resp.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 when disabled", resp.Code)
		}
	})
}
