import type { FS882Header } from '../bindings/github.com/boostao/vpro-wails';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';

export const ordinaryFields = [
  { key: 'airPhotoNum', column: 'AirPhotoNum', label: 'Air photo number', scope: 'site', kind: 'text', maximum: 20 },
  { key: 'enteredBy', column: 'EnteredBy', label: 'Entered by', scope: 'site', kind: 'text', maximum: 50 },
  { key: 'strataCoverTree', column: 'StrataCoverTree', label: 'Tree stratum cover', scope: 'veg', kind: 'single' },
  { key: 'strataCoverShrub', column: 'StrataCoverShrub', label: 'Shrub stratum cover', scope: 'veg', kind: 'single' },
  { key: 'strataCoverHerb', column: 'StrataCoverHerb', label: 'Herb stratum cover', scope: 'veg', kind: 'single' },
  { key: 'strataCoverMoss', column: 'StrataCoverMoss', label: 'Moss stratum cover', scope: 'veg', kind: 'single' },
  { key: 'vegSurveyor', column: 'VegSurveyor', label: 'Vegetation surveyor', scope: 'veg', kind: 'text', maximum: 30 },
  { key: 'vegNotes', column: 'VegNotes', label: 'Vegetation notes', scope: 'veg', kind: 'memo' },
  { key: 'soilSurveyor', column: 'SoilSurveyor', label: 'Soil surveyor', scope: 'soils', kind: 'text', maximum: 30 },
  { key: 'humusThickness', column: 'HumusThickness', label: 'Humus thickness', scope: 'soils', kind: 'single' },
  { key: 'rootingDepth', column: 'RootingDepth', label: 'Rooting depth', scope: 'soils', kind: 'integer' },
  { key: 'rootRestrictingDepth', column: 'RootRestrictingDepth', label: 'Root-restricting depth', scope: 'soils', kind: 'integer' },
  { key: 'seepageDepth', column: 'SeepageDepth', label: 'Seepage depth', scope: 'soils', kind: 'integer' },
  { key: 'soilNotes', column: 'SoilNotes', label: 'Soil notes', scope: 'soils', kind: 'memo' }
] as const;
export type OrdinaryField = typeof ordinaryFields[number];
export type OrdinaryScope = OrdinaryField['scope'];
export type OrdinaryKey = OrdinaryField['key'];
export type OrdinaryTextField = Extract<OrdinaryField, { kind: 'text' | 'memo' }>;
export type OrdinaryNumberField = Extract<OrdinaryField, { kind: 'integer' | 'single' }>;
export type OrdinaryValues = Pick<FS882Header, OrdinaryKey>;
export interface OrdinaryCell { raw: string; value: number | null; error: string | null }
export type OrdinaryDrafts = Partial<Record<OrdinaryNumberField['key'], OrdinaryCell>>;

export function ordinaryField(column: string | undefined): OrdinaryField | undefined {
  return ordinaryFields.find(field => field.column === column);
}
export function ordinaryTextError(field: OrdinaryTextField, value: string | null, original: string | null): string | null {
  if (value === null || value === original) return null;
  if (value === '') return `Clear ${field.label} to NULL instead of an empty string.`;
  if (!wellFormedUTF16(value)) return `${field.label} contains an incomplete Unicode character.`;
  if (field.kind === 'text' && value.length > field.maximum) {
    return `${field.label} must be at most ${field.maximum} UTF-16 characters; the entry has not been truncated.`;
  }
  return null;
}
export function ordinaryNumberValue(field: OrdinaryNumberField, raw: string, original: number | null): OrdinaryCell {
  const parsed = finiteSingleValue(field.label, raw, original);
  if (field.kind === 'integer' && parsed.error === null && parsed.value !== null &&
    (!Number.isInteger(parsed.value) || (parsed.value !== original && (parsed.value < -32768 || parsed.value > 32767)))) {
    return { raw, value: null, error: `${field.label} requires an integer from -32768 to 32767 or NULL.` };
  }
  return { raw, ...parsed };
}
export function ordinaryValidation(scope: OrdinaryScope, values: OrdinaryValues,
  original: OrdinaryValues | null, drafts: OrdinaryDrafts): string | null {
  for (const field of ordinaryFields.filter(field => field.scope === scope)) {
    if (field.kind === 'text' || field.kind === 'memo') {
      const error = ordinaryTextError(field, values[field.key], original?.[field.key] ?? null);
      if (error) return error;
    } else {
      const error = drafts[field.key]?.error ??
        ordinaryNumberValue(field, String(values[field.key] ?? ''), original?.[field.key] ?? null).error;
      if (error) return error;
    }
  }
  return null;
}

const sessions = new WeakMap<object, { original: object | null; drafts: OrdinaryDrafts }>();
export function ordinarySession(identity: object, original: object | null): OrdinaryDrafts {
  const session = sessions.get(identity);
  if (session?.original === original) return session.drafts;
  const drafts = {};
  sessions.set(identity, { original, drafts });
  return drafts;
}
export function rememberOrdinarySession(identity: object, original: object | null, drafts: OrdinaryDrafts): void {
  sessions.set(identity, { original, drafts });
}
