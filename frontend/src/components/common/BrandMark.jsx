export default function BrandMark({ variant = "default" }) {
  const colors =
    variant === "light"
      ? "bg-[#ecf1df] [&>span]:bg-[#315a4f]"
      : "bg-deep [&>span]:bg-lime";

  return (
    <span
      className={`inline-flex h-7 w-7 flex-none rotate-[-4deg] items-end gap-0.75 rounded-[9px] px-1.5 py-1.25 ${colors}`}
      aria-hidden="true"
    >
      <span className="block h-2.25 w-1 rounded" />
      <span className="block h-3.75 w-1 rounded" />
      <span className="block h-3 w-1 rounded" />
    </span>
  );
}
