package auth

import "golang.org/x/crypto/bcrypt"

// DummyHash is a real cost-10 bcrypt hash of a random 32-byte string that was discarded
// right after hashing, so no password matches it. Login compares the submitted password
// against it when the e-mail is unknown or the account never got a password: an unknown
// e-mail then costs the same time as a wrong password, and the response time cannot be
// used to enumerate accounts. Generated once with bcrypt.GenerateFromPassword.
const DummyHash = "$2a$10$wo5ujLiIienUKAN9eQ1usOCIIScE23NlxlcTlx475Ew2jAark/LCe"

// HashPassword returns the bcrypt hash of password at bcrypt.DefaultCost (10).
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches hash. An empty or malformed hash (an
// account that was never given a password) never matches.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
