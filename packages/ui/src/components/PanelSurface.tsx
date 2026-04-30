import type { PropsWithChildren, ReactNode } from "react";

type PanelSurfaceProps = PropsWithChildren<{
  title: string;
  eyebrow?: string;
  titleAccessory?: ReactNode;
}>;

// PanelSurface renders the shared dashboard card shell.
// The optional titleAccessory lets feature modules place compact controls
// on the same row as the title without rewriting the shared header markup.
export function PanelSurface({ title, eyebrow, titleAccessory, children }: PanelSurfaceProps) {
  return (
    <section
      className="rounded-[var(--cc-radius-lg)] border p-5 text-[color:var(--cc-ink)] shadow-[var(--cc-card-shadow)] backdrop-blur"
      style={{ background: "var(--cc-paper)", borderColor: "var(--cc-line)" }}
    >
      <div className="cc-panel-surface__header mb-4">
        {eyebrow ? (
          <p className="cc-panel-surface__eyebrow mb-2 text-xs uppercase tracking-[0.28em] text-[color:var(--cc-ink-muted)]">{eyebrow}</p>
        ) : null}
        <div className="cc-panel-surface__title-row flex flex-wrap items-center justify-between gap-3">
          <h2 className="cc-panel-surface__title text-lg font-semibold text-[color:var(--cc-ink)]">{title}</h2>
          {titleAccessory ? <div className="cc-panel-surface__title-accessory shrink-0">{titleAccessory}</div> : null}
        </div>
      </div>
      {children}
    </section>
  );
}
