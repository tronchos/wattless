const defaultPublicAppURL = "http://localhost:5173";

export function resolvePublicAppURL(value?: string): string {
  const trimmed = value?.trim();
  if (!trimmed) {
    return defaultPublicAppURL;
  }

  try {
    return new URL(trimmed).toString().replace(/\/$/, "");
  } catch {
    return defaultPublicAppURL;
  }
}

export function resolveAbsolutePublicAssetURL(
  publicAppURL: string | undefined,
  assetPath: string,
): string {
  const normalizedBaseURL = `${resolvePublicAppURL(publicAppURL)}/`;
  const normalizedAssetPath = assetPath.replace(/^\//, "");

  return new URL(normalizedAssetPath, normalizedBaseURL).toString();
}
