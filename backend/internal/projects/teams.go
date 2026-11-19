package projects

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

type teamRef struct {
	ProjectID            string
	CourseID             string
	TeamSize             int
	RequiresPresentation bool
}

func (s *Service) teamRef(ctx context.Context, db database.DBTX, teamID string, lock bool) (*teamRef, error) {
	q := `SELECT t.project_id, p.course_id, p.team_size, p.requires_presentation FROM project_teams t JOIN projects p ON p.id = t.project_id WHERE t.id = $1`
	if lock {
		q += ` FOR UPDATE OF t`
	}
	var r teamRef
	err := db.QueryRow(ctx, q, teamID).Scan(&r.ProjectID, &r.CourseID, &r.TeamSize, &r.RequiresPresentation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

func (s *Service) isMember(ctx context.Context, db database.DBTX, teamID, userID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_team_members WHERE team_id = $1 AND student_id = $2)`, teamID, userID).Scan(&ok)
	return ok, err
}

func (s *Service) loadTeams(ctx context.Context, projectID, teamID string) ([]Team, error) {
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.project_id, t.name,
			COALESCE((SELECT jsonb_agg(jsonb_build_object('id', u.id, 'name', u.name) ORDER BY u.name)
				FROM project_team_members m JOIN users u ON u.id = m.student_id WHERE m.team_id = t.id), '[]'::jsonb),
			g.url, g.owner, g.name, g.linked_at, g.metadata, g.last_synced_at, g.sync_error,
			pr.id, pr.scheduled_at, pr.location, pr.status, pr.feedback
		FROM project_teams t
		LEFT JOIN github_repositories g ON g.team_id = t.id
		LEFT JOIN presentations pr ON pr.team_id = t.id
		WHERE ($1::uuid IS NULL OR t.project_id = $1::uuid) AND ($2::uuid IS NULL OR t.id = $2::uuid)
		ORDER BY t.name, t.id LIMIT 500`, nullable(projectID), nullable(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Team{}
	for rows.Next() {
		var t Team
		var members []byte
		var url, owner, name, syncErr *string
		var linkedAt, synced *time.Time
		var metadata []byte
		var prID, prLocation, prStatus, prFeedback *string
		var prAt *time.Time
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &members, &url, &owner, &name, &linkedAt, &metadata, &synced, &syncErr,
			&prID, &prAt, &prLocation, &prStatus, &prFeedback); err != nil {
			return nil, err
		}
		t.Members = []Member{}
		_ = json.Unmarshal(members, &t.Members)
		if url != nil {
			r := &Repository{URL: *url, Owner: *owner, Name: *name, LinkedAt: *linkedAt, LastSyncedAt: synced, SyncError: syncErr}
			if len(metadata) > 0 {
				var m RepoMetadata
				if json.Unmarshal(metadata, &m) == nil {
					r.Metadata = &m
				}
			}
			t.Repository = r
		}
		if prID != nil {
			d, tm := s.local(*prAt)
			t.Presentation = &Presentation{ID: *prID, ProjectID: t.ProjectID, TeamID: t.ID, ScheduledAt: prAt.UTC(),
				PresentationDate: d, PresentationTime: tm, Location: *prLocation, Status: *prStatus, Feedback: prFeedback}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) Teams(ctx context.Context, v learning.Viewer, projectID string) ([]Team, error) {
	if _, err := s.Project(ctx, v, projectID); err != nil {
		return nil, err
	}
	return s.loadTeams(ctx, projectID, "")
}

func (s *Service) team(ctx context.Context, teamID string) (*Team, error) {
	list, err := s.loadTeams(ctx, "", teamID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

func (s *Service) createTeam(ctx context.Context, tx pgx.Tx, projectID, name, studentID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO project_teams (project_id, name, created_by) VALUES ($1, $2, $3) RETURNING id`,
		projectID, name, studentID).Scan(&id)
	if database.IsUniqueViolation(err, "project_teams_name_key") {
		return "", ErrTeamNameTaken
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_team_members (team_id, project_id, student_id) VALUES ($1, $2, $3)`, id, projectID, studentID)
	if database.IsUniqueViolation(err, "project_team_members_one_team") {
		return "", ErrAlreadyInTeam
	}
	return id, err
}

func (s *Service) CreateTeam(ctx context.Context, v learning.Viewer, projectID, name string) (*Team, error) {
	if !v.Has(identity.RoleStudent) {
		return nil, ErrForbidden
	}
	p, err := s.Project(ctx, v, projectID)
	if err != nil {
		return nil, err
	}
	if !p.Published {
		return nil, ErrProjectNotOpen
	}
	var id string
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		ok, err := s.enrolled(ctx, tx, v.UserID, p.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEnrolled
		}
		id, err = s.createTeam(ctx, tx, projectID, strings.TrimSpace(name), v.UserID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.team(ctx, id)
}

func (s *Service) hasSubmission(ctx context.Context, db database.DBTX, teamID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_submissions WHERE team_id = $1)`, teamID).Scan(&ok)
	return ok, err
}

func (s *Service) AddMember(ctx context.Context, v learning.Viewer, teamID, studentID string) (*Team, error) {
	ref, err := s.teamRef(ctx, s.db, teamID, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.Project(ctx, v, ref.ProjectID); err != nil {
		return nil, err
	}
	if studentID == "" || studentID == v.UserID {
		if !v.Has(identity.RoleStudent) {
			return nil, ErrForbidden
		}
		studentID = v.UserID
	} else {
		ok, err := s.canManage(ctx, v, ref.CourseID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrForbidden
		}
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.teamRef(ctx, tx, teamID, true)
		if err != nil {
			return err
		}
		ok, err := s.enrolled(ctx, tx, studentID, locked.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEnrolled
		}
		submitted, err := s.hasSubmission(ctx, tx, teamID)
		if err != nil {
			return err
		}
		if submitted {
			return ErrTeamLocked
		}
		var size int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM project_team_members WHERE team_id = $1`, teamID).Scan(&size); err != nil {
			return err
		}
		if size >= locked.TeamSize {
			return ErrTeamFull
		}
		_, err = tx.Exec(ctx, `INSERT INTO project_team_members (team_id, project_id, student_id) VALUES ($1, $2, $3)`, teamID, locked.ProjectID, studentID)
		if database.IsUniqueViolation(err, "") {
			return ErrAlreadyInTeam
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.team(ctx, teamID)
}

func (s *Service) RemoveMember(ctx context.Context, v learning.Viewer, teamID, studentID string) error {
	ref, err := s.teamRef(ctx, s.db, teamID, false)
	if err != nil {
		return err
	}
	if studentID != v.UserID {
		ok, err := s.canManage(ctx, v, ref.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			if _, perr := s.Project(ctx, v, ref.ProjectID); perr != nil {
				return perr
			}
			return ErrForbidden
		}
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.teamRef(ctx, tx, teamID, true); err != nil {
			return err
		}
		submitted, err := s.hasSubmission(ctx, tx, teamID)
		if err != nil {
			return err
		}
		if submitted {
			return ErrTeamLocked
		}
		tag, err := tx.Exec(ctx, `DELETE FROM project_team_members WHERE team_id = $1 AND student_id = $2`, teamID, studentID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM project_teams t WHERE t.id = $1
			AND NOT EXISTS (SELECT 1 FROM project_team_members WHERE team_id = t.id)
			AND NOT EXISTS (SELECT 1 FROM presentations WHERE team_id = t.id)
			AND NOT EXISTS (SELECT 1 FROM github_repositories WHERE team_id = t.id)`, teamID)
		return err
	})
}

func (s *Service) SchedulePresentation(ctx context.Context, v learning.Viewer, teamID string, at time.Time, location string) (*Team, error) {
	ref, err := s.teamRef(ctx, s.db, teamID, false)
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, ref.CourseID); err != nil {
		return nil, err
	}
	if !ref.RequiresPresentation {
		return nil, ErrPresentationNotNeeded
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO presentations (project_id, team_id, scheduled_at, location, updated_by) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (team_id) DO UPDATE SET scheduled_at = EXCLUDED.scheduled_at, location = EXCLUDED.location,
				status = 'scheduled', updated_by = EXCLUDED.updated_by`, ref.ProjectID, teamID, at, location, v.UserID); err != nil {
			return err
		}
		return record(ctx, tx, v, "presentation.scheduled", "team", teamID, map[string]any{"scheduledAt": at})
	})
	if err != nil {
		return nil, err
	}
	return s.team(ctx, teamID)
}

func (s *Service) UpdatePresentation(ctx context.Context, v learning.Viewer, id string, status, feedback *string) (*Team, error) {
	var teamID, courseID string
	err := s.db.QueryRow(ctx, `SELECT pr.team_id, p.course_id FROM presentations pr JOIN projects p ON p.id = pr.project_id WHERE pr.id = $1`, id).Scan(&teamID, &courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE presentations SET status = COALESCE($2, status), feedback = COALESCE($3, feedback), updated_by = $4 WHERE id = $1`,
			id, status, feedback, v.UserID); err != nil {
			return err
		}
		return record(ctx, tx, v, "presentation.updated", "presentation", id, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.team(ctx, teamID)
}

func (s *Service) canTouchTeam(ctx context.Context, v learning.Viewer, teamID string) (*teamRef, error) {
	ref, err := s.teamRef(ctx, s.db, teamID, false)
	if err != nil {
		return nil, err
	}
	member, err := s.isMember(ctx, s.db, teamID, v.UserID)
	if err != nil {
		return nil, err
	}
	if member {
		return ref, nil
	}
	staff, err := s.isStaff(ctx, v, ref.CourseID)
	if err != nil {
		return nil, err
	}
	if staff {
		return ref, nil
	}
	if _, err := s.Project(ctx, v, ref.ProjectID); err != nil {
		return nil, err
	}
	return nil, ErrForbidden
}

func (s *Service) LinkRepository(ctx context.Context, v learning.Viewer, teamID, rawURL string) (*Team, error) {
	owner, name, canonical, err := ParseRepoURL(rawURL)
	if err != nil {
		return nil, err
	}
	if _, err := s.canTouchTeam(ctx, v, teamID); err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO github_repositories (team_id, owner, name, url, linked_by) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (team_id) DO UPDATE SET owner = EXCLUDED.owner, name = EXCLUDED.name, url = EXCLUDED.url,
				linked_by = EXCLUDED.linked_by, linked_at = now(), metadata = NULL, last_synced_at = NULL, sync_error = NULL`,
			teamID, owner, name, canonical, v.UserID); err != nil {
			return err
		}
		return record(ctx, tx, v, "github.repository_linked", "team", teamID, map[string]any{"url": canonical})
	})
	if err != nil {
		return nil, err
	}
	s.sync(ctx, teamID, owner, name)
	return s.team(ctx, teamID)
}

func (s *Service) SyncRepository(ctx context.Context, v learning.Viewer, teamID string) (*Team, error) {
	if _, err := s.canTouchTeam(ctx, v, teamID); err != nil {
		return nil, err
	}
	var owner, name string
	err := s.db.QueryRow(ctx, `SELECT owner, name FROM github_repositories WHERE team_id = $1`, teamID).Scan(&owner, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.sync(ctx, teamID, owner, name)
	return s.team(ctx, teamID)
}

func (s *Service) sync(ctx context.Context, teamID, owner, name string) {
	if s.github == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	meta, err := s.github.Repository(cctx, owner, name)
	if err != nil {
		msg := "GitHub is unavailable or the repository is not accessible"
		_, _ = s.db.Exec(ctx, `UPDATE github_repositories SET sync_error = $2, last_synced_at = now() WHERE team_id = $1`, teamID, msg)
		return
	}
	b, _ := json.Marshal(meta)
	_, _ = s.db.Exec(ctx, `UPDATE github_repositories SET metadata = $2, sync_error = NULL, last_synced_at = now() WHERE team_id = $1`, teamID, b)
}

func (s *Service) TeamForProject(ctx context.Context, v learning.Viewer, teamID string) (*Team, error) {
	ref, err := s.teamRef(ctx, s.db, teamID, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.Project(ctx, v, ref.ProjectID); err != nil {
		return nil, err
	}
	return s.team(ctx, teamID)
}
