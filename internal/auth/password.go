package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordConfig struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

// Default password hashing configuration
var DefaultPasswordConfig = &PasswordConfig{
	time:    3,         // Number of iterations
	memory:  64 * 1024, // 64MB
	threads: 2,         // Number of threads to use
	keyLen:  32,        // Length of the generated key
}

// HashPassword hashes a password using Argon2id
func HashPassword(password string, c *PasswordConfig) (string, error) {
	if c == nil {
		c = DefaultPasswordConfig
	}

	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash the password
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		c.time,
		c.memory,
		c.threads,
		c.keyLen,
	)

	// Encode as base64
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=memory,t=time,p=threads$salt$hash
	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		c.memory,
		c.time,
		c.threads,
		b64Salt,
		b64Hash,
	)

	return encodedHash, nil
}

// VerifyPassword checks if a password matches a hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	// Extract the parameters, salt and hash from the encoded hash string
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var version int
	var memory, time uint32
	var threads uint8

	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return false, err
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	// Compute the hash of the provided password using the same parameters
	otherHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(hash)))

	// Compare the computed hash with the stored hash
	return string(hash) == string(otherHash), nil
}
