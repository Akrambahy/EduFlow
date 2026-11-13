import { useState } from "react";
import {
  ArrowRight,
  ArrowUpRight,
  ClipboardCheck,
  FileText,
} from "lucide-react";
import { demoCourses } from "../data/courses.js";
import {
  demoAssignments,
  demoQuizzes,
  demoSubmissions,
} from "../data/assessments.js";
import { prettyDate } from "../utils/formatters.js";
import { PageIntro, StatusPill } from "../components/common/index.js";

export default function AssignmentsPage() {
  const [tab, setTab] = useState("All work");
  const rows = demoAssignments.map((assignment) => ({
    ...assignment,
    course: demoCourses.find((course) => course.id === assignment.courseId),
    submission: demoSubmissions.find(
      (submission) => submission.assignmentId === assignment.id,
    ),
  }));
  const filtered = rows.filter(
    (item) =>
      tab === "All work" ||
      (tab === "To do"
        ? !item.submission && item.status !== "closed"
        : !!item.submission),
  );
  return (
    <>
      <PageIntro
        kicker="KEEP YOUR MOMENTUM"
        title="Assignments"
        subtitle="Your work, feedback, and next deadlines in one place."
      />
      <div className="toolbar">
        <div className="filter-tabs">
          {["All work", "To do", "Submitted"].map((name) => (
            <button
              className={`filter-tab ${tab === name ? "active" : ""}`}
              key={name}
              onClick={() => setTab(name)}
            >
              {name}
            </button>
          ))}
        </div>
        <span className="quiet-label">{filtered.length} items</span>
      </div>
      <div className="table-card">
        <div className="table-head assignments-head">
          <span>ASSIGNMENT</span>
          <span>COURSE</span>
          <span>DUE DATE</span>
          <span>STATUS</span>
          <span>GRADE</span>
          <span />
        </div>
        {filtered.map((item) => (
          <div className="table-row assignments-row" key={item.id}>
            <div className="assignment-title">
              <span className="table-icon">
                <FileText size={16} />
              </span>
              <span>
                <strong>{item.title}</strong>
                <small>{item.description}</small>
              </span>
            </div>
            <span className="table-course">{item.course?.title}</span>
            <span className="table-date">{prettyDate(item.dueDate)}</span>
            <StatusPill
              status={
                item.submission?.status ||
                (item.status === "closed" ? "closed" : "to do")
              }
            />
            <span className="grade-value">
              {item.submission
                ? `${item.submission.grade}/${item.submission.maxGrade}`
                : "—"}
            </span>
            <button
              className="icon-button"
              aria-label={`Open ${item.title}`}
              onClick={() =>
                alert(
                  "Assignment details will connect to the Go API when its contract is ready.",
                )
              }
            >
              <ArrowUpRight size={16} />
            </button>
          </div>
        ))}
      </div>
      <section className="quiz-feature">
        <div className="quiz-feature-mark">
          <ClipboardCheck size={20} />
        </div>
        <div>
          <span className="eyebrow">UP NEXT · QUICK CHECK</span>
          <h3>{demoQuizzes[0].title}</h3>
          <p>
            {demoQuizzes[0].totalQuestions} questions ·{" "}
            {demoQuizzes[0].timeLimit} minutes · Due{" "}
            {prettyDate(demoQuizzes[0].dueDate)}
          </p>
        </div>
        <button
          className="button button-outline"
          onClick={() =>
            alert(
              "Quiz attempt is a demo placeholder. The backend will calculate the score.",
            )
          }
        >
          Review quiz
          <ArrowRight size={15} />
        </button>
      </section>
    </>
  );
}
