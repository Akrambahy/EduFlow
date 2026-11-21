import { useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  ClipboardCheck,
  FileText,
  Send,
  X,
} from "lucide-react";
import { demoCourses } from "../data/courses.js";
import {
  demoAssignments,
  demoQuizQuestions,
  demoQuizzes,
  demoSubmissions,
} from "../data/assessments.js";
import { prettyDate } from "../utils/formatters.js";
import { PageIntro, StatusPill } from "../components/common/index.js";

export default function AssignmentsPage() {
  const [tab, setTab] = useState("All work");
  const [submissions, setSubmissions] = useState(demoSubmissions);
  const [selectedAssignment, setSelectedAssignment] = useState(null);
  const [repoUrl, setRepoUrl] = useState("");
  const [notes, setNotes] = useState("");
  const [toast, setToast] = useState("");

  const [quizOpen, setQuizOpen] = useState(false);
  const [quizAnswers, setQuizAnswers] = useState({});
  const [quizResult, setQuizResult] = useState(null);

  const rows = demoAssignments.map((assignment) => ({
    ...assignment,
    course: demoCourses.find((course) => course.id === assignment.courseId),
    submission: submissions.find(
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

  function handleSubmitWork(e) {
    e.preventDefault();
    if (!selectedAssignment) return;
    const newSub = {
      id: `sub_${Date.now()}`,
      assignmentId: selectedAssignment.id,
      studentId: "usr_student_01",
      studentName: "Marwan Ashour",
      submittedAt: new Date().toISOString(),
      fileName: repoUrl.trim() || "submission-archive.zip",
      grade: null,
      maxGrade: selectedAssignment.maxGrade,
      feedback: notes.trim() || "Submitted via EduFlow student portal.",
      status: "submitted",
    };
    setSubmissions((prev) => [
      ...prev.filter((s) => s.assignmentId !== selectedAssignment.id),
      newSub,
    ]);
    setToast(`Submitted "${selectedAssignment.title}" for instructor review.`);
    setSelectedAssignment(null);
    setRepoUrl("");
    setNotes("");
  }

  function handleGradeQuiz() {
    const questions = demoQuizQuestions.qz_001 || [];
    const correct = questions.reduce(
      (acc, q, idx) => (quizAnswers[idx] === q.correctIndex ? acc + 1 : acc),
      0,
    );
    setQuizResult({ correct, total: questions.length });
  }

  return (
    <>
      <PageIntro
        kicker="DELIVERABLES & ASSESSMENTS"
        title="Assignments"
        subtitle="Submit coursework, review instructor feedback, and complete quick checks."
      />
      {toast && (
        <div className="toast-banner" role="status">
          <span>{toast}</span>
          <button
            type="button"
            className="icon-button"
            style={{ width: 24, height: 24 }}
            onClick={() => setToast("")}
            aria-label="Dismiss notice"
          >
            <X size={14} />
          </button>
        </div>
      )}
      <div className="toolbar">
        <div className="filter-tabs">
          {["All work", "To do", "Submitted"].map((name) => (
            <button
              type="button"
              className={`filter-tab ${tab === name ? "active" : ""}`}
              key={name}
              onClick={() => setTab(name)}
            >
              {name}
            </button>
          ))}
        </div>
        <span className="quiet-label">
          {filtered.length} deliverable{filtered.length === 1 ? "" : "s"}
        </span>
      </div>
      <div className="table-card">
        <div className="table-head assignments-head">
          <span>ASSIGNMENT</span>
          <span>COURSE</span>
          <span>DUE DATE</span>
          <span>STATUS</span>
          <span>GRADE</span>
          <span>ACTION</span>
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
                (item.status === "closed" ? "closed" : "todo")
              }
            />
            <span className="grade-value">
              {item.submission?.grade != null
                ? `${item.submission.grade}/${item.submission.maxGrade}`
                : "—"}
            </span>
            <button
              type="button"
              className="button button-outline button-sm"
              onClick={() => setSelectedAssignment(item)}
            >
              {item.submission ? "View details" : "Submit work"}
            </button>
          </div>
        ))}
      </div>

      <section className="quiz-feature">
        <div className="quiz-feature-mark">
          <ClipboardCheck size={20} />
        </div>
        <div>
          <span className="eyebrow">QUICK CHECK · TIMED ASSESSMENT</span>
          <h3>{demoQuizzes[0].title}</h3>
          <p>
            {demoQuizzes[0].totalQuestions} questions ·{" "}
            {demoQuizzes[0].timeLimit} minutes · Due{" "}
            {prettyDate(demoQuizzes[0].dueDate)}
          </p>
        </div>
        <button
          type="button"
          className="button button-dark"
          onClick={() => {
            setQuizResult(null);
            setQuizOpen(true);
          }}
        >
          Start quick check <ArrowRight size={15} />
        </button>
      </section>

      {selectedAssignment && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-header">
              <h3>{selectedAssignment.title}</h3>
              <button
                type="button"
                className="icon-button"
                onClick={() => setSelectedAssignment(null)}
                aria-label="Close modal"
              >
                <X size={16} />
              </button>
            </div>
            {selectedAssignment.submission ? (
              <div className="modal-body">
                <div className="flex items-center justify-between">
                  <span className="eyebrow">SUBMISSION RECORD</span>
                  <StatusPill status={selectedAssignment.submission.status} />
                </div>
                <p className="muted">{selectedAssignment.description}</p>
                <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-[12px]">
                  <div>
                    <strong>Artifact:</strong>{" "}
                    {selectedAssignment.submission.fileName}
                  </div>
                  {selectedAssignment.submission.grade != null && (
                    <div className="mt-1">
                      <strong>Score:</strong>{" "}
                      {selectedAssignment.submission.grade}/
                      {selectedAssignment.submission.maxGrade}
                    </div>
                  )}
                  <div className="mt-2 text-muted">
                    <strong>Notes / Feedback:</strong>{" "}
                    {selectedAssignment.submission.feedback}
                  </div>
                </div>
                <div className="modal-footer" style={{ margin: "0 -20px -20px" }}>
                  <button
                    type="button"
                    className="button button-outline"
                    onClick={() => setSelectedAssignment(null)}
                  >
                    Close
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleSubmitWork}>
                <div className="modal-body">
                  <p className="muted">{selectedAssignment.description}</p>
                  <label className="field-group">
                    Repository URL or Archive Name
                    <input
                      type="text"
                      required
                      placeholder="https://github.com/username/eduflow-todo"
                      value={repoUrl}
                      onChange={(e) => setRepoUrl(e.target.value)}
                    />
                  </label>
                  <label className="field-group">
                    Implementation Notes
                    <textarea
                      placeholder="Summarize your component architecture and testing steps…"
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                    />
                  </label>
                </div>
                <div className="modal-footer">
                  <button
                    type="button"
                    className="button button-outline"
                    onClick={() => setSelectedAssignment(null)}
                  >
                    Cancel
                  </button>
                  <button type="submit" className="button button-dark">
                    <Send size={14} /> Submit assignment
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {quizOpen && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-header">
              <h3>{demoQuizzes[0].title}</h3>
              <button
                type="button"
                className="icon-button"
                onClick={() => setQuizOpen(false)}
                aria-label="Close quiz"
              >
                <X size={16} />
              </button>
            </div>
            <div className="modal-body" style={{ maxHeight: "65vh", overflowY: "auto" }}>
              {quizResult ? (
                <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-center">
                  <CheckCircle2
                    size={28}
                    className="mx-auto mb-2 text-emerald-700"
                  />
                  <h4 className="m-0 font-display text-[17px] font-bold text-ink">
                    Score: {quizResult.correct} / {quizResult.total} (
                    {Math.round((quizResult.correct / quizResult.total) * 100)}%)
                  </h4>
                  <p className="mt-1 text-[12px] text-muted">
                    Your response has been recorded for React Fundamentals.
                  </p>
                </div>
              ) : (
                (demoQuizQuestions.qz_001 || []).map((q, qIdx) => (
                  <div key={q.id} className="flex flex-col gap-2">
                    <strong className="text-[13px] text-ink">
                      {qIdx + 1}. {q.text}
                    </strong>
                    {q.options.map((opt, optIdx) => (
                      <label
                        key={opt}
                        className={`quiz-option ${quizAnswers[qIdx] === optIdx ? "selected" : ""}`}
                      >
                        <input
                          type="radio"
                          name={q.id}
                          checked={quizAnswers[qIdx] === optIdx}
                          onChange={() =>
                            setQuizAnswers((prev) => ({
                              ...prev,
                              [qIdx]: optIdx,
                            }))
                          }
                        />
                        <span>{opt}</span>
                      </label>
                    ))}
                  </div>
                ))
              )}
            </div>
            <div className="modal-footer">
              <button
                type="button"
                className="button button-outline"
                onClick={() => setQuizOpen(false)}
              >
                Close
              </button>
              {!quizResult && (
                <button
                  type="button"
                  className="button button-dark"
                  onClick={handleGradeQuiz}
                >
                  Check answers
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
