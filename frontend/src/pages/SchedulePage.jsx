import { useState } from "react";
import { motion } from "motion/react";
import {
  ArrowRight,
  CalendarDays,
  CheckCircle2,
  Clock3,
  Plus,
  Sparkles,
  X,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses } from "../data/courses.js";
import { demoSessions } from "../data/sessions.js";
import { ROLES } from "../utils/constants.js";
import { Avatar, PageIntro } from "../components/common/index.js";

export default function SchedulePage() {
  const { user } = useAuth();
  const [selected, setSelected] = useState("Upcoming");
  const [sessions, setSessions] = useState(demoSessions);
  const [activeSession, setActiveSession] = useState(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [reminderSaved, setReminderSaved] = useState(false);

  const [newTitle, setNewTitle] = useState("");
  const [newDate, setNewDate] = useState("2026-11-12");
  const [newLocation, setNewLocation] = useState("Room 301");

  const items = sessions.filter((session) =>
    selected === "Upcoming"
      ? session.status === "upcoming"
      : session.status === "completed",
  );

  function handleCreateSession(e) {
    e.preventDefault();
    if (!newTitle.trim()) return;
    const created = {
      id: `ses_${Date.now()}`,
      courseId: "crs_react",
      title: newTitle.trim(),
      date: newDate,
      startTime: "11:00",
      endTime: "12:30",
      location: newLocation.trim() || "Room 301",
      type: "workshop",
      instructorId: user.id,
      status: "upcoming",
    };
    setSessions((prev) => [created, ...prev]);
    setNewTitle("");
    setCreateOpen(false);
  }

  return (
    <>
      <PageIntro
        kicker="TIMETABLE & LIVE WORKSHOPS"
        title="Schedule"
        subtitle="Upcoming lectures, lab sessions, and attendance check-ins."
        action={
          user.role === ROLES.INSTRUCTOR || user.role === ROLES.ADMIN ? (
            <button
              type="button"
              className="button button-dark"
              onClick={() => setCreateOpen(true)}
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
              type="button"
              key={name}
              onClick={() => setSelected(name)}
              className={`filter-tab ${selected === name ? "active" : ""}`}
            >
              {name}
            </button>
          ))}
        </div>
        <div className="month-label">
          <CalendarDays size={15} /> NOVEMBER 2026
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
              initial={{ opacity: 0, x: 6 }}
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
                  <Clock3 size={14} /> {session.startTime} – {session.endTime} ·{" "}
                  {session.location}
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
                type="button"
                className="button button-outline button-sm"
                onClick={() => setActiveSession(session)}
              >
                Session details
              </button>
            </motion.article>
          );
        })}
      </div>

      <div className="info-strip">
        <Sparkles size={17} />
        <p>
          <strong>Sync your academic calendar.</strong>{" "}
          {reminderSaved
            ? "Calendar reminders enabled for all Fall 2026 sessions."
            : "Enable automated reminders 30 minutes before each live workshop."}
        </p>
        <button
          type="button"
          onClick={() => setReminderSaved((prev) => !prev)}
        >
          {reminderSaved ? (
            <>
              <CheckCircle2 size={14} /> Reminders active
            </>
          ) : (
            <>
              Enable reminders <ArrowRight size={14} />
            </>
          )}
        </button>
      </div>

      {activeSession && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-header">
              <h3>{activeSession.title}</h3>
              <button
                type="button"
                className="icon-button"
                onClick={() => setActiveSession(null)}
                aria-label="Close session details"
              >
                <X size={16} />
              </button>
            </div>
            <div className="modal-body">
              <span className="eyebrow">
                {activeSession.type.toUpperCase()} · {activeSession.location}
              </span>
              <p className="muted">
                Scheduled on {activeSession.date} from {activeSession.startTime}{" "}
                to {activeSession.endTime}. Attendance check-in opens 10 minutes
                prior to start time.
              </p>
            </div>
            <div className="modal-footer">
              <button
                type="button"
                className="button button-dark"
                onClick={() => setActiveSession(null)}
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}

      {createOpen && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-header">
              <h3>Schedule new session</h3>
              <button
                type="button"
                className="icon-button"
                onClick={() => setCreateOpen(false)}
                aria-label="Close modal"
              >
                <X size={16} />
              </button>
            </div>
            <form onSubmit={handleCreateSession}>
              <div className="modal-body">
                <label className="field-group">
                  Session title
                  <input
                    type="text"
                    required
                    placeholder="e.g., Custom Hooks & Context Workshop"
                    value={newTitle}
                    onChange={(e) => setNewTitle(e.target.value)}
                  />
                </label>
                <label className="field-group">
                  Date
                  <input
                    type="date"
                    value={newDate}
                    onChange={(e) => setNewDate(e.target.value)}
                  />
                </label>
                <label className="field-group">
                  Room / Venue
                  <input
                    type="text"
                    value={newLocation}
                    onChange={(e) => setNewLocation(e.target.value)}
                  />
                </label>
              </div>
              <div className="modal-footer">
                <button
                  type="button"
                  className="button button-outline"
                  onClick={() => setCreateOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="button button-dark">
                  Create session
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
