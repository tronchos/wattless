import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { ScreenshotInspector } from "./screenshot-inspector";
import type { ScreenshotPayload, VampireElement } from "@/lib/types";

const baseScreenshot: ScreenshotPayload = {
  mime_type: "image/png",
  strategy: "tiled",
  viewport_width: 1200,
  viewport_height: 900,
  document_width: 1200,
  document_height: 1800,
  captured_height: 1800,
  tiles: [
    {
      id: "tile-1",
      y: 0,
      width: 1200,
      height: 900,
      data_base64: "AQ==",
    },
    {
      id: "tile-2",
      y: 900,
      width: 1200,
      height: 900,
      data_base64: "Ag==",
    },
  ],
};

const baseElements: VampireElement[] = [];

describe("ScreenshotInspector", () => {
  it("renders tiles using the server screenshot endpoint as decorative images", () => {
    const { container } = render(
      <ScreenshotInspector
        jobId="wl_job"
        screenshot={baseScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    const images = Array.from(container.querySelectorAll("img"));

    expect(images[0]?.getAttribute("src")).toBe(
      "/api/v1/scans/wl_job/screenshot?tile=0",
    );
    expect(images[0]?.getAttribute("alt")).toBe("");
    expect(images[0]?.getAttribute("aria-hidden")).toBe("true");
    expect(images[1]?.getAttribute("src")).toBe(
      "/api/v1/scans/wl_job/screenshot?tile=1",
    );
  });

  it("updates tile URLs when the job changes", () => {
    const { container, rerender } = render(
      <ScreenshotInspector
        jobId="wl_job"
        screenshot={baseScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    const nextScreenshot: ScreenshotPayload = {
      ...baseScreenshot,
      tiles: [
        {
          id: "tile-3",
          y: 0,
          width: 1200,
          height: 1800,
          data_base64: "Aw==",
        },
      ],
    };

    rerender(
      <ScreenshotInspector
        jobId="wl_job_2"
        screenshot={nextScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    expect(container.querySelector("img")?.getAttribute("src")).toBe(
      "/api/v1/scans/wl_job_2/screenshot?tile=0",
    );
  });

  it("renders a composed long capture with translated labels", () => {
    const composedScreenshot: ScreenshotPayload = {
      ...baseScreenshot,
      strategy: "single",
      document_height: 4200,
      captured_height: 4200,
      tiles: [
        {
          id: "tile-0",
          y: 0,
          width: 1200,
          height: 4200,
          data_base64: "AQ==",
        },
      ],
    };

    const { container } = render(
      <ScreenshotInspector
        jobId="wl_single"
        screenshot={composedScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    expect(container.querySelector("img")?.getAttribute("src")).toBe(
      "/api/v1/scans/wl_single/screenshot?tile=0",
    );
    expect(screen.queryByText("Captura segmentada")).toBeNull();
    expect(screen.getByText("Altura completa")).toBeDefined();
    expect(screen.getByRole("region", { name: /inspector visual/i })).toBeDefined();
  });
});
