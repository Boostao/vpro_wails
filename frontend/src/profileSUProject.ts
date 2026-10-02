import type { ProfileSUProjectReview, ProfileSUCreated, ProjectPlotProfileFilterRequest } from '../bindings/github.com/boostao/vpro-wails';
import { validateProfileSUReview, validateProfileSUCreated, type ReviewedProfileSU } from './profileSU';
import { validateTable, type ProfileReviewTable } from './projectPlotProfileReview';

export type ReviewedProjectSU = Omit<ProfileSUProjectReview, 'su' | 'metadata'> & {
  su: ReviewedProfileSU;
  metadata: ProfileReviewTable;
};

export function validateProjectSUReview(review: ProfileSUProjectReview, filter: ProjectPlotProfileFilterRequest,
  project: string, path: string): ReviewedProjectSU {
  if (!review || review.project !== project || review.path !== path || !path) {
    throw new Error('Existing-file SU review belongs to another project or physical path.');
  }
  const su = validateProfileSUReview(review.su, filter);
  if (su.descriptions !== null) {
    throw new Error('Existing-project SU creation does not discard or infer present template descriptions.');
  }
  const metadata = validateTable(review.metadata, ['table_name', 'description']);
  return { ...review, su, metadata };
}

export function validateProjectSUCreated(created: ProfileSUCreated, name: string, count: number, path: string): ProfileSUCreated {
  if (created?.path !== path) throw new Error('SU table committed in an unexpected destination; do not replay creation.');
  return validateProfileSUCreated(created, name, count);
}
