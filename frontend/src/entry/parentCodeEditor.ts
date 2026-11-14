import { createReferenceCodeEditor } from './referenceCodeEditor';
import type { QualityView } from './qualityEditor';

export const parentCodeFields = [
  { key: 'coarseFragLith1', column: 'CoarseFragLith1', label: 'Coarse fragment lithology 1', list: 'BedrockType', maximum: 12 },
  { key: 'coarseFragLith2', column: 'CoarseFragLith2', label: 'Coarse fragment lithology 2', list: 'BedrockType', maximum: 12 },
  { key: 'coarseFragLith3', column: 'CoarseFragLith3', label: 'Coarse fragment lithology 3', list: 'BedrockType', maximum: 12 },
  { key: 'floodingRegimeDur', column: 'FloodingRegimeDur', label: 'Flooding duration', list: 'FloodingRegimeDur', maximum: 2 },
  { key: 'floodingRegimeFreq', column: 'FloodingRegimeFreq', label: 'Flooding frequency', list: 'FloodingRegimeFreq', maximum: 7 },
  { key: 'geoMorProSubSurf', column: 'GeoMorProSubSurf', label: 'Subsurface geomorphological process', list: 'GeoMorPro', maximum: 3 },
  { key: 'geoMorProSurf', column: 'GeoMorProSurf', label: 'Surface geomorphological process', list: 'GeoMorPro', maximum: 3 },
  { key: 'humusForm', column: 'HumusForm', label: 'Humus form', list: 'HumusForm', maximum: 4 },
  { key: 'humusFormPhase', column: 'HumusFormPhase', label: 'Humus form phase', list: 'HumusFormPhase', maximum: 50 },
  { key: 'hydroGeoSubSystem', column: 'HydroGeoSubSystem', label: 'Hydrogeological subsystem', list: 'HydrogeoSubsystem', maximum: 2 },
  { key: 'hydroGeoSystem', column: 'HydroGeoSystem', label: 'Hydrogeological system', list: 'HydrogeoSystem', maximum: 3 },
  { key: 'realmClass', column: 'RealmClass', label: 'Realm class', list: 'RealmClass', maximum: 5 },
  { key: 'rootRestrictingType', column: 'RootRestrictingType', label: 'Root restricting type', list: 'RootRestrictingType', maximum: 1 },
  { key: 'rootZoneParticleSize', column: 'RootZoneParticleSize', label: 'Root zone particle size', list: 'RootZoneParticleSize', maximum: 6 },
  { key: 'surfaceExpSubSurf', column: 'SurfaceExpSubSurf', label: 'Subsurface expression', list: 'SurfaceExp', maximum: 3 },
  { key: 'surfaceExpSurf', column: 'SurfaceExpSurf', label: 'Surface expression', list: 'SurfaceExp', maximum: 3 },
  { key: 'surficialMaterialSubSurf', column: 'SurficialMaterialSubSurf', label: 'Subsurface material', list: 'SurficialMaterial', maximum: 6 },
  { key: 'surficialMaterialSurf', column: 'SurficialMaterialSurf', label: 'Surface material', list: 'SurficialMaterial', maximum: 6 },
  { key: 'terrainTextureSubSurf', column: 'TerrainTextureSubSurf', label: 'Subsurface terrain texture', list: 'TerrainTexture', maximum: 3 },
  { key: 'terrainTextureSurf', column: 'TerrainTextureSurf', label: 'Surface terrain texture', list: 'TerrainTexture', maximum: 3 },
  { key: 'waterSource', column: 'WaterSource', label: 'Water source', list: 'WaterSource', maximum: 5 }
] as const;
export type ParentCodeKey = typeof parentCodeFields[number]['key'];
export type ParentCodeList = typeof parentCodeFields[number]['list'];
export type ParentCodes = Record<ParentCodeKey, string | null>;
export type ParentCodeScope = 'site' | 'soils';
export type ParentCodeViews = Partial<Record<ParentCodeList, QualityView>>;
export const parentCodeKeys = parentCodeFields.map(field => field.key);
export function parentCodeField(column: string | undefined) {
  return parentCodeFields.find(field => field.column === column);
}
const labels = Object.fromEntries(parentCodeFields.map(field => [field.key, field.label])) as Record<ParentCodeKey, string>;
const lists = Object.fromEntries(parentCodeFields.map(field => [field.key, field.list])) as Record<ParentCodeKey, ParentCodeList>;
const limits = Object.fromEntries(parentCodeFields.map(field => [field.list, field.maximum])) as Record<ParentCodeList, number>;
const editors = {
  site: createReferenceCodeEditor<ParentCodeKey, ParentCodeList>({ keys: ['realmClass'], labels, list: key => lists[key], limits }),
  soils: createReferenceCodeEditor<ParentCodeKey, ParentCodeList>({ keys: parentCodeKeys.filter(key => key !== 'realmClass'), labels, list: key => lists[key], limits })
};
export function createParentCodeEditor(scope: ParentCodeScope) {
  const fields = parentCodeFields.filter(field => (field.key === 'realmClass') === (scope === 'site'));
  const reference = editors[scope];
  function busy(codes: ParentCodes, original: ParentCodes | null, views: ParentCodeViews): boolean {
    return fields.some(({ key, list }) => codes[key] !== null && codes[key] !== (original?.[key] ?? null) && views[list]?.busy);
  }
  function warnings(codes: ParentCodes, original: ParentCodes | null, views: ParentCodeViews): string[] {
    return fields.flatMap(({ key, list, label }) => {
      const view = views[list];
      if (codes[key] === null || codes[key] === (original?.[key] ?? null) || view?.busy) return [];
      if (!view?.ready) return [`${label}: ${view?.error ?? 'Reference choices are unavailable; this raw code has not been checked.'}`];
      return reference.definitions(view.choices, list, codes[key]).length ? []
        : [`${label} "${codes[key]}" is not a listed Item. It will be kept exactly as entered.`];
    });
  }
  function validation(codes: ParentCodes, original: ParentCodes | null, views: ParentCodeViews, accepted: boolean): string | null {
    for (const { key } of fields) {
      const error = reference.valueError(key, codes[key], original?.[key] ?? null);
      if (error) return error;
    }
    if (busy(codes, original, views)) return 'Wait for changed parent code choices to finish refreshing.';
    return warnings(codes, original, views).length && !accepted
      ? 'Review and explicitly acknowledge the unmatched or unchecked parent codes before saving.' : null;
  }
  return { ...reference, fields, lists: [...new Set(fields.map(field => field.list))], busy, warnings, validation };
}
