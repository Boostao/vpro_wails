import type { VegetationReadAvailability, VegRecord } from '../bindings/github.com/boostao/vpro-wails';

export function vegetationReadAvailability(result: VegetationReadAvailability | null): { records: VegRecord[]; reason: string | null } {
  if (!result || typeof result.available !== 'boolean' || typeof result.reason !== 'string' ||
      result.records === undefined ||
      result.available && result.reason !== '' || !result.available && (!result.reason || result.records !== null)) {
    throw new Error('Vegetation availability acknowledgement is incomplete; no ordinary rows were assigned.');
  }
  if (!result.available) return { records: [], reason: result.reason };
  const records = result.records ?? [];
  if (!Array.isArray(records) || records.some(record => !record || !Number.isSafeInteger(record.id)) ||
      new Set(records.map(record => record.id)).size !== records.length) {
    throw new Error('Vegetation availability returned unsupported ordinary identities; no rows were assigned.');
  }
  return { records, reason: null };
}
