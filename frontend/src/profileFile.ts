import type { PlotProfileFileReview, PlotProfileFileCreated } from '../bindings/github.com/boostao/vpro-wails';
import { plotProfileFields, validateTable, type ProfileReviewTable } from './projectPlotProfileReview';

export interface ReviewedProfileFile {
  template: ProfileReviewTable;
  metadataAbsent: true;
}

export function validateProfileFileReview(review: PlotProfileFileReview): ReviewedProfileFile {
  if (!review || review.metadataAbsent !== true) {
    throw new Error('Blank profile creation requires explicitly absent original description metadata; no descriptions are inferred.');
  }
  const template = validateTable(review.template, plotProfileFields);
  if (template.rows.length !== 0 || template.columns.length !== 9 ||
      template.columns.some((column, index) => column.name !== plotProfileFields[index] ||
        column.declaredType !== (index === 0 ? 'SMALLINT' : index === 8 ? 'INTEGER' : 'VARCHAR'))) {
    throw new Error('Blank profile creation requires the complete original empty nine-field template.');
  }
  return { template, metadataAbsent: true };
}

export function validateProfileFileCreated(response: PlotProfileFileCreated, expectedName: string): PlotProfileFileCreated {
  if (!response || response.source?.name !== expectedName || typeof response.source.path !== 'string' ||
      response.source.path.length === 0 || response.table !== `${expectedName}_Profile` || response.ruleCount !== 0) {
    throw new Error('Profile file published, but response is incomplete or belongs to another proposal; do not replay creation.');
  }
  return response;
}
