import type { ProjectMetadataCell, ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import source from '../../resources/fs1333-sivi-layout.json';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';

const policies = {
  SV_StandHeight: { owner: 'Env', domain: 'scalars', kind: 'number', maximum: 0 },
  SV_AhorizonDepth: { owner: 'Env', domain: 'scalars', kind: 'number', maximum: 0 },
  SV_GleyingMottlingCM: { owner: 'Env', domain: 'scalars', kind: 'number', maximum: 0 },
  SV_PercentCoarseFrags: { owner: 'Env', domain: 'scalars', kind: 'number', maximum: 0 },
  SV_SoilDepth: { owner: 'Env', domain: 'scalars', kind: 'number', maximum: 0 },
  StrataCoverTotal: { owner: 'Admin', domain: 'scalars', kind: 'number', maximum: 0 },
  SV_FloodPlain: { owner: 'Env', domain: 'scalars', kind: 'boolean', maximum: 0 },
  SV_StandAgeEstMeas: { owner: 'Env', domain: 'options', kind: 'option', maximum: 1 },
  SV_StandHeightEstMeas: { owner: 'Env', domain: 'options', kind: 'option', maximum: 1 },
  SV_PolygonNumber: { owner: 'Env', domain: 'text', kind: 'text', maximum: 25 },
  SV_CanopyComposition: { owner: 'Env', domain: 'text', kind: 'text', maximum: 50 },
  SnowCoverregime: { owner: 'Env', domain: 'categorical', kind: 'categorical', maximum: 1 },
  SV_RootZoneTexture: { owner: 'Env', domain: 'categorical', kind: 'categorical', maximum: 100 },
  SV_AhorizonType: { owner: 'Env', domain: 'categorical', kind: 'categorical', maximum: 5 },
  PlotType: { owner: 'Admin', domain: 'actions', kind: 'action', maximum: 10 },
  SpeciesListComplete: { owner: 'Env', domain: 'actions', kind: 'action', maximum: 0 },
} as const;
export type SIVIParentColumn = keyof typeof policies;
type Domain = typeof policies[SIVIParentColumn]['domain'];
export interface SIVIParentBinding {
  ControlID: string;
  Binding: string;
  Table: string;
  Column: number;
  Implicit: boolean;
}
interface ParentRow { rowId: string; cells: ProjectMetadataCell[] }
export interface SIVIParentOriginal {
  ContextID: string;
  Project: string;
  Plot: string;
  Form: string;
  Query: string;
  Membership: string;
  EnvTable: string;
  AdminTable: string;
  EnvColumns: ProjectMetadataColumn[];
  AdminColumns: ProjectMetadataColumn[];
  Bindings: SIVIParentBinding[];
  Rows: { Env: ParentRow; Admin: ParentRow }[];
}
export type SIVIParentInput =
  | { kind: 'original' }
  | { kind: 'clear' }
  | { kind: 'empty' }
  | { kind: 'text'; raw: string }
  | { kind: 'boolean'; value: boolean }
  | { kind: 'option'; option: number | null };
export interface SIVIParentDraft {
  contextId: string;
  project: string;
  plot: string;
  column: SIVIParentColumn;
  controlId: string;
  table: string;
  rowId: string;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
  input: SIVIParentInput;
  error: string | null;
}
export type SIVIParentDrafts = Partial<Record<SIVIParentColumn, SIVIParentDraft>>;
export interface SIVIParentEdit {
  contextId: string;
  table: string;
  rowId: string;
  column: SIVIParentColumn;
  controlId: string;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
}
export interface SIVIParentActionEdit {
  contextId: string;
  table: string;
  rowId: string;
  controlId: string;
  expected: ProjectMetadataCell;
  option: number | null;
}
export interface SIVIParentProposal {
  original: SIVIParentOriginal;
  scalars: SIVIParentEdit[];
  options: SIVIParentEdit[];
  text: SIVIParentEdit[];
  categorical: SIVIParentEdit[];
  actions: SIVIParentActionEdit[];
}

const parent = source.forms[0];
const direct = parent.fields.filter(field => field.binding);
const columns = Object.keys(policies) as SIVIParentColumn[];
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
const validIdentity = (value: string) => typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0');
const isColumn = (value: string): value is SIVIParentColumn => Object.hasOwn(policies, value);

function controlId(column: SIVIParentColumn): string {
  const action = column === 'PlotType' ? 'optPlotType' : column === 'SpeciesListComplete' ? 'optSpeciesListComplete' : null;
  const fields = parent.fields.filter(field => action ? field.controlName === action : field.binding === column);
  if (fields.length !== 1 || !fields[0].readOnly || fields[0].implementation !== 'unmapped' ||
      !fields[0].controlId || action && (fields[0].type !== 'OptionGroup' || fields[0].binding)) {
    throw new Error('SIVI parent requires its exact original source control.');
  }
  return fields[0].controlId;
}

export function validateSIVIParentOriginal(original: SIVIParentOriginal): SIVIParentOriginal {
  if (!original || !validIdentity(original.ContextID) || !validIdentity(original.Project) || !validIdentity(original.Plot) ||
      original.Form !== 'frmSIVIsite' || original.Query !== 'USysEnv' ||
      original.Membership !== 'literal-binary-inner-pairs' || original.EnvTable !== `${original.Project}_Env` ||
      original.AdminTable !== `${original.Project}_Admin` || parent.name !== original.Form ||
      parent.recordSource !== original.Query || direct.length !== 77 ||
      !Array.isArray(original.Rows) || original.Rows.length !== 1 ||
      !Array.isArray(original.Bindings) || original.Bindings.length !== 78) {
    throw new Error('SIVI parent requires one complete owned literal physical pair; this does not authorize a write.');
  }
  const schemas = [original.EnvColumns, original.AdminColumns];
  const rows = [original.Rows[0]?.Env, original.Rows[0]?.Admin];
  schemas.forEach((schema, i) => {
    if (!Array.isArray(schema) || !schema.length || Array.from(schema).some(column => !column || !validIdentity(column.name) ||
        typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType) ||
        ['rowid', '_rowid_', 'oid'].includes(column.name.toLowerCase())) ||
        new Set(schema.map(column => column.name.toLowerCase())).size !== schema.length ||
        !rows[i] || !exactSigned64(rows[i].rowId) || !Array.isArray(rows[i].cells) ||
        rows[i].cells.length !== schema.length || !Array.from(rows[i].cells).every(completeCell)) {
      throw new Error('SIVI parent requires complete unshadowed schemas and exact raw physical rows.');
    }
    const join = schema.findIndex(column => column.name === (i === 0 ? 'PlotNumber' : 'Plot'));
    if (join < 0 || rows[i].cells[join].storage !== 'text' || rows[i].cells[join].text !== original.Plot) {
      throw new Error('SIVI parent literal join identity changed; no alias normalization is available.');
    }
  });
  Array.from(original.Bindings).forEach((binding, index) => {
    const name = index < direct.length ? direct[index].binding : 'SpeciesListComplete';
    const matches = schemas.flatMap((schema, owner) => schema.flatMap((column, columnIndex) =>
      column.name.toLowerCase() === name?.toLowerCase() ? [{ owner, columnIndex }] : []));
    if (!binding || matches.length !== 1 || binding.Binding !== name ||
        binding.ControlID !== (index < direct.length ? direct[index].controlId : '') ||
        binding.Implicit !== (index === direct.length) || binding.Column !== matches[0].columnIndex ||
        binding.Table !== (matches[0].owner === 0 ? original.EnvTable : original.AdminTable)) {
      throw new Error('SIVI parent source binding identity or physical ownership changed.');
    }
    if (isColumn(binding.Binding) && binding.Table !== `${original.Project}_${policies[binding.Binding].owner}`) {
      throw new Error('SIVI parent target is outside its original physical owner.');
    }
  });
  columns.forEach(controlId);
  return structuredClone(original);
}

