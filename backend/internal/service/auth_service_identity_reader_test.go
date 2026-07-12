package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type authIdentityReaderStub struct {
	user *User
}

func (s authIdentityReaderStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

func (s authIdentityReaderStub) GetByEmail(context.Context, string) (*User, error) {
	return s.user, nil
}

func (s authIdentityReaderStub) GetFirstAdmin(context.Context) (*User, error) {
	return s.user, nil
}

func TestLoginUsesConfiguredIdentityReader(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("sub2api-password"), bcrypt.MinCost)
	require.NoError(t, err)
	identity := &User{
		ID:           42,
		Email:        "admin@example.com",
		PasswordHash: string(hash),
		Role:         RoleAdmin,
		Status:       StatusActive,
	}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	svc := NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil).
		WithIdentityReader(authIdentityReaderStub{user: identity})

	token, user, err := svc.Login(context.Background(), "ADMIN@example.com", "sub2api-password")

	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Same(t, identity, user)
}
