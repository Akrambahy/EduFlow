package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidRoles      = errors.New("roles must be a non-empty list of known roles")
	ErrSelfModification  = errors.New("you cannot change your own roles or activation status")
	ErrWrongPassword     = errors.New("current password is incorrect")
	ErrPasswordUnchanged = errors.New("new password must differ from the current password")
)

type Actor struct {
	ID string
	IP string
}

type CreateUserInput struct {
	Name       string
	Email      string
	Password   string
	University *string
	AvatarURL  *string
	Roles      []string
}

func NormalizeRoles(roles []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if !IsValidRole(r) {
			return nil, ErrInvalidRoles
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, ErrInvalidRoles
	}
	return SortRoles(out), nil
}

func (s *Service) ListUsers(ctx context.Context, p *Principal, f UserFilter, page httpx.Page) ([]User, int, error) {
	if !p.IsAdmin() {
		ids, err := s.visibleIDs(ctx, p)
		if err != nil {
			return nil, 0, err
		}
		f.Scoped = true
		f.IDs = ids
	}
	return s.repo.ListUsers(ctx, f, page)
}

func (s *Service) CanView(ctx context.Context, p *Principal, id string) (bool, error) {
	if p.IsAdmin() || p.UserID == id {
		return true, nil
	}
	ids, err := s.visibleIDs(ctx, p)
	if err != nil {
		return false, err
	}
	for _, v := range ids {
		if v == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) visibleIDs(ctx context.Context, p *Principal) ([]string, error) {
	if s.scoper == nil {
		return []string{p.UserID}, nil
	}
	return s.scoper.VisibleUserIDs(ctx, p.UserID, p.Roles)
}

func (s *Service) CreateUser(ctx context.Context, actor Actor, in CreateUserInput) (*User, error) {
	roles, err := NormalizeRoles(in.Roles)
	if err != nil {
		return nil, err
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	var id string
	err = s.repo.inTx(ctx, func(tx pgx.Tx) error {
		id, err = s.repo.insertUser(ctx, tx, NewUser{
			Name:         strings.TrimSpace(in.Name),
			Email:        NormalizeEmail(in.Email),
			PasswordHash: hash,
			University:   emptyToNil(in.University),
			AvatarURL:    emptyToNil(in.AvatarURL),
		})
		if err != nil {
			return err
		}
		if err := s.repo.replaceRoles(ctx, tx, id, roles, actor.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{
			ActorID: actor.ID, Action: audit.ActionUserCreated, TargetType: "user", TargetID: id,
			Context: map[string]any{"roles": roles}, IP: actor.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.UserByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, actor Actor, id string, p UserPatch) (*User, error) {
	if p.Email != nil {
		e := NormalizeEmail(*p.Email)
		p.Email = &e
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		p.Name = &n
	}
	if p.IsActive != nil && actor.ID == id {
		return nil, ErrSelfModification
	}

	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.repo.updateUser(ctx, tx, id, p); err != nil {
			return err
		}
		if fields := p.withoutActive().Fields(); len(fields) > 0 {
			if err := audit.Record(ctx, tx, audit.Event{
				ActorID: actor.ID, Action: audit.ActionUserUpdated, TargetType: "user", TargetID: id,
				Context: map[string]any{"fields": fields}, IP: actor.IP,
			}); err != nil {
				return err
			}
		}
		if p.IsActive != nil && *p.IsActive != current.IsActive {
			action := audit.ActionUserActivated
			if !*p.IsActive {
				action = audit.ActionUserDeactivated
				if err := s.repo.revokeAll(ctx, tx, id); err != nil {
					return err
				}
			}
			return audit.Record(ctx, tx, audit.Event{
				ActorID: actor.ID, Action: action, TargetType: "user", TargetID: id, IP: actor.IP,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.UserByID(ctx, id)
}

func (s *Service) UpdateProfile(ctx context.Context, actor Actor, p UserPatch) (*User, error) {
	return s.UpdateUser(ctx, actor, actor.ID, UserPatch{Name: p.Name, University: p.University, AvatarURL: p.AvatarURL})
}

func (s *Service) SetRoles(ctx context.Context, actor Actor, id string, roles []string) (*User, error) {
	if actor.ID == id {
		return nil, ErrSelfModification
	}
	normalized, err := NormalizeRoles(roles)
	if err != nil {
		return nil, err
	}
	err = s.repo.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if equalRoles(current.Roles, normalized) {
			return nil
		}
		if err := s.repo.replaceRoles(ctx, tx, id, normalized, actor.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{
			ActorID: actor.ID, Action: audit.ActionUserRolesChanged, TargetType: "user", TargetID: id,
			Context: map[string]any{"before": current.Roles, "after": normalized}, IP: actor.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.UserByID(ctx, id)
}

func (s *Service) ResetPassword(ctx context.Context, actor Actor, id, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.lockUser(ctx, tx, id); err != nil {
			return err
		}
		if err := s.repo.setPassword(ctx, tx, id, hash); err != nil {
			return err
		}
		if err := s.repo.revokeAll(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{
			ActorID: actor.ID, Action: audit.ActionUserPasswordReset, TargetType: "user", TargetID: id, IP: actor.IP,
		})
	})
}

func (s *Service) ChangePassword(ctx context.Context, actor Actor, current, next string) error {
	if current == next {
		return ErrPasswordUnchanged
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	return s.repo.inTx(ctx, func(tx pgx.Tx) error {
		u, err := s.repo.lockUser(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		if !CheckPassword(u.PasswordHash, current) {
			return ErrWrongPassword
		}
		if err := s.repo.setPassword(ctx, tx, actor.ID, hash); err != nil {
			return err
		}
		if err := s.repo.revokeAll(ctx, tx, actor.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{
			ActorID: actor.ID, Action: audit.ActionPasswordChanged, TargetType: "user", TargetID: actor.ID, IP: actor.IP,
		})
	})
}

func (p UserPatch) withoutActive() UserPatch {
	p.IsActive = nil
	return p
}

func equalRoles(a, b []string) bool {
	a, b = SortRoles(a), SortRoles(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func emptyToNil(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
