import { TrendingUp } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoAttendanceSummary } from "../data/sessions.js";
import { demoGrades } from "../data/assessments.js";
import { PageIntro, SectionHeading } from "../components/common/index.js";

export default function PerformancePage() {
  const { user } = useAuth();
  const record = demoGrades[user.id] || demoGrades.usr_student_01;
  const rows = record.courses;
  return (
    <>
      <PageIntro
        kicker="YOUR GROWTH, AT A GLANCE"
        title="Performance"
        subtitle="A thoughtful view of progress, attendance, and feedback."
      />
      <section className="performance-hero">
        <div>
          <span className="eyebrow light">CURRENT STANDING</span>
          <h2>
            Steady progress
            <br />
            <em>is still progress.</em>
          </h2>
          <p>
            Your latest snapshot reflects published grades and recorded
            attendance.
          </p>
        </div>
        <div className="performance-score">
          <span>OVERALL</span>
          <strong>
            {record.gpa}
            <small>%</small>
          </strong>
          <span className="score-foot">
            <TrendingUp size={14} /> On track
          </span>
        </div>
        <div className="score-orbit" />
      </section>
      <div className="performance-layout">
        <section className="content-panel">
          <SectionHeading
            title="Course breakdown"
            subtitle="Published results only"
          />
          <div className="grade-list">
            {rows.map((row) => (
              <div className="grade-row" key={row.courseId}>
                <div className="grade-course">
                  <span className="grade-mark">
                    {row.courseName.slice(0, 1)}
                  </span>
                  <strong>{row.courseName}</strong>
                </div>
                <div className="grade-track">
                  <span>
                    <i style={{ width: `${row.overall ?? 0}%` }} />
                  </span>
                </div>
                <strong className="grade-percent">
                  {row.overall == null ? "—" : `${row.overall}%`}
                </strong>
                <span className="grade-label">
                  {row.overall >= 90
                    ? "Excellent"
                    : row.overall >= 80
                      ? "Strong"
                      : row.overall
                        ? "Building"
                        : "In progress"}
                </span>
              </div>
            ))}
          </div>
        </section>
        <aside className="content-panel attendance-panel">
          <SectionHeading title="Showing up matters" />
          <div className="attendance-ring">
            <div>
              <strong>{demoAttendanceSummary.usr_student_01.rate}%</strong>
              <small>attendance</small>
            </div>
          </div>
          <div className="attendance-legend">
            <span>
              <i className="legend-present" /> Present <strong>7</strong>
            </span>
            <span>
              <i className="legend-late" /> Late <strong>2</strong>
            </span>
            <span>
              <i className="legend-absent" /> Absent <strong>1</strong>
            </span>
          </div>
          <p className="panel-note">
            Attendance outcomes and grade calculations are provided by your
            learning team.
          </p>
        </aside>
      </div>
    </>
  );
}
