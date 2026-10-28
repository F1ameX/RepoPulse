package password

import "testing"

func TestHashAndVerifySamePasswordArgon2id(t *testing.T) {
	password := "qwerty12345"
	argon2id := NewArgon2id(16 * 1024, 3, 1, 32, 16)
	hash, err := argon2id.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	
	verified, err := argon2id.Verify(password, hash)
	if !verified {
		t.Fatalf("password is not verified eith hash")
	} else if err != nil{
		t.Fatalf("verification: %v", err)
	}

}

func TestHashAndVerifyDifferentPasswordsArgon2id(t * testing.T) {
	pass1 := "qwerty12345"
	pass2 := "54321ytrewq"
	
	argon2id := NewArgon2id(16 * 1024, 3, 1, 32, 16)

	hash1, err := argon2id.Hash(pass1)
	if err != nil {
		t.Fatal(err)
	}
	
	verified, err := argon2id.Verify(pass2, hash1)
	if verified {
		t.Fatalf("passwords are different!")
	} else if err != nil{
		t.Fatalf("verification: %v", err)
	}
}

func TestDifferentsBetweenHashsOfPasswordArgon2id(t * testing.T) {
	password := "qwerty12345"
	argon2id := NewArgon2id(16 * 1024, 3, 1, 32, 16)
	
	hash1, err1 := argon2id.Hash(password)
	if err1 != nil {
		t.Fatal(err1)
	}
	
	hash2, err2 := argon2id.Hash(password)
	if err2 != nil {
		t.Fatal(err2)
	}

	if hash1 == hash2 {
		t.Fatal("hashs must be different because salt is different")
	}

}
