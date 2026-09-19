package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"smallgo/server/auth"
	"smallgo/server/config"
	"smallgo/server/database"
	_ "smallgo/server/qrcode"
	"smallgo/server/sysconfig"
	"smallgo/server/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setupTestRouter spins up the full router against a fresh temp-dir SQLite DB.
func setupTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

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
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || secret == "" {
		t.Fatalf("jwt secret: %v", err)
	}

	cfg := config.Config{
		CORSOrigin: "*",
		RateLimit:  0, // unlimited for tests
		UploadDir:  filepath.Join(dir, "uploads"),
	}
	return NewRouter(cfg, db, secret), db, dir
}

func setupFnOSTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || secret == "" {
		t.Fatalf("jwt secret: %v", err)
	}
	cfg := config.Config{CORSOrigin: "*", RateLimit: 0, FnOSApp: true, GatewayPrefix: "/app/techfunway-reminders"}
	return NewRouter(cfg, db, secret), db
}

func doJSON(r http.Handler, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doFnOSJSON(r http.Handler, method, path, uid, username string, body interface{}) *httptest.ResponseRecorder {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if uid != "" {
		req.Header.Set("X-Trim-Userid", uid)
		req.Header.Set("X-Trim-Username", username)
		req.Header.Set("X-Trim-Isadmin", "true")
		req = req.WithContext(user.MarkFnOSGateway(req.Context()))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doAPIKey(r http.Handler, method, path, apiKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doUpload(r http.Handler, token, filename string, content []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", filename)
	_, _ = part.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return m
}

func registerUser(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	w := doJSON(r, "POST", "/api/auth/register", "", map[string]string{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("register %s: status %d body %s", username, w.Code, w.Body.String())
	}
	data := decode(t, w)["data"].(map[string]interface{})
	return data["token"].(string)
}

func TestHealthAndVersion(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	for _, path := range []string{"/api/health", "/api/version"} {
		w := doJSON(r, "GET", path, "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if decode(t, w)["code"].(float64) != 0 {
			t.Fatalf("%s: non-zero code", path)
		}
	}
	// 非飞牛部署必须显式报 fnosApp=false：登录页靠它藏「使用飞牛 NAS 登录」。
	w := doJSON(r, "GET", "/api/version", "", nil)
	if got := decode(t, w)["data"].(map[string]interface{})["fnosApp"]; got != false {
		t.Fatalf("plain deployment fnosApp = %v, want false", got)
	}
}

// The SPA uses servicePort to tell "served under the fnOS gateway" apart from
// "served on the direct TCP listener"; only the former may derive the
// trampoline URL from the current origin.
func TestFnOSVersionExposesServicePort(t *testing.T) {
	r, _ := setupFnOSTestRouter(t)
	w := doJSON(r, "GET", "/app/techfunway-reminders/api/version", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("fnos version status %d", w.Code)
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["servicePort"] != "0" {
		t.Fatalf("servicePort = %v, want 0 (default test config port)", data["servicePort"])
	}
	if data["fnosApp"] != true {
		t.Fatalf("fnosApp = %v, want true in fnOS mode", data["fnosApp"])
	}
}

// The SPA renders its first frame from /api/bootstrap alone: public configs,
// setup state, the current session and (on fnOS) the trampoline entry all come
// back in one round trip. This pins the contract the frontend store relies on.
func TestBootstrapReturnsStartupState(t *testing.T) {
	r, _, _ := setupTestRouter(t)

	w := doJSON(r, "GET", "/api/bootstrap", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap status %d body %s", w.Code, w.Body.String())
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["setup_required"] != true {
		t.Fatalf("fresh install must report setup_required=true, got %v", data["setup_required"])
	}
	configs, ok := data["configs"].(map[string]interface{})
	if !ok || len(configs) == 0 {
		t.Fatalf("bootstrap must include public configs, got %v", data["configs"])
	}
	auth := data["auth"].(map[string]interface{})
	if auth["authenticated"] != false {
		t.Fatalf("anonymous bootstrap must not be authenticated")
	}
	if data["fnos_app"] != false {
		t.Fatalf("plain deployment must report fnos_app=false, got %v", data["fnos_app"])
	}

	token := registerUser(t, r, "admin", "secret123")

	// 带令牌的引导请求必须直接给出登录态，前端因此省掉一次 check 往返。
	w = doJSON(r, "GET", "/api/bootstrap", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("authenticated bootstrap status %d", w.Code)
	}
	data = decode(t, w)["data"].(map[string]interface{})
	if data["setup_required"] != false {
		t.Fatalf("setup_required must clear once an account exists")
	}
	auth = data["auth"].(map[string]interface{})
	if auth["authenticated"] != true {
		t.Fatalf("token must resolve to an authenticated session, got %v", auth)
	}
	user := auth["user"].(map[string]interface{})
	if user["role"] != "admin" {
		t.Fatalf("bootstrap user role = %v, want admin", user["role"])
	}

	// 无效令牌不能让引导请求 500/401：它是公开读，按未登录处理即可。
	w = doJSON(r, "GET", "/api/bootstrap", "not-a-token", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap with a bad token must still render, got %d", w.Code)
	}
	if decode(t, w)["data"].(map[string]interface{})["auth"].(map[string]interface{})["authenticated"] != false {
		t.Fatalf("bad token must not authenticate")
	}
}

// fnOS 部署下登录页需要服务端口与跳板页入口才能发起授权，二者必须随引导
// 一起下发，否则手机端「打开就点」还会再等一次版本查询。
func TestFnOSBootstrapCarriesGatewayEntry(t *testing.T) {
	r, _ := setupFnOSTestRouter(t)
	w := doJSON(r, "GET", "/app/techfunway-reminders/api/bootstrap", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("fnos bootstrap status %d", w.Code)
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["service_port"] != "0" {
		t.Fatalf("service_port = %v, want the configured port", data["service_port"])
	}
	if data["fnos_app"] != true {
		t.Fatalf("fnos_app = %v, want true in fnOS mode", data["fnos_app"])
	}
	if _, present := data["fnos_gateway_entry"]; present {
		t.Fatalf("entry must stay absent until the trampoline registers it")
	}
}

func TestFirstUserBecomesAdminAndLogin(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")

	w := doJSON(r, "POST", "/api/auth/login", "", map[string]string{"username": "admin", "password": "secret123"})
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d", w.Code)
	}
	user := decode(t, w)["data"].(map[string]interface{})["user"].(map[string]interface{})
	if user["role"] != "admin" {
		t.Fatalf("first user role = %v, want admin", user["role"])
	}

	// Wrong password is rejected.
	w = doJSON(r, "POST", "/api/auth/login", "", map[string]string{"username": "admin", "password": "nope"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want 401", w.Code)
	}
}

func TestFnOSOneClickLoginAndBinding(t *testing.T) {
	r, db := setupFnOSTestRouter(t)
	const base = "/app/techfunway-reminders/api/auth/fnos"

	w := doFnOSJSON(r, http.MethodGet, base+"/identity", "1000", "nas-admin", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("fnos identity status %d body %s", w.Code, w.Body.String())
	}
	identity := decode(t, w)["data"].(map[string]interface{})
	if identity["fnos_username"] != "nas-admin" {
		t.Fatalf("unexpected fnos identity: %#v", identity)
	}

	w = doFnOSJSON(r, http.MethodPost, base+"/login", "1000", "nas-admin", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unbound fnos login status %d body %s", w.Code, w.Body.String())
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["binding_required"] != true || data["fnos_username"] != "nas-admin" || data["has_accounts"] != false || data["suggested_mode"] != "register" || data["suggested_username"] != "" {
		t.Fatalf("unexpected unbound fnos result: %#v", data)
	}

	w = doFnOSJSON(r, http.MethodPost, base+"/bind", "1000", "nas-admin", map[string]string{
		"mode": "register", "username": "reminder-admin", "password": "secret123",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("fnos register/bind status %d body %s", w.Code, w.Body.String())
	}
	bound := decode(t, w)["data"].(map[string]interface{})
	if bound["token"] == "" || bound["user"].(map[string]interface{})["role"] != "admin" {
		t.Fatalf("unexpected bound result: %#v", bound)
	}
	var account database.User
	if err := db.Where("username = ?", "reminder-admin").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.FnOSUserID == nil || *account.FnOSUserID != 1000 || account.FnOSUsername != "nas-admin" {
		t.Fatalf("fnos binding not persisted: %#v", account)
	}

	w = doFnOSJSON(r, http.MethodPost, base+"/login", "1000", "renamed-nas-admin", nil)
	if w.Code != http.StatusOK || decode(t, w)["data"].(map[string]interface{})["token"] == "" {
		t.Fatalf("bound fnos login failed: status %d body %s", w.Code, w.Body.String())
	}
	if err := db.First(&account, account.ID).Error; err != nil || account.FnOSUsername != "renamed-nas-admin" {
		t.Fatalf("fnos username refresh failed: user=%#v err=%v", account, err)
	}

	// If the NAS username already exists as an application account, guide the
	// user to bind that account instead of accidentally creating a second data
	// silo with empty reminders and channel bindings.
	w = doJSON(r, http.MethodPost, "/app/techfunway-reminders/api/auth/register", "", map[string]string{"username": "same-name", "password": "secret123"})
	if w.Code != http.StatusOK {
		t.Fatalf("create matching account: status %d body %s", w.Code, w.Body.String())
	}
	w = doFnOSJSON(r, http.MethodPost, base+"/login", "3000", "same-name", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("matching fnos login status %d body %s", w.Code, w.Body.String())
	}
	data = decode(t, w)["data"].(map[string]interface{})
	if data["binding_required"] != true || data["has_accounts"] != true || data["suggested_mode"] != "bind" || data["suggested_username"] != "same-name" {
		t.Fatalf("matching account should suggest bind: %#v", data)
	}

	// Once any application account exists, an unbound NAS user must be sent to
	// existing-account login even when the NAS and application usernames differ.
	w = doFnOSJSON(r, http.MethodPost, base+"/login", "4000", "different-nas-name", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("different-name fnos login status %d body %s", w.Code, w.Body.String())
	}
	data = decode(t, w)["data"].(map[string]interface{})
	if data["binding_required"] != true || data["has_accounts"] != true || data["suggested_mode"] != "bind" || data["suggested_username"] != "" {
		t.Fatalf("existing accounts should require login and bind: %#v", data)
	}

	w = doJSON(r, http.MethodPost, "/app/techfunway-reminders/api/auth/register", "", map[string]string{"username": "existing-user", "password": "secret123"})
	if w.Code != http.StatusOK {
		t.Fatalf("create existing account: status %d body %s", w.Code, w.Body.String())
	}
	w = doFnOSJSON(r, http.MethodPost, base+"/bind", "2000", "nas-user", map[string]string{
		"mode": "bind", "username": "existing-user", "password": "secret123",
	})
	if w.Code != http.StatusOK || decode(t, w)["data"].(map[string]interface{})["token"] == "" {
		t.Fatalf("existing account fnos bind failed: status %d body %s", w.Code, w.Body.String())
	}
}

func TestFnOSAuthIsUnavailableOutsideGatewayMode(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	if got := doFnOSJSON(r, http.MethodGet, "/api/auth/fnos/identity", "1000", "nas-admin", nil).Code; got != http.StatusNotFound {
		t.Fatalf("non-fnos deployment exposed fnos identity route: status %d", got)
	}
	if got := doFnOSJSON(r, http.MethodPost, "/api/auth/fnos/login", "1000", "nas-admin", nil).Code; got != http.StatusNotFound {
		t.Fatalf("non-fnos deployment exposed fnos login route: status %d", got)
	}
}

func TestFnOSBindingRequiresGatewayIdentity(t *testing.T) {
	r, _ := setupFnOSTestRouter(t)
	const loginPath = "/app/techfunway-reminders/api/auth/fnos/login"
	if got := doFnOSJSON(r, http.MethodPost, "/app/techfunway-reminders/api/auth/fnos/bind", "", "", map[string]string{
		"mode": "register", "username": "admin", "password": "secret123",
	}).Code; got != http.StatusUnauthorized {
		t.Fatalf("missing gateway identity status %d, want 401", got)
	}

	req := httptest.NewRequest(http.MethodPost, loginPath, nil)
	req.Header.Set("X-Trim-Userid", "1000")
	req.Header.Set("X-Trim-Username", "nas-admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("untrusted fnos headers status %d, want 401", w.Code)
	}
}

// 飞牛授权只是身份来源，应用自己的登录态必须能独立退出：网关域上点「退出登录」
// 之后，网关注入的 X-Trim-* 不得再把登录态自动认回来（否则退出立即被撤销，
// 表现就是「飞牛登录的应用退不出去」）。重新显式登录后再恢复隐式维持。
func TestFnOSGatewayLogoutIsNotUndoneByGatewayIdentity(t *testing.T) {
	r, db := setupFnOSTestRouter(t)
	const base = "/app/techfunway-reminders/api"

	// 建号并绑定 NAS 用户 1000。
	if w := doFnOSJSON(r, http.MethodPost, base+"/auth/fnos/bind", "1000", "nas-admin", map[string]string{
		"mode": "register", "username": "reminder-admin", "password": "secret123",
	}); w.Code != http.StatusOK {
		t.Fatalf("bind status %d body %s", w.Code, w.Body.String())
	}
	var account database.User
	if err := db.Where("username = ?", "reminder-admin").First(&account).Error; err != nil {
		t.Fatal(err)
	}

	// 退出前：网关身份隐式认人，bootstrap 报告的是网关来源的会话。
	w := doFnOSJSON(r, http.MethodGet, base+"/bootstrap", "1000", "nas-admin", nil)
	if got := decode(t, w)["data"].(map[string]interface{})["session_source"]; got != "gateway" {
		t.Fatalf("before logout session_source = %v, want gateway", got)
	}

	// 退出登录：网关域上必须落持久化抑制标记，而不是只清前端。
	if w := doFnOSJSON(r, http.MethodPost, base+"/auth/logout", "1000", "nas-admin", nil); w.Code != http.StatusOK {
		t.Fatalf("logout status %d body %s", w.Code, w.Body.String())
	}
	var session database.UserSession
	if err := db.Where("user_id = ?", account.ID).First(&session).Error; err != nil || !session.Suppressed {
		t.Fatalf("logout did not persist suppression: session=%#v err=%v", session, err)
	}

	// 退出后：同一个网关身份不再换来登录态。
	w = doFnOSJSON(r, http.MethodGet, base+"/bootstrap", "1000", "nas-admin", nil)
	if auth := decode(t, w)["data"].(map[string]interface{})["auth"].(map[string]interface{}); auth["authenticated"] == true {
		t.Fatalf("gateway identity resurrected the session after logout: %#v", auth)
	}
	if w := doFnOSJSON(r, http.MethodGet, base+"/auth/me", "1000", "nas-admin", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("authed route still reachable after logout: status %d body %s", w.Code, w.Body.String())
	}

	// 退出是幂等的：没有会话时重复调用也要成功。
	if w := doJSON(r, http.MethodPost, base+"/auth/logout", "", nil); w.Code != http.StatusOK {
		t.Fatalf("idempotent logout status %d body %s", w.Code, w.Body.String())
	}

	// 显式重新登录（登录页的「使用飞牛 NAS 登录」）后抑制解除，刷新不再掉线。
	if w := doFnOSJSON(r, http.MethodPost, base+"/auth/fnos/login", "1000", "nas-admin", nil); w.Code != http.StatusOK {
		t.Fatalf("re-login status %d body %s", w.Code, w.Body.String())
	}
	if err := db.Where("user_id = ?", account.ID).First(&session).Error; err != nil || session.Suppressed {
		t.Fatalf("explicit login did not clear suppression: session=%#v err=%v", session, err)
	}
	w = doFnOSJSON(r, http.MethodGet, base+"/bootstrap", "1000", "nas-admin", nil)
	if auth := decode(t, w)["data"].(map[string]interface{})["auth"].(map[string]interface{}); auth["authenticated"] != true {
		t.Fatalf("gateway session not restored after explicit login: %#v", auth)
	}
}

// 直连端口上的退出只结束应用自己的会话：没有网关身份，也不该写坏别的用户。
func TestLogoutOnDirectPortIsHarmless(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	const base = "/api"

	if w := doJSON(r, http.MethodPost, base+"/auth/register", "", map[string]string{"username": "admin", "password": "secret123"}); w.Code != http.StatusOK {
		t.Fatalf("register status %d body %s", w.Code, w.Body.String())
	}
	w := doJSON(r, http.MethodPost, base+"/auth/login", "", map[string]string{"username": "admin", "password": "secret123"})
	token := decode(t, w)["data"].(map[string]interface{})["token"].(string)

	if w := doJSON(r, http.MethodPost, base+"/auth/logout", token, nil); w.Code != http.StatusOK {
		t.Fatalf("direct-port logout status %d body %s", w.Code, w.Body.String())
	}
	var count int64
	if err := db.Model(&database.UserSession{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one session row for the logged-out user, got %d", count)
	}
}

func TestFnOSListensOnPortAndGatewaySocket(t *testing.T) {
	socketFile, err := os.CreateTemp("/tmp", "rem-fnos-")
	if err != nil {
		t.Fatal(err)
	}
	socket := socketFile.Name()
	if err := socketFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}
	listeners, err := listen(config.Config{
		Port: 0, FnOSApp: true, GatewaySocket: socket, GatewayPrefix: "/app/techfunway-reminders",
	})
	if err != nil {
		t.Fatalf("listen fnos: %v", err)
	}
	defer os.Remove(socket)
	defer func() {
		for _, listener := range listeners {
			listener.Listener.Close()
		}
	}()
	if len(listeners) != 2 || listeners[0].Listener.Addr().Network() != "tcp" || !listeners[1].IsFnOSGateway || listeners[1].Listener.Addr().Network() != "unix" {
		t.Fatalf("unexpected fnos listeners: %#v", listeners)
	}
}

func TestFnOSDirectPortRedirectsToGatewayPrefix(t *testing.T) {
	r, _ := setupFnOSTestRouter(t)
	handler := newDirectHandler(r, config.Config{FnOSApp: true, GatewayPrefix: "/app/techfunway-reminders"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTemporaryRedirect || w.Header().Get("Location") != "/app/techfunway-reminders/" {
		t.Fatalf("direct fnos entry = status %d location %q", w.Code, w.Header().Get("Location"))
	}
}

// The trampoline's protocol evolves with releases; a cached old copy speaks the
// old protocol and deadlocks the login flow (the v0.6.2 incident).
func TestFnOSEntryTrampolineNotCached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fnos-entry.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}
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
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || secret == "" {
		t.Fatalf("jwt secret: %v", err)
	}
	cfg := config.Config{CORSOrigin: "*", FnOSApp: true, GatewayPrefix: "/app/techfunway-reminders", WebDir: dir}
	r := NewRouter(cfg, db, secret)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app/techfunway-reminders/fnos-entry.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("trampoline status %d body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("trampoline Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(w.Body.String(), "<html>") {
		t.Fatalf("trampoline body unexpected: %q", w.Body.String())
	}
}

func TestShortPasswordRejected(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	w := doJSON(r, "POST", "/api/auth/register", "", map[string]string{"username": "bob", "password": "123"})
	if w.Code == http.StatusOK {
		t.Fatalf("short password accepted")
	}
}

func TestJWTSecretNeverLeaked(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	token := registerUser(t, r, "admin", "secret123")

	for _, path := range []string{"/api/configs", "/api/configs/system", "/api/configs/public", "/api/configs/meta", "/api/configs/user/meta"} {
		w := doJSON(r, "GET", path, token, nil)
		if strings.Contains(w.Body.String(), "jwt_secret") {
			t.Fatalf("%s leaked jwt_secret: %s", path, w.Body.String())
		}
	}

	// jwt_secret must not be writable via the config API.
	w := doJSON(r, "PUT", "/api/configs", token, map[string]string{"key": "jwt_secret", "value": "hacked"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("updating jwt_secret: status %d, want 403", w.Code)
	}
}

func TestConfigRequiresAuth(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	w := doJSON(r, "GET", "/api/configs", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous /configs status = %d, want 401", w.Code)
	}
}

func TestAdminEndpointsForbiddenForNormalUser(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123") // first user = admin
	userTok := registerUser(t, r, "bob", "secret123")

	w := doJSON(r, "GET", "/api/users", userTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("normal user GET /users = %d, want 403", w.Code)
	}
}

func TestCredentialsReflectCurrentUserState(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	adminToken := registerUser(t, r, "admin", "secret123")
	bobToken := registerUser(t, r, "bob", "secret123")

	me := doJSON(r, http.MethodGet, "/api/auth/me", bobToken, nil)
	apiKey := decode(t, me)["data"].(map[string]interface{})["api_key"].(string)
	if err := db.Model(&database.User{}).Where("username = ?", "bob").Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	if got := doJSON(r, http.MethodGet, "/api/auth/me", bobToken, nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("disabled user's JWT returned %d, want 401", got)
	}
	if got := doAPIKey(r, http.MethodGet, "/api/auth/me", apiKey).Code; got != http.StatusUnauthorized {
		t.Fatalf("disabled user's API key returned %d, want 401", got)
	}

	if got := doJSON(r, http.MethodGet, "/api/users", adminToken, nil).Code; got != http.StatusOK {
		t.Fatalf("active admin token returned %d", got)
	}
}

func TestJWTUsesCurrentRoleAndPasswordVersion(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	adminToken := registerUser(t, r, "admin", "secret123")
	registerUser(t, r, "bob", "secret123")
	role := "admin"
	if got := doJSON(r, http.MethodPut, "/api/users/2", adminToken, map[string]interface{}{"role": role}).Code; got != http.StatusOK {
		t.Fatalf("promote bob returned %d", got)
	}
	bobToken := loginToken(t, r, "bob", "secret123")
	role = "user"
	if got := doJSON(r, http.MethodPut, "/api/users/2", adminToken, map[string]interface{}{"role": role}).Code; got != http.StatusOK {
		t.Fatalf("demote bob returned %d", got)
	}
	if got := doJSON(r, http.MethodGet, "/api/users", bobToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("demoted user's old JWT returned %d, want 403", got)
	}

	freshToken := loginToken(t, r, "bob", "secret123")
	changed := doJSON(r, http.MethodPut, "/api/auth/password", freshToken, map[string]string{
		"old_password": "secret123", "new_password": "newsecret123",
	})
	if changed.Code != http.StatusOK {
		t.Fatalf("change password returned %d: %s", changed.Code, changed.Body.String())
	}
	if got := doJSON(r, http.MethodGet, "/api/auth/me", freshToken, nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("pre-change JWT returned %d, want 401", got)
	}
	loginToken(t, r, "bob", "newsecret123")
}

func TestLegacyBrowserPasswordMigratesOnLogin(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	sum := sha256.Sum256([]byte("secret123"))
	legacyBrowserPassword := hex.EncodeToString(sum[:])
	registerUser(t, r, "admin", legacyBrowserPassword)

	loginToken(t, r, "admin", "secret123")
	var admin database.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(admin.Password, "secret123") {
		t.Fatal("legacy browser-side SHA-256 password was not migrated")
	}
}

func TestUserListPaginated(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	token := registerUser(t, r, "admin", "secret123")

	w := doJSON(r, "GET", "/api/users?page=1&pageSize=10", token, nil)
	data := decode(t, w)["data"].(map[string]interface{})
	for _, key := range []string{"items", "total", "page", "pageSize"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("list response missing %q: %v", key, data)
		}
	}
}

func TestUpdateUserMassAssignmentGuard(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")
	registerUser(t, r, "bob", "secret123") // id 2

	token := loginToken(t, r, "admin", "secret123")
	// Attempt to overwrite username + password alongside a legit status change.
	doJSON(r, "PUT", "/api/users/2", token, map[string]interface{}{
		"username": "hacker", "password": "x", "status": 0,
	})

	var bob database.User
	if err := db.First(&bob, 2).Error; err != nil {
		t.Fatal(err)
	}
	if bob.Username != "bob" {
		t.Fatalf("username was mass-assigned to %q", bob.Username)
	}
	if bob.Status != 0 {
		t.Fatalf("legit status change not applied, status = %d", bob.Status)
	}
}

func TestCannotRemoveLastAdmin(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123") // id 1, only admin
	token := loginToken(t, r, "admin", "secret123")

	// Demote the only admin -> rejected.
	role := "user"
	w := doJSON(r, "PUT", "/api/users/1", token, map[string]interface{}{"role": role})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("demote last admin status = %d, want 400", w.Code)
	}

	// Disable the only admin via toggle -> rejected.
	w = doJSON(r, "PUT", "/api/users/1/status", token, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("disable last admin status = %d, want 400", w.Code)
	}
}

func TestAuditLogRecordsLogin(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")
	token := loginToken(t, r, "admin", "secret123")

	w := doJSON(r, "GET", "/api/audit-logs", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("audit list status %d", w.Code)
	}
	items := decode(t, w)["data"].(map[string]interface{})["items"].([]interface{})
	found := false
	for _, it := range items {
		if it.(map[string]interface{})["action"] == "login" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no login entry in audit log: %s", w.Body.String())
	}
}

// TestRegisterErrorCodeContract locks in the documented error-code mapping
// documented in AGENTS.md: user-exists → HTTP 409 code 1001, register
// disabled → HTTP 403 code 1003. A previous version regressed this contract
// by string-matching English error messages that lived in a different
// package than the (Chinese) sentinels — this test fails the moment that
// matching drift returns.
func TestRegisterErrorCodeContract(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")

	dup := doJSON(r, "POST", "/api/auth/register", "", map[string]string{"username": "admin", "password": "secret123"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate register: HTTP %d, want 409; body=%s", dup.Code, dup.Body.String())
	}
	if got := int(decode(t, dup)["code"].(float64)); got != 1001 {
		t.Fatalf("duplicate register: code %d, want 1001", got)
	}

	adminToken := loginToken(t, r, "admin", "secret123")
	disable := doJSON(r, "PUT", "/api/configs", adminToken, map[string]string{"key": "allow_register", "value": "false"})
	if disable.Code != http.StatusOK {
		t.Fatalf("disable allow_register: HTTP %d, body=%s", disable.Code, disable.Body.String())
	}

	blocked := doJSON(r, "POST", "/api/auth/register", "", map[string]string{"username": "newbie", "password": "secret123"})
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("register while disabled: HTTP %d, want 403; body=%s", blocked.Code, blocked.Body.String())
	}
	if got := int(decode(t, blocked)["code"].(float64)); got != 1003 {
		t.Fatalf("register while disabled: code %d, want 1003", got)
	}
}

// TestLoginGenericErrorDoesNotEnumerateUsers verifies the user-enumeration
// fix: disabled accounts and non-existent accounts return the same error
// message so an attacker can't distinguish them.
func TestLoginGenericErrorDoesNotEnumerateUsers(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")

	disabledToken := loginToken(t, r, "admin", "secret123")
	// Flip admin to status=0 directly so we don't trip the last-admin guard.
	if err := db.Model(&database.User{}).Where("id = ?", 1).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	_ = disabledToken

	disabledResp := doJSON(r, "POST", "/api/auth/login", "", map[string]string{"username": "admin", "password": "secret123"})
	missingResp := doJSON(r, "POST", "/api/auth/login", "", map[string]string{"username": "ghost", "password": "secret123"})

	if disabledResp.Code != http.StatusUnauthorized || missingResp.Code != http.StatusUnauthorized {
		t.Fatalf("both should return 401: disabled=%d missing=%d", disabledResp.Code, missingResp.Code)
	}
	disabledMsg := decode(t, disabledResp)["message"]
	missingMsg := decode(t, missingResp)["message"]
	if disabledMsg != missingMsg {
		t.Fatalf("user enumeration leak: disabled=%q missing=%q (must match)", disabledMsg, missingMsg)
	}
}

// TestResetPasswordEnforcesMinLength locks the security fix that admin
// password resets can no longer downgrade a user to a 1-char brute-forceable
// password.
func TestResetPasswordEnforcesMinLength(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")
	registerUser(t, r, "bob", "secret123")
	adminToken := loginToken(t, r, "admin", "secret123")

	short := doJSON(r, "PUT", "/api/users/2/password", adminToken, map[string]string{"new_password": "a"})
	if short.Code != http.StatusBadRequest {
		t.Fatalf("short reset: HTTP %d, want 400; body=%s", short.Code, short.Body.String())
	}

	long := doJSON(r, "PUT", "/api/users/2/password", adminToken, map[string]string{"new_password": "longpassword"})
	if long.Code != http.StatusOK {
		t.Fatalf("valid reset: HTTP %d, want 200; body=%s", long.Code, long.Body.String())
	}
}

func TestForgotPasswordEnforcesMinLength(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	token := registerUser(t, r, "admin", "secret123")
	questions := map[string]string{
		"question1": "q1", "answer1": "answer1",
		"question2": "q2", "answer2": "answer2",
		"question3": "q3", "answer3": "answer3",
	}
	if got := doJSON(r, http.MethodPost, "/api/security/questions", token, questions).Code; got != http.StatusOK {
		t.Fatalf("set security questions returned %d", got)
	}
	reset := map[string]string{
		"username": "admin", "answer1": "answer1", "answer2": "answer2", "answer3": "answer3", "new_password": "a",
	}
	if got := doJSON(r, http.MethodPost, "/api/security/forgot/reset", "", reset).Code; got != http.StatusBadRequest {
		t.Fatalf("short forgot-password reset returned %d, want 400", got)
	}
}

// TestUserListDoesNotLeakAPIKeys locks the bearer-token suppression fix:
// GetAllUsers no longer returns api_key for every user — the owner reads
// their own key via /auth/me.
func TestUserListDoesNotLeakAPIKeys(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")
	token := loginToken(t, r, "admin", "secret123")

	w := doJSON(r, "GET", "/api/users", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("user list: HTTP %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "api_key") {
		t.Fatalf("user list response leaks api_key: %s", w.Body.String())
	}
}

func TestUploadAndPathTraversal(t *testing.T) {
	r, _, dir := setupTestRouter(t)
	token := registerUser(t, r, "admin", "secret123")

	if got := doUpload(r, "", "image.png", []byte("not authenticated")).Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous upload returned %d, want 401", got)
	}
	if got := doUpload(r, token, "payload.html", []byte("<script>alert(1)</script>")).Code; got != http.StatusBadRequest {
		t.Fatalf("active-content upload returned %d, want 400", got)
	}
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if got := doUpload(r, token, "safe.png", pngHeader).Code; got != http.StatusOK {
		t.Fatalf("valid PNG upload returned %d, want 200", got)
	}

	// Plant a secret file outside the upload directory.
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0644); err != nil {
		t.Fatal(err)
	}

	// Traversal attempts must not return the secret.
	for _, p := range []string{
		"/uploads/../secret.txt",
		"/uploads/..%2f..%2fsecret.txt",
		"/uploads/....//secret.txt",
	} {
		w := doJSON(r, "GET", p, "", nil)
		if strings.Contains(w.Body.String(), "TOPSECRET") {
			t.Fatalf("path traversal via %q leaked secret", p)
		}
	}
}

// TestAppRoutesUsePublicRateLimiter locks the limiter onto an app's public
// routes: with RateLimit=1 the second call must be rejected.
func TestAppRoutesUsePublicRateLimiter(t *testing.T) {
	_, db, dir := setupTestRouter(t)
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(config.Config{
		CORSOrigin: "*",
		RateLimit:  1,
		UploadDir:  filepath.Join(dir, "uploads"),
	}, db, secret)
	if got := doJSON(r, http.MethodGet, "/api/qrcode?content=hello", "", nil).Code; got != http.StatusOK {
		t.Fatalf("first QR request returned %d", got)
	}
	if got := doJSON(r, http.MethodGet, "/api/qrcode?content=hello", "", nil).Code; got != http.StatusTooManyRequests {
		t.Fatalf("second QR request returned %d, want 429", got)
	}
}

// TestBootstrapReadsBypassRateLimiter protects the SPA startup reads. They are
// called on every navigation and must never 429: a rejected setup-required call
// leaves setupRequired=false, which sends a first-time install to the login
// page instead of admin creation, with no visible error. With RateLimit=1 any
// counted call would be rejected on the second attempt.
func TestBootstrapReadsBypassRateLimiter(t *testing.T) {
	_, db, _ := setupTestRouter(t)
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(config.Config{CORSOrigin: "*", RateLimit: 1}, db, secret)
	for i := 0; i < 5; i++ {
		if got := doJSON(r, http.MethodGet, "/api/configs/public", "", nil).Code; got != http.StatusOK {
			t.Fatalf("configs/public call %d returned %d, want 200", i+1, got)
		}
		if got := doJSON(r, http.MethodGet, "/api/auth/setup-required", "", nil).Code; got != http.StatusOK {
			t.Fatalf("setup-required call %d returned %d, want 200", i+1, got)
		}
		// 首屏引导请求同样不能被限流：它是「应用是否可用」的唯一依据，被 429
		// 后前端只能按默认值渲染，首次安装会被错误地带到登录页。
		if got := doJSON(r, http.MethodGet, "/api/bootstrap", "", nil).Code; got != http.StatusOK {
			t.Fatalf("bootstrap call %d returned %d, want 200", i+1, got)
		}
	}
}

// TestFnOSGatewayTrafficIsNotRateLimitedPerIP covers the mobile symptom where
// every NAS user shared one bucket: gateway requests arrive over the local unix
// socket, so ClientIP() is empty for all of them. With the limiter enabled and
// a fresh budget spent by one caller, the next gateway request must still pass.
func TestFnOSGatewayTrafficIsNotSharedAcrossIP(t *testing.T) {
	_, db := setupFnOSTestRouter(t)
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(config.Config{
		CORSOrigin:    "*",
		RateLimit:     1,
		FnOSApp:       true,
		GatewayPrefix: "/app/techfunway-reminders",
	}, db, secret)

	// One NAS user spends the whole allowance on a limited public endpoint
	// (the QR app route is public and is not a bootstrap read).
	if got := doFnOSJSON(r, http.MethodGet, "/app/techfunway-reminders/api/qrcode?content=a", "1000", "nas-a", nil).Code; got != http.StatusOK {
		t.Fatalf("first gateway request returned %d", got)
	}
	if got := doFnOSJSON(r, http.MethodGet, "/app/techfunway-reminders/api/qrcode?content=a", "1000", "nas-a", nil).Code; got != http.StatusTooManyRequests {
		t.Fatalf("second request from same NAS user returned %d, want 429", got)
	}
	// A different NAS user on the same box has its own bucket and must not be
	// throttled by the first user's activity.
	if got := doFnOSJSON(r, http.MethodGet, "/app/techfunway-reminders/api/qrcode?content=b", "2000", "nas-b", nil).Code; got != http.StatusOK {
		t.Fatalf("another NAS user was throttled: got %d, want 200", got)
	}
}

// TestSystemConfigsEndpointRequiresAdmin moves /api/configs/system behind the
// admin gate: anonymous callers get 401, regular users 403, admins 200.
func TestSystemConfigsEndpointRequiresAdmin(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	adminTok := registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123")

	w := doJSON(r, "GET", "/api/configs/system", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anon GET /configs/system = %d, want 401", w.Code)
	}
	w = doJSON(r, "GET", "/api/configs/system", userTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin GET /configs/system = %d, want 403", w.Code)
	}
	w = doJSON(r, "GET", "/api/configs/system", adminTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET /configs/system = %d, want 200", w.Code)
	}
}

// TestSystemConfigMeta locks the admin metadata endpoint: access control,
// payload shape (type/label/description), DB-backed values, and exclusion of
// internal keys.
func TestSystemConfigMeta(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	adminTok := registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123")

	w := doJSON(r, "GET", "/api/configs/meta", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anon GET /configs/meta = %d, want 401", w.Code)
	}
	w = doJSON(r, "GET", "/api/configs/meta", userTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin GET /configs/meta = %d, want 403", w.Code)
	}

	w = doJSON(r, "GET", "/api/configs/meta", adminTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET /configs/meta = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	items := decode(t, w)["data"].([]interface{})
	var allowReg map[string]interface{}
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["key"] == "jwt_secret" {
			t.Fatalf("meta leaks jwt_secret: %s", w.Body.String())
		}
		if m["key"] == "allow_register" {
			allowReg = m
		}
	}
	if allowReg == nil {
		t.Fatalf("meta missing allow_register: %s", w.Body.String())
	}
	if allowReg["scope"] != "system" || allowReg["type"] != "bool" {
		t.Fatalf("allow_register meta wrong: %v", allowReg)
	}
	if allowReg["value"] != "true" || allowReg["default"] != "true" {
		t.Fatalf("allow_register value/default wrong: %v", allowReg)
	}
	if allowReg["label"] == "" || allowReg["description"] == "" || allowReg["group"] != "access" {
		t.Fatalf("allow_register missing label/description/group: %v", allowReg)
	}
}

// TestUserConfigMeta covers the per-user metadata endpoint: any authenticated
// user sees user-scope configs with the registered default as the effective
// value until they override it.
func TestUserConfigMeta(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	token := registerUser(t, r, "admin", "secret123")

	w := doJSON(r, "GET", "/api/configs/user/meta", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anon GET /configs/user/meta = %d, want 401", w.Code)
	}

	w = doJSON(r, "GET", "/api/configs/user/meta", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /configs/user/meta = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	items := decode(t, w)["data"].([]interface{})
	var theme map[string]interface{}
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["key"] == "jwt_secret" {
			t.Fatalf("user meta leaks jwt_secret: %s", w.Body.String())
		}
		if m["key"] == "theme_mode" {
			theme = m
		}
	}
	if theme == nil {
		t.Fatalf("user meta missing theme_mode: %s", w.Body.String())
	}
	if theme["scope"] != "user" || theme["type"] != "select" {
		t.Fatalf("theme_mode meta wrong: %v", theme)
	}
	if theme["value"] != "system" || theme["default"] != "system" {
		t.Fatalf("theme_mode default resolution wrong: %v", theme)
	}
	opts, ok := theme["options"].([]interface{})
	if !ok || len(opts) != 3 {
		t.Fatalf("theme_mode options wrong: %v", theme)
	}
}

// TestConfigListScopesForNonAdmin verifies the merged config list no longer
// leaks non-public system configs to regular users: they see public system
// keys plus their own per-user keys, while admins still see all system keys.
func TestConfigListScopesForNonAdmin(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	adminTok := registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123") // id 2

	// Given: a non-public system config and a per-user config owned by bob.
	if err := db.Create(&database.SystemConfig{UserID: 0, Key: "feature_flag", Value: "on", Public: false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.SystemConfig{UserID: 2, Key: "my_pref", Value: "compact"}).Error; err != nil {
		t.Fatal(err)
	}

	// When/Then: bob gets public system keys + his own keys, not feature_flag.
	w := doJSON(r, "GET", "/api/configs", userTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /configs = %d", w.Code)
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if _, ok := data["site_title"]; !ok {
		t.Fatalf("non-admin list missing public key site_title: %v", data)
	}
	if _, ok := data["feature_flag"]; ok {
		t.Fatalf("non-admin list leaked non-public system key: %v", data)
	}
	if data["my_pref"] != "compact" {
		t.Fatalf("non-admin list missing own user key: %v", data)
	}

	// When/Then: the admin still sees the non-public system key.
	w = doJSON(r, "GET", "/api/configs", adminTok, nil)
	adminData := decode(t, w)["data"].(map[string]interface{})
	if adminData["feature_flag"] != "on" {
		t.Fatalf("admin list missing non-public system key: %v", adminData)
	}
}

// TestUpdateConfigScopeEnforcement covers the registry-driven write path:
// system keys are admin-only and type-validated, user keys are type-validated
// and stored per-user.
func TestUpdateConfigScopeEnforcement(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	adminTok := registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123") // id 2

	// Non-admin writing a system key -> 403.
	w := doJSON(r, "PUT", "/api/configs", userTok, map[string]string{"key": "allow_register", "value": "false"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin system write = %d, want 403", w.Code)
	}

	// Admin writing an invalid bool -> 400.
	w = doJSON(r, "PUT", "/api/configs", adminTok, map[string]string{"key": "allow_register", "value": "maybe"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bool write = %d, want 400; body=%s", w.Code, w.Body.String())
	}

	// Admin writing a valid bool -> 200, persisted at user_id=0.
	w = doJSON(r, "PUT", "/api/configs", adminTok, map[string]string{"key": "allow_register", "value": "false"})
	if w.Code != http.StatusOK {
		t.Fatalf("admin bool write = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var row database.SystemConfig
	if err := db.Where("user_id = 0 AND key = ?", "allow_register").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "false" {
		t.Fatalf("allow_register = %q, want false", row.Value)
	}

	// User-scope key with an option outside the select list -> 400.
	w = doJSON(r, "PUT", "/api/configs", userTok, map[string]string{"key": "theme_mode", "value": "neon"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid select write = %d, want 400; body=%s", w.Code, w.Body.String())
	}

	// User-scope key with a valid option -> 200, stored under the caller.
	w = doJSON(r, "PUT", "/api/configs", userTok, map[string]string{"key": "theme_mode", "value": "dark"})
	if w.Code != http.StatusOK {
		t.Fatalf("user config write = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var urow database.SystemConfig
	if err := db.Where("user_id = 2 AND key = ?", "theme_mode").First(&urow).Error; err != nil {
		t.Fatalf("theme_mode not stored per-user: %v", err)
	}
	if urow.Value != "dark" {
		t.Fatalf("theme_mode = %q, want dark", urow.Value)
	}
}

// TestUserConfigIsolation proves two users' overrides of the same user-scope
// key are independent.
func TestUserConfigIsolation(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	adminTok := registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123")

	if w := doJSON(r, "PUT", "/api/configs", adminTok, map[string]string{"key": "theme_mode", "value": "dark"}); w.Code != http.StatusOK {
		t.Fatalf("admin theme write = %d", w.Code)
	}
	if w := doJSON(r, "PUT", "/api/configs", userTok, map[string]string{"key": "theme_mode", "value": "light"}); w.Code != http.StatusOK {
		t.Fatalf("bob theme write = %d", w.Code)
	}

	readTheme := func(token string) interface{} {
		w := doJSON(r, "GET", "/api/configs/user/meta", token, nil)
		for _, it := range decode(t, w)["data"].([]interface{}) {
			m := it.(map[string]interface{})
			if m["key"] == "theme_mode" {
				return m["value"]
			}
		}
		return nil
	}
	if v := readTheme(adminTok); v != "dark" {
		t.Fatalf("admin theme_mode = %v, want dark", v)
	}
	if v := readTheme(userTok); v != "light" {
		t.Fatalf("bob theme_mode = %v, want light", v)
	}
}

// TestAdHocConfigWritesCallerScope locks the legacy behavior for unregistered
// keys: they are stored against the caller's user_id and show up in the
// caller's merged config list.
func TestAdHocConfigWritesCallerScope(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	registerUser(t, r, "admin", "secret123")
	userTok := registerUser(t, r, "bob", "secret123") // id 2

	w := doJSON(r, "PUT", "/api/configs", userTok, map[string]string{"key": "custom_note", "value": "hello"})
	if w.Code != http.StatusOK {
		t.Fatalf("ad-hoc write = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var row database.SystemConfig
	if err := db.Where("user_id = 2 AND key = ?", "custom_note").First(&row).Error; err != nil {
		t.Fatalf("ad-hoc key not stored per-user: %v", err)
	}
	if row.Value != "hello" {
		t.Fatalf("custom_note = %q, want hello", row.Value)
	}

	w = doJSON(r, "GET", "/api/configs", userTok, nil)
	if v := decode(t, w)["data"].(map[string]interface{})["custom_note"]; v != "hello" {
		t.Fatalf("ad-hoc key missing from merged list: %v", v)
	}
}

func loginToken(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	w := doJSON(r, "POST", "/api/auth/login", "", map[string]string{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: status %d", username, w.Code)
	}
	return decode(t, w)["data"].(map[string]interface{})["token"].(string)
}
