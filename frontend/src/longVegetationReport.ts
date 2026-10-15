import type { EnvironmentReportName, LongVegetationDiagnostic, LongVegetationOptions, LongVegetationPlot, LongVegetationPreview, LongVegetationQualityOccurrence, LongVegetationQualityReference, LongVegetationQualitySelection, LongVegetationReport, LongVegetationRow, LongVegetationSelectedUnitOccurrence, LongVegetationSettings, LongVegetationUnit, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { reportTitleError } from './longEnvironmentReport';
import { wellFormedUTF16 } from './qualityEditor';
import { validateReportUnitNames } from './reportUnitNames';

type ValidatedVegetationRow = Omit<LongVegetationRow, 'plots'> & { plots: LongVegetationPlot[] };
type ValidatedVegetationUnit = Omit<LongVegetationUnit, 'membershipIds' | 'rows' | 'nameCandidates'> & {
  membershipIds: string[]; rows: ValidatedVegetationRow[]; nameCandidates: EnvironmentReportName[];
};
export type ValidatedLongVegetationPreview = Omit<LongVegetationPreview, 'report'> & {
  report: Omit<LongVegetationReport, 'units' | 'diagnostics' | 'quality'> & {
    units: ValidatedVegetationUnit[]; diagnostics: LongVegetationDiagnostic[];
    quality: (Omit<LongVegetationQualitySelection, 'thresholdRowIds' | 'occurrences' | 'excludedMembershipIds' | 'references'> & {
      thresholdRowIds: string[][]; occurrences: LongVegetationQualityOccurrence[]; excludedMembershipIds: string[];
      references: LongVegetationQualityReference[];
    }) | null;
  };
};

export function validateSettings(settings: LongVegetationSettings, selectedUnitsEnabled = false): void {
  if ((settings?.unitFields !== undefined || settings?.unitGrouping === 'selected-fields') && !selectedUnitsEnabled) {
    throw new Error('Selected report unit field selection is unavailable in this build; no source2 consumer was inferred.');
  }
  if (!settings || typeof settings.title !== 'string' || reportTitleError(settings.title) ||
      !['layer', 'none', 'lifeform', 'strata'].includes(settings.grouping) ||
      settings.unitGrouping !== undefined && !['all-plots', 'selected-fields'].includes(settings.unitGrouping) ||
      !['all-plots', 'observations'].includes(settings.average) ||
      !['species', 'presence'].includes(settings.order) ||
      typeof settings.constantSpeciesList !== 'boolean' || typeof settings.showEnglishName !== 'boolean' ||
      typeof settings.showSpeciesCode !== 'boolean' || settings.showEnglishName && settings.showSpeciesCode ||
      settings.selectedField !== undefined && (typeof settings.selectedField !== 'string' || !settings.selectedField ||
        !wellFormedUTF16(settings.selectedField) || settings.selectedField.includes('\0') ||
        settings.showEnglishName || settings.showSpeciesCode) ||
      typeof settings.presenceGreaterThan !== 'number' || !Number.isFinite(settings.presenceGreaterThan) ||
      typeof settings.meanCoverGreaterThan !== 'number' || !Number.isFinite(settings.meanCoverGreaterThan)) {
    throw new Error('Long Vegetation settings are incomplete or unsupported; no defaults were inferred.');
  }
  if (settings.unitGrouping === 'selected-fields') {
    if (!Array.isArray(settings.unitFields) || !settings.unitFields.length ||
        settings.unitFields.some(name => typeof name !== 'string' || !/^[A-Za-z0-9_]+$/.test(name)) ||
        new Set(settings.unitFields.map(name => name.toLowerCase())).size !== settings.unitFields.length) {
      throw new Error('Selected report units require explicit distinct ordered physical field identifiers.');
    }
  } else if (settings.unitFields !== undefined) {
    throw new Error('Ordinary report units invented selected physical fields.');
  }
  if (settings.quality !== null) {
    const quality = settings.quality;
    if (!quality || [quality.site, quality.veg, quality.soil].some(criterion =>
        !criterion || typeof criterion.minimum !== 'string' || !wellFormedUTF16(criterion.minimum) ||
        criterion.minimum.includes('\0') || typeof criterion.includeNull !== 'boolean')) {
      throw new Error('Long Vegetation plot-quality criteria are incomplete; no defaults were inferred.');
    }
  }
}

function identityCell(cell: ProjectMetadataCell): boolean {
  return completeCell(cell) && (cell.storage === 'null' || cell.storage === 'text');
}

function groupCell(cell: ProjectMetadataCell, grouping: string): boolean {
  if (grouping !== 'lifeform') return identityCell(cell);
  return completeCell(cell) && (cell.storage === 'null' ||
    cell.storage === 'integer' && typeof cell.integer === 'string' && exactSigned64(cell.integer) &&
      BigInt(cell.integer) >= -32768n && BigInt(cell.integer) <= 32767n);
}

export function requireLongVegetationGrouping(settings: LongVegetationSettings, lifeformEnabled: boolean, strataEnabled = false, codeEnabled = false, selectedFieldEnabled = false, noneUnitsEnabled = false, selectedUnitsEnabled = false): void {
  validateSettings(settings, selectedUnitsEnabled);
  if (settings.unitGrouping === 'all-plots' && !noneUnitsEnabled) {
    throw new Error('Long Vegetation None report units are disabled in this build; retained LVUnitGroups=3 was not reset.');
  }
  if (settings.grouping === 'lifeform' && !lifeformEnabled) {
    throw new Error('Long Vegetation Lifeform grouping is disabled in this build; retained LVGroupBy=3 was not reset.');
  }
  if (settings.grouping === 'strata' && !strataEnabled) {
    throw new Error('Long Vegetation Strata grouping is disabled in this build; retained LVGroupBy=2 was not reset.');
  }
  if (settings.showSpeciesCode && !codeEnabled) {
    throw new Error('Long Vegetation species Code display is disabled in this build; retained LVShowEnglishName=2 was not reset.');
  }
  if (settings.selectedField !== undefined && !selectedFieldEnabled) {
    throw new Error('Long Vegetation selected field display is disabled in this build; retained LVShowEnglishName=4 was not reset.');
  }
}

function selectedCell(cell: ProjectMetadataCell, kind: string | undefined): boolean {
  if (!completeCell(cell)) return false;
  if (cell.storage === 'null') return true;
  switch (kind) {
    case 'text': return cell.storage === 'text' && typeof cell.text === 'string' && !cell.text.includes('\0');
    case 'integer': return cell.storage === 'integer' && typeof cell.integer === 'string' && exactSigned64(cell.integer) &&
      BigInt(cell.integer) >= -2147483648n && BigInt(cell.integer) <= 2147483647n;
    case 'boolean': return cell.storage === 'integer' && (cell.integer === '-1' || cell.integer === '0');
    case 'real': return cell.storage === 'real' && typeof cell.real === 'number' && Number.isFinite(cell.real);
    default: return false;
  }
}

function finiteOrNull(value: number | null): boolean {
  return value === null || typeof value === 'number' && Number.isFinite(value);
}

function qualityTextEqual(left: string | null, right: string): boolean {
  return left !== null && (left === right || /^[\x00-\x7f]*$/.test(left + right) && left.toLowerCase() === right.toLowerCase());
}

function qualityThreshold(cell: ProjectMetadataCell): number {
  const value = cell.storage === 'integer' && typeof cell.integer === 'string' ? Number(BigInt(cell.integer)) :
    cell.storage === 'real' && typeof cell.real === 'number' ? cell.real : null;
  if (value === null || !Number.isInteger(value) || value < -32768 || value > 32767) {
    throw new Error('Plot-quality threshold lost its exact source Integer rank.');
  }
  return value;
}

function qualityRankAccepts(cell: ProjectMetadataCell, minimum: number, includeNull: boolean): boolean {
  if (cell.storage === 'null') return includeNull;
  if (cell.storage === 'integer' && typeof cell.integer === 'string') return BigInt(cell.integer) >= BigInt(minimum);
  if (cell.storage === 'real' && typeof cell.real === 'number' && Number.isFinite(cell.real)) return cell.real >= minimum;
  throw new Error('Qualified reference rank has unsupported historical storage.');
}

export function validateQualitySelection(value: LongVegetationPreview): Map<string, {count: number; unit: string}> {
  const qualified = new Map<string, {count: number; unit: string}>(), identities = new Map<string, string>();
  const selection = value.report.quality;
  if (value.settings.quality === null) {
    if (selection !== null) throw new Error('Unfiltered report invented plot-quality qualification.');
    return qualified;
  }
  if (!selection || !Array.isArray(selection.thresholdRowIds) || selection.thresholdRowIds.length !== 3 ||
      selection.thresholdRowIds.some(rows => !Array.isArray(rows) || !rows.length ||
        rows.some(id => !exactSigned64(id)) || new Set(rows).size !== rows.length) ||
      !Array.isArray(selection.occurrences) || !selection.occurrences.length ||
      !Array.isArray(selection.excludedMembershipIds) || !Array.isArray(selection.references) || !selection.references.length) {
    throw new Error('Plot-quality threshold or occurrence provenance is incomplete.');
  }
  const references = new Map<string, LongVegetationQualityReference>();
  for (const reference of selection.references) {
    if (!reference || !exactSigned64(reference.rowId) || references.has(reference.rowId) ||
        !identityCell(reference.item) || !identityCell(reference.listName) || !completeCell(reference.itemOrder)) {
      throw new Error('Plot-quality reference definition lost raw storage or physical identity.');
    }
    references.set(reference.rowId, reference);
  }
  const tuples = new Set<string>(), parents = new Map<string, string>();
  const envPlots = new Map<string, string>(), adminPlots = new Map<string, string>();
  const criteria = [value.settings.quality.site, value.settings.quality.veg, value.settings.quality.soil];
  const thresholds = selection.thresholdRowIds.map((ids, index) => {
    if (!Array.isArray(ids)) throw new Error('Missing quality threshold definitions.');
    const definitions = [...references.values()].filter(reference =>
      qualityTextEqual(reference.listName.text, 'DataQuality') &&
      qualityTextEqual(reference.item.text, criteria[index].minimum));
    const supplied = new Set(ids);
    if (definitions.length !== ids.length || definitions.some(reference => !supplied.has(reference.rowId))) {
      throw new Error('Quality minimum omitted matching original DataQuality definitions.');
    }
    let minimum: number | undefined;
    for (const reference of definitions) {
      const rank = qualityThreshold(reference.itemOrder);
      if (minimum !== undefined && minimum !== rank) throw new Error('Conflicting quality thresholds chose an arbitrary first definition.');
      minimum = rank;
    }
    if (minimum === undefined) throw new Error('Missing quality minimum rank.');
    return minimum;
  });
  const expectedCounts = new Map<string, number>();
  for (const occurrence of selection.occurrences) {
    if (!occurrence || !exactSigned64(occurrence.membershipId) || !exactSigned64(occurrence.envRowId) ||
        !exactSigned64(occurrence.adminRowId) || !identityCell(occurrence.plotNumber) ||
        occurrence.plotNumber.storage !== 'text' || !identityCell(occurrence.siteUnit) ||
        !Array.isArray(occurrence.listRowIds) || occurrence.listRowIds.length !== 3 ||
        occurrence.listRowIds.some((id, index) => id === null ? !criteria[index].includeNull : !exactSigned64(id)) ||
        !Array.isArray(occurrence.values) || occurrence.values.length !== 3 || !occurrence.values.every(identityCell)) {
      throw new Error('Plot-quality occurrence lost physical identities or invented a NULL join.');
    }
    let combinations = 1;
    for (let i = 0; i < 3; i++) {
      const code = occurrence.values[i], criterion = criteria[i], literal = code.text;
      const matches = literal === null ? [] : selection.references.filter(reference =>
        qualityTextEqual(reference.item.text, literal));
      const accepted: (string | null)[] = matches.length === 0 ? (criterion.includeNull ? [null] : []) : matches.filter(reference =>
        (reference.listName.storage === 'null' || qualityTextEqual(reference.listName.text, 'DataQuality')) &&
        qualityRankAccepts(reference.itemOrder, thresholds[i], criterion.includeNull)).map(reference => reference.rowId);
      if (!accepted.includes(occurrence.listRowIds[i])) throw new Error('Quality occurrence contradicts the original rank/domain/NULL predicates.');
      combinations *= accepted.length;
    }
    if (!Number.isSafeInteger(combinations) || combinations <= 0) throw new Error('Incomplete quality join multiplicity.');
    expectedCounts.set(occurrence.membershipId, combinations);
    const unit = JSON.stringify([occurrence.siteUnit.storage, occurrence.siteUnit.text]);
    const parent = JSON.stringify([occurrence.envRowId, occurrence.adminRowId]);
    const plot = JSON.stringify([occurrence.plotNumber.storage, occurrence.plotNumber.text]);
    const values = occurrence.values.map(cell => [cell.storage, cell.text]);
    const observation = JSON.stringify([parent, values]);
    const identity = JSON.stringify([plot, unit, observation]);
    if (identities.has(occurrence.membershipId) && identities.get(occurrence.membershipId) !== identity ||
        parents.has(plot) && parents.get(plot) !== observation ||
        envPlots.has(occurrence.envRowId) && envPlots.get(occurrence.envRowId) !== plot ||
        adminPlots.has(occurrence.adminRowId) && adminPlots.get(occurrence.adminRowId) !== plot) {
      throw new Error('Plot-quality occurrences changed an original membership or physical parent identity.');
    }
    const tuple = JSON.stringify([occurrence.membershipId, parent, occurrence.listRowIds]);
    if (tuples.has(tuple)) throw new Error('Plot-quality physical reference tuple is repeated.');
    tuples.add(tuple); identities.set(occurrence.membershipId, identity); parents.set(plot, observation);
    envPlots.set(occurrence.envRowId, plot); adminPlots.set(occurrence.adminRowId, plot);
    qualified.set(occurrence.membershipId, {unit, count: (qualified.get(occurrence.membershipId)?.count ?? 0) + 1});
  }
  for (const [id, source] of qualified) {
    if (source.count !== expectedCounts.get(id)) throw new Error('Qualified physical membership lost complete Cartesian reference fanout.');
  }
  const excluded = new Set<string>();
  for (const id of selection.excludedMembershipIds) {
    if (!exactSigned64(id) || excluded.has(id) || qualified.has(id)) {
      throw new Error('Excluded plot-quality membership is repeated, incomplete or also qualified.');
    }
    excluded.add(id);
  }
  return qualified;
}

function validateNoneUnitSelection(value: LongVegetationPreview): Map<string, number> {
  const counts = new Map<string, number>(), report = value.report;
  if (value.settings.unitGrouping !== 'all-plots') {
    if (report.noneUnits !== undefined) throw new Error('Site-unit-table report invented None unit derivation.');
    return counts;
  }
  if (!Array.isArray(report.noneUnits) || !report.noneUnits.length) {
    throw new Error('None report units lack explicit derived Env/current-list join evidence.');
  }
  const environments = new Map<string, string>(), members = new Map<string, string>(), tuples = new Set<string>();
  const literals = new Map<string, string>();
  for (const occurrence of report.noneUnits) {
    if (!occurrence || !exactSigned64(occurrence.envRowId) || !exactSigned64(occurrence.membershipId) ||
        !identityCell(occurrence.plotNumber) || occurrence.plotNumber.storage !== 'text' ||
        !/^[\x01-\x7f]*$/.test(occurrence.plotNumber.text!) ||
        !identityCell(occurrence.siteUnit) || occurrence.siteUnit.storage !== 'text' || occurrence.siteUnit.text !== 'All plots') {
      throw new Error('None unit derivation lost calibrated physical identities or the literal All plots unit.');
    }
    const plot = occurrence.plotNumber.text!;
    const folded = plot.toLowerCase();
    if (literals.has(folded) && literals.get(folded) !== plot) {
      throw new Error('None unit derivation has unverified distinct case-equivalent plot identities.');
    }
    literals.set(folded, plot);
    if (environments.has(occurrence.envRowId) && environments.get(occurrence.envRowId) !== plot ||
        members.has(occurrence.membershipId) && members.get(occurrence.membershipId) !== plot) {
      throw new Error('None unit derivation changed an original physical Env/SU plot identity.');
    }
    if (report.quality === null) {
      if (occurrence.qualityOccurrence !== null) throw new Error('Unfiltered None unit invented a quality link.');
    } else {
      const index = occurrence.qualityOccurrence;
      if (!Number.isSafeInteger(index) || index === null || index < 0 || index >= report.quality!.occurrences!.length) {
        throw new Error('None unit derivation lost its original qualified occurrence index.');
      }
      const original = report.quality!.occurrences![index];
      if (original.membershipId !== occurrence.membershipId || original.plotNumber.text !== plot) {
        throw new Error('None unit derivation differs from its original qualified membership.');
      }
    }
    const tuple = JSON.stringify([occurrence.envRowId, occurrence.membershipId, occurrence.qualityOccurrence]);
    if (tuples.has(tuple)) throw new Error('None unit derivation repeated the same physical join witness.');
    tuples.add(tuple); environments.set(occurrence.envRowId, plot); members.set(occurrence.membershipId, plot);
    counts.set(occurrence.membershipId, (counts.get(occurrence.membershipId) ?? 0) + 1);
  }
  const envCounts = new Map<string, number>();
  for (const plot of environments.values()) envCounts.set(plot, (envCounts.get(plot) ?? 0) + 1);
  let expected = 0;
  if (report.quality === null) {
    for (const plot of members.values()) expected += envCounts.get(plot)!;
  } else {
    for (const occurrence of report.quality!.occurrences!) {
      if (environments.get(occurrence.envRowId) !== occurrence.plotNumber.text) {
        throw new Error('Qualified None unit derivation omitted its original Env witness.');
      }
      expected += envCounts.get(occurrence.plotNumber.text!)!;
    }
  }
  if (!Number.isSafeInteger(expected) || expected !== tuples.size) {
    throw new Error('None unit derivation lost the complete Env/current-list occurrence join.');
  }
  return counts;
}

function validateSelectedUnitSelection(value: LongVegetationPreview): {
  counts: Map<string, number>; units: Map<string, { count: number; members: Set<string> }>;
} {
  const report = value.report;
  if (value.settings.unitGrouping !== 'selected-fields') {
    if (report.selectedUnits !== undefined) throw new Error('Ordinary report units invented selected field derivation.');
    return { counts: new Map(), units: new Map() };
  }
  return validateSelectedUnitOccurrences(report.selectedUnits, report.quality);
}

export function validateSelectedUnitOccurrences(occurrences: LongVegetationSelectedUnitOccurrence[] | null | undefined,
  quality: LongVegetationQualitySelection | null = null): {
  counts: Map<string, number>; units: Map<string, { count: number; members: Set<string> }>;
} {
  const counts = new Map<string, number>(), units = new Map<string, { count: number; members: Set<string> }>();
  if (!Array.isArray(occurrences) || !occurrences.length || occurrences.length > 1048572) {
    throw new Error('Selected report units lack bounded complete original SU/Env/Admin witnesses.');
  }
  const members = new Map<string, string>(), env = new Map<string, string>(), admin = new Map<string, string>();
  const parents = new Map<string, string>(), tuples = new Set<string>(), literals = new Map<string, string>();
  const plots = new Map<string, { members: Set<string>; env: Set<string>; admin: Set<string>; count: number; unmatched: boolean }>();
  const destination = (cell: ProjectMetadataCell, bound: number) => identityCell(cell) && Object.keys(cell).length === 5 &&
    ['storage', 'text', 'integer', 'real', 'blobHex'].every(key => Object.hasOwn(cell, key)) &&
    (cell.storage === 'null' || typeof cell.text === 'string' && cell.text.length > 0 &&
      cell.text.length <= bound && wellFormedUTF16(cell.text) && !cell.text.includes('\0'));
  for (const row of occurrences) {
    if (!row || Object.keys(row).length !== 5 ||
        !['membershipId', 'envRowId', 'adminRowId', 'plotNumber', 'siteUnit'].every(key => Object.hasOwn(row, key)) ||
        !exactSigned64(row.membershipId) || !destination(row.plotNumber, 7) || !destination(row.siteUnit, 255) ||
        row.plotNumber.storage === 'text' && !/^[\x01-\x7f]+$/.test(row.plotNumber.text!) ||
        (row.envRowId === null) !== (row.adminRowId === null) ||
        row.envRowId === null && row.siteUnit.storage !== 'null' ||
        row.envRowId !== null && (!exactSigned64(row.envRowId) || row.adminRowId === null ||
          !exactSigned64(row.adminRowId) || row.plotNumber.storage !== 'text')) {
      throw new Error('Selected report unit destination or nullable physical join identity is unsupported.');
    }
    const plot = JSON.stringify([row.plotNumber.storage, row.plotNumber.text]);
    const unit = JSON.stringify([row.siteUnit.storage, row.siteUnit.text]);
    if (row.plotNumber.storage === 'text') {
      const folded = row.plotNumber.text!.toLowerCase();
      if (literals.has(folded) && literals.get(folded) !== row.plotNumber.text) {
        throw new Error('Selected report units changed calibrated literal plot ownership.');
      }
      literals.set(folded, row.plotNumber.text!);
    }
    if (members.has(row.membershipId) && members.get(row.membershipId) !== plot) {
      throw new Error('Selected report units changed an original membership plot.');
    }
    members.set(row.membershipId, plot);
    const tuple = JSON.stringify([row.membershipId, row.envRowId, row.adminRowId]);
    if (tuples.has(tuple)) throw new Error('Selected report units repeat an original physical join tuple.');
    tuples.add(tuple);
    const group = plots.get(plot) ?? { members: new Set<string>(), env: new Set<string>(), admin: new Set<string>(), count: 0, unmatched: false };
    group.members.add(row.membershipId); group.count++; plots.set(plot, group);
    if (row.envRowId === null) group.unmatched = true;
    if (row.envRowId !== null && row.adminRowId !== null) {
      if (env.has(row.envRowId) && env.get(row.envRowId) !== plot ||
          admin.has(row.adminRowId) && admin.get(row.adminRowId) !== plot) {
        throw new Error('Selected report units changed a physical parent plot.');
      }
      env.set(row.envRowId, plot); admin.set(row.adminRowId, plot);
      group.env.add(row.envRowId); group.admin.add(row.adminRowId);
      const pair = JSON.stringify([row.envRowId, row.adminRowId]);
      if (parents.has(pair) && parents.get(pair) !== unit) {
        throw new Error('Selected report units changed the same parent field projection across memberships.');
      }
      parents.set(pair, unit);
    }
    counts.set(row.membershipId, (counts.get(row.membershipId) ?? 0) + 1);
    const derived = units.get(unit) ?? { count: 0, members: new Set<string>() };
    if (row.plotNumber.storage !== 'null') derived.count++;
    derived.members.add(row.membershipId); units.set(unit, derived);
  }
  for (const group of plots.values()) {
    const expected = group.members.size * (group.env.size ? group.env.size * group.admin.size : 1);
    if (group.unmatched && group.env.size > 0 || !Number.isSafeInteger(expected) || expected !== group.count) {
      throw new Error('Selected report units lost the complete original SU/Env/Admin Cartesian join.');
    }
  }
  if (quality !== null) {
    const qualifiedPlots = new Set(quality.occurrences!.map(row => JSON.stringify([row.plotNumber.storage, row.plotNumber.text])));
    if ([...plots.keys()].some(plot => !qualifiedPlots.has(plot))) {
      throw new Error('Selected report units retained a plot outside original quality existence.');
    }
    for (const row of quality.occurrences!) {
      if (members.get(row.membershipId) !== JSON.stringify([row.plotNumber.storage, row.plotNumber.text]) ||
          !parents.has(JSON.stringify([row.envRowId, row.adminRowId]))) {
        throw new Error('Selected report units omitted original qualified membership/parent witnesses.');
      }
    }
  }
  return { counts, units };
}

export function validateVegetationRows(unit: LongVegetationUnit, settings: LongVegetationSettings,
  selectedFieldType: string | undefined): asserts unit is LongVegetationUnit & {
    rows: ValidatedVegetationRow[]; nameCandidates: EnvironmentReportName[];
  } {
  validateReportUnitNames(unit, unit.code.storage === 'null');
  if (!Array.isArray(unit.rows)) throw new Error('Long Vegetation unit rows are incomplete.');
  let columns: string[] | undefined;
  for (const row of unit.rows) {
    if (!row || !groupCell(row.layer, settings.grouping) ||
        !identityCell(row.species) ||
        (settings.selectedField !== undefined ?
          ![row.englishName, row.matchedName].every(cell => selectedCell(cell, selectedFieldType)) :
          ![row.englishName, row.matchedName].every(identityCell)) ||
        !finiteOrNull(row.presence) || !finiteOrNull(row.meanCover) ||
        (row.presence === null) !== (row.meanCover === null) || !Array.isArray(row.plots) ||
        !settings.showEnglishName && !settings.showSpeciesCode && settings.selectedField === undefined &&
          (row.englishName.storage !== 'null' || row.matchedName.storage !== 'null')) {
      throw new Error('Long Vegetation row lost typed grouping/identity, names or finite statistics.');
    }
    const plots = new Set<string>();
    for (const plot of row.plots) {
      if (!plot || typeof plot.plotNumber !== 'string' || !wellFormedUTF16(plot.plotNumber) ||
          plots.has(plot.plotNumber) || !finiteOrNull(plot.cover) ||
          row.presence === null && plot.cover !== null) {
        throw new Error('Long Vegetation pivot is incomplete, repeated or invents an absent observation.');
      }
      plots.add(plot.plotNumber);
    }
    const names = row.plots.map(plot => plot.plotNumber);
    if (columns && (columns.length !== names.length || columns.some((name, index) => name !== names[index]))) {
      throw new Error('Long Vegetation pivot columns differ within the same unit.');
    }
    columns = names;
  }
}

export function validateLongVegetationOptions(value: LongVegetationOptions, contextId: string,
  project: string, projectPath: string, su: string, suPath: string, selectedUnitsEnabled = false): LongVegetationOptions {
  if (!value || !contextId || !project || !projectPath || !su || su === 'None' || !suPath ||
      value.contextId !== contextId || value.project !== project || value.projectPath !== projectPath ||
      value.su !== su || value.suPath !== suPath) {
    throw new Error('Long Vegetation options belong to a different or incomplete selected-SU context.');
  }
  validateSettings(value.settings, selectedUnitsEnabled);
  return value;
}

export function validateLongVegetationPreview(value: LongVegetationPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string, selectedUnitsEnabled = false): ValidatedLongVegetationPreview {
  assertLongVegetationPreview(value, contextId, project, projectPath, su, suPath, selectedUnitsEnabled);
  return value;
}

function assertLongVegetationPreview(value: LongVegetationPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string, selectedUnitsEnabled: boolean): asserts value is ValidatedLongVegetationPreview {
  const report = value?.report;
  if (report?.sourceKind !== undefined || report?.projectUnits !== undefined ||
      report?.excludedProjectUnitIndexes !== undefined ||
      report?.units?.some(unit => unit?.projectUnitIndexes !== undefined)) {
    throw new Error('Whole-project report provenance is unavailable in this selected-list consumer; no physical SU identities were inferred.');
  }
  if (!value || !contextId || !project || !projectPath || !su || su === 'None' || !suPath ||
      value.contextId !== contextId || value.projectPath !== projectPath || value.suPath !== suPath ||
      !report || report.project !== project || report.su !== su ||
      !Array.isArray(report.units) || !Array.isArray(report.diagnostics)) {
    throw new Error('Long Vegetation preview belongs to a different or incomplete selected-SU context.');
  }
  validateSettings(value.settings, selectedUnitsEnabled);
  if (report.selectedUnits !== undefined && !selectedUnitsEnabled) {
    throw new Error('Selected report unit derivation is unavailable in this build; no source2 consumer was inferred.');
  }
  if (report.title !== value.settings.title) throw new Error('Long Vegetation title differs from the settings used to calculate it.');
  if (value.settings.selectedField !== undefined ?
      !['text', 'integer', 'boolean', 'real'].includes(report.selectedFieldType ?? '') :
      report.selectedFieldType !== undefined) {
    throw new Error('Long Vegetation selected field lost its explicit typed scalar provenance.');
  }
  const qualified = validateQualitySelection(value), qualityEnforced = value.settings.quality !== null;
  const noneCounts = validateNoneUnitSelection(value), noneUnits = value.settings.unitGrouping === 'all-plots';
  const selected = validateSelectedUnitSelection(value), selectedUnits = value.settings.unitGrouping === 'selected-fields';
  if (noneUnits && report.units.length > 1) throw new Error('None report units must retain only the literal All plots unit.');
  const units = new Set<string>(), memberships = new Set<string>();
  for (const unit of report.units) {
    if (!unit || !identityCell(unit.code) || !Number.isSafeInteger(unit.numPlots) || unit.numPlots <= 0 ||
        !Array.isArray(unit.membershipIds) || !unit.membershipIds.length ||
        !qualityEnforced && !noneUnits && !selectedUnits && unit.membershipIds.length < unit.numPlots || !Array.isArray(unit.rows)) {
      throw new Error('Long Vegetation unit lost typed identity or physical membership denominator.');
    }
    const identity = JSON.stringify([unit.code.storage, unit.code.text]);
    if (units.has(identity) || !qualityEnforced && !noneUnits && !selectedUnits && unit.code.storage === 'text' && unit.numPlots !== unit.membershipIds.length) {
      throw new Error('Long Vegetation unit is repeated or its named-unit denominator differs.');
    }
    units.add(identity);
    const unitMembers = new Set<string>();
    for (const id of unit.membershipIds) {
      if (!exactSigned64(id) || unitMembers.has(id) || !selectedUnits && memberships.has(id)) throw new Error('Long Vegetation physical membership identity is missing or repeated.');
      unitMembers.add(id);
      memberships.add(id);
    }
    if (selectedUnits) {
      const derived = selected.units.get(identity);
      if (!derived || unit.numPlots !== derived.count || unitMembers.size !== derived.members.size ||
          unit.membershipIds.some(id => !derived.members.has(id))) {
        throw new Error('Selected unit denominator differs from complete derived joins or unique original membership IDs.');
      }
    } else if (noneUnits) {
      if (unit.code.storage !== 'text' || unit.code.text !== 'All plots' || unit.membershipIds.length !== noneCounts.size ||
          unit.membershipIds.some(id => !noneCounts.has(id)) || unit.numPlots !== report.noneUnits!.length) {
        throw new Error('None unit denominator differs from its derived joins or original physical membership IDs.');
      }
    } else if (qualityEnforced) {
      let denominator = 0;
      for (const id of unit.membershipIds) {
        const source = qualified.get(id);
        if (!source || source.unit !== identity) throw new Error('Qualified unit changed or omitted a physical membership identity.');
        denominator += source.count;
      }
      if (denominator !== unit.numPlots) throw new Error('Qualified denominator differs from its physical reference occurrences.');
    }
    validateVegetationRows(unit, value.settings, report.selectedFieldType);
  }
  const qualityDiagnostics = new Set<string>();
  const noneDiagnostics = new Set<string>();
  const selectedDiagnostics = new Set<string>();
  const selectedExclusions = new Set<string>();
  for (const diagnostic of report.diagnostics) {
    if (!diagnostic || !['physical_membership_multiplicity', 'missing_master_species', 'master_species_multiplicity',
          'missing_layer_label', 'layer_label_multiplicity', 'excluded_unit_without_vegetation',
          'constant_list_name_fanout', 'unit_name_missing', 'unit_name_conflicting', 'unit_name_unsupported_storage',
          'quality_reference_multiplicity', 'none_unit_join_multiplicity', 'selected_unit_join_multiplicity'].includes(diagnostic.code) ||
        typeof diagnostic.identity !== 'string' || !wellFormedUTF16(diagnostic.identity) ||
        !Number.isSafeInteger(diagnostic.count) || diagnostic.count < 0 ||
        diagnostic.count === 0 && diagnostic.code !== 'unit_name_missing') {
      throw new Error('Long Vegetation diagnostic lost its source identity or multiplicity count.');
    }
    if (diagnostic.code === 'quality_reference_multiplicity' &&
        (!qualityEnforced || diagnostic.count <= 1 || qualified.get(diagnostic.identity)?.count !== diagnostic.count ||
          qualityDiagnostics.has(diagnostic.identity))) {
      throw new Error('Quality multiplicity diagnostic differs from original reference occurrences.');
    }
    if (diagnostic.code === 'quality_reference_multiplicity') qualityDiagnostics.add(diagnostic.identity);
    if (diagnostic.code === 'none_unit_join_multiplicity') {
      if (!noneUnits || diagnostic.count <= 1 || noneCounts.get(diagnostic.identity) !== diagnostic.count ||
          noneDiagnostics.has(diagnostic.identity)) {
        throw new Error('None join multiplicity diagnostic differs from its derived occurrences.');
      }
      noneDiagnostics.add(diagnostic.identity);
    }
    if (diagnostic.code === 'selected_unit_join_multiplicity') {
      if (!selectedUnits || diagnostic.count <= 1 || selected.counts.get(diagnostic.identity) !== diagnostic.count ||
          selectedDiagnostics.has(diagnostic.identity)) {
        throw new Error('Selected unit multiplicity diagnostic differs from its complete derived joins.');
      }
      selectedDiagnostics.add(diagnostic.identity);
    }
    if (selectedUnits && diagnostic.code === 'excluded_unit_without_vegetation') {
      const key = diagnostic.identity === 'null:' ? '["null",null]' :
        diagnostic.identity.startsWith('text:') ? JSON.stringify(['text', diagnostic.identity.slice(5)]) : '';
      const entry = selected.units.get(key);
      if (!entry || units.has(key) || diagnostic.count !== entry.members.size ||
          selectedExclusions.has(key) || key === '["null",null]' && entry.count > 0) {
        throw new Error('Selected unit exclusion differs from complete derived unit evidence.');
      }
      selectedExclusions.add(key);
    }
  }
  for (const [id, source] of qualified) {
    if (source.count > 1 && !qualityDiagnostics.has(id)) {
      throw new Error('Qualified reference fanout lost its multiplicity diagnostic.');
    }
  }
  for (const [id, count] of noneCounts) {
    if (count > 1 && !noneDiagnostics.has(id)) throw new Error('None join fanout lost its explicit multiplicity diagnostic.');
  }
  for (const [id, count] of selected.counts) {
    if (count > 1 && !selectedDiagnostics.has(id)) throw new Error('Selected unit fanout lost its explicit multiplicity diagnostic.');
  }
  for (const key of selected.units.keys()) {
    if (!units.has(key) && !selectedExclusions.has(key)) throw new Error('Selected report omitted a derived unit without source exclusion evidence.');
  }
}
