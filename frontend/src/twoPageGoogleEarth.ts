import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import type { TwoPageParentForm } from './twoPageParentSession';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';
import { googleEarthXMLText, validateGoogleEarthKMLReview } from './googleEarthKMLReview';

export interface TwoPageGoogleEarthOwner {
  contextId: string; project: string; projectPath: string; plot: string; form: TwoPageParentForm;
}
type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface TwoPageGoogleEarthOptionsView extends TwoPageGoogleEarthOwner {
  fields: Table['columns'];
  title: string;
  descriptionField: string;
}
export interface TwoPageGoogleEarthReviewView extends TwoPageGoogleEarthOwner {
  original: Table;
  title: string;
  descriptionField: string;
  kml: string;
  byteCount: number;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

function owned(value: unknown, owner: TwoPageGoogleEarthOwner): Record<string, unknown> {
  if (![owner.contextId, owner.project, owner.projectPath, owner.plot].every(value => !!value && wellFormedUTF16(value)) ||
      !['FS882-8x6XL', 'FS882-8x6XL-CHARS'].includes(owner.form) || !record(value) ||
      Object.entries(owner).some(([key, expected]) => value[key] !== expected)) {
    throw new Error('Saved-plot KML belongs to another context, project, path, literal plot or source form.');
  }
  return value;
}
function schema(columns: Table['columns']) {
  if (!columns.length || new Set(columns.map(column => column.name)).size !== columns.length ||
      columns.some(column => !column.name || !wellFormedUTF16(column.name) || !wellFormedUTF16(column.declaredType))) {
    throw new Error('Saved-plot KML requires complete literal physical fields; no schema repaired.');
  }
}
export function twoPageGoogleEarthOptionsFromWire(value: unknown, owner: TwoPageGoogleEarthOwner): TwoPageGoogleEarthOptionsView {
  const wire = owned(value, owner);
  const fields = siviMetadataTableFromWire({ columns: wire.fields, rows: [] }).columns;
  schema(fields);
  if (!record(wire.values) || typeof wire.values.title !== 'string' || typeof wire.values.descriptionField !== 'string' ||
      !wellFormedUTF16(wire.values.title) || !wellFormedUTF16(wire.values.descriptionField)) {
    throw new Error('Saved-plot KML options are incomplete or malformed; no defaults applied.');
  }
  return { ...owner, fields, title: wire.values.title, descriptionField: wire.values.descriptionField };
}
function number(cell: ProjectMetadataCell): number {
  if (cell.storage === 'real' && typeof cell.real === 'number') return cell.real;
  if (cell.storage === 'integer' && typeof cell.integer === 'string') return Number(cell.integer);
  throw new Error('Saved-plot KML coordinates require original numeric storage.');
}
export function twoPageGoogleEarthCell(review: TwoPageGoogleEarthReviewView, column: string): ProjectMetadataCell {
  const index = review.original.columns.findIndex(field => field.name === column);
  if (index < 0 || !review.original.rows[0]?.cells[index]) throw new Error(`Saved-plot KML ${column} original is unavailable.`);
  return review.original.rows[0].cells[index];
}
export async function twoPageGoogleEarthReviewFromWire(value: unknown, owner: TwoPageGoogleEarthOwner,
  descriptionField: string, title: string): Promise<TwoPageGoogleEarthReviewView> {
  const wire = owned(value, owner);
  if (wire.descriptionField !== descriptionField || wire.title !== title ||
      typeof wire.kml !== 'string' || typeof wire.byteCount !== 'number') {
    throw new Error('Saved-plot KML has foreign options or incomplete output.');
  }
  const original = siviMetadataTableFromWire(wire.original);
  schema(original.columns);
  if (original.rows.length !== 1) throw new Error('Saved-plot KML requires exactly one complete physical Env row.');
  const result: TwoPageGoogleEarthReviewView = { ...owner, original, title, descriptionField, kml: wire.kml, byteCount: wire.byteCount };
  const plot = twoPageGoogleEarthCell(result, 'PlotNumber');
  const longitude = number(twoPageGoogleEarthCell(result, 'Longitude'));
  const latitude = number(twoPageGoogleEarthCell(result, 'Latitude'));
  const description = twoPageGoogleEarthCell(result, descriptionField);
  if (plot.storage !== 'text' || plot.text !== owner.plot || !Number.isFinite(longitude) || Object.is(longitude, -0) ||
      longitude < 0 || longitude > 180 || !Number.isFinite(latitude) || latitude < -90 || latitude > 90 ||
      description.storage !== 'null' && (description.storage !== 'text' || typeof description.text !== 'string' ||
        !googleEarthXMLText(description.text))) {
    throw new Error('Saved-plot KML original ownership, coordinates or description are unavailable; no values repaired.');
  }
  const scope = { contextId: owner.contextId, project: owner.project, projectPath: owner.projectPath, su: 'None', suPath: '' };
  const checked = await validateGoogleEarthKMLReview({
    ...scope, descriptionField, title, placemarkCount: 1, byteCount: result.byteCount, kml: result.kml,
  }, scope, descriptionField, title);
  const document = new DOMParser().parseFromString(checked.kml, 'application/xml');
  const mark = document.documentElement.children[0].children[1];
  const coordinates = (mark.children[2].children[0].textContent ?? '').split(',');
  const descriptionText = description.storage === 'null' ? '' : description.text;
  if (typeof descriptionText !== 'string') throw new Error('Saved-plot KML description original is unavailable.');
  const escapedDescription = (descriptionText + '.').replaceAll('&', '&amp;').replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;').replaceAll('"', '&#34;').replaceAll("'", '&#39;');
  if (mark.children[0].textContent !== owner.plot || Number(coordinates[0]) !== -longitude || Number(coordinates[1]) !== latitude ||
      mark.children[1].textContent !== escapedDescription) {
    throw new Error('Saved-plot KML does not match its complete original Env row.');
  }
  return result;
}
