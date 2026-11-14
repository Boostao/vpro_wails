import { fieldListCell, type FieldListCell } from './summaryVegetationFieldList';

export type SummaryEnvironmentalValue =
  | { kind: 'empty'; cell: null }
  | { kind: 'cell'; cell: FieldListCell };

function reject(): never {
  throw new Error('Numeric environmental value or argument is unavailable or inconsistent; no repairs applied.');
}

function exact(value: unknown): value is Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const keys = Reflect.ownKeys(value);
  return keys.length === 2 && keys.every(key => key === 'kind' || key === 'cell');
}

export function numericEnvironmentalArgument(functionName: string, value: unknown): string {
  const argument = fieldListCell(value);
  switch (functionName) {
    case 'SVSubstrate':
      if (argument.storage === 'text' &&
          ['DecWood', 'BedRock', 'Rocks', 'MineralSoil', 'OrganicMatter', 'Water'].includes(argument.text ?? ''))
        return argument.text!;
      return reject();
    case 'SVStrataCover':
      if (argument.storage === 'text' && ['Tree', 'Shrub', 'Herb', 'Moss'].includes(argument.text ?? ''))
        return argument.text!;
      return reject();
    case 'SVSeepage': case 'SVRootingDepth': case 'SVStandAge': case 'SVElevation': case 'SVSlope':
      if (argument.storage === 'null') return '';
      return reject();
    default: return reject();
  }
}

export function numericEnvironmentalValue(functionName: string, argument: unknown, value: unknown): SummaryEnvironmentalValue {
  numericEnvironmentalArgument(functionName, argument);
  if (!exact(value)) reject();
  if (value.kind === 'empty') {
    if (value.cell !== null || functionName === 'SVStandAge' || functionName === 'SVElevation') reject();
    return { kind: 'empty', cell: null };
  }
  if (value.kind !== 'cell') reject();
  const cell = fieldListCell(value.cell);
  if (cell.storage !== 'text') reject();
  return { kind: 'cell', cell };
}
