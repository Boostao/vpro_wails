import type { ProjectMetadataCell, ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { validateSIVIParentOriginal, type SIVIParentBinding, type SIVIParentOriginal } from './siviParentEditor';
import { SIVIParentSession } from './siviParentSession';

export interface SIVIParentOwner {
  contextId: string;
  project: string;
  plot: string;
}
export interface SIVIParentActionDisplay {
  controlId: string;
  expected: ProjectMetadataCell;
  option: number | null;
  diagnostic: string | null;
}
type Row = SIVIParentOriginal['Rows'][number]['Env'];
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const nullableString = (value: unknown) => value === null || typeof value === 'string';

function cell(value: unknown): value is ProjectMetadataCell {
  return record(value) && typeof value.storage === 'string' && nullableString(value.text) &&
    nullableString(value.integer) && (value.real === null || typeof value.real === 'number') && nullableString(value.blobHex);
}
function schema(value: unknown): value is ProjectMetadataColumn[] {
  return Array.isArray(value) && Array.from(value).every(column => record(column) &&
    typeof column.name === 'string' && typeof column.declaredType === 'string');
}
function row(value: unknown): value is Row {
  return record(value) && typeof value.rowId === 'string' && Array.isArray(value.cells) && Array.from(value.cells).every(cell);
}
function binding(value: unknown): value is SIVIParentBinding {
  return record(value) && typeof value.ControlID === 'string' && typeof value.Binding === 'string' &&
    typeof value.Table === 'string' && typeof value.Column === 'number' && typeof value.Implicit === 'boolean';
}
export function siviMetadataTableFromWire(wire: unknown): { columns: ProjectMetadataColumn[]; rows: Row[] } {
  if (!record(wire) || !schema(wire.columns) || !Array.isArray(wire.rows)) {
    throw new Error('Complete physical SIVI metadata rows were not returned.');
  }
  const rows: Row[] = [], columns = wire.columns;
  for (const item of Array.from(wire.rows)) {
    if (!row(item) || !exactSigned64(item.rowId) || item.cells.length !== columns.length ||
      Array.from(item.cells).some(value => !completeCell(value))) {
      throw new Error('SIVI metadata row identities or original storage are incomplete.');
    }
    rows.push(item);
  }
  if (new Set(rows.map(item => item.rowId)).size !== rows.length) {
    throw new Error('SIVI metadata physical row identities are duplicated.');
  }
  return structuredClone({ columns, rows });
}

function originalShape(value: unknown): value is SIVIParentOriginal {
  return record(value) &&
    ['ContextID', 'Project', 'Plot', 'Form', 'Query', 'Membership', 'EnvTable', 'AdminTable']
      .every(key => typeof value[key] === 'string') &&
    schema(value.EnvColumns) && schema(value.AdminColumns) &&
    Array.isArray(value.Bindings) && Array.from(value.Bindings).every(binding) &&
    Array.isArray(value.Rows) && Array.from(value.Rows).every(pair => record(pair) && row(pair.Env) && row(pair.Admin));
}

export function siviParentOriginalFromWire(wire: unknown, owner: SIVIParentOwner): SIVIParentOriginal {
  if (!originalShape(wire)) throw new Error('Complete raw SIVI parent original transport was not returned.');
  if (wire.ContextID !== owner.contextId || wire.Project !== owner.project || wire.Plot !== owner.plot) {
    throw new Error('SIVI parent response belongs to another active context/project/plot; no drafts were initialized.');
  }
  return validateSIVIParentOriginal(wire);
}

export function siviParentSessionFromWire(editorId: string, wire: unknown, owner: SIVIParentOwner): SIVIParentSession {
  return new SIVIParentSession(editorId, siviParentOriginalFromWire(wire, owner));
}

// Display initialization never stages an option or repairs historical storage.
export function siviParentActionDisplay(original: SIVIParentOriginal): SIVIParentActionDisplay[] {
  const review = validateSIVIParentOriginal(original);
  return ['PlotType', 'SpeciesListComplete'].map(name => {
    const source = review.Bindings.find(binding => binding.Binding === name);
    if (!source) throw new Error('SIVI parent action source was not returned.');
    const expected = review.Rows[0][source.Table === review.EnvTable ? 'Env' : 'Admin'].cells[source.Column];
    let option: number | null = null;
    let diagnostic: string | null = null;
    if (expected.storage !== 'null') {
      if (name === 'PlotType' && expected.storage === 'text') {
        const index = ['Ground', 'Visual', 'Note', 'FS882', 'Other'].indexOf(expected.text ?? '');
        if (index >= 0) option = index + 1;
      } else if (name === 'SpeciesListComplete' && expected.storage === 'integer') {
        if (expected.integer === '-1') option = 1;
        else if (expected.integer === '0') option = 2;
      }
      if (option === null) diagnostic = `${name} original storage has no verified literal display mapping; retain it unchanged or select an explicit supported action.`;
    }
    return { controlId: `form:frmSIVIsite/${name === 'PlotType' ? 'optPlotType' : 'optSpeciesListComplete'}`,
      expected: structuredClone(expected), option, diagnostic };
  });
}
