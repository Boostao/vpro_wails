import type { EnvironmentReportField, PlotLocationReview, PlotLocationReport, PlotLocationRow, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';

export const plotLocationFieldDefinitions = [
  ['PlotNumber', 'Plot Number'], ['Zone', 'Zone'], ['SubZone', 'Subzone'], ['SiteSeries', 'Site Series'],
  ['LocationAccuracy', 'Accuracy'], ['Latitude', 'Latitude'], ['Longitude', 'Longitude'], ['Elevation', 'Elevation'],
] as const;

export type ValidatedPlotLocationRow = Omit<PlotLocationRow, 'values' | 'membershipRowIds'> & {
  values: ProjectMetadataCell[]; membershipRowIds: string[];
};
export type ValidatedPlotLocationReview = Omit<PlotLocationReview, 'report'> & {
  report: Omit<PlotLocationReport, 'fields' | 'rows'> & {
    fields: EnvironmentReportField[]; rows: ValidatedPlotLocationRow[];
  };
};

export function exactLongitudeNegation(original: ProjectMetadataCell, converted: ProjectMetadataCell): boolean {
  if (original.storage === 'real' && converted.storage === 'real' && original.real !== null && converted.real !== null) {
    return Object.is(converted.real, -original.real);
  }
  if (original.storage === 'integer' && converted.storage === 'integer' && original.integer !== null && converted.integer !== null) {
    return original.integer !== '-9223372036854775808' && converted.integer === (-BigInt(original.integer)).toString();
  }
  return false;
}

export async function validatePlotLocationReview(value: PlotLocationReview | null, contextId: string,
  project: string, projectPath: string, su: string, suPath: string,
  yieldTask: () => Promise<void> = () => new Promise(resolve => setTimeout(resolve, 0))): Promise<ValidatedPlotLocationReview> {
  value = value ? structuredClone(value) : null;
  const report = value?.report;
  if (!value || !contextId || value.contextId !== contextId || value.projectPath !== projectPath ||
      value.suPath !== suPath || !report || report.project !== project || report.su !== su ||
      !Array.isArray(report.fields) || report.fields.length !== plotLocationFieldDefinitions.length ||
      !Array.isArray(report.rows) || !report.fields.every((field, i) => field.source === 'Env' &&
        field.key === plotLocationFieldDefinitions[i][0] && field.label === plotLocationFieldDefinitions[i][1] && field.heading === false)) {
    throw new Error('Plot locations have a foreign/incomplete owned scope or source field order.');
  }
  const fields = report.fields, rows: ValidatedPlotLocationRow[] = [];
  const envIds = new Set<string>(), adminIds = new Set<string>(), membershipIds = new Set<string>();
  for (const row of report.rows) {
    if (!exactSigned64(row.envRowId) || !exactSigned64(row.adminRowId) ||
        envIds.has(row.envRowId) || adminIds.has(row.adminRowId) ||
        !Array.isArray(row.membershipRowIds) || (su === 'None' ? row.membershipRowIds.length !== 0 : row.membershipRowIds.length === 0) ||
        !Array.isArray(row.values) || row.values.length !== fields.length || !row.values.every(completeCell) ||
        row.values[0].storage !== 'text' || !['integer', 'real'].includes(row.values[5].storage) ||
        !completeCell(row.storedLongitude) || !exactLongitudeNegation(row.storedLongitude, row.values[6])) {
      throw new Error('Plot locations have incomplete physical identities/values or changed longitude semantics; no partial review accepted.');
    }
    envIds.add(row.envRowId); adminIds.add(row.adminRowId);
    for (const identity of row.membershipRowIds) {
      if (!exactSigned64(identity) || membershipIds.has(identity)) throw new Error('Plot location SU provenance is incomplete or reused.');
      membershipIds.add(identity);
    }
    rows.push({ ...row, values: row.values, membershipRowIds: row.membershipRowIds });
    if (rows.length % 500 === 0) await yieldTask();
  }
  return { ...value, report: { ...report, fields, rows } };
}
