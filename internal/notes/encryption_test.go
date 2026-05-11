package notes

import (
	"bytes"
	"testing"
)

func TestPasswordEncryptorRoundTrip(t *testing.T) {
	encryptor := NewPasswordEncryptor("correct horse battery staple")
	plaintext := []byte("private note content")

	salt, nonce, ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if len(salt) != saltSize {
		t.Fatalf("Encrypt() salt length = %d, want %d", len(salt), saltSize)
	}
	if len(nonce) == 0 {
		t.Fatal("Encrypt() returned empty nonce")
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("Encrypt() returned plaintext as ciphertext")
	}

	got, err := encryptor.Decrypt(salt, nonce, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestPasswordEncryptorRejectsWrongPassword(t *testing.T) {
	encryptor := NewPasswordEncryptor("correct password")
	salt, nonce, ciphertext, err := encryptor.Encrypt([]byte("private note content"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	wrongEncryptor := NewPasswordEncryptor("wrong password")
	if _, err := wrongEncryptor.Decrypt(salt, nonce, ciphertext); err == nil {
		t.Fatal("Decrypt() error = nil, want authentication failure")
	}
}
