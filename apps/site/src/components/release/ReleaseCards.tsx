import { useEffect, useState } from "react";
import type { ReactElement } from "react";
import { formatBytes, formatDateLabel } from "@/lib/format";
import type { SiteRelease, SiteReleasePayload } from "@/lib/github-releases";
import { Badge, Button, Card, Flex, Heading, Text } from "@radix-ui/themes";

interface ReleaseErrorPayload extends Partial<SiteReleasePayload> {
  error?: string;
}

function readErrorMessage(data: SiteReleasePayload | ReleaseErrorPayload, status: number): string {
  return "error" in data && typeof data.error === "string"
    ? data.error
    : `Release API 请求失败：${status}`;
}

function summarizeNotes(notes: string): string {
  const condensed = notes.replace(/[#>*`]/g, " ").replace(/\s+/g, " ").trim();

  if (!condensed) {
    return "详细版本说明和安装信息请查看 GitHub Release 页面。";
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
      badge: "推荐使用",
      accentClassName: "bg-emerald-400/18 text-emerald-100",
    };
  }

  return {
    title: "Tip Preview",
    badge: "抢先体验",
    accentClassName: "bg-orange-400/18 text-orange-100",
  };
}

function ReleaseCard({ release }: { release: SiteRelease }): ReactElement {
  const meta = channelMeta(release);

  return (
    <Card className="site-soft-card rounded-[2rem] !p-6">
      <Flex justify="between" align="start" gap="4">
        <Flex direction="column" gap="3">
          <Badge color={release.channel === "stable" ? "green" : "orange"} variant="soft" radius="full">发布通道</Badge>
          <Heading size="6">{meta.title}</Heading>
        </Flex>
        <span className={`rounded-full px-3 py-1 text-xs font-medium ${meta.accentClassName}`}>{meta.badge}</span>
      </Flex>

      <div className="mt-5 flex flex-wrap gap-2 text-sm text-white/60">
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">{release.name}</span>
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">{release.tagName}</span>
        <span className="rounded-full border border-white/10 bg-white/6 px-3 py-1.5">发布时间 {formatDateLabel(release.publishedAt)}</span>
      </div>

      <Text as="p" size="3" color="gray" className="mt-5 leading-7">{summarizeNotes(release.notes)}</Text>

      <div className="mt-6 flex flex-wrap gap-3">
        <Button asChild radius="full" size="3" highContrast>
          <a href={release.primaryAsset?.downloadUrl ?? release.htmlUrl}>
            {release.primaryAsset ? `下载 ${release.primaryAsset.name}` : "查看 GitHub Release"}
          </a>
        </Button>
        <Button asChild radius="full" size="3" variant="soft">
          <a href={release.htmlUrl}>版本说明</a>
        </Button>
      </div>

      {release.primaryAsset ? (
        <p className="mt-4 text-sm text-white/56">
          主下载：{release.primaryAsset.kind.toUpperCase()} · {formatBytes(release.primaryAsset.size)} · 已下载 {release.primaryAsset.downloadCount} 次
        </p>
      ) : (
        <p className="mt-4 text-sm text-white/56">当前没有明确的 Windows 主下载包，所以按钮会回退到 Release 页面。</p>
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
    </Card>
  );
}

function LoadingCard(): ReactElement {
  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {Array.from({ length: 2 }, (_, index) => (
        <article key={index} className="site-soft-card rounded-[2rem] p-6">
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
    <Card className="site-soft-card rounded-[2rem] !p-6">
      <Badge color="orange" variant="soft" radius="full">版本同步降级</Badge>
      <Heading size="6" className="mt-4">暂时无法获取 GitHub Release 元数据。</Heading>
      <Text as="p" size="3" color="gray" className="mt-4 max-w-3xl leading-7">{message}</Text>
      <div className="mt-6 flex flex-wrap gap-3">
        <Button asChild radius="full" size="3" highContrast>
          <a href="https://github.com/1024XEngineer/CialloClaw/releases">打开 GitHub Releases</a>
        </Button>
        <Button asChild radius="full" size="3" variant="soft">
          <a href="https://github.com/1024XEngineer/CialloClaw/issues/332#issue-4321666828">官网需求说明</a>
        </Button>
      </div>
    </Card>
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
          setError(loadError instanceof Error ? loadError.message : "未知的版本同步错误");
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
    return <ErrorCard message="接口没有返回 stable 或 tip 的版本数据。请暂时以 GitHub Releases 页面作为下载真源。" />;
  }

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {releases.map((release) => (
        <ReleaseCard key={release.channel} release={release} />
      ))}
    </div>
  );
}
