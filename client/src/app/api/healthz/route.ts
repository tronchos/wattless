import { NextResponse } from "next/server";

import { fetchHealth } from "@/lib/server-api";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  const upstream = await fetchHealth().catch(() => null);
  if (!upstream || !upstream.ok) {
    return NextResponse.json(
      { status: "degraded", scanner: "unreachable" },
      { status: 503 },
    );
  }

  return NextResponse.json({
    status: "ok",
    scanner: "ok",
  });
}
