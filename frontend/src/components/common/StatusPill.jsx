const statuses = {
  graded: {
    label: "Graded",
    classes: "bg-[#e8f3e9] text-[#438163]",
  },
  submitted: {
    label: "Submitted",
    classes: "bg-[#f2eee0] text-[#9b7b39]",
  },
  closed: {
    label: "Closed",
    classes: "bg-[#f0f1ed] text-[#89948e]",
  },
  todo: {
    label: "To do",
    classes: "bg-[#f7e9e2] text-[#b17156]",
  },
};

export default function StatusPill({ status }) {
  const normalizedStatus = status === "to do" ? "todo" : status;
  const { label, classes } = statuses[normalizedStatus] || statuses.todo;

  return (
    <span
      className={`inline-flex w-max items-center gap-[5px] rounded-[20px] px-[7px] py-1 text-[8px] font-bold ${classes}`}
    >
      <i className="h-[5px] w-[5px] rounded-full bg-current" />
      {label}
    </span>
  );
}
