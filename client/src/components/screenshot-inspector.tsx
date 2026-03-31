import { useEffect, useMemo, useRef } from "react";

import {
  buildScreenshotTileURL,
  formatResourceLabel,
} from "@/lib/api";
import type { BoundingBox, ScreenshotPayload, VampireElement } from "@/lib/types";

interface ScreenshotInspectorProps {
  jobId: string;
  screenshot: ScreenshotPayload;
  elements: VampireElement[];
  selectedElement: VampireElement | null;
  selectionSignal: number;
  onSelect: (id: string) => void;
}

export function ScreenshotInspector({
  jobId,
  screenshot,
  elements,
  selectedElement,
  selectionSignal,
  onSelect,
}: ScreenshotInspectorProps) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const tileSources = useMemo(
    () =>
      screenshot.tiles.map((tile, tileIndex) => ({
        id: tile.id,
        y: tile.y,
        height: tile.height,
        src: buildScreenshotTileURL(jobId, tileIndex),
      })),
    [jobId, screenshot.tiles],
  );
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
  }, [selectedElement, screenshot, selectionSignal]);

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
            {tileSources.map((tile) => (
              <img
                key={tile.id}
                alt={`Document tile ${tile.id}`}
                className="absolute left-0 w-full object-cover"
                src={tile.src}
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
        {selectedElement ? (
          <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-label text-primary">
            Activo: {selectedElement.asset_insight.title || formatResourceLabel(selectedElement.type)}
          </span>
        ) : null}
      </div>
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

interface TileSource {
  id: string;
  y: number;
  height: number;
  src: string;
}
