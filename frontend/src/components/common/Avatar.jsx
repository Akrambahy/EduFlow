import { getInitials } from "../../utils/formatters.js";

const sizes = {
  xs: "h-[23px] w-[23px] text-[8px]",
  sm: "h-[29px] w-[29px] text-[9px]",
  md: "h-[34px] w-[34px] text-[10px]",
  lg: "h-[47px] w-[47px] text-[13px]",
  xl: "h-[68px] w-[68px] text-[18px]",
};

export default function Avatar({ name, size = "md" }) {
  return (
    <span
      className={`inline-grid flex-none place-items-center rounded-full bg-[#e6eeea] font-display font-extrabold tracking-[-.3px] text-deep ${sizes[size] || sizes.md}`}
      aria-hidden="true"
    >
      {getInitials(name)}
    </span>
  );
}