function target(original: SIVIParentOriginal, column: SIVIParentColumn) {
  const binding = original.Bindings.find(binding => binding.Binding === column);
  if (!binding) throw new Error('SIVI parent source target is unavailable.');
  const row = original.Rows[0][policies[column].owner];
  return { controlId: controlId(column), table: binding.Table, rowId: row.rowId, expected: row.cells[binding.Column] };
}

function parseInput(column: SIVIParentColumn, expected: ProjectMetadataCell, input: SIVIParentInput) {
  const policy = policies[column];
  let value = nullCell();
  let error: string | null = null;
  if (input.kind === 'original') value = structuredClone(expected);
  else if (input.kind === 'clear') {
    if (column === 'PlotType') error = 'PlotType NULL action is unverified; select a source option1..5.';
  } else if (input.kind === 'empty') {
    if (policy.kind !== 'categorical') error = 'Explicit empty TEXT is available only for categorical storage.';
    else value = { ...value, storage: 'text', text: '' };
  } else if (input.kind === 'boolean') {
    if (policy.kind !== 'boolean' || typeof input.value !== 'boolean') error = 'Select an explicit floodplain BOOLEAN or NULL.';
    else value = { ...value, storage: 'integer', integer: input.value ? '-1' : '0' };
  } else if (input.kind === 'option') {
    if (policy.kind !== 'option' && policy.kind !== 'action') error = 'This field is not a source option target.';
    else if (column === 'PlotType') {
      if (!Number.isInteger(input.option) || input.option === null || input.option < 1 || input.option > 5) {
        error = 'PlotType requires a selected source option1..5; NULL/other actions are unverified.';
      } else value = { ...value, storage: 'text', text: ['Ground', 'Visual', 'Note', 'FS882', 'Other'][input.option - 1] };
    } else if (input.option !== null && input.option !== 1 && input.option !== 2) {
      error = 'Select source option1/2 or explicit NULL.';
    } else if (input.option !== null) {
      value = column === 'SpeciesListComplete'
        ? { ...value, storage: 'integer', integer: input.option === 1 ? '-1' : '0' }
        : { ...value, storage: 'text', text: String(input.option) };
    }
  } else if (input.kind === 'text') {
    if (policy.kind !== 'number' && policy.kind !== 'text' && policy.kind !== 'categorical') {
      error = 'This field requires explicit source option/BOOLEAN intent, not text.';
    } else if (typeof input.raw !== 'string' || !wellFormedUTF16(input.raw) || input.raw.includes('\0')) {
      error = `${column} contains malformed Unicode or NUL; literal input was not repaired.`;
    } else if (input.raw !== '' && expected.storage !== 'null' && input.raw === metadataCellText(expected)) {
      value = structuredClone(expected);
    } else if (policy.kind === 'number') {
      const parsed = finiteSingleValue(column, input.raw);
      error = parsed.error;
      if (parsed.value !== null) value = { ...value, storage: 'real', real: parsed.value };
    } else if (input.raw !== '') {
      value = { ...value, storage: 'text', text: input.raw };
      if (input.raw.length > policy.maximum) error = `${column} exceeds${policy.maximum} UTF-16 units; literal input was not truncated.`;
    }
  } else error = 'SIVI parent input intent is unavailable.';
  return { value, error };
}

