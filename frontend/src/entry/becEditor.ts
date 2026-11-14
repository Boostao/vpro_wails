export type BECKey = 'zone' | 'subZone' | 'siteSeries';
export interface BECCodes {
  zone: string | null;
  subZone: string | null;
  siteSeries: string | null;
}
export interface BECZone {
  zone: string | null;
  description: string | null;
}
export interface BECSubZone extends BECZone {
  subZone: string | null;
}
export interface BECSiteSeries {
  rowId: string;
  sourceId: string | null;
  zone: string | null;
  subZone: string | null;
  siteSeries: string | null;
  description: string | null;
  region: string | null;
  variant: string | null;
  phase: string | null;
  selectable?: boolean;
  diagnostic?: string;
}
export interface BECAPI {
  zones(): Promise<BECZone[]>;
  subZones(zone: string | null): Promise<BECSubZone[]>;
  siteSeries(zone: string | null, subZone: string | null): Promise<BECSiteSeries[]>;
}
export interface BECView {
  zones: BECZone[];
  subZones: BECSubZone[];
  siteSeries: BECSiteSeries[];
  busy: boolean;
  ready: boolean;
  error: string | null;
}
export const becLengths: Record<BECKey, number> = { zone: 4, subZone: 8, siteSeries: 5 };
export const becLabels: Record<BECKey, string> = { zone: 'BEC zone', subZone: 'BEC subzone / variant', siteSeries: 'Site series' };
export const becKeys = ['zone', 'subZone', 'siteSeries'] as const;
export function becKey(column: string | undefined): BECKey | undefined {
  return column === 'Zone' ? 'zone' : column === 'SubZone' ? 'subZone' : column === 'SiteSeries' ? 'siteSeries' : undefined;
}
export function becFingerprint(codes: BECCodes): string {
  return JSON.stringify(becKeys.map(key => codes[key]));
}
export function becChanged(codes: BECCodes, original: BECCodes | null): boolean {
  return becKeys.some(key => codes[key] !== (original?.[key] ?? null));
}
export function becLengthError(codes: BECCodes, original: BECCodes | null = null): string | null {
  for (const key of becKeys) {
    const value = codes[key];
    if (value === original?.[key] || value === null) continue;
    if (value === '') return `Clear ${becLabels[key]} to NULL instead of an empty string.`;
    if (value.length > becLengths[key]) return `${becLabels[key]} must be at most ${becLengths[key]} UTF-16 characters.`;
  }
  return null;
}
export function equalCode(left: string | null, right: string | null): boolean {
  const fold = (value: string) => value.replace(/[A-Z]/g, letter => letter.toLowerCase());
  return left !== null && right !== null && fold(left) === fold(right);
}
export interface BECGroup<T> {
  code: string;
  records: T[];
}
export function becGroups<T>(rows: T[], code: (row: T) => string | null): BECGroup<T>[] {
  const groups = new Map<string, T[]>();
  for (const row of rows) {
    const value = code(row);
    if (value === null || value === '') continue;
    const records = groups.get(value);
    if (records) records.push(row);
    else groups.set(value, [row]);
  }
  return [...groups].map(([value, records]) => ({ code: value, records }));
}
export function becWarnings(codes: BECCodes, view: BECView): string[] {
  if (view.busy) return [];
  if (!view.ready) return [view.error ?? 'The BEC catalogue is unavailable; these codes have not been checked.'];
  const messages: string[] = [];
  if (codes.zone !== null && !view.zones.some(row => equalCode(row.zone, codes.zone))) {
    messages.push(`Zone "${codes.zone}" is not in the catalogue.`);
  }
  if (codes.subZone !== null && !view.subZones.some(row => equalCode(row.subZone, codes.subZone))) {
    messages.push(`Subzone "${codes.subZone}" does not match the current zone.`);
  }
  if (codes.subZone !== null && codes.zone === null) {
    messages.push('A subzone is present without a zone; its classification is incomplete.');
  }
  if (codes.siteSeries !== null && !view.siteSeries.some(row => row.selectable !== false && row.siteSeries !== '' && equalCode(row.siteSeries, codes.siteSeries))) {
    messages.push(`Site series "${codes.siteSeries}" has no matching definition for the current zone and subzone.`);
  }
  return messages;
}
export function becValidation(codes: BECCodes, original: BECCodes | null, view: BECView, acknowledged: boolean): string | null {
  if (!becChanged(codes, original)) return null;
  const invalid = becLengthError(codes, original);
  if (invalid) return invalid;
  if (view.busy) return 'Wait for the BEC choices to finish refreshing.';
  if (becWarnings(codes, view).length && !acknowledged) {
    return 'Review and explicitly acknowledge the unmatched BEC classification codes before saving.';
  }
  return null;
}
const acknowledgements = new WeakMap<object, string>();
export function rememberBECAcknowledgement(draft: object, codes: BECCodes, accepted: boolean): void {
  if (accepted) acknowledgements.set(draft, becFingerprint(codes));
  else acknowledgements.delete(draft);
}
export function becAcknowledged(draft: object, codes: BECCodes): boolean {
  return acknowledgements.get(draft) === becFingerprint(codes);
}

export class BECLookup {
  private revision = 0;
  private disposed = false;
  private zones: Promise<BECZone[]> | null = null;
  private zonesReady = false;
  private view: BECView = { zones: [], subZones: [], siteSeries: [], busy: false, ready: false, error: null };
  constructor(private api: BECAPI, private state: (view: BECView) => void, private cancel?: () => void) {}
  snapshot(): BECView { return { ...this.view }; }
  private publish(update: Partial<BECView>): void {
    this.view = { ...this.view, ...update };
    this.state(this.snapshot());
  }
  async refresh(zone: string | null, subZone: string | null): Promise<void> {
    if (this.disposed) return;
    const revision = ++this.revision;
    if (this.cancel) {
      if (!this.zonesReady) this.zones = null;
      this.cancel();
    }
    this.publish({ busy: true, ready: false, error: null, subZones: [], siteSeries: [] });
    try {
      const invalid = becLengthError({ zone, subZone, siteSeries: null });
      if (invalid) throw new Error(invalid);
      if (!this.zones) {
        const pending = this.api.zones().then(rows => {
          if (this.zones === pending) this.zonesReady = true;
          return rows;
        }, cause => {
          if (this.zones === pending) { this.zones = null; this.zonesReady = false; }
          throw cause;
        });
        this.zones = pending;
      }
      const [zones, subZones, siteSeries] = await Promise.all([
        this.zones, this.api.subZones(zone), this.api.siteSeries(zone, subZone)
      ]);
      if (![zones, subZones, siteSeries].every(Array.isArray)) throw new Error('The BEC service did not return catalogue arrays.');
      if (this.disposed || revision !== this.revision) return;
      this.publish({ zones, subZones, siteSeries, ready: true, busy: false });
    } catch (cause) {
      if (this.disposed || revision !== this.revision) return;
      this.publish({ ready: false, busy: false, error: `BEC choices could not be loaded: ${String(cause)}` });
    }
  }
  dispose(): void {
    this.disposed = true;
    this.revision++;
    this.cancel?.();
  }
}
