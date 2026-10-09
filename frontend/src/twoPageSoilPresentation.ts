import { metadataCellText } from './projectMetadataEditor';
import { twoPageSoilEditableID, twoPageSoilKind, type TwoPageSoilOriginal } from './twoPageSoilEditor';
import { soilFields } from './soilChildEditor';

export const twoPageSoilGroups = {
  Humus: [
    { label: 'Horizon and depth', fields: [['Horizon', 'Horizon'], ['UpperDepth', 'Depth Up'], ['LowerDepth', 'Depth Low']] },
    { label: 'Structure', fields: [['HumusStructureDegree', 'Structure AB'], ['HumusStructureKind', 'Fabric'], ['vonPost', 'Von Post']] },
    { label: 'Organisms and roots', fields: [['MycelAbundance', 'Mycel AB'], ['FecalAbundance', 'Fecal AB'], ['RootsAbundance', 'Roots AB'], ['RootsSize', 'Roots Size']] },
    { label: 'pH and comments', fields: [['HumusFormpH', 'pH'], ['Comment', 'Comments (consistency, character, fauna, etc.)']] },
  ],
  Mineral: [
    { label: 'Horizon and depth', fields: [['Horizon', 'Horizon'], ['UpperDepth', 'Depth Up'], ['LowerDepth', 'Depth Low'], ['PitDepthLimit', '+ (Pit depth limit)']] },
    { label: 'Texture and aspect', fields: [['Texture', 'Texture'], ['Colour', 'Colour'], ['ASP', 'ASP']] },
    { label: '% Coarse fragments', fields: [['PercentCoarseFragsGravel', 'G'], ['PercentCoarseFragsCobbles', 'C'], ['PercentCoarseFragsStones', 'S'], ['PercentCoarseFragsTotal', 'Tot.'], ['PercentCoarseFragsShape', 'Shape']] },
    { label: 'Roots and structure', fields: [['RootsAbundance', 'Roots AB'], ['RootsSize', 'Roots Size'], ['MineralStructureClass', 'Structure Class'], ['MineralStructureKind', 'Structure Kind']] },
    { label: 'pH and comments', fields: [['MineralFormpH', 'pH'], ['Comments', 'Comments (mottles, clay films, effervesc., etc.)']] },
  ],
} as const;

export function twoPageSoilCell(original: TwoPageSoilOriginal, rowId: string, column: string) {
  const row = original.records.rows.find(row => row.rowId === rowId);
  const index = original.records.columns.findIndex(field => field.name === column);
  if (!row || index < 0) throw new Error(`Soil physical row ${rowId} / source column ${column} is unavailable.`);
  return row.cells[index];
}

export function twoPageSoilStoredText(original: TwoPageSoilOriginal, rowId: string, column: string) {
  const cell = twoPageSoilCell(original, rowId, column);
  return cell.storage === 'null' ? 'NULL' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell);
}

export function twoPageSoilRowDiagnostic(original: TwoPageSoilOriginal, rowId: string): string | null {
  try { twoPageSoilEditableID(original, rowId); }
  catch (cause) { return `Read-only physical row: ${String(cause)}`; }
  const missing = soilFields[twoPageSoilKind(original.form)].filter(field =>
    !original.records.columns.some(column => column.name === field.column));
  return missing.length ? `Read-only physical row: source columns unavailable: ${missing.map(field => field.column).join(', ')}.` : null;
}

export function twoPageSoilDepthOrder(original: TwoPageSoilOriginal) {
  const rows = original.records.rows.slice();
  const index = original.records.columns.findIndex(column => column.name === 'UpperDepth');
  const fallback = (reason: string) => ({ rows, diagnostic: `${reason} Physical reader order is shown without coercion.` });
  if (index < 0) return fallback('Depth order unavailable: UpperDepth is missing.');
  const values = new Map<string, bigint | number | null>();
  for (const row of rows) {
    const cell = row.cells[index];
    if (cell.storage === 'null') values.set(row.rowId, null);
    else if (cell.storage === 'integer' && cell.integer !== null) values.set(row.rowId, BigInt(cell.integer));
    else if (cell.storage === 'real' && cell.real !== null && Number.isFinite(cell.real)) values.set(row.rowId, cell.real);
    else return fallback(`Depth order unavailable: physical row ${row.rowId} has historical ${cell.storage} UpperDepth.`);
  }
  const direction = original.form === 'SoilHumusXL' ? -1 : 1;
  rows.sort((left, right) => {
    const a = values.get(left.rowId), b = values.get(right.rowId);
    if (a === undefined || b === undefined) throw new Error('Soil depth order lost its validated physical row.');
    const comparison = a === null ? b === null ? 0 : -1 : b === null ? 1 : a < b ? -1 : a > b ? 1 : 0;
    return direction * comparison;
  });
  return { rows, diagnostic: null };
}
