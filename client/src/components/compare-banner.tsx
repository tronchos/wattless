import {
  formatSignedDelta,
} from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface CompareBannerProps {
  current: ScanReport;
  previous: ScanReport;
}

export function CompareBanner({ current, previous }: CompareBannerProps) {
  const bytesDelta =
    current.total_bytes_transferred - previous.total_bytes_transferred;
  const co2Delta = current.co2_grams_per_visit - previous.co2_grams_per_visit;
  const hasComparableLCP =
    current.performance.render_metrics_complete &&
    previous.performance.render_metrics_complete;
  const lcpDelta = hasComparableLCP
    ? current.performance.lcp_ms - previous.performance.lcp_ms
    : null;

  return (
    <section className="surface-secondary rounded-[1.2rem] px-4 py-3">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <p className="section-kicker">Comparación del mismo sitio</p>

        <div className="flex flex-wrap gap-2">
          <span className="soft-chip bg-[rgba(255,255,255,0.03)]">
            Bytes {formatSignedDelta(bytesDelta)}
          </span>
          <span className="soft-chip bg-[rgba(255,255,255,0.03)]">
            CO2 {co2Delta > 0 ? "+" : ""}
            {co2Delta.toFixed(3)} g
          </span>
          <span className="soft-chip bg-[rgba(255,255,255,0.03)]">
            LCP {lcpDelta === null ? "n/d" : formatSignedDelta(lcpDelta, " ms")}
          </span>
        </div>
      </div>
    </section>
  );
}
