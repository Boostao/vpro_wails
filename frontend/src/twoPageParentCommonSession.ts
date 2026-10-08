import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { parseSIVIParentInput, type SIVIParentInput, type SIVIParentOriginal } from './siviParentEditor';
import { twoPageParentSource, validateTwoPageParentOriginal, type TwoPageParentForm } from './twoPageParentSession';
import { sourceParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort } from './siviParentWriteSession';
import { conservativePort } from './siviParentSharedSession';
import { ordinaryTextError } from './ordinaryEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { equalCell, metadataCellText } from './projectMetadataEditor';

const policies = {
  SV_StandHeight: { owner: 'Env', kind: 'single' },
  SV_AhorizonDepth: { owner: 'Env', kind: 'single' },
  SV_GleyingMottlingCM: { owner: 'Env', kind: 'single' },
  SV_PercentCoarseFrags: { owner: 'Env', kind: 'single' },
  SV_SoilDepth: { owner: 'Env', kind: 'single' },
  StrataCoverTotal: { owner: 'Admin', kind: 'single' },
  SV_FloodPlain: { owner: 'Env', kind: 'boolean' },
  SV_StandAgeEstMeas: { owner: 'Env', kind: 'option' },
  SV_StandHeightEstMeas: { owner: 'Env', kind: 'option' },
  SV_PolygonNumber: { owner: 'Env', kind: 'text', maximum: 25 },
  SV_CanopyComposition: { owner: 'Env', kind: 'text', maximum: 50 },
  SV_RootZoneTexture: { owner: 'Env', kind: 'categorical', maximum: 100 },
  SV_AhorizonType: { owner: 'Env', kind: 'categorical', maximum: 5 },
  PlotType: { owner: 'Admin', kind: 'plot-type', maximum: 10 },
} as const;
export type TwoPageParentCommonColumn = keyof typeof policies;
export type TwoPageParentCommonInput = SIVIParentInput;
export interface TwoPageParentCommonEdit {
  contextId: string; table: string; rowId: string; column: TwoPageParentCommonColumn;
  expected: ProjectMetadataCell; value: ProjectMetadataCell;
}
export interface TwoPageParentCommonDraft extends TwoPageParentCommonEdit {
  input: TwoPageParentCommonInput; error: string | null;
}
export type TwoPageParentCommonDrafts = Partial<Record<TwoPageParentCommonColumn, TwoPageParentCommonDraft>>;
export interface TwoPageParentCommonWrite { original: SIVIParentOriginal; edits: TwoPageParentCommonEdit[] }
export type TwoPageParentCommonSessionCache = Map<string, Map<TwoPageParentForm, TwoPageParentCommonSession>>;

export function twoPageParentCommonFields(form: TwoPageParentForm) {
  const controls = twoPageParentSource(form).fields;
  const seen = new Set<TwoPageParentCommonColumn>();
  const fields = controls.flatMap(control => {
    if (!control.binding || !Object.hasOwn(policies, control.binding)) return [];
    const column = control.binding as TwoPageParentCommonColumn;
    if (seen.has(column)) return [];
    seen.add(column);
    const policy = policies[column];
    const labelName = column === 'PlotType' ? 'Label261'
      : column === 'SV_StandAgeEstMeas' ? form.endsWith('-CHARS') ? 'Label240' : 'Label291'
      : column === 'SV_StandHeightEstMeas' ? 'Label541'
      : column === 'SV_StandHeight' && !form.endsWith('-CHARS') ? 'Label292' : null;
    const labelled = labelName ? controls.find(field => field.controlName === labelName && field.type === 'Label') : control;
    const label = labelled && 'caption' in labelled ? labelled.caption : null;
    if (!label) throw new Error(`Two-page common field ${column} requires its source label.`);
    const options = controls.flatMap(option => {
      const optionValue = 'OptionValue' in option.properties ? option.properties.OptionValue : undefined;
      if (!optionValue) return [];
      let parentId = option.parentId;
      const visited = new Set<string>();
      while (parentId && parentId !== control.controlId) {
        if (visited.has(parentId)) throw new Error('Two-page source option ancestry is cyclic.');
        visited.add(parentId);
        parentId = controls.find(field => field.controlId === parentId)?.parentId;
      }
      if (parentId !== control.controlId) return [];
      const value = Number(optionValue.value);
      const caption = 'caption' in option ? option.caption : null;
      if ((value !== 1 && value !== 2) || !caption) throw new Error('Two-page Est/Meas option source changed.');
      return [{ value, label: caption }];
    });
    if (policy.kind === 'option' && (options.length !== 2 || new Set(options.map(option => option.value)).size !== 2)) {
      throw new Error('Two-page Est/Meas requires both exact source options.');
    }
    return [{ column, label, policy, options, controlId: control.controlId,
      sourceInstances: controls.filter(field => field.binding === column).map(field => field.controlId),
      page: 'pageId' in control ? control.pageId : null }];
  });
  if (fields.length !== (form.endsWith('-CHARS') ? 14 : 13)) throw new Error('Two-page common-field source scope changed.');
  return fields;
}

