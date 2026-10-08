import type { GoogleEarthPreferencesReview, GoogleEarthPreferencesOutcome, GoogleEarthPreferenceValues } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { googleEarthXMLText } from './googleEarthKMLReview';

const literal = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value);
function values(value: GoogleEarthPreferenceValues | null | undefined): value is GoogleEarthPreferenceValues {
  return !!value && literal(value.title) && literal(value.descriptionField);
}
export function validateGoogleEarthPreferences(value: GoogleEarthPreferencesReview | null, scope: GoogleEarthScope): GoogleEarthPreferencesReview {
  if (!value || !ownedScope(value, scope) || !values(value.values)) {
    throw new Error('Saved Google Earth preferences are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return structuredClone(value);
}
export interface GoogleEarthPreferencesView {
  draft: GoogleEarthPreferenceValues;
  saved: GoogleEarthPreferenceValues | null;
  busy: boolean;
  blocked: boolean;
  attempted: boolean;
  outcome: GoogleEarthPreferencesOutcome | null;
  error: string;
  draftError: string;
}
export class GoogleEarthPreferencesSession {
  private state: GoogleEarthPreferencesView = { draft: { title: 'VPro Plot Locations', descriptionField: '' },
    saved: null, busy: false, blocked: false, attempted: false, outcome: null, error: '', draftError: '' };
  private listeners = new Set<() => void>();
  private loadGeneration = 0;
  private physicalFields: string[] | null = null;
  constructor(private readonly scope: GoogleEarthScope) { this.scope = structuredClone(scope); }
  view(): GoogleEarthPreferencesView { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  private unlocked() {
    if (this.state.busy || this.state.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
  }
  private validateDraft() {
    const expected = this.state.saved ?? { title: 'VPro Plot Locations', descriptionField: '' };
    const proposed = this.state.draft;
    this.state.draftError = proposed.title !== expected.title && !googleEarthXMLText(proposed.title)
      ? 'Changed Place Name requires valid Unicode and XML text; correct it or Undo the preference draft.'
      : proposed.descriptionField !== expected.descriptionField &&
          (!literal(proposed.descriptionField) || !proposed.descriptionField || proposed.descriptionField.includes('\0') ||
            this.physicalFields !== null && !this.physicalFields.includes(proposed.descriptionField))
        ? 'Changed description field requires a literal nonempty Unicode name without NUL and current owned physical membership; correct it or Undo the preference draft.'
        : '';
  }
  setPhysicalFields(fields: string[]) {
    this.unlocked();
    this.physicalFields = [...fields]; this.validateDraft(); this.notify();
  }
  edit(draft: GoogleEarthPreferenceValues) {
    this.unlocked();
    this.cancelLoad();
    this.state.draft = structuredClone(draft); this.validateDraft(); this.notify();
  }
  undo() {
    this.unlocked();
    if (!this.state.saved) throw new Error('Undo requires owned saved preferences; explicitly Load them first.');
    this.cancelLoad();
    this.state.draft = structuredClone(this.state.saved); this.state.draftError = ''; this.notify();
  }
  cancelLoad() { this.loadGeneration++; }
  beginLoad(): number { this.unlocked(); return ++this.loadGeneration; }
  applyLoaded(value: GoogleEarthPreferencesReview | null, generation: number): boolean {
    if (generation !== this.loadGeneration) return false;
    this.apply(value); return true;
  }
  apply(value: GoogleEarthPreferencesReview | null) {
    this.unlocked();
    const review = validateGoogleEarthPreferences(value, this.scope);
    this.cancelLoad();
    this.state.saved = structuredClone(review.values); this.state.draft = structuredClone(review.values);
    this.state.draftError = ''; this.notify();
  }
  async save(physicalFields: string[], port: (request: { expected: GoogleEarthPreferenceValues; proposed: GoogleEarthPreferenceValues }) =>
    PromiseLike<GoogleEarthPreferencesOutcome | null>): Promise<void> {
    this.unlocked();
    this.setPhysicalFields(physicalFields);
    if (this.state.draftError) throw new Error(this.state.draftError);
    const expected = this.state.saved, proposed = this.state.draft;
    if (!values(expected) || !values(proposed)) {
      throw new Error('Load saved preferences, correct the changed title, and choose a current physical field for any field change.');
    }
    const request = structuredClone({ expected, proposed });
    this.cancelLoad();
    this.state.busy = true; this.state.blocked = true; this.state.attempted = true;
    this.state.outcome = null; this.state.error = ''; this.notify();
    try {
      const outcome = await port(structuredClone(request));
      if (!outcome || outcome.contextId !== this.scope.contextId || typeof outcome.changed !== 'boolean' ||
          typeof outcome.committed !== 'boolean' || outcome.changed !== outcome.committed || !literal(outcome.errorMessage) ||
          outcome.errorMessage.includes('\0') ||
          !outcome.errorMessage && outcome.committed !== (request.expected.title !== request.proposed.title ||
            request.expected.descriptionField !== request.proposed.descriptionField)) {
        throw new Error('Incomplete or contradictory preference acknowledgement.');
      }
      this.state.outcome = structuredClone(outcome);
      this.state.error = outcome.errorMessage;
      if (outcome.committed || !outcome.errorMessage) this.state.saved = structuredClone(request.proposed);
    } catch (cause) {
      this.state.error = `Preference save outcome unknown. Do not replay; acknowledge and explicitly reload saved preferences before another save: ${String(cause)}`;
      this.state.saved = null;
    } finally {
      this.state.busy = false; this.notify();
    }
  }
  acknowledge() {
    if (this.state.busy) throw new Error('Preference save cannot be cancelled; wait for the outcome.');
    this.state.blocked = false; this.notify();
  }
}
const sessions = new Map<string, { scope: GoogleEarthScope; session: GoogleEarthPreferencesSession }>();
export function googleEarthPreferencesSession(scope: GoogleEarthScope): GoogleEarthPreferencesSession {
  const existing = sessions.get(scope.contextId);
  if (existing) {
    if (!ownedScope(existing.scope, scope)) throw new Error('Preference drafts and receipt belong to different owned paths.');
    return existing.session;
  }
  const owner = structuredClone(scope), session = new GoogleEarthPreferencesSession(owner);
  sessions.set(owner.contextId, { scope: owner, session }); return session;
}