export function stageSIVIParent(original: SIVIParentOriginal, drafts: SIVIParentDrafts,
  column: SIVIParentColumn, input: SIVIParentInput): SIVIParentDrafts {
  if (!isColumn(column)) throw new Error('SIVI parent target is outside the sixteen-field draft scope.');
  const reviewed = validateSIVIParentOriginal(original);
  const identity = target(reviewed, column);
  const previous = drafts[column];
  if (previous && (previous.contextId !== reviewed.ContextID || previous.project !== reviewed.Project ||
      previous.plot !== reviewed.Plot || previous.column !== column || previous.controlId !== identity.controlId ||
      previous.table !== identity.table || previous.rowId !== identity.rowId || !equalCell(previous.expected, identity.expected))) {
    throw new Error('SIVI parent original changed; retain drafts and Undo explicitly.');
  }
  return { ...structuredClone(drafts), [column]: { contextId: reviewed.ContextID, project: reviewed.Project,
    plot: reviewed.Plot, column, ...structuredClone(identity),
    ...parseInput(column, identity.expected, input), input: structuredClone(input) } };
}

export function siviParentErrors(drafts: SIVIParentDrafts): string[] {
  return Object.values(drafts).flatMap(draft => draft?.error ? [draft.error] : []);
}
export function siviParentDirty(drafts: SIVIParentDrafts): boolean {
  return Object.values(drafts).some(draft => draft && (draft.error !== null || !equalCell(draft.expected, draft.value)));
}
export function siviParentProposal(original: SIVIParentOriginal, drafts: SIVIParentDrafts): SIVIParentProposal {
  const reviewed = validateSIVIParentOriginal(original);
  const result: SIVIParentProposal = { original: reviewed, scalars: [], options: [], text: [], categorical: [], actions: [] };
  for (const [name, draft] of Object.entries(drafts)) {
    if (!isColumn(name) || !draft || draft.column !== name || !completeCell(draft.expected) || !completeCell(draft.value)) {
      throw new Error('SIVI parent draft target or raw cell is incomplete.');
    }
    const identity = target(reviewed, name);
    const parsed = parseInput(name, identity.expected, draft.input);
    if (draft.contextId !== reviewed.ContextID || draft.project !== reviewed.Project || draft.plot !== reviewed.Plot ||
        draft.controlId !== identity.controlId || draft.table !== identity.table || draft.rowId !== identity.rowId ||
        !equalCell(draft.expected, identity.expected) || !equalCell(draft.value, parsed.value) || draft.error !== parsed.error) {
      throw new Error('SIVI parent original or explicit draft intent changed; retain drafts and Undo explicitly.');
    }
    if (parsed.error) throw new Error(parsed.error);
    if (equalCell(draft.expected, draft.value)) continue;
    const common = { contextId: reviewed.ContextID, table: identity.table, rowId: identity.rowId,
      controlId: identity.controlId, expected: structuredClone(draft.expected) };
    const domain: Domain = policies[name].domain;
    if (domain === 'actions') {
      if (draft.input.kind !== 'option' && draft.input.kind !== 'clear') throw new Error('SIVI source action intent was not selected.');
      result.actions.push({ ...common, option: draft.input.kind === 'clear' ? null : draft.input.option });
    } else result[domain].push({ ...common, column: name, value: structuredClone(draft.value) });
  }
  return result;
}
