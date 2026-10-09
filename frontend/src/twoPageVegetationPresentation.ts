import { metadataCellText } from './projectMetadataEditor';
import { twoPageVegetationEditableID, twoPageVegetationFields } from './twoPageVegetationEditor';
import { vegetationColumnIndex, vegetationIdentifierKey, vegetationSourceCell, type TwoPageVegetationForm, type TwoPageVegetationOriginal } from './twoPageVegetationWire';

interface VegetationGroup { label: string; fields: [string, string][] }
export function twoPageVegetationGroups(form: TwoPageVegetationForm): VegetationGroup[] {
  const groups: VegetationGroup[] = form === 'SubVegCXL'
    ? [{ label: 'Herb', fields: [['Cover6', 'C']] }]
    : form === 'SubVegDXL'
      ? [{ label: 'Moss/Lichen', fields: [['Cover7', 'D'], ['Cover8', 'Dr/Dw'], ['Cover9', 'Ep']] }]
      : [
        { label: 'Tree', fields: [['Cover1', 'A1'], ['Cover2', 'A2'], ['Cover3', 'A3'], ['TotalA', 'A']] },
        { label: 'Shrubs', fields: [['Cover4', 'B1'], ['Cover5', 'B2'],
          ...(form === 'SubVegAXL' ? [['Cover5a', 'B3'], ['Cover5b', 'B4'], ['Cover5c', 'B5']] satisfies [string, string][] : []),
          ['TotalB', 'B']] },
      ];
  return [...groups, { label: 'Collection/verification', fields: [['Collected', '? (Collected)']] }];
}
export function twoPageVegetationStoredText(original: TwoPageVegetationOriginal, rowId: string, column: string): string {
  const cell = vegetationSourceCell(original, rowId, column);
  return cell.storage === 'null' ? 'NULL' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell);
}
export function twoPageVegetationHasColumn(original: TwoPageVegetationOriginal, column: string): boolean {
  const key = vegetationIdentifierKey(column);
  return original.records.columns.some(field => vegetationIdentifierKey(field.name) === key);
}
export function twoPageVegetationRowDiagnostic(original: TwoPageVegetationOriginal, rowId: string): string | null {
  try {
    twoPageVegetationEditableID(original, rowId);
    for (const field of [...twoPageVegetationFields(original), 'Species']) vegetationColumnIndex(original, field);
    return null;
  } catch (cause) {
    return `Read-only physical row ${rowId}: ${String(cause)}`;
  }
}
