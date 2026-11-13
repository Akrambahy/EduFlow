import { useState } from "react";
import { motion } from "motion/react";
import {
  ArrowRight,
  ArrowUpRight,
  CalendarDays,
  Clock3,
  Plus,
  Sparkles,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses } from "../data/courses.js";
import { demoSessions } from "../data/sessions.js";
import { ROLES } from "../utils/constants.js";
import { Avatar, PageIntro } from "../components/common/index.js";

export default function SchedulePage() {
  const { user } = useAuth();
  const [selected, setSelected] = useState("Upcoming");
  const items = demoSessions.filter((session) =>
    selected === "Upcoming"
      ? session.status === "upcoming"
      : session.status === "completed",
  );
  return (
    <>
      <PageIntro
        kicker="MAKE SPACE TO LEARN"
        title="Schedule"
        subtitle="Sessions, milestones, and time well spent."
        action={
          user.role === ROLES.INSTRUCTOR || user.role === ROLES.ADMIN ? (
            <button
              className="button button-dark"
              onClick={() =>
                alert(
                  "Session creation will connect to the Go API when its contract is ready.",
                )
              }
            >
              <Plus size={16} /> New session
            </button>
          ) : null
        }
      />
      <div className="toolbar">
        <div className="filter-tabs">
          {["Upcoming", "Past sessions"].map((name) => (
            <button
              key={name}
              onClick={() => setSelected(name)}
              className={`filter-tab ${selected === name ? "active" : ""}`}
            >
              {name}
            </button>
          ))}
        </div>
        <div className="month-label">
          <CalendarDays size={15} /> November 2026
        </div>
      </div>
      <div className="schedule-list">
        {items.map((session, index) => {
          const course = demoCourses.find(
            (item) => item.id === session.courseId,
          );
          return (
            <motion.article
              className="schedule-row"
              key={session.id}
              initial={{ opacity: 0, x: 8 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: index * 0.04 }}
            >
              <div className="schedule-date">
                <small>
                  {new Intl.DateTimeFormat("en", { month: "short" }).format(
                    new Date(`${session.date}T12:00:00`),
                  )}
                </small>
                <strong>
                  {new Intl.DateTimeFormat("en", { day: "2-digit" }).format(
                    new Date(`${session.date}T12:00:00`),
                  )}
                </strong>
                <span>
                  {new Intl.DateTimeFormat("en", { weekday: "short" }).format(
                    new Date(`${session.date}T12:00:00`),
                  )}
                </span>
              </div>
              <div className="schedule-divider" />
              <div className="schedule-details">
                <span className="eyebrow">
                  {course?.title} · {session.type}
                </span>
                <h3>{session.title}</h3>
                <p>
                  <Clock3 size={14} /> {session.startTime} – {session.endTime}{" "}
                  <span>·</span> {session.location}
                </p>
              </div>
              <div className="schedule-owner">
                <Avatar name="Ahmed Khalil" size="sm" />
                <span>
                  <strong>Ahmed Khalil</strong>
                  <small>Instructor</small>
                </span>
              </div>
              <button
                className="icon-button row-action"
                aria-label={`Open ${session.title}`}
                onClick={() => alert("Session detail is a demo placeholder.")}
              >
                <ArrowUpRight size={17} />
              </button>
            </motion.article>
          );
        })}
      </div>
      <div className="info-strip">
        <Sparkles size={17} />
        <p>
          <strong>Make it a good session.</strong> Add a reminder to your
          calendar so you can arrive ready to learn.
        </p>
        <button
          onClick={() => alert("Calendar integration is not connected yet.")}
        >
          Add reminder <ArrowRight size={14} />
        </button>
      </div>
    </>
  );
}
