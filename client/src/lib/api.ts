import type {
  APIErrorPayload,
  Analysis,
  AnalysisFinding,
  AssetInsight,
  FixSuggestion,
  ResourceBreakdown,
  ResourceGroup,
  ScanJobResponse,
  ScanInsightsResponse,
  ScanReport,
  ScanInsights,
  ScanMethodology,
  ScreenshotPayload,
  SiteProfile,
  TopAction,
  VampireElement,
} from "@/lib/types";

export class APIError extends Error {
  status: number;
  retryAfterSeconds: number | null;
  code: string | null;
  job?: ScanJobResponse;

  constructor(
    message: string,
    options: {
      status: number;
      retryAfterSeconds?: number | null;
      code?: string | null;
      job?: ScanJobResponse;
    },
  ) {
    super(message);
    this.name = "APIError";
    this.status = options.status;
    this.retryAfterSeconds = options.retryAfterSeconds ?? null;
    this.code = options.code ?? null;
    this.job = options.job;
  }
}

const clientIdentityStorageKey = "wattless.client_id";

export async function submitScan(url: string): Promise<ScanJobResponse> {
  const response = await fetch(buildAPIURL("/api/v1/scans"), {
    method: "POST",
    headers: withClientIdentity({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ url }),
    cache: "no-store",
  });

  return parseJobResponse(response, "El escaneo no pudo encolarse");
}

export async function pollScanJob(jobId: string): Promise<ScanJobResponse> {
  const response = await fetch(buildAPIURL(`/api/v1/scans/${encodeURIComponent(jobId)}`), {
    headers: withClientIdentity(),
    cache: "no-store",
  });

  return parseJobResponse(response, "No se pudo consultar el estado del turno");
}

