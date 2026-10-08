import normal from '../../resources/fs882-two-page-layout.json';
import chars from '../../resources/fs882-two-page-chars-layout.json';
import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { validateSourceParentPhysical, type SIVIParentOriginal } from './siviParentEditor';
import { sourceParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort } from './siviParentWriteSession';
import { conservativePort } from './siviParentSharedSession';
import { ordinaryNumberValue, ordinaryTextError } from './ordinaryEditor';
import { equalCell, metadataCellText } from './projectMetadataEditor';

export type TwoPageParentForm = 'FS882-8x6XL' | 'FS882-8x6XL-CHARS';
const sources = { 'FS882-8x6XL': normal, 'FS882-8x6XL-CHARS': chars };
const policies = {
  SV_WaterTableCM: { owner: 'Env', kind: 'single' },
  ActiveLayerDepth: { owner: 'Env', kind: 'single' },
  SV_FullCruiseCard: { owner: 'Env', kind: 'text', maximum: 50 },
  PlotSize: { owner: 'Admin', kind: 'single' },
  ProvinceStateTerritory: { owner: 'Admin', kind: 'text', maximum: 255 },
  SiteUnitLongName: { owner: 'Admin', kind: 'text', maximum: 100 },
  GIS_BGC: { owner: 'Admin', kind: 'text', maximum: 255 },
  GIS_BGC_VER: { owner: 'Admin', kind: 'integer' },
  BEC_Use: { owner: 'Admin', kind: 'text', maximum: 255 },
} as const;
export type TwoPageParentColumn = keyof typeof policies;
export type TwoPageParentInput = { kind: 'original' | 'clear' } | { kind: 'text'; raw: string };
export interface TwoPageParentEdit {
  contextId: string; table: string; rowId: string; column: TwoPageParentColumn;
  expected: ProjectMetadataCell; value: ProjectMetadataCell;
}
export interface TwoPageParentDraft extends TwoPageParentEdit {
  input: TwoPageParentInput; error: string | null;
}
export type TwoPageParentDrafts = Partial<Record<TwoPageParentColumn, TwoPageParentDraft>>;
export interface TwoPageParentWrite { original: SIVIParentOriginal; edits: TwoPageParentEdit[] }
export type TwoPageParentSessionCache = Map<string, Map<TwoPageParentForm, TwoPageParentSession>>;

export function twoPageParentSource(form: TwoPageParentForm) {
  if (!Object.hasOwn(sources, form)) throw new Error('Exact two-page source variant is required.');
  const parent = sources[form].forms[0];
  if (parent.name !== form || parent.recordSource !== 'USysEnv') {
    throw new Error('Two-page parent source closure changed.');
  }
  return parent;
}

export function twoPageParentFields(form: TwoPageParentForm) {
  return twoPageParentSource(form).fields.flatMap(field => {
    if (!field.binding || !Object.hasOwn(policies, field.binding)) return [];
    if (!('caption' in field) || !field.caption) throw new Error(`Two-page field ${field.binding} has no source label.`);
    const column = field.binding as TwoPageParentColumn;
    return [{ column, label: field.caption, controlId: field.controlId,
      page: 'pageId' in field ? field.pageId : null, policy: policies[column] }];
  });
}

export function validateTwoPageParentOriginal(original: SIVIParentOriginal, form: TwoPageParentForm): SIVIParentOriginal {
  const controls = twoPageParentSource(form).fields.filter(field => field.binding);
  const expected = form === 'FS882-8x6XL' ? 121 : 122;
  if (!original || original.Form !== form || controls.length !== expected ||
      new Set(controls.map(field => field.binding)).size !== (form === 'FS882-8x6XL' ? 118 : 120)) {
    throw new Error('Two-page parent requires its exact source variant and every control instance.');
  }
  validateSourceParentPhysical(original, controls.map(field => ({
    Binding: field.binding!, ControlID: field.controlId, Implicit: false,
  })));
  const fields = twoPageParentFields(form);
  if (fields.length !== (form === 'FS882-8x6XL' ? 9 : 8) ||
      new Set(fields.map(field => field.column)).size !== fields.length) {
    throw new Error('Two-page additional-field source scope changed.');
  }
  for (const field of fields) {
    const binding = original.Bindings.find(binding => binding.ControlID === field.controlId);
    if (!binding || binding.Table !== `${original.Project}_${field.policy.owner}`) {
      throw new Error('Two-page additional-field physical ownership changed.');
    }
  }
  return structuredClone(original);
}

export function twoPageParentOriginalFromWire(wire: unknown, owner: SIVIParentOwner,
  form: TwoPageParentForm): SIVIParentOriginal {
  return sourceParentOriginalFromWire(wire, owner, original => validateTwoPageParentOriginal(original, form));
}

