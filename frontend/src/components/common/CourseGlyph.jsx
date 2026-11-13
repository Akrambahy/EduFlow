import { Command, GraduationCap, ShieldCheck, TrendingUp } from "lucide-react";

export default function CourseGlyph({ category, large = false }) {
  const Icon =
    category === "Backend"
      ? Command
      : category === "Data"
        ? TrendingUp
        : category === "Quality"
          ? ShieldCheck
          : GraduationCap;

  return (
    <span
      className={`course-glyph ${large ? "course-glyph-large" : ""}`}
      aria-hidden="true"
    >
      <Icon size={large ? 33 : 19} strokeWidth={1.45} />
    </span>
  );
}
