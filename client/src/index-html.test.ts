import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import {
  resolveAbsolutePublicAssetURL,
  resolvePublicAppURL,
} from "../public-meta";

describe("index.html metadata", () => {
  it("includes favicon and social metadata for Wattless", () => {
    const html = readFileSync(resolve(import.meta.dirname, "../index.html"), "utf8");

    expect(html).toContain('<link rel="icon" type="image/svg+xml" href="/favicon.svg" />');
    expect(html).toContain('<meta name="theme-color" content="#34d399" />');
    expect(html).toContain('<meta property="og:title" content="Wattless" />');
    expect(html).toContain('<meta property="og:image" content="__WATTLESS_OG_IMAGE_URL__" />');
    expect(html).toContain('<meta property="og:url" content="__WATTLESS_PUBLIC_APP_URL__" />');
    expect(html).toContain('<meta name="twitter:image" content="__WATTLESS_OG_IMAGE_URL__" />');
    expect(html).toContain('<meta name="twitter:card" content="summary_large_image" />');
  });

  it("resolves absolute social asset URLs from the configured public app URL", () => {
    expect(resolvePublicAppURL("https://wattless.example/app/")).toBe("https://wattless.example/app");
    expect(resolveAbsolutePublicAssetURL("https://wattless.example/app/", "/og-image.svg")).toBe(
      "https://wattless.example/app/og-image.svg",
    );
  });

  it("falls back to the local dev origin when the public app URL is missing", () => {
    expect(resolvePublicAppURL(undefined)).toBe("http://localhost:5173");
    expect(resolveAbsolutePublicAssetURL(undefined, "/og-image.svg")).toBe(
      "http://localhost:5173/og-image.svg",
    );
  });
});
