import type { EditorCloseState } from './closeLifecycle';
import { wellFormedUTF16 } from './qualityEditor';
import {
  validateSIVIParentOriginal, stageSIVIParent, siviParentDirty, siviParentErrors, siviParentProposal,
  type SIVIParentColumn, type SIVIParentDrafts, type SIVIParentInput, type SIVIParentOriginal, type SIVIParentProposal,
} from './siviParentEditor';

export interface SIVIParentView {
  editorId: string;
  original: SIVIParentOriginal;
  drafts: SIVIParentDrafts;
  error: string | null;
}

// Owned outside component lifetime; this preparation has no write port.
export class SIVIParentSession {
  private state: SIVIParentView;
  private disposed = false;

  constructor(editorId: string, original: SIVIParentOriginal) {
    if (typeof editorId !== 'string' || !editorId || !wellFormedUTF16(editorId) || editorId.includes('\0')) {
      throw new Error('SIVI parent drafts require an explicit valid editor identity.');
    }
    this.state = { editorId, original: validateSIVIParentOriginal(original), drafts: {}, error: null };
  }

  private requireOwner(editorId: string) {
    if (this.disposed || editorId !== this.state.editorId) throw new Error('SIVI parent editor ownership changed; drafts were not cleared.');
  }

  view(): SIVIParentView {
    return structuredClone(this.state);
  }

  closeState(): EditorCloseState {
    const errors = siviParentErrors(this.state.drafts);
    return { unsaved: siviParentDirty(this.state.drafts), busy: false, blocked: errors.length > 0,
      canSave: false, saveReason: errors[0] ?? 'SIVI parent writes remain unavailable; this is an unwired draft model.',
      error: this.state.error };
  }

  stage(editorId: string, column: SIVIParentColumn, input: SIVIParentInput) {
    this.requireOwner(editorId);
    this.state.drafts = stageSIVIParent(this.state.original, this.state.drafts, column, input);
    this.state.error = siviParentErrors(this.state.drafts)[0] ?? null;
  }

  proposal(editorId: string): SIVIParentProposal {
    this.requireOwner(editorId);
    return siviParentProposal(this.state.original, this.state.drafts);
  }

  undo(editorId: string) {
    this.requireOwner(editorId);
    this.state.drafts = {};
    this.state.error = null;
  }

  replaceOriginal(editorId: string, original: SIVIParentOriginal) {
    this.requireOwner(editorId);
    if (siviParentDirty(this.state.drafts)) throw new Error('Finish or Undo SIVI parent drafts before switching original context.');
    this.state.original = validateSIVIParentOriginal(original);
    this.state.drafts = {};
    this.state.error = null;
  }

  dispose(editorId: string) {
    this.requireOwner(editorId);
    if (siviParentDirty(this.state.drafts)) throw new Error('Undo SIVI parent drafts before ending editor ownership.');
    this.disposed = true;
  }
}
