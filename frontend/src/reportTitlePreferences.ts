import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';

export interface ReportTitlePreferenceValues { title: string }
export interface ReportTitlePreferencesReview extends GoogleEarthScope { values: ReportTitlePreferenceValues }
export interface ReportTitlePreferencesOutcome {
  contextId: string; changed: boolean; committed: boolean; errorMessage: string;
}
const literal = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value);
export function validateReportTitlePreferences(value: ReportTitlePreferencesReview | null, scope: GoogleEarthScope, label: string): ReportTitlePreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !literal(value.values.title)) {
    throw new Error(`Saved ${label} title is missing, malformed or belongs to another owned context; no defaults applied.`);
  }
  return { ...scope, values: { title: value.values.title } };
}
export interface ReportTitlePreferenceRequest {
  expected: ReportTitlePreferenceValues;
  proposed: ReportTitlePreferenceValues;
}
export interface ReportPreferenceRequest<Values> { expected: Values; proposed: Values }
export type ReportPreferenceReceipt<Values> =
  | { kind: 'known'; request: ReportPreferenceRequest<Values>; outcome: ReportTitlePreferencesOutcome }
  | { kind: 'unknown'; contextId: string; request: ReportPreferenceRequest<Values>; errorMessage: string };
export interface ReportPreferencesView<Values, Draft> {
  draft: Draft;
  initialized: boolean;
  saved: Values | null;
  busy: boolean;
  blocked: boolean;
  authorityUnknown: boolean;
  outcome: ReportTitlePreferencesOutcome | null;
  receipts: ReportPreferenceReceipt<Values>[];
  error: string;
  draftError: string;
}
export type ReportTitlePreferenceReceipt = ReportPreferenceReceipt<ReportTitlePreferenceValues>;
export type ReportTitlePreferencesView = ReportPreferencesView<ReportTitlePreferenceValues, ReportTitlePreferenceValues>;
interface ReportPreferencePolicy<Values, Draft, Review> {
  initialDraft: Draft;
  savedLabel: string;
  review: (value: Review | null, scope: GoogleEarthScope) => Values;
  draft: (saved: Values) => Draft;
  validate: (draft: Draft, saved: Values | null) => string;
  proposed: (draft: Draft) => Values;
  equal: (a: Values, b: Values) => boolean;
}
export class ReportPreferencesSession<Values, Draft, Review> {
  private state: ReportPreferencesView<Values, Draft>;
  private listeners = new Set<() => void>();
  private loadGeneration = 0;
  constructor(private readonly scope: GoogleEarthScope, private readonly policy: ReportPreferencePolicy<Values, Draft, Review>) {
    this.scope = structuredClone(scope);
    this.state = { draft: structuredClone(policy.initialDraft), initialized: false, saved: null,
      busy: false, blocked: false, authorityUnknown: false, outcome: null, receipts: [], error: '', draftError: '' };
  }
  view(): ReportPreferencesView<Values, Draft> { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener();
    return () => this.listeners.delete(listener);
  }
  protected notify() { for (const listener of this.listeners) listener(); }
  private unlocked() {
    if (this.state.busy || this.state.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
  }
  protected editDraft(draft: Draft) {
    this.unlocked(); this.cancelLoad(); this.state.draft = structuredClone(draft); this.state.initialized = true;
    this.state.draftError = this.policy.validate(this.state.draft, this.state.saved);
    this.notify();
  }
  undo() {
    this.unlocked();
    if (!this.state.saved) throw new Error(`Undo requires known owned ${this.policy.savedLabel}; explicitly Load it first.`);
    this.cancelLoad();
    this.state.draft = this.policy.draft(this.state.saved); this.state.initialized = true;
    this.state.draftError = ''; this.notify();
  }
  cancelLoad() { this.loadGeneration++; }
  beginLoad(): number { this.unlocked(); return ++this.loadGeneration; }
  applyLoaded(value: Review | null, generation: number, preserveDraft = false): boolean {
    if (generation !== this.loadGeneration) return false;
    this.apply(value, preserveDraft); return true;
  }
  apply(value: Review | null, preserveDraft = false) {
    this.unlocked();
    const values = this.policy.review(value, this.scope);
    this.cancelLoad(); this.state.saved = structuredClone(values);
    if (!preserveDraft) this.state.draft = this.policy.draft(values);
    this.state.initialized = true;
    this.state.draftError = preserveDraft ? this.policy.validate(this.state.draft, this.state.saved) : '';
    this.state.authorityUnknown = false; this.state.error = ''; this.notify();
  }
  async save(port: (request: ReportPreferenceRequest<Values>) =>
    PromiseLike<ReportTitlePreferencesOutcome | null>): Promise<void> {
    this.unlocked();
    if (this.state.draftError) throw new Error(this.state.draftError);
    if (!this.state.saved) throw new Error(`Explicitly Load ${this.policy.savedLabel} before saving.`);
    const draftError = this.policy.validate(this.state.draft, this.state.saved);
    if (draftError) throw new Error(draftError);
    const request = structuredClone({ expected: this.state.saved, proposed: this.policy.proposed(this.state.draft) });
    const changed = !this.policy.equal(request.expected, request.proposed);
    this.cancelLoad(); this.state.busy = true; this.state.blocked = true; this.state.outcome = null; this.state.error = ''; this.notify();
    try {
      const outcome = await port(structuredClone(request));
      if (!outcome || outcome.contextId !== this.scope.contextId || typeof outcome.changed !== 'boolean' ||
          typeof outcome.committed !== 'boolean' || outcome.changed !== outcome.committed || !literal(outcome.errorMessage) ||
          outcome.errorMessage.includes('\0') ||
          outcome.committed && !changed || !outcome.errorMessage && outcome.committed !== changed) {
        throw new Error('Incomplete or contradictory preference acknowledgement.');
      }
      this.state.outcome = structuredClone(outcome);
      this.state.receipts.push({ kind: 'known', request: structuredClone(request), outcome: structuredClone(outcome) });
      this.state.error = outcome.errorMessage;
      if (outcome.committed || !outcome.errorMessage) this.state.saved = structuredClone(request.proposed);
    } catch (cause) {
      this.state.error = `Preference save outcome unknown. Do not replay; acknowledge and explicitly Load ${this.policy.savedLabel} before another Save: ${String(cause)}`;
      this.state.saved = null; this.state.authorityUnknown = true;
      this.state.receipts.push({ kind: 'unknown', contextId: this.scope.contextId,
        request: structuredClone(request), errorMessage: this.state.error });
    } finally {
      this.state.busy = false; this.notify();
    }
  }
  acknowledge() {
    if (this.state.busy) throw new Error('Preference Save cannot be cancelled; wait for its outcome.');
    this.state.blocked = false; this.notify();
  }
}
export class ReportTitlePreferencesSession extends ReportPreferencesSession<ReportTitlePreferenceValues, ReportTitlePreferenceValues, ReportTitlePreferencesReview> {
  constructor(scope: GoogleEarthScope, label: string) {
    super(scope, {
      initialDraft: { title: '' }, savedLabel: 'saved title',
      review: (value, owner) => validateReportTitlePreferences(value, owner, label).values,
      draft: saved => ({ title: saved.title }),
      validate: (draft, saved) => draft.title !== saved?.title && (!literal(draft.title) || draft.title.includes('\0'))
        ? 'Changed title requires complete Unicode without NUL; correct it or Undo the draft.' : '',
      proposed: draft => ({ title: draft.title }),
      equal: (a, b) => a.title === b.title,
    });
  }
  edit(title: string) { this.editDraft({ title }); }
}
export function reportPreferencesRegistry<T>(create: (scope: GoogleEarthScope) => T): (scope: GoogleEarthScope) => T {
  const sessions = new Map<string, { scope: GoogleEarthScope; session: T }>();
  return scope => {
    const existing = sessions.get(scope.contextId);
    if (existing) {
      if (!ownedScope(existing.scope, scope)) throw new Error('Preference drafts and receipts belong to different owned paths.');
      return existing.session;
    }
    const owner = structuredClone(scope), session = create(owner);
    sessions.set(owner.contextId, { scope: owner, session }); return session;
  };
}
export const reportTitlePreferencesRegistry = reportPreferencesRegistry;
