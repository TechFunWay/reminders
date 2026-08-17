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

func TestNormalizeDingTalkWebhook(t *testing.T) {
	got, err := normalizeTarget(ChannelDingTalk, "https://oapi.dingtalk.com/robot/send?access_token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://oapi.dingtalk.com/robot/send?access_token=test-token"; got != want {
		t.Fatalf("webhook = %q, want %q", got, want)
	}
	for _, raw := range []string{
		"http://oapi.dingtalk.com/robot/send?access_token=test-token",
		"https://example.com/robot/send?access_token=test-token",
		"https://oapi.dingtalk.com/robot/send?access_token=test-token&sign=stale",
		"https://oapi.dingtalk.com/robot/send",
	} {
		if _, err := normalizeTarget(ChannelDingTalk, raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
	if got := maskTarget(ChannelDingTalk, "https://oapi.dingtalk.com/robot/send?access_token=secret"); got != "钉钉机器人 Webhook（已加密）" {
		t.Fatalf("masked webhook = %q", got)
	}
}

func TestSendDingTalkUsesKeywordCompatibleText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var payload struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.MsgType != "text" || !strings.Contains(payload.Text.Content, "提醒：喝水") {
			t.Fatalf("unexpected DingTalk payload: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	if _, err := sendDingTalk(context.Background(), nil, server.URL, Reminder{Title: "喝水"}); err != nil {
		t.Fatal(err)
	}
}