function target(original: SIVIParentOriginal, column: TwoPageParentColumn) {
  const field = twoPageParentFields(original.Form as TwoPageParentForm).find(field => field.column === column);
  const binding = original.Bindings.find(binding => binding.ControlID === field?.controlId);
  if (!field || !binding || binding.Implicit || binding.Binding !== column ||
      binding.Table !== `${original.Project}_${field.policy.owner}`) {
    throw new Error('Two-page additional-field source/physical ownership changed.');
  }
  const row = original.Rows[0][field.policy.owner];
  return { field, binding, row, expected: structuredClone(row.cells[binding.Column]) };
}
export function twoPageParentCell(original: SIVIParentOriginal, column: TwoPageParentColumn) {
  return target(original, column).expected;
}
export function twoPageParentErrors(drafts: TwoPageParentDrafts): string[] {
  return Object.values(drafts).flatMap(draft => draft?.error ? [draft.error] : []);
}
export function twoPageParentDirty(drafts: TwoPageParentDrafts): boolean {
  return Object.values(drafts).some(draft => draft && (draft.error !== null || !equalCell(draft.expected, draft.value)));
}
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

function stage(original: SIVIParentOriginal, drafts: TwoPageParentDrafts, column: TwoPageParentColumn,
  input: TwoPageParentInput): TwoPageParentDrafts {
  const { field, binding, row, expected } = target(original, column);
  let value = structuredClone(expected), error: string | null = null;
  if (input.kind === 'clear') value = nullCell();
  else if (input.kind === 'text') {
    if (typeof input.raw !== 'string') throw new Error('Two-page input requires literal text.');
    if (!(expected.storage !== 'null' && input.raw === metadataCellText(expected))) {
      const policy = { ...field.policy, label: field.label };
      if (policy.kind === 'text') {
        error = ordinaryTextError(policy, input.raw, expected.storage === 'text' ? expected.text : null);
        value = { ...nullCell(), storage: 'text', text: input.raw };
      } else {
        const previous = expected.storage === 'real' ? expected.real :
          expected.storage === 'integer' && Number.isSafeInteger(Number(expected.integer)) ? Number(expected.integer) : null;
        const parsed = ordinaryNumberValue(policy, input.raw, null);
        error = parsed.error;
        value = parsed.value !== null && parsed.value === previous && !error ? structuredClone(expected)
          : parsed.value === null ? nullCell() : policy.kind === 'integer'
          ? { ...nullCell(), storage: 'integer', integer: String(parsed.value) }
          : { ...nullCell(), storage: 'real', real: parsed.value };
      }
    }
  } else if (input.kind !== 'original') throw new Error('Two-page input intent is unavailable.');
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  return { ...drafts, [column]: { contextId: original.ContextID, table: binding.Table, rowId: row.rowId,
    column, expected, value, input: structuredClone(input), error } };
}

export function twoPageParentRequest(original: SIVIParentOriginal, drafts: TwoPageParentDrafts,
  form: TwoPageParentForm): TwoPageParentWrite {
  const reviewed = validateTwoPageParentOriginal(original, form), edits: TwoPageParentEdit[] = [];
  for (const [name, draft] of Object.entries(drafts)) {
    const field = twoPageParentFields(form).find(field => field.column === name);
    if (!field || !draft || draft.column !== field.column) throw new Error('Two-page writes exclude other fields and callbacks.');
    const verified = stage(reviewed, {}, field.column, draft.input)[field.column]!;
    if (!equalCell(verified.expected, draft.expected) || !equalCell(verified.value, draft.value) ||
        verified.contextId !== draft.contextId || verified.table !== draft.table || verified.rowId !== draft.rowId) {
      throw new Error('Two-page draft belongs to another original/physical owner.');
    }
    if (verified.error) throw new Error(verified.error);
    if (!equalCell(verified.expected, verified.value)) {
      const { contextId, table, rowId, column, expected, value } = verified;
      edits.push({ contextId, table, rowId, column, expected, value });
    }
  }
  return { original: reviewed, edits };
}

export class TwoPageParentSession extends SIVIParentScopedWriteSession<TwoPageParentColumn,
  TwoPageParentWrite, TwoPageParentDrafts, TwoPageParentInput> {
  constructor(owner: SIVIParentOwner, form: TwoPageParentForm, port: SIVIParentWritePort<TwoPageParentWrite>,
    notify: () => void) {
    const fields = twoPageParentFields(form);
    super(owner, conservativePort(port, 'Two-page parent'), notify, {
      label: 'Two-page parent',
      accepts: (value): value is TwoPageParentColumn => fields.some(field => field.column === value),
      unavailable: 'Two-page additional-field drafts require owned exact-variant originals.',
      drafts: { empty: () => ({}), stage, errors: twoPageParentErrors, dirty: twoPageParentDirty },
      request: (original, drafts) => twoPageParentRequest(original, drafts, form),
      count: request => request.edits.length,
      decodeOriginal: (wire, active) => twoPageParentOriginalFromWire(wire, active, form),
    });
  }
}
