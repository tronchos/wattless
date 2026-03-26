const defaultDevelopmentScannerURLs = [
  "http://localhost:8080",
  "http://localhost:8081",
  "http://localhost:18080",
] as const;

let resolvedScannerURL: string | null = null;
let pendingScannerURL: Promise<string> | null = null;

async function getScannerAPIURL(): Promise<string> {
  if (process.env.SCANNER_API_URL?.trim()) {
    return process.env.SCANNER_API_URL.trim();
  }

  if (process.env.NODE_ENV === "production") {
    throw new Error("SCANNER_API_URL no está configurado");
  }

  if (resolvedScannerURL) {
    return resolvedScannerURL;
  }

  if (!pendingScannerURL) {
    pendingScannerURL = resolveDevelopmentScannerURL();
  }

  try {
    resolvedScannerURL = await pendingScannerURL;
    return resolvedScannerURL;
  } finally {
    pendingScannerURL = null;
  }
}

async function resolveDevelopmentScannerURL(): Promise<string> {
  for (const candidate of defaultDevelopmentScannerURLs) {
    if (await isHealthyScanner(candidate)) {
      return candidate;
    }
  }

  throw new Error(
    "No se encontró un escáner activo. Define SCANNER_API_URL o levanta el backend en 8080, 8081 o 18080.",
  );
}

async function isHealthyScanner(baseURL: string): Promise<boolean> {
  try {
    const response = await fetch(`${baseURL}/healthz`, {
      cache: "no-store",
      signal: AbortSignal.timeout(800),
    });
    return response.ok;
  } catch {
    return false;
  }
}

export async function forwardJSON(
  path: string,
  body: unknown,
): Promise<Response> {
  const scannerAPIURL = await getScannerAPIURL();

  return fetch(`${scannerAPIURL}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    cache: "no-store",
  });
}

export async function fetchHealth(): Promise<Response> {
  const scannerAPIURL = await getScannerAPIURL();

  return fetch(`${scannerAPIURL}/healthz`, {
    cache: "no-store",
  });
}
