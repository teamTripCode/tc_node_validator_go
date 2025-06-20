package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
	// Assuming "tripcodechain_go/utils" will be available later.
	// For now, using "log" for basic logging.
	"log"
)

// LocalSigner implements the Signer interface using a local key file.
type LocalSigner struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	address    string
}

// NewLocalSigner creates or loads a local signer.
// dataDir is the directory where the key file (node_key.enc) is stored.
func NewLocalSigner(dataDir string, passphrase string) (*LocalSigner, error) {
	if dataDir == "" {
		return nil, errors.New("dataDir cannot be empty")
	}
	if passphrase == "" {
		return nil, errors.New("passphrase cannot be empty")
	}

	keyFilePath := filepath.Join(dataDir, "node_key.enc")
	var privateKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey

	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ed25519 key pair: %w", err)
		}

		if err := os.MkdirAll(dataDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
		}

		if err := saveEncryptedKey(keyFilePath, privateKey, passphrase); err != nil {
			return nil, fmt.Errorf("failed to save encrypted key: %w", err)
		}
		log.Printf("INFO: New Ed25519 key pair generated and saved to %s", keyFilePath)
	} else {
		decryptedKeyBytes, err := loadAndDecryptKey(keyFilePath, passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to load and decrypt key: %w", err)
		}
		privateKey = ed25519.PrivateKey(decryptedKeyBytes)
		publicKey = privateKey.Public().(ed25519.PublicKey)
		log.Printf("INFO: Loaded Ed25519 node key from %s", keyFilePath)
	}

	address := hex.EncodeToString(publicKey)

	return &LocalSigner{
		privateKey: privateKey,
		publicKey:  publicKey,
		address:    address,
	}, nil
}

// deriveKey derives a key from a passphrase and salt using PBKDF2.
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, 4096, 32, sha256.New)
}

// saveEncryptedKey encrypts the private key and saves it to the specified file.
func saveEncryptedKey(filePath string, privateKey ed25519.PrivateKey, passphrase string) error {
	salt := make([]byte, 16) // 16 bytes for salt is a common practice
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, privateKey, nil)

	// Data format: salt (16 bytes) || nonce (gcm.NonceSize()) || ciphertext
	encryptedData := append(salt, nonce...)
	encryptedData = append(encryptedData, ciphertext...)

	if err := os.WriteFile(filePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted key to file %s: %w", filePath, err)
	}
	return nil
}

// loadAndDecryptKey loads an encrypted private key from a file and decrypts it.
func loadAndDecryptKey(filePath string, passphrase string) ([]byte, error) {
	encryptedData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted key file %s: %w", filePath, err)
	}

	// Data format: salt (16 bytes) || nonce (gcm.NonceSize()) || ciphertext
	if len(encryptedData) < 16 { // Minimum length for salt
		return nil, errors.New("encrypted data is too short to contain salt")
	}
	salt := encryptedData[:16]

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < 16+nonceSize {
		return nil, errors.New("encrypted data is too short to contain nonce")
	}
	nonce := encryptedData[16 : 16+nonceSize]
	ciphertext := encryptedData[16+nonceSize:]

	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Common error here is "cipher: message authentication failed",
		// which can happen if the passphrase is wrong or data is corrupt.
		return nil, fmt.Errorf("failed to decrypt key (check passphrase or data integrity): %w", err)
	}
	return decryptedData, nil
}

// Sign signs data using the local private key.
func (ls *LocalSigner) Sign(data []byte) ([]byte, error) {
	if ls.privateKey == nil {
		return nil, errors.New("private key is not initialized")
	}
	return ed25519.Sign(ls.privateKey, data), nil
}

// PublicKey returns the public key.
func (ls *LocalSigner) PublicKey() []byte {
	return ls.publicKey
}

// Address returns the hex-encoded public key.
func (ls *LocalSigner) Address() string {
	return ls.address
}
