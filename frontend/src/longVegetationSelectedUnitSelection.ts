import type { LongVegetationSelectedUnitField, LongVegetationSelectedUnitSelection } from '../bindings/github.com/boostao/vpro-wails';
import type { GoogleEarthScope } from './googleEarthReview';
import { ownedScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { validateSelectedUnitOccurrences } from './longVegetationReport';

export type SelectedUnitSelectionReview = Omit<LongVegetationSelectedUnitSelection, 'fields' | 'definitions' | 'build'> & {
  fields: string[];
  definitions: NonNullable<LongVegetationSelectedUnitSelection['definitions']>;
  build: (Omit<NonNullable<LongVegetationSelectedUnitSelection['build']>, 'occurrences'> & {
    occurrences: NonNullable<NonNullable<LongVegetationSelectedUnitSelection['build']>['occurrences']>;
  }) | null;
};
const shape = (value: unknown, keys: string[]): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && Object.keys(value).length === keys.length &&
  keys.every(key => Object.hasOwn(value, key));
const text = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
const revision = (value: unknown): value is string => typeof value === 'string' && /^(0|[1-9]\d*)$/.test(value) &&
  BigInt(value) <= 9223372036854775807n;
const fieldName = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_]+$/.test(value);

export function selectedUnitFieldError(fields: string[], definitions: SelectedUnitSelectionReview['definitions']): string {
  if (!fields.length) return 'Select at least one physical field before Build, or Undo the field draft.';
  const seen = new Set<string>();
  for (const name of fields) {
    if (!fieldName(name) || seen.has(name.toLowerCase())) {
      return 'Selected fields require distinct physical identifiers; correct the list or Undo.';
    }
    seen.add(name.toLowerCase());
    const matches = definitions.filter(field => field.name.toLowerCase() === name.toLowerCase());
    if (matches.length !== 1 || !matches[0].available) {
      return `Selected field ${name} is absent, ambiguous or unavailable; correct it or Undo.`;
    }
  }
  return '';
}

export function validateReportUnitFieldDefinitions(values: LongVegetationSelectedUnitField[] | null): LongVegetationSelectedUnitField[] {
  if (!Array.isArray(values)) throw new Error('Report unit Field List is incomplete.');
  const definitions = values.map(field => {
    if (!shape(field, ['table', 'name', 'declaredType', 'available', 'reason']) ||
        !['Env', 'Admin'].includes(field.table) || !text(field.name) || !text(field.declaredType) ||
        typeof field.available !== 'boolean' || !text(field.reason) ||
        field.available && (!fieldName(field.name) || !['TEXT', 'VARCHAR', 'INTEGER', 'LONG', 'BOOLEAN', 'BIT'].includes(field.declaredType) || field.reason !== '') ||
        !field.available && !field.reason) {
      throw new Error('Selected unit Field List lost original physical definitions or availability diagnostics.');
    }
    return { ...field };
  });
  const byName = new Map<string, number>();
  for (const field of definitions) byName.set(field.name.toLowerCase(), (byName.get(field.name.toLowerCase()) ?? 0) + 1);
  if (definitions.some(field => field.available && byName.get(field.name.toLowerCase()) !== 1)) {
    throw new Error('Selected unit Field List silently resolved duplicate physical names.');
  }
  return definitions;
}

export function validateSelectedUnitSelection(value: LongVegetationSelectedUnitSelection | null,
  owner: GoogleEarthScope): SelectedUnitSelectionReview {
  if (!shape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'revision', 'fields', 'definitions', 'build', 'buildCurrent']) ||
      !ownedScope(value, owner) || !revision(value.revision) || !Array.isArray(value.fields) ||
      value.fields.some(field => !text(field)) || !Array.isArray(value.definitions) || typeof value.buildCurrent !== 'boolean') {
    throw new Error('Selected unit fields belong to an incomplete or different owned session; no defaults were inferred.');
  }
  const definitions = validateReportUnitFieldDefinitions(value.definitions);
  let build: SelectedUnitSelectionReview['build'] = null;
  if (value.build !== null) {
    if (!shape(value.build, ['approvalHash', 'occurrences']) ||
        typeof value.build.approvalHash !== 'string' || !/^[0-9a-f]{64}$/.test(value.build.approvalHash)) {
      throw new Error('Selected unit Build receipt has an incomplete approval witness.');
    }
    validateSelectedUnitOccurrences(value.build.occurrences);
    if (value.buildCurrent && selectedUnitFieldError(value.fields, definitions)) {
      throw new Error('Current selected unit Build contradicts its physical field selection.');
    }
    build = { approvalHash: value.build.approvalHash, occurrences: value.build.occurrences!.map(row => ({
      ...row, plotNumber: { ...row.plotNumber }, siteUnit: { ...row.siteUnit },
    })) };
  } else if (value.buildCurrent) {
    throw new Error('Selected unit selection invented current approval without an explicit Build receipt.');
  }
  return { ...owner, revision: value.revision, fields: [...value.fields], definitions, build, buildCurrent: value.buildCurrent };
}

