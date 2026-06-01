package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TEsting(t *testing.T) {

	new_u := uuid.New()
	secret := "some-secret"
	expiresIn := time.Hour

	tokenS, err := MakeJWT(new_u, secret, expiresIn)
	if err != nil {
		t.Fatalf("expected no error making JWT, got %v", err)
	}

	parsedUserID, err := ValidateJWT(tokenS, secret)
	if err != nil {
		t.Fatalf("expected no error validating JWT, got %v", err)
	}

	if parsedUserID != new_u {
		t.Errorf("expected user ID %v, got %v", new_u, parsedUserID)
	}
}
