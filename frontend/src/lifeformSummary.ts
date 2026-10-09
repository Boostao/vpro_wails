export type LifeformTextCell = {
  storage: 'text' | 'null'; text: string | null; integer: null; real: null; blobHex: null;
};
export type LifeformMembership = { rowId: string; plotNumber: LifeformTextCell; siteUnit: LifeformTextCell };
export type LifeformEntry = {
  plotNumber: string; species: string; lifeform: number; cover: number;
  insertSuRowId: string; speciesRowId: string; projectId: LifeformTextCell;
};
export type LifeformCatalogue = {
  rowId: string; lifeform: number; label: LifeformTextCell; definition: LifeformTextCell; shortName: LifeformTextCell;
};
export type LifeformRow = {
  catalogueRowId: string; lifeform: number; plotGroups: number; coverCount: number;
  presence: number | null; meanCover: number | null;
};
export type LifeformUnit = {
  code: LifeformTextCell; nPlots: number; suRowIds: string[]; uniqueSpecies: number; occurrences: number; rows: LifeformRow[];
};
export type LifeformSummaryPreview = {
  contextId: string; projectPath: string; suPath: string;
  report: {
    project: string; su: string; querySource: 'normal-su-global-entrydat-plot-rejoin';
    ordering: 'physical-catalogue-rowid-and-first-membership';
    memberships: LifeformMembership[]; entries: LifeformEntry[]; catalogue: LifeformCatalogue[]; units: LifeformUnit[];
  };
};
export type LifeformSummaryOwner = { contextId: string; project: string; projectPath: string; su: string; suPath: string };

export { text as lifeformText, shape as lifeformShape, cell as lifeformCell, count as lifeformCount, id as lifeformRowID };

function text(value: unknown): value is string {
  if (typeof value !== 'string' || value.includes('\0')) return false;
  for (let i = 0; i < value.length; i++) {
    const n = value.charCodeAt(i);
    if (n >= 0xd800 && n <= 0xdbff) {
      const next = value.charCodeAt(++i);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (n >= 0xdc00 && n <= 0xdfff) return false;
  }
  return true;
}
function shape(value: unknown, keys: string[]): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
}
function cell(value: unknown, limit = Infinity): value is LifeformTextCell {
  return shape(value, ['storage', 'text', 'integer', 'real', 'blobHex']) &&
    value.integer === null && value.real === null && value.blobHex === null &&
    (value.storage === 'null' ? value.text === null : value.storage === 'text' && text(value.text) && value.text.length <= limit);
}
const count = (n: unknown): n is number => typeof n === 'number' && Number.isSafeInteger(n) && n >= 0;
const form = (n: unknown): n is number => typeof n === 'number' && Number.isInteger(n) && n >= -32768 && n <= 32767;
const number = (n: unknown): n is number => typeof n === 'number' && Number.isFinite(n);
function id(n: unknown): n is string {
  if (typeof n !== 'string' || !/^-?(?:0|[1-9]\d*)$/.test(n) || n === '-0') return false;
  const parsed = BigInt(n);
  return parsed >= -9223372036854775808n && parsed <= 9223372036854775807n;
}
const cellKey = (value: LifeformTextCell) => JSON.stringify([value.storage, value.text]);
const sameCell = (a: LifeformTextCell, b: LifeformTextCell) => cellKey(a) === cellKey(b);
function reject(): never { throw new Error('Lifeform Summary is incomplete, inconsistent or belongs to another owned context; no repairs applied.'); }
// Every finite SINGLE is an exact integer multiple of 2^-149. Accumulate that
// integer, matching the backend's exact sums even for historical cancellation.
function singleCoefficient(value: number): bigint {
  const view = new DataView(new ArrayBuffer(4));
  view.setFloat32(0, value);
  const bits = view.getUint32(0), exponent = (bits >>> 23) & 255, fraction = bits & 0x7fffff;
  const coefficient = exponent === 0 ? BigInt(fraction) : BigInt(fraction | 0x800000) << BigInt(exponent - 1);
  return bits >>> 31 ? -coefficient : coefficient;
}

