package auth

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

// Claims is the shape of every JWT issued by smallgo. Using a typed struct
// (vs jwt.MapClaims) keeps (float64) type-assertions out of callers and lets
// the compiler enforce the claim surface at every site.
type Claims struct {
	UserID      uint   `json:"userID"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	AuthVersion uint   `json:"authVersion"`
	jwt.RegisteredClaims
}

// GenerateToken issues a 7-day HS256 JWT for the given user. The serialized
// body includes an authVersion claim so password changes and resets can revoke
// every previously-issued token for the account.
func GenerateToken(userID uint, username, role string, authVersion uint, secret string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		Username:    username,
		Role:        role,
		AuthVersion: authVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken verifies signature, expiry, and claim shape, then returns the
// user identity triple. An expired, malformed, or wrong-secret token returns
// the underlying jwt.Parse error.
func ParseToken(tokenString, secret string) (uint, string, string, uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return 0, "", "", 0, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, "", "", 0, jwt.ErrSignatureInvalid
	}
	return claims.UserID, claims.Username, claims.Role, claims.AuthVersion, nil
}

func GenerateAPIKey() string {
	return "sg_" + uuid.New().String()
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// VerifyPassword checks a password against a stored hash.
// Supports both bcrypt and legacy MD5 hashes for migration.
func VerifyPassword(storedHash string, password string) bool {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	return storedHash == legacyHash(password)
}

// IsLegacyHash returns true if the stored hash is the old MD5 format.
func IsLegacyHash(storedHash string) bool {
	return !isBcryptHash(storedHash)
}

func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$")
}

// legacyHash preserves the original double-MD5 algorithm for seamless migration.
func legacyHash(password string) string {
	first := md5.Sum([]byte(password))
	firstHex := hex.EncodeToString(first[:])
	second := md5.Sum([]byte(firstHex))
	return hex.EncodeToString(second[:])
}
