import type { LongVegetationPreview, LongVegetationProjectUnit, LongVegetationSettings, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import type { ValidatedLongVegetationPreview } from './longVegetationReport';
import { validateQualitySelection, validateSettings, validateVegetationRows } from './longVegetationReport';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';

export type ProjectReportSource = 'project-selected-fields' | 'project-all-plots';
export type ProjectReportOwner = {
  contextId: string; project: string; projectPath: string; su: string; suPath: string;
};
export type ValidatedProjectPreview = Omit<ValidatedLongVegetationPreview, 'report'> & {
  report: ValidatedLongVegetationPreview['report'] & {
    sourceKind: ProjectReportSource; projectUnits: LongVegetationProjectUnit[];
  };
};

const text = (value: unknown): value is string =>
  typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
const identity = (cell: ProjectMetadataCell): boolean =>
  completeCell(cell) && (cell.storage === 'null' || cell.storage === 'text');
const key = (cell: ProjectMetadataCell): string => cell.storage === 'null' ? 'null:' : `text:${cell.text}`;

export function projectReportOwnedScope(value: Partial<ProjectReportOwner> | null | undefined, owner: ProjectReportOwner): boolean {
  return !!value && text(owner.contextId) && !!owner.contextId && text(owner.project) && !!owner.project &&
    text(owner.projectPath) && !!owner.projectPath && text(owner.su) && !!owner.su && text(owner.suPath) &&
    (owner.su === 'None' ? owner.suPath === '' : owner.suPath !== '') &&
    value.contextId === owner.contextId && value.project === owner.project && value.projectPath === owner.projectPath &&
    value.su === owner.su && value.suPath === owner.suPath;
}

export function validateProjectReportSettings(settings: LongVegetationSettings, owner: ProjectReportOwner,
  source: ProjectReportSource, fields: string[]): void {
  validateSettings(settings, true);
  if (source !== 'project-selected-fields' && source !== 'project-all-plots' ||
      (source === 'project-selected-fields' ?
        settings.unitGrouping !== 'selected-fields' || !Array.isArray(settings.unitFields) ||
          fields.length !== settings.unitFields.length || fields.some((field, i) => field !== settings.unitFields![i]) :
        settings.unitGrouping !== 'all-plots' || settings.unitFields !== undefined || fields.length !== 0) ||
      settings.quality !== null && owner.su === 'None') {
    throw new Error('Project settings contradict original scope, field order or quality authority.');
  }
}

export function validateProjectUnitTuples(rows: LongVegetationProjectUnit[] | null | undefined,
  source: ProjectReportSource): Map<string, number> {
  if (source !== 'project-selected-fields' && source !== 'project-all-plots' ||
      !Array.isArray(rows) || !rows.length || rows.length > 1048572) {
    throw new Error('Project report requires bounded original project tuples.');
  }
  const tuples = new Set<string>(), literals = new Map<string, string>();
  const envPlots = new Map<string, string>(), adminPlots = new Map<string, string>();
  const groups = new Map<string, { env: Set<string>; admin: Set<string>; count: number }>();
  const counts = new Map<string, number>();
  for (const row of rows) {
    if (!row || Object.keys(row).length !== 4 ||
        !['envRowId', 'adminRowId', 'plotNumber', 'siteUnit'].every(name => Object.hasOwn(row, name)) ||
        !exactSigned64(row.envRowId) || !identity(row.plotNumber) || !identity(row.siteUnit) ||
        row.siteUnit.storage === 'text' && (!text(row.siteUnit.text) || row.siteUnit.text.length > 255) ||
        row.plotNumber.storage === 'text' && (!text(row.plotNumber.text) || !/^[\x01-\x7f]*$/.test(row.plotNumber.text)) ||
        (source === 'project-selected-fields' ?
          row.adminRowId === null || !exactSigned64(row.adminRowId) || row.plotNumber.storage !== 'text' :
          row.adminRowId !== null || row.siteUnit.storage !== 'text' || row.siteUnit.text !== 'All plots')) {
      throw new Error('Project tuple lost original physical IDs, source kind or literal cells.');
    }
    const tuple = JSON.stringify([row.envRowId, row.adminRowId]), plot = key(row.plotNumber);
    if (tuples.has(tuple)) throw new Error('Project tuples repeat original physical ownership.');
    tuples.add(tuple);
    if (envPlots.has(row.envRowId) && envPlots.get(row.envRowId) !== plot ||
        row.adminRowId !== null && adminPlots.has(row.adminRowId) && adminPlots.get(row.adminRowId) !== plot) {
      throw new Error('Project tuple changed its original physical parent plot.');
    }
    envPlots.set(row.envRowId, plot);
    if (row.adminRowId !== null) adminPlots.set(row.adminRowId, plot);
    if (row.plotNumber.storage === 'text') {
      const literal = row.plotNumber.text!, folded = literal.toLowerCase();
      if (literals.has(folded) && literals.get(folded) !== literal) {
        throw new Error('Project report changed calibrated literal plot ownership.');
      }
      literals.set(folded, literal);
    }
    const group = groups.get(plot) ?? { env: new Set<string>(), admin: new Set<string>(), count: 0 };
    group.env.add(row.envRowId);
    if (row.adminRowId !== null) group.admin.add(row.adminRowId);
    group.count++; groups.set(plot, group);
    counts.set(key(row.siteUnit), (counts.get(key(row.siteUnit)) ?? 0) + 1);
  }
  for (const group of groups.values()) {
    const expected = source === 'project-selected-fields' ? group.env.size * group.admin.size : group.env.size;
    if (!Number.isSafeInteger(expected) || group.count !== expected) {
      throw new Error('Project report omitted original Env/Admin Cartesian tuples.');
    }
  }
  return counts;
}

export function validateLongVegetationProjectPreview(value: LongVegetationPreview, owner: ProjectReportOwner,
  source: ProjectReportSource, fields: string[], enabled = false): ValidatedProjectPreview {
  assertProjectPreview(value, owner, source, fields, enabled);
  return value;
}

function assertProjectPreview(value: LongVegetationPreview, owner: ProjectReportOwner,
  source: ProjectReportSource, fields: string[], enabled: boolean): asserts value is ValidatedProjectPreview {
  if (!enabled) throw new Error('Whole-project report consumer is disabled in this build.');
  if (source !== 'project-selected-fields' && source !== 'project-all-plots') {
    throw new Error('Project report requires an explicit source kind.');
  }
  const report = value?.report;
  if (!value || !projectReportOwnedScope({ contextId: value.contextId, projectPath: value.projectPath, suPath: value.suPath,
        project: report?.project, su: report?.su }, owner) ||
      !report || report.sourceKind !== source ||
      report.noneUnits !== undefined || report.selectedUnits !== undefined ||
      !Array.isArray(report.units) || !report.units.length || !Array.isArray(report.diagnostics) ||
      source === 'project-all-plots' && owner.su !== 'None') {
    throw new Error('Project preview belongs to a different or incomplete explicit source context.');
  }
  validateProjectReportSettings(value.settings, owner, source, fields);
  if (value.settings.title !== report.title ||
      (value.settings.selectedField !== undefined ?
        !['text', 'integer', 'boolean', 'real'].includes(report.selectedFieldType ?? '') :
        report.selectedFieldType !== undefined)) {
    throw new Error('Project preview settings contradict original scope, field order or quality authority.');
  }
  if (!Array.isArray(report.projectUnits)) throw new Error('Project report requires original tuples.');
  const counts = validateProjectUnitTuples(report.projectUnits, source);
  const qualified = validateQualitySelection(value);
  const rows = report.projectUnits;
  if (value.settings.quality !== null) {
    const plots = new Set(report.quality!.occurrences!.map(row => key(row.plotNumber)));
    const parents = new Set(rows.map(row => JSON.stringify([row.envRowId, row.adminRowId, key(row.plotNumber)])));
    if (rows.some(row => !plots.has(key(row.plotNumber))) ||
        report.quality!.occurrences!.some(row => !parents.has(JSON.stringify([row.envRowId, row.adminRowId, key(row.plotNumber)])))) {
      throw new Error('Project output contradicts original selected-SU quality existence witnesses.');
    }
  }
  const seen = new Set<number>(), emitted = new Set<string>();
  for (const unit of report.units) {
    if (!unit || !identity(unit.code) || !Number.isSafeInteger(unit.numPlots) || unit.numPlots <= 0 ||
        !Array.isArray(unit.membershipIds) || unit.membershipIds.length !== 0 ||
        !Array.isArray(unit.projectUnitIndexes) || unit.projectUnitIndexes.length !== unit.numPlots ||
        unit.numPlots !== counts.get(key(unit.code)) || emitted.has(key(unit.code))) {
      throw new Error('Project unit lost its original tuple denominator or invented physical SU membership IDs.');
    }
    emitted.add(key(unit.code));
    for (const index of unit.projectUnitIndexes) {
      if (!Number.isSafeInteger(index) || index < 0 || index >= rows.length || seen.has(index) ||
          key(rows[index].siteUnit) !== key(unit.code)) {
        throw new Error('Project unit index has repeated or mismatched original ownership.');
      }
      seen.add(index);
    }
    validateVegetationRows(unit, value.settings, report.selectedFieldType);
  }
  const excluded = new Map<string, number>(), multiplicities = new Map<string, number>(), diagnostics = new Set<string>();
  for (const row of rows) {
    if (row.plotNumber.storage === 'text') multiplicities.set(row.plotNumber.text!, (multiplicities.get(row.plotNumber.text!) ?? 0) + 1);
  }
  for (const diagnostic of report.diagnostics) {
    if (!diagnostic || !text(diagnostic.identity) ||
        !['missing_master_species', 'master_species_multiplicity', 'missing_layer_label', 'layer_label_multiplicity',
          'excluded_unit_without_vegetation', 'constant_list_name_fanout', 'unit_name_missing', 'unit_name_conflicting',
          'unit_name_unsupported_storage', 'quality_reference_multiplicity', 'project_unit_join_multiplicity'].includes(diagnostic.code) ||
        !Number.isSafeInteger(diagnostic.count) || diagnostic.count < 0 ||
        diagnostic.count === 0 && diagnostic.code !== 'unit_name_missing' ||
        diagnostics.has(JSON.stringify([diagnostic.code, diagnostic.identity]))) {
      throw new Error('Project diagnostic lost literal identity or original multiplicity.');
    }
    diagnostics.add(JSON.stringify([diagnostic.code, diagnostic.identity]));
    if (diagnostic.code === 'excluded_unit_without_vegetation') {
      if (emitted.has(diagnostic.identity) || counts.get(diagnostic.identity) !== diagnostic.count) {
        throw new Error('Project excluded-unit evidence contradicts original tuple ownership.');
      }
      excluded.set(diagnostic.identity, diagnostic.count);
    }
    if (diagnostic.code === 'project_unit_join_multiplicity' &&
        (diagnostic.count <= 1 || multiplicities.get(diagnostic.identity) !== diagnostic.count) ||
        diagnostic.code === 'quality_reference_multiplicity' &&
        (diagnostic.count <= 1 || qualified.get(diagnostic.identity)?.count !== diagnostic.count)) {
      throw new Error('Project multiplicity diagnostic contradicts original tuple or quality witnesses.');
    }
  }
  for (const [plot, count] of multiplicities) {
    if (count > 1 && !diagnostics.has(JSON.stringify(['project_unit_join_multiplicity', plot]))) {
      throw new Error('Project report omitted original project join multiplicity evidence.');
    }
  }
  for (const [id, entry] of qualified) {
    if (entry.count > 1 && !diagnostics.has(JSON.stringify(['quality_reference_multiplicity', id]))) {
      throw new Error('Project report omitted original selected-SU reference multiplicity evidence.');
    }
  }
  if (report.excludedProjectUnitIndexes !== undefined && !Array.isArray(report.excludedProjectUnitIndexes)) {
    throw new Error('Project excluded tuple indexes are incomplete.');
  }
  const excludedCounts = new Map<string, number>();
  for (const index of report.excludedProjectUnitIndexes ?? []) {
    if (!Number.isSafeInteger(index) || index < 0 || index >= rows.length || seen.has(index) || emitted.has(key(rows[index].siteUnit))) {
      throw new Error('Project excluded index has repeated or emitted ownership.');
    }
    seen.add(index);
    excludedCounts.set(key(rows[index].siteUnit), (excludedCounts.get(key(rows[index].siteUnit)) ?? 0) + 1);
  }
  if (seen.size !== rows.length || excluded.size !== excludedCounts.size ||
      [...excluded].some(([label, count]) => excludedCounts.get(label) !== count)) {
    throw new Error('Project report omitted original tuple ownership without exact exclusion evidence.');
  }
}
