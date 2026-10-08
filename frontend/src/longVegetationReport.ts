import type { EnvironmentReportName, LongVegetationDiagnostic, LongVegetationOptions, LongVegetationPlot, LongVegetationPreview, LongVegetationQualityOccurrence, LongVegetationQualityReference, LongVegetationQualitySelection, LongVegetationReport, LongVegetationRow, LongVegetationSettings, LongVegetationUnit, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
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

function validateSettings(settings: LongVegetationSettings): void {
  if (!settings || typeof settings.title !== 'string' || reportTitleError(settings.title) ||
      !['all-plots', 'observations'].includes(settings.average) ||
      !['species', 'presence'].includes(settings.order) ||
      typeof settings.constantSpeciesList !== 'boolean' || typeof settings.showEnglishName !== 'boolean' ||
      typeof settings.presenceGreaterThan !== 'number' || !Number.isFinite(settings.presenceGreaterThan) ||
      typeof settings.meanCoverGreaterThan !== 'number' || !Number.isFinite(settings.meanCoverGreaterThan)) {
    throw new Error('Long Vegetation settings are incomplete or unsupported; no defaults were inferred.');
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

function validateQualitySelection(value: LongVegetationPreview): Map<string, {count: number; unit: string}> {
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

export function validateLongVegetationOptions(value: LongVegetationOptions, contextId: string,
  project: string, projectPath: string, su: string, suPath: string): LongVegetationOptions {
  if (!value || !contextId || !project || !projectPath || !su || su === 'None' || !suPath ||
      value.contextId !== contextId || value.project !== project || value.projectPath !== projectPath ||
      value.su !== su || value.suPath !== suPath) {
    throw new Error('Long Vegetation options belong to a different or incomplete selected-SU context.');
  }
  validateSettings(value.settings);
  return value;
}

export function validateLongVegetationPreview(value: LongVegetationPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string): ValidatedLongVegetationPreview {
  assertLongVegetationPreview(value, contextId, project, projectPath, su, suPath);
  return value;
}

function assertLongVegetationPreview(value: LongVegetationPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string): asserts value is ValidatedLongVegetationPreview {
  const report = value?.report;
  if (!value || !contextId || !project || !projectPath || !su || su === 'None' || !suPath ||
      value.contextId !== contextId || value.projectPath !== projectPath || value.suPath !== suPath ||
      !report || report.project !== project || report.su !== su ||
      !Array.isArray(report.units) || !Array.isArray(report.diagnostics)) {
    throw new Error('Long Vegetation preview belongs to a different or incomplete selected-SU context.');
  }
  validateSettings(value.settings);
  if (report.title !== value.settings.title) throw new Error('Long Vegetation title differs from the settings used to calculate it.');
  const qualified = validateQualitySelection(value), qualityEnforced = value.settings.quality !== null;
  const units = new Set<string>(), memberships = new Set<string>();
  for (const unit of report.units) {
    if (!unit || !identityCell(unit.code) || !Number.isSafeInteger(unit.numPlots) || unit.numPlots <= 0 ||
        !Array.isArray(unit.membershipIds) || !unit.membershipIds.length ||
        !qualityEnforced && unit.membershipIds.length < unit.numPlots || !Array.isArray(unit.rows)) {
      throw new Error('Long Vegetation unit lost typed identity or physical membership denominator.');
    }
    const identity = JSON.stringify([unit.code.storage, unit.code.text]);
    if (units.has(identity) || !qualityEnforced && unit.code.storage === 'text' && unit.numPlots !== unit.membershipIds.length) {
      throw new Error('Long Vegetation unit is repeated or its named-unit denominator differs.');
    }
    units.add(identity);
    validateReportUnitNames(unit, unit.code.storage === 'null');
    for (const id of unit.membershipIds) {
      if (!exactSigned64(id) || memberships.has(id)) throw new Error('Long Vegetation physical membership identity is missing or repeated.');
      memberships.add(id);
    }
    if (qualityEnforced) {
      let denominator = 0;
      for (const id of unit.membershipIds) {
        const source = qualified.get(id);
        if (!source || source.unit !== identity) throw new Error('Qualified unit changed or omitted a physical membership identity.');
        denominator += source.count;
      }
      if (denominator !== unit.numPlots) throw new Error('Qualified denominator differs from its physical reference occurrences.');
    }
    let columns: string[] | undefined;
    for (const row of unit.rows) {
      if (!row || ![row.layer, row.species, row.englishName, row.matchedName].every(identityCell) ||
          !finiteOrNull(row.presence) || !finiteOrNull(row.meanCover) ||
          (row.presence === null) !== (row.meanCover === null) || !Array.isArray(row.plots) ||
          !value.settings.showEnglishName && (row.englishName.storage !== 'null' || row.matchedName.storage !== 'null')) {
        throw new Error('Long Vegetation row lost NULL/text identities, names or finite statistics.');
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
  const qualityDiagnostics = new Set<string>();
  for (const diagnostic of report.diagnostics) {
    if (!diagnostic || !['physical_membership_multiplicity', 'missing_master_species', 'master_species_multiplicity',
          'missing_layer_label', 'layer_label_multiplicity', 'excluded_unit_without_vegetation',
          'constant_list_name_fanout', 'unit_name_missing', 'unit_name_conflicting', 'unit_name_unsupported_storage',
          'quality_reference_multiplicity'].includes(diagnostic.code) ||
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
  }
  for (const [id, source] of qualified) {
    if (source.count > 1 && !qualityDiagnostics.has(id)) {
      throw new Error('Qualified reference fanout lost its multiplicity diagnostic.');
    }
  }
}
