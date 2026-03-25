const scoreProgress: Record<string, number> = {
  A: 92,
  B: 78,
  C: 64,
  D: 48,
  E: 32,
  F: 18,
};

interface ScoreRingProps {
  score: string;
  grams: string;
}

export function ScoreRing({ score, grams }: ScoreRingProps) {
  const progress = scoreProgress[score] ?? 0;
  const radius = 84;
  const circumference = 2 * Math.PI * radius;
  const dashOffset = circumference - (progress / 100) * circumference;

  return (
    <div className="panel flex flex-col rounded-[2rem] p-6">
      <div className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
        Score de carbono
      </div>
      <div className="relative mt-6 flex items-center justify-center">
        <svg className="ring-glow h-56 w-56 -rotate-90" viewBox="0 0 220 220">
          <circle
            cx="110"
            cy="110"
            r={radius}
            fill="none"
            stroke="rgba(255,255,255,0.07)"
            strokeWidth="18"
          />
          <circle
            cx="110"
            cy="110"
            r={radius}
            fill="none"
            stroke="url(#scoreGradient)"
            strokeDasharray={circumference}
            strokeDashoffset={dashOffset}
            strokeLinecap="round"
            strokeWidth="18"
          />
          <defs>
            <linearGradient id="scoreGradient" x1="0%" x2="100%">
              <stop offset="0%" stopColor="#9bd67e" />
              <stop offset="100%" stopColor="#d8ff7f" />
            </linearGradient>
          </defs>
        </svg>
        <div className="absolute text-center">
          <div className="text-6xl font-medium tracking-[-0.08em] text-white">
            {score}
          </div>
          <div className="mono mt-2 text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
            {grams}
          </div>
        </div>
      </div>
      <p className="mt-4 text-center text-sm leading-6 text-[var(--muted)]">
        Menos transferencia, menos terceros pesados y mejor infraestructura hacen subir la nota.
      </p>
    </div>
  );
}
