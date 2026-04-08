import type { PropsWithChildren } from "react";

type PanelSurfaceProps = PropsWithChildren<{
  title: string;
  eyebrow?: string;
}>;

export function PanelSurface({ title, eyebrow, children }: PanelSurfaceProps) {
  return (
    <section className="rounded-3xl border border-white/10 bg-slate-950/55 p-5 shadow-[0_24px_80px_-40px_rgba(14,165,233,0.6)] backdrop-blur">
      <div className="mb-4">
        {eyebrow ? (
          <p className="mb-2 text-xs uppercase tracking-[0.28em] text-cyan-300/80">{eyebrow}</p>
        ) : null}
        <h2 className="text-lg font-semibold text-white">{title}</h2>
      </div>
      {children}
    </section>
  );
}
