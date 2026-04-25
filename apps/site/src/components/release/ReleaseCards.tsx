import { useEffect, useState } from "react";
import type { ReactElement } from "react";
import { formatBytes, formatDateLabel } from "@/lib/format";
import type { SiteRelease, SiteReleasePayload } from "@/lib/github-releases";
import { Badge, Button, Card, Heading, Text } from "@radix-ui/themes";

interface ReleaseErrorPayload extends Partial<SiteReleasePayload> {
  error?: string;
}

function readErrorMessage(data: SiteReleasePayload | ReleaseErrorPayload, status: number): string {
  return "error" in data && typeof data.error === "string"
    ? data.error
    : `版本接口请求失败：${status}`;
}

function releaseTitle(release: SiteRelease): string {
  return release.channel === "stable" ? "Stable 通道" : "Tip Preview 通道";
}

function ReleaseCard({ release, updatedAt }: { release: SiteRelease; updatedAt: string }): ReactElement {
  return (
    <Card className="site-soft-card rounded-[2rem] !p-6 lg:!p-8">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.05fr)_minmax(18rem,0.95fr)] lg:items-center">
        <div>
          <Badge color={release.channel === "stable" ? "green" : "orange"} variant="soft" radius="full">
            GitHub 版本同步
          </Badge>
          <Heading size="8" className="mt-4">
            {releaseTitle(release)}
          </Heading>
            <Text as="p" size="3" color="gray" className="mt-4 max-w-2xl leading-7">
            当前首页只展示推荐安装包，保持下载区域清爽直接。
            预览版、历史版本和完整更新说明统一放在 GitHub 版本页查看。
          </Text>

          <div className="mt-7 flex flex-wrap gap-3">
            <Button asChild radius="full" size="4" highContrast>
              <a href={release.primaryAsset?.downloadUrl ?? release.htmlUrl}>
                {release.primaryAsset ? "下载 Windows 安装包" : "打开 GitHub 版本页"}
              </a>
            </Button>
            <Button asChild radius="full" size="4" variant="soft">
              <a href={release.htmlUrl}>查看全部版本</a>
            </Button>
          </div>

          <p className="mt-4 text-sm text-white/56">最近同步：{formatDateLabel(updatedAt)}</p>
        </div>

        <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
          <div className="rounded-[1.45rem] border border-white/8 bg-white/6 p-4">
            <p className="text-xs uppercase tracking-[0.22em] text-white/46">版本</p>
            <p className="mt-2 text-base font-semibold text-white">{release.name}</p>
          </div>
          <div className="rounded-[1.45rem] border border-white/8 bg-white/6 p-4">
            <p className="text-xs uppercase tracking-[0.22em] text-white/46">发布时间</p>
            <p className="mt-2 text-base font-semibold text-white">{formatDateLabel(release.publishedAt)}</p>
          </div>
          <div className="rounded-[1.45rem] border border-white/8 bg-white/6 p-4">
            <p className="text-xs uppercase tracking-[0.22em] text-white/46">安装包</p>
            <p className="mt-2 text-base font-semibold text-white">
              {release.primaryAsset ? formatBytes(release.primaryAsset.size) : "前往版本页"}
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
}

function LoadingCard(): ReactElement {
  return (
    <article className="site-soft-card rounded-[2rem] p-6">
      <div className="h-4 w-28 animate-pulse rounded-full bg-white/10"></div>
      <div className="mt-4 h-12 w-48 animate-pulse rounded-full bg-white/10"></div>
      <div className="mt-6 h-28 animate-pulse rounded-[1.5rem] bg-white/8"></div>
    </article>
  );
}

function ErrorCard({ message }: { message: string }): ReactElement {
  return (
    <Card className="site-soft-card rounded-[2rem] !p-6">
      <Badge color="orange" variant="soft" radius="full">版本同步降级</Badge>
      <Heading size="6" className="mt-4">暂时无法获取 GitHub 版本元数据。</Heading>
      <Text as="p" size="3" color="gray" className="mt-4 max-w-3xl leading-7">{message}</Text>
      <div className="mt-6 flex flex-wrap gap-3">
        <Button asChild radius="full" size="3" highContrast>
          <a href="https://github.com/1024XEngineer/CialloClaw/releases">打开 GitHub 版本页</a>
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

  const latest = payload.stable ?? payload.tip;

  if (!latest) {
    return <ErrorCard message="接口没有返回可用的最新版本数据。请暂时以 GitHub 版本页作为下载真源。" />;
  }

  return <ReleaseCard release={latest} updatedAt={payload.updatedAt} />;
}
