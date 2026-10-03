import type { EnvironmentSiteUnitReview, EnvironmentSiteUnitTransfer, EnvironmentSiteUnitResult, ProjectMetadataTable } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell } from './projectMetadataEditor';

function columns(table: ProjectMetadataTable, required: readonly string[]): Map<string, number> {
  if (!table || !Array.isArray(table.columns) || !Array.isArray(table.rows)) throw new Error('Complete original transfer tables were not returned.');
  const index = new Map(table.columns.map((column, i) => [column.name, i]));
  if (index.size !== table.columns.length || table.columns.some(column => !column.name || typeof column.declaredType !== 'string') ||
      required.some(name => !index.has(name)) || new Set(table.rows.map(row => row.rowId)).size !== table.rows.length ||
      table.rows.some(row => !exactSigned64(row.rowId) || !row.cells || row.cells.length !== table.columns!.length || !row.cells.every(completeCell))) {
    throw new Error('Transfer requires complete typed rows with exact distinct physical/schema identities.');
  }
  return index;
}

export function validateEnvironmentSUReview(review: EnvironmentSiteUnitReview, contextId: string,
  project: string, su: string, path: string): EnvironmentSiteUnitReview {
  if (!review || !contextId || review.contextId !== contextId || review.project !== project ||
      review.su !== su || !su || su === 'None' || review.path !== path || !Array.isArray(review.changes) ||
      typeof review.historyHash !== 'string' || review.historyHash !== '' && !/^[0-9a-f]{64}$/.test(review.historyHash)) {
    throw new Error('Transfer review does not belong to the explicitly selected owned project and SU.');
  }
  const env = columns(review.env, ['PlotNumber']);
  const admin = columns(review.admin, ['Plot', 'UserSiteUnit']);
  const target = columns(review.target, ['PlotNumber', 'SiteUnit']);
  const seen = new Set<string>();
  for (const change of review.changes) {
    const source = review.env.rows!.filter(row => row.cells![env.get('PlotNumber')!].text === change.plotNumber);
    const parent = review.admin.rows!.filter(row => row.cells![admin.get('Plot')!].text === change.plotNumber);
    const destination = review.target.rows!.filter(row => row.cells![target.get('PlotNumber')!].text === change.plotNumber);
    if (typeof change.plotNumber !== 'string' || source.length !== 1 || parent.length !== 1 || destination.length !== 1 ||
        source[0].rowId !== change.envRowId || parent[0].rowId !== change.adminRowId || destination[0].rowId !== change.suRowId ||
        seen.has(change.suRowId) || !completeCell(change.before) || !completeCell(change.after) ||
        !['text', 'null'].includes(change.before.storage) || !['text', 'null'].includes(change.after.storage) ||
        !equalCell(change.before, destination[0].cells![target.get('SiteUnit')!]) ||
        !equalCell(change.after, parent[0].cells![admin.get('UserSiteUnit')!]) ||
        equalCell(change.before, change.after) ||
        change.after.text !== null && (change.after.text.length > 255 || change.after.text.includes('\0'))) {
      throw new Error('Transfer changes must retain exact unique Env/Admin/SU links and bounded original typed values.');
    }
    seen.add(change.suRowId);
  }
  return review;
}

export function environmentSURequest(review: EnvironmentSiteUnitReview, contextId: string,
  project: string, su: string, path: string): EnvironmentSiteUnitTransfer {
  const checked = validateEnvironmentSUReview(review, contextId, project, su, path);
  if (!checked.changes?.length) throw new Error('Review a nonempty transfer before explicit confirmation.');
  return { review: structuredClone(checked), confirmed: true };
}

export function validateEnvironmentSUResult(result: EnvironmentSiteUnitResult | null, request: EnvironmentSiteUnitTransfer): asserts result is EnvironmentSiteUnitResult {
  if (!result || result.changedRows !== request.review.changes!.length || !exactSigned64(result.historyId) || BigInt(result.historyId) < 1n) {
    throw new Error('Transfer committed but returned an unexpected result; reload without replaying it.');
  }
}
