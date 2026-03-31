import { createReadStream, existsSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, normalize, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const distDir = resolve(__dirname, "dist");
const indexPath = resolve(distDir, "index.html");
const port = Number.parseInt(process.env.PORT ?? "4173", 10) || 4173;
const host = "0.0.0.0";

const contentTypes = new Map([
  [".css", "text/css; charset=utf-8"],
  [".html", "text/html; charset=utf-8"],
  [".ico", "image/x-icon"],
  [".js", "application/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".png", "image/png"],
  [".svg", "image/svg+xml"],
  [".txt", "text/plain; charset=utf-8"],
  [".webp", "image/webp"],
]);

createServer((req, res) => {
  const method = req.method ?? "GET";
  if (method !== "GET" && method !== "HEAD") {
    res.writeHead(405, { "Content-Type": "text/plain; charset=utf-8" });
    res.end("Method Not Allowed");
    return;
  }

  const requestURL = new URL(req.url ?? "/", `http://${req.headers.host ?? "localhost"}`);
  if (requestURL.pathname === "/healthz") {
    respondWithBuffer(
      res,
      Buffer.from(JSON.stringify({ status: "ok" })),
      "application/json; charset=utf-8",
      "no-store",
      method,
    );
    return;
  }

  const filePath = resolveRequestPath(requestURL.pathname);
  if (!filePath) {
    respondWithStatus(res, 403, method);
    return;
  }

  if (existsSync(filePath) && statSync(filePath).isFile()) {
    respondWithFile(res, filePath, method);
    return;
  }

  if (extname(requestURL.pathname)) {
    respondWithStatus(res, 404, method);
    return;
  }

  respondWithFile(res, indexPath, method, "no-cache");
}).listen(port, host, () => {
  console.log(`Wattless client listening on http://${host}:${port}`);
});

function resolveRequestPath(pathname) {
  const decodedPath = decodeURIComponent(pathname);
  const normalizedPath = normalize(decodedPath).replace(/^(\.\.(\/|\\|$))+/, "");
  const requestedPath = normalizedPath === "/" ? "index.html" : normalizedPath.replace(/^\/+/, "");
  const filePath = resolve(distDir, requestedPath);

  if (!filePath.startsWith(distDir)) {
    return null;
  }

  return filePath;
}

function respondWithFile(res, filePath, method, cacheControl) {
  const extension = extname(filePath);
  const contentType = contentTypes.get(extension) ?? "application/octet-stream";
  const caching = cacheControl ?? (filePath === indexPath ? "no-cache" : "public, max-age=31536000, immutable");

  res.writeHead(200, {
    "Content-Type": contentType,
    "Cache-Control": caching,
  });

  if (method === "HEAD") {
    res.end();
    return;
  }

  createReadStream(filePath).pipe(res);
}

function respondWithBuffer(res, buffer, contentType, cacheControl, method) {
  res.writeHead(200, {
    "Content-Type": contentType,
    "Cache-Control": cacheControl,
    "Content-Length": buffer.byteLength,
  });

  if (method === "HEAD") {
    res.end();
    return;
  }

  res.end(buffer);
}

function respondWithStatus(res, statusCode, method) {
  res.writeHead(statusCode, { "Content-Type": "text/plain; charset=utf-8" });
  if (method === "HEAD") {
    res.end();
    return;
  }

  res.end(statusCode === 404 ? "Not Found" : "Forbidden");
}
