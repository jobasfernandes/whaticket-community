//go:build integration

package dbtest

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/queue"
	"github.com/canove/whaticket-community/backend/internal/user"
	"github.com/canove/whaticket-community/backend/internal/whatsapp"
)

const (
	TestAccessSecret  = "test-access-secret-development-only"
	TestRefreshSecret = "test-refresh-secret-development-only"
)

type SeededUser struct {
	User     *user.User
	Password string
	Token    string
}

func SeedAdmin(t *testing.T, p *Postgres, name, email, password string) *SeededUser {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	u := &user.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Profile:      "admin",
	}
	if err := p.DB.WithContext(context.Background()).Create(u).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	tok, err := auth.CreateAccessToken(auth.UserSummary{ID: u.ID, Name: u.Name, Profile: u.Profile}, []byte(TestAccessSecret))
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return &SeededUser{User: u, Password: password, Token: tok}
}

func SeedRegularUser(t *testing.T, p *Postgres, name, email, password string) *SeededUser {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	u := &user.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Profile:      "user",
	}
	if err := p.DB.WithContext(context.Background()).Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tok, err := auth.CreateAccessToken(auth.UserSummary{ID: u.ID, Name: u.Name, Profile: u.Profile}, []byte(TestAccessSecret))
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return &SeededUser{User: u, Password: password, Token: tok}
}

func SeedWhatsapp(t *testing.T, p *Postgres, name string) *whatsapp.Whatsapp {
	t.Helper()
	w := &whatsapp.Whatsapp{
		Name:          name,
		Status:        whatsapp.StatusOpening,
		MediaDelivery: whatsapp.MediaDeliveryBase64,
	}
	if err := p.DB.WithContext(context.Background()).Create(w).Error; err != nil {
		t.Fatalf("seed whatsapp: %v", err)
	}
	return w
}

func SeedQueue(t *testing.T, p *Postgres, name, color string) *queue.Queue {
	t.Helper()
	q := &queue.Queue{Name: name, Color: color}
	if err := p.DB.WithContext(context.Background()).Create(q).Error; err != nil {
		t.Fatalf("seed queue: %v", err)
	}
	return q
}

func SeedContact(t *testing.T, p *Postgres, name, number string) *contact.Contact {
	t.Helper()
	c := &contact.Contact{Name: name, Number: number}
	if err := p.DB.WithContext(context.Background()).Create(c).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	return c
}
