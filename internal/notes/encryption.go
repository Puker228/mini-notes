package notes

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type Encryptor interface {
	Encrypt(plaintext []byte) (salt, nonce, ciphertext []byte, err error)
	Decrypt(salt, nonce, ciphertext []byte) ([]byte, error)
}

type PasswordEncryptor struct {
	password string
}

const saltSize = 16

func NewPasswordEncryptor(password string) *PasswordEncryptor {
	return &PasswordEncryptor{password: password}
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		2,       // iterations
		64*1024, // memory KiB = 64 MiB
		1,       // parallelism
		32,      // key length
	)
}

func (p *PasswordEncryptor) Encrypt(plaintext []byte) (salt, nonce, ciphertext []byte, err error) {
	salt = make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, err
	}

	key := deriveKey(p.password, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, nil, err
	}

	nonce = make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, nil, err
	}

	ciphertext = aead.Seal(nil, nonce, plaintext, nil)
	return salt, nonce, ciphertext, nil
}

func (p *PasswordEncryptor) Decrypt(salt, nonce, ciphertext []byte) ([]byte, error) {
	if len(salt) != saltSize {
		return nil, fmt.Errorf("invalid salt size: got %d, want %d", len(salt), saltSize)
	}

	key := deriveKey(p.password, salt)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonce, ciphertext, nil)
}
