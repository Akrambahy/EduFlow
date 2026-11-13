import { BookOpen } from "lucide-react";

export default function EmptyState({ title, text, action }) {
  return (
    <div className="mx-auto my-[80px] flex max-w-[390px] flex-col items-center p-6 text-center">
      <span className="grid h-[45px] w-[45px] place-items-center rounded-[13px] bg-[#e9f0e8] text-teal">
        <BookOpen size={20} />
      </span>
      <h2 className="mb-1 mt-[14px] font-display text-[17px]">{title}</h2>
      <p className="mb-4 mt-0 text-[10px] leading-[1.6] text-muted">{text}</p>
      {action}
    </div>
  );
}
