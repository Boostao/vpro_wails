import { createReferenceCodeEditor } from './referenceCodeEditor';
import { workingUnitDefinitions, type WorkingUnitView } from './workingUnitEditor';

const reference = createReferenceCodeEditor({
  keys: ['becSiteUnit'] as const,
  labels: { becSiteUnit: 'BEC Master' },
  list: () => 'Master' as const,
  limits: { Master: 100 }
});
export const masterBECAcknowledged = reference.acknowledged;
export const rememberMasterBECAcknowledgement = reference.rememberAcknowledgement;
export function masterBECWarnings(value: string | null, view: WorkingUnitView): string[] {
  if (value === null || view.busy) return [];
  if (!view.masterReady) return [view.masterError ?? 'BEC Master choices are unavailable; this code has not been checked.'];
  return workingUnitDefinitions(view.master, value).length ? []
    : [`BEC Master "${value}" is not in the Master catalogue. No code or partner field will be replaced automatically.`];
}
export function masterBECValidation(value: string | null, original: string | null, allowed: boolean,
  view: WorkingUnitView, acknowledged: boolean): string | null {
  if (value === original) return null;
  if (!allowed) return 'BEC Master editing is restricted to the source-authorized user; use Working Unit instead.';
  const physical = reference.valueError('becSiteUnit', value, original);
  if (physical) return physical;
  if (view.busy) return 'Wait for BEC Master choices to finish updating.';
  return masterBECWarnings(value, view).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched BEC Master code before saving.' : null;
}
