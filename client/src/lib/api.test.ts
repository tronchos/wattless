import { afterEach, describe, expect, it, vi } from "vitest";

import { APIError, fetchInsights } from "./api";

describe("fetchInsights", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
  });

  it("returns null when the insights endpoint responds 404", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "disabled", code: "insights_unavailable" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_missing")).resolves.toBeNull();
  });

  it("throws APIError when the insights endpoint responds 404 for a missing job", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "missing", code: "job_not_found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_missing")).rejects.toBeInstanceOf(APIError);
  });

  it("returns the parsed ready payload", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "wl_ready",
          status: "ready",
          insights: {
            provider: "gemini",
            executive_summary: "Resumen Gemini",
            pitch_line: "Pitch line",
            top_actions: [],
          },
          vampire_elements: [],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await expect(fetchInsights("wl_ready")).resolves.toMatchObject({
      job_id: "wl_ready",
      status: "ready",
    });
  });

  it("throws APIError for expired jobs", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "expired" }), {
        status: 410,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_expired")).rejects.toBeInstanceOf(APIError);
  });
});