export async function fetchInsights(jobId: string): Promise<ScanInsightsResponse | null> {
  const response = await fetch(
    buildAPIURL(`/api/v1/scans/${encodeURIComponent(jobId)}/insights`),
    {
      headers: withClientIdentity(),
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    const payload = (await response.json().catch(() => null)) as unknown;
    const errorPayload = asAPIErrorPayload(payload);

    if (errorPayload?.code === "insights_unavailable") {
      return null;
    }

    throw new APIError(errorPayload?.error ?? "No se pudo consultar el estado de insights", {
      status: response.status,
      code: errorPayload?.code ?? null,
    });
  }

  return parseInsightsResponse(response, "No se pudo consultar el estado de insights");
}

export function buildScreenshotTileURL(jobId: string, tileIndex: number): string {
  const params = new URLSearchParams({
    tile: String(tileIndex),
  });

  return buildAPIURL(
    `/api/v1/scans/${encodeURIComponent(jobId)}/screenshot?${params.toString()}`,
  );
}

export function buildAPIURL(path: string): string {
  const apiBaseURL = resolveAPIBaseURL();
  if (!apiBaseURL) {
    return path;
  }

  const normalizedPath = path.replace(/^\//, "");
  return new URL(normalizedPath, `${apiBaseURL}/`).toString();
}

function resolveAPIBaseURL(): string | null {
  const raw = import.meta.env.VITE_API_BASE_URL?.trim();
  if (!raw) {
    return null;
  }

  try {
    const url = new URL(raw);
    url.pathname = url.pathname.replace(/\/+$/, "");
    if (url.pathname === "/api" || url.pathname === "/api/v1") {
      url.pathname = "";
    }

    return url.toString().replace(/\/$/, "");
  } catch {
    return null;
  }
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

export function isScanInsightsResponse(value: unknown): value is ScanInsightsResponse {
  if (!isRecord(value)) {
    return false;
  }

  if (typeof value.job_id !== "string" || typeof value.status !== "string") {
    return false;
  }

  if (
    value.status !== "processing" &&
    value.status !== "ready" &&
    value.status !== "failed"
  ) {
    return false;
  }

  if (value.insights !== undefined && !isRecord(value.insights)) {
    return false;
  }

  if (
    value.vampire_elements !== undefined &&
    !Array.isArray(value.vampire_elements)
  ) {
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
      code: errorPayload?.code ?? null,
      job: conflictJob,
    });
  }

  if (!isScanJobResponse(payload)) {
    throw new Error("La respuesta del servidor no tiene el formato esperado.");
  }

  return normalizeScanJobResponse(payload);
}

async function parseInsightsResponse(
  response: Response,
  fallbackMessage: string,
): Promise<ScanInsightsResponse> {
  const retryAfterSeconds = parseRetryAfter(response.headers.get("Retry-After"));
  const payload = (await response.json().catch(() => null)) as unknown;

  if (!response.ok) {
    const errorPayload = asAPIErrorPayload(payload);
    throw new APIError(errorPayload?.error ?? fallbackMessage, {
      status: response.status,
      retryAfterSeconds,
      code: errorPayload?.code ?? null,
    });
  }

  if (!isScanInsightsResponse(payload)) {
    throw new Error("La respuesta del servidor no tiene el formato esperado.");
  }

  return normalizeScanInsightsResponse(payload);
}

function normalizeScanJobResponse(job: ScanJobResponse): ScanJobResponse {
  return {
    ...job,
    report: job.report ? normalizeScanReport(job.report) : undefined,
  };
}

function normalizeScanInsightsResponse(
  response: ScanInsightsResponse,
): ScanInsightsResponse {
  return {
    ...response,
    insights: response.insights
      ? normalizeScanInsights(response.insights)
      : undefined,
    vampire_elements: response.vampire_elements
      ? normalizeVampireElements(response.vampire_elements)
      : undefined,
  };
}

function normalizeScanReport(report: ScanReport): ScanReport {
  const rawReport = asRecord(report) ?? {};

  return {
    ...report,
    site_profile: normalizeSiteProfile(rawReport.site_profile),
    breakdown_by_type: normalizeResourceBreakdowns(rawReport.breakdown_by_type),
    breakdown_by_party: normalizeResourceBreakdowns(rawReport.breakdown_by_party),
    insights: normalizeScanInsights(rawReport.insights),
    vampire_elements: normalizeVampireElements(rawReport.vampire_elements),
    analysis: normalizeAnalysis(rawReport.analysis),
    screenshot: normalizeScreenshot(rawReport.screenshot),
    methodology: normalizeMethodology(rawReport.methodology),
    warnings: normalizeStringArray(rawReport.warnings),
  };
}

function normalizeSiteProfile(value: unknown): SiteProfile {
  const profile = asRecord(value) ?? {};
  const frameworkHint = profile.framework_hint;
  const normalizedHint =
    frameworkHint === "astro" ||
    frameworkHint === "nextjs" ||
    frameworkHint === "generic" ||
    frameworkHint === "unknown"
      ? frameworkHint
      : "generic";

  return {
    framework_hint: normalizedHint,
    evidence: normalizeStringArray(profile.evidence),
  };
}

function normalizeResourceBreakdowns(value: unknown): ResourceBreakdown[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .filter(isRecord)
    .map((item) => ({
      label: coerceString(item.label),
      bytes: coerceNumber(item.bytes),
      requests: coerceNumber(item.requests),
      percentage: coerceNumber(item.percentage),
    }));
}

function normalizeScanInsights(value: unknown): ScanInsights {
  const insights = asRecord(value) ?? {};

  return {
    provider: coerceString(insights.provider),
    executive_summary: coerceString(insights.executive_summary),
    pitch_line: coerceString(insights.pitch_line),
    top_actions: normalizeTopActions(insights.top_actions),
  };
}

function normalizeTopActions(value: unknown): TopAction[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord).map((action) => ({
    id: coerceString(action.id),
    related_finding_id: coerceString(action.related_finding_id),
    title: coerceString(action.title),
    reason: coerceString(action.reason),
    confidence: normalizeConfidence(action.confidence),
    evidence: normalizeStringArray(action.evidence),
    estimated_savings_bytes: coerceNumber(action.estimated_savings_bytes),
    likely_lcp_impact: normalizeImpact(action.likely_lcp_impact),
    related_resource_ids: normalizeStringArray(action.related_resource_ids),
    visible_related_resource_ids: normalizeStringArray(
      action.visible_related_resource_ids,
    ),
    recommended_fix: normalizeFixSuggestion(action.recommended_fix),
  }));
}

