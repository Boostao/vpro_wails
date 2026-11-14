export interface SpeciesPresenceScope {
  contextId: string;
  project: string;
  table: string;
  su: string;
}

export interface SpeciesPresencePreview extends SpeciesPresenceScope {
  mode: 'saved-species-only-readonly';
  totalPlots: number;
  plotNumbers: string[];
}

const presenceKeys = ['contextId', 'mode', 'project', 'table', 'su', 'totalPlots', 'plotNumbers'];

function presenceRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

export async function validateSpeciesPresencePreview(
  value: unknown,
  expected: SpeciesPresenceScope,
  yieldValidation: () => Promise<void> = () => new Promise(resolve => setTimeout(resolve, 0)),
): Promise<SpeciesPresencePreview> {
  if (!expected.contextId || !expected.project || !expected.table || !expected.su ||
      !presenceRecord(value) || Object.keys(value).length !== presenceKeys.length ||
      presenceKeys.some(key => !Object.hasOwn(value, key)) ||
      value.contextId !== expected.contextId || value.project !== expected.project ||
      value.table !== expected.table || value.su !== expected.su ||
      value.mode !== 'saved-species-only-readonly' ||
      typeof value.totalPlots !== 'number' || !Number.isSafeInteger(value.totalPlots) || value.totalPlots < 0 ||
      value.plotNumbers !== null && !Array.isArray(value.plotNumbers)) {
    throw new Error('Species-only preview returned an inconsistent readonly scope or wire; no navigation or SU action is available.');
  }
  const { contextId, project, table, su } = expected;
  const totalPlots = value.totalPlots;
  const plots: unknown[] = value.plotNumbers === null ? [] : value.plotNumbers.slice();
  if (plots.length > totalPlots) {
    throw new Error('Species-only preview contains more matches than its complete scope.');
  }
  const seen = new Set<string>();
  const plotNumbers: string[] = [];
  for (let index = 0; index < plots.length; index++) {
    const plot = plots[index];
    if (typeof plot !== 'string' || !/^[\x01-\x7f]{1,7}$/.test(plot) ||
        index > 0 && plotNumbers[index - 1] >= plot) {
      throw new Error('Species-only preview contains invalid, duplicate or unordered literal plot identities.');
    }
    const comparison = plot.replace(/ +$/, '').toUpperCase();
    if (seen.has(comparison)) {
      throw new Error('Species-only preview contains source-equivalent but distinct literal plot identities.');
    }
    seen.add(comparison);
    plotNumbers.push(plot);
    if ((index + 1) % 128 === 0) await yieldValidation();
  }
  return { contextId, project, table, su, mode: 'saved-species-only-readonly', totalPlots, plotNumbers };
}
