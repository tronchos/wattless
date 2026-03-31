import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@/components/scan-workbench", () => ({
  ScanWorkbench: () => <div>Workbench</div>,
}));

import App from "./App";

describe("App", () => {
  it("uses Wattless as the only visible brand and exposes semantic navigation", () => {
    render(<App />);

    expect(screen.getAllByText("Wattless").length).toBeGreaterThan(0);
    const navigation = screen.getByRole("navigation", { name: /navegación principal/i });

    expect(navigation).toBeDefined();
    expect(screen.getAllByRole("link", { name: "Metodología" }).length).toBeGreaterThan(0);
    expect(screen.queryByText(/Digital Biome Auditor/i)).toBeNull();
    expect(screen.queryByText(/Documentation|Privacy Policy|Carbon Calculator/i)).toBeNull();
  });
});
