package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-do-not-use-in-prod"

func TestClaimsRoundTrip(t *testing.T) {
	tok, err := GenerateToken(42, "alice", "admin", 3, testSecret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	uid, name, role, authVersion, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if uid != 42 || name != "alice" || role != "admin" || authVersion != 3 {
		t.Fatalf("round-trip identity: got (%d,%q,%q), want (42,alice,admin)", uid, name, role)
	}

	// Verify the wire format uses the documented JSON keys (userID/username/
	// role) rather than the Go field names — external clients that mint or
	// parse tokens independently depend on these literal keys.
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not 3 segments: %d parts", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	for _, key := range []string{"userID", "username", "role", "authVersion", "exp", "iat"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("payload missing key %q: %s", key, payload)
		}
	}
}

// TestClaimsJSONKeyCompat locks the wire format against the legacy
// jwt.MapClaims shape — old clients / externally-minted tokens rely on the
// lower-case keys userID/username/role and the standard exp/iat pair.
func TestClaimsJSONKeyCompat(t *testing.T) {
	now := time.Now()
	legacy := jwt.MapClaims{
		"userID":   uint(7),
		"username": "legacy",
		"role":     "user",
		"exp":      now.Add(time.Hour).Unix(),
		"iat":      now.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy)
	signed, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}

	uid, name, role, authVersion, err := ParseToken(signed, testSecret)
	if err != nil {
		t.Fatalf("ParseToken rejected a legacy MapClaims token: %v", err)
	}
	if uid != 7 || name != "legacy" || role != "user" {
		t.Fatalf("legacy round-trip: got (%d,%q,%q), want (7,legacy,user)", uid, name, role)
	}
	if authVersion != 0 {
		t.Fatalf("legacy authVersion = %d, want 0", authVersion)
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	tok, _ := GenerateToken(1, "u", "user", 1, testSecret)
	if _, _, _, _, err := ParseToken(tok, "different-secret"); err == nil {
		t.Fatal("ParseToken accepted a token signed with a different secret")
	}
}

func TestParseTokenRejectsExpired(t *testing.T) {
	claims := Claims{
		UserID:   1,
		Username: "u",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(testSecret))
	if _, _, _, _, err := ParseToken(signed, testSecret); err == nil {
		t.Fatal("ParseToken accepted an expired token")
	}
}

func TestParseTokenRejectsAlgConfusion(t *testing.T) {
	claims := Claims{UserID: 1, Username: "u", Role: "user"}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, _ := tok.SignedString([]byte(testSecret))
	if _, _, _, _, err := ParseToken(signed, testSecret); err == nil {
		t.Fatal("ParseToken accepted an alg=none token (signing-method bypass)")
	}
}

func TestParseTokenRejectsOtherHMACAlgorithms(t *testing.T) {
	claims := Claims{UserID: 1, Username: "u", Role: "user", AuthVersion: 1}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := ParseToken(signed, testSecret); err == nil {
		t.Fatal("ParseToken accepted HS384 when only HS256 is issued")
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "hunter2") {
		t.Fatal("VerifyPassword rejected a freshly-hashed password")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("VerifyPassword accepted a wrong password")
	}
	if _, err := HashPassword(strings.Repeat("x", 73)); err == nil {
		t.Fatal("HashPassword accepted input beyond bcrypt's 72-byte limit")
	}
}

func TestLegacyHashMigrationStillWorks(t *testing.T) {
	if !VerifyPassword(legacyHash("legacy"), "legacy") {
		t.Fatal("VerifyPassword rejected a legacy-MD5 hash")
	}
	hash, err := HashPassword("x")
	if err != nil {
		t.Fatal(err)
	}
	if IsLegacyHash(hash) {
		t.Fatal("IsLegacyHash reported a bcrypt hash as legacy")
	}
	if !IsLegacyHash(legacyHash("x")) {
		t.Fatal("IsLegacyHash failed to recognize a legacy MD5 hash")
	}
}
