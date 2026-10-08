package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyStore_EncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")

	ks, err := NewKeyStore(keyPath)
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}

	secret := "sk-ant-api03-test-secret-key-12345"
	enc, err := ks.Encrypt(secret)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if enc == secret {
		t.Fatalf("encrypted value should not match secret")
	}

	dec, err := ks.Decrypt(enc)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if dec != secret {
		t.Fatalf("expected %s, got %s", secret, dec)
	}

	// Test reload existing key
	ks2, err := NewKeyStore(keyPath)
	if err != nil {
		t.Fatalf("failed to reload keystore: %v", err)
	}
	dec2, err := ks2.Decrypt(enc)
	if err != nil {
		t.Fatalf("failed to decrypt with reloaded keystore: %v", err)
	}
	if dec2 != secret {
		t.Fatalf("expected %s, got %s", secret, dec2)
	}
}

func TestKeyStore_Resolve(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")
	ks, err := NewKeyStore(keyPath)
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}

	// Test env var
	envKey := "TEST_DOO_KEY_XYZ"
	t.Setenv(envKey, "my-env-secret-val")

	resolved, err := ks.Resolve("env:" + envKey)
	if err != nil {
		t.Fatalf("resolve env failed: %v", err)
	}
	if resolved != "my-env-secret-val" {
		t.Fatalf("expected my-env-secret-val, got %s", resolved)
	}

	// Test empty env var
	emptyEnvKey := "TEST_DOO_KEY_EMPTY"
	os.Unsetenv(emptyEnvKey)
	_, err = ks.Resolve("env:" + emptyEnvKey)
	if err == nil {
		t.Fatalf("expected error for empty env var, got nil")
	}

	// Test enc
	enc, err := ks.Encrypt("my-enc-secret")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	resolvedEnc, err := ks.Resolve(enc)
	if err != nil {
		t.Fatalf("resolve enc failed: %v", err)
	}
	if resolvedEnc != "my-enc-secret" {
		t.Fatalf("expected my-enc-secret, got %s", resolvedEnc)
	}
}

func TestMaskRef(t *testing.T) {
	if got := MaskRef("env:OPENAI_API_KEY"); got != "env:OPENAI_API_KEY" {
		t.Errorf("expected env:OPENAI_API_KEY, got %s", got)
	}
	if got := MaskRef("enc:abc123xyz"); got != "enc:***" {
		t.Errorf("expected enc:***, got %s", got)
	}
}

func TestMaskHeaders(t *testing.T) {
	input := map[string]string{
		"Content-Type":    "application/json",
		"Authorization":   "Bearer sk-12345",
		"x-api-key":       "my-anthropic-key",
		"X-Custom-Secret": "secret-value",
		"User-Agent":      "doo/1.0",
		"X-Auth-Token":    "token-value",
	}

	masked := MaskHeaders(input)
	if masked["Content-Type"] != "application/json" {
		t.Errorf("Content-Type should not be masked")
	}
	if masked["User-Agent"] != "doo/1.0" {
		t.Errorf("User-Agent should not be masked")
	}
	if masked["Authorization"] != "***" {
		t.Errorf("Authorization should be masked")
	}
	if masked["x-api-key"] != "***" {
		t.Errorf("x-api-key should be masked")
	}
	if masked["X-Custom-Secret"] != "***" {
		t.Errorf("X-Custom-Secret should be masked")
	}
	if masked["X-Auth-Token"] != "***" {
		t.Errorf("X-Auth-Token should be masked")
	}
}
