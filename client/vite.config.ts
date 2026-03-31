import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tsconfigPaths from "vite-tsconfig-paths";
import {
  resolveAbsolutePublicAssetURL,
  resolvePublicAppURL,
} from "./public-meta";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const proxyTarget = env.VITE_PROXY_TARGET?.trim() || "http://localhost:8080";
  const publicAppURL = resolvePublicAppURL(env.VITE_PUBLIC_APP_URL);
  const ogImageURL = resolveAbsolutePublicAssetURL(env.VITE_PUBLIC_APP_URL, "/og-image.svg");

  return {
    plugins: [
      {
        name: "wattless-social-metadata",
        transformIndexHtml(html) {
          return html
            .replaceAll("__WATTLESS_PUBLIC_APP_URL__", publicAppURL)
            .replaceAll("__WATTLESS_OG_IMAGE_URL__", ogImageURL);
        },
      },
      react(),
      tsconfigPaths(),
    ],
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/api": {
          target: proxyTarget,
          changeOrigin: true,
        },
        "/healthz": {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
    preview: {
      host: "0.0.0.0",
      port: 4173,
    },
  };
});
