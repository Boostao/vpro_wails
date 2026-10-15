import source from '../../resources/fs1333-chars-sivi-layout.json';
import type { ProjectMetadataCell, SIVIParentCellEdit,
  SIVICHARSParentEntryPolicy as PolicyWire } from '../bindings/github.com/boostao/vpro-wails';
import { validateSourceParentPhysical, type SIVIParentOriginal } from './siviParentEditor';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { parseSourceParentEntryValue, sourceParentEntryNull,
  type SourceParentEntryInput, type SourceParentEntryParsed } from './sourceParentEntryDraft';

export interface SIVICHARSParentEntryPolicy extends Omit<PolicyWire, 'owner' | 'kind'> {
  owner: 'Env' | 'Admin';
  kind: 'text' | 'date' | 'integer' | 'single' | 'double' | 'boolean' | 'option';
}
export interface SIVICHARSParentEntryDraft extends SIVIParentCellEdit {
  input: SourceParentEntryInput; error: string | null;
}
export type SIVICHARSParentEntryDrafts = Record<string, SIVICHARSParentEntryDraft>;
export const siviCHARSParentFields = source.forms[0].fields.filter(field => field.binding);
const editable = siviCHARSParentFields.filter(field =>
  !['PlotNumber', 'ProjectID', 'PlotType'].includes(field.binding!));
const kinds = ['text', 'date', 'integer', 'single', 'double', 'boolean', 'option'];
const columnFor = (binding: string) => binding === 'siteNotes' ? 'SiteNotes' : binding;
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
export const equalSIVICHARSParentCell = (left: ProjectMetadataCell, right: ProjectMetadataCell) =>
  equalCell(left, right) && (left.storage !== 'real' || Object.is(left.real, right.real));

export function validateSIVICHARSParentOriginal(original: SIVIParentOriginal): SIVIParentOriginal {
  if (source.root !== 'frmSIVIsite-CHARS' || source.forms[0].name !== source.root ||
      original?.Form !== source.root || siviCHARSParentFields.length !== 71 || editable.length !== 68) {
    throw new Error('CHARS entry requires its exact original71-binding form, not normal SIVI.');
  }
  validateSourceParentPhysical(original, [
    ...siviCHARSParentFields.map(field => ({ Binding: field.binding!, ControlID: field.controlId, Implicit: false })),
    { Binding: 'SpeciesListComplete', ControlID: '', Implicit: true },
  ]);
  return structuredClone(original);
}

export function siviCHARSParentPoliciesFromWire(wire: unknown, original: SIVIParentOriginal): SIVICHARSParentEntryPolicy[] {
  validateSIVICHARSParentOriginal(original);
  if (!Array.isArray(wire) || wire.length !== editable.length) {
    throw new Error('CHARS entry requires its complete68-policy source cohort.');
  }
  const policies: SIVICHARSParentEntryPolicy[] = [];
  for (const [index, value] of wire.entries()) {
    const sourceField = editable[index], binding = original.Bindings.find(row => row.ControlID === sourceField.controlId);
    if (!record(value) || value.column !== columnFor(sourceField.binding!) || value.binding !== sourceField.binding ||
        value.controlId !== sourceField.controlId || !['Env', 'Admin'].includes(String(value.owner)) ||
        !binding || binding.Table !== `${original.Project}_${value.owner}` ||
        !kinds.includes(String(value.kind)) || !Number.isInteger(value.maximum) || (value.maximum as number) < 0 ||
        typeof value.allowEmpty !== 'boolean' || value.allowEmpty !== (value.column === 'SnowCoverregime') ||
        typeof value.writable !== 'boolean' || value.writable !== (value.column !== 'SubZone') ||
        typeof value.diagnostic !== 'string' || !wellFormedUTF16(value.diagnostic) || value.diagnostic.includes('\0') ||
        (!value.writable || value.column === 'SV_AhorizonType' || value.kind === 'boolean') && !value.diagnostic) {
      throw new Error('CHARS entry policy identity, physical owner or source availability changed.');
    }
    const integer = value.kind === 'integer';
    const lower = integer ? value.column === 'StartDate' ? 1900 : -32768 : null;
    const upper = integer ? value.column === 'StartDate' ? 2500 : 32767 : null;
    if (value.lower !== lower || value.upper !== upper) {
      throw new Error('CHARS entry policy requires exact source INTEGER bounds.');
    }
    policies.push({ column: value.column as string, binding: value.binding as string, controlId: value.controlId as string,
      owner: value.owner as SIVICHARSParentEntryPolicy['owner'], kind: value.kind as SIVICHARSParentEntryPolicy['kind'],
      maximum: value.maximum as number, allowEmpty: value.allowEmpty, lower, upper,
      writable: value.writable, diagnostic: value.diagnostic });
  }
  for (const [column, kind, maximum] of [
    ['RootZoneParticleSize', 'text', 6], ['SV_WaterTableCM', 'single', 0],
    ['Latitude', 'double', 0], ['Longitude', 'double', 0], ['StartDate', 'integer', 0],
    ['SnowCoverregime', 'text', 1], ['SV_AhorizonType', 'text', 5],
  ] as const) {
    const policy = policies.find(field => field.column === column);
    if (policy?.kind !== kind || policy.maximum !== maximum) throw new Error(`CHARS ${column} source domain changed.`);
  }
  return policies;
}

