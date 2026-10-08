package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sensitiveHeaderRegex = regexp.MustCompile(`(?i)(?:key|token|secret|auth)`)

type KeyStore struct {
	masterKey []byte
}

func NewKeyStore(keyPath string) (*KeyStore, error) {
	if keyPath == "" {
		return nil, errors.New("master key path cannot be empty")
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("create master key dir: %w", err)
	}

	data, err := os.ReadFile(keyPath)
	if err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("master key at %s must be exactly 32 bytes, got %d", keyPath, len(data))
		}
		return &KeyStore{masterKey: data}, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key: %w", err)
	}

	// Generate 32 random bytes
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}

	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}

	return &KeyStore{masterKey: key}, nil
}

func NewKeyStoreWithBytes(key []byte) (*KeyStore, error) {
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	k := make([]byte, 32)
	copy(k, key)
	return &KeyStore{masterKey: k}, nil
}

func (ks *KeyStore) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(ks.masterKey)
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (ks *KeyStore) Decrypt(encRef string) (string, error) {
	if !strings.HasPrefix(encRef, "enc:") {
		return "", errors.New("invalid encrypted key ref format")
	}

	rawB64 := strings.TrimPrefix(encRef, "enc:")
	rawBytes, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(ks.masterKey)
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(rawBytes) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce := rawBytes[:nonceSize]
	ciphertext := rawBytes[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

func (ks *KeyStore) Resolve(ref string) (string, error) {
	if strings.HasPrefix(ref, "env:") {
		varName := strings.TrimPrefix(ref, "env:")
		val := os.Getenv(varName)
		if val == "" {
			return "", fmt.Errorf("env var %s is empty", varName)
		}
		return val, nil
	}

	if strings.HasPrefix(ref, "enc:") {
		return ks.Decrypt(ref)
	}

	return "", fmt.Errorf("unsupported api key ref format: %s", ref)
}

func MaskRef(ref string) string {
	if strings.HasPrefix(ref, "env:") {
		return ref
	}
	if strings.HasPrefix(ref, "enc:") {
		return "enc:***"
	}
	return "***"
}

func MaskHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveHeader(k) {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}
	return masked
}

func isSensitiveHeader(key string) bool {
	lower := strings.ToLower(key)
	if lower == "authorization" || lower == "x-api-key" {
		return true
	}
	return sensitiveHeaderRegex.MatchString(key)
}
