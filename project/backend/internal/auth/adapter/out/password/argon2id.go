// Package password is using for hashing and verifying passwords in auth service
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

//TODO: Вынести в конфигурацию
const (
	memory  = 16 * 1024 * 1024 // MiB
	time    = 3
	threads = 1 
	keyLen  = 32
	saltLen = 16
)

var b64 = base64.RawStdEncoding

type Argon2id struct {
	memory uint32
	time uint32
	threads uint8
	keyLen uint32
	saltLen uint32
}

func NewArgon2id(memory, time, threads, keyLen, saltLen int) *Argon2id{
	return &Argon2id{
		memory: uint32(memory),
		time: uint32(time),
		threads: uint8(threads),
		keyLen: uint32(keyLen),
		saltLen: uint32(saltLen),
	}
}

func (pass * Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, pass.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, pass.time, pass.memory, pass.threads, pass.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, pass.memory, pass.time, pass.threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func (pass * Argon2id) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("not an argon2id hash")
	}
	var version int
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, err
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

