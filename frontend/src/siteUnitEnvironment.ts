import type { SiteUnitEnvironmentReview, SiteUnitEnvironmentTransfer, SiteUnitEnvironmentResult } from '../bindings/github.com/boostao/vpro-wails';
import { transferColumns } from './environmentSUTransfer';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell } from './projectMetadataEditor';

export function validateSiteUnitEnvironmentReview(review: SiteUnitEnvironmentReview, contextId: string,
  project: string, su: string, path: string): SiteUnitEnvironmentReview {
  if (!review || !contextId || review.contextId !== contextId || review.project !== project || review.su !== su ||
      !su || su === 'None' || review.path !== path || !Array.isArray(review.changes) ||
      typeof review.historyHash !== 'string' || review.historyHash !== '' && !/^[0-9a-f]{64}$/.test(review.historyHash)) {
    throw new Error('Reverse transfer requires the explicitly selected owned project and SU.');
  }
  const env = transferColumns(review.env, ['PlotNumber', 'Flag']);
  const admin = transferColumns(review.admin, ['Plot', 'UserSiteUnit', 'SiteUnitShortName', 'SiteUnitLongName']);
  const source = transferColumns(review.source, ['PlotNumber', 'SiteUnit']);
  const master = transferColumns(review.master, ['SiteSeries', 'SiteSeriesLongName']);
  const personal = transferColumns(review.personal, ['SiteSeries', 'SiteSeriesLongName']);
  const seen = new Set<string>();
  for (const change of review.changes) {
    const environments = review.env.rows!.filter(row => row.cells![env.get('PlotNumber')!].text === change.plotNumber);
    const parents = review.admin.rows!.filter(row => row.cells![admin.get('Plot')!].text === change.plotNumber);
    const units = review.source.rows!.filter(row => row.cells![source.get('PlotNumber')!].text === change.plotNumber);
    const key = `${change.adminRowId}/${change.field}`;
    if (typeof change.plotNumber !== 'string' || environments.length !== 1 || parents.length !== 1 || units.length !== 1 ||
        environments[0].rowId !== change.envRowId || parents[0].rowId !== change.adminRowId || units[0].rowId !== change.suRowId ||
        !['UserSiteUnit', 'SiteUnitShortName', 'SiteUnitLongName'].includes(change.field) || seen.has(key) ||
        !completeCell(change.before) || !completeCell(change.after) ||
        !['text', 'null'].includes(change.before.storage) || !['text', 'null'].includes(change.after.storage) ||
        !equalCell(change.before, parents[0].cells![admin.get(change.field)!]) || equalCell(change.before, change.after)) {
      throw new Error('Reverse transfer requires exact unique typed physical links and changed Admin cells.');
    }
    const flag = environments[0].cells![env.get('Flag')!];
    if (flag.storage !== 'null' && !(flag.storage === 'integer' && flag.integer === '0')) {
      throw new Error('Reverse transfer cannot change a locked or unverifiably unlocked plot.');
    }
    const bound = change.field === 'SiteUnitShortName' ? 50 : 100;
    if (change.after.text !== null && (change.after.text.length > bound || change.after.text.includes('\0'))) {
      throw new Error(`Reverse ${change.field} exceeds the original ${bound} UTF-16-unit bound.`);
    }
    const unit = units[0].cells![source.get('SiteUnit')!];
    if (change.field === 'UserSiteUnit') {
      if (change.origin !== 'selected SU' || !equalCell(change.after, unit)) throw new Error('Reverse unit value changed after review.');
    } else {
      if (unit.storage !== 'text') throw new Error('NULL SU values cannot choose name definitions.');
      const masters = review.master.rows!.filter(row => row.cells![master.get('SiteSeries')!].text === unit.text);
      const users = review.personal.rows!.filter(row => row.cells![personal.get('SiteSeries')!].text === unit.text);
      if (masters.length > 1 || users.length > 1 || masters.length === 0 && users.length === 0) {
        throw new Error('Reverse names require a unique matching master or personal definition.');
      }
      const definition = users.length === 1 ? users[0] : masters[0];
      const columns = users.length === 1 ? personal : master;
      const origin = users.length === 1 ? 'personal override' : 'master';
      const column = change.field === 'SiteUnitShortName' ? 'SiteSeries' : 'SiteSeriesLongName';
      if (change.origin !== origin || !equalCell(change.after, definition.cells![columns.get(column)!])) {
        throw new Error('Reverse name definition or precedence changed after review.');
      }
    }
    seen.add(key);
  }
  return review;
}

export function siteUnitEnvironmentRequest(review: SiteUnitEnvironmentReview, contextId: string,
  project: string, su: string, path: string): SiteUnitEnvironmentTransfer {
  const checked = validateSiteUnitEnvironmentReview(review, contextId, project, su, path);
  if (!checked.changes?.length) throw new Error('Review a nonempty reverse transfer before confirmation.');
  return { review: structuredClone(checked), confirmed: true };
}

export function validateSiteUnitEnvironmentResult(result: SiteUnitEnvironmentResult | null,
  request: SiteUnitEnvironmentTransfer): asserts result is SiteUnitEnvironmentResult {
  if (!result || result.changedCells !== request.review.changes!.length ||
      result.changedRows !== new Set(request.review.changes!.map(change => change.adminRowId)).size ||
      !exactSigned64(result.historyId) || BigInt(result.historyId) < 1n) {
    throw new Error('Reverse transfer committed with an unexpected result; reload without replaying it.');
  }
}
