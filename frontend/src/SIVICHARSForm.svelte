<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SIVICombinedService, SIVICollectedService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { SIVICombinedSession } from './siviCombinedSession';
  import { SIVICollectedSession } from './siviCollectedSession';
  import type { SIVICombinedColumn } from './siviCombinedEditor';
  import type { EditorCloseState } from './closeLifecycle';
  import SIVICHARSParentForm from './SIVICHARSParentForm.svelte';
  import SIVICombinedPanel from './SIVICombinedPanel.svelte';
  import SIVICollectedPanel from './SIVICollectedPanel.svelte';

  let { contextId, project, plotNumber, onSaved, onBusyChange }: {
    contextId: string; project: string; plotNumber: string;
    onSaved?: (plot: string) => void | Promise<void>; onBusyChange?: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot: plotNumber }));
  const linkedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COMBINED_EDITING === 'true';
  const collectedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COLLECTED_EDITING === 'true';
  const vegetationEnabled = linkedEnabled || collectedEnabled;
  type Child = 'combined' | 'collected';
  type Operation = 'load' | 'save' | 'undo' | 'retain' | 'prune';
  type ChildService = Pick<typeof SIVICombinedService, 'GetOriginal' | 'SaveReviewed' | 'RestoreReviewed'>;
  let parent = $state<{
    getCloseState(): EditorCloseState;
    saveForClose(): Promise<boolean>;
    canReloadLinkedParent(): boolean;
    canOpenLinkedVegetation(): boolean;
    refreshAfterLinkedSave(): Promise<void>;
  }>();
  let parentBusy = $state(false);
  let opened = $state(false);
  let collectedOpened = $state(false);
  let operationError = $state('');
  let childOperation = $state<{ target: Child; kind: Operation } | null>(null);
  let disposed = false;
  const reads = { combined: new ReadRequests(), collected: new ReadRequests() };
  const refreshNeeded: Record<Child, boolean> = { combined: false, collected: false };
  const child = new SIVICombinedSession(owner.plot, port('combined', SIVICombinedService),
    () => { childView = child.view(); }, true);
  let childView = $state.raw(child.view());
  const collected = new SIVICollectedSession(owner.plot, port('collected', SIVICollectedService),
    () => { collectedView = collected.view(); }, true);
  let collectedView = $state.raw(collected.view());
  const childClose = $derived.by(() => { childView; return child.closeState(); });
  const collectedClose = $derived.by(() => { collectedView; return collected.closeState(); });
  const parentReady = $derived(parent?.canOpenLinkedVegetation() ?? false);
  const combinedPending = $derived(linkedEnabled && Boolean(childClose.unsaved || childClose.busy || childClose.blocked));
  const collectedPending = $derived(collectedEnabled && Boolean(collectedClose.unsaved || collectedClose.busy || collectedClose.blocked));
  const peerBlockedReason = $derived(combinedPending || collectedPending
    ? 'Finish, correct or Undo linked vegetation drafts/recovery before changing the CHARS parent.' : '');
  $effect(() => { onBusyChange?.(parentBusy || linkedEnabled && childView.busy || collectedEnabled && collectedView.busy); });
  onDestroy(() => {
    disposed = true;
    reads.combined.cancelAll(); reads.collected.cancelAll();
    child.dispose(); collected.dispose(); onBusyChange?.(false);
  });

  function port(target: Child, service: ChildService) {
    return {
      read: (extended: boolean) => reads[target].track(service.GetOriginal(owner.contextId, owner.plot, extended)),
      save: (extended: boolean, request: string) => service.SaveReviewed(owner.contextId, owner.plot, extended, request),
      restore: (history: string, action: AuditRestoreAction) => service.RestoreReviewed(owner.contextId, owner.plot, history, action),
      refreshParent: () => refreshAfterChildSave(target),
    };
  }
  function entry(target: Child) {
    return target === 'combined' ? { enabled: linkedEnabled, session: child }
      : { enabled: collectedEnabled, session: collected };
  }
  function requireCleanPeer(target: Child) {
    const peer = entry(target);
    if (!peer.enabled) return;
    const close = peer.session.closeState();
    if (close.unsaved || close.busy || close.blocked) {
      throw new Error(`Linked ${target} drafts/recovery must finish before a peer operation.`);
    }
  }
  async function refreshCleanPeer(target: Child) {
    requireCleanPeer(target);
    const peer = entry(target);
    if (peer.enabled && (peer.session.view().review || refreshNeeded[target])) {
      refreshNeeded[target] = true;
      if (!await peer.session.refreshSource()) throw new Error(peer.session.view().error || `Linked ${target} source refresh failed.`);
      refreshNeeded[target] = false;
    }
  }
  function requireLive() {
    if (disposed) throw new Error('CHARS linked vegetation ownership ended; never replay a completed mutation.');
  }
  async function refreshAfterChildSave(target: Child) {
    requireLive();
    const sibling = target === 'combined' ? 'collected' : 'combined';
    requireCleanPeer(sibling);
    if (!parent) throw new Error('CHARS linked parent ownership ended before refresh.');
    await parent.refreshAfterLinkedSave();
    await refreshCleanPeer(sibling);
    await onSaved?.(owner.plot);
    requireLive();
  }
  async function refreshAfterParentSave() {
    requireLive();
    requireCleanPeer('combined'); requireCleanPeer('collected');
    await refreshCleanPeer('combined');
    await refreshCleanPeer('collected');
    await onSaved?.(owner.plot);
    requireLive();
  }
  async function operate(kind: Operation, target: Child = 'combined'): Promise<boolean> {
    if (childOperation) {
      operationError = 'Wait for the current linked vegetation operation.';
      return false;
    }
    operationError = '';
    try {
      requireLive();
      const active = entry(target);
      requireCleanPeer(target === 'combined' ? 'collected' : 'combined');
      const recovering = kind === 'undo' && active.session.view().blocked && parent?.canReloadLinkedParent();
      if (!active.enabled || !parent?.canOpenLinkedVegetation() && !recovering) {
        throw new Error('Linked vegetation requires both entry gates and a clean, unlocked available CHARS parent.');
      }
      childOperation = { target, kind };
      if (target === 'combined') opened = true;
      else collectedOpened = true;
      const ok = kind === 'load' ? await active.session.load()
        : kind === 'save' ? await active.session.save()
        : kind === 'undo' ? await active.session.undo()
        : await active.session.restore(kind === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
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
      requireCleanPeer('collected');
      if (!linkedEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before editing linked vegetation.');
      }
      child.stage(rowId, column, raw, nullValue);
    } catch (cause) { operationError = `CHARS linked vegetation draft retained: ${String(cause)}`; }
  }
  function cycleCollected(rowId: string) {
    operationError = '';
    try {
      requireLive();
      requireCleanPeer('combined');
      if (!collectedEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before cycling linked Collected.');
      }
      collected.cycle(rowId);
    } catch (cause) { operationError = `CHARS linked Collected draft retained: ${String(cause)}`; }
  }
  function cancelRead(target: Child = 'combined') {
    if (childOperation?.kind !== 'load' || childOperation.target !== target || !entry(target).session.view().busy) {
      operationError = 'Only a linked source read can be cancelled; mutations must finish.';
      return;
    }
    reads[target].cancelAll();
  }
  export function getCloseState(): EditorCloseState {
    childView;
    collectedView;
    const own = parent?.getCloseState();
    const states = [own, linkedEnabled ? child.closeState() : null, collectedEnabled ? collected.closeState() : null]
      .filter((state): state is EditorCloseState => Boolean(state));
    const dirty = states.filter(state => state.unsaved);
    const conflict = dirty.length > 1;
    const busy = states.some(state => state.busy);
    const pending = dirty[0] ?? own;
    const saveReason = conflict ? 'Multiple CHARS owners have drafts; Undo one explicitly before saving.'
      : busy ? 'Wait for the current CHARS parent or linked vegetation operation.'
      : pending?.saveReason ?? 'Load the CHARS parent before saving.';
    return { unsaved: dirty.length > 0, busy,
      blocked: conflict || states.some(state => state.blocked),
      canSave: !conflict && !busy && !!pending?.canSave, saveReason,
      error: operationError || states.find(state => state.error)?.error || null };
  }
  export async function saveForClose(): Promise<boolean> {
    const close = getCloseState();
    if (!close.canSave || !parent) {
      operationError = close.saveReason || 'CHARS parent ownership is unavailable.';
      return false;
    }
    if (linkedEnabled && child.closeState().unsaved) return operate('save');
    if (collectedEnabled && collected.closeState().unsaved) return operate('save', 'collected');
    return parent.saveForClose();
  }
  export function canOpenProjectMetadata() { return false; }
  export function openProjectMetadata() { operationError = 'CHARS project-metadata action remains unavailable.'; }
</script>

{#snippet linkedVegetation()}
  {#if vegetationEnabled}
    <section class="min-w-0 space-y-3" aria-label="CHARS linked vegetation" data-chars-linked-vegetation>
      <h4 class="font-semibold">Linked vegetation: Tree/Shrubs, Herb, Moss/Lichen</h4>
      {#if operationError}<p role="alert" class="text-red-800">{operationError}</p>{/if}
      {#if linkedEnabled}
      {#if !opened}
        <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-linked-open
          disabled={!parentReady || Boolean(childOperation) || collectedPending}
          onclick={() => void operate('load')}>Open linked covers &amp; heights</button>
      {:else}
        {#if childOperation?.target === 'combined' && childOperation.kind === 'load' && childView.busy}
          <button type="button" class="rounded border px-3 py-2" data-chars-linked-cancel onclick={() => cancelRead()}>Cancel linked source read</button>
        {/if}
        <SIVICombinedPanel view={childView}
          disabled={!parentReady || collectedPending} canSave={childClose.canSave}
          onstage={stage} onsave={() => void operate('save')} onundo={() => void operate('undo')}
          onreload={() => void operate('load')}
          onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
      {/if}
      {/if}
      {#if collectedEnabled}
        {#if !collectedOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-collected-open
            disabled={!parentReady || Boolean(childOperation) || combinedPending}
            onclick={() => void operate('load', 'collected')}>Open linked Collected cycles</button>
        {:else}
          {#if childOperation?.target === 'collected' && childOperation.kind === 'load' && collectedView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-collected-cancel
              onclick={() => cancelRead('collected')}>Cancel linked Collected source read</button>
          {/if}
          <SIVICollectedPanel view={collectedView}
            disabled={!parentReady || combinedPending}
            canSave={collectedClose.canSave} oncycle={cycleCollected}
            onsave={() => void operate('save', 'collected')} onundo={() => void operate('undo', 'collected')}
            onreload={() => void operate('load', 'collected')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'collected')} />
        {/if}
      {/if}
      <p class="text-sm text-stone-600">Saved rows use the original PlotNumber links and extended B3/B4/B5 controls.
        Parent, covers/heights and independently gated Collected cycles retain separate drafts and typed histories.
        Same-row histories restore in reverse edit order; changed physical rows refuse restoration.
        Species, identity/lifecycle, automatic totals and unqualified source callbacks remain unavailable.
        Collected Save is an explicit desktop adaptation, not original Click/focus timing parity.</p>
    </section>
  {/if}
{/snippet}

<SIVICHARSParentForm bind:this={parent} contextId={owner.contextId} project={owner.project} plotNumber={owner.plot}
  onSaved={refreshAfterParentSave} onBusyChange={value => { parentBusy = value; }}
  {peerBlockedReason} collectedAvailable={collectedEnabled} linkedVegetation={vegetationEnabled ? linkedVegetation : undefined} />
{#if operationError && !vegetationEnabled}<p role="alert" class="text-red-800">{operationError}</p>{/if}