export function validateTwoPageParentCommonOriginal(original: SIVIParentOriginal, form: TwoPageParentForm) {
  const reviewed = validateTwoPageParentOriginal(original, form);
  for (const field of twoPageParentCommonFields(form)) {
    for (const id of field.sourceInstances) {
      const binding = reviewed.Bindings.find(binding => binding.ControlID === id);
      if (!binding || binding.Table !== `${reviewed.Project}_${field.policy.owner}`) {
        throw new Error('Two-page common-field physical ownership changed.');
      }
    }
  }
  return reviewed;
}

function target(original: SIVIParentOriginal, column: TwoPageParentCommonColumn) {
  const field = twoPageParentCommonFields(original.Form as TwoPageParentForm).find(field => field.column === column);
  const binding = original.Bindings.find(binding => binding.ControlID === field?.controlId);
  if (!field || !binding || binding.Implicit || binding.Binding !== column || binding.Table !== `${original.Project}_${field.policy.owner}`) {
    throw new Error('Two-page common-field source/physical ownership changed.');
  }
  const row = original.Rows[0][field.policy.owner];
  return { field, binding, row, expected: structuredClone(row.cells[binding.Column]) };
}
export function twoPageParentCommonCell(original: SIVIParentOriginal, column: TwoPageParentCommonColumn) {
  return target(original, column).expected;
}
export function twoPageParentCommonErrors(drafts: TwoPageParentCommonDrafts) {
  return Object.values(drafts).flatMap(draft => draft?.error ? [draft.error] : []);
}
export function twoPageParentCommonDirty(drafts: TwoPageParentCommonDrafts) {
  return Object.values(drafts).some(draft => draft && (draft.error !== null || !equalCell(draft.expected, draft.value)));
}
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

function stage(original: SIVIParentOriginal, drafts: TwoPageParentCommonDrafts,
  column: TwoPageParentCommonColumn, input: TwoPageParentCommonInput): TwoPageParentCommonDrafts {
  const { field, binding, row, expected } = target(original, column);
  let value = structuredClone(expected), error: string | null = null;
  if (column !== 'PlotType') {
    ({ value, error } = parseSIVIParentInput(column, expected, input));
    if (input.kind === 'text' && !error && expected.storage !== 'null' && input.raw === metadataCellText(expected)) {
      value = structuredClone(expected);
    }
  }
  else if (input.kind === 'clear') value = nullCell();
  else if (input.kind === 'text') {
    if (typeof input.raw !== 'string' || !wellFormedUTF16(input.raw) || input.raw.includes('\0')) {
      error = 'PlotType contains malformed Unicode or NUL; literal input was not repaired.';
    } else if (!(expected.storage !== 'null' && input.raw === metadataCellText(expected))) {
      error = ordinaryTextError({ label: field.label, kind: 'text', maximum: 10 }, input.raw, null);
      value = { ...nullCell(), storage: 'text', text: input.raw };
    }
  } else if (input.kind !== 'original') error = 'Two-page PlotType requires literal bound text, not a SIVI action option.';
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  return { ...drafts, [column]: { contextId: original.ContextID, table: binding.Table, rowId: row.rowId,
    column, expected, value, input: structuredClone(input), error } };
}

export function twoPageParentCommonRequest(original: SIVIParentOriginal, drafts: TwoPageParentCommonDrafts,
  form: TwoPageParentForm): TwoPageParentCommonWrite {
  const reviewed = validateTwoPageParentCommonOriginal(original, form), edits: TwoPageParentCommonEdit[] = [];
  for (const [name, draft] of Object.entries(drafts)) {
    const field = twoPageParentCommonFields(form).find(field => field.column === name);
    if (!field || !draft || draft.column !== field.column) throw new Error('Two-page common writes exclude additional fields and actions.');
    const verified = stage(reviewed, {}, field.column, draft.input)[field.column]!;
    if (!equalCell(verified.expected, draft.expected) || !equalCell(verified.value, draft.value) ||
        verified.contextId !== draft.contextId || verified.table !== draft.table || verified.rowId !== draft.rowId) {
      throw new Error('Two-page common draft belongs to another original/physical owner.');
    }
    if (verified.error) throw new Error(verified.error);
    if (!equalCell(verified.expected, verified.value)) {
      const { contextId, table, rowId, column, expected, value } = verified;
      edits.push({ contextId, table, rowId, column, expected, value });
    }
  }
  return { original: reviewed, edits };
}

export class TwoPageParentCommonSession extends SIVIParentScopedWriteSession<TwoPageParentCommonColumn,
  TwoPageParentCommonWrite, TwoPageParentCommonDrafts, TwoPageParentCommonInput> {
  constructor(owner: SIVIParentOwner, form: TwoPageParentForm, port: SIVIParentWritePort<TwoPageParentCommonWrite>,
    notify: () => void) {
    const fields = twoPageParentCommonFields(form);
    super(owner, conservativePort(port, 'Two-page common fields'), notify, {
      label: 'Two-page common fields',
      accepts: (value): value is TwoPageParentCommonColumn => fields.some(field => field.column === value),
      unavailable: 'Two-page common-field drafts require owned exact-variant originals.',
      drafts: { empty: () => ({}), stage, errors: twoPageParentCommonErrors, dirty: twoPageParentCommonDirty },
      request: (original, drafts) => twoPageParentCommonRequest(original, drafts, form),
      count: request => request.edits.length,
      decodeOriginal: (wire, active) => sourceParentOriginalFromWire(wire, active,
        original => validateTwoPageParentCommonOriginal(original, form)),
    });
  }
}
