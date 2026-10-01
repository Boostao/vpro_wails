import { becGroups, equalCode } from './becEditor';

export const qualityKeys = ['sitePlotQuality', 'vegPlotQuality', 'soilPlotQuality'] as const;
export type QualityKey = typeof qualityKeys[number];
export type QualityCodes = Record<QualityKey, string | null>;
export const qualityLabels: Record<QualityKey, string> = {
  sitePlotQuality: 'Site data quality', vegPlotQuality: 'Vegetation data quality', soilPlotQuality: 'Soil data quality'
};
export interface PlotQualityChoice {
  rowId: string;
  code: string | null;
  listName: string | null;
  listFilter: string | null;
  itemOrder: number | null;
  description: string | null;
  fieldUsedIn: string | null;
  validateLoops: string | null;
  validate: boolean | null;
  note: string | null;
  flag: boolean | null;
  selectable: boolean;
  diagnostic: string;
}
export interface QualityView {
  choices: PlotQualityChoice[];
  busy: boolean;
  ready: boolean;
  error: string | null;
}
export function qualityKey(column: string | undefined): QualityKey | undefined {
  return column === 'SitePlotQuality' ? 'sitePlotQuality' : column === 'VegPlotQuality' ? 'vegPlotQuality'
    : column === 'SoilPlotQuality' ? 'soilPlotQuality' : undefined;
}
export function wellFormedUTF16(value: string): boolean {
  for (let i = 0; i < value.length; i++) {
    const unit = value.charCodeAt(i);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(++i);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false;
  }
  return true;
}
export function qualityLengthError(codes: QualityCodes, original: QualityCodes | null = null): string | null {
  for (const key of qualityKeys) {
    const error = qualityValueError(key, codes[key], original?.[key] ?? null);
    if (error) return error;
  }
  return null;
}
export function qualityValueError(key: QualityKey, value: string | null, original: string | null): string | null {
  if (value === null || value === original) return null;
  if (value === '') return `Clear ${qualityLabels[key]} to NULL instead of an empty string.`;
  if (!wellFormedUTF16(value)) return `${qualityLabels[key]} contains an incomplete Unicode character. Correct it instead of truncating or replacing it.`;
  if (value.length > 15) return `${qualityLabels[key]} must be at most 15 UTF-16 characters; the raw entry has not been truncated.`;
  return null;
}
export function qualityChanged(codes: QualityCodes, original: QualityCodes | null): boolean {
  return qualityKeys.some(key => codes[key] !== (original?.[key] ?? null));
}
export function qualityGroups(rows: PlotQualityChoice[]) {
  return becGroups(rows, row => row.selectable && row.code !== null && row.code.length <= 15 && wellFormedUTF16(row.code) ? row.code : null);
}
export function qualityDefinitions(rows: PlotQualityChoice[], code: string | null): PlotQualityChoice[] {
  return rows.filter(row => row.selectable && equalCode(row.code, code));
}
export function qualityWarnings(codes: QualityCodes, original: QualityCodes | null, view: QualityView): string[] {
  if (view.busy) return [];
  const changed = qualityKeys.filter(key => codes[key] !== (original?.[key] ?? null) && codes[key] !== null);
  if (!changed.length) return [];
  if (!view.ready) return [view.error ?? 'Quality choices are unavailable; changed raw codes have not been checked.'];
  return changed.filter(key => !qualityDefinitions(view.choices, codes[key]).length)
    .map(key => `${qualityLabels[key]} "${codes[key]}" is not in the shared PlotQualitySite list. It will be kept exactly as entered, not converted to a note code.`);
}
export function qualityValidation(codes: QualityCodes, original: QualityCodes | null, view: QualityView, acknowledged: boolean): string | null {
  if (!qualityChanged(codes, original)) return null;
  const invalid = qualityLengthError(codes, original);
  if (invalid) return invalid;
  if (view.busy) return 'Wait for quality choices to finish refreshing.';
  return qualityWarnings(codes, original, view).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched quality codes before saving.' : null;
}
function fingerprint(codes: QualityCodes): string { return JSON.stringify(qualityKeys.map(key => codes[key])); }
const acknowledgements = new WeakMap<object, string>();
export function rememberQualityAcknowledgement(draft: object, codes: QualityCodes, accepted: boolean): void {
  if (accepted) acknowledgements.set(draft, fingerprint(codes));
  else acknowledgements.delete(draft);
}
export function qualityAcknowledged(draft: object, codes: QualityCodes): boolean {
  return acknowledgements.get(draft) === fingerprint(codes);
}
export class QualityLookup {
  private revision = 0;
  private disposed = false;
  private view: QualityView = { choices: [], busy: false, ready: false, error: null };
  constructor(private fetch: () => Promise<PlotQualityChoice[]>, private state: (view: QualityView) => void,
    private label = 'Quality') {}
  snapshot(): QualityView { return { ...this.view }; }
  private publish(update: Partial<QualityView>): void {
    this.view = { ...this.view, ...update };
    this.state(this.snapshot());
  }
  async refresh(): Promise<void> {
    if (this.disposed) return;
    const revision = ++this.revision;
    this.publish({ choices: [], busy: true, ready: false, error: null });
    try {
      const choices = await this.fetch();
      if (!Array.isArray(choices)) throw new Error(`${this.label} service did not return choice rows.`);
      if (this.disposed || revision !== this.revision) return;
      this.publish({ choices, ready: true, busy: false });
    } catch (cause) {
      if (this.disposed || revision !== this.revision) return;
      this.publish({ busy: false, ready: false, error: `${this.label} choices could not be loaded: ${String(cause)}` });
    }
  }
  dispose(): void { this.disposed = true; this.revision++; }
}
