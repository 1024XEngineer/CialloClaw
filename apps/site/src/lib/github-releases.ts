export type ReleaseChannel = "stable" | "tip";

export interface SiteReleaseAsset {
  name: string;
  size: number;
  digest?: string;
  downloadUrl: string;
  downloadCount: number;
  contentType: string;
  kind: "exe" | "msi" | "zip" | "other";
}

export interface SiteRelease {
  channel: ReleaseChannel;
  tagName: string;
  name: string;
  prerelease: boolean;
  publishedAt: string | null;
  htmlUrl: string;
  notes: string;
  assets: SiteReleaseAsset[];
  primaryAsset: SiteReleaseAsset | null;
}

export interface SiteReleasePayload {
  stable: SiteRelease | null;
  tip: SiteRelease | null;
  updatedAt: string;
  source: "github";
}

const OWNER = "1024XEngineer";
const REPO = "CialloClaw";
const API_BASE = `https://api.github.com/repos/${OWNER}/${REPO}`;

interface GitHubReleaseAsset {
  name?: string;
  size?: number;
  digest?: string;
  browser_download_url?: string;
  download_count?: number;
  content_type?: string;
}

interface GitHubReleaseResponse {
  tag_name?: string;
  name?: string;
  prerelease?: boolean;
  published_at?: string | null;
  html_url?: string;
  body?: string;
  assets?: GitHubReleaseAsset[];
}

function githubHeaders(): HeadersInit {
  const headers: HeadersInit = {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
  };

  if (process.env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  }

  return headers;
}

function classifyAsset(name: string): SiteReleaseAsset["kind"] {
  const normalized = name.toLowerCase();

  if (normalized.endsWith(".exe")) {
    return "exe";
  }

  if (normalized.endsWith(".msi")) {
    return "msi";
  }

  if (normalized.endsWith(".zip")) {
    return "zip";
  }

  return "other";
}

function pickPrimaryAsset(assets: SiteReleaseAsset[]): SiteReleaseAsset | null {
  return (
    assets.find((asset) => asset.kind === "exe") ??
    assets.find((asset) => asset.kind === "msi") ??
    assets.find((asset) => asset.kind === "zip") ??
    assets[0] ??
    null
  );
}

function normalizeAssets(rawAssets: GitHubReleaseAsset[] | undefined): SiteReleaseAsset[] {
  if (!Array.isArray(rawAssets)) {
    return [];
  }

  return rawAssets
    .filter((asset): asset is Required<Pick<GitHubReleaseAsset, "name" | "browser_download_url">> & GitHubReleaseAsset => {
      return typeof asset.name === "string" && typeof asset.browser_download_url === "string";
    })
    .map((asset) => ({
      name: asset.name,
      size: typeof asset.size === "number" ? asset.size : 0,
      digest: asset.digest,
      downloadUrl: asset.browser_download_url,
      downloadCount: typeof asset.download_count === "number" ? asset.download_count : 0,
      contentType: asset.content_type ?? "application/octet-stream",
      kind: classifyAsset(asset.name),
    }));
}

function normalizeRelease(raw: GitHubReleaseResponse, channel: ReleaseChannel): SiteRelease {
  const assets = normalizeAssets(raw.assets);

  return {
    channel,
    tagName: raw.tag_name ?? channel,
    name: raw.name ?? raw.tag_name ?? channel,
    prerelease: Boolean(raw.prerelease),
    publishedAt: raw.published_at ?? null,
    htmlUrl: raw.html_url ?? `https://github.com/${OWNER}/${REPO}/releases`,
    notes: raw.body ?? "",
    assets,
    primaryAsset: pickPrimaryAsset(assets),
  };
}

async function fetchJson(url: string): Promise<GitHubReleaseResponse | null> {
  const response = await fetch(url, {
    headers: githubHeaders(),
  });

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`GitHub release request failed: ${response.status}`);
  }

  return (await response.json()) as GitHubReleaseResponse;
}

export async function getSiteReleases(): Promise<SiteReleasePayload> {
  const [stableRaw, tipRaw] = await Promise.all([
    fetchJson(`${API_BASE}/releases/latest`),
    fetchJson(`${API_BASE}/releases/tags/tip`),
  ]);

  return {
    stable: stableRaw ? normalizeRelease(stableRaw, "stable") : null,
    tip: tipRaw ? normalizeRelease(tipRaw, "tip") : null,
    updatedAt: new Date().toISOString(),
    source: "github",
  };
}
