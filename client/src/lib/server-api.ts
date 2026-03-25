const defaultDevelopmentScannerURL = "http://localhost:8080";

function getScannerAPIURL(): string {
  if (process.env.SCANNER_API_URL) {
    return process.env.SCANNER_API_URL;
  }

  if (process.env.NODE_ENV !== "production") {
    return defaultDevelopmentScannerURL;
  }

  throw new Error("SCANNER_API_URL no está configurado");
}

export async function forwardJSON(
  path: string,
  body: unknown,
): Promise<Response> {
  return fetch(`${getScannerAPIURL()}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    cache: "no-store",
  });
}

export async function fetchHealth(): Promise<Response> {
  return fetch(`${getScannerAPIURL()}/healthz`, {
    cache: "no-store",
  });
}