export type SelectedUnitSelectionView = {
  review: SelectedUnitSelectionReview | null;
  fields: string[];
  dirty: boolean;
  draftError: string;
  error: string;
  busy: boolean;
  authorityUnknown: boolean;
};

export class SelectedUnitSelectionSession {
  private review: SelectedUnitSelectionReview | null = null;
  private fields: string[] = [];
  private dirty = false;
  private error = '';
  private busy = false;
  private changingSelection = false;
  private authorityUnknown = false;
  private generation = 0;
  private readonly listeners = new Set<() => void>();
  constructor(private readonly owner: GoogleEarthScope) {}
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
  private notify(): void { for (const listener of this.listeners) listener(); }
  view(): SelectedUnitSelectionView {
    return {
      review: this.review ? validateSelectedUnitSelection(this.review, this.owner) : null,
      fields: [...this.fields], dirty: this.dirty, error: this.error, busy: this.busy, authorityUnknown: this.authorityUnknown,
      draftError: this.review ? selectedUnitFieldError(this.fields, this.review.definitions) : '',
    };
  }
  cancel(): void {
    if (this.busy && this.changingSelection) this.authorityUnknown = true;
    this.generation++; this.busy = false; this.notify();
  }
  edit(fields: string[]): void {
    if (!this.review || this.busy) throw new Error('Load owned Field List before editing, and finish or cancel the current read.');
    this.generation++; this.fields = [...fields]; this.dirty = true; this.error = ''; this.notify();
  }
  undo(): void {
    if (!this.review || this.busy) throw new Error('Load owned Field List before Undo, and finish or cancel the current read.');
    this.generation++; this.fields = [...this.review.fields]; this.dirty = false; this.error = ''; this.notify();
  }
  private apply(value: LongVegetationSelectedUnitSelection | null, preserveDraft: boolean): void {
    const review = validateSelectedUnitSelection(value, this.owner);
    this.review = review;
    this.authorityUnknown = false;
    if (!preserveDraft || !this.dirty) { this.fields = [...review.fields]; this.dirty = false; }
    this.error = '';
  }
  async load(fetch: () => Promise<LongVegetationSelectedUnitSelection | null>): Promise<void> {
    if (this.busy) throw new Error('Selected unit field read is already running.');
    const request = ++this.generation;
    this.busy = true; this.changingSelection = false; this.error = ''; this.notify();
    try {
      const value = await fetch();
      if (request !== this.generation) return;
      this.apply(value, true);
    } catch (cause) {
      if (request === this.generation) {
        this.error = `Selected unit fields unavailable; no defaults applied: ${String(cause)}`;
      }
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }
  async build(set: (request: { revision: string; fields: string[] }) => Promise<LongVegetationSelectedUnitSelection | null>,
    build: (revision: string) => Promise<LongVegetationSelectedUnitSelection | null>): Promise<void> {
    if (!this.review || this.busy || this.authorityUnknown || selectedUnitFieldError(this.fields, this.review.definitions)) {
      throw new Error('Load and correct the owned selected field draft before Build.');
    }
    const request = ++this.generation, fields = [...this.fields], expected = this.review.revision;
    this.busy = true; this.changingSelection = true; this.error = ''; this.notify();
    try {
      const draft = validateSelectedUnitSelection(await set({ revision: expected, fields }), this.owner);
      if (request !== this.generation) return;
      if (draft.revision !== String(BigInt(expected) + 1n) || draft.build !== null ||
          draft.fields.length !== fields.length || draft.fields.some((field, index) => field !== fields[index])) {
        throw new Error('Selected field draft acknowledgement differs from the explicit ordered request.');
      }
      this.apply(draft, false);
      const approved = validateSelectedUnitSelection(await build(draft.revision), this.owner);
      if (request !== this.generation) return;
      if (!approved.buildCurrent || !approved.build || approved.revision !== String(BigInt(draft.revision) + 1n) ||
          approved.fields.length !== fields.length || approved.fields.some((field, index) => field !== fields[index])) {
        throw new Error('Selected unit Build differs from the explicit owned draft or lost current approval.');
      }
      this.apply(approved, false);
    } catch (cause) {
      if (request === this.generation) {
        this.authorityUnknown = true;
        this.error = `Selected unit Build unavailable; reload owned fields before retry: ${String(cause)}`;
      }
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }
}

export function createSelectedUnitSelectionRegistry() {
  const sessions = new Map<string, { owner: GoogleEarthScope; session: SelectedUnitSelectionSession }>();
  return (owner: GoogleEarthScope): SelectedUnitSelectionSession => {
    const existing = sessions.get(owner.contextId);
    if (existing && !ownedScope(existing.owner, owner)) throw new Error('Selected unit session identity changed physical paths.');
    if (existing) return existing.session;
    const session = new SelectedUnitSelectionSession({ ...owner });
    sessions.set(owner.contextId, { owner: { ...owner }, session });
    return session;
  };
}
