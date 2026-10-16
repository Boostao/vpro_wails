<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SIVICombinedService, SIVICollectedService, SIVISpeciesService, SIVIIdentityService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { SIVICombinedSession } from './siviCombinedSession';
  import { SIVICollectedSession } from './siviCollectedSession';
  import { SIVISpeciesSession } from './siviSpeciesSession';
  import { SIVIIdentitySession } from './siviIdentitySession';
  import type { SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import type { SIVICombinedColumn } from './siviCombinedEditor';
  import type { EditorCloseState } from './closeLifecycle';
  import SIVICHARSParentForm from './SIVICHARSParentForm.svelte';
  import SIVICombinedPanel from './SIVICombinedPanel.svelte';
  import SIVICollectedPanel from './SIVICollectedPanel.svelte';
  import SIVISpeciesPanel from './SIVISpeciesPanel.svelte';
  import SIVIIdentityPanel from './SIVIIdentityPanel.svelte';
  import SIVICreationPanel from './SIVICreationPanel.svelte';
  import { siviCreationSession as ownedCreationSession } from './siviCreationSessions';
  import type { SIVICreationDraft, SIVICreationForm } from './siviCreationEditor';
  import SIVIDeletionPanel from './SIVIDeletionPanel.svelte';
  import SIVIDeletionRestorationPanel from './SIVIDeletionRestorationPanel.svelte';
  import SIVICreationUndoPanel from './SIVICreationUndoPanel.svelte';
  import { siviDeletionSession as ownedDeletionSession,
    siviDeletionRestorationSession as ownedRestorationSession } from './siviDeletionSessions';
  import { siviCreationUndoSession as ownedCreationUndoSession } from './siviCreationUndoSessions';
  import ProjectMetadataEditor from './ProjectMetadataEditor.svelte';
  import { bindContextPlots } from './contextPlots';

  let { contextId, project, plotNumber, onSaved, onBusyChange }: {
    contextId: string; project: string; plotNumber: string;
    onSaved?: (plot: string) => void | Promise<void>; onBusyChange?: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot: plotNumber }));
  const metadataEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_PROJECT_METADATA_EDITING === 'true';
  const metadataRestorationEnabled = import.meta.env.VITE_PROJECT_METADATA_RESTORE === 'true';
  const metadataClient = metadataEnabled ? bindContextPlots(owner.contextId) : null;
  let metadataOpen = $state(false);
  let metadataBusy = $state(false);
  let metadataEditor = $state.raw<ProjectMetadataEditor>();
  const linkedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COMBINED_EDITING === 'true';
  const collectedEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_COLLECTED_EDITING === 'true';
  const speciesEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_SPECIES_EDITING === 'true';
  const identityEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_IDENTITY_EDITING === 'true';
  const creationEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_CREATION_WRITING === 'true';
  const deletionEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_DELETION_WRITING === 'true';
  const restorationEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_DELETION_RESTORING === 'true';
  const creationUndoEnabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true' &&
    import.meta.env.VITE_SIVI_CREATION_UNDO === 'true';
  const creationForms: SIVICreationForm[] = ['SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI'];
  const vegetationEnabled = linkedEnabled || collectedEnabled || speciesEnabled || identityEnabled ||
    creationEnabled || deletionEnabled || restorationEnabled || creationUndoEnabled;
  type Child = 'combined' | 'collected' | 'species' | 'identity';
  type Lifecycle = 'deletion' | 'restoration' | 'creationUndo';
  const lifecycleKinds: Lifecycle[] = ['deletion', 'restoration', 'creationUndo'];
  const targets: Child[] = ['combined', 'collected', 'species', 'identity'];
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
  let identityOpened = $state(false);
  let operationError = $state('');
  let childOperation = $state<{ target: Child; kind: Operation } | null>(null);
  let disposed = false;
  const reads = { combined: new ReadRequests(), collected: new ReadRequests(), species: new ReadRequests(), identity: new ReadRequests() };
  const refreshNeeded: Record<Child, boolean> = { combined: false, collected: false, species: false, identity: false };
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
  const identity = new SIVIIdentitySession(owner.plot, port('identity', SIVIIdentityService),
    () => { identityView = identity.view(); }, true);
  let identityView = $state.raw(identity.view());
  const creation = creationEnabled ? ownedCreationSession(owner) : null;
  let creationRevision = $state(0);
  let creationHostBusy = $state(false);
  let creationRefreshedRevision = $state(-1);
  $effect(() => creation?.subscribe(() => untrack(() => creationRevision++)));
  const creationView = $derived.by(() => { creationRevision; return creation?.view(); });
  const creationRefreshNeeded = $derived(Boolean(creationView?.receipt &&
    creationView.revision !== creationRefreshedRevision));
  const foreignCreationDraft = $derived(Boolean(creationView?.draft && !creationForms.includes(creationView.draft.form)));
  const creationClose = $derived.by((): EditorCloseState | null => {
    creationRevision;
    if (!creation) return null;
    const close = creation.closeState();
    const reason = creationHostBusy ? 'Wait for the verified creation parent and peer refresh.'
      : foreignCreationDraft ? 'Return to the retained source-form editor or explicitly Undo its creation draft.'
      : creationRefreshNeeded ? 'Creation is verified. Refresh parent and peers without repeating Create.'
      : close.saveReason;
    return { ...close, unsaved: close.unsaved || creationRefreshNeeded,
      busy: close.busy || creationHostBusy, blocked: close.blocked || foreignCreationDraft || creationRefreshNeeded,
      canSave: close.canSave && !creationHostBusy && !foreignCreationDraft && !creationRefreshNeeded, saveReason: reason };
  });
  const creationPending = $derived(Boolean(creationClose &&
    (creationClose.unsaved || creationClose.busy || creationClose.blocked)));
  const deletion = deletionEnabled ? ownedDeletionSession(owner) : null;
  const restoration = restorationEnabled ? ownedRestorationSession(owner) : null;
  const creationUndo = creationUndoEnabled ? ownedCreationUndoSession(owner) : null;
  let deletionView = $state.raw(deletion?.view());
  let restorationView = $state.raw(restoration?.view());
  let creationUndoView = $state.raw(creationUndo?.view());
  $effect(() => deletion?.subscribe(() => untrack(() => { deletionView = deletion.view(); })));
  $effect(() => restoration?.subscribe(() => untrack(() => { restorationView = restoration.view(); })));
  $effect(() => creationUndo?.subscribe(() => untrack(() => { creationUndoView = creationUndo.view(); })));
  let lifecycleHostBusy = $state<Lifecycle | null>(null);
  let lifecycleRefreshedRevision = $state<Record<Lifecycle, number>>({ deletion: -1, restoration: -1, creationUndo: -1 });
  const lifecycleListRefreshNeeded: Record<Lifecycle, boolean> = { deletion: false, restoration: false, creationUndo: false };
  const foreignDeletionReview = $derived(Boolean(deletionView?.original &&
    !creationForms.includes(deletionView.original.form)));
  const lifecycleStates = $derived({
    deletion: lifecycleState('deletion'), restoration: lifecycleState('restoration'),
    creationUndo: lifecycleState('creationUndo'),
  });
  const lifecyclePending = $derived(lifecycleKinds.some(kind => pendingLifecycle(kind)));
  const deletionTargets = $derived((deletionView?.targets ?? []).filter(target => creationForms.includes(target.form)));
  const restorationTargets = $derived((restorationView?.history?.events ?? []).map(event => ({
    historyId: event.historyId, restored: event.restored,
    label: `${event.species}; ${event.form}; application ID ${event.id}; physical row ${event.rowId}; history ${event.historyId}`,
  })));
  const childClose = $derived.by(() => { childView; return child.closeState(); });
  const collectedClose = $derived.by(() => { collectedView; return collected.closeState(); });
  const speciesClose = $derived.by(() => { speciesView; return species.closeState(); });
  const identityClose = $derived.by(() => { identityView; return identity.closeState(); });
  const parentReady = $derived(parent?.canOpenLinkedVegetation() ?? false);
  const combinedPending = $derived(linkedEnabled && Boolean(childClose.unsaved || childClose.busy || childClose.blocked));
  const collectedPending = $derived(collectedEnabled && Boolean(collectedClose.unsaved || collectedClose.busy || collectedClose.blocked));
  const speciesPending = $derived(speciesEnabled && Boolean(speciesClose.unsaved || speciesClose.busy || speciesClose.blocked));
  const identityPending = $derived(identityEnabled && Boolean(identityClose.unsaved || identityClose.busy || identityClose.blocked));
  const creationRecoveryDisabled = $derived(creationRecoveryBlocked());
  const creationEditingDisabled = $derived(creationRecoveryDisabled || !parentReady || creationRefreshNeeded);
  const linkedParentReady = $derived(parentReady && !metadataOpen && !creationPending && !lifecyclePending);
  const peerBlockedReason = $derived(metadataOpen
    ? 'Finish the owned project metadata review before changing the CHARS parent or linked children.'
    : lifecyclePending
    ? 'Finish the explicit deletion, restoration or historical creation Undo review/recovery before changing the CHARS parent.'
    : creationPending
    ? 'Finish, correct, Undo or resolve the retained creation request and refresh before changing the CHARS parent.'
    : combinedPending || collectedPending || speciesPending || identityPending
    ? 'Finish, correct or Undo linked vegetation drafts/recovery before changing the CHARS parent.' : '');
  $effect(() => { onBusyChange?.(metadataBusy || metadataOpen && !metadataEditor || parentBusy || linkedEnabled && childView.busy ||
    collectedEnabled && collectedView.busy || speciesEnabled && speciesView.busy || identityEnabled && identityView.busy ||
    Boolean(creationClose?.busy) || lifecycleKinds.some(kind => lifecycleStates[kind]?.busy)); });
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
      : target === 'species' ? { enabled: speciesEnabled, session: species }
      : { enabled: identityEnabled, session: identity };
  }
  function requireCleanPeer(target: Child) {
    const peer = entry(target);
    if (!peer.enabled) return;
    const close = peer.session.closeState();
    if (close.unsaved || close.busy || close.blocked) {
      throw new Error(`Linked ${target} drafts/recovery must finish before a peer operation.`);
    }
  }
  function requireCleanPeers(except?: Child | 'creation' | Lifecycle | 'metadata', verifiedRefresh = false) {
    if (metadataOpen && except !== 'metadata') {
      throw new Error('Finish the owned project metadata review before a peer operation.');
    }
    for (const target of targets) if (target !== except) requireCleanPeer(target);
    if (except !== 'creation' && creationPending && !(verifiedRefresh && verifiedReceiptOnly('creation'))) {
      throw new Error('Finish creation drafts, unknown acknowledgement or verified host refresh before a peer operation.');
    }
    for (const kind of lifecycleKinds) if (kind !== except && pendingLifecycle(kind) &&
      !(verifiedRefresh && verifiedReceiptOnly(kind))) {
      throw new Error(`Finish the explicit ${kind} review, unknown request or verified host refresh before a peer operation.`);
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
    requireCleanPeers(target);
    if (!parent) throw new Error('CHARS linked parent ownership ended before refresh.');
    await parent.refreshAfterLinkedSave();
    for (const peer of targets) if (peer !== target) await refreshCleanPeer(peer);
    await refreshLifecycleLists();
    await onSaved?.(owner.plot);
    requireLive();
  }
  async function refreshAfterParentSave() {
    requireLive();
    requireCleanPeers();
    for (const target of targets) await refreshCleanPeer(target);
    await refreshLifecycleLists();
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
      else if (target === 'species') speciesOpened = true;
      else identityOpened = true;
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
  function stageIdentity(rowId: string, raw: string, nullValue: boolean) {
    operationError = '';
    try {
      requireLive();
      requireCleanPeers('identity');
      if (!identityEnabled || !parent?.canOpenLinkedVegetation()) {
        throw new Error('Unlock the clean available CHARS parent before editing linked IDs.');
      }
      identity.stage(rowId, 'ID', raw, nullValue);
    } catch (cause) { operationError = `CHARS linked identity draft retained: ${String(cause)}`; }
  }
  function cancelRead(target: Child = 'combined') {
    if (childOperation?.kind !== 'load' || childOperation.target !== target || !entry(target).session.view().busy) {
      operationError = 'Only a linked source read can be cancelled; mutations must finish.';
      return;
    }
    reads[target].cancelAll();
  }
  async function refreshAfterCreation() {
    await refreshVerifiedSources('creation');
  }
  async function creationOperation(kind: 'load' | 'start' | 'save' | 'undo' | 'resolve' | 'cancel' | 'refresh',
    form?: SIVICreationForm): Promise<boolean> {
    operationError = '';
    try {
      requireLive();
      if (!creation) throw new Error('CHARS creation requires its independent entry gates.');
      if (kind === 'cancel') { creation.cancel(); return true; }
      const verifiedRefresh = kind === 'refresh' || kind === 'resolve' || kind === 'undo';
      requireCleanPeers('creation', verifiedRefresh);
      if (creationRecoveryBlocked(verifiedRefresh) || (kind === 'load' || kind === 'start' || kind === 'save') && creationEditingDisabled) {
        throw new Error('Unlock the clean owned parent and finish conflicting operations before CHARS creation.');
      }
      if (kind === 'refresh') await refreshAfterCreation();
      else if (kind === 'undo') creation.discard();
      else if (kind === 'start') {
        if (!form || !creationForms.includes(form)) throw new Error('Choose an actual CHARS extended A, C or D source form.');
        creation.start(form);
      } else if (kind === 'load') return await creation.loadReferences();
      else {
        if (kind === 'save' && (foreignCreationDraft || !creationClose?.canSave)) {
          throw new Error(creationClose?.saveReason || 'Correct the retained CHARS creation draft.');
        }
        rememberLifecycleLists();
        const ok = kind === 'save' ? await creation.save(crypto.randomUUID()) : await creation.resolve();
        if (!ok) return false;
        await refreshAfterCreation();
      }
      return true;
    } catch (cause) {
      if (!disposed) operationError = creationRefreshNeeded
        ? `CHARS creation is already verified; read-only host refresh remains pending. Never repeat Create: ${String(cause)}`
        : `CHARS creation operation failed; retained owner unchanged: ${String(cause)}`;
      return false;
    }
  }
  function stageCreation(draft: SIVICreationDraft) {
    operationError = '';
    try {
      requireLive();
      requireCleanPeers('creation');
      if (!creation || creationEditingDisabled || foreignCreationDraft || !creationForms.includes(draft.form)) {
        throw new Error('Correct creation only in its available, unlocked source-form owner.');
      }
      creation.updateDraft(draft);
    } catch (cause) { operationError = `CHARS creation draft retained: ${String(cause)}`; }
  }
  function lifecycleEntry(kind: Lifecycle) {
    return kind === 'deletion' ? deletion : kind === 'restoration' ? restoration : creationUndo;
  }
  function verifiedReceiptOnly(kind: 'creation' | Lifecycle) {
    const session = kind === 'creation' ? creation : lifecycleEntry(kind);
    const view = kind === 'creation' ? creationView : kind === 'deletion' ? deletionView
      : kind === 'restoration' ? restorationView : creationUndoView;
    if (!session || !view?.receipt) return false;
    const close = session.closeState();
    return !close.unsaved && !close.busy && !close.blocked;
  }
  function creationRecoveryBlocked(verifiedRefresh = false) {
    return metadataOpen || !creationEnabled || parentBusy || Boolean(childOperation) || creationHostBusy || lifecycleHostBusy !== null ||
      combinedPending || collectedPending || speciesPending || identityPending ||
      lifecycleKinds.some(kind => pendingLifecycle(kind) && !(verifiedRefresh && verifiedReceiptOnly(kind))) ||
      !parent?.canReloadLinkedParent();
  }
  function lifecycleRefreshNeeded(kind: Lifecycle) {
    const view = kind === 'deletion' ? deletionView : kind === 'restoration' ? restorationView : creationUndoView;
    return Boolean(view?.receipt && view.revision !== lifecycleRefreshedRevision[kind]);
  }
  function lifecycleState(kind: Lifecycle): EditorCloseState | null {
    const session = lifecycleEntry(kind);
    if (!session) return null;
    if (kind === 'deletion') deletionView;
    else if (kind === 'restoration') restorationView;
    else creationUndoView;
    const close = session.closeState();
    const refresh = lifecycleRefreshNeeded(kind);
    const foreign = kind === 'deletion' && foreignDeletionReview;
    const reason = lifecycleHostBusy === kind ? `Wait for verified ${kind} parent and peer refresh.`
      : foreign ? 'Return to the retained source-form editor or explicitly discard its deletion review.'
      : refresh ? `${kind} is verified. Refresh parent and peers without repeating the mutation.` : close.saveReason;
    return { ...close, unsaved: close.unsaved || refresh, busy: close.busy || lifecycleHostBusy === kind,
      blocked: close.blocked || refresh || foreign,
      canSave: close.canSave && !refresh && !foreign && lifecycleHostBusy !== kind, saveReason: reason };
  }
  function pendingLifecycle(kind: Lifecycle) {
    const close = lifecycleStates[kind];
    return Boolean(close && (close.unsaved || close.busy || close.blocked));
  }
  function lifecycleRecoveryDisabled(kind: Lifecycle, verifiedRefresh = false) {
    return metadataOpen || !lifecycleEntry(kind) || parentBusy || Boolean(childOperation) || creationHostBusy ||
      lifecycleHostBusy !== null || combinedPending || collectedPending || speciesPending || identityPending ||
      creationPending && !(verifiedRefresh && verifiedReceiptOnly('creation')) ||
      lifecycleKinds.some(peer => peer !== kind && pendingLifecycle(peer) &&
        !(verifiedRefresh && verifiedReceiptOnly(peer))) || !parent?.canReloadLinkedParent();
  }
  function lifecycleEditingDisabled(kind: Lifecycle) {
    return lifecycleRecoveryDisabled(kind) || !parentReady || lifecycleRefreshNeeded(kind) ||
      kind === 'deletion' && foreignDeletionReview;
  }
  function rememberLifecycleLists() {
    if (deletionView?.targets !== null && deletionView?.targets !== undefined) lifecycleListRefreshNeeded.deletion = true;
    if (restorationView?.history) lifecycleListRefreshNeeded.restoration = true;
    if (creationUndoView?.history) lifecycleListRefreshNeeded.creationUndo = true;
  }
  async function refreshLifecycleLists() {
    rememberLifecycleLists();
    for (const kind of lifecycleKinds) {
      const session = lifecycleEntry(kind);
      if (!session || !lifecycleListRefreshNeeded[kind]) continue;
      const close = session.closeState();
      if (close.unsaved || close.busy || close.blocked) throw new Error(`Retain the pending ${kind} owner before refreshing its list.`);
      let ok: boolean;
      if (kind === 'deletion' && deletion) ok = await deletion.loadTargets();
      else if (kind === 'restoration' && restoration) ok = await restoration.loadHistory();
      else if (kind === 'creationUndo' && creationUndo) ok = await creationUndo.loadHistory();
      else throw new Error(`The loaded ${kind} list lost its owned session.`);
      requireLive();
      if (!ok) throw new Error(session.view().error || `${kind} list refresh failed; the committed mutation must not repeat.`);
      lifecycleListRefreshNeeded[kind] = false;
    }
  }
  async function refreshAfterLifecycle(kind: Lifecycle) {
    await refreshVerifiedSources(kind);
  }
  async function refreshVerifiedSources(kind: 'creation' | Lifecycle) {
    requireLive();
    requireCleanPeers(kind, true);
    if (!verifiedReceiptOnly(kind) || !parent?.canReloadLinkedParent()) {
      throw new Error(`Retain the verified ${kind} receipt and unlock the clean parent before read-only refresh.`);
    }
    const revisions = { creation: creation?.view().revision, deletion: deletionView?.revision,
      restoration: restorationView?.revision, creationUndo: creationUndoView?.revision };
    if (kind === 'creation') creationHostBusy = true;
    else lifecycleHostBusy = kind;
    try {
      await parent.refreshAfterLinkedSave();
      for (const target of targets) await refreshCleanPeer(target);
      await refreshLifecycleLists();
      await onSaved?.(owner.plot);
      requireLive();
      if (creationView && verifiedReceiptOnly('creation') && creationView.revision === revisions.creation) {
        creationRefreshedRevision = creationView.revision;
      }
      for (const peer of lifecycleKinds) {
        const view = peer === 'deletion' ? deletionView : peer === 'restoration' ? restorationView : creationUndoView;
        if (view && verifiedReceiptOnly(peer) && view.revision === revisions[peer]) {
          lifecycleRefreshedRevision[peer] = view.revision;
        }
      }
    } finally {
      if (!disposed) {
        if (kind === 'creation') creationHostBusy = false;
        else lifecycleHostBusy = null;
      }
    }
  }
  async function lifecycleOperation(kind: Lifecycle,
    operation: 'load' | 'history' | 'review' | 'save' | 'undo' | 'discard' | 'resolve' | 'cancel' | 'refresh',
    identity?: string, form?: SIVICreationForm): Promise<boolean> {
    operationError = '';
    try {
      requireLive();
      const session = lifecycleEntry(kind);
      if (!session) throw new Error(`CHARS ${kind} requires its independent entry gates.`);
      if (operation === 'cancel') { session.cancel(); return true; }
      const discard = operation === 'discard' && kind === 'creationUndo' || operation === 'undo' && kind !== 'creationUndo';
      const verifiedRefresh = operation === 'resolve' || operation === 'refresh' || discard;
      requireCleanPeers(kind, verifiedRefresh);
      if (lifecycleRecoveryDisabled(kind, verifiedRefresh) ||
        !discard && operation !== 'resolve' && operation !== 'refresh' && lifecycleEditingDisabled(kind)) {
        throw new Error(`Unlock the clean owned parent and finish conflicting operations before ${kind}.`);
      }
      if (discard) session.discard();
      else if (operation === 'refresh') await refreshAfterLifecycle(kind);
      else if (operation === 'load' && kind === 'deletion' && deletion) return await deletion.loadTargets();
      else if (operation === 'history' && kind === 'restoration' && restoration) return await restoration.loadHistory();
      else if (operation === 'history' && kind === 'creationUndo' && creationUndo) return await creationUndo.loadHistory();
      else if (operation === 'review') {
        if (!identity) throw new Error('Choose one explicit loaded source row or historical event.');
        if (kind === 'deletion' && deletion) {
          if (!form || !creationForms.includes(form)) throw new Error('Choose an actual CHARS extended A, C or D deletion source.');
          return await deletion.review(form, identity);
        }
        if (kind === 'restoration' && restoration) return await restoration.review(identity);
        if (kind === 'creationUndo' && creationUndo) return await creationUndo.review(identity);
        throw new Error('The owned lifecycle review is unavailable; no source form was inferred.');
      } else if (operation === 'resolve' || operation === 'save' && kind !== 'creationUndo' ||
        operation === 'undo' && kind === 'creationUndo') {
        if (operation !== 'resolve' && !lifecycleStates[kind]?.canSave) {
          throw new Error(lifecycleStates[kind]?.saveReason || 'Confirm the exact retained lifecycle review.');
        }
        rememberLifecycleLists();
        let ok: boolean;
        if (operation === 'resolve') ok = await session.resolve();
        else if (kind === 'creationUndo' && creationUndo) ok = await creationUndo.undo(crypto.randomUUID());
        else if (kind === 'deletion' && deletion) ok = await deletion.save(crypto.randomUUID());
        else if (kind === 'restoration' && restoration) ok = await restoration.save(crypto.randomUUID());
        else throw new Error(`The ${kind} mutation lost its owned session.`);
        if (!ok) return false;
        await refreshAfterLifecycle(kind);
      } else throw new Error(`Unsupported ${kind} operation; no mutation was inferred.`);
      return true;
    } catch (cause) {
      if (!disposed) operationError = lifecycleRefreshNeeded(kind)
        ? `CHARS ${kind} is already verified; read-only host refresh remains pending. Never repeat the mutation: ${String(cause)}`
        : `CHARS ${kind} operation failed; retained owner unchanged: ${String(cause)}`;
      return false;
    }
  }
  function stageLifecycle(kind: Lifecycle, confirmed: boolean, action?: 'retain' | 'prune') {
    operationError = '';
    try {
      requireLive();
      requireCleanPeers(kind);
      const session = lifecycleEntry(kind);
      if (!session || lifecycleEditingDisabled(kind)) throw new Error(`Correct ${kind} only in its available unlocked owner.`);
      if (action) {
        if (kind === 'restoration' && restoration) restoration.choose(action);
        else if (kind === 'creationUndo' && creationUndo) creationUndo.choose(action);
        else throw new Error('Deletion has no inferred restoration audit action.');
      } else session.confirm(confirmed);
    } catch (cause) { operationError = `CHARS ${kind} confirmation retained: ${String(cause)}`; }
  }
  export function getCloseState(): EditorCloseState {
    childView;
    collectedView;
    speciesView;
    identityView;
    if (metadataOpen) {
      const close = metadataEditor?.getCloseState();
      return { unsaved: true, busy: metadataBusy || !close || close.busy,
        blocked: !close || Boolean(close.blocked), canSave: false,
        saveReason: 'Finish Save, Undo or recovery in the owned metadata editor, then close its review explicitly.',
        error: close?.error || operationError || null };
    }
    const own = parent?.getCloseState();
    const states = [own, creationClose, ...lifecycleKinds.map(kind => lifecycleStates[kind]),
      ...targets.map(target => entry(target).enabled ? entry(target).session.closeState() : null)]
      .filter((state): state is EditorCloseState => Boolean(state));
    const dirty = states.filter(state => state.unsaved);
    const conflict = dirty.length > 1;
    const busy = states.some(state => state.busy);
    const pending = dirty[0] ?? own;
    const saveReason = lifecyclePending
      ? 'Confirm deletion, restoration or historical creation Undo explicitly in its reviewed panel, or discard a known unsubmitted review. Save-and-close never performs it.'
      : conflict ? 'Multiple CHARS owners have drafts; Undo one explicitly before saving.'
      : busy ? 'Wait for the current CHARS parent or linked vegetation operation.'
      : pending?.saveReason ?? 'Load the CHARS parent before saving.';
    return { unsaved: dirty.length > 0, busy,
      blocked: conflict || states.some(state => state.blocked),
      canSave: !lifecyclePending && !conflict && !busy && !!pending?.canSave, saveReason,
      error: operationError || states.find(state => state.error)?.error || null };
  }
  export async function saveForClose(): Promise<boolean> {
    const close = getCloseState();
    if (!close.canSave || !parent) {
      operationError = close.saveReason || 'CHARS parent ownership is unavailable.';
      return false;
    }
    if (creationClose?.unsaved) return creationOperation('save');
    for (const target of targets) {
      const active = entry(target);
      if (active.enabled && active.session.closeState().unsaved) return operate('save', target);
    }
    return parent.saveForClose();
  }
  export function canOpenProjectMetadata() {
    if (!metadataEnabled || !metadataClient || disposed || metadataOpen || !parentReady ||
      parentBusy || childOperation) return false;
    const close = getCloseState();
    return !close.unsaved && !close.busy && !close.blocked;
  }
  export function openProjectMetadata() {
    if (!canOpenProjectMetadata()) {
      operationError = 'Project metadata requires its editing gate and a clean, unlocked available CHARS parent and peers.';
      return;
    }
    operationError = '';
    metadataOpen = true;
  }
  async function refreshAfterMetadataSave() {
    requireLive();
    if (!metadataOpen || !parent?.canReloadLinkedParent()) {
      throw new Error('Retain the owned metadata result until its clean unlocked CHARS parent can refresh; never replay Save.');
    }
    requireCleanPeers('metadata');
    await parent.refreshAfterLinkedSave();
    for (const target of targets) await refreshCleanPeer(target);
    await refreshLifecycleLists();
    requireLive();
    requireCleanPeers('metadata');
    await onSaved?.(owner.plot);
    requireLive();
  }
  function closeProjectMetadata() {
    if (!metadataOpen || metadataBusy || metadataEditor?.getCloseState().busy) {
      operationError = 'Wait for the owned metadata operation before closing its review.';
      return;
    }
    metadataOpen = false;
    operationError = '';
  }
</script>

{#snippet linkedVegetation()}
  {#if vegetationEnabled}
    <section class="min-w-0 space-y-3" aria-label="CHARS linked vegetation" data-chars-linked-vegetation>
      <h4 class="font-semibold">Linked vegetation: Tree/Shrubs, Herb, Moss/Lichen</h4>
      {#if operationError}<p role="alert" class="text-red-800">{operationError}</p>{/if}
      {#if linkedEnabled}
      {#if !opened}
        <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-linked-open
          disabled={!linkedParentReady || Boolean(childOperation) || collectedPending || speciesPending || identityPending}
          onclick={() => void operate('load')}>Open linked covers &amp; heights</button>
      {:else}
        {#if childOperation?.target === 'combined' && childOperation.kind === 'load' && childView.busy}
          <button type="button" class="rounded border px-3 py-2" data-chars-linked-cancel onclick={() => cancelRead()}>Cancel linked source read</button>
        {/if}
        <SIVICombinedPanel view={childView}
          disabled={!linkedParentReady || collectedPending || speciesPending || identityPending} canSave={childClose.canSave}
          onstage={stage} onsave={() => void operate('save')} onundo={() => void operate('undo')}
          onreload={() => void operate('load')}
          onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
      {/if}
      {/if}
      {#if collectedEnabled}
        {#if !collectedOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-collected-open
            disabled={!linkedParentReady || Boolean(childOperation) || combinedPending || speciesPending || identityPending}
            onclick={() => void operate('load', 'collected')}>Open linked Collected cycles</button>
        {:else}
          {#if childOperation?.target === 'collected' && childOperation.kind === 'load' && collectedView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-collected-cancel
              onclick={() => cancelRead('collected')}>Cancel linked Collected source read</button>
          {/if}
          <SIVICollectedPanel view={collectedView}
            disabled={!linkedParentReady || combinedPending || speciesPending || identityPending}
            canSave={collectedClose.canSave} oncycle={cycleCollected}
            onsave={() => void operate('save', 'collected')} onundo={() => void operate('undo', 'collected')}
            onreload={() => void operate('load', 'collected')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'collected')} />
        {/if}
      {/if}
      {#if speciesEnabled}
        {#if !speciesOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-species-open
            disabled={!linkedParentReady || Boolean(childOperation) || combinedPending || collectedPending || identityPending}
            onclick={() => void operate('load', 'species')}>Open linked Species selections</button>
        {:else}
          {#if childOperation?.target === 'species' && childOperation.kind === 'load' && speciesView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-species-cancel
              onclick={() => cancelRead('species')}>Cancel linked Species source/reference read</button>
          {/if}
          <SIVISpeciesPanel view={speciesView} disabled={!linkedParentReady || combinedPending || collectedPending || identityPending}
            canSave={speciesClose.canSave} onstage={stageSpecies} oncontext={contextSpecies} onchoose={chooseSpecies}
            onsave={() => void operate('save', 'species')} onundo={() => void operate('undo', 'species')}
            onreload={() => void operate('load', 'species')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'species')} />
        {/if}
      {/if}
      {#if identityEnabled}
        {#if !identityOpened}
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-identity-open
            disabled={!linkedParentReady || Boolean(childOperation) || combinedPending || collectedPending || speciesPending}
            onclick={() => void operate('load', 'identity')}>Open linked vegetation IDs</button>
        {:else}
          {#if childOperation?.target === 'identity' && childOperation.kind === 'load' && identityView.busy}
            <button type="button" class="rounded border px-3 py-2" data-chars-identity-cancel
              onclick={() => cancelRead('identity')}>Cancel linked ID source read</button>
          {/if}
          <SIVIIdentityPanel view={identityView} disabled={!linkedParentReady || combinedPending || collectedPending || speciesPending}
            canSave={identityClose.canSave} onstage={stageIdentity}
            onsave={() => void operate('save', 'identity')} onundo={() => void operate('undo', 'identity')}
            onreload={() => void operate('load', 'identity')}
            onrestore={action => void operate(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune', 'identity')} />
        {/if}
      {/if}
      {#if creationEnabled && creationView && creationClose}
        {#if creationRefreshNeeded}
          <p role="alert" class="break-words text-sm">Creation is verified. Refresh the owned parent and peers without repeating Create.</p>
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-creation-refresh
            disabled={creationRecoveryBlocked(true) || creationView.busy}
            onclick={() => void creationOperation('refresh')}>Refresh verified creation (read-only)</button>
        {/if}
        <SIVICreationPanel view={creationView} close={creationClose} {owner} allowedForms={creationForms}
          disabled={creationEditingDisabled} recoveryDisabled={creationRecoveryBlocked(true)}
          onoperation={(kind, form) => void creationOperation(kind, form)} onchange={stageCreation} />
      {/if}
      {#each lifecycleKinds as kind}
        {#if lifecycleRefreshNeeded(kind)}
          <p role="alert" class="break-words text-sm">{kind} is verified. Refresh the owned parent and peers without repeating the mutation.</p>
          <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-lifecycle-refresh={kind}
            disabled={lifecycleRecoveryDisabled(kind, true) || lifecycleStates[kind]?.busy}
            onclick={() => void lifecycleOperation(kind, 'refresh')}>Refresh verified {kind} (read-only)</button>
        {/if}
      {/each}
      {#if deletionView && lifecycleStates.deletion}
        {#if foreignDeletionReview}
          <p role="alert" class="break-words text-sm" data-chars-deletion-foreign>The retained deletion review belongs to another source form.
            Return to its owning editor or explicitly discard it; CHARS will not reinterpret or submit it.</p>
        {/if}
        <SIVIDeletionPanel view={deletionView} close={lifecycleStates.deletion} targets={deletionTargets}
          disabled={lifecycleEditingDisabled('deletion')} recoveryDisabled={lifecycleRecoveryDisabled('deletion', true)}
          onreview={(form, rowId) => void lifecycleOperation('deletion', 'review', rowId, form)}
          onconfirm={confirmed => stageLifecycle('deletion', confirmed)}
          onoperation={operation => void lifecycleOperation('deletion', operation)} />
      {/if}
      {#if restorationView && lifecycleStates.restoration}
        <SIVIDeletionRestorationPanel view={restorationView} close={lifecycleStates.restoration} targets={restorationTargets}
          disabled={lifecycleEditingDisabled('restoration')} recoveryDisabled={lifecycleRecoveryDisabled('restoration', true)}
          onreview={historyId => void lifecycleOperation('restoration', 'review', historyId)}
          onchoose={action => stageLifecycle('restoration', false, action)}
          onconfirm={confirmed => stageLifecycle('restoration', confirmed)}
          onoperation={operation => void lifecycleOperation('restoration', operation)} />
      {/if}
      {#if creationUndoView && lifecycleStates.creationUndo}
        <SIVICreationUndoPanel view={creationUndoView} close={lifecycleStates.creationUndo}
          disabled={lifecycleEditingDisabled('creationUndo')} recoveryDisabled={lifecycleRecoveryDisabled('creationUndo', true)}
          onreview={historyId => void lifecycleOperation('creationUndo', 'review', historyId)}
          onchoose={action => stageLifecycle('creationUndo', false, action)}
          onconfirm={confirmed => stageLifecycle('creationUndo', confirmed)}
          onoperation={operation => void lifecycleOperation('creationUndo', operation)} />
      {/if}
      <p class="text-sm text-stone-600">Saved rows use the original PlotNumber links and extended B3/B4/B5 controls.
        Parent and independently gated covers/heights, Collected, Species and IDs keep separate drafts and typed histories.
        Same-row histories restore in reverse edit order; changed physical rows refuse restoration.
        ID edits have technical history without source field audits. Species selections require an eligible source group;
        personal-species creation, {creationEnabled ? '' : 'vegetation creation, '}{deletionEnabled ? '' : 'vegetation deletion, '}
        automatic totals and unqualified source callbacks remain unavailable.
        Deletion removes one complete physical row, including fields in other source views. Typed restoration and
        historical creation Undo are explicit desktop safety adaptations, not original Access undelete or destructive cleanup.
        Parent Save and Save-and-close never submit these reviewed lifecycle actions.
        Reviewed Save is a desktop adaptation, not original Click/NotInList/focus timing parity.</p>
    </section>
  {/if}
{/snippet}

<div hidden={metadataOpen}>
<SIVICHARSParentForm bind:this={parent} contextId={owner.contextId} project={owner.project} plotNumber={owner.plot}
  onSaved={refreshAfterParentSave} onBusyChange={value => { parentBusy = value; }}
  {peerBlockedReason} collectedAvailable={collectedEnabled} speciesAvailable={speciesEnabled} identityAvailable={identityEnabled}
  creationAvailable={creationEnabled} deletionAvailable={deletionEnabled} restorationAvailable={restorationEnabled}
  creationUndoAvailable={creationUndoEnabled}
  metadataAvailable={canOpenProjectMetadata()} onMetadata={metadataEnabled ? openProjectMetadata : undefined}
  peerUnlockAvailable={(creationPending || lifecyclePending) && !creationClose?.busy &&
    !lifecycleKinds.some(kind => lifecycleStates[kind]?.busy)}
  linkedVegetation={vegetationEnabled ? linkedVegetation : undefined} />
</div>
{#if metadataOpen && metadataClient}
  <ProjectMetadataEditor bind:this={metadataEditor} client={metadataClient} contextId={owner.contextId} plot={owner.plot}
    allowRestoration={metadataRestorationEnabled} onclosed={closeProjectMetadata}
    onbusy={value => { metadataBusy = value; }} oncommitted={refreshAfterMetadataSave} />
{/if}
{#if operationError && (metadataOpen || !vegetationEnabled)}<p role="alert" class="text-red-800">{operationError}</p>{/if}
