import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { siviMetadataTableFromWire, type SIVIParentOwner } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';

export const siviReferencePolicies = [
  { column: 'FSRegionDistrict', list: 'Region', maximum: 7, required: false, scope: 'site', label: 'Region' },
  { column: 'Zone', list: 'Zone', maximum: 4, required: false, scope: 'site', label: 'BGC' },
  { column: 'SubZone', list: 'SubZone', maximum: 8, required: false, scope: 'site', label: 'BGC / SubZone' },
  { column: 'RealmClass', list: 'RealmClass', maximum: 5, required: false, scope: 'site', label: 'Realm Class' },
  { column: 'MoistureRegime', list: 'MoistureRegime', maximum: 3, required: false, scope: 'site', label: 'SMR' },
  { column: 'NutrientRegime', list: 'NutrientRegime', maximum: 2, required: false, scope: 'site', label: 'SNR' },
  { column: 'MesoSlopePosition', list: 'MesoSlopePosition', maximum: 3, required: false, scope: 'topography', label: 'Meso Slope' },
  { column: 'SurfaceShape', list: 'SurfaceShape', maximum: 3, required: false, scope: 'topography', label: 'Surface Shape' },
  { column: 'Exposure1', list: 'Exposure', maximum: 2, required: true, scope: 'topography', label: 'Exposure 1' },
  { column: 'Exposure2', list: 'Exposure', maximum: 2, required: true, scope: 'topography', label: 'Exposure 2' },
  { column: 'SiteDisturbance1', list: 'SiteDisturbance', maximum: 8, required: false, scope: 'site', label: 'Site Disturbance 1' },
  { column: 'SiteDisturbance2', list: 'SiteDisturbance', maximum: 8, required: false, scope: 'site', label: 'Site Disturbance 2' },
  { column: 'SiteDisturbance3', list: 'SiteDisturbance', maximum: 8, required: false, scope: 'site', label: 'Site Disturbance 3' },
  { column: 'StructuralStage', list: 'StructuralStage', maximum: 6, required: false, scope: 'site', label: 'Structural Stage' },
  { column: 'SuccessionalStatus', list: 'SuccessionalStatus', maximum: 3, required: true, scope: 'site', label: 'Successional Status' },
  { column: 'TerrainTextureSurf', list: 'TerrainTexture', maximum: 3, required: false, scope: 'terrain', label: 'Texture / Surface' },
  { column: 'SurficialMaterialSurf', list: 'SurficialMaterial', maximum: 6, required: false, scope: 'terrain', label: 'Surf. Material / Surface' },
  { column: 'SurfaceExpSurf', list: 'SurfaceExp', maximum: 3, required: false, scope: 'terrain', label: 'S. Express. / Surface' },
  { column: 'GeoMorProSurf', list: 'GeoMorPro', maximum: 3, required: false, scope: 'terrain', label: 'Geo. Process / Surface' },
  { column: 'BedrockGeology1', list: 'BedrockType', maximum: 4, required: false, scope: 'terrain', label: 'Bedrock / Surface' },
  { column: 'TerrainTextureSubSurf', list: 'TerrainTexture', maximum: 3, required: false, scope: 'terrain', label: 'Texture / Sub Surface' },
  { column: 'SurficialMaterialSubSurf', list: 'SurficialMaterial', maximum: 6, required: false, scope: 'terrain', label: 'Surf. Material / Sub Surface' },
  { column: 'SurfaceExpSubSurf', list: 'SurfaceExp', maximum: 3, required: false, scope: 'terrain', label: 'S. Express. / Sub Surface' },
  { column: 'GeoMorProSubSurf', list: 'GeoMorPro', maximum: 3, required: false, scope: 'terrain', label: 'Geo. Process / Sub Surface' },
  { column: 'BedrockGeology2', list: 'BedrockType', maximum: 4, required: false, scope: 'terrain', label: 'Bedrock / Sub Surface' },
  { column: 'HumusForm', list: 'HumusForm', maximum: 4, required: false, scope: 'soils', label: 'Humus Form' },
  { column: 'SoilDrainage', list: 'SoilDrainage', maximum: 5, required: true, scope: 'soils', label: 'Soil Drainage' },
  { column: 'RootRestrictingType', list: 'RootRestrictingType', maximum: 1, required: false, scope: 'soils', label: 'Root Restricting Type' },
] as const;
export type SIVIReferenceColumn = typeof siviReferencePolicies[number]['column'];
export const siviReferenceBuildEnabled = (value: unknown): boolean => value === 'true';
export interface SIVIReferenceChoice {
  rowId: string; code: string | null; description: string | null; selectable: boolean; diagnostic: string;
}
export interface SIVIReferenceField {
  column: SIVIReferenceColumn; listName: string; required: boolean; source: string;
  available: boolean; diagnostic: string;
  definitions: ReturnType<typeof siviMetadataTableFromWire>; choices: SIVIReferenceChoice[];
}
export interface SIVIReferences {
  contextId: string; project: string; plot: string; zone: ProjectMetadataCell; fields: SIVIReferenceField[];
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const nullableText = (value: unknown): value is string | null => value === null || typeof value === 'string';
function choice(value: unknown): value is SIVIReferenceChoice {
  return record(value) && typeof value.rowId === 'string' && nullableText(value.code) &&
    nullableText(value.description) && typeof value.selectable === 'boolean' && typeof value.diagnostic === 'string';
}
export function siviReferencesFromWire(wire: unknown, owner: SIVIParentOwner, zone: ProjectMetadataCell): SIVIReferences {
  if (!record(wire) || wire.contextId !== owner.contextId || wire.project !== owner.project ||
      wire.plot !== owner.plot || !Array.isArray(wire.fields) || wire.fields.length !== siviReferencePolicies.length) {
    throw new Error('SIVI reference response is incomplete or belongs to another owner.');
  }
  const receivedZone = siviMetadataTableFromWire({ columns: [{ name: 'Zone', declaredType: 'TEXT' }],
    rows: [{ rowId: '1', cells: [wire.zone] }] }).rows[0].cells[0];
  if (!equalCell(receivedZone, zone)) throw new Error('SIVI reference response belongs to another effective Zone.');
  const receivedFields = wire.fields;
  const fields = siviReferencePolicies.map(policy => {
    const matches = receivedFields.filter((field: unknown) => record(field) && field.column === policy.column);
    const field: unknown = matches[0];
    if (matches.length !== 1 || !record(field) || field.listName !== policy.list || field.required !== policy.required ||
        typeof field.source !== 'string' || typeof field.available !== 'boolean' || typeof field.diagnostic !== 'string' ||
        !field.available && !field.diagnostic || !Array.isArray(field.choices) || !field.choices.every(choice)) {
      throw new Error(`SIVI ${policy.column} reference policy/receipt is incomplete.`);
    }
    return { column: policy.column, listName: policy.list, required: policy.required, source: field.source,
      available: field.available, diagnostic: field.diagnostic,
      definitions: siviMetadataTableFromWire(field.definitions), choices: structuredClone(field.choices) };
  });
  return { ...owner, zone: structuredClone(receivedZone), fields };
}
export function siviReferenceIssue(column: string, expected: ProjectMetadataCell, value: ProjectMetadataCell,
  references: SIVIReferences | null): { error: string | null; advisory: string | null } {
  const policy = siviReferencePolicies.find(field => field.column === column);
  if (!policy || equalCell(expected, value) || value.storage === 'null') return { error: null, advisory: null };
  const field = references?.fields.find(field => field.column === column);
  const listed = field?.available && field.choices.some(row => row.selectable && row.code !== null &&
    row.code !== '' && row.code.length <= policy.maximum && wellFormedUTF16(row.code) && row.code === value.text);
  const diagnostic = !field?.available ? field?.diagnostic || `${policy.label} reference choices are unavailable.`
    : `${policy.label} is not an exact listed Item${column === 'SubZone' ? ' for the effective Zone' : ''}.`;
  return listed ? { error: null, advisory: null } : policy.required
    ? { error: diagnostic, advisory: null } : { error: null, advisory: `${diagnostic} Free entry will be retained literally.` };
}
