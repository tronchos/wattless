import type {
  APIErrorPayload,
  GreenFixRequest,
  GreenFixResponse,
  ScanReport,
} from "@/lib/types";

export async function scanURL(url: string): Promise<ScanReport> {
  const response = await fetch("/api/scan", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ url }),
    cache: "no-store",
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as APIErrorPayload | null;
    throw new Error(payload?.error ?? "El escaneo falló");
  }

  return (await response.json()) as ScanReport;
}

export async function generateGreenFix(
  request: GreenFixRequest,
): Promise<GreenFixResponse> {
  const response = await fetch("/api/green-fix", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
    cache: "no-store",
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as APIErrorPayload | null;
    throw new Error(payload?.error ?? "No se pudo generar el Green Fix");
  }

  return (await response.json()) as GreenFixResponse;
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
