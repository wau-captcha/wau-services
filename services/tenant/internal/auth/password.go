package auth

import "golang.org/x/crypto/bcrypt"

// Bcrypt hashes passwords at the library default cost.
type Bcrypt struct{}

func NewBcrypt() *Bcrypt { return &Bcrypt{} }

func (Bcrypt) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// Compare reports whether password matches hash; a malformed hash counts as a mismatch.
func (Bcrypt) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
