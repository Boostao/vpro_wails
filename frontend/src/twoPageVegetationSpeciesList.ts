import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import type { VegetationReferences, VegetationReferenceTable } from './twoPageVegetationReferences';
import type { TwoPageVegetationDraft } from './twoPageVegetationEditor';
import { equalVegetationCell, vegetationNullCell, vegetationSourceCell,
  type TwoPageVegetationEdit, type TwoPageVegetationOriginal, type TwoPageVegetationOwner } from './twoPageVegetationWire';
import { wellFormedUTF16 } from './qualityEditor';

export interface VegetationSpeciesListOption {
  role: 'master' | 'personal';
  rowId: string;
  code: string;
  cells: ProjectMetadataCell[];
}

export function vegetationSpeciesListOptions(references: VegetationReferences, owner: TwoPageVegetationOwner): VegetationSpeciesListOption[] {
  if (references.contextId !== owner.contextId || references.project !== owner.project ||
    references.plot !== owner.plot || references.form !== owner.form ||
    owner.form !== 'SubVegAXL_BC' && owner.form !== 'SubVegAXL') {
    throw new Error('Species list selection requires the exact A/CHARS physical reference owner.');
  }
  const options: VegetationSpeciesListOption[] = [];
  const tables: { role: 'master' | 'personal'; table: VegetationReferenceTable }[] =
    [{ role: 'master', table: references.master }, { role: 'personal', table: references.personal }];
  for (const { role, table } of tables) {
    for (const row of table.rows) {
      const [code, , lifeform, , codetype] = row.cells;
      if (code.storage !== 'text' || code.text === null || code.text === '' || code.text.length > 8 ||
        !wellFormedUTF16(code.text) || code.text.includes('\0') ||
        lifeform.storage !== 'integer' || lifeform.integer === null ||
        !['1', '2', '3', '4'].includes(lifeform.integer) ||
        codetype.storage !== 'text' || codetype.text === null || !['u', 'U', 'x', 'X'].includes(codetype.text)) continue;
      options.push({ role, rowId: row.rowId, code: code.text, cells: row.cells.map(cell => ({ ...cell })) });
    }
  }
  return options;
}

export function stageVegetationSpeciesList(original: TwoPageVegetationOriginal, rowId: string,
  raw: string, nullValue: boolean, references: VegetationReferences): TwoPageVegetationDraft {
  const expected = vegetationSourceCell(original, rowId, 'Species');
  let value = vegetationNullCell(), error: string | null = null;
  if (nullValue) error = 'Species list selection requires non-NULL literal text.';
  else if (!wellFormedUTF16(raw) || raw.includes('\0')) error = 'Species contains incomplete Unicode or NUL; raw input was not repaired.';
  else if (expected.storage === 'text' && expected.text === raw) value = { ...expected };
  else {
    value = { ...value, storage: 'text', text: raw };
    if (!vegetationSpeciesListOptions(references, original).some(option => option.code === raw)) {
      error = 'Select an exact qualified existing A/CHARS Species list code; manual, alias and NotInList routes remain unavailable.';
    }
  }
  return { contextId: original.contextId, form: original.form, rowId, column: 'Species',
    expected: { ...expected }, value, raw, nullValue, error };
}

const commonCovers = ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB'];

export function vegetationSpeciesListHasCommonCover(original: TwoPageVegetationOriginal, rowId: string): boolean {
  return commonCovers.some(column => vegetationSourceCell(original, rowId, column).storage !== 'null');
}

export function requireVegetationSpeciesListCovers(original: TwoPageVegetationOriginal, edits: TwoPageVegetationEdit[]): void {
  for (const edit of edits) {
    if (edit.column !== 'Species' || equalVegetationCell(edit.expected, edit.value)) continue;
    const originalCover = vegetationSpeciesListHasCommonCover(original, edit.rowId);
    const plannedCover = commonCovers.some(column => (edits.find(candidate => candidate.rowId === edit.rowId &&
      candidate.column === column)?.value ?? vegetationSourceCell(original, edit.rowId, column)).storage !== 'null');
    if (!originalCover || !plannedCover) {
      throw new Error('Species selection with all seven common covers NULL requires the unavailable source warning/advance route.');
    }
  }
}
