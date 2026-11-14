import { createReferenceCodeEditor } from './referenceCodeEditor';
import type { QualityView } from './qualityEditor';

const reference = createReferenceCodeEditor({
  keys: ['soilDrainage'] as const,
  labels: { soilDrainage: 'Soil drainage' },
  list: () => 'SoilDrainage' as const,
  limits: { SoilDrainage: 5 }
});
export const drainageGroups = reference.groups;
export function drainageError(value: string | null, original: string | null, view: QualityView): string | null {
  const physical = reference.valueError('soilDrainage', value, original);
  if (physical) return physical;
  if (value === null || value === original) return null;
  if (view.busy) return 'Wait for Soil drainage choices to finish refreshing.';
  if (!view.ready) return view.error ?? 'Soil drainage choices are unavailable; the new code cannot be saved.';
  return view.choices.some(row => reference.selectable(row, 'SoilDrainage') && row.code === value) ? null
    : 'Soil drainage requires an exact canonical Item. Select the full code; case and prefixes are not silently rewritten.';
}
