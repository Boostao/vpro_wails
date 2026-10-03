import type { ScopedPlotLookupResult } from '../bindings/github.com/boostao/vpro-wails';
import { wellFormedUTF16 } from './qualityEditor';

export function plotLookupInputError(value: string): string | null {
  if (!value || !wellFormedUTF16(value) || value.includes('\0')) {
    return 'Enter one complete literal plot number without NUL. Spaces and case are preserved; no wildcard or partial matching.';
  }
  return null;
}

export function validateScopedPlotLookup(result: ScopedPlotLookupResult, number: string, contextId: string): ScopedPlotLookupResult {
  if (!contextId || !result || result.contextId !== contextId || !result.plot ||
      result.plot.plotNumber !== number || plotLookupInputError(number) ||
      (['fieldNumber', 'plotRepresenting', 'zone', 'subZone', 'siteSeries'] as const).some(name => {
        const value = result.plot[name];
        return value !== null && typeof value !== 'string';
      })) {
    throw new Error('Find returned an incomplete, normalized or foreign scoped plot; current editor and filters remain unchanged.');
  }
  return result;
}
