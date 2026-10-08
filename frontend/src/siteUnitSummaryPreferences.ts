import type { SiteUnitSummaryOptions, SiteUnitSummaryPreferencesOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { validateSiteUnitSummaryOptions } from './siteUnitSummary';

export interface SummaryPreferenceValues { method: number; siteUnitType: number; }
export interface SummaryPreferenceRequest { expected: SummaryPreferenceValues; proposed: SummaryPreferenceValues; }
type Receipt =
  | { kind: 'known'; request: SummaryPreferenceRequest; outcome: SiteUnitSummaryPreferencesOutcome }
  | { kind: 'unknown'; request: SummaryPreferenceRequest; errorMessage: string };
interface View {
  draft: SummaryPreferenceValues | null;
  saved: SummaryPreferenceValues | null;
  busy: boolean;
  blocked: boolean;
  authorityUnknown: boolean;
  error: string;
  draftError: string;
  receipts: Receipt[];
}
const equal = (a: SummaryPreferenceValues, b: SummaryPreferenceValues) =>
  a.method === b.method && a.siteUnitType === b.siteUnitType;
const supported = (value: SummaryPreferenceValues) => [1, 2].includes(value.method) && value.siteUnitType === 1;
export class SiteUnitSummaryPreferencesSession {
  private state: View = { draft: null, saved: null, busy: false, blocked: false,
    authorityUnknown: false, error: '', draftError: '', receipts: [] };
  private listeners = new Set<() => void>();
  private generation = 0;
  constructor(private readonly scope: GoogleEarthScope) { this.scope = structuredClone(scope); }
  view(): View { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener(); return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  private unlocked() {
    if (this.state.busy || this.state.blocked) throw new Error('Wait for and acknowledge the summary preference outcome first.');
  }
  cancelLoad() { this.generation++; }
  beginLoad(): number { this.unlocked(); return ++this.generation; }
  applyLoaded(value: SiteUnitSummaryOptions | null, generation: number): boolean {
    if (generation !== this.generation) return false;
    this.unlocked();
    const source = validateSiteUnitSummaryOptions(value, this.scope.contextId, this.scope.project,
      this.scope.projectPath, this.scope.su, this.scope.suPath);
    this.cancelLoad();
    this.state.saved = { method: source.method, siteUnitType: source.siteUnitType };
    this.state.draft = structuredClone(this.state.saved);
    this.state.authorityUnknown = false; this.state.error = ''; this.state.draftError = ''; this.notify();
    return true;
  }
  editMethod(method: number) {
    this.unlocked();
    if (!this.state.draft) throw new Error('Load saved summary options before editing.');
    this.cancelLoad(); this.state.draft.method = method;
    this.state.draftError = [1, 2].includes(method) ? '' : 'Correct the summary method or Undo the preference draft.';
    this.notify();
  }
  selectNormalSU() {
    this.unlocked();
    if (!this.state.draft) throw new Error('Load saved summary options before selecting normal SU.');
    this.cancelLoad(); this.state.draft.siteUnitType = 1; this.notify();
  }
  undo() {
    this.unlocked();
    if (!this.state.saved) throw new Error('Undo requires known owned saved options; explicitly Load first.');
    this.cancelLoad(); this.state.draft = structuredClone(this.state.saved); this.state.draftError = ''; this.notify();
  }
  async save(port: (request: SummaryPreferenceRequest) => PromiseLike<SiteUnitSummaryPreferencesOutcome | null>): Promise<void> {
    this.unlocked();
    if (!this.state.saved || !this.state.draft) throw new Error('Explicitly Load saved summary options before saving.');
    if (!supported(this.state.draft) || this.state.draftError) throw new Error('Only a valid normal-SU method can be saved; correct the draft first.');
    const request = structuredClone({ expected: this.state.saved, proposed: this.state.draft });
    this.cancelLoad(); this.state.busy = true; this.state.blocked = true; this.state.error = ''; this.notify();
    try {
      const outcome = await port(structuredClone(request));
      const changed = !equal(request.expected, request.proposed);
      if (!outcome || outcome.contextId !== this.scope.contextId || typeof outcome.changed !== 'boolean' ||
          typeof outcome.committed !== 'boolean' || outcome.changed !== outcome.committed ||
          typeof outcome.errorMessage !== 'string' || !wellFormedUTF16(outcome.errorMessage) || outcome.errorMessage.includes('\0') ||
          outcome.committed && !changed || !outcome.errorMessage && outcome.committed !== changed) {
        throw new Error('Incomplete or contradictory summary preference acknowledgement.');
      }
      this.state.receipts.push({ kind: 'known', request: structuredClone(request), outcome: structuredClone(outcome) });
      this.state.error = outcome.errorMessage;
      if (outcome.committed || !outcome.errorMessage) this.state.saved = structuredClone(request.proposed);
    } catch (cause) {
      this.state.error = `Summary preference outcome unknown. Do not replay; acknowledge and explicitly Load before another Save: ${String(cause)}`;
      this.state.saved = null; this.state.authorityUnknown = true;
      this.state.receipts.push({ kind: 'unknown', request: structuredClone(request), errorMessage: this.state.error });
    } finally { this.state.busy = false; this.notify(); }
  }
  acknowledge() {
    if (this.state.busy) throw new Error('Summary preference Save cannot be cancelled; wait for its outcome.');
    this.state.blocked = false; this.notify();
  }
}
const sessions = new Map<string, { scope: GoogleEarthScope; session: SiteUnitSummaryPreferencesSession }>();
export function siteUnitSummaryPreferencesSession(scope: GoogleEarthScope): SiteUnitSummaryPreferencesSession {
  const previous = sessions.get(scope.contextId);
  if (previous) {
    if (!ownedScope(previous.scope, scope)) throw new Error('Summary preference drafts/receipts belong to different owned paths.');
    return previous.session;
  }
  const owner = structuredClone(scope), session = new SiteUnitSummaryPreferencesSession(owner);
  sessions.set(owner.contextId, { scope: owner, session }); return session;
}