function normalizeVampireElements(value: unknown): VampireElement[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord).map((element) => {
    const assetInsight = asRecord(element.asset_insight) ?? {};
    const boundingBox = asRecord(element.bounding_box);

    return {
      id: coerceString(element.id),
      url: coerceString(element.url),
      type: coerceString(element.type),
      mime_type: coerceString(element.mime_type),
      hostname: coerceString(element.hostname),
      party: normalizeParty(element.party),
      status_code: coerceNumber(element.status_code),
      bytes: coerceNumber(element.bytes),
      failed: coerceBoolean(element.failed),
      failure_reason: coerceString(element.failure_reason),
      transfer_share: coerceNumber(element.transfer_share),
      estimated_savings_bytes: coerceNumber(element.estimated_savings_bytes),
      position_band: normalizePositionBand(element.position_band),
      visual_role: normalizeVisualRole(element.visual_role),
      dom_tag: coerceString(element.dom_tag),
      loading_attr: coerceString(element.loading_attr),
      fetch_priority: coerceString(element.fetch_priority),
      responsive_image: coerceBoolean(element.responsive_image),
      natural_width:
        typeof element.natural_width === "number" ? element.natural_width : undefined,
      natural_height:
        typeof element.natural_height === "number"
          ? element.natural_height
          : undefined,
      visible_ratio:
        typeof element.visible_ratio === "number" ? element.visible_ratio : undefined,
      is_third_party_tool: coerceBoolean(element.is_third_party_tool),
      third_party_kind: normalizeThirdPartyKind(element.third_party_kind),
      asset_insight: normalizeAssetInsight(assetInsight),
      bounding_box: boundingBox
        ? {
            x: coerceNumber(boundingBox.x),
            y: coerceNumber(boundingBox.y),
            width: coerceNumber(boundingBox.width),
            height: coerceNumber(boundingBox.height),
          }
        : null,
    };
  });
}

function normalizeAssetInsight(value: Record<string, unknown>): AssetInsight {
  return {
    source: normalizeInsightSource(value.source),
    scope: normalizeInsightScope(value.scope),
    title: coerceString(value.title),
    short_problem: coerceString(value.short_problem),
    why_it_matters: coerceString(value.why_it_matters),
    recommended_action: coerceString(value.recommended_action),
    confidence: normalizeConfidence(value.confidence),
    likely_lcp_impact: normalizeImpact(value.likely_lcp_impact),
    related_finding_id:
      typeof value.related_finding_id === "string"
        ? value.related_finding_id
        : undefined,
    related_action_id:
      typeof value.related_action_id === "string"
        ? value.related_action_id
        : undefined,
    evidence: normalizeStringArray(value.evidence),
    recommended_fix: normalizeFixSuggestion(value.recommended_fix),
  };
}

function normalizeAnalysis(value: unknown): Analysis {
  const analysis = asRecord(value) ?? {};
  const summary = asRecord(analysis.summary) ?? {};

  return {
    summary: {
      above_fold_visual_bytes: coerceNumber(summary.above_fold_visual_bytes),
      below_fold_bytes: coerceNumber(summary.below_fold_bytes),
      lcp_resource_id:
        typeof summary.lcp_resource_id === "string"
          ? summary.lcp_resource_id
          : undefined,
      lcp_resource_url:
        typeof summary.lcp_resource_url === "string"
          ? summary.lcp_resource_url
          : undefined,
      lcp_resource_bytes:
        typeof summary.lcp_resource_bytes === "number"
          ? summary.lcp_resource_bytes
          : undefined,
      analytics_bytes: coerceNumber(summary.analytics_bytes),
      analytics_requests: coerceNumber(summary.analytics_requests),
      font_bytes: coerceNumber(summary.font_bytes),
      font_requests: coerceNumber(summary.font_requests),
      repeated_gallery_bytes: coerceNumber(summary.repeated_gallery_bytes),
      repeated_gallery_count: coerceNumber(summary.repeated_gallery_count),
      render_critical_bytes: coerceNumber(summary.render_critical_bytes),
    },
    findings: normalizeAnalysisFindings(analysis.findings),
    resource_groups: normalizeResourceGroups(analysis.resource_groups),
  };
}

