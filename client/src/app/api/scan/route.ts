import { NextResponse } from "next/server";

import { forwardJSON } from "@/lib/server-api";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(request: Request) {
  const body = await request.json().catch(() => null);
  if (!body || typeof body.url !== "string" || !body.url.trim()) {
    return NextResponse.json({ error: "Payload inválido" }, { status: 400 });
  }

  let normalizedURL = body.url.trim();
  if (!/^https?:\/\//i.test(normalizedURL)) {
    normalizedURL = `https://${normalizedURL}`;
  }

  try {
    new URL(normalizedURL);
  } catch {
    return NextResponse.json(
      { error: "La URL no es válida" },
      { status: 400 },
    );
  }

  try {
    const upstream = await forwardJSON("/api/v1/scans", { url: normalizedURL });
    const payload = await upstream.json().catch(() => {
      console.error("[scan route] Failed to parse upstream JSON response", {
        status: upstream.status,
        url: normalizedURL,
      });
      return { error: "Error inesperado" };
    }) as
      | { url?: string; [key: string]: unknown }
      | { error: string };

    return NextResponse.json(payload, { status: upstream.status });
  } catch (err) {
    console.error("[scan route] Failed to contact scanner", err);
    return NextResponse.json(
      { error: "No se pudo contactar con el escáner" },
      { status: 502 },
    );
  }
}
