/* eslint-disable @next/next/no-img-element */

import {
  formatBytes,
  formatParty,
  formatPercentage,
  formatRequestStatus,
  formatResourceLabel,
} from "@/lib/api";
import type { ScreenshotPayload, VampireElement } from "@/lib/types";

interface ScreenshotInspectorProps {
  screenshot: ScreenshotPayload;
  elements: VampireElement[];
  selectedElement: VampireElement | null;
  onSelect: (id: string) => void;
}

export function ScreenshotInspector({
  screenshot,
  elements,
  selectedElement,
  onSelect,
}: ScreenshotInspectorProps) {
  const src = `data:${screenshot.mime_type};base64,${screenshot.data_base64}`;

  return (
    <section className="panel rounded-[2rem] p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            Inspector visual
          </p>
          <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
            Haz clic en los puntos donde más energía se pierde
          </h2>
        </div>
        <span className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
          {screenshot.width} x {screenshot.height}
        </span>
      </div>

      <div className="mt-6 overflow-hidden rounded-[1.75rem] border border-[var(--line)] bg-black">
        <div className="relative">
          <img
            alt="Website screenshot for highlighted assets"
            className="block h-auto w-full"
            src={src}
          />
          {elements
            .filter((element) => element.bounding_box)
            .map((element) => {
              const box = element.bounding_box;
              if (!box) {
                return null;
              }

              const isActive = selectedElement?.id === element.id;
              return (
                <button
                  key={element.id}
                  type="button"
                  aria-label={`Highlight ${element.url}`}
                  className={`absolute rounded-[0.9rem] border transition ${
                    isActive
                      ? "border-[var(--accent-strong)] bg-[rgba(216,255,127,0.16)]"
                      : "border-[rgba(255,255,255,0.3)] bg-[rgba(255,255,255,0.05)] hover:border-[var(--accent)]"
                  }`}
                  style={{
                    left: `${(box.x / screenshot.width) * 100}%`,
                    top: `${(box.y / screenshot.height) * 100}%`,
                    width: `${(box.width / screenshot.width) * 100}%`,
                    height: `${(box.height / screenshot.height) * 100}%`,
                  }}
                  onClick={() => onSelect(element.id)}
                />
              );
            })}
        </div>
      </div>

      {selectedElement ? (
        <div className="mt-5 grid gap-4 rounded-[1.75rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4 lg:grid-cols-[minmax(0,1fr)_auto]">
          <div>
            <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
              Recurso seleccionado
            </div>
            <div className="mt-2 text-lg text-white">
              {formatResourceLabel(selectedElement.type)}
            </div>
            <div className="mt-2 break-all text-sm leading-6 text-[var(--muted)]">
              {selectedElement.url}
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                {formatParty(selectedElement.party)}
              </span>
              <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                {formatRequestStatus(selectedElement.status_code, selectedElement.failed)}
              </span>
              <span className="mono rounded-full border border-[var(--line)] px-2 py-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                {formatPercentage(selectedElement.transfer_share)}
              </span>
            </div>
          </div>
          <div className="grid gap-2 text-right">
            <div>
              <div className="mono text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                Transferido
              </div>
              <div className="text-lg text-white">
                {formatBytes(selectedElement.bytes)}
              </div>
            </div>
            <div>
              <div className="mono text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                Ahorro estimado
              </div>
              <div className="text-lg text-[var(--accent-strong)]">
                {formatBytes(selectedElement.estimated_savings_bytes)}
              </div>
            </div>
          </div>
          <div className="lg:col-span-2">
            <p className="text-sm leading-6 text-[var(--foreground)]">
              {selectedElement.recommendation}
            </p>
            {!selectedElement.bounding_box ? (
              <p className="mt-2 text-sm leading-6 text-[var(--warning)]">
                Este recurso no tiene un ancla visual directa en la captura.
                Probablemente sea un script, una fuente, una hoja de estilos o
                una transferencia no visual.
              </p>
            ) : null}
            {selectedElement.failed && selectedElement.failure_reason ? (
              <p className="mt-2 text-sm leading-6 text-[var(--warning)]">
                Motivo del fallo: {selectedElement.failure_reason}
              </p>
            ) : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
