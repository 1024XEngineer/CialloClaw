import type { APIRoute } from "astro";
import { getSiteReleases } from "@/lib/github-releases";

export const prerender = false;

export const GET: APIRoute = async () => {
  try {
    const payload = await getSiteReleases();

    return new Response(JSON.stringify(payload), {
      status: 200,
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Cache-Control": "public, s-maxage=300, stale-while-revalidate=600",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown release API error";

    return new Response(
      JSON.stringify({
        stable: null,
        tip: null,
        updatedAt: new Date().toISOString(),
        source: "github",
        error: message,
      }),
      {
        status: 502,
        headers: {
          "Content-Type": "application/json; charset=utf-8",
          "Cache-Control": "public, s-maxage=60, stale-while-revalidate=120",
        },
      },
    );
  }
};
