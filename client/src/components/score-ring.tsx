import { formatEntropyLabel } from "@/lib/api";

const scoreProgress: Record<string, number> = {
  A: 94,
  B: 80,
  C: 62,
  D: 46,
  E: 28,
  F: 12,
};

interface ScoreRingProps {
  score: string;
  grams: string;
}

export function ScoreRing({ score, grams }: ScoreRingProps) {
  const entropyLabel = formatEntropyLabel(score);
  const progress = scoreProgress[score] ?? 0;

  return (
    <article className="bg-surface-container-low p-6 lg:p-8 rounded-3xl flex flex-col gap-2 group hover:bg-surface-container transition-colors h-full w-full border border-outline-variant/5 hover:border-outline-variant/20">
      <div className="flex justify-between w-full">
        <span className="text-on-surface-variant text-xs uppercase tracking-widest font-label flex items-center gap-2">
          Carbon Score
        </span>
        <div className="opacity-50 text-xs font-headline font-bold text-primary">
          GRADE {score}
        </div>
      </div>
      
      <div className="text-5xl font-headline font-bold text-primary mt-2 flex items-baseline gap-2">
        {grams.split(" ")[0]}
        <span className="text-2xl opacity-60 font-body">
           {grams.split(" ").slice(1).join(" ")}
        </span>
      </div>

      <p className="text-sm text-on-surface-variant mt-2 italic">
        {entropyLabel}. Referencia interna: {progress}/100.
      </p>

      <div className="w-full h-1.5 bg-surface-container-highest rounded-full mt-4 overflow-hidden">
        <div
          className="h-full bg-primary"
          style={{ width: `${progress}%` }}
        />
      </div>
      
      <p className="mt-4 text-sm leading-7 text-on-surface-variant/80">
        Menos transferencia, menos bloqueo en el render y menos dependencia de
        terceros empujan esta nota hacia arriba.
      </p>
    </article>
  );
}
