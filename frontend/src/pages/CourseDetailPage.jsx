import { NavLink, useParams } from "react-router-dom";
import {
  ArrowRight,
  BookOpen,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  Users,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses, demoEnrollments, demoModules } from "../data/courses.js";
import { CourseGlyph, SectionHeading } from "../components/common/index.js";

export default function CourseDetailPage() {
  const { courseId } = useParams();
  const { user } = useAuth();
  const course =
    demoCourses.find((item) => item.id === courseId) || demoCourses[0];
  const modules = demoModules[course.id] || [];
  const enrollment = demoEnrollments.find(
    (item) => item.courseId === course.id && item.studentId === user.id,
  );
  const progress = enrollment?.progress || 0;
  const lessons = modules.flatMap((module) => module.lessons);
  const currentLesson =
    lessons.find((lesson) => lesson.status === "in_progress") ||
    lessons.find((lesson) => lesson.status === "locked");
  return (
    <>
      <NavLink to="/courses" className="back-link">
        <ChevronLeft size={16} /> Back to courses
      </NavLink>
      <section className="course-hero">
        <div className={`course-hero-art art-${course.color}`}>
          <span className="art-overline">
            {course.category} · LEARNING PATH
          </span>
          <CourseGlyph category={course.category} large />
          <span className="art-course-index">
            EF / {course.id.slice(-2).toUpperCase()}
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
              <Users size={15} /> {course.enrolledCount} learners
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
            className="button button-dark"
            onClick={() =>
              alert(
                currentLesson
                  ? `Opening “${currentLesson.title}” is a demo placeholder.`
                  : "Course content is a demo placeholder.",
              )
            }
          >
            {progress ? "Continue learning" : "Start learning"}
            <ArrowRight size={16} />
          </button>
        </div>
      </section>
      <section className="content-panel">
        <SectionHeading
          title="Course outline"
          subtitle={`${modules.length} modules · ${course.totalLessons} lessons`}
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
                  key={lesson.id}
                  className="lesson-row"
                  onClick={() =>
                    alert(`Lesson “${lesson.title}” is a demo placeholder.`)
                  }
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
