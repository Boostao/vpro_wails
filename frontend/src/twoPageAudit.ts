import { siviMetadataTableFromWire } from './siviParentTransport';
import { metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';

export interface TwoPageAuditOwner {
  contextId: string;
  project: string;
  plot: string;
  form: 'FS882-8x6XL' | 'FS882-8x6XL-CHARS';
}
export const sourceAuditFields = [
  ['EditWhen', 'Date/Time'], ['PlotNumber', 'Plot'], ['EditField', 'Field'],
  ['BeforeEdit', 'Value Before Edit'], ['Table', 'Table'], ['AfterEdit', 'Value After Edit'],
  ['Flag', 'Flag'], ['User', 'User'], ['Restore', 'Restore'], ['ID', 'Child ID'], ['Project', 'Project'],
] as const;
export interface TwoPageAuditReview extends TwoPageAuditOwner {
  records: ReturnType<typeof siviMetadataTableFromWire>;
}
export function twoPageAuditFromWire(value: unknown, owner: TwoPageAuditOwner): TwoPageAuditReview {
  if (!owner.contextId || !owner.project || !owner.plot ||
      !['FS882-8x6XL', 'FS882-8x6XL-CHARS'].includes(owner.form) ||
      ![owner.contextId, owner.project, owner.plot].every(wellFormedUTF16) ||
      !value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Source audit requires an immutable valid literal owner and complete response.');
  }
  const wire = value as Record<string, unknown>;
  if (wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
      wire.form !== owner.form || wire.recordSource !== 'UsysAuditTrail' || wire.table !== `${owner.project}_Audit`) {
    throw new Error('Source audit belongs to another context, project, literal plot or source form.');
  }
  const records = siviMetadataTableFromWire(wire.records);
  for (const [name] of sourceAuditFields) {
    if (records.columns.filter(column => column.name === name).length !== 1) {
      throw new Error(`Source audit requires the exact ${name} binding; no column repaired.`);
    }
  }
  for (const [name, expected] of [['Project', owner.project], ['PlotNumber', owner.plot]]) {
    const index = records.columns.findIndex(column => column.name === name);
    if (records.rows.some(row => row.cells[index].storage !== 'text' || row.cells[index].text !== expected)) {
      throw new Error('Source audit physical rows have foreign or nonliteral ownership.');
    }
  }
  return { ...owner, records };
}
export function twoPageAuditLabel(review: TwoPageAuditReview, row: TwoPageAuditReview['records']['rows'][number], column: string): string {
  const index = review.records.columns.findIndex(item => item.name === column);
  if (index < 0 || !row.cells[index]) throw new Error(`Source audit ${column} original is unavailable.`);
  const cell = row.cells[index];
  if (cell.storage === 'null') return '(NULL)';
  if (cell.storage === 'text' && cell.text === '') return '(empty text)';
  if (column === 'Restore' || column === 'Flag') {
    if (cell.storage === 'integer' && cell.integer === '-1') return 'Yes (-1)';
    if (cell.storage === 'integer' && cell.integer === '0') return 'No (0)';
  }
  return metadataCellText(cell);
}