function normalizeAnalysisFindings(value: unknown): AnalysisFinding[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord).map((finding) => ({
    id: coerceString(finding.id),
    category: normalizeFindingCategory(finding.category),
    severity: normalizeSeverity(finding.severity),
    confidence: normalizeConfidence(finding.confidence),
    title: coerceString(finding.title),
    summary: coerceString(finding.summary),
    evidence: normalizeStringArray(finding.evidence),
    estimated_savings_bytes: coerceNumber(finding.estimated_savings_bytes),
    related_resource_ids: normalizeStringArray(finding.related_resource_ids),
  }));
}

function normalizeResourceGroups(value: unknown): ResourceGroup[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord).map((group) => ({
    id: coerceString(group.id),
    kind: normalizeGroupKind(group.kind),
    label: coerceString(group.label),
    total_bytes: coerceNumber(group.total_bytes),
    resource_count: coerceNumber(group.resource_count),
    position_band: normalizePositionBand(group.position_band),
    related_resource_ids: normalizeStringArray(group.related_resource_ids),
  }));
}

function normalizeScreenshot(value: unknown): ScreenshotPayload {
  const screenshot = asRecord(value) ?? {};

  return {
    mime_type: coerceString(screenshot.mime_type),
    strategy: screenshot.strategy === "tiled" ? "tiled" : "single",
    viewport_width: coerceNumber(screenshot.viewport_width),
    viewport_height: coerceNumber(screenshot.viewport_height),
    document_width: coerceNumber(screenshot.document_width),
    document_height: coerceNumber(screenshot.document_height),
    captured_height: coerceNumber(screenshot.captured_height),
    tiles: normalizeScreenshotTiles(screenshot.tiles),
  };
}

function normalizeScreenshotTiles(value: unknown): ScreenshotPayload["tiles"] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isRecord).map((tile) => ({
    id: coerceString(tile.id),
    y: coerceNumber(tile.y),
    width: coerceNumber(tile.width),
    height: coerceNumber(tile.height),
    data_base64: coerceString(tile.data_base64),
  }));
}

function normalizeMethodology(value: unknown): ScanMethodology {
  const methodology = asRecord(value) ?? {};

  return {
    model: coerceString(methodology.model),
    formula: coerceString(methodology.formula),
    source: coerceString(methodology.source),
    assumptions: normalizeStringArray(methodology.assumptions),
  };
}

function normalizeFixSuggestion(value: unknown): FixSuggestion | undefined {
  const fix = asRecord(value);
  if (!fix) {
    return undefined;
  }

  return {
    summary: coerceString(fix.summary),
    optimized_code: coerceString(fix.optimized_code),
    changes: normalizeStringArray(fix.changes),
    expected_impact: coerceString(fix.expected_impact),
  };
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
    code: typeof payload.code === "string" ? payload.code : undefined,
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

function asRecord(value: unknown): Record<string, unknown> | null {
  return isRecord(value) ? value : null;
}

function normalizeStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter((item): item is string => typeof item === "string");
}

function coerceString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function coerceNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function coerceBoolean(value: unknown): boolean {
  return typeof value === "boolean" ? value : false;
}

function normalizeConfidence(value: unknown): TopAction["confidence"] {
  return value === "high" || value === "medium" || value === "low" ? value : "low";
}

