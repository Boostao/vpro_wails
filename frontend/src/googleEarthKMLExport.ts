import type { GoogleEarthKMLExportReview, GoogleEarthKMLExportOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { validateGoogleEarthKMLReview } from './googleEarthKMLReview';
import { wellFormedUTF16 } from './qualityEditor';
import { PublicationSession, type PublicationView } from './publicationSession';

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const text = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');

export async function validateGoogleEarthKMLExportReview(value: GoogleEarthKMLExportReview | null,
  scope: GoogleEarthScope, descriptionField: string, title: string): Promise<GoogleEarthKMLExportReview> {
  value = value ? structuredClone(value) : null;
  if (!value || !hash(value.approvalHash) || !hash(value.kmlSHA256)) {
    throw new Error('KML file review has an incomplete source approval or byte hash.');
  }
  const review = await validateGoogleEarthKMLReview(value.review, scope, descriptionField, title);
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(review.kml));
  const actual = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
  if (actual !== value.kmlSHA256) throw new Error('KML file review byte hash differs from the displayed XML.');
  return { ...value, review };
}

export function validateGoogleEarthKMLExportOutcome(value: GoogleEarthKMLExportOutcome | null,
  destination: string, kmlSHA256: string): GoogleEarthKMLExportOutcome {
  value = value ? structuredClone(value) : null;
  if (!value || !text(value.requestedDestination) || value.requestedDestination !== destination ||
      !text(value.path) || !text(value.errorMessage) ||
      !['published', 'published-with-errors', 'not-published'].includes(value.status) ||
      value.sha256 !== '' && !hash(value.sha256)) {
    throw new Error('KML publication acknowledgement is incomplete; outcome unknown, never replay.');
  }
  const committed = value.status !== 'not-published';
  if (committed && (!value.path || value.sha256 !== kmlSHA256) ||
      value.status === 'published' && value.errorMessage !== '' ||
      value.status !== 'published' && value.errorMessage === '') {
    throw new Error('KML publication acknowledgement contradicts the planned bytes/outcome; never replay.');
  }
  return value;
}

export type KMLPublicationView = PublicationView<GoogleEarthKMLExportOutcome>;

export class GoogleEarthKMLPublicationSession {
  private receipt = new PublicationSession<GoogleEarthKMLExportOutcome>('KML file', 'KML');
  private readonly scope: GoogleEarthScope;
  constructor(scope: GoogleEarthScope) { this.scope = structuredClone(scope); }
  view(): KMLPublicationView { return this.receipt.view(); }
  subscribe(listener: () => void): () => void {
    return this.receipt.subscribe(listener);
  }
  async publish(review: GoogleEarthKMLExportReview, destination: string,
    port: (request: { descriptionField: string; title: string; approvalHash: string; destination: string }) =>
      PromiseLike<GoogleEarthKMLExportOutcome | null>): Promise<void> {
    if (this.receipt.view().busy || this.receipt.view().blocked) throw new Error('Acknowledge the prior KML file outcome before a new publication.');
    if (!ownedScope(review.review, this.scope) || !hash(review.approvalHash) || !hash(review.kmlSHA256) ||
        !text(destination) || !destination) throw new Error('KML publication requires the owned review and an explicit literal destination.');
    const requested = structuredClone({ descriptionField: review.review.descriptionField, title: review.review.title,
      approvalHash: review.approvalHash, destination });
    const expectedHash = review.kmlSHA256;
    await this.receipt.publish(requested, destination, port,
      value => validateGoogleEarthKMLExportOutcome(value, destination, expectedHash));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { scope: GoogleEarthScope; session: GoogleEarthKMLPublicationSession }>();
export function googleEarthKMLPublicationSession(scope: GoogleEarthScope): GoogleEarthKMLPublicationSession {
  const existing = sessions.get(scope.contextId);
  if (existing) {
    if (!ownedScope(existing.scope, scope)) throw new Error('KML publication receipt belongs to different owned paths.');
    return existing.session;
  }
  const owner = structuredClone(scope);
  const session = new GoogleEarthKMLPublicationSession(owner);
  sessions.set(owner.contextId, { scope: owner, session });
  return session;
}
