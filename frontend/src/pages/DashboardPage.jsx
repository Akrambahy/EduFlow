import { NavLink } from "react-router-dom";
import { motion } from "motion/react";
import {
  ArrowDownRight,
  ArrowRight,
  ArrowUpRight,
  Award,
  BookOpen,
  ClipboardCheck,
  Clock3,
  FileText,
  FolderKanban,
  MoreHorizontal,
  Sparkles,
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
  const role = user.role;
  const isStudent = role === ROLES.STUDENT;
  const courses = demoCourses.filter((course) =>
    isStudent
      ? user.enrolledCourses?.includes(course.id)
      : course.status === "published",
  );
  const firstName = user.name.split(" ")[0];
  const stats = isStudent
    ? [
        {
          label: "Active courses",
          value: "04",
          note: "+1 this term",
          icon: BookOpen,
          tone: "teal",
        },
        {
          label: "Hours learned",
          value: "28.5",
          note: "Across all courses",
          icon: Clock3,
          tone: "lilac",
        },
        {
          label: "Current standing",
          value: "Good",
          note: "On track this week",
          icon: TrendingUp,
          tone: "sand",
        },
      ]
    : role === ROLES.INSTRUCTOR
      ? [
          {
            label: "Assigned courses",
            value: "03",
            note: "Across 2 tracks",
            icon: BookOpen,
            tone: "teal",
          },
          {
            label: "To review",
            value: "08",
            note: "Submissions pending",
            icon: ClipboardCheck,
            tone: "lilac",
          },
          {
            label: "Next session",
            value: "10:00",
            note: "React · today",
            icon: Clock3,
            tone: "sand",
          },
        ]
      : [
          {
            label: "Active tracks",
            value: role === ROLES.ADMIN ? "03" : "02",
            note: "Currently running",
            icon: FolderKanban,
            tone: "teal",
          },
          {
            label: "Learners",
            value: role === ROLES.ADMIN ? "128" : "42",
            note: "In your assigned scope",
            icon: Users,
            tone: "lilac",
          },
          {
            label: "Need attention",
            value: "06",
            note: "Across current cohorts",
            icon: ArrowDownRight,
            tone: "sand",
          },
        ];
  const welcomeSub = isStudent
    ? "You’re building a good rhythm. Keep it going."
    : role === ROLES.INSTRUCTOR
      ? "Here’s what your learners need from you today."
      : "Here’s a clear view of your learning community today.";
  const nextSession = demoSessions.find(
    (session) => session.status === "upcoming",
  );

  return (
    <>
      <PageIntro
        kicker={`${new Intl.DateTimeFormat("en", { weekday: "long", month: "long", day: "numeric" }).format(new Date())} · FALL SEMESTER`}
        title={`Good morning, ${firstName}.`}
        subtitle={welcomeSub}
      />
      <section className="welcome-banner">
        <div className="welcome-copy">
          <span className="eyebrow light">
            <Sparkles size={13} /> YOUR LEARNING, IN FOCUS
          </span>
          <h2>{roleHome[role]}</h2>
          <p>
            {isStudent
              ? "Make today count with one small step toward your goals."
              : role === ROLES.INSTRUCTOR
                ? "Your next session is ready when you are."
                : "A thoughtful overview of the work happening across your space."}
          </p>
          <NavLink
            to={isStudent ? "/courses" : "/schedule"}
            className="button button-light"
          >
            {isStudent ? "Continue learning" : "View schedule"}
            <ArrowRight size={16} />
          </NavLink>
        </div>
        <div className="banner-art" aria-hidden="true">
          <div className="banner-sun" />
          <div className="banner-ring ring-a" />
          <div className="banner-ring ring-b" />
          <div className="banner-note">
            <span>WEEKLY FOCUS</span>
            <strong>Keep showing up.</strong>
            <div className="mini-bars">
              <i />
              <i />
              <i />
              <i />
              <i />
              <i />
              <i />
            </div>
          </div>
          <span className="banner-index">EF · 01</span>
        </div>
      </section>
      <section className="stats-grid" aria-label="At a glance">
        {stats.map(({ label, value, note, icon: Icon, tone }, index) => (
          <motion.article
            className="stat-card"
            key={label}
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.06 }}
          >
            <span className={`stat-icon stat-${tone}`}>
              <Icon size={17} />
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
                ? "A little progress goes a long way."
                : "The learning experiences you support."
            }
            action={
              <NavLink to="/courses" className="text-link">
                All courses <ArrowRight size={14} />
              </NavLink>
            }
          />
          <div className="course-list">
            {courses.slice(0, 3).map((course, index) => {
              const enrollment = demoEnrollments.find(
                (item) =>
                  item.studentId === user.id && item.courseId === course.id,
              );
              const progress = enrollment?.progress ?? [68, 42, 81][index];
              return (
                <NavLink
                  to={`/courses/${course.id}`}
                  className="course-row"
                  key={course.id}
                >
                  <div className={`course-thumb thumb-${course.color}`}>
                    <CourseGlyph category={course.category} />
                  </div>
                  <div className="course-main">
                    <div className="course-meta">
                      <span>{course.category}</span>
                      <span className="meta-dot" />
                      {course.duration}
                    </div>
                    <h3>{course.title}</h3>
                    <div className="progress-line">
                      <span>
                        <i style={{ width: `${progress}%` }} />
                      </span>
                      <small>{progress}%</small>
                    </div>
                  </div>
                  <span className="course-row-arrow">
                    <ArrowUpRight size={17} />
                  </span>
                </NavLink>
              );
            })}
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
              Session details <ArrowRight size={14} />
            </NavLink>
          </section>
          <section className="section-block task-card">
            <SectionHeading
              title="A few next steps"
              action={<MoreHorizontal size={17} className="quiet-icon" />}
            />
            <div className="task-list">
              <TaskItem
                icon={FileText}
                title="Build a Todo App"
                detail="React Fundamentals · due Nov 15"
                tone="teal"
              />
              <TaskItem
                icon={ClipboardCheck}
                title="React Basics Quiz"
                detail="5 questions · about 15 min"
                tone="peach"
              />
              <TaskItem
                icon={Award}
                title="Review your feedback"
                detail="Component Library · graded"
                tone="lilac"
                done
              />
            </div>
            <NavLink to="/assignments" className="task-link">
              View all work <ArrowRight size={14} />
            </NavLink>
          </section>
        </aside>
      </div>
    </>
  );
}
