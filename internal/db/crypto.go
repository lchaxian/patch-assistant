package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"sync"
)

var (
	encryptionKey []byte
	keyOnce       sync.Once
)

// getEncryptionKey 获取或生成加密密钥
func getEncryptionKey() []byte {
	keyOnce.Do(func() {
		keyFile := "encryption.key"
		data, err := os.ReadFile(keyFile)
		if err == nil && len(data) == 64 {
			key, _ := hex.DecodeString(string(data))
			if len(key) == 32 {
				encryptionKey = key
				return
			}
		}
		// 生成新密钥
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic("failed to generate encryption key: " + err.Error())
		}
		encryptionKey = key
		_ = os.WriteFile(keyFile, []byte(hex.EncodeToString(key)), 0600)
	})
	return encryptionKey
}

// EncryptPassword 使用 AES-256-GCM 加密密码
func EncryptPassword(password string) (string, error) {
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(password), nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptPassword 使用 AES-256-GCM 解密密码
func DecryptPassword(encrypted string) (string, error) {
	key := getEncryptionKey()
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", err
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
