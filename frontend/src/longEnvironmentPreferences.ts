import type { LongEnvironmentPreferencesReview, LongEnvironmentPreferencesOutcome, LongEnvironmentPreferenceValues } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';

const literal = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value);
export function validateLongEnvironmentPreferences(value: LongEnvironmentPreferencesReview | null, scope: GoogleEarthScope): LongEnvironmentPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !literal(value.values.title)) {
    throw new Error('Saved Long Environment title is missing, malformed or belongs to another owned context; no defaults applied.');
  }
  return structuredClone(value);
}
export interface LongEnvironmentPreferenceRequest {
  expected: LongEnvironmentPreferenceValues;
  proposed: LongEnvironmentPreferenceValues;
}
export type LongEnvironmentPreferenceReceipt =
  | { kind: 'known'; request: LongEnvironmentPreferenceRequest; outcome: LongEnvironmentPreferencesOutcome }
  | { kind: 'unknown'; contextId: string; request: LongEnvironmentPreferenceRequest; errorMessage: string };
export interface LongEnvironmentPreferencesView {
  draft: LongEnvironmentPreferenceValues;
  initialized: boolean;
  saved: LongEnvironmentPreferenceValues | null;
  busy: boolean;
  blocked: boolean;
  authorityUnknown: boolean;
  outcome: LongEnvironmentPreferencesOutcome | null;
  receipts: LongEnvironmentPreferenceReceipt[];
  error: string;
  draftError: string;
}
export class LongEnvironmentPreferencesSession {
  private state: LongEnvironmentPreferencesView = { draft: { title: '' }, initialized: false, saved: null,
    busy: false, blocked: false, authorityUnknown: false, outcome: null, receipts: [], error: '', draftError: '' };
  private listeners = new Set<() => void>();
  private loadGeneration = 0;
  constructor(private readonly scope: GoogleEarthScope) { this.scope = structuredClone(scope); }
  view(): LongEnvironmentPreferencesView { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  private unlocked() {
    if (this.state.busy || this.state.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
  }
  edit(title: string) {
    this.unlocked(); this.cancelLoad(); this.state.draft = { title }; this.state.initialized = true;
    this.state.draftError = title !== this.state.saved?.title && (!literal(title) || title.includes('\0'))
      ? 'Changed title requires complete Unicode without NUL; correct it or Undo the draft.' : '';
    this.notify();
  }
  undo() {
    this.unlocked();
    if (!this.state.saved) throw new Error('Undo requires known owned saved title; explicitly Load it first.');
    this.cancelLoad();
    this.state.draft = structuredClone(this.state.saved); this.state.initialized = true;
    this.state.draftError = ''; this.notify();
  }
  cancelLoad() { this.loadGeneration++; }
  beginLoad(): number { this.unlocked(); return ++this.loadGeneration; }
  applyLoaded(value: LongEnvironmentPreferencesReview | null, generation: number): boolean {
    if (generation !== this.loadGeneration) return false;
    this.apply(value); return true;
  }
  apply(value: LongEnvironmentPreferencesReview | null) {
    this.unlocked();
    const review = validateLongEnvironmentPreferences(value, this.scope);
    this.cancelLoad(); this.state.saved = structuredClone(review.values); this.state.draft = structuredClone(review.values);
    this.state.initialized = true;
    this.state.draftError = ''; this.state.authorityUnknown = false; this.state.error = ''; this.notify();
  }
  async save(port: (request: LongEnvironmentPreferenceRequest) =>
    PromiseLike<LongEnvironmentPreferencesOutcome | null>): Promise<void> {
    this.unlocked();
    if (this.state.draftError) throw new Error(this.state.draftError);
    if (!this.state.saved) throw new Error('Explicitly Load saved title before saving.');
    const request = structuredClone({ expected: this.state.saved, proposed: this.state.draft });
    this.cancelLoad(); this.state.busy = true; this.state.blocked = true; this.state.outcome = null; this.state.error = ''; this.notify();
    try {
      const outcome = await port(structuredClone(request));
      if (!outcome || outcome.contextId !== this.scope.contextId || typeof outcome.changed !== 'boolean' ||
          typeof outcome.committed !== 'boolean' || outcome.changed !== outcome.committed || !literal(outcome.errorMessage) ||
          outcome.errorMessage.includes('\0') ||
          outcome.committed && request.expected.title === request.proposed.title ||
          !outcome.errorMessage && outcome.committed !== (request.expected.title !== request.proposed.title)) {
        throw new Error('Incomplete or contradictory preference acknowledgement.');
      }
      this.state.outcome = structuredClone(outcome);
      this.state.receipts.push({ kind: 'known', request: structuredClone(request), outcome: structuredClone(outcome) });
      this.state.error = outcome.errorMessage;
      if (outcome.committed || !outcome.errorMessage) this.state.saved = structuredClone(request.proposed);
    } catch (cause) {
      this.state.error = `Preference save outcome unknown. Do not replay; acknowledge and explicitly Load saved title before another Save: ${String(cause)}`;
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
const sessions = new Map<string, { scope: GoogleEarthScope; session: LongEnvironmentPreferencesSession }>();
export function longEnvironmentPreferencesSession(scope: GoogleEarthScope): LongEnvironmentPreferencesSession {
  const existing = sessions.get(scope.contextId);
  if (existing) {
    if (!ownedScope(existing.scope, scope)) throw new Error('Preference drafts and receipts belong to different owned paths.');
    return existing.session;
  }
  const owner = structuredClone(scope), session = new LongEnvironmentPreferencesSession(owner);
  sessions.set(owner.contextId, { scope: owner, session }); return session;
}
