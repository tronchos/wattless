import { NextResponse } from "next/server";

import { forwardJSON } from "@/lib/server-api";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(request: Request) {
  const body = await request.json().catch(() => null);
  if (!body || typeof body.code !== "string") {
    return NextResponse.json({ error: "Payload inválido" }, { status: 400 });
  }

  try {
    const upstream = await forwardJSON("/api/v1/green-fix", body);
    const payload = await upstream.json().catch(() => ({ error: "Error inesperado" }));

    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "No se pudo contactar con la capa de IA" },
      { status: 502 },
    );
  }
}
