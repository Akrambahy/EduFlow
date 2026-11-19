package identity

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
)

var dummyHash, _ = HashPassword("dummy-password-1")

type Session struct {
	AccessToken  string
	RefreshToken string
	User         *User
}

type Service struct {
	repo       *Repository
	tokens     *TokenIssuer
	refreshTTL time.Duration
}

func NewService(repo *Repository, tokens *TokenIssuer, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, tokens: tokens, refreshTTL: refreshTTL}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) Login(ctx context.Context, email, password string, meta SessionMeta) (*Session, error) {
	user, err := s.repo.UserByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, ErrNotFound) {
		CheckPassword(dummyHash, password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	refresh, hash, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateSession(ctx, user.ID, hash, time.Now().Add(s.refreshTTL), meta); err != nil {
		return nil, err
	}
	if err := s.repo.TouchLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	return s.session(user, refresh)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, meta SessionMeta) (*Session, error) {
	if refreshToken == "" {
		return nil, ErrSessionInvalid
	}
	newRaw, newHash, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	userID, err := s.repo.RotateSession(ctx, HashRefreshToken(refreshToken), newHash, time.Now().Add(s.refreshTTL), meta)
	if errors.Is(err, ErrSessionReused) {
		if rerr := s.repo.RevokeAllSessions(ctx, userID); rerr != nil {
			return nil, rerr
		}
		return nil, ErrSessionInvalid
	}
	if err != nil {
		return nil, err
	}

	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		if rerr := s.repo.RevokeAllSessions(ctx, user.ID); rerr != nil {
			return nil, rerr
		}
		return nil, ErrAccountDisabled
	}
	return s.session(user, newRaw)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.repo.RevokeSession(ctx, HashRefreshToken(refreshToken))
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (*User, error) {
	claims, err := s.tokens.Parse(accessToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	user, err := s.repo.UserByID(ctx, claims.Subject)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}
	return user, nil
}

func (s *Service) User(ctx context.Context, id string) (*User, error) {
	return s.repo.UserByID(ctx, id)
}

func (s *Service) session(user *User, refresh string) (*Session, error) {
	token, err := s.tokens.Issue(user.ID, user.Roles)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: token, RefreshToken: refresh, User: user}, nil
}
