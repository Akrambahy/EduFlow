import { useState } from "react";
import { NavLink } from "react-router-dom";
import { motion } from "motion/react";
import { ArrowRight, Search } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCourses, demoEnrollments } from "../data/courses.js";
import { ROLES } from "../utils/constants.js";
import {
  CourseGlyph,
  EmptyState,
  PageIntro,
} from "../components/common/index.js";

export default function CoursesPage() {
  const { user } = useAuth();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("All courses");
  const courses = demoCourses.filter(
    (course) =>
      user.role !== ROLES.STUDENT || user.enrolledCourses?.includes(course.id),
  );
  const visible = courses.filter(
    (course) =>
      `${course.title} ${course.category}`
        .toLowerCase()
        .includes(query.toLowerCase()) &&
      (filter === "All courses" ||
        (filter === "In progress"
          ? demoEnrollments.some(
              (e) =>
                e.courseId === course.id && e.progress > 0 && e.progress < 100,
            )
          : filter === "Completed"
            ? demoEnrollments.some(
                (e) => e.courseId === course.id && e.progress === 100,
              )
            : true)),
  );
  return (
    <>
      <PageIntro
        kicker="YOUR LEARNING SPACE"
        title="Courses"
        subtitle="A clear path from curious to confident."
      />
      <div className="toolbar">
        <div className="filter-tabs">
          {["All courses", "In progress", "Completed"].map((name) => (
            <button
              className={filter === name ? "filter-tab active" : "filter-tab"}
              key={name}
              onClick={() => setFilter(name)}
            >
              {name}
            </button>
          ))}
        </div>
        <label className="inline-search">
          <Search size={15} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Find a course"
          />
        </label>
      </div>
      {visible.length ? (
        <div className="catalog-grid">
          {visible.map((course, index) => {
            const progress =
              demoEnrollments.find((e) => e.courseId === course.id)?.progress ||
              0;
            return (
              <motion.article
                className="catalog-card"
                key={course.id}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: index * 0.04 }}
              >
                <NavLink
                  to={`/courses/${course.id}`}
                  className={`catalog-art art-${course.color}`}
                >
                  <span className="art-overline">{course.category} / 2026</span>
                  <CourseGlyph category={course.category} large />
                  <span className="art-course-index">0{index + 1}</span>
                </NavLink>
                <div className="catalog-body">
                  <div className="catalog-meta">
                    <span>{course.totalLessons} lessons</span>
                    <span>{course.duration}</span>
                  </div>
                  <h3>{course.title}</h3>
                  <p>{course.description}</p>
                  <div className="catalog-progress">
                    <div className="progress-line">
                      <span>
                        <i style={{ width: `${progress}%` }} />
                      </span>
                      <small>{progress}%</small>
                    </div>
                  </div>
                  <NavLink
                    to={`/courses/${course.id}`}
                    className="catalog-continue"
                  >
                    {progress ? "Continue course" : "Explore course"}
                    <ArrowRight size={15} />
                  </NavLink>
                </div>
              </motion.article>
            );
          })}
        </div>
      ) : (
        <EmptyState
          title="No courses match that search"
          text="Try another title or switch to a different filter."
        />
      )}
    </>
  );
}
