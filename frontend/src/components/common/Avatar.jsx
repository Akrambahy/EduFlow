import { getInitials } from "../../utils/formatters.js";

const sizes = {
  xs: "h-6 w-6 text-[10px]",
  sm: "h-7 w-7 text-[11px]",
  md: "h-8 w-8 text-[11px]",
  lg: "h-11 w-11 text-[14px]",
  xl: "h-16 w-16 text-[19px]",
};

export default function Avatar({ name, size = "md" }) {
  return (
    <span
      className={`inline-grid flex-none place-items-center rounded-md border border-slate-200 bg-slate-100 font-display font-bold tracking-[-0.02em] text-slate-800 ${sizes[size] || sizes.md}`}
      aria-hidden="true"
    >
      {getInitials(name)}
    </span>
  );
}
