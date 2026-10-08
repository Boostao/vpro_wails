import {
  lifeformText as text, lifeformShape as shape, lifeformCell as cell,
  lifeformCount as count, lifeformRowID as id,
  type LifeformTextCell, type LifeformMembership, type LifeformSummaryOwner,
} from './lifeformSummary';

export const speciesAttributeDefinitions = [
  { field: 'SRank', label: 'SRank Detail', categories: ['S1', 'S2', 'S2S3', 'S3', 'S3S4', 'S4', 'S4S5', 'S5', 'SE1', 'SE1SE2', 'SE2', 'SE3', 'SE3SE4', 'SE4', 'SE5', 'SEH', 'SEX', 'SH', 'SU', 'SX'] },
  { field: 'Wetland_Ind', label: 'Wetland Indicator', categories: ['1', '2', '3', '4'] },
  { field: 'WeedStatus', label: 'Weed Status', categories: ['I', 'P', 'R'] },
  { field: 'RedBlueList', label: 'Red Blue List', categories: ['R', 'B'] },
  { field: 'Est_ASMR', label: 'Est. ASMR', categories: ['0', '1', '2', '3', '4', '5', '6'] },
  { field: 'Climate', label: 'Climate', categories: ['0', '1', '2', '3', '4', '5', '6'] },
] as const;

export type SpeciesAttributeMatch = {
  suRowId: string; vegRowId: string; attributeRowId: string; plotNumber: string; species: string; values: LifeformTextCell[];
};
export type SpeciesAttributeCount = { field: string; count: number | null; plotOccurrences: number; categories: (number | null)[] };
export type SpeciesAttributeUnit = { code: LifeformTextCell; nPlots: number; suRowIds: string[]; rows: SpeciesAttributeCount[] };
export type SpeciesAttributeSummaryPreview = {
  contextId: string; projectPath: string; suPath: string;
  report: {
    project: string; su: string; querySource: 'normal-su-raw-veg-attribute-join';
    definitions: { field: string; label: string; categories: string[] }[];
    memberships: LifeformMembership[]; matches: SpeciesAttributeMatch[]; units: SpeciesAttributeUnit[];
  };
};

const key = (value: LifeformTextCell) => JSON.stringify([value.storage, value.text]);
function reject(): never { throw new Error('Species attribute summary is incomplete, inconsistent or belongs to another owned context; no repairs applied.'); }

export function validateSpeciesAttributeSummary(value: unknown, owner: LifeformSummaryOwner): SpeciesAttributeSummaryPreview {
  if (!shape(value, ['contextId', 'projectPath', 'suPath', 'report']) ||
      !Object.values(owner).every(text) || owner.contextId === '' || owner.su === 'None' || owner.su === 'USysSuTableDynamic' ||
      value.contextId !== owner.contextId || value.projectPath !== owner.projectPath || value.suPath !== owner.suPath ||
      !shape(value.report, ['project', 'su', 'querySource', 'definitions', 'memberships', 'matches', 'units'])) reject();
  const report = value.report;
  if (report.project !== owner.project || report.su !== owner.su || report.querySource !== 'normal-su-raw-veg-attribute-join' ||
      !Array.isArray(report.definitions) || report.definitions.length !== speciesAttributeDefinitions.length ||
      !Array.isArray(report.memberships) || !Array.isArray(report.matches) || !Array.isArray(report.units)) reject();
  for (const [i, definition] of report.definitions.entries()) {
    const expected = speciesAttributeDefinitions[i];
    if (!shape(definition, ['field', 'label', 'categories']) || definition.field !== expected.field || definition.label !== expected.label ||
        !Array.isArray(definition.categories) || definition.categories.length !== expected.categories.length ||
        definition.categories.some((value, index) => value !== expected.categories[index])) reject();
  }
  const memberships = new Map<string, LifeformMembership>(), byMember = new Map<string, SpeciesAttributeMatch[]>();
  for (const m of report.memberships) {
    if (!shape(m, ['rowId', 'plotNumber', 'siteUnit']) || !id(m.rowId) || memberships.has(m.rowId) || !cell(m.plotNumber) || !cell(m.siteUnit)) reject();
    memberships.set(m.rowId, m as LifeformMembership);
  }
  const triples = new Set<string>(), vegetation = new Map<string, string>(), attributes = new Map<string, string>();
  for (const m of report.matches) {
    if (!shape(m, ['suRowId', 'vegRowId', 'attributeRowId', 'plotNumber', 'species', 'values']) ||
        !id(m.suRowId) || !id(m.vegRowId) || !id(m.attributeRowId) || !text(m.plotNumber) || !text(m.species) ||
        !Array.isArray(m.values) || m.values.length !== speciesAttributeDefinitions.length || !m.values.every(value => cell(value))) reject();
    const member = memberships.get(m.suRowId), triple = JSON.stringify([m.suRowId, m.vegRowId, m.attributeRowId]);
    const veg = JSON.stringify([m.plotNumber, m.species]), attribute = JSON.stringify([m.species, m.values]);
    if (!member || member.siteUnit.storage === 'null' || member.plotNumber.storage !== 'text' || member.plotNumber.text !== m.plotNumber ||
        triples.has(triple) || vegetation.has(m.vegRowId) && vegetation.get(m.vegRowId) !== veg ||
        attributes.has(m.attributeRowId) && attributes.get(m.attributeRowId) !== attribute) reject();
    triples.add(triple); vegetation.set(m.vegRowId, veg); attributes.set(m.attributeRowId, attribute);
    const matches = byMember.get(m.suRowId) ?? [];
    matches.push(m as SpeciesAttributeMatch); byMember.set(m.suRowId, matches);
  }
  const used = new Set<string>(), units = new Set<string>();
  for (const u of report.units) {
    if (!shape(u, ['code', 'nPlots', 'suRowIds', 'rows']) || !cell(u.code) || units.has(key(u.code)) || !count(u.nPlots) ||
        !Array.isArray(u.suRowIds) || u.suRowIds.length === 0 || !Array.isArray(u.rows) || u.rows.length !== speciesAttributeDefinitions.length) reject();
    units.add(key(u.code));
    const matches: SpeciesAttributeMatch[] = [];
    let nPlots = 0;
    for (const rowId of u.suRowIds) {
      if (!id(rowId) || used.has(rowId)) reject();
      const member = memberships.get(rowId);
      if (!member || key(member.siteUnit) !== key(u.code)) reject();
      used.add(rowId); if (member.plotNumber.storage === 'text') nPlots++;
      matches.push(...byMember.get(rowId) ?? []);
    }
    if (u.nPlots !== nPlots) reject();
    for (const [i, row] of u.rows.entries()) {
      const definition = speciesAttributeDefinitions[i];
      if (!shape(row, ['field', 'count', 'plotOccurrences', 'categories']) || row.field !== definition.field ||
          row.count !== null && (!count(row.count) || row.count === 0) || !count(row.plotOccurrences) ||
          !Array.isArray(row.categories) || row.categories.length !== definition.categories.length) reject();
      const observations = matches.filter(m => m.values[i].storage === 'text');
      if (row.count !== (observations.length || null) || row.plotOccurrences !== new Set(observations.map(m => m.plotNumber)).size) reject();
      for (const [j, literal] of definition.categories.entries()) {
        const expected = observations.filter(m => m.values[i].text === literal).length;
        if (row.categories[j] !== (expected || null)) reject();
      }
    }
  }
  if (used.size !== memberships.size) reject();
  return structuredClone(value) as SpeciesAttributeSummaryPreview;
}

export function speciesAttributeCountText(value: number | null): string { return value === null ? 'NULL' : String(value); }
