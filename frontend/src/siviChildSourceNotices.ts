import { aCoverSourceNotice } from './heightEditor';
import { completeCell } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import type { SIVICoverEdit } from './siviCoverEditor';
import type { SIVIHeightEdit, SIVIProjection, SIVIRow } from './siviHeightEditor';

export interface SIVIChildSourceNotice {
  rowId: string;
  form: string;
  event: 'BeforeUpdate' | 'AfterUpdate';
  focus: 'Species';
  species: string;
  message: string;
}

const aColumns = ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB'] as const;

export function siviChildSourceNotices(review: readonly SIVIProjection[], edits: readonly (SIVICoverEdit | SIVIHeightEdit)[]): SIVIChildSourceNotice[] {
  const effective = new Map<string, Map<string, ProjectMetadataCell>>();
  const changedForms = new Map<string, { rowId: string; form: string; group: SIVIProjection; row: SIVIRow }>();
  const assignments = new Set<string>();
  for (const edit of edits) {
    if (!['SubVegA-SIVI_BC', 'SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI'].includes(edit.form)) {
      throw new Error('SIVI notice source form is unavailable.');
    }
    const groups = review.filter(group => group.Form === edit.form);
    const rows = groups.length === 1 ? groups[0].Rows.filter(row => row.rowId === edit.rowId) : [];
    const index = groups.length === 1 ? groups[0].Columns.indexOf(edit.column) : -1;
    const key = JSON.stringify([edit.rowId, edit.column]);
    if (rows.length !== 1 || index < 0 || groups[0].Columns.filter(column => column === edit.column).length !== 1 ||
        assignments.has(key) || !completeCell(rows[0].cells[index]) ||
        !completeCell(edit.value) || !completeCell(edit.expected) || !equalCell(rows[0].cells[index], edit.expected)) {
      throw new Error('SIVI notice source row, assignment or original is missing, repeated or changed; reload explicitly.');
    }
    assignments.add(key);
    if (equalCell(edit.expected, edit.value)) continue;
    const values = effective.get(edit.rowId) ?? new Map<string, ProjectMetadataCell>();
    values.set(edit.column, edit.value);
    effective.set(edit.rowId, values);
    changedForms.set(JSON.stringify([edit.rowId, edit.form]), { rowId: edit.rowId, form: edit.form, group: groups[0], row: rows[0] });
  }
  const notices: SIVIChildSourceNotice[] = [];
  for (const { rowId, form, group, row } of changedForms.values()) {
    const after = (column: string): ProjectMetadataCell => {
      const index = group.Columns.indexOf(column);
      if (index < 0 || group.Columns.filter(name => name === column).length !== 1 || !completeCell(row.cells[index])) {
        throw new Error('SIVI notice source projection is incomplete.');
      }
      return effective.get(rowId)?.get(column) ?? row.cells[index];
    };
    const speciesCell = after('Species');
    const species = speciesCell.storage === 'null' ? 'NULL' : metadataCellText(speciesCell);
    if (form === 'SubVegA-SIVI_BC' || form === 'SubVegA-SIVI') {
      // Only NULL presence is projected; these sentinel values are never assignments.
      const values = Object.fromEntries(aColumns.map(column => [column[0].toLowerCase() + column.slice(1),
        after(column).storage === 'null' ? null : 0]));
      const message = aCoverSourceNotice(form, `physical ${rowId}`, values);
      if (message) notices.push({ rowId, form, event: 'AfterUpdate', focus: 'Species', species, message });
    } else if (form === 'SubVegC-SIVI' && after('Cover6').storage === 'null') {
      notices.push({ rowId, form, event: 'BeforeUpdate', focus: 'Species', species,
        message: `Vegetation physical row ${rowId}: Cover6 is NULL. Saving removes this row from the C source view; it does not delete the vegetation record.` });
    }
  }
  return notices;
}
