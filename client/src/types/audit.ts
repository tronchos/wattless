export type AssetImpact = "HIGH" | "MED" | "LOW";

export interface AssetNode {
  filename: string;
  size: string;
  impact: AssetImpact;
}

export interface MetricScore {
  value: string;
  unit: string;
  label: string;
  icon: string;
  description: string;
  scorePercentage: number;
}

export interface GreenFix {
  id: string;
  title: string;
  description: string;
  originalCode: string;
  optimizedCode: string;
}

export interface AuditReport {
  url: string;
  carbonScore: MetricScore;
  payloadSize: MetricScore;
  performance: MetricScore;
  assets: AssetNode[];
  greenFixes: GreenFix[];
  summary: {
    grade: string;
    energyEfficiency: string;
    cleanHosting: string;
    actionItems: string[];
  };
}
