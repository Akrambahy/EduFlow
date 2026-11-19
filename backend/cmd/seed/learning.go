package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type demoLesson struct {
	title   string
	kind    string
	minutes int
}

type demoModule struct {
	title   string
	lessons []demoLesson
}

type demoCourse struct {
	key         string
	title       string
	description string
	category    string
	color       string
	weeks       int
	status      string
	track       string
	sequential  bool
	modules     []demoModule
	completed   int
	inProgress  bool
}

var demoTracks = []struct {
	name        string
	description string
}{
	{"Full-Stack Development", "Master both frontend and backend development with modern technologies."},
	{"Frontend Engineering", "Build beautiful, accessible, and performant user interfaces."},
	{"Backend Engineering", "Design and build scalable server-side applications and APIs."},
}

var reactModules = []demoModule{
	{"Getting Started with React", []demoLesson{
		{"Introduction to React", "video", 15}, {"Setting Up Your Environment", "text", 10}, {"Your First Component", "video", 20},
	}},
	{"Components & Props", []demoLesson{
		{"Understanding Components", "video", 18}, {"Props and Data Flow", "text", 12}, {"Component Composition", "video", 22},
	}},
	{"State & Hooks", []demoLesson{
		{"useState Hook", "video", 20}, {"useEffect Hook", "video", 25}, {"Custom Hooks", "text", 15},
	}},
	{"Routing & Navigation", []demoLesson{
		{"React Router Setup", "video", 18}, {"Dynamic Routes", "text", 12}, {"Final Project", "project", 45},
	}},
}

func genericModules(prefix string, modules, lessons int) []demoModule {
	out := make([]demoModule, 0, modules)
	n := 0
	for m := 1; m <= modules; m++ {
		mod := demoModule{title: fmt.Sprintf("%s: Part %d", prefix, m)}
		per := lessons / modules
		if m <= lessons%modules {
			per++
		}
		for l := 1; l <= per; l++ {
			n++
			kind := "video"
			if l%2 == 0 {
				kind = "text"
			}
			mod.lessons = append(mod.lessons, demoLesson{fmt.Sprintf("%s lesson %d", prefix, n), kind, 15})
		}
		out = append(out, mod)
	}
	return out
}

var demoCourses = []demoCourse{
	{"react", "React Fundamentals", "Learn to build dynamic user interfaces with React. Covers components, state, hooks, and routing.",
		"Frontend", "teal", 6, "published", "Full-Stack Development", true, reactModules, 3, true},
	{"go", "Go Backend Basics", "Build robust backend services with Go. Learn the standard library, concurrency, and web servers.",
		"Backend", "emerald", 8, "published", "Backend Engineering", false, genericModules("Go", 5, 15), 3, false},
	{"api", "RESTful API Design", "Design and implement clean, scalable REST APIs following industry best practices.",
		"Backend", "sky", 4, "published", "Backend Engineering", false, genericModules("API", 3, 8), 0, false},
	{"db", "Database Foundations", "Understand relational databases, SQL, schema design, and query optimization.",
		"Data", "amber", 5, "published", "Backend Engineering", false, genericModules("SQL", 4, 10), 6, false},
	{"css", "Modern CSS & Layout", "Master CSS Grid, Flexbox, animations, and responsive design patterns.",
		"Frontend", "violet", 3, "published", "Frontend Engineering", false, genericModules("CSS", 2, 6), 0, false},
	{"testing", "Testing Strategies", "Learn unit testing, integration testing, and test-driven development workflows.",
		"Quality", "rose", 3, "draft", "Frontend Engineering", false, genericModules("Testing", 2, 6), 0, false},
}

var studentCourses = map[string]bool{"react": true, "go": true, "db": true, "api": true}

func seedLearning(ctx context.Context, tx pgx.Tx) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tracks WHERE lower(name) = lower($1))`, demoTracks[0].name).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	users := map[string]string{}
	for _, email := range []string{"admin@eduflow.dev", "instructor@eduflow.dev", "mentor@eduflow.dev", "tracker@eduflow.dev", "student@eduflow.dev"} {
		var id string
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id); err != nil {
			return false, fmt.Errorf("demo user %s missing: %w", email, err)
		}
		users[email] = id
	}
	admin := users["admin@eduflow.dev"]

	tracks := map[string]string{}
	for _, t := range demoTracks {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO tracks (name, description, status, created_by) VALUES ($1, $2, 'active', $3) RETURNING id`,
			t.name, t.description, admin).Scan(&id); err != nil {
			return false, err
		}
		tracks[t.name] = id
		if _, err := tx.Exec(ctx, `INSERT INTO track_mentors (track_id, user_id, assigned_by) VALUES ($1, $2, $3)`, id, users["mentor@eduflow.dev"], admin); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO track_trackers (track_id, user_id, assigned_by) VALUES ($1, $2, $3)`, id, users["tracker@eduflow.dev"], admin); err != nil {
			return false, err
		}
	}

	student := users["student@eduflow.dev"]
	for _, c := range demoCourses {
		var courseID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO courses (track_id, instructor_id, title, description, category, color, duration_weeks,
				sequential_unlock, status, published_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CASE WHEN $9 = 'published' THEN now() END, $10)
			RETURNING id`,
			tracks[c.track], users["instructor@eduflow.dev"], c.title, c.description, c.category, c.color, c.weeks,
			c.sequential, c.status, admin).Scan(&courseID); err != nil {
			return false, err
		}

		var lessonIDs []string
		for mi, m := range c.modules {
			var moduleID string
			if err := tx.QueryRow(ctx, `INSERT INTO modules (course_id, title, position) VALUES ($1, $2, $3) RETURNING id`,
				courseID, m.title, mi+1).Scan(&moduleID); err != nil {
				return false, err
			}
			for li, l := range m.lessons {
				var lessonID string
				if err := tx.QueryRow(ctx, `
					INSERT INTO lessons (module_id, title, type, duration_minutes, content, position)
					VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
					moduleID, l.title, l.kind, l.minutes, "Demo content for "+l.title+".", li+1).Scan(&lessonID); err != nil {
					return false, err
				}
				lessonIDs = append(lessonIDs, lessonID)
			}
		}

		if !studentCourses[c.key] {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO enrollments (student_id, course_id, enrolled_by) VALUES ($1, $2, $3)`, student, courseID, admin); err != nil {
			return false, err
		}
		for i := 0; i < c.completed && i < len(lessonIDs); i++ {
			if _, err := tx.Exec(ctx, `INSERT INTO lesson_progress (student_id, lesson_id, completed_at) VALUES ($1, $2, now())`, student, lessonIDs[i]); err != nil {
				return false, err
			}
		}
		if c.inProgress && c.completed < len(lessonIDs) {
			if _, err := tx.Exec(ctx, `INSERT INTO lesson_progress (student_id, lesson_id) VALUES ($1, $2)`, student, lessonIDs[c.completed]); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
