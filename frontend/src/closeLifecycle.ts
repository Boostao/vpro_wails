export type CloseDecision = 'cancel' | 'save' | 'discard';

export interface EditorCloseState {
  unsaved: boolean;
  busy: boolean;
  canSave: boolean;
  saveReason: string;
  error: string | null;
}

export function closeDisposition(editor: EditorCloseState | null, contextBusy: boolean): 'busy' | 'dirty' | 'clean' {
  if (contextBusy || editor?.busy) return 'busy';
  return editor?.unsaved ? 'dirty' : 'clean';
}
