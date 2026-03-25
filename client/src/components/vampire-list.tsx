import type { VampireElement } from "@/lib/types";
import {
  formatBytes,
  formatParty,
  formatPercentage,
  formatRequestStatus,
  formatResourceLabel,
} from "@/lib/api";

interface VampireListProps {
  elements: VampireElement[];
  selectedElementID: string | null;
  onSelect: (id: string) => void;
}

export function VampireList({
  elements,
  selectedElementID,
  onSelect,
}: VampireListProps) {
  return (
    <section className="panel rounded-[2rem] p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
                  <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            Elementos vampiro
          </p>
          <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
            Recursos que más drenan la página
          </h2>
        </div>
        <span className="mono rounded-full border border-[var(--line-strong)] px-3 py-1 text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
          {elements.length} activos
        </span>
      </div>

      <div className="mt-6 space-y-3">
        {elements.map((element, index) => {
          const isActive = selectedElementID === element.id;
          return (
            <button
              key={element.id}
              type="button"
              onClick={() => onSelect(element.id)}
              className={`w-full rounded-[1.5rem] border p-4 text-left transition ${
                isActive
                  ? "border-[var(--accent)] bg-[rgba(155,214,126,0.1)]"
                  : "border-[var(--line)] bg-[rgba(255,255,255,0.02)] hover:border-[var(--line-strong)]"
              }`}
            >
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
                    #{index + 1} {formatResourceLabel(element.type)}
                  </div>
                  <p className="mt-2 break-all text-sm leading-6 text-white">
                    {element.url}
                  </p>
                </div>
                <div className="mono whitespace-nowrap text-sm text-[var(--accent-strong)]">
                  {formatBytes(element.bytes)}
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  {formatParty(element.party)}
                </span>
                <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  {formatPercentage(element.transfer_share)}
                </span>
                <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  {formatRequestStatus(element.status_code, element.failed)}
                </span>
                <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  Ahorra {formatBytes(element.estimated_savings_bytes)}
                </span>
              </div>
              <p className="mt-3 text-sm leading-6 text-[var(--muted)]">
                {element.recommendation}
              </p>
              {element.failed && element.failure_reason ? (
                <p className="mt-2 text-sm leading-6 text-[var(--warning)]">
                  Motivo del fallo: {element.failure_reason}
                </p>
              ) : null}
            </button>
          );
        })}
      </div>
    </section>
  );
}
