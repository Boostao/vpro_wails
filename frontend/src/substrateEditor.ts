import { finiteSingleValue } from './numericEditor';

export const substrateKeys = ['substrateOrganicMatter', 'substrateRocks', 'substrateDecWood',
  'substrateMineralSoil', 'substrateBedRock', 'substrateWater'] as const;
export type SubstrateKey = typeof substrateKeys[number];
export type SubstrateValues = Record<SubstrateKey, number | null>;
export const substrateLabels: Record<SubstrateKey, string> = {
  substrateOrganicMatter: 'Substrate organic matter', substrateRocks: 'Substrate rocks',
  substrateDecWood: 'Substrate decaying wood', substrateMineralSoil: 'Substrate mineral soil',
  substrateBedRock: 'Substrate bedrock', substrateWater: 'Substrate water'
};
export interface SubstrateCell {
  raw: string;
  expected: number | null;
  value: number | null;
  error: string | null;
}
export type SubstrateDrafts = Partial<Record<SubstrateKey, SubstrateCell>>;
export function substrateKey(column: string | undefined): SubstrateKey | undefined {
  return substrateKeys.find(key => key.toLowerCase() === column?.toLowerCase());
}
export function stageSubstrate(drafts: SubstrateDrafts, key: SubstrateKey, raw: string, stored: number | null): SubstrateDrafts {
  const previous = drafts[key];
  const expected = previous ? previous.expected : stored;
  return { ...drafts, [key]: { raw, expected, ...finiteSingleValue(substrateLabels[key], raw, expected) } };
}
export function substrateErrors(drafts: SubstrateDrafts): string[] {
  return substrateKeys.flatMap(key => {
    const cell = drafts[key];
    return cell?.error ? [cell.error] : [];
  });
}
export function substrateDirty(drafts: SubstrateDrafts): boolean {
  return substrateKeys.some(key => {
    const cell = drafts[key];
    return cell && (cell.error !== null || cell.value !== cell.expected);
  });
}
export function substrateValidation(drafts: SubstrateDrafts, values: SubstrateValues, original: SubstrateValues | null): string | null {
  const errors = substrateErrors(drafts);
  if (errors.length) return errors[0];
  for (const key of substrateKeys) {
    const error = finiteSingleValue(substrateLabels[key], String(values[key] ?? ''), original?.[key] ?? null).error;
    if (error) return error;
  }
  return null;
}
const sessions = new WeakMap<object, { original: object | null; drafts: SubstrateDrafts }>();
export function substrateSession(identity: object, original: object | null = null): SubstrateDrafts {
  const session = sessions.get(identity);
  if (session && session.original === original) return session.drafts;
  const drafts = {};
  sessions.set(identity, { original, drafts });
  return drafts;
}
export function rememberSubstrateSession(identity: object, drafts: SubstrateDrafts, original: object | null = null): void {
  sessions.set(identity, { original, drafts });
}
