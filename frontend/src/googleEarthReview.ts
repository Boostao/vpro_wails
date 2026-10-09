import type { GoogleEarthDescriptionFields, GoogleEarthReview, GoogleEarthReviewRow, EnvironmentReportField, ProjectMetadataColumn, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { exactLongitudeNegation } from './plotLocationReview';

export type GoogleEarthScope = { contextId: string; project: string; projectPath: string; su: string; suPath: string };
export type ValidatedGoogleEarthFields = Omit<GoogleEarthDescriptionFields, 'fields'> & { fields: ProjectMetadataColumn[] };
export type ValidatedGoogleEarthReview = Omit<GoogleEarthReview, 'fields' | 'rows'> & {
  fields: EnvironmentReportField[]; rows: GoogleEarthReviewRow[];
};

function text(value: string): boolean {
  return typeof value === 'string' && wellFormedUTF16(value);
}

export function ownedScope(value: GoogleEarthScope, expected: GoogleEarthScope): boolean {
  return !!expected.contextId && (['contextId', 'project', 'projectPath', 'su', 'suPath'] as const).every(key =>
    text(expected[key]) && value[key] === expected[key]);
}

function sameCell(left: ProjectMetadataCell, right: ProjectMetadataCell): boolean {
  return left.storage === right.storage && left.text === right.text && left.integer === right.integer &&
    Object.is(left.real, right.real) && left.blobHex === right.blobHex;
}

export function validateGoogleEarthFields(value: GoogleEarthDescriptionFields | null, scope: GoogleEarthScope): ValidatedGoogleEarthFields {
  value = value ? structuredClone(value) : null;
  if (!value || !ownedScope(value, scope) || value.envTable !== `${scope.project}_Env` ||
      !Array.isArray(value.fields) || !value.fields.length ||
      value.fields.some(field => !field || !text(field.name) || !field.name || field.name.includes('\0') ||
        !text(field.declaredType)) || new Set(value.fields.map(field => field.name)).size !== value.fields.length ||
      !['PlotNumber', 'Longitude', 'Latitude'].every(name => value?.fields?.some(field => field.name === name))) {
    throw new Error('Description fields have a foreign/incomplete owned physical Env scope; no inferred choices accepted.');
  }
  return { ...value, fields: value.fields };
}

export async function validateGoogleEarthReview(value: GoogleEarthReview | null, scope: GoogleEarthScope,
  descriptionField: string, offset: number, limit: number,
  yieldTask: () => Promise<void> = () => new Promise(resolve => setTimeout(resolve, 0))): Promise<ValidatedGoogleEarthReview> {
  value = value ? structuredClone(value) : null;
  const keys = ['PlotNumber', 'Longitude', 'Latitude', descriptionField];
  const labels = ['Plot Number', 'Longitude', 'Latitude', descriptionField];
  if (!value || !ownedScope(value, scope) || !text(descriptionField) || !descriptionField || descriptionField.includes('\0') ||
      value.descriptionField !== descriptionField || value.offset !== offset || value.limit !== limit ||
      !Number.isSafeInteger(offset) || offset < 0 || !Number.isSafeInteger(limit) || limit < 1 || limit > 500 ||
      !Number.isSafeInteger(value.totalRows) || value.totalRows < 0 ||
      !Array.isArray(value.fields) || value.fields.length !== 4 ||
      !value.fields.every((field, i) => field.source === 'Env' && field.key === keys[i] &&
        field.label === labels[i] && field.heading === false) ||
      !Array.isArray(value.rows) || value.rows.length !== Math.min(limit, Math.max(0, value.totalRows - offset))) {
    throw new Error('Google Earth review has a foreign/incomplete owned scope, field selection or page; no partial review accepted.');
  }
  const pairs = new Set<string>();
  const envValues = new Map<string, ProjectMetadataCell[]>();
  for (let i = 0; i < value.rows.length; i++) {
    const row = value.rows[i];
    if (!row || !exactSigned64(row.envRowId) || (scope.su === 'None' ? row.membershipRowId !== '' : !exactSigned64(row.membershipRowId)) ||
        ![row.plotNumber, row.storedLongitude, row.longitude, row.latitude, row.description].every(completeCell) ||
        !['integer', 'real'].includes(row.latitude.storage) || !exactLongitudeNegation(row.storedLongitude, row.longitude)) {
      throw new Error('Google Earth review has incomplete physical identities/cells or changed longitude semantics.');
    }
    const pair = `${row.envRowId}:${row.membershipRowId}`;
    const original = [row.plotNumber, row.storedLongitude, row.latitude, row.description];
    const previous = envValues.get(row.envRowId);
    const selected = descriptionField === 'PlotNumber' ? row.plotNumber : descriptionField === 'Longitude'
      ? row.storedLongitude : descriptionField === 'Latitude' ? row.latitude : undefined;
    if (pairs.has(pair) || (previous && !previous.every((cell, i) => sameCell(cell, original[i]))) ||
        (selected && !sameCell(selected, row.description))) {
      throw new Error('Google Earth review duplicates a physical pair or changes one Env row between memberships.');
    }
    pairs.add(pair); envValues.set(row.envRowId, original);
    if ((i + 1) % 100 === 0) await yieldTask();
  }
  return { ...value, fields: value.fields, rows: value.rows };
}
