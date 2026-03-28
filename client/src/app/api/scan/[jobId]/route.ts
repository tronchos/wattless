import { NextResponse } from "next/server";

import {
  clientIdentityCookieName,
  forwardScannerRequest,
} from "@/lib/server-api";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const allowedUpstreamStatuses = new Set([200, 202, 400, 404, 409, 410, 429, 503]);

export async function GET(
  request: Request,
  { params }: { params: Promise<{ jobId: string }> },
) {
  const { jobId } = await params;

  try {
    const {
      upstream,
      clientIdentity,
      shouldSetCookie,
    } = await forwardScannerRequest(
      request,
      `/api/v1/scans/${jobId}`,
      { method: "GET" },
    );
    const payload = await upstream.json().catch(() => {
      console.error("[scan job route] Failed to parse upstream JSON response", {
        status: upstream.status,
        jobId,
      });
      return { error: "Error inesperado" };
    });

    if (!allowedUpstreamStatuses.has(upstream.status)) {
      console.error("[scan job route] Unexpected upstream status", {
        status: upstream.status,
        jobId,
      });
      return NextResponse.json(
        { error: "Respuesta inesperada del escáner" },
        { status: 502 },
      );
    }

    const response = NextResponse.json(payload, { status: upstream.status });
    copyRetryAfter(upstream, response);
    setClientIdentityCookie(response, clientIdentity, shouldSetCookie);
    return response;
  } catch (err) {
    console.error("[scan job route] Failed to contact scanner", err);
    return NextResponse.json(
      { error: "No se pudo contactar con el escáner" },
      { status: 502 },
    );
  }
}

function copyRetryAfter(upstream: Response, response: NextResponse) {
  const retryAfter = upstream.headers.get("Retry-After");
  if (retryAfter?.trim()) {
    response.headers.set("Retry-After", retryAfter.trim());
  }
}

function setClientIdentityCookie(
  response: NextResponse,
  clientIdentity: string,
  shouldSetCookie: boolean,
) {
  if (!shouldSetCookie) {
    return;
  }

  response.cookies.set({
    name: clientIdentityCookieName,
    value: clientIdentity,
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });
}
