import type { ReactElement } from "react";

function PlaceholderCard(props: {
  title: string;
  description: string;
  badge: string;
  accentClassName: string;
}): ReactElement {
  return (
    <article className="glass-card rounded-[2rem] p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.24em] text-white/56">Release channel</p>
          <h3 className="mt-3 font-display text-3xl font-semibold text-white">{props.title}</h3>
        </div>
        <span className={`rounded-full px-3 py-1 text-xs font-medium ${props.accentClassName}`}>{props.badge}</span>
      </div>

      <p className="mt-4 text-sm leading-7 text-white/68">{props.description}</p>

      <div className="mt-6 flex flex-wrap gap-3">
        <a className="focus-ring inline-flex rounded-full bg-white px-5 py-3 text-sm font-semibold text-slate-950 transition hover:scale-[1.02]" href="https://github.com/1024XEngineer/CialloClaw/releases">
          View releases
        </a>
        <a className="focus-ring inline-flex rounded-full border border-white/10 bg-white/6 px-5 py-3 text-sm font-semibold text-white transition hover:bg-white/10" href="https://github.com/1024XEngineer/CialloClaw/issues/332#issue-4321666828">
          Website requirements
        </a>
      </div>
    </article>
  );
}

export function ReleaseCards(): ReactElement {
  return (
    <div className="grid gap-5 lg:grid-cols-2">
      <PlaceholderCard
        title="Stable"
        badge="Recommended"
        accentClassName="bg-emerald-400/18 text-emerald-100"
        description="Stable is the default channel for most users. The next slice will wire this card to the GitHub Releases API so the latest Windows installer is always current."
      />
      <PlaceholderCard
        title="Tip Preview"
        badge="Fast lane"
        accentClassName="bg-orange-400/18 text-orange-100"
        description="Tip Preview is intended for developers and early testers. This card will also be switched to live GitHub metadata rather than hardcoded filenames."
      />
    </div>
  );
}
