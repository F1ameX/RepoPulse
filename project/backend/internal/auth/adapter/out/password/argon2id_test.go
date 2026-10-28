package password

import (
	"strings"
	"testing"
)

// testParams are the cheapest allowed parameters to keep tests fast.
var testParams = Params{HashMem: MinMemoryKiB, HashTime: 1, HashThreads: 1, KeyLen: 32, SaltLen: 16}

func newTestHasher(t *testing.T) *Argon2id {
	t.Helper()
	hasher, err := NewArgon2id(testParams)
	if err != nil {
		t.Fatal(err)
	}
	return hasher
}

func TestHashAndVerifySamePasswordArgon2id(t *testing.T) {
	argon2id := newTestHasher(t)
	hash, err := argon2id.Hash("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	verified, err := argon2id.Verify("qwerty12345", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !verified {
		t.Fatal("password is not verified with its hash")
	}
}

func TestHashAndVerifyDifferentPasswordsArgon2id(t *testing.T) {
	argon2id := newTestHasher(t)
	hash, err := argon2id.Hash("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	verified, err := argon2id.Verify("54321ytrewq", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified {
		t.Fatal("a different password must not match")
	}
}

func TestDifferentsBetweenHashsOfPasswordArgon2id(t *testing.T) {
	argon2id := newTestHasher(t)
	hash1, err := argon2id.Hash("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := argon2id.Hash("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash2 {
		t.Fatal("hashes must differ because the salt is random")
	}
}

// FR-27: the full password is checked. bcrypt would ignore everything after 72 bytes.
func TestWholeLongPasswordIsVerified(t *testing.T) {
	argon2id := newTestHasher(t)
	password := strings.Repeat("ж", 100) // 200 bytes
	hash, err := argon2id.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := argon2id.Verify(strings.Repeat("ж", 99)+"з", hash)
	if err != nil {
		t.Fatal(err)
	}
	if verified {
		t.Fatal("a change after 72 bytes must be detected")
	}
}

func TestVerifyRejectsUnsafeStoredHashes(t *testing.T) {
	argon2id := newTestHasher(t)
	valid, err := argon2id.Hash("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, "$") // "", argon2id, v=19, m=..,t=..,p=.., salt, key
	salt, key := parts[4], parts[5]

	for name, encoded := range map[string]string{
		// An empty key would make ConstantTimeCompare of two empty slices succeed for any password.
		"empty key":      "$argon2id$v=19$m=19456,t=1,p=1$" + salt + "$",
		"empty salt":     "$argon2id$v=19$m=19456,t=1,p=1$$" + key,
		"zero time":      "$argon2id$v=19$m=19456,t=0,p=1$" + salt + "$" + key, // argon2 panics
		"zero threads":   "$argon2id$v=19$m=19456,t=1,p=0$" + salt + "$" + key, // argon2 panics
		"huge memory":    "$argon2id$v=19$m=4194304,t=1,p=1$" + salt + "$" + key,
		"other alg":      "$argon2i$v=19$m=19456,t=1,p=1$" + salt + "$" + key,
		"other version":  "$argon2id$v=16$m=19456,t=1,p=1$" + salt + "$" + key,
		"bad base64":     "$argon2id$v=19$m=19456,t=1,p=1$" + salt + "$!!!",
		"missing fields": "$argon2id$v=19$" + key,
		"not a hash":     "qwerty12345",
	} {
		t.Run(name, func(t *testing.T) {
			verified, err := argon2id.Verify("anything", encoded)
			if err == nil || verified {
				t.Fatalf("Verify = %v, %v; want an error", verified, err)
			}
		})
	}
}

func TestNewArgon2idValidatesParams(t *testing.T) {
	for name, mutate := range map[string]func(*Params){
		"memory below minimum":   func(p *Params) { p.HashMem = MinMemoryKiB - 1 },
		"memory above maximum":   func(p *Params) { p.HashMem = MaxMemoryKiB + 1 },
		"zero time":              func(p *Params) { p.HashTime = 0 },
		"too many threads":       func(p *Params) { p.HashThreads = 300 }, // would wrap to 44 as uint8
		"zero key length":        func(p *Params) { p.KeyLen = 0 },
		"short salt":             func(p *Params) { p.SaltLen = 8 },
		"too long key length":    func(p *Params) { p.KeyLen = MaxKeyLen + 1 },
		"too long salt length":   func(p *Params) { p.SaltLen = MaxSaltLen + 1 },
		"time above the maximum": func(p *Params) { p.HashTime = MaxTime + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			p := testParams
			mutate(&p)
			if _, err := NewArgon2id(p); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
