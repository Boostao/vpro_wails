import type {
  LongVegetationProjectFieldDefinitions, LongVegetationProjectOptions, LongVegetationProjectPreview,
  LongVegetationProjectUnit, LongVegetationProjectUnitOutcome, LongVegetationProjectUnitPreferences,
  LongVegetationReportScope,
} from '../bindings/github.com/boostao/vpro-wails';
import type { ProjectReportOwner, ProjectReportSource, ValidatedProjectPreview } from './longVegetationProjectReport';
import { projectReportOwnedScope, validateLongVegetationProjectPreview, validateProjectReportSettings,
  validateProjectUnitTuples } from './longVegetationProjectReport';
import { selectedUnitFieldError, validateReportUnitFieldDefinitions } from './longVegetationSelectedUnitSelection';
import { wellFormedUTF16 } from './qualityEditor';

export type ReportSourceScope = 'selected-list' | ProjectReportSource;
export type ValidatedReportScope = Omit<LongVegetationReportScope, 'sourceKind' | 'fields' | 'build'> & {
  sourceKind: ReportSourceScope;
  fields: string[];
  build: { approvalHash: string; projectTuples: LongVegetationProjectUnit[] } | null;
};
export type CurrentProjectScope = ValidatedReportScope & {
  sourceKind: ProjectReportSource;
  build: NonNullable<ValidatedReportScope['build']>;
  buildCurrent: true;
};

const shape = (value: unknown, keys: string[]): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && Object.keys(value).length === keys.length &&
  keys.every(key => Object.hasOwn(value, key));
const text = (value: unknown): value is string =>
  typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
const revision = (value: unknown): value is string => typeof value === 'string' && value.length <= 19 &&
  /^(0|[1-9]\d*)$/.test(value) && BigInt(value) <= 9223372036854775807n;
const tupleKey = (row: LongVegetationProjectUnit): string =>
  JSON.stringify([row.envRowId, row.adminRowId, row.plotNumber.storage, row.plotNumber.text, row.siteUnit.storage, row.siteUnit.text]);
export const sameProjectTuples = (left: LongVegetationProjectUnit[], right: LongVegetationProjectUnit[]): boolean =>
  left.length === right.length && left.every((row, i) => tupleKey(row) === tupleKey(right[i]));
const plotKey = (row: LongVegetationProjectUnit): string => JSON.stringify([row.plotNumber.storage, row.plotNumber.text]);

function requireEnabled(enabled: boolean): void {
  if (enabled !== true) throw new Error('Whole-project report scope client is disabled in this build.');
}

export function validateReportScope(value: LongVegetationReportScope, owner: ProjectReportOwner,
  enabled = false): ValidatedReportScope {
  assertReportScope(value, owner, enabled);
  return value;
}

function assertReportScope(value: LongVegetationReportScope, owner: ProjectReportOwner,
  enabled: boolean): asserts value is ValidatedReportScope {
  requireEnabled(enabled);
  if (!shape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'sourceKind', 'revision', 'fields', 'build', 'buildCurrent']) ||
      !projectReportOwnedScope(value, owner) || !revision(value.revision) ||
      (value.sourceKind !== 'selected-list' && value.sourceKind !== 'project-selected-fields' && value.sourceKind !== 'project-all-plots') ||
      !Array.isArray(value.fields) || value.fields.some(field => !text(field)) || typeof value.buildCurrent !== 'boolean' ||
      value.sourceKind === 'project-all-plots' && owner.su !== 'None') {
    throw new Error('Report scope belongs to an incomplete or different explicit owned session.');
  }
  if (value.sourceKind === 'selected-list') {
    if (value.build !== null || value.buildCurrent) throw new Error('Selected-list scope cannot retain a competing project approval.');
  } else if (value.build !== null) {
    if (!shape(value.build, ['approvalHash', 'projectTuples']) || typeof value.build.approvalHash !== 'string' ||
        !/^[0-9a-f]{64}$/.test(value.build.approvalHash) ||
        value.sourceKind === 'project-selected-fields' && (!value.fields.length ||
          value.fields.some(field => !/^[A-Za-z0-9_]+$/.test(field)) ||
          new Set(value.fields.map(field => field.toLowerCase())).size !== value.fields.length)) {
      throw new Error('Project Build receipt lost its explicit field order or approval hash.');
    }
    validateProjectUnitTuples(value.build.projectTuples, value.sourceKind);
  } else if (value.buildCurrent) {
    throw new Error('Project scope invented current authority without a Build receipt.');
  }
}

export function currentProjectScope(value: LongVegetationReportScope, owner: ProjectReportOwner,
  enabled = false): CurrentProjectScope {
  assertCurrentProjectScope(value, owner, enabled);
  return value;
}

function assertCurrentProjectScope(value: LongVegetationReportScope, owner: ProjectReportOwner,
  enabled: boolean): asserts value is CurrentProjectScope {
  assertReportScope(value, owner, enabled);
  if (value.sourceKind === 'selected-list' || !value.build || !value.buildCurrent) {
    throw new Error('Project output requires its current explicit scope and Build receipt.');
  }
}

function matchingProjectScope(value: LongVegetationReportScope, expected: LongVegetationReportScope,
  owner: ProjectReportOwner, enabled: boolean): CurrentProjectScope {
  const scope = currentProjectScope(value, owner, enabled), planned = currentProjectScope(expected, owner, enabled);
  if (scope.sourceKind !== planned.sourceKind || scope.revision !== planned.revision ||
      scope.fields.length !== planned.fields.length || scope.fields.some((field, i) => field !== planned.fields[i]) ||
      scope.build.approvalHash !== planned.build.approvalHash || !sameProjectTuples(scope.build.projectTuples, planned.build.projectTuples)) {
    throw new Error('Project result changed the independently requested scope, revision or complete Build witness.');
  }
  return scope;
}

