package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// 32字节的密钥
var secretKey = []byte("518df8185cdc7d832b5f52b59fdd6185")

// GenerateSecureToken 生成加密token
func GenerateSecureToken(data string, expireTime time.Duration) (string, error) {

	// 准备明文数据：4字节过期时间 + 原始数据
	expireAt := uint32(time.Now().Add(expireTime).Unix())
	payload := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(payload[:4], expireAt)
	copy(payload[4:], []byte(data))

	out, err := AESGCMEncrypt(payload, secretKey)
	if err != nil {
		return "", fmt.Errorf("加密数据失败: %v", err)
	}
	// 转换为base62编码
	fmt.Printf("加密数据: %x\n", out)
	return base64.URLEncoding.EncodeToString(out), nil
}

// ValidateSecureToken 验证并解密token
func ValidateSecureToken(token string) (string, error) {
	// 从base62解码
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("无效的token格式: %v", err)
	}
	bytes, err := AESGCMDecrypt(data, secretKey)
	if err != nil {
		return "", fmt.Errorf("解密数据失败: %v", err)
	}
	// 解析过期时间和数据
	expireAt := binary.BigEndian.Uint32(bytes[:4])
	if uint32(time.Now().Unix()) > expireAt {
		return "", fmt.Errorf("token已过期")
	}

	return string(bytes[4:]), nil
}

// AES-GCM加密
func AESGCMEncrypt(plaintext []byte, key []byte) ([]byte, error) {
	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密块失败: %v", err)
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM模式失败: %v", err)
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %v", err)
	}

	fmt.Printf("生成的nonce: %x\n加密数据: %x", nonce, plaintext)
	// 加密数据并附加nonce
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// AES-GCM解密
func AESGCMDecrypt(ciphertext []byte, key []byte) ([]byte, error) {
	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密块失败: %v", err)
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM模式失败: %v", err)
	}

	// 检查密文长度是否足够包含nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	// 分离nonce和实际密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	fmt.Printf("解析的nonce: %x\n加密数据: %x", nonce, ciphertext)
	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %v", err)
	}

	return plaintext, nil
}
