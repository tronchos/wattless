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
  const mappedElements = elements.filter((element) => element.bounding_box);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-end">
        <h2 className="text-2xl font-bold font-headline text-on-surface">Visual Inspector</h2>
        <span className="text-xs text-on-surface-variant font-label uppercase">
          VIEWPORT {screenshot.width}x{screenshot.height}
        </span>
      </div>

      <div className="relative bg-surface-container-highest rounded-xl overflow-hidden aspect-[4/3] group cursor-zoom-in">
        <img
          alt="Site Preview"
          className="w-full h-full object-contain bg-surface-container-lowest grayscale brightness-50 contrast-125 group-hover:grayscale-0 group-hover:brightness-100 transition-all duration-700"
          src={src}
        />

        {/* Hover overlay hint */}
        <div className="absolute inset-0 bg-primary/5 border border-primary/10 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />

        {mappedElements.map((element) => {
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
              className={`absolute rounded-lg transition-all ${
                isActive
                  ? "bg-primary/20 shadow-[inset_0_0_0_2px_rgba(155,214,126,0.8)] z-10 scale-[1.02]"
                  : "bg-surface-variant/40 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.2)] hover:bg-surface-variant/60"
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

      <div className="flex flex-wrap items-center gap-2">
        <span className="bg-surface-container text-on-surface-variant px-3 py-1 rounded-full text-xs font-label">
          {mappedElements.length} anclajes visuales
        </span>
        <span className="bg-surface-container text-on-surface-variant px-3 py-1 rounded-full text-xs font-label">
          {elements.length} recursos priorizados
        </span>
      </div>

      {selectedElement ? (
        <div className="bg-surface-container-low mt-5 grid gap-4 rounded-xl p-6 lg:grid-cols-[minmax(0,1fr)_auto] border border-outline-variant/10">
          <div>
            <div className="text-primary text-xs uppercase tracking-widest font-label font-bold">
              Recurso seleccionado
            </div>
            <div className="mt-2 text-xl font-headline font-bold text-on-surface">
              {formatResourceLabel(selectedElement.type)}
            </div>
            <div className="mt-2 break-all text-sm leading-6 text-on-surface-variant">
              {selectedElement.url}
            </div>
            <div className="mt-4 flex flex-wrap gap-2 text-xs font-bold font-label">
              <span className="bg-surface-container-highest text-on-surface px-2 py-1 rounded">
                {formatParty(selectedElement.party)}
              </span>
              <span className="bg-surface-container-highest text-on-surface px-2 py-1 rounded">
                {formatRequestStatus(
                  selectedElement.status_code,
                  selectedElement.failed
                )}
              </span>
              <span className="bg-surface-container-highest text-on-surface px-2 py-1 rounded">
                ({formatPercentage(selectedElement.transfer_share)})
              </span>
            </div>
          </div>

          <div className="grid gap-4 text-left lg:text-right content-start">
            <div>
              <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Transferido</div>
              <div className="mt-1 text-xl font-headline font-bold text-on-surface">
                {formatBytes(selectedElement.bytes)}
              </div>
            </div>
            <div>
              <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Ahorro estimado</div>
              <div className="mt-1 text-xl font-headline font-bold text-primary">
                {formatBytes(selectedElement.estimated_savings_bytes)}
              </div>
            </div>
          </div>

          <div className="lg:col-span-2 mt-2 pt-4 border-t border-outline-variant/10">
            <p className="text-sm leading-relaxed text-on-surface-variant">
              {selectedElement.recommendation}
            </p>
            {!selectedElement.bounding_box ? (
              <p className="mt-3 text-sm leading-6 text-warning">
                Este recurso no tiene un ancla visual directa en la captura.
              </p>
            ) : null}
            {selectedElement.failed && selectedElement.failure_reason ? (
              <p className="mt-3 text-sm leading-6 text-error">
                Motivo del fallo: {selectedElement.failure_reason}
              </p>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
