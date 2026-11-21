import { ArrowUpRight, Check } from "lucide-react";

export default function TaskItem({
  icon: Icon,
  title,
  detail,
  tone,
  done,
  onToggle,
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className={`task-item ${done ? "task-done" : ""}`}
      aria-pressed={!!done}
    >
      <span className={`task-icon task-${tone}`}>
        {done ? <Check size={15} /> : <Icon size={15} />}
      </span>
      <span>
        <strong>{title}</strong>
        <small>{detail}</small>
      </span>
      <ArrowUpRight size={14} className="task-arrow" />
    </button>
  );
}
