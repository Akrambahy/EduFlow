const statuses = {
  graded: {
    label: "Graded",
    classes: "bg-emerald-50 text-emerald-700 border border-emerald-200",
  },
  submitted: {
    label: "Submitted",
    classes: "bg-blue-50 text-blue-700 border border-blue-200",
  },
  closed: {
    label: "Closed",
    classes: "bg-slate-100 text-slate-600 border border-slate-200",
  },
  todo: {
    label: "To do",
    classes: "bg-amber-50 text-amber-800 border border-amber-200",
  },
};

export default function StatusPill({ status }) {
  const normalizedStatus = status === "to do" ? "todo" : status;
  const { label, classes } = statuses[normalizedStatus] || statuses.todo;

  return (
    <span
      className={`inline-flex w-max items-center gap-1.5 rounded-md px-2 py-0.5 font-mono text-[11px] font-semibold ${classes}`}
    >
      <i className="h-1.5 w-1.5 rounded-full bg-current" />
      {label}
    </span>
  );
}
