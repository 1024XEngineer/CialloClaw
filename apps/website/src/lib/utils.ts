import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Resolves a public asset path that works both in development
 * (where BASE_URL is "/") and on GitHub Pages (where BASE_URL is
 * "/CialloClaw/").
 *
 * Pass the path relative to the `public/` directory, with or without
 * a leading slash:
 *
 *   assetUrl("assets/icons/logo.png")
 *   => "/assets/icons/logo.png"          (dev)
 *   => "/CialloClaw/assets/icons/logo.png"  (GitHub Pages)
 */
export function assetUrl(path: string): string {
  return `${import.meta.env.BASE_URL}${path.replace(/^\/+/, "")}`;
}
