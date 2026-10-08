import type { SiteUnitSummaryPreview, SiteUnitSummaryReport, SiteUnitSummaryUnit } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { validateReportUnitNames } from './reportUnitNames';

const keys = ['Zone', 'SubZone', 'Elevation', 'Aspect', 'SlopeGradient', 'MesoSlopePosition',
  'MoistureRegime', 'NutrientRegime', 'SiteDisturbance2', 'SiteDisturbance2', 'Exposure1',
  'SurfaceTopographyType', 'SubstrateOrganicMatter', 'SubstrateRocks', 'SubstrateDecWood',
  'SubstrateMineralSoil', 'SubstrateBedRock', 'SubstrateWater', 'HydroGeoSystem',
  'HydroGeoSubSystem', 'StandAge', 'SuccessionalStatus', 'StructuralStage', 'StrataCoverTree',
  'StrataCoverShrub', 'StrataCoverHerb', 'StrataCoverMoss', 'SoilClassGroup', 'HumusForm',
  'HumusThickness', 'SoilDrainage', 'SeepageDepth', 'SurficialMaterialSurf',
  'RootZoneParticleSize', 'RootingDepth', 'RootRestrictingType', 'BedrockGeology1',
  'BedrockGeology2', 'BedrockGeology3'];

function completeText(value: unknown): value is string {
  return typeof value === 'string' && completeCell({ storage: 'text', text: value,
    integer: null, real: null, blobHex: null });
}

export type ValidatedSiteUnitSummary = Omit<SiteUnitSummaryPreview, 'report'> & {
  report: Omit<SiteUnitSummaryReport, 'fields' | 'memberships' | 'units'> & {
    fields: NonNullable<SiteUnitSummaryReport['fields']>;
    memberships: NonNullable<SiteUnitSummaryReport['memberships']>;
    units: (Omit<SiteUnitSummaryUnit, 'plots' | 'values' | 'nameCandidates'> & {
      plots: NonNullable<SiteUnitSummaryUnit['plots']>;
      values: NonNullable<SiteUnitSummaryUnit['values']>;
      nameCandidates: NonNullable<SiteUnitSummaryUnit['nameCandidates']>;
    })[];
  };
};

function hasSummaryArrays(value: SiteUnitSummaryPreview): value is ValidatedSiteUnitSummary {
  return !!value?.report && Array.isArray(value.report.fields) && Array.isArray(value.report.memberships) &&
    Array.isArray(value.report.units) && value.report.units.every(unit =>
      !!unit && Array.isArray(unit.plots) && Array.isArray(unit.values) && Array.isArray(unit.nameCandidates));
}

export function validateSiteUnitSummary(value: SiteUnitSummaryPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string, method: number): ValidatedSiteUnitSummary {
  if (!hasSummaryArrays(value)) throw new Error('Summary arrays are incomplete.');
  const report = value.report;
  if (!value || value.contextId !== contextId || value.projectPath !== projectPath || value.suPath !== suPath ||
      !report || report.project !== project || report.su !== su || su === 'None' ||
      ![1, 2].includes(method) || report.method !== method || report.querySource !== 'selected-su-filtered-env-admin' ||
      !Array.isArray(report.fields) || report.fields.length !== keys.length ||
      !Array.isArray(report.units) || !Array.isArray(report.memberships)) {
    throw new Error('Summary belongs to a different or incomplete context/method.');
  }
  for (let i = 0; i < keys.length; i++) {
    const field = report.fields[i];
    if (!field || field.key !== keys[i] || field.source !== (i === 29 ? 'Admin' : 'Env') ||
        field.section !== (i < 21 ? 'SITE' : i < 27 ? 'VEGETATION' : 'SOILS') ||
        !completeText(field.label) || !['category', 'bgc-unit', 'numeric', 'aspect', 'moisture'].includes(field.kind)) {
      throw new Error('Summary field registry differs from the source report.');
    }
  }
  const memberships = new Map<string, typeof report.memberships[number]>();
  for (const membership of report.memberships) {
    if (!membership || !exactSigned64(membership.rowId) || memberships.has(membership.rowId) ||
        !completeCell(membership.plotNumber) || !completeCell(membership.siteUnit) ||
        !['text', 'null'].includes(membership.plotNumber.storage) || !['text', 'null'].includes(membership.siteUnit.storage) ||
        !Number.isSafeInteger(membership.joinedRows) || membership.joinedRows < 0 ||
        !['joined', 'null-unit', 'null-plot', 'missing-env', 'missing-admin'].includes(membership.status)) {
      throw new Error('Summary membership provenance is incomplete.');
    }
    const nullUnit = membership.siteUnit.storage === 'null', nullPlot = membership.plotNumber.storage === 'null';
    if ((membership.status === 'joined') !== (membership.joinedRows > 0) ||
        (nullUnit ? membership.status !== 'null-unit' :
          nullPlot ? membership.status !== 'null-plot' : ['null-unit', 'null-plot'].includes(membership.status))) {
      throw new Error('Summary exclusion status contradicts physical membership.');
    }
    memberships.set(membership.rowId, membership);
  }
  const units = new Set<string>(), triples = new Set<string>(), counts = new Map<string, number>();
  for (const unit of report.units) {
    if (!unit || !completeText(unit.code) || units.has(unit.code) || !Array.isArray(unit.plots) || unit.plots.length === 0 ||
        !Array.isArray(unit.values) || unit.values.length !== keys.length || !unit.values.every(completeText)) {
      throw new Error('Summary unit values or joined plot scope are incomplete.');
    }
    units.add(unit.code); validateReportUnitNames(unit);
    for (const plot of unit.plots) {
      const membership = memberships.get(plot?.suRowId);
      const triple = JSON.stringify([plot?.suRowId, plot?.envRowId, plot?.adminRowId]);
      if (!plot || !completeText(plot.plotNumber) || !exactSigned64(plot.suRowId) || !exactSigned64(plot.envRowId) ||
          !exactSigned64(plot.adminRowId) || triples.has(triple) || !membership || membership.status !== 'joined' ||
          membership.plotNumber.text !== plot.plotNumber || membership.siteUnit.text !== unit.code) {
        throw new Error('Summary lost physical duplicate weights or invented a plot join.');
      }
      triples.add(triple); counts.set(plot.suRowId, (counts.get(plot.suRowId) ?? 0) + 1);
    }
  }
  for (const membership of memberships.values()) {
    if ((counts.get(membership.rowId) ?? 0) !== membership.joinedRows) {
      throw new Error('Summary joined row count differs from membership provenance.');
    }
  }
  return value;
}
