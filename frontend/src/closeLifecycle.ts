export type CloseDecision = 'cancel' | 'save' | 'discard';

export interface EditorCloseState {
  unsaved: boolean;
  busy: boolean;
  canSave: boolean;
  saveReason: string;
  error: string | null;
  blocked?: boolean;
}

export function closeDisposition(editor: EditorCloseState | null, contextBusy: boolean): 'busy' | 'dirty' | 'clean' {
  if (contextBusy || editor?.busy || editor?.blocked) return 'busy';
  return editor?.unsaved ? 'dirty' : 'clean';
}