export function validateProjectFieldCatalogue(value: LongVegetationProjectFieldDefinitions,
  expected: LongVegetationReportScope, owner: ProjectReportOwner, enabled = false): LongVegetationProjectFieldDefinitions {
  const scope = validateReportScope(expected, owner, enabled);
  if (!shape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'sourceKind', 'revision', 'definitions']) ||
      !projectReportOwnedScope(value, owner) || value.sourceKind !== scope.sourceKind || value.revision !== scope.revision) {
    throw new Error('Project field catalogue changed its independently observed owner, scope or revision.');
  }
  const definitions = validateReportUnitFieldDefinitions(value.definitions);
  if (!definitions.length || scope.sourceKind === 'project-selected-fields' && scope.buildCurrent &&
      selectedUnitFieldError(scope.fields, definitions)) {
    throw new Error('Project field catalogue contradicts its current physical field Build.');
  }
  return value;
}

export function validateProjectOptions(value: LongVegetationProjectOptions, expected: LongVegetationReportScope,
  owner: ProjectReportOwner, enabled = false): LongVegetationProjectOptions {
  requireEnabled(enabled);
  if (!shape(value, ['scope', 'options'])) throw new Error('Project options require an explicit scope wrapper.');
  const scope = matchingProjectScope(value.scope, expected, owner, enabled), options = value.options;
  if (!shape(options, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'settings']) ||
      !projectReportOwnedScope(options, owner)) throw new Error('Project options changed their physical context owner.');
  validateProjectReportSettings(options.settings, owner, scope.sourceKind,
    scope.sourceKind === 'project-selected-fields' ? scope.fields : []);
  return value;
}

export type ValidatedProjectScopePreview = Omit<LongVegetationProjectPreview, 'scope' | 'preview'> & {
  scope: CurrentProjectScope;
  preview: ValidatedProjectPreview;
};

export function validateProjectPreview(value: LongVegetationProjectPreview, expected: LongVegetationReportScope,
  owner: ProjectReportOwner, enabled = false): ValidatedProjectScopePreview {
  assertProjectScopePreview(value, expected, owner, enabled);
  return value;
}

function assertProjectScopePreview(value: LongVegetationProjectPreview, expected: LongVegetationReportScope,
  owner: ProjectReportOwner, enabled: boolean): asserts value is ValidatedProjectScopePreview {
  requireEnabled(enabled);
  if (!shape(value, ['scope', 'preview'])) throw new Error('Project preview requires an explicit scope wrapper.');
  const scope = matchingProjectScope(value.scope, expected, owner, enabled);
  const preview = validateLongVegetationProjectPreview(value.preview, owner, scope.sourceKind,
    scope.sourceKind === 'project-selected-fields' ? scope.fields : [], enabled);
  let planned = scope.build.projectTuples;
  if (preview.settings.quality !== null) {
    const plots = new Set(preview.report.quality!.occurrences!.map(row =>
      JSON.stringify([row.plotNumber.storage, row.plotNumber.text])));
    planned = planned.filter(row => plots.has(plotKey(row)));
  }
  if (!sameProjectTuples(preview.report.projectUnits, planned)) {
    throw new Error('Project preview lost complete original Build tuples or changed qualified existence pruning.');
  }
}

export function validateProjectUnitPreferences(value: LongVegetationProjectUnitPreferences,
  expected: LongVegetationReportScope, owner: ProjectReportOwner, enabled = false): LongVegetationProjectUnitPreferences {
  requireEnabled(enabled);
  if (!shape(value, ['scope', 'preferences'])) throw new Error('Project unit preferences require an explicit scope wrapper.');
  const scope = matchingProjectScope(value.scope, expected, owner, enabled), preferences = value.preferences;
  if (!shape(preferences, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'values', 'availability']) ||
      !projectReportOwnedScope(preferences, owner) || !shape(preferences.values, ['unitGroups']) ||
      ![1, 2, 3].includes(preferences.values.unitGroups) ||
      !shape(preferences.availability, ['siteUnitTable', 'selectedFields', 'none']) ||
      preferences.availability.siteUnitTable !== false ||
      preferences.availability.selectedFields !== (scope.sourceKind === 'project-selected-fields') ||
      preferences.availability.none !== (scope.sourceKind === 'project-all-plots')) {
    throw new Error('Project unit preferences lost actual saved choice or exclusive active-scope availability.');
  }
  return value;
}

export function validateProjectUnitOutcome(value: LongVegetationProjectUnitOutcome, expected: LongVegetationReportScope,
  owner: ProjectReportOwner, plannedChange: boolean, enabled = false): LongVegetationProjectUnitOutcome {
  const scope = currentProjectScope(expected, owner, enabled);
  if (!shape(value, ['sourceKind', 'revision', 'outcome']) ||
      !shape(value.outcome, ['contextId', 'changed', 'committed', 'errorMessage']) ||
      value.outcome.contextId !== owner.contextId || typeof plannedChange !== 'boolean' ||
      typeof value.outcome.changed !== 'boolean' || typeof value.outcome.committed !== 'boolean' ||
      value.outcome.changed !== value.outcome.committed || !text(value.outcome.errorMessage) ||
      value.outcome.committed && !plannedChange || !value.outcome.errorMessage && value.outcome.committed !== plannedChange ||
      (value.sourceKind !== scope.sourceKind && !(value.sourceKind === '' && value.outcome.errorMessage && !value.outcome.committed)) ||
      (value.revision !== scope.revision && !(value.revision === '' && value.outcome.errorMessage && !value.outcome.committed))) {
    throw new Error('Project unit Save returned an incomplete or contradictory owned mutation receipt.');
  }
  return value;
}
