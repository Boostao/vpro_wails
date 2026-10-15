import type { PlotLabelPDFExportReview, PlotLabelPDFExportOutcome, PlotLabelPDFAttemptOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { validatePlotLabelRegionalPreview } from './plotLabelRegional';
import { wellFormedUTF16 } from './qualityEditor';
import { PublicationSession, type PublicationView } from './publicationSession';

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const text = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
const exact = (value: object, keys: string[]) => typeof value === 'object' && !Array.isArray(value) &&
  Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));

export function validatePlotLabelPDFReview(value: PlotLabelPDFExportReview | null, scope: GoogleEarthScope, plot: string): PlotLabelPDFExportReview {
  value = value ? structuredClone(value) : null;
  if (!value || !exact(value, ['preview', 'project', 'projectPath', 'su', 'suPath', 'approvalHash', 'pdfSHA256', 'layout', 'sessionId']) ||
      !value.preview || !ownedScope({ ...value, contextId: value.preview.contextId }, scope) ||
      !hash(value.approvalHash) || !hash(value.pdfSHA256) || !text(value.sessionId) || !value.sessionId || value.sessionId.length > 128 ||
      value.layout !== 'portable-letter-single-fixed-ascii-v1') {
    throw new Error('Label PDF review has a foreign/incomplete owner, layout or approval; no partial export allowed.');
  }
  const native = validatePlotLabelRegionalPreview(value.preview, scope.contextId, plot);
  if (native.evidence.locale !== 'en-US' || native.evidence.pattern !== 'dddd, MMMM d, yyyy') {
    throw new Error('Label PDF review is outside the calibrated regional profile.');
  }
  return value;
}

export function validatePlotLabelPDFOutcome(value: PlotLabelPDFExportOutcome | null, destination: string, pdfSHA256: string): PlotLabelPDFExportOutcome {
  value = value ? structuredClone(value) : null;
  if (!value || !exact(value, ['status', 'requestedDestination', 'path', 'sha256', 'errorMessage']) ||
      value.requestedDestination !== destination || !text(value.path) || !text(value.errorMessage) ||
      !['published', 'published-with-errors', 'not-published'].includes(value.status) ||
      value.sha256 !== '' && !hash(value.sha256) ||
      value.status !== 'not-published' && (!value.path || value.sha256 !== pdfSHA256) ||
      value.status === 'published' && value.errorMessage !== '' ||
      value.status !== 'published' && value.errorMessage === '') {
    throw new Error('Label PDF acknowledgement contradicts its exact request/bytes; outcome unknown, never replay.');
  }
  return value;
}

export interface PlotLabelPDFRequest {
  plotNumber: string;
  approvalHash: string;
  destination: string;
  requestId: string;
  sessionId: string;
}

export class PlotLabelPDFPublicationSession {
  private readonly receipt = new PublicationSession<PlotLabelPDFExportOutcome>('Label PDF file', 'Label PDF');
  private readonly scope: GoogleEarthScope;
  private request: PlotLabelPDFRequest | null = null;
  private pdfSHA256 = '';
  private recovered: PlotLabelPDFExportOutcome | null = null;
  private recoveryError = '';
  private recovering = false;
  private readonly listeners = new Set<() => void>();
  constructor(scope: GoogleEarthScope) {
    this.scope = structuredClone(scope);
    this.receipt.subscribe(() => this.notify());
  }
  view(): PublicationView<PlotLabelPDFExportOutcome> {
    const original = this.receipt.view();
    return { ...original, busy: original.busy || this.recovering, blocked: original.blocked || this.recovering,
      outcome: this.recovered ? structuredClone(this.recovered) : original.outcome,
      error: this.recoveryError || (this.recovered ? this.recovered.errorMessage : original.error) };
  }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  async publish(review: PlotLabelPDFExportReview, destination: string,
    port: (request: PlotLabelPDFRequest) => PromiseLike<PlotLabelPDFExportOutcome | null>,
    requestId = crypto.randomUUID()): Promise<void> {
    if (this.view().busy || this.view().blocked) throw new Error('Resolve and acknowledge the prior label PDF outcome first.');
    if (!review?.preview) throw new Error('Label PDF publication requires the complete approved preview.');
    const checked = validatePlotLabelPDFReview(review, this.scope, review.preview.plotNumber);
    if (!text(destination) || !destination || !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(requestId)) {
      throw new Error('Label PDF publication requires a literal destination and stable lowercase UUIDv4.');
    }
    this.request = { plotNumber: checked.preview.plotNumber, approvalHash: checked.approvalHash,
      destination, requestId, sessionId: checked.sessionId };
    this.pdfSHA256 = checked.pdfSHA256;
    this.recovered = null; this.recoveryError = '';
    await this.receipt.publish(this.request, destination, port,
      value => validatePlotLabelPDFOutcome(value, destination, checked.pdfSHA256));
  }
  async recover(port: (request: PlotLabelPDFRequest) => PromiseLike<PlotLabelPDFAttemptOutcome | null>): Promise<void> {
    if (!this.request || this.view().busy || !this.view().blocked) throw new Error('Readonly recovery requires an unresolved retained PDF attempt.');
    const request = structuredClone(this.request);
    this.recovering = true; this.recoveryError = ''; this.notify();
    try {
      const result = await port(request);
      const value = result ? structuredClone(result) : null;
      if (!value || !exact(value, ['contextId', 'requestId', 'status', 'reason', 'outcome']) ||
          value.contextId !== this.scope.contextId || value.requestId !== request.requestId || !text(value.reason) || !value.reason) {
        throw new Error('Readonly PDF recovery has a foreign/incomplete attempt identity.');
      }
      if (value.status === 'running' && value.outcome === null) {
        this.recoveryError = value.reason;
      } else if (value.status === 'completed') {
        this.recovered = validatePlotLabelPDFOutcome(value.outcome, request.destination, this.pdfSHA256);
      } else {
        throw new Error('Readonly PDF recovery cannot certify the retained attempt.');
      }
    } catch (cause) {
      this.recoveryError = `Label PDF outcome remains unknown; never replay the retained request: ${String(cause)}`;
    } finally {
      this.recovering = false; this.notify();
    }
  }
  acknowledge() {
    if (!this.view().outcome) throw new Error('Recover the unknown label PDF outcome before acknowledgement; never replay.');
    if (this.view().busy) throw new Error('Wait for the retained label PDF operation.');
    this.receipt.acknowledge(); this.notify();
  }
}

const sessions = new Map<string, { scope: GoogleEarthScope; session: PlotLabelPDFPublicationSession }>();
export function plotLabelPDFPublicationSession(scope: GoogleEarthScope): PlotLabelPDFPublicationSession {
  const existing = sessions.get(scope.contextId);
  if (existing) {
    if (!ownedScope(existing.scope, scope)) throw new Error('Retained PDF receipt belongs to different owned paths.');
    return existing.session;
  }
  const owner = structuredClone(scope);
  const session = new PlotLabelPDFPublicationSession(owner);
  sessions.set(owner.contextId, { scope: owner, session });
  return session;
}
