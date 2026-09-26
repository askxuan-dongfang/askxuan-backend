package common

import "testing"

func TestValidateJWTSecret(t *testing.T) {
	for _, secret := range []string{"", "short", placeholderJWTSecret, " 123456789012345678901234567890123456 "} {
		if err := ValidateJWTSecret(secret); err == nil {
			t.Errorf("accepted weak JWT secret %q", secret)
		}
	}
	if err := ValidateJWTSecret("uP4F4FhW30tKT9fCm2cAhtj19InWPLmXzi3peMsiQng"); err != nil {
		t.Fatal(err)
	}
}
