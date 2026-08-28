package auth

import "golang.org/x/crypto/bcrypt"

// Hash is not deterministic, so it is not suitable for comparing passwords.
// Use CompareHashAndPassword for that purpose.
func Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func CompareHashAndPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
