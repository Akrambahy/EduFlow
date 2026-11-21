export default function PageIntro({ kicker, title, subtitle, action }) {
  return (
    <div className="mb-6 flex items-end justify-between gap-5 max-[560px]:mb-4 max-[560px]:flex-col max-[560px]:items-start max-[560px]:gap-3">
      <div>
        <span className="eyebrow">{kicker}</span>
        <h1 className="mb-1 mt-1.5 font-display text-[28px] font-bold leading-[1.2] tracking-[-0.025em] text-ink max-[560px]:text-[23px]">
          {title}
        </h1>
        <p className="m-0 text-[13px] text-muted">{subtitle}</p>
      </div>
      {action && <div className="pb-[2px] max-[560px]:p-0">{action}</div>}
    </div>
  );
}
