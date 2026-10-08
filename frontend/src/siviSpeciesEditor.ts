import type { ProjectMetadataCell, VegetationSpeciesAlias, VegetationSpeciesOption } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { resolveSpeciesDecision, type SpeciesChoices, type SpeciesDecision, type SpeciesDecisionKind } from './vegetationSpeciesEditor';
import { siviChildPhysicalRows, type SIVIProjection } from './siviHeightEditor';

export interface SIVISpeciesOwner { contextId: string; project: string; plot: string }
export interface SIVISpeciesDefinitionTable {
  columns: { name: string; declaredType: string }[];
  rows: { rowId: string; cells: ProjectMetadataCell[] }[];
}
export interface SIVISpeciesReferences extends SIVISpeciesOwner { master: SIVISpeciesDefinitionTable; personal: SIVISpeciesDefinitionTable }
export interface SIVISpeciesDraft {
  group: number;
  expected: ProjectMetadataCell;
  raw: string;
  error: string | null;
  decision?: SpeciesDecision;
}
export type SIVISpeciesDrafts = Record<string, SIVISpeciesDraft>;
export interface SIVISpeciesEdit {
  rowId: string; form: string; expected: ProjectMetadataCell; value: string;
  decision?: SpeciesDecisionKind; entered?: string; selected?: string;
}
const definitionColumns = ['Code', 'ScientificName', 'Lifeform', 'EnglishName', 'Codetype'];
const lifeforms = [['1', '2', '3', '4'], ['5', '6', '7', '8', '12'], ['1', '2', '9', '10', '11']];
const lookupKey = (value: string) => value.replace(/[a-z]/g, character => character.toUpperCase());
const storedText = (cell: ProjectMetadataCell) => cell.storage === 'text' ? cell.text : null;
const originalRaw = (cell: ProjectMetadataCell) => cell.storage === 'text' ? cell.text ?? '' : '';

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function definitionTable(value: unknown, names: readonly string[]): SIVISpeciesDefinitionTable {
  const diagnostic = 'SIVI Species requires complete ordered typed definitions and distinct physical reference identities.';
  if (!record(value) || !Array.isArray(value.columns) || value.columns.length !== names.length || !Array.isArray(value.rows)) {
    throw new Error(diagnostic);
  }
  const columns = value.columns.map((column, index) => {
    if (!record(column) || typeof column.name !== 'string' || column.name.toLowerCase() !== names[index].toLowerCase() ||
        typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) throw new Error(diagnostic);
    return { name: column.name, declaredType: column.declaredType };
  });
  const rows = value.rows.map(row => {
    if (!record(row) || typeof row.rowId !== 'string' || !exactSigned64(row.rowId) ||
        !Array.isArray(row.cells) || row.cells.length !== names.length) throw new Error(diagnostic);
    const cells = row.cells.map((cell, index) => {
      if (!completeCell(cell)) throw new Error(diagnostic);
      if (cell.storage !== 'null' && (index === 2
        ? cell.storage !== 'integer' || BigInt(cell.integer ?? '') < -32768n || BigInt(cell.integer ?? '') > 32767n
        : cell.storage !== 'text')) throw new Error(diagnostic);
      return structuredClone(cell);
    });
    return { rowId: row.rowId, cells };
  });
  if (new Set(rows.map(row => row.rowId)).size !== rows.length) throw new Error(diagnostic);
  return { columns, rows };
}

export function validateSIVISpeciesReferences(references: unknown, owner: SIVISpeciesOwner): SIVISpeciesReferences {
  if (!record(references) || references.contextId !== owner.contextId || references.project !== owner.project || references.plot !== owner.plot) {
    throw new Error('SIVI Species definitions belong to a different context/project/plot.');
  }
  return { ...owner, master: definitionTable(references.master, [...definitionColumns, 'OldCode']),
    personal: definitionTable(references.personal, definitionColumns) };
}

