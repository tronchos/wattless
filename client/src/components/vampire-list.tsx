import {
  formatBytes,
  formatResourceLabel,
} from "@/lib/api";
import type { VampireElement } from "@/lib/types";

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
              
              // Impact styling logic based on estimated savings or type
              let impactLevel: "HIGH" | "MED" | "LOW" = "LOW";
              if (element.estimated_savings_bytes > 50000) impactLevel = "HIGH";
              else if (element.estimated_savings_bytes > 10000) impactLevel = "MED";

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
                    <div className="text-[10px] text-on-surface-variant mt-1">
                       {formatResourceLabel(element.type)}
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
