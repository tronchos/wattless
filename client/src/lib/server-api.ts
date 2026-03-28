const defaultDevelopmentScannerURLs = [
  "http://localhost:8080",
  "http://localhost:8081",
  "http://localhost:18080",
] as const;
export const clientIdentityCookieName = "wattless_client_id";

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

export async function forwardScannerRequest(
  request: Request,
  path: string,
  init: {
    method: "GET" | "POST";
    body?: unknown;
  }
): Promise<{
  upstream: Response;
  clientIdentity: string;
  shouldSetCookie: boolean;
}> {
  const scannerAPIURL = await getScannerAPIURL();
  const headers = new Headers();
  const { value: clientIdentity, shouldSetCookie } = getOrCreateClientIdentity(request);

  if (init.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  copyClientIPHeader(request, headers, "x-forwarded-for");
  copyClientIPHeader(request, headers, "x-real-ip");
  headers.set("x-wattless-client-id", clientIdentity);

  return {
    upstream: await fetch(`${scannerAPIURL}${path}`, {
      method: init.method,
      headers,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
      cache: "no-store",
    }),
    clientIdentity,
    shouldSetCookie,
  };
}

export async function fetchHealth(): Promise<Response> {
  const scannerAPIURL = await getScannerAPIURL();

  return fetch(`${scannerAPIURL}/healthz`, {
    cache: "no-store",
  });
}

function copyClientIPHeader(
  request: Request,
  headers: Headers,
  headerName: "x-forwarded-for" | "x-real-ip",
) {
  const value = request.headers.get(headerName);
  if (value?.trim()) {
    headers.set(headerName, value.trim());
  }
}

function getOrCreateClientIdentity(request: Request): {
  value: string;
  shouldSetCookie: boolean;
} {
  const existingCookie = readCookie(request, clientIdentityCookieName);
  if (existingCookie) {
    return {
      value: existingCookie,
      shouldSetCookie: false,
    };
  }

  return {
    value: `wlc_${crypto.randomUUID()}`,
    shouldSetCookie: true,
  };
}

function readCookie(request: Request, cookieName: string): string | null {
  const cookieHeader = request.headers.get("cookie");
  if (!cookieHeader) {
    return null;
  }

  const cookies = cookieHeader.split(";");
  for (const entry of cookies) {
    const [rawName, ...rawValueParts] = entry.split("=");
    if (!rawName || rawValueParts.length === 0) {
      continue;
    }

    if (rawName.trim() !== cookieName) {
      continue;
    }

    const value = rawValueParts.join("=").trim();
    if (!value) {
      return null;
    }

    try {
      return decodeURIComponent(value);
    } catch {
      return value;
    }
  }

  return null;
}
