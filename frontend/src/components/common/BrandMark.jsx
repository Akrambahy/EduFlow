export default function BrandMark({ variant = "default" }) {
  const colors =
    variant === "light"
      ? "bg-blue-600 [&>span]:bg-white"
      : "bg-slate-900 [&>span]:bg-blue-400";

  return (
    <span
      className={`inline-flex h-7 w-7 flex-none items-end gap-0.75 rounded-md px-1.5 py-1.5 ${colors}`}
      aria-hidden="true"
    >
      <span className="block h-2 w-1 rounded-[1px]" />
      <span className="block h-3.5 w-1 rounded-[1px]" />
      <span className="block h-2.5 w-1 rounded-[1px]" />
    </span>
  );
}
