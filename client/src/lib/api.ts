import type {
  APIErrorPayload,
  ScanJobResponse,
  ScanReport,
} from "@/lib/types";

export class APIError extends Error {
  status: number;
  retryAfterSeconds: number | null;
  job?: ScanJobResponse;

  constructor(
    message: string,
    options: {
      status: number;
      retryAfterSeconds?: number | null;
      job?: ScanJobResponse;
    },
  ) {
    super(message);
    this.name = "APIError";
    this.status = options.status;
    this.retryAfterSeconds = options.retryAfterSeconds ?? null;
    this.job = options.job;
  }
}

export async function submitScan(url: string): Promise<ScanJobResponse> {
  const response = await fetch("/api/scan", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ url }),
    cache: "no-store",
  });

  return parseJobResponse(response, "El escaneo no pudo encolarse");
}

export async function pollScanJob(jobId: string): Promise<ScanJobResponse> {
  const response = await fetch(`/api/scan/${jobId}`, {
    cache: "no-store",
  });

  return parseJobResponse(response, "No se pudo consultar el estado del turno");
}

export function isScanReport(value: unknown): value is ScanReport {
  if (!isRecord(value)) {
    return false;
  }

  return (
    typeof value.url === "string" &&
    typeof value.score === "string" &&
    typeof value.total_bytes_transferred === "number" &&
    Array.isArray(value.vampire_elements)
  );
}

export function isScanJobResponse(value: unknown): value is ScanJobResponse {
  if (!isRecord(value)) {
    return false;
  }

  const status = value.status;
  if (
    typeof value.job_id !== "string" ||
    typeof value.url !== "string" ||
    typeof value.position !== "number" ||
    typeof status !== "string"
  ) {
    return false;
  }

  if (
    status !== "queued" &&
    status !== "scanning" &&
    status !== "completed" &&
    status !== "failed" &&
    status !== "expired"
  ) {
    return false;
  }

  if (
    value.estimated_wait_seconds !== undefined &&
    typeof value.estimated_wait_seconds !== "number"
  ) {
    return false;
  }

  if (value.deduplicated !== undefined && typeof value.deduplicated !== "boolean") {
    return false;
  }

  if (value.error !== undefined && typeof value.error !== "string") {
    return false;
  }

  if (value.report !== undefined && !isScanReport(value.report)) {
    return false;
  }

  return true;
}

async function parseJobResponse(
  response: Response,
  fallbackMessage: string,
): Promise<ScanJobResponse> {
  const retryAfterSeconds = parseRetryAfter(response.headers.get("Retry-After"));
  const payload = (await response.json().catch(() => null)) as unknown;

  if (!response.ok) {
    const errorPayload = asAPIErrorPayload(payload);
    const conflictJob = extractConflictJob(payload);
    throw new APIError(errorPayload?.error ?? fallbackMessage, {
      status: response.status,
      retryAfterSeconds,
      job: conflictJob,
    });
  }

  if (!isScanJobResponse(payload)) {
    throw new Error("La respuesta del servidor no tiene el formato esperado.");
  }

  return payload;
}

function extractConflictJob(payload: unknown): ScanJobResponse | undefined {
  if (!isRecord(payload) || !("job" in payload)) {
    return undefined;
  }

  return isScanJobResponse(payload.job) ? payload.job : undefined;
}

function asAPIErrorPayload(payload: unknown): APIErrorPayload | null {
  if (!isRecord(payload) || typeof payload.error !== "string") {
    return null;
  }

  return {
    error: payload.error,
  };
}

function parseRetryAfter(value: string | null): number | null {
  if (!value) {
    return null;
  }

  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }

  return parsed;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[unitIndex]}`;
}

export function formatGrams(value: number): string {
  return `${value.toFixed(value < 1 ? 3 : 2)} g CO2`;
}

export function formatMilliseconds(value: number): string {
  return `${value.toLocaleString()} ms`;
}

export function formatPercentage(value: number): string {
  return `${value.toFixed(1)}%`;
}

export function formatParty(value: "first_party" | "third_party"): string {
  return value === "first_party" ? "Primera parte" : "Terceros";
}

export function formatRequestStatus(statusCode: number, failed: boolean): string {
  if (failed) {
    return statusCode > 0 ? `Falló (${statusCode})` : "Petición fallida";
  }
  return `HTTP ${statusCode}`;
}

export function formatResourceLabel(value: string): string {
  return value
    .split("_")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

export function formatSignedDelta(value: number, suffix = ""): string {
  if (value === 0) {
    return `0${suffix}`;
  }
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toLocaleString()}${suffix}`;
}

export function formatImpactLabel(value: "high" | "medium" | "low"): string {
  switch (value) {
    case "high":
      return "Impacto alto";
    case "medium":
      return "Impacto medio";
    default:
      return "Impacto bajo";
  }
}

export function formatSeverityLabel(value: "high" | "medium" | "low"): string {
  switch (value) {
    case "high":
      return "Severidad alta";
    case "medium":
      return "Severidad media";
    default:
      return "Severidad baja";
  }
}

export function formatConfidenceLabel(value: "high" | "medium" | "low"): string {
  switch (value) {
    case "high":
      return "Confianza alta";
    case "medium":
      return "Confianza media";
    default:
      return "Confianza baja";
  }
}

export function formatPositionBand(value: string): string {
  switch (value) {
    case "above_fold":
      return "Above the fold";
    case "near_fold":
      return "Near fold";
    case "below_fold":
      return "Below the fold";
    case "mixed":
      return "Mixto";
    default:
      return "Sin banda";
  }
}

export function formatThirdPartyKind(value: string): string {
  switch (value) {
    case "analytics":
      return "Analytics";
    case "ads":
      return "Ads";
    case "support":
      return "Support";
    case "social":
      return "Social";
    case "video_embed":
      return "Video";
    case "payment":
      return "Payment";
    default:
      return "Third-party";
  }
}

export function formatVisualRole(value: string): string {
  switch (value) {
    case "lcp_candidate":
      return "LCP";
    case "hero_media":
      return "Hero";
    case "repeated_card_media":
      return "Repeated card";
    case "above_fold_media":
      return "Above the fold";
    case "below_fold_media":
      return "Below the fold";
    case "decorative":
      return "Decorative";
    default:
      return "Unknown";
  }
}

export function formatEntropyLabel(score: string): string {
  switch (score) {
    case "A":
    case "B":
      return "Baja entropía";
    case "C":
    case "D":
      return "Entropía media";
    default:
      return "Alta entropía";
  }
}

export function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("es-CO", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
