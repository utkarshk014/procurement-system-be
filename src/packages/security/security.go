// Package security holds password hashing.
package security

import (
	"github.com/alexedwards/argon2id"
)

// passwordParams is OWASP's argon2id baseline (19 MiB, 2 passes); the library default uses 64 MiB per hash.
var passwordParams = &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// HashPassword returns an argon2id hash; parameters and salt are encoded in the string.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, passwordParams)
}

// CheckPassword compares a password with an argon2id hash in constant time.
func CheckPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

// dummyHash makes an unknown email take as long to reject as a wrong password.
var dummyHash = func() string {
	h, err := HashPassword("not-a-real-password")
	if err != nil {
		panic(err)
	}
	return h
}()

// BurnPasswordCheck spends the same time as a real password check.
func BurnPasswordCheck(password string) {
	_, _ = CheckPassword(password, dummyHash)
}
