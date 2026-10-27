// Package auth contains JWT primitives shared by the issuer and gateway.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type claims struct {
	UserID  uint64 `json:"sub"`
	Role    string `json:"role"`
	Expires int64  `json:"exp"`
}

func IssueToken(userID uint64, role string, secret []byte, lifetime time.Duration) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	body, err := json.Marshal(claims{UserID: userID, Role: role, Expires: time.Now().Add(lifetime).Unix()})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyToken(token string, secret []byte) (uint64, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, "", errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return 0, "", errors.New("invalid signature")
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, "", err
	}
	var headerValues map[string]string
	if err := json.Unmarshal(header, &headerValues); err != nil || headerValues["alg"] != "HS256" {
		return 0, "", errors.New("invalid algorithm")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, "", err
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil || c.UserID == 0 || c.Expires <= time.Now().Unix() || (c.Role != "admin" && c.Role != "member") {
		return 0, "", errors.New("invalid claims")
	}
	return c.UserID, c.Role, nil
}
