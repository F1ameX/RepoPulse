// Package password hashes and verifies account passwords with Argon2id.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var b64 = base64.RawStdEncoding

// Parameter bounds. They protect against a process panic (time or threads 0),
// excessive memory or CPU per hash, and an empty key that would match any password.
// The same bounds are applied to configuration and to parameters read from stored hashes.
const (
	MinMemoryKiB = 19456 // 19 MiB, OWASP minimum
	MaxMemoryKiB = 262144
	MinTime      = 1
	MaxTime      = 10
	MinThreads   = 1
	MaxThreads   = 8
	MinKeyLen    = 16
	MaxKeyLen    = 64
	MinSaltLen   = 16
	MaxSaltLen   = 64
)

// Params configures Argon2id. HashMem is in KiB.
type Params struct {
	HashMem     uint64
	HashTime    uint64
	HashThreads uint64
	KeyLen      int
	SaltLen     int
}

// Validate checks every parameter against its bounds.
func (p Params) Validate() error {
	switch {
	case p.HashMem < MinMemoryKiB || p.HashMem > MaxMemoryKiB:
		return fmt.Errorf("argon2id memory must be between %d and %d KiB", MinMemoryKiB, MaxMemoryKiB)
	case p.HashTime < MinTime || p.HashTime > MaxTime:
		return fmt.Errorf("argon2id time must be between %d and %d", MinTime, MaxTime)
	case p.HashThreads < MinThreads || p.HashThreads > MaxThreads:
		return fmt.Errorf("argon2id threads must be between %d and %d", MinThreads, MaxThreads)
	case p.KeyLen < MinKeyLen || p.KeyLen > MaxKeyLen:
		return fmt.Errorf("argon2id key length must be between %d and %d bytes", MinKeyLen, MaxKeyLen)
	case p.SaltLen < MinSaltLen || p.SaltLen > MaxSaltLen:
		return fmt.Errorf("argon2id salt length must be between %d and %d bytes", MinSaltLen, MaxSaltLen)
	}
	return nil
}

type Argon2id struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// NewArgon2id validates p before narrowing the values to argon2 types.
func NewArgon2id(p Params) (*Argon2id, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &Argon2id{
		memory:  uint32(p.HashMem),
		time:    uint32(p.HashTime),
		threads: uint8(p.HashThreads),
		keyLen:  uint32(p.KeyLen),
		saltLen: uint32(p.SaltLen),
	}, nil
}

// Hash returns a PHC string: $argon2id$v=19$m=<KiB>,t=<time>,p=<threads>$<salt>$<key>.
func (pass *Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, pass.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, pass.time, pass.memory, pass.threads, pass.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, pass.memory, pass.time, pass.threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify checks password against a stored hash using the parameters stored in it,
// so hashes created with older settings keep working. A malformed hash or
// parameters outside the bounds are an error, not a mismatch.
func (pass *Argon2id) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("not an argon2id hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var m, t, p uint64
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, fmt.Errorf("parse argon2id parameters: %w", err)
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode argon2id salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode argon2id key: %w", err)
	}
	stored := Params{HashMem: m, HashTime: t, HashThreads: p, KeyLen: len(want), SaltLen: len(salt)}
	if err := stored.Validate(); err != nil {
		return false, fmt.Errorf("stored hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, uint32(t), uint32(m), uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
