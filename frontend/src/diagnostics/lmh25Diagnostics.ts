import { Call } from '@wailsio/runtime';

export type DiagnosticSeverity = 'info' | 'warning' | 'error';

export interface DiagnosticIssue {
  code: string;
  severity: DiagnosticSeverity;
  category: string;
  field?: string;
  horizonId?: number | null;
  horizon?: string;
  message: string;
  suggested?: string;
  lmh25Ref: string;
}

export interface KeyTraceStep {
  key: string;
  step: string;
  condition: string;
  outcome: string;
}

export interface PlotDiagnosticsResult {
  plotNumber: string;
  derivedHumusThickness: number | null;
  headerHumusThickness: number | null;
  humusThicknessDiff: number | null;
  substratePopulated: number;
  substrateSum: number | null;
  substrateValid: boolean;
  isOrganicSoilOrder: boolean;
  keyedSmr: string;
  keyedSmrClass: number | null;
  fieldSmr: string;
  fieldSmrClass: number | null;
  smrDivergence: number | null;
  smrDivergent: boolean;
  smrTrace: KeyTraceStep[];
  keyedSnr: string;
  keyedSnrClass: number | null;
  fieldSnr: string;
  fieldSnrClass: number | null;
  snrDivergence: number | null;
  snrDivergent: boolean;
  snrTrace: KeyTraceStep[];
  issues: DiagnosticIssue[];
}

export interface LMH25DiagnosticsPort {
  evaluatePlotDiagnostics(
    header: Record<string, unknown>,
    humus: Record<string, unknown>[],
    mineral: Record<string, unknown>[]
  ): Promise<PlotDiagnosticsResult>;
  evaluateStoredPlotDiagnostics(contextID: string, plotNumber: string): Promise<PlotDiagnosticsResult>;
}

export const defaultLMH25DiagnosticsPort: LMH25DiagnosticsPort = {
  evaluatePlotDiagnostics: (header, humus, mineral) =>
    Call.ByName('main.PlotEntryService.EvaluatePlotDiagnostics', header, humus, mineral),
  evaluateStoredPlotDiagnostics: (contextID, plotNumber) =>
    Call.ByName('main.PlotEntryService.EvaluateStoredPlotDiagnostics', contextID, plotNumber),
};

export function substrateProgressPercent(sum: number | null): number {
  if (sum === null || !Number.isFinite(sum) || sum <= 0) return 0;
  if (sum >= 100) return 100;
  return Math.round(sum * 100) / 100;
}

export function substrateStatusLabel(result: PlotDiagnosticsResult | null): string {
  if (!result || result.substratePopulated === 0 || result.substrateSum === null) {
    return 'No substrates entered';
  }
  if (result.substrateValid && Math.abs(result.substrateSum - 100) <= 0.01) {
    return `${result.substrateSum.toFixed(1)}% (Valid 100%)`;
  }
  return `${result.substrateSum.toFixed(1)}% (Expected 100%)`;
}

export function formatHumusComparison(result: PlotDiagnosticsResult | null): string {
  if (!result || result.derivedHumusThickness === null) {
    return 'No upland L/F/H depths';
  }
  const derived = `${result.derivedHumusThickness.toFixed(1)} cm`;
  if (result.headerHumusThickness === null) {
    return `Derived ${derived} (Header empty)`;
  }
  if (result.humusThicknessDiff !== null && Math.abs(result.humusThicknessDiff) > 0.05) {
    const sign = result.humusThicknessDiff > 0 ? '+' : '';
    return `Derived ${derived} vs Header ${result.headerHumusThickness.toFixed(1)} cm (${sign}${result.humusThicknessDiff.toFixed(1)} cm)`;
  }
  return `${derived} (Matches Header)`;
}

export function countDiagnosticsBySeverity(issues: DiagnosticIssue[] | null | undefined): {
  errors: number;
  warnings: number;
  infos: number;
} {
  let errors = 0;
  let warnings = 0;
  let infos = 0;
  for (const item of issues ?? []) {
    if (item.severity === 'error') errors++;
    else if (item.severity === 'warning') warnings++;
    else infos++;
  }
  return { errors, warnings, infos };
}

export function normalizeDiagnosticsWire(raw: unknown): PlotDiagnosticsResult {
  if (!raw || typeof raw !== 'object') {
    throw new Error('Invalid LMH25 PlotDiagnosticsResult payload');
  }
  const obj = raw as Partial<PlotDiagnosticsResult>;
  if (typeof obj.plotNumber !== 'string') {
    throw new Error('LMH25 diagnostics requires string plotNumber');
  }
  return {
    plotNumber: obj.plotNumber,
    derivedHumusThickness: typeof obj.derivedHumusThickness === 'number' ? obj.derivedHumusThickness : null,
    headerHumusThickness: typeof obj.headerHumusThickness === 'number' ? obj.headerHumusThickness : null,
    humusThicknessDiff: typeof obj.humusThicknessDiff === 'number' ? obj.humusThicknessDiff : null,
    substratePopulated: typeof obj.substratePopulated === 'number' ? obj.substratePopulated : 0,
    substrateSum: typeof obj.substrateSum === 'number' ? obj.substrateSum : null,
    substrateValid: Boolean(obj.substrateValid),
    isOrganicSoilOrder: Boolean(obj.isOrganicSoilOrder),
    keyedSmr: typeof obj.keyedSmr === 'string' ? obj.keyedSmr : '',
    keyedSmrClass: typeof obj.keyedSmrClass === 'number' ? obj.keyedSmrClass : null,
    fieldSmr: typeof obj.fieldSmr === 'string' ? obj.fieldSmr : '',
    fieldSmrClass: typeof obj.fieldSmrClass === 'number' ? obj.fieldSmrClass : null,
    smrDivergence: typeof obj.smrDivergence === 'number' ? obj.smrDivergence : null,
    smrDivergent: Boolean(obj.smrDivergent),
    smrTrace: Array.isArray(obj.smrTrace) ? obj.smrTrace : [],
    keyedSnr: typeof obj.keyedSnr === 'string' ? obj.keyedSnr : '',
    keyedSnrClass: typeof obj.keyedSnrClass === 'number' ? obj.keyedSnrClass : null,
    fieldSnr: typeof obj.fieldSnr === 'string' ? obj.fieldSnr : '',
    fieldSnrClass: typeof obj.fieldSnrClass === 'number' ? obj.fieldSnrClass : null,
    snrDivergence: typeof obj.snrDivergence === 'number' ? obj.snrDivergence : null,
    snrDivergent: Boolean(obj.snrDivergent),
    snrTrace: Array.isArray(obj.snrTrace) ? obj.snrTrace : [],
    issues: Array.isArray(obj.issues) ? obj.issues : [],
  };
}
