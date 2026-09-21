// Package token 提供基于 HMAC-SHA256 的短时效签名令牌。
//
// 用于下载链接：令牌内容为存储对象键，过期时间内有效，服务端不需存储任何状态。
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 常见错误。
var (
	ErrInvalidToken = errors.New("令牌格式无效")
	ErrExpired      = errors.New("令牌已过期")
)

// Signer 签名器。
type Signer struct {
	key []byte
}

// NewSigner 构造签名器，密钥长度不足 16 时会返回错误。
func NewSigner(key string) (*Signer, error) {
	if len(key) < 16 {
		return nil, fmt.Errorf("签名密钥长度不足 16")
	}
	return &Signer{key: []byte(key)}, nil
}

// Sign 生成令牌，payload 为任意字符串（下载场景下即存储对象键）。
func (s *Signer) Sign(payload string, ttl time.Duration) string {
	expireAt := time.Now().Add(ttl).Unix()
	raw := strconv.FormatInt(expireAt, 10) + "\n" + payload
	return s.encode(raw)
}

// Verify 校验令牌并返回原始 payload。
func (s *Signer) Verify(tok string) (string, error) {
	raw, err := s.decode(tok)
	if err != nil {
		return "", err
	}
	idx := strings.IndexByte(raw, '\n')
	if idx <= 0 {
		return "", ErrInvalidToken
	}
	expireAt, err := strconv.ParseInt(raw[:idx], 10, 64)
	if err != nil {
		return "", ErrInvalidToken
	}
	if time.Now().Unix() > expireAt {
		return "", ErrExpired
	}
	return raw[idx+1:], nil
}

func (s *Signer) encode(raw string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(raw))
	return base64.RawURLEncoding.EncodeToString([]byte(raw)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Signer) decode(tok string) (string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		return "", ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidToken
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", ErrInvalidToken
	}
	return string(raw), nil
}
