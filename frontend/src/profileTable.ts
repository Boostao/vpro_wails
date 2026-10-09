import type { PlotProfileTableReview, PlotProfileFileCreated } from '../bindings/github.com/boostao/vpro-wails';
import { validateProfileFileReview, validateProfileFileCreated, type ReviewedProfileFile } from './profileFile';
import { validateProjectPlotProfileReview, type ValidatedProjectPlotProfileReview } from './projectPlotProfileReview';

export interface ReviewedProfileTable {
  template: ReviewedProfileFile;
  profile: ValidatedProjectPlotProfileReview;
}

export function validateProfileTableReview(review: PlotProfileTableReview): ReviewedProfileTable {
  if (!review) throw new Error('Existing-file creation review is missing.');
  const template = validateProfileFileReview(review.template);
  const profile = validateProjectPlotProfileReview(review.profile);
  if (!profile.source.available || !profile.source.writable) {
    throw new Error('Existing-file profile creation requires current selected-profile write ownership.');
  }
  return { template, profile };
}

export function validateProfileTableCreated(response: PlotProfileFileCreated, expectedName: string, expectedPath: string): PlotProfileFileCreated {
  if (response?.source?.path !== expectedPath) {
    throw new Error('Profile table committed, but response belongs to another file; do not replay creation.');
  }
  return validateProfileFileCreated(response, expectedName);
}
