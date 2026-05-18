package agents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ErrAgentNotFound is returned when no agents row exists for the id.
var ErrAgentNotFound = errors.New("agent not found")

// ErrAPIKeyNotConfigured when api_key_hash is null or empty.
var ErrAPIKeyNotConfigured = errors.New("api key not configured for agent")

// GenerateAPIKey returns a random hex secret suitable for X-API-Key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashAPIKey bcrypt-hashes a plaintext API key for storage in agents.api_key_hash.
func HashAPIKey(plaintext string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckAPIKey compares plaintext to a bcrypt hash.
func CheckAPIKey(hash, plaintext string) bool {
	if hash == "" || plaintext == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// FetchAPIKeyHash loads api_key_hash for an agent (may be empty).
func FetchAPIKeyHash(ctx context.Context, pool *pgxpool.Pool, agentID uuid.UUID) (string, error) {
	if pool == nil {
		return "", errors.New("database not configured")
	}
	row := pool.QueryRow(ctx, `SELECT COALESCE(api_key_hash, '') FROM agents WHERE id = $1`, agentID)
	var s string
	if err := row.Scan(&s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrAgentNotFound
		}
		return "", err
	}
	return s, nil
}

// ValidateCredentials checks agent exists and API key matches stored hash.
func ValidateCredentials(ctx context.Context, pool *pgxpool.Pool, agentID uuid.UUID, apiKey string) error {
	hash, err := FetchAPIKeyHash(ctx, pool, agentID)
	if err != nil {
		return err
	}
	if hash == "" {
		return ErrAPIKeyNotConfigured
	}
	if !CheckAPIKey(hash, apiKey) {
		return errors.New("invalid api key")
	}
	return nil
}

// SetAPIKeyHash updates stored hash (used when rotating keys).
func SetAPIKeyHash(ctx context.Context, pool *pgxpool.Pool, agentID uuid.UUID, hash string) error {
	if pool == nil {
		return errors.New("database not configured")
	}
	tag, err := pool.Exec(ctx, `UPDATE agents SET api_key_hash = $2 WHERE id = $1`, agentID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAgentNotFound
	}
	return nil
}
