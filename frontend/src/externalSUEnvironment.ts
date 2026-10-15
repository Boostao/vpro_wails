import type { ExternalSUEnvironmentReview, ExternalSUEnvironmentOutcome, ExternalSUEnvironmentAttemptOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { validateSiteUnitEnvironmentReview, validateSiteUnitEnvironmentResult } from './siteUnitEnvironment';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { ReadRequests, type CancellableRead } from './readRequests';
import type { EditorCloseState } from './closeLifecycle';
import type { TwoPageParentForm } from './twoPageParentSession';

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const text = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
const exact = (value: object, keys: string[]) => !Array.isArray(value) &&
  Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));

export function validateExternalSUEnvironmentReview(value: ExternalSUEnvironmentReview | null, scope: GoogleEarthScope): ExternalSUEnvironmentReview {
  value = value ? structuredClone(value) : null;
  if (!value || !exact(value, ['review', 'projectPath', 'suPath', 'sourceHash', 'approvalHash', 'sessionId']) ||
      !value.review || !ownedScope({ ...value, contextId: value.review.contextId, project: value.review.project, su: value.review.su }, scope) ||
      !scope.projectPath || !scope.suPath || scope.su === 'None' ||
      !hash(value.sourceHash) || !hash(value.approvalHash) || !text(value.sessionId) || !value.sessionId || value.sessionId.length > 128) {
    throw new Error('External SU Into Env review has incomplete or foreign physical paths, owner or source approval.');
  }
  validateSiteUnitEnvironmentReview(value.review, scope.contextId, scope.project, scope.su, scope.projectPath);
  return value;
}

export function validateExternalSUEnvironmentOutcome(value: ExternalSUEnvironmentOutcome | null, review: ExternalSUEnvironmentReview): ExternalSUEnvironmentOutcome {
  value = value ? structuredClone(value) : null;
  if (!value || !exact(value, ['status', 'receipt', 'errorMessage']) || !text(value.errorMessage) ||
      !['committed', 'notCommitted', 'unknown'].includes(value.status) ||
      value.status !== 'committed' && (value.receipt !== null || value.errorMessage === '')) {
    throw new Error('External SU Into Env response cannot certify its exact request; never replay.');
  }
  if (value.status === 'committed') {
    if (!value.receipt || !exact(value.receipt, ['changedRows', 'changedCells', 'historyId'])) {
      throw new Error('Committed external transfer lacks its complete immutable receipt.');
    }
    validateSiteUnitEnvironmentResult(value.receipt, { review: review.review, confirmed: true });
  }
  return value;
}

export interface ExternalSUEnvironmentRequest {
  approvalHash: string;
  requestId: string;
  sessionId: string;
  confirmed: boolean;
}
export interface ExternalSUEnvironmentView {
  busy: boolean;
  blocked: boolean;
  phase: 'idle' | 'transferring' | 'recovering';
  request: ExternalSUEnvironmentRequest | null;
  outcome: ExternalSUEnvironmentOutcome | null;
  error: string;
}

