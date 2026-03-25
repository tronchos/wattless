import {
  formatBytes,
  formatPositionBand,
  formatResourceLabel,
  formatThirdPartyKind,
  formatVisualRole,
} from "@/lib/api";
import type { VampireElement } from "@/lib/types";

interface VampireListProps {
  elements: VampireElement[];
  selectedElementID: string | null;
  capturedHeight: number;
  onSelect: (id: string) => void;
}

export function VampireList({
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
      
      <div className="bg-surface-container-low rounded-xl overflow-hidden flex-1 border border-outline-variant/10">
        <table className="w-full text-left text-sm block md:table">
          <thead className="bg-surface-container-high text-on-surface-variant uppercase text-[10px] tracking-widest block md:table-header-group">
            <tr className="block md:table-row">
              <th className="px-4 py-3 font-medium font-label">Type</th>
              <th className="px-4 py-3 font-medium font-label">Size</th>
              <th className="px-4 py-3 font-medium font-label">Impact</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-outline-variant/10 block md:table-row-group">
            {elements.map((element) => {
              const isActive = selectedElement?.id === element.id;
              const coverage = getCoverageState(element, capturedHeight);
              const badges = buildBadges(element);
              
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
                <tr
                  key={element.id}
                  onClick={() => onSelect(element.id)}
                  className={`cursor-pointer transition-colors block md:table-row ${
                    isActive
                      ? "bg-surface-container-highest"
                      : "hover:bg-surface-container-highest/50"
                  }`}
                >
                  <td className="px-4 py-4 font-body block md:table-cell">
                    <div className="text-on-surface truncate max-w-[150px] sm:max-w-[200px]" title={element.url}>
                       {element.url.split('/').pop() || formatResourceLabel(element.type)}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-on-surface-variant">
                       <span>{formatResourceLabel(element.type)}</span>
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
                  </td>
                  <td className="px-4 py-4 opacity-70 block md:table-cell">
                    {formatBytes(element.bytes)}
                  </td>
                  <td className="px-4 py-4 block md:table-cell">
                    {impactLevel === "HIGH" && (
                       <span className="bg-error-container text-on-error-container px-2 py-0.5 rounded-full text-[10px] font-bold font-label">HIGH</span>
                    )}
                    {impactLevel === "MED" && (
                       <span className="bg-secondary-container text-on-secondary-container px-2 py-0.5 rounded-full text-[10px] font-bold font-label">MED</span>
                    )}
                    {impactLevel === "LOW" && (
                       <span className="bg-tertiary-container text-on-tertiary-container px-2 py-0.5 rounded-full text-[10px] font-bold font-label">LOW</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function buildBadges(element: VampireElement) {
  const badges = [formatPositionBand(element.position_band)];

  if (element.visual_role !== "unknown") {
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

function getCoverageState(element: VampireElement, capturedHeight: number) {
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