function normalizeImpact(value: unknown): TopAction["likely_lcp_impact"] {
  return value === "high" || value === "medium" || value === "low" ? value : "low";
}

function normalizeParty(value: unknown): VampireElement["party"] {
  return value === "third_party" ? "third_party" : "first_party";
}

function normalizePositionBand(value: unknown): VampireElement["position_band"] {
  return value === "above_fold" ||
    value === "near_fold" ||
    value === "below_fold" ||
    value === "unknown" ||
    value === "mixed"
    ? value
    : "unknown";
}

function normalizeVisualRole(value: unknown): VampireElement["visual_role"] {
  return value === "lcp_candidate" ||
    value === "hero_media" ||
    value === "repeated_card_media" ||
    value === "above_fold_media" ||
    value === "below_fold_media" ||
    value === "decorative" ||
    value === "unknown"
    ? value
    : "unknown";
}

function normalizeThirdPartyKind(value: unknown): VampireElement["third_party_kind"] {
  return value === "analytics" ||
    value === "ads" ||
    value === "support" ||
    value === "social" ||
    value === "video_embed" ||
    value === "payment" ||
    value === "unknown"
    ? value
    : "unknown";
}

function normalizeInsightSource(value: unknown): AssetInsight["source"] {
  return value === "gemini" || value === "rule_based" || value === "hybrid"
    ? value
    : "rule_based";
}

function normalizeInsightScope(value: unknown): AssetInsight["scope"] {
  return value === "asset" || value === "group" || value === "global"
    ? value
    : "asset";
}

function normalizeFindingCategory(value: unknown): AnalysisFinding["category"] {
  return value === "render" ||
    value === "media" ||
    value === "third_party" ||
    value === "fonts" ||
    value === "cpu"
    ? value
    : "render";
}

function normalizeSeverity(value: unknown): AnalysisFinding["severity"] {
  return value === "high" || value === "medium" || value === "low" ? value : "low";
}

function normalizeGroupKind(value: unknown): ResourceGroup["kind"] {
  return value === "repeated_gallery" ||
    value === "third_party_cluster" ||
    value === "font_cluster"
    ? value
    : "repeated_gallery";
}

function withClientIdentity(headers?: HeadersInit): Headers {
  const nextHeaders = new Headers(headers);
  nextHeaders.set("X-Wattless-Client-Id", getClientIdentity());
  return nextHeaders;
}

function getClientIdentity(): string {
  if (typeof window === "undefined") {
    return "wlc_server";
  }

  const storedIdentity = window.localStorage.getItem(clientIdentityStorageKey);
  if (storedIdentity) {
    return storedIdentity;
  }

  const generatedIdentity = `wlc_${generateUUID()}`;
  window.localStorage.setItem(clientIdentityStorageKey, generatedIdentity);
  return generatedIdentity;
}

function generateUUID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
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
  switch (value) {
    case "document":
      return "Documento";
    case "image":
      return "Imagen";
    case "script":
      return "Script";
    case "font":
      return "Fuente";
    case "stylesheet":
      return "CSS";
    case "xhr":
      return "XHR";
    case "fetch":
      return "Fetch";
    default:
      break;
  }

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
      return "Primer viewport";
    case "near_fold":
      return "Cerca del fold";
    case "below_fold":
      return "Debajo del fold";
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
      return "Publicidad";
    case "support":
      return "Soporte";
    case "social":
      return "Social";
    case "video_embed":
      return "Video embebido";
    case "payment":
      return "Pago";
    default:
      return "Tercero";
  }
}

export function formatVisualRole(value: string): string {
  switch (value) {
    case "lcp_candidate":
      return "LCP";
    case "hero_media":
      return "Hero";
    case "repeated_card_media":
      return "Media repetida";
    case "above_fold_media":
      return "Media en primer viewport";
    case "below_fold_media":
      return "Media bajo el fold";
    case "decorative":
      return "Decorativo";
    default:
      return "Desconocido";
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
