# CialloClaw Site Deployment Guide

## 1. Scope

This document describes how to build and deploy the CialloClaw product website under `apps/site`.

The website is intentionally independent from:

- `apps/desktop`
- `services/local-service`
- Named Pipe runtime bridges
- desktop worker execution
- local SQLite state

It is a product and download website, not a browser shell for the desktop runtime.

## 2. Current stack

- Framework: Astro
- Interactive islands: React
- Motion layer: Motion
- Styling: Tailwind CSS v4 through the Astro Vite pipeline
- Deployment target: Vercel
- Release sync: server-side GitHub Releases API fetch through `/api/releases`

## 3. Local development

Install dependencies from the repository root:

```bash
corepack pnpm install
```

Start the website locally:

```bash
corepack pnpm --dir apps/site dev
```

Run checks:

```bash
corepack pnpm --dir apps/site check
```

## 4. Current verification status

The current local verification baseline is:

- `corepack pnpm --dir apps/site check` passes
- `astro build` completes through Astro compilation and prerendering

### Windows note

When the Vercel adapter writes `.vercel/output` on this Windows environment, the final bundling step currently fails with a local `EPERM` symlink error while copying traced dependencies.

This is a local filesystem permission limitation in the current environment rather than an application type or route error. The server-side release endpoint and page code still type-check correctly.

For production deployment, rely on Vercel's Linux build environment.

## 5. Vercel project settings

Recommended settings:

- Framework Preset: Astro
- Root Directory: `apps/site`
- Install Command: `pnpm install --frozen-lockfile`
- Build Command: `pnpm build`
- Production Branch: `main`

Environment variables:

- `PUBLIC_SITE_URL=https://cialloclaw.vercel.app`
- `GITHUB_TOKEN=<optional but recommended for release API stability>`

`GITHUB_TOKEN` is only used on the server side by `/api/releases` and must never be exposed to client-side code.

## 6. Release sync behavior

The site release endpoint is:

```text
GET /api/releases
```

It currently:

- reads Stable from `/releases/latest`
- reads Tip Preview from `/releases/tags/tip`
- normalizes release assets
- prioritizes `.exe`, then `.msi`, then `.zip`, then the release page
- returns cache headers suitable for short-term CDN caching

## 7. CI

Website CI is defined in:

```text
.github/workflows/site-check.yml
```

It installs dependencies, runs `pnpm --dir apps/site check`, and then runs `pnpm --dir apps/site build`.

## 8. Next follow-up options

1. Add a static screenshot or OG image pipeline under `apps/site/public/`.
2. Add a changelog route backed by curated markdown content.
3. Revisit local Windows build ergonomics for the Vercel adapter symlink limitation if local `.vercel/output` generation becomes a strict requirement.
