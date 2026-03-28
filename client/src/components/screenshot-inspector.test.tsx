import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

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
  beforeEach(() => {
    let nextURL = 0;
    vi.spyOn(URL, "createObjectURL").mockImplementation(() => {
      nextURL += 1;
      return `blob:tile-${nextURL}`;
    });
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders tiles using blob URLs and revokes them on unmount", async () => {
    const { unmount } = render(
      <ScreenshotInspector
        screenshot={baseScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    expect(URL.createObjectURL).toHaveBeenCalledTimes(2);
    expect((await screen.findByAltText("Document tile tile-1")).getAttribute("src")).toBe(
      "blob:tile-1",
    );
    expect((await screen.findByAltText("Document tile tile-2")).getAttribute("src")).toBe(
      "blob:tile-2",
    );

    unmount();

    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:tile-1");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:tile-2");
  });

  it("revokes previous tile URLs when the screenshot changes", async () => {
    const { rerender } = render(
      <ScreenshotInspector
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
        screenshot={nextScreenshot}
        elements={baseElements}
        selectedElement={null}
        selectionSignal={0}
        onSelect={() => {}}
      />,
    );

    await waitFor(() => {
      expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:tile-1");
      expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:tile-2");
    });
    expect((await screen.findByAltText("Document tile tile-3")).getAttribute("src")).toBe(
      "blob:tile-3",
    );
  });
});