export function validateLifeformSummary(value: unknown, owner: LifeformSummaryOwner): LifeformSummaryPreview {
  if (!shape(value, ['contextId', 'projectPath', 'suPath', 'report']) ||
      !Object.values(owner).every(text) || owner.contextId === '' || owner.su === 'None' || owner.su === 'USysSuTableDynamic' ||
      value.contextId !== owner.contextId || value.projectPath !== owner.projectPath || value.suPath !== owner.suPath ||
      !shape(value.report, ['project', 'su', 'querySource', 'ordering', 'memberships', 'entries', 'catalogue', 'units'])) reject();
  const report = value.report;
  if (report.project !== owner.project || report.su !== owner.su ||
      report.querySource !== 'normal-su-global-entrydat-plot-rejoin' ||
      report.ordering !== 'physical-catalogue-rowid-and-first-membership' ||
      !Array.isArray(report.memberships) || !Array.isArray(report.entries) ||
      !Array.isArray(report.catalogue) || !Array.isArray(report.units)) reject();
  const memberships = new Map<string, LifeformMembership>();
  for (const m of report.memberships) {
    if (!shape(m, ['rowId', 'plotNumber', 'siteUnit']) || !id(m.rowId) || memberships.has(m.rowId) ||
        !cell(m.plotNumber) || !cell(m.siteUnit)) reject();
    memberships.set(m.rowId, m as LifeformMembership);
  }
  const catalogue = new Map<number, LifeformCatalogue>(), catIDs = new Set<string>();
  for (const c of report.catalogue) {
    if (!shape(c, ['rowId', 'lifeform', 'label', 'definition', 'shortName']) || !id(c.rowId) ||
        catIDs.has(c.rowId) || !form(c.lifeform) || c.lifeform === 13 || catalogue.has(c.lifeform) ||
        !cell(c.label, 255) || !cell(c.definition, 255) || !cell(c.shortName, 255)) reject();
    catIDs.add(c.rowId); catalogue.set(c.lifeform, c as LifeformCatalogue);
  }
  const entries: LifeformEntry[] = [], insertions = new Set<string>();
  for (const e of report.entries) {
    if (!shape(e, ['plotNumber', 'species', 'lifeform', 'cover', 'insertSuRowId', 'speciesRowId', 'projectId']) ||
        !text(e.plotNumber) || e.plotNumber.length > 8 || !text(e.species) || e.species.length > 8 ||
        !form(e.lifeform) || e.lifeform < 0 || e.lifeform > 12 || !number(e.cover) || Math.fround(e.cover) !== e.cover ||
        !id(e.insertSuRowId) || !id(e.speciesRowId) || !cell(e.projectId, 20)) reject();
    const m = memberships.get(e.insertSuRowId), key = JSON.stringify([e.plotNumber, e.species, e.insertSuRowId, e.speciesRowId]);
    if (!m || m.plotNumber.text !== e.plotNumber || !sameCell(m.siteUnit, e.projectId) || insertions.has(key)) reject();
    insertions.add(key); entries.push(e as LifeformEntry);
  }
  const units = new Set<string>(), used = new Set<string>();
  for (const u of report.units) {
    if (!shape(u, ['code', 'nPlots', 'suRowIds', 'uniqueSpecies', 'occurrences', 'rows']) || !cell(u.code) ||
        units.has(cellKey(u.code)) || !count(u.nPlots) || !count(u.uniqueSpecies) || !count(u.occurrences) ||
        !Array.isArray(u.suRowIds) || u.suRowIds.length === 0 || !Array.isArray(u.rows) || u.rows.length !== catalogue.size) reject();
    units.add(cellKey(u.code));
    let nPlots = 0, occurrences = 0;
    const unique = new Set<string>(), groups = new Map<number, Map<string, { sum: bigint; count: number }>>();
    for (const rowId of u.suRowIds) {
      if (!id(rowId) || used.has(rowId)) reject();
      const m = memberships.get(rowId);
      if (!m || !sameCell(m.siteUnit, u.code)) reject();
      used.add(rowId); if (m.plotNumber.storage === 'text') nPlots++;
      if (u.code.storage === 'null' || m.plotNumber.storage === 'null') continue;
      for (const e of entries) {
        if (e.plotNumber !== m.plotNumber.text) continue;
        occurrences++; unique.add(e.species);
        let plots = groups.get(e.lifeform);
        if (!plots) { plots = new Map(); groups.set(e.lifeform, plots); }
        const group = plots.get(e.plotNumber) ?? { sum: 0n, count: 0 };
        group.sum += singleCoefficient(e.cover); group.count++; plots.set(e.plotNumber, group);
      }
    }
    if (u.nPlots !== nPlots || u.occurrences !== occurrences || u.uniqueSpecies !== unique.size) reject();
    let index = 0;
    for (const cat of catalogue.values()) {
      const row = u.rows[index++];
      if (!shape(row, ['catalogueRowId', 'lifeform', 'plotGroups', 'coverCount', 'presence', 'meanCover']) ||
          row.catalogueRowId !== cat.rowId || row.lifeform !== cat.lifeform || !count(row.plotGroups) || !count(row.coverCount)) reject();
      const plots = groups.get(cat.lifeform), plotCount = plots?.size ?? 0;
      let coverCount = 0, total = 0n;
      for (const group of plots?.values() ?? []) { coverCount += group.count; total += group.sum; }
      const presence = nPlots === 0 ? null : plotCount / nPlots;
      const mean = nPlots === 0 || plotCount === 0 ? null : (Number(total) * 2 ** -149) / nPlots / 100;
      if (row.plotGroups !== plotCount || row.coverCount !== coverCount || row.presence !== presence ||
          (mean === null ? row.meanCover !== null : !number(row.meanCover) ||
            Math.abs(row.meanCover - mean) > Math.max(1, Math.abs(mean)) * Number.EPSILON * Math.max(8, occurrences))) reject();
    }
  }
  if (used.size !== memberships.size) reject();
  return structuredClone(value) as LifeformSummaryPreview;
}

export function lifeformCellText(value: LifeformTextCell): string {
  return value.storage === 'null' ? '(NULL)' : value.text === '' ? '(empty)' : value.text!;
}

export function lifeformRatioText(value: number | null): string {
  return value === null ? '(NULL)' : `${(value * 100).toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`;
}
