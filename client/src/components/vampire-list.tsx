import { memo, useMemo } from "react";

import {
  formatBytes,
  formatPositionBand,
  formatResourceLabel,
  formatThirdPartyKind,
  formatVisualRole,
} from "@/lib/api";
import { DominantAssetDetail } from "@/components/dominant-asset-detail";
import type { VampireElement } from "@/lib/types";

interface VampireListProps {
  elements: VampireElement[];
  selectedElementID: string | null;
  capturedHeight: number;
  onSelect: (id: string) => void;
}

export const VampireList = memo(function VampireList({
  elements,
  selectedElementID,
  capturedHeight,
  onSelect,
}: VampireListProps) {
  const selectedElement =
    elements.find((element) => element.id === selectedElementID) ?? elements[0] ?? null;

  return (
    <div className="space-y-6 flex flex-col h-full">
      <h2 className="text-2xl font-bold font-headline text-on-surface">Dominant Assets</h2>

      <div className="space-y-3">
        {elements.map((element) => (
          <VampireRow
            key={element.id}
            element={element}
            isActive={selectedElement?.id === element.id}
            capturedHeight={capturedHeight}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  );
});

interface VampireRowProps {
  element: VampireElement;
  isActive: boolean;
  capturedHeight: number;
  onSelect: (id: string) => void;
}

const VampireRow = memo(function VampireRow({
  element,
  isActive,
  capturedHeight,
  onSelect,
}: VampireRowProps) {
  const coverage = useMemo(
    () => getCoverageState(element, capturedHeight),
    [element, capturedHeight],
  );
  const badges = useMemo(() => buildBadges(element), [element]);

  let impactLevel: "HIGH" | "MED" | "LOW" = "LOW";
  if (
    element.visual_role === "lcp_candidate" ||
    element.estimated_savings_bytes > 50000
  ) {
    impactLevel = "HIGH";
  } else if (
    element.visual_role === "hero_media" ||
    element.estimated_savings_bytes > 10000
  ) {
    impactLevel = "MED";
  }

  return (
    <article
      className={`overflow-hidden rounded-[1.35rem] border transition-all duration-300 ${
        isActive
          ? "border-primary/40 bg-primary/5 ring-1 ring-primary/20 shadow-[0_0_15px_rgba(52,211,153,0.1)]"
          : "border-outline-variant/10 bg-surface-container-low hover:bg-surface-container hover:border-outline-variant/30"
      }`}
    >
      <button
        type="button"
        aria-label={`Select asset ${element.url.split("/").pop() || element.type}`}
        onClick={() => onSelect(element.id)}
        className="w-full px-4 py-3.5 text-left md:px-5"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-headline font-bold text-on-surface" title={element.url}>
              {element.url.split("/").pop() || formatResourceLabel(element.type)}
            </div>
            <div className="mt-1 truncate text-xs leading-5 text-on-surface-variant" title={element.url}>
              {element.url}
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-[10px] text-on-surface-variant">
              <span className={`rounded-full px-2 py-0.5 font-label font-bold ${coverage.className}`}>
                {coverage.label}
              </span>
              {badges.map((badge) => (
                <span
                  key={`${element.id}-${badge}`}
                  className="rounded-full bg-surface-container-highest px-2 py-0.5 font-label font-bold text-on-surface-variant"
                >
                  {badge}
                </span>
              ))}
            </div>
          </div>

          <div className="grid shrink-0 gap-2 text-right">
            <div className="text-lg font-headline font-bold text-on-surface">
              {formatBytes(element.bytes)}
            </div>
            <span
              className={`rounded-full px-2 py-0.5 text-[10px] font-bold font-label ${
                impactLevel === "HIGH"
                  ? "bg-error-container text-on-error-container"
                  : impactLevel === "MED"
                    ? "bg-secondary-container text-on-secondary-container"
                    : "bg-tertiary-container text-on-tertiary-container"
              }`}
            >
              {impactLevel}
            </span>
          </div>
        </div>
      </button>

      {isActive ? (
        <DominantAssetDetail element={element} />
      ) : null}
    </article>
  );
});

function buildBadges(element: VampireElement) {
  const badges = [formatResourceLabel(element.type)];

  if (element.position_band !== "unknown") {
    badges.push(formatPositionBand(element.position_band));
  }

  if (element.visual_role === "lcp_candidate") {
    badges.push("LCP");
  } else if (element.visual_role !== "unknown") {
    badges.push(formatVisualRole(element.visual_role));
  }
  if (element.is_third_party_tool && element.third_party_kind !== "unknown") {
    badges.push(formatThirdPartyKind(element.third_party_kind));
  }
  if (element.type === "font") {
    badges.push("Font cost");
  }

  return badges.filter((badge, index, all) => badge && all.indexOf(badge) === index);
}

export function getCoverageState(element: VampireElement, capturedHeight: number) {
  if (!element.bounding_box) {
    return {
      label: "No visual anchor",
      className: "bg-surface-container-highest text-on-surface-variant",
    };
  }

  if (element.bounding_box.y >= capturedHeight) {
    return {
      label: "Outside captured range",
      className: "bg-secondary-container text-on-surface",
    };
  }

  return {
    label: "Visible in inspector",
    className: "bg-primary/10 text-primary",
  };
}