export const siviSpeciesRows = (review: readonly SIVIProjection[]) => siviChildPhysicalRows(review, 'SIVI Species');

function original(review: readonly SIVIProjection[], rowId: string, group?: number) {
  const physical = siviSpeciesRows(review).find(source => source.row.rowId === rowId);
  const context = physical?.contexts.find(context => group === undefined || context.index === group);
  if (!exactSigned64(rowId) || !physical || !context) throw new Error('SIVI Species physical row/source context is unavailable; reload explicitly.');
  return { expected: physical.row.cells[2], group: context.index, form: context.group.Form };
}

function codeError(raw: string): string | null {
  if (!wellFormedUTF16(raw)) return 'Species contains incomplete Unicode; raw entry was not repaired.';
  if (raw.length > 8) return 'Species exceeds 8 UTF-16 units; raw entry was not truncated.';
  if (raw === '') return 'A new Species code must be nonempty; NULL/empty was not assigned.';
  return null;
}

export function siviSpeciesListed(references: SIVISpeciesReferences, group: number): VegetationSpeciesOption[] {
  if (!Number.isInteger(group) || group < 0 || group > 2) throw new Error('SIVI Species source group is unavailable.');
  const seen = new Set<string>();
  return [...references.master.rows, ...references.personal.rows].flatMap(row => {
    const code = storedText(row.cells[0]);
    const type = storedText(row.cells[4]);
    if (code === null || codeError(code) || type === null || !/^[ux]$/i.test(type) ||
        row.cells[2].storage !== 'integer' || !lifeforms[group].includes(row.cells[2].integer ?? '')) return [];
    const tuple = JSON.stringify(row.cells.slice(0, 5));
    if (seen.has(tuple)) return [];
    seen.add(tuple);
    return [{ code, scientificName: storedText(row.cells[1]), lifeform: Number(row.cells[2].integer),
      englishName: storedText(row.cells[3]), codeType: type }];
  });
}

export function siviSpeciesChoices(references: SIVISpeciesReferences, form: string, entered: string): SpeciesChoices {
  const option = (cells: ProjectMetadataCell[]): VegetationSpeciesOption => ({
    code: storedText(cells[0]), scientificName: storedText(cells[1]),
    lifeform: cells[2].storage === 'integer' && /^-?\d{1,5}$/.test(cells[2].integer ?? '') ? Number(cells[2].integer) : null,
    englishName: storedText(cells[3]), codeType: storedText(cells[4]),
  });
  const aliases: VegetationSpeciesAlias[] = references.master.rows.flatMap(row => {
    const oldCode = storedText(row.cells[5]);
    return oldCode !== null && lookupKey(oldCode) === lookupKey(entered) ? [{ ...option(row.cells), oldCode }] : [];
  });
  const users = references.personal.rows.filter(row => {
    const code = storedText(row.cells[0]);
    return code !== null && lookupKey(code) === lookupKey(entered);
  }).map(row => option(row.cells));
  return { form, entered, aliases, users };
}

function draftError(draft: SIVISpeciesDraft, references: SIVISpeciesReferences | null, form: string): string | null {
  if (!completeCell(draft.expected) || draft.expected.storage !== 'null' && draft.expected.storage !== 'text') {
    return 'Historical non-text Species is read-only; no value was coerced.';
  }
  if (draft.raw === originalRaw(draft.expected) && !draft.decision) return null;
  const invalid = codeError(draft.raw);
  if (invalid) return invalid;
  if (!references) return 'SIVI Species definitions are unavailable; reload before saving.';
  if (draft.decision) {
    const resolved = resolveSpeciesDecision(siviSpeciesChoices(references, form, draft.decision.entered), draft.decision.kind, draft.decision.selected);
    return resolved.value === draft.raw ? null : 'SIVI Species value differs from its explicit source decision.';
  }
  return siviSpeciesListed(references, draft.group).some(option => option.code === draft.raw) ? null
    : 'Select an exact source-list code or review old-code/personal choices before saving.';
}