function parseNumber(policy: SIVICHARSParentEntryPolicy, raw: string): SourceParentEntryParsed {
  const empty = sourceParentEntryNull();
  if (raw === '') return { value: empty, error: null };
  if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(raw)) {
    return { value: empty, error: `${policy.column} requires a literal finite number or explicit NULL.` };
  }
  let number = Number(raw);
  if (policy.kind === 'single') number = Math.fround(number);
  if (!Number.isFinite(number) || policy.kind !== 'integer' && Object.is(number, -0)) {
    return { value: empty, error: `${policy.column} exceeds its finite source ${policy.kind} domain or cannot preserve new negative zero.` };
  }
  if (policy.kind === 'integer') {
    if (!Number.isInteger(number) || policy.lower === null || policy.upper === null ||
        number < policy.lower || number > policy.upper) {
      return { value: empty, error: `${policy.column} requires a source integer from ${policy.lower} to ${policy.upper} or NULL.` };
    }
    return { value: { ...empty, storage: 'integer', integer: String(number) }, error: null };
  }
  return { value: { ...empty, storage: 'real', real: number }, error: null };
}

export function parseSIVICHARSParentValue(policy: SIVICHARSParentEntryPolicy,
  expected: ProjectMetadataCell, input: SourceParentEntryInput): SourceParentEntryParsed {
  const parsed = parseSourceParentEntryValue(policy, expected, input, {
    label: 'CHARS SIVI', kinds, allowsEmpty: field => field.allowEmpty, number: parseNumber,
  });
  if (!equalSIVICHARSParentCell(expected, parsed.value) && !policy.writable) {
    return { value: parsed.value, error: policy.diagnostic };
  }
  return parsed;
}

export function stageSIVICHARSParentEntry(original: SIVIParentOriginal, policies: readonly SIVICHARSParentEntryPolicy[],
  drafts: SIVICHARSParentEntryDrafts, column: string, input: SourceParentEntryInput): SIVICHARSParentEntryDrafts {
  const policy = policies.find(field => field.column === column);
  const binding = original.Bindings.find(row => row.Binding === policy?.binding);
  if (!policy || !binding || binding.Implicit || binding.Table !== `${original.Project}_${policy.owner}`) {
    throw new Error('CHARS entry excludes readonly, implicit and foreign source fields.');
  }
  const row = original.Rows[0][policy.owner], expected = row.cells[binding.Column];
  const parsed = parseSIVICHARSParentValue(policy, expected, input);
  return { ...drafts, [column]: { contextId: original.ContextID, table: binding.Table, rowId: row.rowId, column,
    expected: structuredClone(expected), value: parsed.value, error: parsed.error, input: structuredClone(input) } };
}

export function siviCHARSParentEntryRequest(original: SIVIParentOriginal, policies: readonly SIVICHARSParentEntryPolicy[],
  drafts: SIVICHARSParentEntryDrafts) {
  const reviewed = siviCHARSParentPoliciesFromWire(policies, original);
  const edits: SIVIParentCellEdit[] = [];
  for (const [column, draft] of Object.entries(drafts)) {
    if (draft.error) throw new Error(draft.error);
    const prepared = stageSIVICHARSParentEntry(original, reviewed, {}, column, draft.input)[column];
    if (prepared.error) throw new Error(prepared.error);
    if (draft.column !== column || draft.contextId !== prepared.contextId || draft.table !== prepared.table ||
        draft.rowId !== prepared.rowId || !equalSIVICHARSParentCell(draft.expected, prepared.expected) ||
        !equalSIVICHARSParentCell(draft.value, prepared.value)) {
      throw new Error('CHARS entry draft identity or prepared source value changed.');
    }
    if (!equalSIVICHARSParentCell(prepared.expected, prepared.value)) {
      const { input: _input, error: _error, ...edit } = prepared;
      edits.push(structuredClone(edit));
    }
  }
  return { original: structuredClone(original), edits };
}
