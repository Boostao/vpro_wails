<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SIVICombinedService, SIVICollectedService, SIVISpeciesService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { SIVICombinedSession } from './siviCombinedSession';
  import { SIVICollectedSession } from './siviCollectedSession';
  import { SIVISpeciesSession } from './siviSpeciesSession';
  import type { SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import type { SIVICombinedColumn } from './siviCombinedEditor';
  import type { EditorCloseState } from './closeLifecycle';
  import SIVICHARSParentForm from './SIVICHARSParentForm.svelte';
  import SIVICombinedPanel from './SIVICombinedPanel.svelte';
  import SIVICollectedPanel from './SIVICollectedPanel.svelte';
  import SIVISpeciesPanel from './SIVISpeciesPanel.svelte';

  let { contextId, project, plotNumber, onSaved, onBusyChange }: {
    contextId: string; project: string; plotNumber: string;
    onSaved?: (plot: string) => void | Promise<void>; onBusyChange?: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot: plotNumber }));
  const linkedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COMBINED_EDITING === 'true';
  const collectedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COLLECTED_EDITING === 'true';
  const speciesEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_SPECIES_EDITING === 'true';
  const vegetationEnabled = linkedEnabled || collectedEnabled || speciesEnabled;
  type Child = 'combined' | 'collected' | 'species';
  const targets: Child[] = ['combined', 'collected', 'species'];
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
  let speciesOpened = $state(false);
  let operationError = $state('');
  let childOperation = $state<{ target: Child; kind: Operation } | null>(null);
  let disposed = false;
  const reads = { combined: new ReadRequests(), collected: new ReadRequests(), species: new ReadRequests() };
  const refreshNeeded: Record<Child, boolean> = { combined: false, collected: false, species: false };
  const child = new SIVICombinedSession(owner.plot, port('combined', SIVICombinedService),
    () => { childView = child.view(); }, true);
  let childView = $state.raw(child.view());
  const collected = new SIVICollectedSession(owner.plot, port('collected', SIVICollectedService),
    () => { collectedView = collected.view(); }, true);
  let collectedView = $state.raw(collected.view());
  const species = new SIVISpeciesSession(owner, {
    ...port('species', SIVISpeciesService),
    read: async (extended: boolean) => {
      const [original, references] = await Promise.all([
        reads.species.track(SIVISpeciesService.GetOriginal(owner.contextId, owner.plot, extended)),
        reads.species.track(SIVISpeciesService.GetReferences(owner.contextId, owner.plot)),
      ]);
      return { original, references };
    },
  }, () => { speciesView = species.view(); }, true);
  let speciesView = $state.raw(species.view());
  const childClose = $derived.by(() => { childView; return child.closeState(); });
  const collectedClose = $derived.by(() => { collectedView; return collected.closeState(); });
  const speciesClose = $derived.by(() => { speciesView; return species.closeState(); });
  const parentReady = $derived(parent?.canOpenLinkedVegetation() ?? false);
  const combinedPending = $derived(linkedEnabled && Boolean(childClose.unsaved || childClose.busy || childClose.blocked));
  const collectedPending = $derived(collectedEnabled && Boolean(collectedClose.unsaved || collectedClose.busy || collectedClose.blocked));
  const speciesPending = $derived(speciesEnabled && Boolean(speciesClose.unsaved || speciesClose.busy || speciesClose.blocked));
  const peerBlockedReason = $derived(combinedPending || collectedPending || speciesPending
    ? 'Finish, correct or Undo linked vegetation drafts/recovery before changing the CHARS parent.' : '');
  $effect(() => { onBusyChange?.(parentBusy || linkedEnabled && childView.busy ||
    collectedEnabled && collectedView.busy || speciesEnabled && speciesView.busy); });
  onDestroy(() => {
    disposed = true;
    for (const target of targets) { reads[target].cancelAll(); entry(target).session.dispose(); }
    onBusyChange?.(false);
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
      : target === 'collected' ? { enabled: collectedEnabled, session: collected }
      : { enabled: speciesEnabled, session: species };
  }
  function requireCleanPeer(target: Child) {
    const peer = entry(target);
    if (!peer.enabled) return;
    const close = peer.session.closeState();
    if (close.unsaved || close.busy || close.blocked) {
      throw new Error(`Linked ${target} drafts/recovery must finish before a peer operation.`);
    }
  }
  function requireCleanPeers(except?: Child) {
    for (const target of targets) if (target !== except) requireCleanPeer(target);
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
    requireCleanPeers(target);
    if (!parent) throw new Error('CHARS linked parent ownership ended before refresh.');
    await parent.refreshAfterLinkedSave();
    for (const peer of targets) if (peer !== target) await refreshCleanPeer(peer);
    await onSaved?.(owner.plot);
    requireLive();
  }
  async function refreshAfterParentSave() {
    requireLive();
    requireCleanPeers();
    for (const target of targets) await refreshCleanPeer(target);
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
      requireCleanPeers(target);
      const recovering = kind === 'undo' && active.session.view().blocked && parent?.canReloadLinkedParent();
      if (!active.enabled || !parent?.canOpenLinkedVegetation() && !recovering) {
        throw new Error('Linked vegetation requires both entry gates and a clean, unlocked available CHARS parent.');
      }
      childOperation = { target, kind };
      if (target === 'combined') opened = true;
      else if (target === 'collected') collectedOpened = true;
      else speciesOpened = true;
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
      requireCleanPeers('combined');
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
      requireCleanPeers('collected');
      if (!collectedEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before cycling linked Collected.');
      }
      collected.cycle(rowId);
    } catch (cause) { operationError = `CHARS linked Collected draft retained: ${String(cause)}`; }
  }
  function changeSpecies(change: () => void) {
    operationError = '';
    try {
      requireLive();
      requireCleanPeers('species');
      if (!speciesEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before editing linked Species.');
      }
      change();
    } catch (cause) { operationError = `CHARS linked Species draft retained: ${String(cause)}`; }
  }
  function stageSpecies(rowId: string, raw: string) { changeSpecies(() => species.stage(rowId, 'Species', raw, false)); }
  function contextSpecies(rowId: string, group: number) { changeSpecies(() => species.context(rowId, group)); }
  function chooseSpecies(rowId: string, kind: SpeciesDecisionKind, selected?: string) {
    changeSpecies(() => species.choose(rowId, kind, selected));
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
    speciesView;
    const own = parent?.getCloseState();
    const states = [own, ...targets.map(target => entry(target).enabled ? entry(target).session.closeState() : null)]
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
    if (speciesEnabled && species.closeState().unsaved) return operate('save', 'species');
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
          disabled={!parentReady || Boolean(childOperation) || collectedPending || speciesPending}
          onclick={() => void operate('load')}>Open linked covers &amp; heights</button>
      {:else}
        {#if childOperation?.target === 'combined' && childOperation.kind === 'load' && childView.busy}
          <button type="button" class="rounded border px-3 py-2" data-chars-linked-cancel onclick={() => cancelRead()}>Cancel linked source read</button>
        {/if}
        <SIVICombinedPanel view={childView}
          disabled={!parentReady || collectedPending || speciesPending} canSave={childClose.canSave}
          onstage={stage} onsave={() => void operate('save')} onundo={() => void operate('undo')}
          onreload={() => void operate('load')}
          onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
      {/if}
      {/if}
      {#if collectedEnabled}
        {#if !collectedOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-collected-open
            disabled={!parentReady || Boolean(childOperation) || combinedPending || speciesPending}
            onclick={() => void operate('load', 'collected')}>Open linked Collected cycles</button>
        {:else}
          {#if childOperation?.target === 'collected' && childOperation.kind === 'load' && collectedView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-collected-cancel
              onclick={() => cancelRead('collected')}>Cancel linked Collected source read</button>
          {/if}
          <SIVICollectedPanel view={collectedView}
            disabled={!parentReady || combinedPending || speciesPending}
            canSave={collectedClose.canSave} oncycle={cycleCollected}
            onsave={() => void operate('save', 'collected')} onundo={() => void operate('undo', 'collected')}
            onreload={() => void operate('load', 'collected')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'collected')} />
        {/if}
      {/if}
      {#if speciesEnabled}
        {#if !speciesOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-species-open
            disabled={!parentReady || Boolean(childOperation) || combinedPending || collectedPending}
            onclick={() => void operate('load', 'species')}>Open linked Species selections</button>
        {:else}
          {#if childOperation?.target === 'species' && childOperation.kind === 'load' && speciesView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-species-cancel
              onclick={() => cancelRead('species')}>Cancel linked Species source/reference read</button>
          {/if}
          <SIVISpeciesPanel view={speciesView} disabled={!parentReady || combinedPending || collectedPending}
            canSave={speciesClose.canSave} onstage={stageSpecies} oncontext={contextSpecies} onchoose={chooseSpecies}
            onsave={() => void operate('save', 'species')} onundo={() => void operate('undo', 'species')}
            onreload={() => void operate('load', 'species')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'species')} />
        {/if}
      {/if}
      <p class="text-sm text-stone-600">Saved rows use the original PlotNumber links and extended B3/B4/B5 controls.
        Parent and independently gated covers/heights, Collected and Species keep separate drafts and typed histories.
        Same-row histories restore in reverse edit order; changed physical rows refuse restoration.
        Species selections require an eligible source group; personal-species creation, identity/lifecycle,
        automatic totals and unqualified source callbacks remain unavailable.
        Reviewed Save is a desktop adaptation, not original Click/NotInList/focus timing parity.</p>
    </section>
  {/if}
{/snippet}

<SIVICHARSParentForm bind:this={parent} contextId={owner.contextId} project={owner.project} plotNumber={owner.plot}
  onSaved={refreshAfterParentSave} onBusyChange={value => { parentBusy = value; }}
  {peerBlockedReason} collectedAvailable={collectedEnabled} speciesAvailable={speciesEnabled}
  linkedVegetation={vegetationEnabled ? linkedVegetation : undefined} />
{#if operationError && !vegetationEnabled}<p role="alert" class="text-red-800">{operationError}</p>{/if}
