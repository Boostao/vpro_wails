<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SIVICombinedService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { SIVICombinedSession } from './siviCombinedSession';
  import type { SIVICombinedColumn } from './siviCombinedEditor';
  import type { EditorCloseState } from './closeLifecycle';
  import SIVICHARSParentForm from './SIVICHARSParentForm.svelte';
  import SIVICombinedPanel from './SIVICombinedPanel.svelte';

  let { contextId, project, plotNumber, onSaved, onBusyChange }: {
    contextId: string; project: string; plotNumber: string;
    onSaved?: (plot: string) => void | Promise<void>; onBusyChange?: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot: plotNumber }));
  const linkedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COMBINED_EDITING === 'true';
  let parent = $state<{
    getCloseState(): EditorCloseState;
    saveForClose(): Promise<boolean>;
    canReloadLinkedParent(): boolean;
    canOpenLinkedVegetation(): boolean;
    refreshAfterLinkedSave(): Promise<void>;
  }>();
  let parentBusy = $state(false);
  let opened = $state(false);
  let operationError = $state('');
  let childOperation = $state<'load' | 'save' | 'undo' | 'retain' | 'prune' | null>(null);
  let disposed = false;
  const reads = new ReadRequests();
  const child = new SIVICombinedSession(owner.plot, {
    read: extended => reads.track(SIVICombinedService.GetOriginal(owner.contextId, owner.plot, extended)),
    save: (extended, request) => SIVICombinedService.SaveReviewed(owner.contextId, owner.plot, extended, request),
    restore: (history, action) => SIVICombinedService.RestoreReviewed(owner.contextId, owner.plot, history, action),
    refreshParent: async () => {
      requireLive();
      if (!parent) throw new Error('CHARS linked parent ownership ended before refresh.');
      await parent.refreshAfterLinkedSave();
      await onSaved?.(owner.plot);
      requireLive();
    },
  }, () => { childView = child.view(); }, true);
  let childView = $state.raw(child.view());
  const childClose = $derived.by(() => { childView; return child.closeState(); });
  const parentReady = $derived(parent?.canOpenLinkedVegetation() ?? false);
  const peerBlockedReason = $derived(linkedEnabled && (childClose.unsaved || childClose.busy || childClose.blocked)
    ? 'Finish, correct or Undo linked vegetation drafts/recovery before changing the CHARS parent.' : '');
  $effect(() => { onBusyChange?.(parentBusy || linkedEnabled && childView.busy); });
  onDestroy(() => { disposed = true; reads.cancelAll(); child.dispose(); onBusyChange?.(false); });

  function requireLive() {
    if (disposed) throw new Error('CHARS linked vegetation ownership ended; never replay a completed mutation.');
  }
  async function refreshAfterParentSave() {
    requireLive();
    if (linkedEnabled) {
      const close = child.closeState();
      if (close.unsaved || close.busy || close.blocked) throw new Error('Linked vegetation changed before CHARS parent refresh.');
      if (child.view().review && !await child.refreshSource()) {
        throw new Error(child.view().error || 'Linked vegetation refresh failed after parent operation.');
      }
    }
    await onSaved?.(owner.plot);
    requireLive();
  }
  async function operate(kind: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (childOperation) {
      operationError = 'Wait for the current linked vegetation operation.';
      return false;
    }
    operationError = '';
    try {
      requireLive();
      const recovering = kind === 'undo' && childView.blocked && parent?.canReloadLinkedParent();
      if (!linkedEnabled || !parent?.canOpenLinkedVegetation() && !recovering) {
        throw new Error('Linked vegetation requires both entry gates and a clean, unlocked available CHARS parent.');
      }
      childOperation = kind;
      opened = true;
      const ok = kind === 'load' ? await child.load()
        : kind === 'save' ? await child.save()
        : kind === 'undo' ? await child.undo()
        : await child.restore(kind === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (ok && !disposed) operationError = '';
      return ok;
    } catch (cause) {
      if (!disposed) operationError = `CHARS linked vegetation operation failed; owner retained: ${String(cause)}`;
      return false;
    } finally {
      if (!disposed) childOperation = null;
    }
  }
  function stage(rowId: string, column: SIVICombinedColumn, raw: string, nullValue: boolean) {
    operationError = '';
    try {
      requireLive();
      if (!linkedEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before editing linked vegetation.');
      }
      child.stage(rowId, column, raw, nullValue);
    } catch (cause) { operationError = `CHARS linked vegetation draft retained: ${String(cause)}`; }
  }
  function cancelRead() {
    if (childOperation !== 'load' || !childView.busy) {
      operationError = 'Only a linked source read can be cancelled; mutations must finish.';
      return;
    }
    reads.cancelAll();
  }
  export function getCloseState(): EditorCloseState {
    childView;
    const own = parent?.getCloseState();
    const linked = linkedEnabled ? child.closeState() : null;
    const conflict = !!(own?.unsaved && linked?.unsaved);
    const busy = !!(own?.busy || linked?.busy);
    const pending = linked?.unsaved ? linked : own;
    const saveReason = conflict ? 'Both CHARS owners have drafts; Undo one explicitly before saving.'
      : busy ? 'Wait for the current CHARS parent or linked vegetation operation.'
      : pending?.saveReason ?? 'Load the CHARS parent before saving.';
    return { unsaved: !!(own?.unsaved || linked?.unsaved), busy,
      blocked: conflict || !!(own?.blocked || linked?.blocked),
      canSave: !conflict && !busy && !!pending?.canSave, saveReason,
      error: operationError || own?.error || linked?.error || null };
  }
  export async function saveForClose(): Promise<boolean> {
    const close = getCloseState();
    if (!close.canSave || !parent) {
      operationError = close.saveReason || 'CHARS parent ownership is unavailable.';
      return false;
    }
    return linkedEnabled && child.closeState().unsaved ? operate('save') : parent.saveForClose();
  }
  export function canOpenProjectMetadata() { return false; }
  export function openProjectMetadata() { operationError = 'CHARS project-metadata action remains unavailable.'; }
</script>

{#snippet linkedVegetation()}
  {#if linkedEnabled}
    <section class="min-w-0 space-y-3" aria-label="CHARS linked vegetation" data-chars-linked-vegetation>
      <h4 class="font-semibold">Linked vegetation: Tree/Shrubs, Herb, Moss/Lichen</h4>
      {#if operationError}<p role="alert" class="text-red-800">{operationError}</p>{/if}
      {#if !opened}
        <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-linked-open
          disabled={!parentReady || childView.busy} onclick={() => void operate('load')}>Open linked covers &amp; heights</button>
      {:else}
        {#if childOperation === 'load' && childView.busy}
          <button type="button" class="rounded border px-3 py-2" data-chars-linked-cancel onclick={cancelRead}>Cancel linked source read</button>
        {/if}
        <SIVICombinedPanel view={childView} disabled={!parentReady} canSave={childClose.canSave}
          onstage={stage} onsave={() => void operate('save')} onundo={() => void operate('undo')}
          onreload={() => void operate('load')}
          onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
      {/if}
      <p class="text-sm text-stone-600">Saved rows use the original PlotNumber links and extended B3/B4/B5 controls.
        Covers and heights save together; parent and child drafts have separate typed histories.
        Species, Collected, identity/lifecycle, automatic totals and unqualified source callbacks remain unavailable.</p>
    </section>
  {/if}
{/snippet}

<SIVICHARSParentForm bind:this={parent} contextId={owner.contextId} project={owner.project} plotNumber={owner.plot}
  onSaved={refreshAfterParentSave} onBusyChange={value => { parentBusy = value; }}
  {peerBlockedReason} linkedVegetation={linkedEnabled ? linkedVegetation : undefined} />
{#if operationError && !linkedEnabled}<p role="alert" class="text-red-800">{operationError}</p>{/if}
