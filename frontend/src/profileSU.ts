import type { ProfileSUReview, ProfileSUCreated, ProfileSUPlot, ProjectPlotProfileFilterRequest } from '../bindings/github.com/boostao/vpro-wails';
import { validateTable, validateProfileRunResult } from './projectPlotProfileReview';

export type ReviewedProfileSU = Omit<ProfileSUReview, 'plots'> & { plots: ProfileSUPlot[] };

export function validateProfileSUReview(review: ProfileSUReview, proposal: ProjectPlotProfileFilterRequest): ReviewedProfileSU {
  if (!review?.filter?.input || !review.filter.preview || !Array.isArray(review.plots) ||
      !('sourceSU' in review) || !('descriptions' in review)) {
    throw new Error('Save as SU requires complete independently reviewed stored inputs, rows and explicit nullable metadata.');
  }
  const original = validateTable(proposal.input.originalRules, ['Order']);
  validateProfileRunResult(review.filter.preview, { project: proposal.preview.project, table: proposal.preview.table, rules: original });
  if (JSON.stringify(review.filter.preview) !== JSON.stringify(proposal.preview) ||
      JSON.stringify(review.filter.input.originalRules) !== JSON.stringify(proposal.input.originalRules) ||
      JSON.stringify(review.filter.input.projectLump ?? null) !== JSON.stringify(proposal.input.projectLump ?? null) ||
      review.filter.input.subvarieties !== proposal.input.subvarieties) {
    throw new Error('Save as SU review belongs to different stored inputs or result membership.');
  }
  const template = validateTable(review.template, ['PlotNumber', 'SiteUnit']);
  if (template.columns.length !== 2 || template.rows.length !== 0 ||
      template.columns[0].name !== 'PlotNumber' || template.columns[1].name !== 'SiteUnit') {
    throw new Error('Save as SU requires the original empty two-column template.');
  }
  if (review.sourceSU !== null) validateTable(review.sourceSU, ['PlotNumber', 'SiteUnit']);
  if ((proposal.preview.su === 'None') !== (review.sourceSU === null)) {
    throw new Error('Save as SU source SiteUnit ownership differs from the selected SU.');
  }
  if (review.descriptions !== null) validateTable(review.descriptions, ['table_name', 'description']);
  if (review.plots.length !== proposal.preview.plotNumbers?.length) throw new Error('Save as SU row count differs from reviewed membership.');
  for (const [index, plot] of review.plots.entries()) {
    if (typeof plot.plotNumber !== 'string' || plot.plotNumber !== proposal.preview.plotNumbers[index] ||
        plot.plotNumber.length > 7 || !('siteUnit' in plot) ||
        plot.siteUnit !== null && (typeof plot.siteUnit !== 'string' || plot.siteUnit.length > 255)) {
      throw new Error('Save as SU requires exact original plot identities and literal/NULL SiteUnit values.');
    }
  }
  return { ...review, plots: review.plots };
}

export function validateProfileSUCreated(created: ProfileSUCreated, name: string, count: number): ProfileSUCreated {
  if (!created || created.name !== name || typeof created.path !== 'string' || !created.path ||
      !created.path.toLowerCase().endsWith('.db') || created.plotCount !== count) {
    throw new Error('Published SU response is incomplete; do not replay creation.');
  }
  return created;
}
