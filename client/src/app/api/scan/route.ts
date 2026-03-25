import { NextResponse } from "next/server";

import {
  rewriteResponseURLs,
  rewriteSelfHostedURL,
} from "@/lib/self-hosted-urls";
import { forwardJSON } from "@/lib/server-api";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(request: Request) {
  const body = await request.json().catch(() => null);
  if (!body || typeof body.url !== "string") {
    return NextResponse.json({ error: "Payload inválido" }, { status: 400 });
  }

  const originalURL = body.url;
  const scannedURL = rewriteSelfHostedURL(
    {
      requestURL: request.url,
      requestHost: request.headers.get("host"),
      forwardedHost: request.headers.get("x-forwarded-host"),
      forwardedProto: request.headers.get("x-forwarded-proto"),
      forwardedPort: request.headers.get("x-forwarded-port"),
      appBaseURL: process.env.APP_BASE_URL,
      internalBaseURL: process.env.SCANNER_SELF_BASE_URL,
    },
    originalURL,
  );

  try {
    const upstream = await forwardJSON("/api/v1/scans", {
      ...body,
      url: scannedURL,
    });
    const payload = await upstream.json().catch(() => ({ error: "Error inesperado" })) as
      | { url?: string; [key: string]: unknown }
      | { error: string };

    if (payload && typeof payload === "object" && "url" in payload) {
      payload.url = originalURL;
      if (scannedURL !== originalURL) {
        rewriteResponseURLs(payload, scannedURL, originalURL);
      }
    }

    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "No se pudo contactar con el escáner" },
      { status: 502 },
    );
  }
}
