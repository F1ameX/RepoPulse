package domain

import (
	"errors"
	"testing"
	"time"
)

func TestVerifiedContextRequiresAnActiveSession(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Second)
	accountID := "7c9357c8-e7f3-4144-bf9d-76ca7051077c"
	guestID := "cfe8ce89-aec5-4773-a951-6d1b45420258"
	emptyID := ""
	for _, tc := range []struct {
		name    string
		context VerifiedContext
		want    error
	}{
		{"missing context", VerifiedContext{}, ErrSessionRequired},
		{"account", VerifiedContext{UserID: &accountID, AccountExpiresAt: &future}, nil},
		{"guest", VerifiedContext{GuestSessionID: &guestID, GuestExpiresAt: &future}, nil},
		{"both", VerifiedContext{UserID: &accountID, AccountExpiresAt: &future, GuestSessionID: &guestID, GuestExpiresAt: &future}, nil},
		{"missing account expiry", VerifiedContext{UserID: &accountID}, ErrSessionRequired},
		{"missing guest expiry", VerifiedContext{GuestSessionID: &guestID}, ErrSessionRequired},
		{"expiry without identity", VerifiedContext{AccountExpiresAt: &future}, ErrSessionRequired},
		{"empty identity", VerifiedContext{UserID: &emptyID, AccountExpiresAt: &future}, ErrSessionRequired},
		{"guest expires at boundary", VerifiedContext{GuestSessionID: &guestID, GuestExpiresAt: &now}, ErrSessionExpired},
		{"account expires at boundary", VerifiedContext{UserID: &accountID, AccountExpiresAt: &now}, ErrSessionExpired},
		{"expired account cannot fall back to guest", VerifiedContext{UserID: &accountID, AccountExpiresAt: &past, GuestSessionID: &guestID, GuestExpiresAt: &future}, ErrSessionExpired},
		{"expired guest does not replace active account", VerifiedContext{UserID: &accountID, AccountExpiresAt: &future, GuestSessionID: &guestID, GuestExpiresAt: &past}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.context.Validate(now); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
