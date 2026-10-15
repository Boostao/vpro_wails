import type { ProjectMetadataTable } from '../bindings/github.com/boostao/vpro-wails';
import { lifeformShape } from './lifeformSummary';
import { exactSigned64 } from './projectMetadataRestore';
import { completeMetadataCellFromWire } from './siviParentTransport';

const schema = [
  ['ListName', 'TEXT'], ['ListFilter', 'TEXT'], ['ItemOrder', 'REAL'], ['Item', 'TEXT'],
  ['ItemDescription', 'TEXT'], ['FieldUsedIn', 'TEXT'], ['ValidateLoops', 'TEXT'],
  ['Validate', 'BOOLEAN'], ['Note', 'TEXT'], ['Flag', 'BOOLEAN'],
] as const;

export function validateQualityMinimumReference(value: unknown, listName: 'PlotQualitySite' | 'BEC_Use'): ProjectMetadataTable {
  if (!lifeformShape(value, ['columns', 'rows']) || !Array.isArray(value.columns) ||
      value.columns.length !== schema.length || Array.from(value.columns).some((column, index) =>
        !lifeformShape(column, ['name', 'declaredType']) || column.name !== schema[index][0] ||
        column.declaredType !== schema[index][1]) || !Array.isArray(value.rows)) {
    throw new Error(`Owned ${listName} suggestion schema is incomplete; no copied catalogue applied.`);
  }
  const seen = new Set<string>();
  const rows = Array.from(value.rows).map(row => {
    if (!lifeformShape(row, ['rowId', 'cells']) || typeof row.rowId !== 'string' ||
        !exactSigned64(row.rowId) || seen.has(row.rowId) ||
        !Array.isArray(row.cells) || row.cells.length !== schema.length) {
      throw new Error(`Owned ${listName} suggestions require distinct physical identities and complete typed metadata.`);
    }
    seen.add(row.rowId);
    const cells = Array.from(row.cells, (cell: unknown) => {
      if (!completeMetadataCellFromWire(cell)) {
        throw new Error(`Owned ${listName} suggestions require complete typed metadata.`);
      }
      return { storage: cell.storage, text: cell.text, integer: cell.integer, real: cell.real, blobHex: cell.blobHex };
    });
    if (cells[0].storage !== 'text' || cells[0].text !== listName) {
      throw new Error(`Quality minimum suggestions belong to another source list, not ${listName}.`);
    }
    return { rowId: row.rowId, cells };
  });
  return { columns: schema.map(([name, declaredType]) => ({ name, declaredType })), rows };
}
