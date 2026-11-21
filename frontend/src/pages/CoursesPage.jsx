import { useState } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { motion } from "motion/react";
import { Play, Search } from "lucide-react";
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
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("All courses");

  const courses = demoCourses.filter(
    (course) =>
      user.role !== ROLES.STUDENT || user.enrolledCourses?.includes(course.id),
  );

  const visible = courses.filter((course) => {
    const matchesQuery = `${course.title} ${course.category}`
      .toLowerCase()
      .includes(query.toLowerCase());
    const enrollment = demoEnrollments.find((e) => e.courseId === course.id);
    const progress = enrollment?.progress || 0;

    if (!matchesQuery) return false;
    if (filter === "In progress") return progress > 0 && progress < 100;
    if (filter === "Not started") return progress === 0;
    if (filter === "Completed") return progress === 100;
    return true;
  });

  return (
    <>
      <PageIntro
        kicker="CURRICULUM & MODULES"
        title="Courses"
        subtitle="Filter by progress or resume your active module."
      />
      <div className="toolbar">
        <div className="filter-tabs" role="tablist" aria-label="Filter by progress">
          {["All courses", "In progress", "Not started", "Completed"].map(
            (name) => (
              <button
                type="button"
                className={filter === name ? "filter-tab active" : "filter-tab"}
                key={name}
                onClick={() => setFilter(name)}
              >
                {name}
              </button>
            ),
          )}
        </div>
        <label className="inline-search">
          <Search size={15} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter by title or track…"
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
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: index * 0.04 }}
              >
                <NavLink
                  to={`/courses/${course.id}`}
                  className={`catalog-art art-${course.color}`}
                >
                  <span className="art-overline">{course.category}</span>
                  <CourseGlyph category={course.category} large />
                  <span className="art-course-index">
                    {course.totalModules} MODULES
                  </span>
                </NavLink>
                <div className="catalog-body">
                  <div className="catalog-meta">
                    <span>{course.totalLessons} lessons</span>
                    <span>{course.duration}</span>
                  </div>
                  <h3>
                    <NavLink to={`/courses/${course.id}`}>
                      {course.title}
                    </NavLink>
                  </h3>
                  <p>{course.description}</p>
                  <div className="catalog-progress">
                    <div className="progress-line">
                      <span>
                        <i style={{ width: `${progress}%` }} />
                      </span>
                      <small>{progress}%</small>
                    </div>
                  </div>
                  <div className="catalog-continue">
                    <NavLink to={`/courses/${course.id}`}>
                      Syllabus overview
                    </NavLink>
                    <button
                      type="button"
                      className="button button-dark button-sm"
                      onClick={() => navigate(`/courses/${course.id}?resume=1`)}
                    >
                      <Play size={13} />
                      {progress > 0 ? "Resume lesson" : "Start course"}
                    </button>
                  </div>
                </div>
              </motion.article>
            );
          })}
        </div>
      ) : (
        <EmptyState
          title="No courses match that filter"
          text="Reset your progress filter or search query to view all enrolled courses."
          action={
            <button
              type="button"
              className="button button-outline button-sm"
              onClick={() => {
                setFilter("All courses");
                setQuery("");
              }}
            >
              Reset filters
            </button>
          }
        />
      )}
    </>
  );
}
