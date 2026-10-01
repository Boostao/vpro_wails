<script lang="ts">
  import type { CloseDecision } from './closeLifecycle';

  let { requestId, working, canSave, saveReason, error, onrespond, purpose = 'close' }: {
    requestId: string;
    working: boolean;
    canSave: boolean;
    saveReason: string;
    error: string;
    onrespond: (decision: CloseDecision) => Promise<void>;
    purpose?: 'close' | 'context';
  } = $props();
  let dialog: HTMLDialogElement;

  $effect(() => {
    if (requestId && !dialog.open) dialog.showModal();
    else if (!requestId && dialog.open) dialog.close();
  });
</script>

<dialog bind:this={dialog} aria-labelledby="close-title" aria-describedby="close-description"
  oncancel={(event) => { event.preventDefault(); if (!working) void onrespond('cancel'); }}>
  <h2 id="close-title">{purpose === 'close' ? 'Save changes before closing VPRO?' : 'Save changes before changing context?'}</h2>
  <p id="close-description">{purpose === 'close'
    ? 'FS882 has an unsaved draft or incomplete entry. Cancel keeps the application open. Discard closes without saving this draft; completed child edits remain saved.'
    : 'FS882 has an unsaved draft or incomplete entry. Save writes to the current project before switching. Cancel keeps the current context and draft. Discard removes this draft only after the switch succeeds; completed child edits remain saved.'}</p>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if !canSave}<p role="status">{saveReason}</p>{/if}
  <div class="actions">
    <button type="button" disabled={working} onclick={() => onrespond('cancel')}>Cancel</button>
    <button type="button" disabled={working || !canSave} onclick={() => onrespond('save')}>{purpose === 'close' ? 'Save and close' : 'Save and continue'}</button>
    <button type="button" disabled={working} onclick={() => onrespond('discard')}>{purpose === 'close' ? 'Discard draft and close' : 'Discard draft and continue'}</button>
  </div>
</dialog>

<style>
  dialog { max-width: 38rem; padding: 1.5rem; border: 1px solid #a8a29e; border-radius: 0.5rem; }
  dialog::backdrop { background: #0007; }
  h2 { font-size: 1.1rem; font-weight: 700; margin-bottom: 0.6rem; }
  p { margin: 0.5rem 0; }
  .actions { display: flex; flex-wrap: wrap; gap: 0.7rem; margin-top: 1rem; }
  button { padding: 0.45rem 0.7rem; border: 1px solid #a8a29e; border-radius: 0.3rem; background: #fff; }
  button:disabled { opacity: 0.5; }
  button:focus-visible { outline: 3px solid #047857; outline-offset: 2px; }
  [role="alert"] { color: #991b1b; }
</style>
