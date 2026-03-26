/* eslint-disable @next/next/no-img-element */

import { useEffect, useMemo, useRef } from "react";

import {
  formatBytes,
  formatParty,
  formatPercentage,
  formatPositionBand,
  formatRequestStatus,
  formatResourceLabel,
  formatThirdPartyKind,
  formatVisualRole,
} from "@/lib/api";
import type { BoundingBox, ScreenshotPayload, VampireElement } from "@/lib/types";

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
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const elementsWithAnchors = useMemo(
    () => elements.filter((element) => element.bounding_box),
    [elements],
  );
  const visibleElements = useMemo(
    () => elementsWithAnchors.filter((element) =>
      isWithinCapturedRange(element.bounding_box, screenshot)
    ),
    [elementsWithAnchors, screenshot],
  );
  const isTruncated = screenshot.captured_height < screenshot.document_height;

  useEffect(() => {
    if (!selectedElement?.bounding_box || !scrollRef.current) {
      return;
    }

    if (!isWithinCapturedRange(selectedElement.bounding_box, screenshot)) {
      return;
    }

    const scroller = scrollRef.current;
    if (scroller.scrollHeight <= scroller.clientHeight) {
      return;
    }

    const targetTop =
      (selectedElement.bounding_box.y / screenshot.captured_height) *
        scroller.scrollHeight -
      scroller.clientHeight * 0.35;

    scroller.scrollTo({
      top: Math.max(0, targetTop),
      behavior: "smooth",
    });
  }, [selectedElement, screenshot]);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <h2 className="text-2xl font-bold font-headline text-on-surface">
            Visual Inspector
          </h2>
          <p className="mt-2 text-sm leading-6 text-on-surface-variant">
            Documento completo capturado en el flujo de auditoría, con anclajes visuales
            sincronizados a la lista de activos dominantes.
          </p>
        </div>

        <div className="flex flex-wrap gap-2 text-[10px] uppercase tracking-widest font-label">
          <span className="rounded-full bg-surface-container px-3 py-1.5 text-on-surface-variant">
            Document {screenshot.document_width} x {screenshot.document_height}
          </span>
          <span className="rounded-full bg-surface-container px-3 py-1.5 text-on-surface-variant">
            {isTruncated
              ? `Captured ${screenshot.captured_height}px`
              : "Full Height"}
          </span>
          {screenshot.strategy === "tiled" ? (
            <span className="rounded-full bg-primary/10 px-3 py-1.5 text-primary">
              Tiled Capture
            </span>
          ) : null}
          {isTruncated ? (
            <span className="rounded-full bg-secondary-container px-3 py-1.5 text-on-surface">
              Captura parcial por eficiencia
            </span>
          ) : null}
        </div>
      </div>

      <div className="rounded-[1.75rem] border border-outline-variant/10 bg-surface-container-low p-3 md:p-4">
        <div
          ref={scrollRef}
          className="overflow-visible rounded-[1.25rem] bg-surface-container-highest/60 md:max-h-[72vh] md:overflow-y-auto"
        >
          <div
            className="relative w-full overflow-hidden rounded-[1.25rem] bg-surface-container-lowest"
            style={{
              aspectRatio: `${screenshot.document_width} / ${screenshot.captured_height}`,
            }}
          >
            {screenshot.tiles.map((tile) => (
              <img
                key={tile.id}
                alt={`Document tile ${tile.id}`}
                className="absolute left-0 w-full object-cover"
                src={`data:${screenshot.mime_type};base64,${tile.data_base64}`}
                style={{
                  top: `${(tile.y / screenshot.captured_height) * 100}%`,
                  height: `${(tile.height / screenshot.captured_height) * 100}%`,
                }}
              />
            ))}

            <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-surface/5 via-transparent to-surface/10" />

            {visibleElements.map((element) => {
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
                  className={`absolute rounded-xl transition-all ${
                    isActive
                      ? "bg-primary/18 shadow-[inset_0_0_0_2px_rgba(155,214,126,0.9)] z-10"
                      : "bg-surface-variant/35 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.18)] hover:bg-surface-variant/55"
                  }`}
                  style={boxToStyle(box, screenshot)}
                  onClick={() => onSelect(element.id)}
                />
              );
            })}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded-full bg-surface-container px-3 py-1 text-xs font-label text-on-surface-variant">
          {visibleElements.length} anclajes visibles
        </span>
        <span className="rounded-full bg-surface-container px-3 py-1 text-xs font-label text-on-surface-variant">
          {elementsWithAnchors.length} anclajes detectados
        </span>
        {elementsWithAnchors.length > visibleElements.length ? (
          <span className="rounded-full bg-secondary-container px-3 py-1 text-xs font-label text-on-surface">
            {elementsWithAnchors.length - visibleElements.length} fuera del rango capturado
          </span>
        ) : null}
      </div>

      {selectedElement ? (
        <div className="grid gap-4 rounded-[1.5rem] border border-outline-variant/10 bg-surface-container-low p-6 lg:grid-cols-[minmax(0,1fr)_auto]">
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
              <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                {formatParty(selectedElement.party)}
              </span>
              <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                {formatRequestStatus(
                  selectedElement.status_code,
                  selectedElement.failed
                )}
              </span>
              <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                {formatPercentage(selectedElement.transfer_share)}
              </span>
              <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                {formatPositionBand(selectedElement.position_band)}
              </span>
              {selectedElement.visual_role !== "unknown" ? (
                <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                  {formatVisualRole(selectedElement.visual_role)}
                </span>
              ) : null}
              {selectedElement.is_third_party_tool &&
              selectedElement.third_party_kind !== "unknown" ? (
                <span className="rounded bg-surface-container-highest px-2 py-1 text-on-surface">
                  {formatThirdPartyKind(selectedElement.third_party_kind)}
                </span>
              ) : null}
              <span
                className={`rounded px-2 py-1 ${
                  !selectedElement.bounding_box
                    ? "bg-surface-container-highest text-on-surface-variant"
                    : isWithinCapturedRange(selectedElement.bounding_box, screenshot)
                      ? "bg-primary/10 text-primary"
                      : "bg-secondary-container text-on-surface"
                }`}
              >
                {selectedElement.bounding_box
                  ? isWithinCapturedRange(selectedElement.bounding_box, screenshot)
                    ? "Visible in inspector"
                    : "Outside captured range"
                  : "No visual anchor"}
              </span>
            </div>
          </div>

          <div className="grid content-start gap-4 text-left lg:text-right">
            <div>
              <div className="text-[10px] uppercase tracking-widest text-on-surface-variant font-label">
                Transferido
              </div>
              <div className="mt-1 text-xl font-headline font-bold text-on-surface">
                {formatBytes(selectedElement.bytes)}
              </div>
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-widest text-on-surface-variant font-label">
                Ahorro estimado
              </div>
              <div className="mt-1 text-xl font-headline font-bold text-primary">
                {formatBytes(selectedElement.estimated_savings_bytes)}
              </div>
            </div>
          </div>

          <div className="mt-2 border-t border-outline-variant/10 pt-4 lg:col-span-2">
            <p className="text-sm leading-relaxed text-on-surface-variant">
              {selectedElement.recommendation}
            </p>
            {!selectedElement.bounding_box ? (
              <p className="mt-3 text-sm leading-6 text-warning">
                Este recurso no tiene un ancla visual directa en la captura.
              </p>
            ) : null}
            {selectedElement.bounding_box &&
            !isWithinCapturedRange(selectedElement.bounding_box, screenshot) ? (
              <p className="mt-3 text-sm leading-6 text-warning">
                Este recurso está por debajo del rango capturado del inspector.
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

function isWithinCapturedRange(
  box: BoundingBox | null,
  screenshot: ScreenshotPayload,
): box is BoundingBox {
  if (!box) {
    return false;
  }

  return box.y < screenshot.captured_height;
}

function boxToStyle(box: BoundingBox, screenshot: ScreenshotPayload) {
  return {
    left: `${(box.x / screenshot.document_width) * 100}%`,
    top: `${(box.y / screenshot.captured_height) * 100}%`,
    width: `${(box.width / screenshot.document_width) * 100}%`,
    height: `${(box.height / screenshot.captured_height) * 100}%`,
  };
}
