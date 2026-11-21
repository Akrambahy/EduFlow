import { BookOpen } from "lucide-react";

export default function EmptyState({ title, text, action }) {
  return (
    <div className="mx-auto my-14 flex max-w-[400px] flex-col items-center rounded-lg border border-slate-200 bg-white p-7 text-center shadow-xs">
      <span className="grid h-11 w-11 place-items-center rounded-lg bg-blue-50 text-blue-700">
        <BookOpen size={20} />
      </span>
      <h2 className="mb-1.5 mt-3.5 font-display text-[16px] font-bold text-ink">
        {title}
      </h2>
      <p className="mb-4 mt-0 text-[12px] leading-[1.6] text-muted">{text}</p>
      {action}
    </div>
  );
}
