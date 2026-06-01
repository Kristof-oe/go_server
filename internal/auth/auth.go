package auth

import (
	"log"
	"time"

	"net/http"

	"errors"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {

	hash_pw, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hash_pw, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	log.Printf("Match: %v", match)
	return match, err
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String()})

	return token.SignedString([]byte(tokenSecret))

}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(claims.Subject)

}

func GetBearerToken(headers http.Header) (string, error) {

	authHead := headers.Get("Authorization")
	if authHead == "" {
		return "", errors.New("wrong")
	}

	parts := strings.Split(authHead, " ")

	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("wrong")
	}
	return parts[1], nil
}
