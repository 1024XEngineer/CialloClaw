import { useEffect, useState } from "react";
import type { ReactElement } from "react";
import { formatBytes, formatDateLabel } from "@/lib/format";
import type { SiteRelease, SiteReleasePayload } from "@/lib/github-releases";

interface ReleaseErrorPayload extends Partial<SiteReleasePayload> {
  error?: string;
}

function readErrorMessage(data: SiteReleasePayload | ReleaseErrorPayload, status: number): string {
  return "error" in data && typeof data.error === "string"
    ? data.error
    : `Release API failed: ${status}`;
}

function summarizeNotes(notes: string): string {
  const condensed = notes.replace(/[#>*`]/g, " ").replace(/\s+/g, " ").trim();

  if (!condensed) {
    return "Release notes are available on GitHub for the latest details and installation guidance.";
  }

  if (condensed.length <= 180) {
    return condensed;
  }

  return `${condensed.slice(0, 177)}...`;
}

function channelMeta(release: SiteRelease): { badge: string; accentClassName: string; title: string } {
  if (release.channel === "stable") {
    return {
      title: "Stable",
      badge: "Recommended",
      accentClassName: "bg-emerald-400/18 text-emerald-100",
    };
  }

  return {
    title: "Tip Preview",
    badge: "Fast lane",
    accentClassName: "bg-orange-400/18 text-orange-100",
  };
}

function ReleaseCard({ release }: { release: SiteRelease }): ReactElement {
  const meta = channelMeta(release);

  return (
    <article className="glass-card rounded-[2rem] p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.24em] text-white/56">Release channel</p>
          <h3 className="mt-3 font-display text-3xl font-semibold text-white">{meta.title}</h3>
        </div>
        <span className={`rounded-full px-3 py-1 text-xs font-medium ${meta.accentClassName}`}>{meta.badge}</span>
      </div>

      <div className="mt-5 flex flex-wrap gap-2 text-sm text-white/60">
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">{release.name}</span>
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">{release.tagName}</span>
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">Published {formatDateLabel(release.publishedAt)}</span>
      </div>

      <p className="mt-5 text-sm leading-7 text-white/68">{summarizeNotes(release.notes)}</p>

      <div className="mt-6 flex flex-wrap gap-3">
        <a
          className="focus-ring inline-flex rounded-full bg-white px-5 py-3 text-sm font-semibold text-slate-950 transition hover:scale-[1.02]"
          href={release.primaryAsset?.downloadUrl ?? release.htmlUrl}
        >
          {release.primaryAsset ? `Download ${release.primaryAsset.name}` : "View GitHub release"}
        </a>
        <a className="focus-ring inline-flex rounded-full border border-white/10 bg-white/6 px-5 py-3 text-sm font-semibold text-white transition hover:bg-white/10" href={release.htmlUrl}>
          Release notes
        </a>
      </div>

      {release.primaryAsset ? (
        <p className="mt-4 text-sm text-white/56">
          Primary asset: {release.primaryAsset.kind.toUpperCase()} · {formatBytes(release.primaryAsset.size)} · {release.primaryAsset.downloadCount} downloads
        </p>
      ) : (
        <p className="mt-4 text-sm text-white/56">No Windows asset is currently marked as primary, so the card links to the release page.</p>
      )}

      {release.assets.length > 0 ? (
        <div className="mt-5 flex flex-wrap gap-2">
          {release.assets.map((asset) => (
            <a
              key={`${release.channel}-${asset.name}`}
              className="focus-ring rounded-full border border-white/10 bg-slate-950/48 px-3 py-1.5 text-xs font-medium uppercase tracking-[0.16em] text-white/72 transition hover:border-white/18 hover:text-white"
              href={asset.downloadUrl}
            >
              {asset.kind} · {formatBytes(asset.size)}
            </a>
          ))}
        </div>
      ) : null}
    </article>
  );
}

function LoadingCard(): ReactElement {
  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {Array.from({ length: 2 }, (_, index) => (
        <article key={index} className="glass-card rounded-[2rem] p-6">
          <div className="h-4 w-28 animate-pulse rounded-full bg-white/10"></div>
          <div className="mt-4 h-10 w-44 animate-pulse rounded-full bg-white/10"></div>
          <div className="mt-6 h-24 animate-pulse rounded-[1.5rem] bg-white/8"></div>
        </article>
      ))}
    </div>
  );
}

function ErrorCard({ message }: { message: string }): ReactElement {
  return (
    <article className="glass-card rounded-[2rem] p-6">
      <p className="text-sm uppercase tracking-[0.24em] text-cc-peach">Release sync fallback</p>
      <h3 className="mt-4 font-display text-3xl font-semibold text-white">GitHub release metadata is temporarily unavailable.</h3>
      <p className="mt-4 max-w-3xl text-sm leading-7 text-white/68">{message}</p>
      <div className="mt-6 flex flex-wrap gap-3">
        <a className="focus-ring inline-flex rounded-full bg-white px-5 py-3 text-sm font-semibold text-slate-950 transition hover:scale-[1.02]" href="https://github.com/1024XEngineer/CialloClaw/releases">
          Open GitHub releases
        </a>
        <a className="focus-ring inline-flex rounded-full border border-white/10 bg-white/6 px-5 py-3 text-sm font-semibold text-white transition hover:bg-white/10" href="https://github.com/1024XEngineer/CialloClaw/issues/332#issue-4321666828">
          Website requirements
        </a>
      </div>
    </article>
  );
}

export function ReleaseCards(): ReactElement {
  const [payload, setPayload] = useState<SiteReleasePayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadReleases(): Promise<void> {
      try {
        const response = await fetch("/api/releases");
        const data = (await response.json()) as SiteReleasePayload | ReleaseErrorPayload;

        if (!response.ok) {
          throw new Error(readErrorMessage(data, response.status));
        }

        if (!cancelled) {
          setPayload(data as SiteReleasePayload);
          setError(null);
        }
      } catch (loadError) {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : "Unknown release sync error");
        }
      }
    }

    void loadReleases();

    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return <ErrorCard message={error} />;
  }

  if (!payload) {
    return <LoadingCard />;
  }

  const releases = [payload.stable, payload.tip].filter((release): release is SiteRelease => release !== null);

  if (releases.length === 0) {
    return <ErrorCard message="No stable or tip release data was returned. Please use the GitHub Releases page as the current source for downloadable builds." />;
  }

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {releases.map((release) => (
        <ReleaseCard key={release.channel} release={release} />
      ))}
    </div>
  );
}
