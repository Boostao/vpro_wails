import type { LongEnvironmentPreview, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';

export function reportTitleError(title: string): string | null {
  return completeCell({ storage: 'text', text: title, integer: null, real: null, blobHex: null }) && !title.includes('\0')
    ? null : 'Title requires complete Unicode without NUL; text is not repaired or normalized.';
}

export function reportCellText(cell: ProjectMetadataCell): string {
  switch (cell.storage) {
    case 'null': return 'NULL';
    case 'text': return cell.text === '' ? '"" (empty text)' : cell.text!;
    case 'integer': return cell.integer!;
    case 'real': return String(cell.real);
    case 'blob': return `BLOB ${cell.blobHex}`;
    default: throw new Error('Unsupported report cell storage.');
  }
}

export function validateLongEnvironmentPreview(value: LongEnvironmentPreview, contextId: string,
  project: string, projectPath: string, su: string, suPath: string, title: string): LongEnvironmentPreview {
  const report = value?.report;
  if (!value || value.contextId !== contextId || value.projectPath !== projectPath || value.suPath !== suPath ||
      !report || report.project !== project || report.su !== su || report.title !== title ||
      !Array.isArray(report.fields) || report.fields.length !== 72 || !Array.isArray(report.units) ||
      !Array.isArray(report.diagnostics)) throw new Error('Report belongs to a different or incomplete context/title.');
  const fields = new Set<string>();
  let headings = 0;
  for (const field of report.fields) {
    if (typeof field.label !== 'string' || typeof field.heading !== 'boolean') throw new Error('Report field metadata is incomplete.');
    if (field.heading) {
      headings++;
      if (field.source !== '' || field.key !== '') throw new Error('Report heading became an observation.');
    } else {
      const identity = `${field.source}.${field.key}`;
      if (!['Env', 'Admin'].includes(field.source) || !field.key || fields.has(identity)) throw new Error('Report field identity is ambiguous.');
      fields.add(identity);
    }
  }
  if (headings !== 5 || report.fields[0].source !== 'Env' || report.fields[0].key !== 'PlotNumber') throw new Error('Report registry differs.');
  const units = new Set<string>(), plots = new Set<string>();
  for (const unit of report.units) {
    if (typeof unit.code !== 'string' || units.has(unit.code) || !Array.isArray(unit.plots) ||
        !Array.isArray(unit.nameCandidates) || !['unique', 'missing', 'conflicting', 'unsupported_storage'].includes(unit.nameStatus) ||
        (unit.nameStatus === 'unique' ? typeof unit.longName !== 'string' : unit.longName !== null)) throw new Error('Report unit/name metadata is incomplete.');
    units.add(unit.code);
    const candidates = new Set<string>();
    for (const candidate of unit.nameCandidates) {
      if (!exactSigned64(candidate.rowId) || candidates.has(candidate.rowId) || !completeCell(candidate.value)) throw new Error('Report name candidate lost typed storage or identity.');
      candidates.add(candidate.rowId);
    }
    for (const plot of unit.plots) {
      if (typeof plot.plotNumber !== 'string' || plots.has(plot.plotNumber) ||
          !['complete', 'missing_env', 'missing_admin', 'missing_env_and_admin'].includes(plot.status) ||
          !Array.isArray(plot.values) || plot.values.length !== 72) throw new Error('Report plot scope is ambiguous or incomplete.');
      plots.add(plot.plotNumber);
      for (let i = 0; i < plot.values.length; i++) {
        const cell = plot.values[i];
        if (!completeCell(cell) || (report.fields[i].heading || plot.status !== 'complete') && cell.storage !== 'null') {
          throw new Error('Report lost typed values or invented a heading/orphan observation.');
        }
      }
      if (plot.status === 'complete' && plot.values[0].text !== plot.plotNumber) throw new Error('Report physical plot identity differs.');
    }
  }
  for (const diagnostic of report.diagnostics) {
    if (!['null_membership', 'unit_name_missing', 'unit_name_conflicting', 'unit_name_unsupported_storage',
          'missing_env', 'missing_admin', 'duplicate_membership'].includes(diagnostic.code) ||
        !Number.isSafeInteger(diagnostic.count) || diagnostic.count < 0 ||
        (diagnostic.unit !== null && (typeof diagnostic.unit !== 'string' || !units.has(diagnostic.unit))) ||
        (diagnostic.plotNumber !== null && (typeof diagnostic.plotNumber !== 'string' || !plots.has(diagnostic.plotNumber)))) {
      throw new Error('Report diagnostic identity/count is incomplete.');
    }
  }
  return value;
}
