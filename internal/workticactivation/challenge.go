package workticactivation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Visual challenge is only one layer; email verification and rate limiting remain mandatory.
var tileKinds = []string{"chat", "spark", "shield", "calendar", "bolt", "heart"}

type Challenge struct {
	ID             string   `json:"id"`
	Target         string   `json:"target"`
	Tiles          []string `json:"tiles"`
	ExpiresSeconds int      `json:"expires_seconds"`
}

type Captcha struct{ DB *sql.DB }

func secureRandom(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func secureIndex(n int) (int, error) {
	if n <= 0 || n > 256 {
		return 0, errors.New("invalid range")
	}
	b, err := secureRandom(1)
	if err != nil {
		return 0, err
	}
	lim := 256 - (256 % n)
	for int(b[0]) >= lim {
		b, err = secureRandom(1)
		if err != nil {
			return 0, err
		}
	}
	return int(b[0]) % n, nil
}

func (s Captcha) Create(ctx context.Context) (Challenge, error) {
	if s.DB == nil {
		return Challenge{}, errors.New("DB required")
	}
	tiles := append([]string(nil), tileKinds...)
	for i := len(tiles) - 1; i > 0; i-- {
		j, err := secureIndex(i + 1)
		if err != nil {
			return Challenge{}, err
		}
		tiles[i], tiles[j] = tiles[j], tiles[i]
	}
	targetIndex, err := secureIndex(len(tiles))
	if err != nil {
		return Challenge{}, err
	}
	tokenBytes, err := secureRandom(32)
	if err != nil {
		return Challenge{}, err
	}
	id := base64.RawURLEncoding.EncodeToString(tokenBytes)
	digest := sha256.Sum256([]byte(id))
	expires := time.Now().UTC().Add(5 * time.Minute)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO worktic_captcha_challenges(id_hash,correct_index,expires_at) VALUES($1,$2,$3)`, digest[:], targetIndex, expires)
	if err != nil {
		return Challenge{}, err
	}
	images := make([]string, len(tiles))
	for i, kind := range tiles {
		raw, err := renderSymbolPNG(kind)
		if err != nil {
			return Challenge{}, err
		}
		images[i] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	}
	return Challenge{ID: id, Target: tiles[targetIndex], Tiles: images, ExpiresSeconds: 300}, nil
}

// Atomic consume on every answer, successful or not; no replay or guessing attempts.
func (s Captcha) Verify(ctx context.Context, id string, choice int) error {
	if len(id) != 43 || choice < 0 || choice >= len(tileKinds) {
		return errors.New("invalid challenge")
	}
	if _, err := base64.RawURLEncoding.DecodeString(id); err != nil {
		return errors.New("invalid challenge")
	}
	digest := sha256.Sum256([]byte(id))
	var expected int
	err := s.DB.QueryRowContext(ctx, `UPDATE worktic_captcha_challenges SET consumed_at=NOW() WHERE id_hash=$1 AND consumed_at IS NULL AND expires_at>NOW() RETURNING correct_index`, digest[:]).Scan(&expected)
	if err != nil {
		return errors.New("expired or used challenge")
	}
	if choice != expected {
		return errors.New("incorrect answer")
	}
	return nil
}

func (s Captcha) HTTPChallenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c, err := s.Create(r.Context())
	if err != nil {
		http.Error(w, "Challenge unavailable", http.StatusServiceUnavailable)
		return
	}
	_ = json.NewEncoder(w).Encode(c)
}
