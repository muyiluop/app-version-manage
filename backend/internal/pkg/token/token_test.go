package token

import (
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	signer, err := NewSigner("test-signing-key-0123456789")
	if err != nil {
		t.Fatalf("构造签名器失败: %v", err)
	}

	tok := signer.Sign("uploads/aa/bb/file.exe", time.Hour)
	payload, err := signer.Verify(tok)
	if err != nil {
		t.Fatalf("校验令牌失败: %v", err)
	}
	if payload != "uploads/aa/bb/file.exe" {
		t.Errorf("载荷不一致: %s", payload)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	signer, _ := NewSigner("test-signing-key-0123456789")
	tok := signer.Sign("secret/file.exe", time.Hour)

	// 篡改载荷
	parts := strings.SplitN(tok, ".", 2)
	if _, err := signer.Verify("Zm9yZ2Vk" + "." + parts[1]); err == nil {
		t.Error("篡改载荷后应校验失败")
	}
	// 篡改签名
	if _, err := signer.Verify(parts[0] + ".AAAA"); err == nil {
		t.Error("篡改签名后应校验失败")
	}
	// 其它密钥签发的令牌
	other, _ := NewSigner("another-signing-key-012345678")
	if _, err := signer.Verify(other.Sign("secret/file.exe", time.Hour)); err == nil {
		t.Error("不同密钥签发的令牌应校验失败")
	}
	// 格式错误
	for _, bad := range []string{"", "abc", "a.b.c", "%%%."} {
		if _, err := signer.Verify(bad); err == nil {
			t.Errorf("非法令牌 %q 应校验失败", bad)
		}
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	signer, _ := NewSigner("test-signing-key-0123456789")
	tok := signer.Sign("file.exe", -time.Minute)
	if _, err := signer.Verify(tok); err != ErrExpired {
		t.Errorf("过期令牌应返回 ErrExpired，实际 %v", err)
	}
}

func TestNewSignerRejectsWeakKey(t *testing.T) {
	if _, err := NewSigner("short"); err == nil {
		t.Error("弱密钥应被拒绝")
	}
}
