import { useState } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { motion } from "motion/react";
import {
  ArrowDownRight,
  ArrowRight,
  Award,
  BookOpen,
  ClipboardCheck,
  Clock3,
  FileText,
  FolderKanban,
  Play,
  TrendingUp,
  Users,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses, demoEnrollments } from "../data/courses.js";
import { demoSessions } from "../data/sessions.js";
import { ROLES } from "../utils/constants.js";
import {
  Avatar,
  CourseGlyph,
  PageIntro,
  SectionHeading,
  TaskItem,
} from "../components/common/index.js";

const roleHome = {
  student: "Your learning, in focus.",
  instructor: "Make every lesson count.",
  mentor: "Small steps. Real progress.",
  tracker: "See the whole cohort clearly.",
  admin: "A healthier learning program starts here.",
};

export default function DashboardPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const role = user.role;
  const isStudent = role === ROLES.STUDENT;
  const [progressFilter, setProgressFilter] = useState("All");
  const [completedTasks, setCompletedTasks] = useState({
    task_feedback: true,
  });

  const allUserCourses = demoCourses.filter((course) =>
    isStudent
      ? user.enrolledCourses?.includes(course.id)
      : course.status === "published",
  );

  const enrichedCourses = allUserCourses.map((course, index) => {
    const enrollment = demoEnrollments.find(
      (item) => item.studentId === user.id && item.courseId === course.id,
    );
    const progress = enrollment?.progress ?? [68, 42, 81, 15][index % 4];
    return { ...course, progress };
  });

  const filteredCourses = enrichedCourses.filter((course) => {
    if (progressFilter === "In progress") {
      return course.progress > 0 && course.progress < 100;
    }
    if (progressFilter === "Needs attention") {
      return course.progress < 40;
    }
    return true;
  });

  const primaryCourse =
    enrichedCourses.find((c) => c.progress > 0 && c.progress < 100) ||
    enrichedCourses[0];

  const firstName = user.name.split(" ")[0];
  const stats = isStudent
    ? [
        {
          label: "Active courses",
          value: "04",
          note: "+1 enrolled this term",
          icon: BookOpen,
          tone: "teal",
        },
        {
          label: "Hours logged",
          value: "28.5",
          note: "Across 4 active modules",
          icon: Clock3,
          tone: "lilac",
        },
        {
          label: "Academic standing",
          value: "Good",
          note: "84.7% weighted average",
          icon: TrendingUp,
          tone: "sand",
        },
      ]
    : role === ROLES.INSTRUCTOR
      ? [
          {
            label: "Assigned courses",
            value: "03",
            note: "Across 2 active tracks",
            icon: BookOpen,
            tone: "teal",
          },
          {
            label: "Pending review",
            value: "08",
            note: "Submissions awaiting grade",
            icon: ClipboardCheck,
            tone: "lilac",
          },
          {
            label: "Next live session",
            value: "10:00",
            note: "React Workshop · Room 301",
            icon: Clock3,
            tone: "sand",
          },
        ]
      : [
          {
            label: "Active tracks",
            value: role === ROLES.ADMIN ? "03" : "02",
            note: "Fall 2026 cohort",
            icon: FolderKanban,
            tone: "teal",
          },
          {
            label: "Active learners",
            value: role === ROLES.ADMIN ? "128" : "42",
            note: "In your assigned scope",
            icon: Users,
            tone: "lilac",
          },
          {
            label: "Needs check-in",
            value: "06",
            note: "Below 75% attendance",
            icon: ArrowDownRight,
            tone: "sand",
          },
        ];

  const welcomeSub = isStudent
    ? "Resume your active lesson or review upcoming deadlines."
    : role === ROLES.INSTRUCTOR
      ? "Review pending submissions and prepare for today’s workshop."
      : "Monitor cohort progress, attendance, and active learning tracks.";

  const nextSession = demoSessions.find(
    (session) => session.status === "upcoming",
  );

  function toggleTask(id) {
    setCompletedTasks((prev) => ({ ...prev, [id]: !prev[id] }));
  }

  return (
    <>
      <PageIntro
        kicker={`${new Intl.DateTimeFormat("en", { weekday: "long", month: "long", day: "numeric" }).format(new Date()).toUpperCase()} · FALL SEMESTER`}
        title={`Good morning, ${firstName}.`}
        subtitle={welcomeSub}
      />

      <section className="welcome-banner">
        <div className="welcome-copy">
          <span className="eyebrow light">ACTIVE WORKSPACE · FALL 2026</span>
          <h2>{roleHome[role]}</h2>
          <p>
            {isStudent
              ? "Pick up directly where you left off in your current module, or review your upcoming deliverables."
              : role === ROLES.INSTRUCTOR
                ? "Your next live session starts at 10:00 AM in Room 301."
                : "All cohort metrics and course completion rates are up to date."}
          </p>
          <div className="banner-actions">
            {isStudent && primaryCourse ? (
              <button
                type="button"
                className="button button-light"
                onClick={() =>
                  navigate(`/courses/${primaryCourse.id}?resume=1`)
                }
              >
                <Play size={15} /> Resume lesson
              </button>
            ) : (
              <NavLink to="/schedule" className="button button-light">
                Open schedule <ArrowRight size={15} />
              </NavLink>
            )}
            <NavLink to="/assignments" className="button button-dark">
              View assignments <ArrowRight size={15} />
            </NavLink>
          </div>
        </div>

        <div className="focus-panel">
          <div className="focus-panel-top">
            <span>CURRENT MODULE</span>
            <span>{primaryCourse?.category.toUpperCase() || "FRONTEND"}</span>
          </div>
          <strong>{primaryCourse?.title || "React Fundamentals"}</strong>
          <small>
            {isStudent
              ? "Lesson 4: Understanding Components · 18 min"
              : "Cohort completion across active modules"}
          </small>
          <div className="focus-meter">
            <div className="focus-meter-track">
              <i style={{ width: `${primaryCourse?.progress ?? 33}%` }} />
            </div>
            <span>{primaryCourse?.progress ?? 33}%</span>
          </div>
        </div>
      </section>

      <section className="stats-grid" aria-label="At a glance">
        {stats.map(({ label, value, note, icon: Icon, tone }, index) => (
          <motion.article
            className="stat-card"
            key={label}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.05 }}
          >
            <span className={`stat-icon stat-${tone}`}>
              <Icon size={18} />
            </span>
            <span className="stat-label">{label}</span>
            <strong className="stat-value">{value}</strong>
            <span className="stat-note">{note}</span>
          </motion.article>
        ))}
      </section>

      <div className="dashboard-grid">
        <section className="section-block course-section">
          <SectionHeading
            title={
              isStudent ? "Pick up where you left off" : "Courses in your space"
            }
            subtitle={
              isStudent
                ? "Filter by progress or jump straight into your next lesson."
                : "Active courses across the current semester."
            }
            action={
              <NavLink to="/courses" className="text-link">
                All courses <ArrowRight size={14} />
              </NavLink>
            }
          />
          <div className="dashboard-course-toolbar">
            <div className="filter-tabs" role="tablist" aria-label="Filter courses by progress">
              {["All", "In progress", "Needs attention"].map((tabName) => (
                <button
                  type="button"
                  key={tabName}
                  className={`filter-tab ${progressFilter === tabName ? "active" : ""}`}
                  onClick={() => setProgressFilter(tabName)}
                >
                  {tabName}
                </button>
              ))}
            </div>
            <span className="quiet-label">
              {filteredCourses.length} course{filteredCourses.length === 1 ? "" : "s"}
            </span>
          </div>
          <div className="course-list">
            {filteredCourses.slice(0, 4).map((course) => (
              <div className="course-row" key={course.id}>
                <div className={`course-thumb thumb-${course.color}`}>
                  <CourseGlyph category={course.category} />
                </div>
                <div className="course-main">
                  <div className="course-meta">
                    <span>{course.category}</span>
                    <span className="meta-dot" />
                    <span>{course.duration}</span>
                  </div>
                  <h3>
                    <NavLink to={`/courses/${course.id}`}>
                      {course.title}
                    </NavLink>
                  </h3>
                  <div className="progress-line">
                    <span>
                      <i style={{ width: `${course.progress}%` }} />
                    </span>
                    <small>{course.progress}%</small>
                  </div>
                </div>
                <div className="course-row-actions">
                  <button
                    type="button"
                    className="button button-outline button-sm"
                    onClick={() => navigate(`/courses/${course.id}?resume=1`)}
                  >
                    <Play size={13} />
                    {course.progress > 0 ? "Resume lesson" : "Start course"}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>

        <aside className="right-rail">
          <section className="section-block session-card">
            <SectionHeading
              title={isStudent ? "Coming up" : "Next session"}
              action={
                <NavLink
                  to="/schedule"
                  className="icon-link"
                  aria-label="See schedule"
                >
                  <ArrowRight size={15} />
                </NavLink>
              }
            />
            <div className="session-date">
              <span className="date-month">
                {new Intl.DateTimeFormat("en", { month: "short" })
                  .format(new Date(`${nextSession.date}T12:00:00`))
                  .toUpperCase()}
              </span>
              <strong>
                {new Intl.DateTimeFormat("en", { day: "2-digit" }).format(
                  new Date(`${nextSession.date}T12:00:00`),
                )}
              </strong>
            </div>
            <div className="session-info">
              <span className="session-tag">
                {nextSession.type} · {nextSession.location}
              </span>
              <h3>{nextSession.title}</h3>
              <p>
                {new Intl.DateTimeFormat("en", { weekday: "long" }).format(
                  new Date(`${nextSession.date}T12:00:00`),
                )}
                , {nextSession.startTime} – {nextSession.endTime}
              </p>
              <span className="instructor-line">
                <Avatar name="Ahmed Khalil" size="xs" /> Ahmed Khalil
              </span>
            </div>
            <NavLink to="/schedule" className="session-footer">
              Open session & check in <ArrowRight size={14} />
            </NavLink>
          </section>

          <section className="section-block task-card">
            <SectionHeading
              title="Action checklist"
              subtitle="Click to mark complete"
            />
            <div className="task-list">
              <TaskItem
                icon={FileText}
                title="Build a Todo App"
                detail="React Fundamentals · due Nov 15"
                tone="teal"
                done={!!completedTasks.task_todo}
                onToggle={() => toggleTask("task_todo")}
              />
              <TaskItem
                icon={ClipboardCheck}
                title="React Basics Quiz"
                detail="5 questions · 15 min time limit"
                tone="peach"
                done={!!completedTasks.task_quiz}
                onToggle={() => toggleTask("task_quiz")}
              />
              <TaskItem
                icon={Award}
                title="Review graded feedback"
                detail="Component Library · 88/100"
                tone="lilac"
                done={!!completedTasks.task_feedback}
                onToggle={() => toggleTask("task_feedback")}
              />
            </div>
            <NavLink to="/assignments" className="task-link">
              View all assignments <ArrowRight size={14} />
            </NavLink>
          </section>
        </aside>
      </div>
    </>
  );
}
