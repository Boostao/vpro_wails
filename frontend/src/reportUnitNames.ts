import type { EnvironmentReportName } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';

type ReportNames = {
  longName: string | null;
  nameStatus: string;
  nameCandidates: EnvironmentReportName[] | null;
};

export function validateReportUnitNames(value: ReportNames, unassigned = false): void {
  if (!value || !Array.isArray(value.nameCandidates)) throw new Error('Report unit name candidates are incomplete.');
  if (unassigned) {
    if (value.nameStatus !== 'unassigned' || value.longName !== '' || value.nameCandidates.length !== 0) {
      throw new Error('Unassigned report units must retain an empty source long name without a NULL-key reference join.');
    }
    return;
  }
  const ids = new Set<string>(), names = new Set<string>();
  let unsupported = false;
  for (const candidate of value.nameCandidates) {
    if (!candidate || !exactSigned64(candidate.rowId) || ids.has(candidate.rowId) || !completeCell(candidate.value)) {
      throw new Error('Report name candidate lost complete typed storage or distinct physical identity.');
    }
    ids.add(candidate.rowId);
    if (candidate.value.storage === 'text') names.add(candidate.value.text!);
    else if (candidate.value.storage !== 'null') unsupported = true;
  }
  const status = unsupported ? 'unsupported_storage' : names.size > 1 ? 'conflicting' : names.size === 1 ? 'unique' : 'missing';
  if (value.nameStatus !== status ||
      (status === 'unique' ? typeof value.longName !== 'string' || !names.has(value.longName) : value.longName !== null)) {
    throw new Error('Report long name/status differs from its preserved reference candidates.');
  }
}
