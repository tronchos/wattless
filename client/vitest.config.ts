import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig(async () => {
  const { default: tsconfigPaths } = await import("vite-tsconfig-paths");

  return {
    plugins: [react(), tsconfigPaths()],
    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: [],
    },
  };
});
