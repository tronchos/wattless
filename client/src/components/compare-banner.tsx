import {
  formatBytes,
  formatGrams,
  formatMilliseconds,
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
  const lcpDelta = current.performance.lcp_ms - previous.performance.lcp_ms;

  return (
    <section className="panel rounded-[2rem] p-6">
      <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            Comparativa
          </p>
          <h3 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
            Qué cambió frente al escaneo anterior
          </h3>
        </div>
        <p className="max-w-xl text-sm leading-6 text-[var(--muted)]">
          Ideal para enseñar antes/después durante la demo o justificar una
          mejora rápida en un README o PR.
        </p>
      </div>

      <div className="mt-6 grid gap-4 md:grid-cols-3">
        <article className="rounded-[1.5rem] border border-[var(--line)] bg-[var(--panel-muted)] p-4">
          <p className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Transferencia
          </p>
          <p className="mt-2 text-2xl text-white">{formatBytes(current.total_bytes_transferred)}</p>
          <p className={`mt-2 text-sm ${bytesDelta <= 0 ? "text-[var(--accent)]" : "text-[var(--warning)]"}`}>
            {formatSignedDelta(bytesDelta)} B frente al anterior
          </p>
        </article>

        <article className="rounded-[1.5rem] border border-[var(--line)] bg-[var(--panel-muted)] p-4">
          <p className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            CO2 por visita
          </p>
          <p className="mt-2 text-2xl text-white">{formatGrams(current.co2_grams_per_visit)}</p>
          <p className={`mt-2 text-sm ${co2Delta <= 0 ? "text-[var(--accent)]" : "text-[var(--warning)]"}`}>
            {co2Delta > 0 ? "+" : ""}
            {co2Delta.toFixed(3)} g frente al anterior
          </p>
        </article>

        <article className="rounded-[1.5rem] border border-[var(--line)] bg-[var(--panel-muted)] p-4">
          <p className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            LCP
          </p>
          <p className="mt-2 text-2xl text-white">{formatMilliseconds(current.performance.lcp_ms)}</p>
          <p className={`mt-2 text-sm ${lcpDelta <= 0 ? "text-[var(--accent)]" : "text-[var(--warning)]"}`}>
            {formatSignedDelta(lcpDelta, " ms")} frente al anterior
          </p>
        </article>
      </div>
    </section>
  );
}
