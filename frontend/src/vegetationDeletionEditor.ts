import type { VegetationDeletionReview } from '../bindings/github.com/boostao/vpro-wails';

export function validateDeletionReview(review: VegetationDeletionReview, form: string, id: number, species: string): VegetationDeletionReview {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647 ||
      review.id !== id || review.form !== form || review.species !== species ||
      !/^[0-9a-f]{64}$/.test(review.expected) || review.columns === null ||
      review.columns.length < 3 || !review.columns.includes('Species')) {
    throw new Error('Vegetation deletion review differs from the planned source row; cancel and reload before reviewing again.');
  }
  return review;
}