export class ExternalSUEnvironmentSession {
  private readonly scope: GoogleEarthScope;
  private state: ExternalSUEnvironmentView = { busy: false, blocked: false, phase: 'idle', request: null, outcome: null, error: '' };
  private review: ExternalSUEnvironmentReview | null = null;
  private reviewForm: TwoPageParentForm | null = null;
  private requestForm: TwoPageParentForm | null = null;
  private generation = 0;
  private readonly reads = new ReadRequests();
  private readonly listeners = new Set<() => void>();
  constructor(scope: GoogleEarthScope) { this.scope = structuredClone(scope); }
  reviewSourceForm(form: TwoPageParentForm | null) {
    if (this.state.busy || this.state.blocked) throw new Error('Retained transfer source identity cannot change before recovery and acknowledgement.');
    if (form !== null && form !== 'FS882-8x6XL' && form !== 'FS882-8x6XL-CHARS') {
      throw new Error('Exact normal/CHARS source form is required.');
    }
    this.reviewForm = form;
  }
  requestSourceForm(): TwoPageParentForm | null { return this.requestForm; }
  view(): ExternalSUEnvironmentView { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  closeState(): EditorCloseState {
    return { unsaved: this.state.blocked, busy: this.state.busy, blocked: this.state.blocked, canSave: false,
      error: this.state.blocked ? this.state.error || null : null,
      saveReason: this.state.blocked ? 'Recover and acknowledge the exact retained SU Into Env attempt before Save, Lock or close; never replay.' : '' };
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++;
    this.state.busy = false; this.state.phase = 'idle';
    this.state.error = 'Transfer acknowledgement cancelled. Retain the exact request and recover readonly; never replay.';
    this.reads.cancelAll(); this.notify();
  }
  private accept(value: ExternalSUEnvironmentOutcome | null) {
    if (!this.review) throw new Error('Retained transfer lost its approved source.');
    const checked = validateExternalSUEnvironmentOutcome(value, this.review);
    this.state.outcome = checked.status === 'unknown' ? null : checked;
    this.state.error = checked.errorMessage;
  }
  async transfer(review: ExternalSUEnvironmentReview,
    port: (request: ExternalSUEnvironmentRequest) => CancellableRead<ExternalSUEnvironmentOutcome | null>,
    requestId = crypto.randomUUID()): Promise<void> {
    if (this.state.busy || this.state.blocked) throw new Error('Resolve and acknowledge the prior retained transfer before a new attempt.');
    const checked = validateExternalSUEnvironmentReview(review, this.scope);
    if (!checked.review.changes?.length || !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(requestId)) {
      throw new Error('Transfer requires nonempty reviewed changes and a stable lowercase UUIDv4.');
    }
    this.review = checked;
    this.requestForm = this.reviewForm;
    const request: ExternalSUEnvironmentRequest = { approvalHash: checked.approvalHash, sessionId: checked.sessionId, requestId, confirmed: true };
    const generation = ++this.generation;
    this.state = { busy: true, blocked: true, phase: 'transferring', request, outcome: null, error: '' };
    this.notify();
    try {
      const value = await this.reads.track(port(structuredClone(request)));
      if (generation !== this.generation) return;
      this.accept(value);
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Transfer outcome remains unknown; retain the request and recover readonly, never replay: ${String(cause)}`;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.state.phase = 'idle'; this.notify(); }
    }
  }
  async recover(port: (request: ExternalSUEnvironmentRequest) => CancellableRead<ExternalSUEnvironmentAttemptOutcome | null>): Promise<void> {
    if (!this.state.request || !this.state.blocked || this.state.busy) throw new Error('Readonly recovery requires the unresolved retained transfer request.');
    const request = structuredClone(this.state.request), generation = ++this.generation;
    this.state.busy = true; this.state.phase = 'recovering'; this.state.error = ''; this.notify();
    try {
      const value = await this.reads.track(port(request));
      if (generation !== this.generation) return;
      const checked = value ? structuredClone(value) : null;
      if (!checked || !exact(checked, ['contextId', 'requestId', 'status', 'reason', 'outcome']) ||
          checked.contextId !== this.scope.contextId || checked.requestId !== request.requestId || !text(checked.reason) || !checked.reason) {
        throw new Error('Readonly recovery returned a foreign or incomplete transfer identity.');
      }
      if (checked.status === 'running' && checked.outcome === null) this.state.error = checked.reason;
      else if (checked.status === 'completed') this.accept(checked.outcome);
      else throw new Error('Readonly recovery cannot certify the retained transfer.');
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Transfer outcome remains unknown; never replay: ${String(cause)}`;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.state.phase = 'idle'; this.notify(); }
    }
  }
  acknowledge() {
    if (this.state.busy || !this.state.outcome) throw new Error('Recover the exact unknown transfer before acknowledgement.');
    this.state.blocked = false; this.state.error = ''; this.notify();
  }
}

const sessions = new Map<string, { scope: GoogleEarthScope; session: ExternalSUEnvironmentSession }>();
const activityListeners = new Set<(activity: { busy: boolean; blocked: boolean }) => void>();
function notifyActivity() {
  const views = [...sessions.values()].map(entry => entry.session.view());
  const activity = { busy: views.some(view => view.busy), blocked: views.some(view => view.blocked) };
  for (const listener of activityListeners) listener(activity);
}
export function subscribeExternalSUEnvironmentActivity(listener: (activity: { busy: boolean; blocked: boolean }) => void): () => void {
  activityListeners.add(listener); notifyActivity();
  return () => activityListeners.delete(listener);
}
export function externalSUEnvironmentSession(scope: GoogleEarthScope): ExternalSUEnvironmentSession {
  const existing = sessions.get(scope.contextId);
  if (existing) {
    if (!ownedScope(existing.scope, scope)) throw new Error('Retained transfer belongs to different owned physical paths.');
    return existing.session;
  }
  const owner = structuredClone(scope), session = new ExternalSUEnvironmentSession(owner);
  sessions.set(scope.contextId, { scope: owner, session });
  session.subscribe(notifyActivity);
  return session;
}
