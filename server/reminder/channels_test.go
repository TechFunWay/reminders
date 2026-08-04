package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMTPErrorMessageExplainsQQAuthorizationCode(t *testing.T) {
	got := smtpErrorMessage("smtp.qq.com", errors.New(`535 "Login fail. Account is abnormal, service is not open, password is incorrect"`))

	for _, want := range []string{"QQ 邮箱 SMTP 登录失败", "开启 POP3/SMTP 或 IMAP/SMTP 服务", "新生成的授权码", "不是 QQ/邮箱登录密码"} {
		if !strings.Contains(got, want) {
			t.Fatalf("smtpErrorMessage() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "Account is abnormal") {
		t.Fatalf("smtpErrorMessage() exposed the provider's ambiguous raw error: %q", got)
	}
}

func TestSMTPErrorMessageKeepsNonAuthenticationFailure(t *testing.T) {
	got := smtpErrorMessage("smtp.example.com", errors.New("dial tcp: connection refused"))
	if got != "邮件发送失败：dial tcp: connection refused" {
		t.Fatalf("smtpErrorMessage() = %q", got)
	}
}

func TestResolveFeishuOpenIDByEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			if got := r.URL.Query().Get("user_id_type"); got != "open_id" {
				t.Fatalf("user_id_type = %q", got)
			}
			var body map[string][]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body["emails"]) != 1 || body["emails"][0] != "person@example.com" {
				t.Fatalf("unexpected lookup payload: %#v", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_list":[{"user_id":"ou_test_user"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_SECRET", "secret")
	t.Setenv("FEISHU_BASE_URL", server.URL)

	got, err := resolveFeishuOpenID(context.Background(), appDB, "person@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ou_test_user" {
		t.Fatalf("OpenID = %q", got)
	}
}
