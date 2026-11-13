export default function PageIntro({ kicker, title, subtitle, action }) {
  return (
    <div className="mb-[25px] flex items-end justify-between gap-5 max-[560px]:mb-[18px] max-[560px]:flex-col max-[560px]:items-start max-[560px]:gap-3">
      <div>
        <span className="eyebrow">{kicker}</span>
        <h1 className="mb-[5px] mt-2 font-display text-[30px] font-bold leading-[1.18] tracking-[-1.15px] text-ink max-[560px]:text-[25px]">
          {title}
        </h1>
        <p className="m-0 text-[12px] text-muted max-[560px]:text-[10px]">
          {subtitle}
        </p>
      </div>
      {action && <div className="pb-[2px] max-[560px]:p-0">{action}</div>}
    </div>
  );
}
