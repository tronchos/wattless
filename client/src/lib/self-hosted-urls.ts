type RewriteContext = {
  requestURL: string;
  requestHost: string | null;
  forwardedHost: string | null;
  forwardedProto: string | null;
  forwardedPort: string | null;
  appBaseURL?: string;
  internalBaseURL?: string;
};

export function rewriteSelfHostedURL(
  context: RewriteContext,
  rawURL: string,
): string {
  if (!context.internalBaseURL) {
    return rawURL;
  }

  let targetURL: URL;
  let internalURL: URL;
  try {
    targetURL = new URL(rawURL);
    internalURL = new URL(context.internalBaseURL);
  } catch {
    return rawURL;
  }

  const publicOrigins = collectPublicAppOrigins(context);
  const targetOrigin = normalizeOrigin(targetURL.toString());
  if (!targetOrigin || !publicOrigins.has(targetOrigin)) {
    return rawURL;
  }

  internalURL.pathname = targetURL.pathname;
  internalURL.search = targetURL.search;
  internalURL.hash = targetURL.hash;

  return internalURL.toString();
}

export function rewriteResponseURLs(
  payload: { [key: string]: unknown },
  internalURL: string,
  publicURL: string,
) {
  const vampireElements = payload.vampire_elements;
  if (!Array.isArray(vampireElements)) {
    return;
  }

  for (const element of vampireElements) {
    if (
      element &&
      typeof element === "object" &&
      "url" in element &&
      typeof element.url === "string"
    ) {
      element.url = rewriteURLOrigin(element.url, internalURL, publicURL);
    }
  }
}

function collectPublicAppOrigins(context: RewriteContext): Set<string> {
  const origins = new Set<string>();

  addOrigin(origins, context.requestURL);
  addOrigin(origins, context.appBaseURL);

  const requestProtocol = protocolFromURL(context.requestURL);
  const forwardedProto = normalizeProtocol(context.forwardedProto);
  const effectiveProtocol = forwardedProto ?? requestProtocol;

  addOriginFromHeaders(
    origins,
    effectiveProtocol,
    context.forwardedHost,
    context.forwardedPort,
  );
  addOriginFromHeaders(
    origins,
    effectiveProtocol,
    context.requestHost,
    context.forwardedPort,
  );

  return origins;
}

function addOrigin(origins: Set<string>, value?: string | null) {
  if (!value) {
    return;
  }

  const normalized = normalizeOrigin(value);
  if (normalized) {
    origins.add(normalized);
  }
}

function addOriginFromHeaders(
  origins: Set<string>,
  protocol: string | null,
  host: string | null,
  forwardedPort: string | null,
) {
  if (!protocol || !host) {
    return;
  }

  const normalizedHost = firstForwardedValue(host);
  if (!normalizedHost) {
    return;
  }

  const hostWithPort = withForwardedPort(
    normalizedHost,
    protocol,
    firstForwardedValue(forwardedPort),
  );
  addOrigin(origins, `${protocol}//${hostWithPort}`);
}

function firstForwardedValue(value: string | null): string | null {
  if (!value) {
    return null;
  }

  return value
    .split(",")
    .map((part) => part.trim())
    .find(Boolean) ?? null;
}

function withForwardedPort(
  host: string,
  protocol: string,
  forwardedPort: string | null,
): string {
  if (!forwardedPort || hasExplicitPort(host, protocol)) {
    return host;
  }

  const isDefaultPort =
    (protocol === "http:" && forwardedPort === "80") ||
    (protocol === "https:" && forwardedPort === "443");

  return isDefaultPort ? host : `${host}:${forwardedPort}`;
}

function hasExplicitPort(host: string, protocol: string): boolean {
  try {
    return new URL(`${protocol}//${host}`).port !== "";
  } catch {
    return host.includes(":");
  }
}

function protocolFromURL(value: string): string | null {
  try {
    return normalizeProtocol(new URL(value).protocol);
  } catch {
    return null;
  }
}

function normalizeOrigin(value: string): string | null {
  try {
    const url = new URL(value);
    const protocol = normalizeProtocol(url.protocol);
    if (!protocol) {
      return null;
    }

    const hostname = url.hostname.toLowerCase();
    const port = normalizePort(protocol, url.port);

    return `${protocol}//${hostname}${port ? `:${port}` : ""}`;
  } catch {
    return null;
  }
}

function normalizeProtocol(value: string | null): string | null {
  if (!value) {
    return null;
  }

  const protocol = value.trim().replace(/:$/, "").toLowerCase();
  if (protocol === "http" || protocol === "https") {
    return `${protocol}:`;
  }

  return null;
}

function normalizePort(protocol: string, port: string): string {
  if (
    (protocol === "http:" && port === "80") ||
    (protocol === "https:" && port === "443")
  ) {
    return "";
  }

  return port;
}

function rewriteURLOrigin(
  rawURL: string,
  internalURL: string,
  publicURL: string,
): string {
  const internalOrigin = normalizeOrigin(internalURL);
  const publicTarget = parseURL(publicURL);
  const resourceURL = parseURL(rawURL);

  if (!internalOrigin || !publicTarget || !resourceURL) {
    return rawURL;
  }

  if (normalizeOrigin(resourceURL.toString()) !== internalOrigin) {
    return rawURL;
  }

  resourceURL.protocol = publicTarget.protocol;
  resourceURL.host = publicTarget.host;

  return resourceURL.toString();
}

function parseURL(value: string): URL | null {
  try {
    return new URL(value);
  } catch {
    return null;
  }
}
