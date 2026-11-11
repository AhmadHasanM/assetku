// Package auth implements password hashing, sessions, login and profile endpoints.
package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	minPasswordLen = 8
)

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// PasswordProblem returns "too_short" when pw violates the password policy, else "".
func PasswordProblem(pw string) string {
	if len([]rune(pw)) < minPasswordLen {
		return "too_short"
	}
	return ""
}
