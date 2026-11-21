import { useEffect, useState } from "react";
import { NavLink, useParams, useSearchParams } from "react-router-dom";
import {
  ArrowRight,
  BookOpen,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  Play,
  Users,
  X,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses, demoEnrollments, demoModules } from "../data/courses.js";
import { CourseGlyph, SectionHeading } from "../components/common/index.js";

function buildFallbackModules(course) {
  return [
    {
      id: `${course.id}_m1`,
      courseId: course.id,
      title: `Core Foundations of ${course.title}`,
      order: 1,
      lessons: [
        {
          id: `${course.id}_l1`,
          title: `Architecture & Mental Model`,
          type: "video",
          duration: "16 min",
          status: "completed",
          order: 1,
        },
        {
          id: `${course.id}_l2`,
          title: `Practical Setup & Tooling`,
          type: "reading",
          duration: "12 min",
          status: "in_progress",
          order: 2,
        },
        {
          id: `${course.id}_l3`,
          title: `Hands-on Guided Exercise`,
          type: "lab",
          duration: "24 min",
          status: "locked",
          order: 3,
        },
      ],
    },
  ];
}

export default function CourseDetailPage() {
  const { courseId } = useParams();
  const [searchParams] = useSearchParams();
  const { user } = useAuth();
  const course =
    demoCourses.find((item) => item.id === courseId) || demoCourses[0];
  const initialModules = demoModules[course.id] || buildFallbackModules(course);

  const [modules, setModules] = useState(initialModules);
  const [activeLessonId, setActiveLessonId] = useState(null);

  useEffect(() => {
    const fresh = demoModules[course.id] || buildFallbackModules(course);
    setModules(fresh);
    if (searchParams.get("resume") === "1") {
      const flat = fresh.flatMap((m) => m.lessons);
      const next =
        flat.find((l) => l.status === "in_progress") ||
        flat.find((l) => l.status !== "completed") ||
        flat[0];
      setActiveLessonId(next?.id || null);
    } else {
      setActiveLessonId(null);
    }
  }, [course.id, searchParams]);

  const enrollment = demoEnrollments.find(
    (item) => item.courseId === course.id && item.studentId === user.id,
  );
  const lessons = modules.flatMap((module) => module.lessons);
  const completedCount = lessons.filter((l) => l.status === "completed").length;
  const progress = lessons.length
    ? Math.max(
        enrollment?.progress || 0,
        Math.round((completedCount / lessons.length) * 100),
      )
    : enrollment?.progress || 0;

  const currentLesson =
    lessons.find((lesson) => lesson.id === activeLessonId) ||
    lessons.find((lesson) => lesson.status === "in_progress") ||
    lessons[0];

  const activeLesson = lessons.find((l) => l.id === activeLessonId);

  function handleMarkComplete(lessonId) {
    const idx = lessons.findIndex((l) => l.id === lessonId);
    const nextLesson = lessons[idx + 1];
    setModules((prev) =>
      prev.map((mod) => ({
        ...mod,
        lessons: mod.lessons.map((l) => {
          if (l.id === lessonId) return { ...l, status: "completed" };
          if (nextLesson && l.id === nextLesson.id && l.status === "locked") {
            return { ...l, status: "in_progress" };
          }
          return l;
        }),
      })),
    );
    if (nextLesson) {
      setActiveLessonId(nextLesson.id);
    }
  }

  return (
    <>
      <NavLink to="/courses" className="back-link">
        <ChevronLeft size={16} /> Back to courses
      </NavLink>
      <section className="course-hero">
        <div className={`course-hero-art art-${course.color}`}>
          <span className="art-overline">{course.category}</span>
          <CourseGlyph category={course.category} large />
          <span className="art-course-index">
            {course.totalModules} MODULES
          </span>
        </div>
        <div className="course-hero-copy">
          <span className="eyebrow">
            {course.category} · {course.duration}
          </span>
          <h1>{course.title}</h1>
          <p>{course.description}</p>
          <div className="hero-facts">
            <span>
              <BookOpen size={15} /> {course.totalLessons} lessons
            </span>
            <span>
              <Users size={15} /> {course.enrolledCount} enrolled learners
            </span>
          </div>
          <div className="hero-progress">
            <div className="progress-line">
              <span>
                <i style={{ width: `${progress}%` }} />
              </span>
              <small>{progress}% complete</small>
            </div>
          </div>
          <button
            type="button"
            className="button button-dark"
            onClick={() => setActiveLessonId(currentLesson?.id || null)}
          >
            <Play size={15} />
            {progress > 0 ? "Resume lesson" : "Start first lesson"}
          </button>
        </div>
      </section>

      {activeLesson && (
        <section className="lesson-workspace" aria-label="Active lesson reader">
          <div className="lesson-workspace-head">
            <div>
              <span className="eyebrow">
                ACTIVE LESSON · {activeLesson.type.toUpperCase()} ·{" "}
                {activeLesson.duration}
              </span>
              <h2>{activeLesson.title}</h2>
            </div>
            <button
              type="button"
              className="icon-button"
              aria-label="Close lesson reader"
              onClick={() => setActiveLessonId(null)}
            >
              <X size={16} />
            </button>
          </div>
          <div className="lesson-workspace-body">
            <p>
              In <strong>{activeLesson.title}</strong>, you will apply core
              concepts from <em>{course.title}</em> using clean, modular
              engineering patterns ready for integration with the backend API.
            </p>
            <div className="lesson-takeaways">
              <strong>Key objectives for this lesson:</strong>
              <ul>
                <li>
                  Structure predictable component state and explicit prop
                  contracts.
                </li>
                <li>
                  Verify edge cases across loading, empty, and error states.
                </li>
                <li>
                  Prepare clean pull-request ready code following team
                  conventions.
                </li>
              </ul>
            </div>
          </div>
          <div className="lesson-workspace-foot">
            <span className="quiet-label">
              Status:{" "}
              {activeLesson.status === "completed"
                ? "Completed"
                : "In progress"}
            </span>
            <button
              type="button"
              className="button button-emerald button-sm"
              onClick={() => handleMarkComplete(activeLesson.id)}
            >
              <CheckCircle2 size={15} />
              {activeLesson.status === "completed"
                ? "Completed — Go to next lesson"
                : "Mark complete & continue"}
            </button>
          </div>
        </section>
      )}

      <section className="content-panel">
        <SectionHeading
          title="Course syllabus"
          subtitle={`${modules.length} modules · ${lessons.length} interactive lessons`}
        />
        <div className="module-list">
          {modules.map((module, i) => (
            <div className="module-block" key={module.id}>
              <div className="module-heading">
                <span className="module-number">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div>
                  <h3>{module.title}</h3>
                  <p>{module.lessons.length} lessons</p>
                </div>
                <ChevronDown size={17} />
              </div>
              {module.lessons.map((lesson) => (
                <button
                  type="button"
                  key={lesson.id}
                  className={`lesson-row ${activeLessonId === lesson.id ? "is-selected" : ""}`}
                  onClick={() => setActiveLessonId(lesson.id)}
                >
                  <span
                    className={`lesson-status ${lesson.status === "completed" ? "is-complete" : lesson.status === "in_progress" ? "is-current" : ""}`}
                  >
                    {lesson.status === "completed" ? (
                      <Check size={13} />
                    ) : (
                      lesson.order
                    )}
                  </span>
                  <span className="lesson-copy">
                    <strong>{lesson.title}</strong>
                    <small>
                      {lesson.duration} · {lesson.type}
                    </small>
                  </span>
                  {lesson.status === "completed" ? (
                    <CheckCircle2 className="lesson-check" size={17} />
                  ) : (
                    <ArrowRight className="lesson-arrow" size={15} />
                  )}
                </button>
              ))}
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