export function stageSIVISpecies(review: readonly SIVIProjection[], references: SIVISpeciesReferences | null,
  drafts: SIVISpeciesDrafts, rowId: string, raw: string, group?: number): SIVISpeciesDrafts {
  const previous = drafts[rowId];
  const source = original(review, rowId, group ?? previous?.group);
  if (previous && !equalCell(previous.expected, source.expected)) throw new Error('SIVI Species original changed; raw draft retained until explicit reload.');
  const draft: SIVISpeciesDraft = { group: source.group, expected: structuredClone(previous?.expected ?? source.expected), raw, error: null };
  draft.error = draftError(draft, references, source.form);
  return { ...drafts, [rowId]: draft };
}

export function contextSIVISpecies(review: readonly SIVIProjection[], references: SIVISpeciesReferences | null,
  drafts: SIVISpeciesDrafts, rowId: string, group: number): SIVISpeciesDrafts {
  const source = original(review, rowId, group);
  const previous = drafts[rowId];
  if (previous && !equalCell(previous.expected, source.expected)) throw new Error('SIVI Species original changed; retained source context requires reload.');
  const draft: SIVISpeciesDraft = { ...previous, group, expected: structuredClone(previous?.expected ?? source.expected),
    raw: previous?.raw ?? originalRaw(source.expected), error: null };
  draft.error = draftError(draft, references, source.form);
  return { ...drafts, [rowId]: draft };
}

export function chooseSIVISpecies(review: readonly SIVIProjection[], references: SIVISpeciesReferences,
  drafts: SIVISpeciesDrafts, rowId: string, kind: SpeciesDecisionKind, selected?: string): SIVISpeciesDrafts {
  const previous = drafts[rowId];
  if (!previous || previous.decision) throw new Error('Review the retained Species entry before choosing a source decision.');
  const source = original(review, rowId, previous.group);
  if (!equalCell(previous.expected, source.expected)) throw new Error('SIVI Species original changed; decision was not applied.');
  const resolved = resolveSpeciesDecision(siviSpeciesChoices(references, source.form, previous.raw), kind, selected);
  return { ...drafts, [rowId]: { ...previous, raw: resolved.value, error: null, decision: resolved.decision } };
}

export const siviSpeciesErrors = (drafts: SIVISpeciesDrafts): string[] =>
  Object.values(drafts).flatMap(draft => draft.error ? [draft.error] : []);
export const siviSpeciesDirty = (drafts: SIVISpeciesDrafts): boolean =>
  Object.values(drafts).some(draft => draft.error !== null || draft.raw !== originalRaw(draft.expected));

export function siviSpeciesEdits(review: readonly SIVIProjection[], references: SIVISpeciesReferences,
  drafts: SIVISpeciesDrafts): SIVISpeciesEdit[] {
  const errors = siviSpeciesErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([rowId, draft]) => {
    const source = original(review, rowId, draft.group);
    if (!equalCell(source.expected, draft.expected)) throw new Error('SIVI Species original changed; reload explicitly.');
    const invalid = draftError(draft, references, source.form);
    if (invalid) throw new Error(invalid);
    if (draft.raw === originalRaw(draft.expected)) return [];
    return [{ rowId, form: source.form, expected: structuredClone(draft.expected), value: draft.raw,
      ...(draft.decision ? { decision: draft.decision.kind, entered: draft.decision.entered,
        ...(draft.decision.selected === undefined ? {} : { selected: draft.decision.selected }) } : {}) }];
  });
}

export function siviSpeciesRequestJSON(review: readonly SIVIProjection[], references: SIVISpeciesReferences,
  owner: SIVISpeciesOwner, drafts: SIVISpeciesDrafts): string {
  siviSpeciesRows(review);
  const checked = validateSIVISpeciesReferences(references, owner);
  return JSON.stringify({ original: structuredClone(review), references: checked, edits: siviSpeciesEdits(review, checked, drafts) });
}
