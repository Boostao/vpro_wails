<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { AuditRestoreAction, TwoPageParentEntryService, type SIVIParentSharedReference } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import type { EditorCloseState } from './closeLifecycle';
  import type { TwoPageParentForm } from './twoPageParentSession';
  import { twoPageParentCommonFields } from './twoPageParentCommonSession';
  import type { TwoPageEntryInput } from './twoPageEntryDraft';
  import { TwoPageEntrySession, twoPageEntryCell, type TwoPageEntrySessionCache } from './twoPageEntrySession';
  import TwoPageEntryLayout from './TwoPageEntryLayout.svelte';
  import { twoPageEntryProjection, twoPageEntryValues, type TwoPageEntryChild, type TwoPageEntryField } from './twoPageEntryProjection';
  import { metadataCellText } from './projectMetadataEditor';
  import { controlLabel, wideControl } from './formPresentation';
  import type { PaperControl } from './paperLayout';
  import type { GoogleEarthScope } from './googleEarthReview';
  import { externalSUEnvironmentSession } from './externalSUEnvironment';
  import ExternalSUEnvironmentTransfer from './ExternalSUEnvironmentTransfer.svelte';
  import TwoPageAuditPanel from './TwoPageAuditPanel.svelte';
  import TwoPageGoogleEarthPanel from './TwoPageGoogleEarthPanel.svelte';
  import TwoPageVegetationTablePanel from './TwoPageVegetationTablePanel.svelte';
  import TwoPageEnvironmentTablePanel from './TwoPageEnvironmentTablePanel.svelte';
  import TwoPageProjectMetadataPanel from './TwoPageProjectMetadataPanel.svelte';
  import TwoPageProfilingPanel from './TwoPageProfilingPanel.svelte';

  let { contextId, project, plot, cache, revision, onchange, onbusy, onclosed, oncommitted, embedded: childContent, childBlocked = false, pictureReaderAvailable = false, auditReaderAvailable = false, googleEarthReaderAvailable = false, vegetationTableReaderAvailable = false, environmentTableReaderAvailable = false, projectMetadataReaderAvailable = false, vegetationTableSU = '', vegetationTableSUPath = '', sourceProjectPath = '',
    profilingReaderAvailable = false, profilingSource = null, externalScope = null, sourceBlocked = true, initialForm = 'FS882-8x6XL' }: {
    contextId: string; project: string; plot: string; cache: TwoPageEntrySessionCache; revision: number;
    onchange: () => void; onbusy: (busy: boolean) => void; onclosed: () => void; oncommitted: () => Promise<void>;
    embedded: Snippet<[TwoPageEntryChild, PaperControl]>;
    childBlocked?: boolean; pictureReaderAvailable?: boolean; auditReaderAvailable?: boolean;
    googleEarthReaderAvailable?: boolean; sourceProjectPath?: string;
    vegetationTableReaderAvailable?: boolean; environmentTableReaderAvailable?: boolean; projectMetadataReaderAvailable?: boolean; vegetationTableSU?: string; vegetationTableSUPath?: string;
    profilingReaderAvailable?: boolean; profilingSource?: { name: string; path: string } | null;
    externalScope?: GoogleEarthScope | null; sourceBlocked?: boolean; initialForm?: TwoPageParentForm;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot }));
  const reviewEnabled = import.meta.env.VITE_TWO_PAGE_PARENT_REVIEW === 'true'
    && import.meta.env.VITE_TWO_PAGE_PARENT_ENTRY_REVIEW === 'true';
  const editingEnabled = reviewEnabled && import.meta.env.VITE_TWO_PAGE_PARENT_ENTRY_EDITING === 'true';
  const sessions = untrack(() => {
    const key = JSON.stringify(owner);
    const existing = cache.get(key) ?? new Map<TwoPageParentForm, TwoPageEntrySession>();
    cache.set(key, existing); return existing;
  });
  const sourceScope = untrack(() => externalScope);
  const externalSession = sourceScope ? externalSUEnvironmentSession(sourceScope) : null;
  let externalOpen = $state(untrack(() => Boolean(externalSession?.view().blocked)));
  let auditOpen = $state(false);
  let auditBusy = $state(false);
  let googleEarthOpen = $state(false);
  let googleEarthBusy = $state(false);
  const googleEarthPath = untrack(() => sourceProjectPath);
  let vegetationTableOpen = $state(false);
  let vegetationTableBusy = $state(false);
  let environmentTableOpen = $state(false);
  let environmentTableBusy = $state(false);
  let projectMetadataOpen = $state(false);
  let projectMetadataBusy = $state(false);
  let profilingOpen = $state(false);
  let profilingBusy = $state(false);
  const profileSource = untrack(() => profilingSource ? { ...profilingSource } : null);
  const vegetationScope = untrack(() => ({ projectPath: sourceProjectPath, su: vegetationTableSU, suPath: vegetationTableSUPath }));
  let sourceRefreshing = $state(false);
  let externalEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  let form = $state<TwoPageParentForm>(untrack(() => initialForm));
  let session = $state(makeSession(untrack(() => initialForm)));
  let error = $state<string | null>(null);
  const view = $derived.by(() => { revision; return session.view(); });
  const close = $derived.by(() => { revision; return session.closeState(); });
  const projection = $derived(twoPageEntryProjection(form));

  function makeSession(sourceForm: TwoPageParentForm) {
    const existing = sessions.get(sourceForm);
    if (existing) return existing;
    const requests = new ReadRequests();
    const created = new TwoPageEntrySession(owner, sourceForm, {
      read: () => requests.track(TwoPageParentEntryService.GetOriginal(owner.contextId, owner.plot, sourceForm)),
      readReferences: filters => requests.track(TwoPageParentEntryService.GetReferences(owner.contextId, owner.plot, sourceForm, filters)),
      cancelRead: () => requests.cancelAll(),
      save: request => TwoPageParentEntryService.Save(owner.contextId, owner.plot, sourceForm, request),
      restore: (history, action) => TwoPageParentEntryService.Restore(owner.contextId, owner.plot, sourceForm, history, action),
      refreshParent: async () => {
        await oncommitted();
        for (const [variant, peer] of sessions) {
          if (variant === sourceForm || !peer.view().original) continue;
          if (peer.closeState().unsaved || peer.closeState().busy || !await peer.load()) {
            throw new Error(peer.view().error ?? 'Another complete-entry variant could not refresh; no write was replayed.');
          }
        }
      },
    }, onchange);
    sessions.set(sourceForm, created); return created;
  }
  $effect(() => { onbusy(view.busy || view.referencesBusy || externalOpen || sourceRefreshing || auditOpen || auditBusy || googleEarthOpen || googleEarthBusy || vegetationTableOpen || vegetationTableBusy || environmentTableOpen || environmentTableBusy || projectMetadataOpen || projectMetadataBusy || profilingOpen || profilingBusy); });
  onDestroy(() => {
    const active = session.view();
    if (active.operation === 'read' || active.referencesBusy) session.cancel();
  });
  export function getCloseState(): EditorCloseState {
    return { ...close, unsaved: true, busy: close.busy || projectMetadataBusy || profilingBusy, canSave: false, error: error ?? close.error,
      saveReason: 'Save or Undo complete-entry drafts explicitly, then close their review before saving or closing the plot.' };
  }
  export function getDraftState(): EditorCloseState {
    const states = [...sessions.values()].map(owned => owned.closeState());
    return { ...close, unsaved: externalOpen || auditOpen || googleEarthOpen || vegetationTableOpen || environmentTableOpen || projectMetadataOpen || profilingOpen || states.some(state => state.unsaved), busy: sourceRefreshing || auditBusy || googleEarthBusy || vegetationTableBusy || environmentTableBusy || projectMetadataBusy || profilingBusy || states.some(state => state.busy),
      error: error ?? close.error, blocked: externalOpen || auditOpen || googleEarthOpen || vegetationTableOpen || environmentTableOpen || projectMetadataOpen || profilingOpen || !view.original || !!view.referenceError || states.some(state => state.blocked) };
  }
  function openProfiling() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!profilingReaderAvailable || !profileSource?.name || profileSource.name === 'None' || !profileSource.path ||
        !vegetationScope.projectPath || !vegetationScope.su || vegetationScope.su !== 'None' && !vegetationScope.suPath ||
        sourceBlocked || childBlocked || draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve source-variant and linked-child drafts, references and retained outcomes; select an explicit owned profile before Plot Profiling.';
      return;
    }
    error = null; profilingOpen = true; onchange();
  }
  function returnFromProfiling() {
    if (profilingBusy) { error = 'Wait for or cancel the owned profile read before returning to the source form.'; return; }
    profilingOpen = false; error = null; onchange();
  }
  function openProjectMetadata() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!projectMetadataReaderAvailable || !vegetationScope.projectPath || !vegetationScope.su ||
        vegetationScope.su !== 'None' && !vegetationScope.suPath || sourceBlocked || childBlocked ||
        draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before project metadata review.';
      return;
    }
    error = null; projectMetadataOpen = true; onchange();
  }
  function returnFromProjectMetadata() {
    if (projectMetadataBusy) { error = 'Wait for or cancel the owned project metadata read before returning to the source form.'; return; }
    projectMetadataOpen = false; error = null; onchange();
  }
  function openEnvironmentTable() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!environmentTableReaderAvailable || !vegetationScope.projectPath || !vegetationScope.su ||
        vegetationScope.su !== 'None' && !vegetationScope.suPath || sourceBlocked || childBlocked ||
        draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before View Env Table.';
      return;
    }
    error = null; environmentTableOpen = true; onchange();
  }
  function returnFromEnvironmentTable() {
    if (environmentTableBusy) { error = 'Wait for or cancel the owned environment table read before returning to the source form.'; return; }
    environmentTableOpen = false; error = null; onchange();
  }
  function openVegetationTable() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!vegetationTableReaderAvailable || !vegetationScope.projectPath || !vegetationScope.su ||
        vegetationScope.su !== 'None' && !vegetationScope.suPath || sourceBlocked || childBlocked ||
        draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before View Veg Table.';
      return;
    }
    error = null; vegetationTableOpen = true; onchange();
  }
  function returnFromVegetationTable() {
    if (vegetationTableBusy) { error = 'Wait for or cancel the owned vegetation table read before returning to the source form.'; return; }
    vegetationTableOpen = false; error = null; onchange();
  }
  function openGoogleEarth() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!googleEarthReaderAvailable || !googleEarthPath || sourceBlocked || childBlocked ||
        draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before saved-plot KML.';
      return;
    }
    error = null; googleEarthOpen = true; onchange();
  }
  function returnFromGoogleEarth() {
    if (googleEarthBusy) { error = 'Wait for or cancel the owned saved-plot read before returning to the source form.'; return; }
    googleEarthOpen = false; error = null; onchange();
  }
  function openAudit() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError));
    if (!auditReaderAvailable || sourceBlocked || childBlocked || draftState.unsaved || draftState.busy ||
        draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before Audit.';
      return;
    }
    error = null; auditOpen = true; onchange();
  }
  function returnFromAudit() {
    if (auditBusy) { error = 'Wait for or cancel the owned audit read before returning to the source form.'; return; }
    auditOpen = false; error = null; onchange();
  }
  function openExternalTransfer() {
    const draftState = getDraftState();
    const peerError = [...sessions.values()].find(peer => peer.view().error || peer.view().referenceError);
    if (!externalSession || !sourceScope?.projectPath || !sourceScope.suPath ||
        sourceScope.contextId !== owner.contextId || sourceScope.project !== owner.project || sourceBlocked || childBlocked ||
        draftState.unsaved || draftState.busy || draftState.blocked || draftState.error || peerError) {
      error = 'Resolve all source-variant and linked-child drafts, references and retained outcomes before SU Into Env.';
      return;
    }
    try {
      externalSession.reviewSourceForm(form);
      error = null; externalOpen = true; onchange();
    } catch (cause) { error = `Source reverse transfer unavailable; owned state retained: ${String(cause)}`; }
  }
  async function returnFromExternalTransfer() {
    if (sourceRefreshing || !externalEditor || externalEditor.getCloseState().unsaved || externalEditor.getCloseState().busy) {
      error = 'Finish the retained transfer before refreshing the source form; never replay.';
      return;
    }
    sourceRefreshing = true; error = null; onchange();
    try {
      await oncommitted();
      for (const peer of sessions.values()) {
        if (peer !== session && !peer.view().original) continue;
        const state = peer.closeState();
        if (state.unsaved || state.busy || state.blocked || !await peer.load()) {
          throw new Error(peer.view().error ?? peer.view().referenceError ?? 'A complete-entry source variant could not refresh.');
        }
      }
      externalOpen = false;
    } catch (cause) {
      error = `Source refresh failed; retain the immutable transfer receipt and retry Return, never replay: ${String(cause)}`;
    } finally { sourceRefreshing = false; onchange(); }
  }
  export function undo() { void operation('undo'); }
  async function chooseForm(next: TwoPageParentForm) {
    if (externalOpen || auditOpen || googleEarthOpen || vegetationTableOpen || environmentTableOpen || projectMetadataOpen || profilingOpen || childBlocked || close.unsaved || close.busy || close.blocked) {
      error = 'Save or Undo the owned complete-entry draft before changing source variant.'; return;
    }
    form = next; session = makeSession(next); onchange();
    await operation('load');
  }
  async function operation(action: 'load' | 'save' | 'undo' | 'retain' | 'prune' | 'references') {
    try {
      if (profilingOpen) throw new Error('Finish the owned profile review before parent operations.');
      if (projectMetadataOpen) throw new Error('Finish the owned project metadata review before parent operations.');
      if (environmentTableOpen) throw new Error('Finish the owned environment table review before parent operations.');
      if (vegetationTableOpen) throw new Error('Finish the owned vegetation table review before parent operations.');
      if (googleEarthOpen) throw new Error('Finish the owned saved-plot KML review before parent operations.');
      if (auditOpen) throw new Error('Finish the owned source audit review before parent operations.');
      if (externalOpen) throw new Error('Finish the retained source transfer and explicit refresh before parent operations.');
      if (!reviewEnabled || !editingEnabled && action !== 'load' && action !== 'undo' && action !== 'references') {
        throw new Error('Complete-entry review/editing is independently disabled.');
      }
      if (childBlocked && action !== 'undo' && action !== 'references') {
        throw new Error('Resolve retained linked-child drafts, reads and receipts before changing the parent.');
      }
      error = null;
      const ok = action === 'load' ? await session.load() : action === 'save' ? await session.save()
        : action === 'undo' ? await session.undo() : action === 'references' ? await session.loadReferences()
        : await session.restore(action === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) error = session.view().error ?? session.view().referenceError;
    } catch (cause) { error = `Complete-entry operation refused; owned state retained: ${String(cause)}`; }
  }
  function stage(column: string, input: TwoPageEntryInput) {
    try {
      if (profilingOpen) throw new Error('Plot Profiling owns the parent; no field was staged.');
      if (projectMetadataOpen) throw new Error('Project metadata owns the parent; no field was staged.');
      if (environmentTableOpen) throw new Error('Environment table owns the parent; no field was staged.');
      if (vegetationTableOpen) throw new Error('Vegetation table owns the parent; no field was staged.');
      if (googleEarthOpen) throw new Error('Saved-plot KML owns the parent; no field was staged.');
      if (auditOpen) throw new Error('Source audit owns the parent; no field was staged.');
      if (externalOpen) throw new Error('Source transfer owns the parent; no field was staged.');
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      if (childBlocked && !view.drafts[column]) throw new Error('Resolve linked-child ownership before new parent staging.');
      session.stage(column, input); error = session.view().error;
    } catch (cause) { error = `Complete-entry draft refused; owned state retained: ${String(cause)}`; }
  }
  function selectProject(rowId: string | null) {
    try {
      if (profilingOpen) throw new Error('Plot Profiling owns the parent; no ProjectID was staged.');
      if (projectMetadataOpen) throw new Error('Project metadata owns the parent; no ProjectID was staged.');
      if (environmentTableOpen) throw new Error('Environment table owns the parent; no ProjectID was staged.');
      if (vegetationTableOpen) throw new Error('Vegetation table owns the parent; no ProjectID was staged.');
      if (googleEarthOpen) throw new Error('Saved-plot KML owns the parent; no ProjectID was staged.');
      if (auditOpen) throw new Error('Source audit owns the parent; no ProjectID was staged.');
      if (externalOpen) throw new Error('Source transfer owns the parent; no ProjectID was staged.');
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      if (childBlocked && !view.drafts.ProjectID) throw new Error('Resolve linked-child ownership before new parent staging.');
      session.selectProject(rowId); error = session.view().error;
    } catch (cause) { error = `Physical ProjectID selection refused: ${String(cause)}`; }
  }
  function acknowledge(column: string) {
    try {
      if (profilingOpen) throw new Error('Plot Profiling owns the parent; no literal draft was acknowledged.');
      if (projectMetadataOpen) throw new Error('Project metadata owns the parent; no literal draft was acknowledged.');
      if (environmentTableOpen) throw new Error('Environment table owns the parent; no literal draft was acknowledged.');
      if (vegetationTableOpen) throw new Error('Vegetation table owns the parent; no literal draft was acknowledged.');
      if (googleEarthOpen) throw new Error('Saved-plot KML owns the parent; no literal draft was acknowledged.');
      if (auditOpen) throw new Error('Source audit owns the parent; no literal draft was acknowledged.');
      if (externalOpen) throw new Error('Source transfer owns the parent; no literal draft was acknowledged.');
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      session.acknowledge(column); error = session.view().error;
    } catch (cause) { error = `Literal acknowledgement refused: ${String(cause)}`; }
  }
  function cancel() {
    try { session.cancel(); error = session.view().error; }
    catch (cause) { error = `Complete-entry cancellation refused: ${String(cause)}`; }
  }
  function closeReview() {
    if (externalOpen || auditOpen || googleEarthOpen || vegetationTableOpen || environmentTableOpen || projectMetadataOpen || profilingOpen || childBlocked || close.unsaved || close.busy || close.blocked) {
      error = close.saveReason || 'Save or Undo complete-entry drafts before closing their review.'; return;
    }
    onclosed();
  }
  function referenceChoices(reference: SIVIParentSharedReference) {
    if (reference.choices === null) throw new Error(`${reference.column} reference lost its complete choice receipt.`);
    return reference.choices;
  }
</script>

<section class="space-y-4 rounded border bg-white p-4" aria-label="Complete two-page entry review" data-two-page-entry-panel={form}>
  <h3 class="font-semibold">{form}: one owned complete-entry draft</h3>
  {#if error}<p role="alert" class="text-sm text-red-700">{error}</p>{/if}
  {#if view.error && view.error !== error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.referenceError}<p role="alert" class="text-sm text-red-700">{view.referenceError}</p>{/if}
  {#if view.blocked}<p role="alert">Write outcome or refresh is unresolved. Undo / reload explicitly; never replay Save.</p>{/if}
  {#if view.busy || view.referencesBusy}<p role="status">Complete-entry {view.operation ?? 'reference read'} in progress...</p>{/if}
  {#if profilingOpen && profileSource}
    <p role="status">Source action {form} / {projection.profilingAction.controlId}; pinned opening plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageProfilingPanel contextId={owner.contextId} project={owner.project} plot={owner.plot} {form}
      projectPath={vegetationScope.projectPath} su={vegetationScope.su} suPath={vegetationScope.suPath}
      profileSource={profileSource} onBusyChange={value => profilingBusy = value} />
    <button type="button" disabled={profilingBusy} onclick={returnFromProfiling}>Return to complete-entry source</button>
  {:else if projectMetadataOpen}
    <p role="status">Source action {form} / {projection.metadataAction.controlId}; pinned opening plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageProjectMetadataPanel contextId={owner.contextId} project={owner.project} plot={owner.plot} {form}
      projectPath={vegetationScope.projectPath} su={vegetationScope.su} suPath={vegetationScope.suPath}
      onBusyChange={value => projectMetadataBusy = value} />
    <button type="button" disabled={projectMetadataBusy} onclick={returnFromProjectMetadata}>Return to complete-entry source</button>
  {:else if environmentTableOpen}
    <p role="status">Source action {form} / {projection.environmentTableAction.controlId}; pinned opening plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageEnvironmentTablePanel contextId={owner.contextId} project={owner.project} plot={owner.plot} {form}
      projectPath={vegetationScope.projectPath} su={vegetationScope.su} suPath={vegetationScope.suPath}
      onBusyChange={value => environmentTableBusy = value} />
    <button type="button" disabled={environmentTableBusy} onclick={returnFromEnvironmentTable}>Return to complete-entry source</button>
  {:else if vegetationTableOpen}
    <p role="status">Source action {form} / {projection.vegetationTableAction.controlId}; pinned opening plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageVegetationTablePanel contextId={owner.contextId} project={owner.project} plot={owner.plot} {form}
      projectPath={vegetationScope.projectPath} su={vegetationScope.su} suPath={vegetationScope.suPath}
      onBusyChange={value => vegetationTableBusy = value} />
    <button type="button" disabled={vegetationTableBusy} onclick={returnFromVegetationTable}>Return to complete-entry source</button>
  {:else if googleEarthOpen}
    <p role="status">Source action {form} / {projection.googleEarthAction.controlId}; pinned saved plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageGoogleEarthPanel contextId={owner.contextId} project={owner.project} projectPath={googleEarthPath} plot={owner.plot} {form}
      onBusyChange={value => googleEarthBusy = value} />
    <button type="button" disabled={googleEarthBusy} onclick={returnFromGoogleEarth}>Return to complete-entry source</button>
  {:else if auditOpen}
    <p role="status">Source action {form} / {projection.auditAction.controlId}; pinned literal plot {JSON.stringify(owner.plot)}.</p>
    <TwoPageAuditPanel contextId={owner.contextId} project={owner.project} plot={owner.plot} {form}
      onBusyChange={value => auditBusy = value} />
    <button type="button" disabled={auditBusy} onclick={returnFromAudit}>Return to complete-entry source</button>
  {:else if externalOpen && sourceScope?.projectPath && sourceScope.suPath}
    <p role="status">Source action {form} / {projection.reverseAction.controlId}: selected-SU bulk scope, not only {owner.plot}.</p>
    <ExternalSUEnvironmentTransfer bind:this={externalEditor} contextId={sourceScope.contextId}
      project={sourceScope.project} projectPath={sourceScope.projectPath} su={sourceScope.su} suPath={sourceScope.suPath}
      onclosed={() => { void returnFromExternalTransfer(); }} />
    {#if sourceRefreshing}<p role="status">Refreshing source variants; the committed transfer will not be replayed.</p>{/if}
  {:else}
  <div class="flex flex-wrap gap-2">
    {#if profilingReaderAvailable}
      <button type="button" data-two-page-profiling-open data-source-control={projection.profilingAction.controlId}
        disabled={!profileSource?.name || profileSource.name === 'None' || !profileSource.path ||
          !vegetationScope.projectPath || !vegetationScope.su || vegetationScope.su !== 'None' && !vegetationScope.suPath ||
          sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openProfiling}>{projection.profilingAction.caption}: read-only saved data</button>
    {/if}
    {#if projectMetadataReaderAvailable}
      <button type="button" data-two-page-project-metadata-open data-source-control={projection.metadataAction.controlId}
        disabled={!vegetationScope.projectPath || !vegetationScope.su || vegetationScope.su !== 'None' && !vegetationScope.suPath ||
          sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openProjectMetadata}>{projection.metadataAction.caption}: read-only saved data</button>
    {/if}
    {#if environmentTableReaderAvailable}
      <button type="button" data-two-page-environment-table-open data-source-control={projection.environmentTableAction.controlId}
        disabled={!vegetationScope.projectPath || !vegetationScope.su || vegetationScope.su !== 'None' && !vegetationScope.suPath ||
          sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openEnvironmentTable}>{projection.environmentTableAction.caption}: read-only saved data</button>
    {/if}
    {#if vegetationTableReaderAvailable}
      <button type="button" data-two-page-vegetation-table-open data-source-control={projection.vegetationTableAction.controlId}
        disabled={!vegetationScope.projectPath || !vegetationScope.su || vegetationScope.su !== 'None' && !vegetationScope.suPath ||
          sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openVegetationTable}>{projection.vegetationTableAction.caption}: read-only saved data</button>
    {/if}
    {#if googleEarthReaderAvailable}
      <button type="button" data-two-page-google-earth-open data-source-control={projection.googleEarthAction.controlId}
        disabled={!googleEarthPath || sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openGoogleEarth}>{projection.googleEarthAction.caption}: saved-plot KML</button>
    {/if}
    {#if auditReaderAvailable}
      <button type="button" data-two-page-audit-open data-source-control={projection.auditAction.controlId}
        disabled={sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked ||
          Boolean(getDraftState().error) || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openAudit}>{projection.auditAction.caption}: source history</button>
    {/if}
    {#if sourceScope}
      <button type="button" data-source-control={projection.reverseAction.controlId} data-two-page-su-environment-open
        disabled={sourceBlocked || childBlocked || getDraftState().unsaved || getDraftState().busy || getDraftState().blocked || Boolean(getDraftState().error)
          || [...sessions.values()].some(peer => Boolean(peer.view().error || peer.view().referenceError))}
        onclick={openExternalTransfer}>{projection.reverseAction.caption}: separate file</button>
    {/if}
    <button type="button" disabled={childBlocked || close.unsaved || close.busy || close.blocked} aria-pressed={form === 'FS882-8x6XL'}
      onclick={() => chooseForm('FS882-8x6XL')}>Normal two-page source</button>
    <button type="button" disabled={childBlocked || close.unsaved || close.busy || close.blocked} aria-pressed={form === 'FS882-8x6XL-CHARS'}
      onclick={() => chooseForm('FS882-8x6XL-CHARS')}>CHARS two-page source</button>
    <button type="button" disabled={childBlocked || !reviewEnabled || close.unsaved || close.busy || close.blocked}
      onclick={() => operation('load')}>Load complete entry</button>
    <button type="button" disabled={childBlocked || !editingEnabled || !close.canSave || close.busy || close.blocked}
      onclick={() => operation('save')}>Save complete entry</button>
    <button type="button" disabled={close.busy} onclick={() => operation('undo')}>Undo / reload</button>
    <button type="button" disabled={!reviewEnabled || close.busy || !view.original || view.blocked}
      onclick={() => operation('references')}>Refresh draft choices</button>
    <button type="button" disabled={childBlocked || close.unsaved || close.busy || close.blocked} onclick={closeReview}>Close complete-entry review</button>
    {#if view.busy && view.operation === 'read' || view.referencesBusy}
      <button type="button" onclick={cancel}>Cancel entry read</button>
    {/if}
    {#if view.historyId}
      <button type="button" disabled={childBlocked || !editingEnabled || close.unsaved || close.busy || close.blocked}
        onclick={() => operation('retain')}>Restore (retain audits)</button>
      <button type="button" disabled={childBlocked || !editingEnabled || close.unsaved || close.busy || close.blocked}
        onclick={() => operation('prune')}>Restore (prune audits)</button>
    {/if}
  </div>
  {#if view.original && view.snapshot}
    <TwoPageEntryLayout {form} {pictureReaderAvailable} values={twoPageEntryValues(view.original, form)}
      editors={(['xl', 'common', 'extra'] as const).map(scope => ({
        scope, columns: projection.fields.filter(field => field.scope === scope && field.page !== null).map(field => field.column),
        input: fieldInput, labelled: true,
      }))} identity={identityField} embedded={childContent} />
  {/if}
  <p class="text-xs text-stone-600">One atomic parent Save covers all three accepted field scopes.
    Identity changes, coordinate-conversion callbacks, metadata creation, picture creation and unverified source actions remain unavailable.
    Linked children use their independently gated readers/editors outside this parent transaction.
    The picture child is read-only; metadata changes and record deletion remain outside this review.</p>
  {/if}
</section>

{#snippet identityField(field: TwoPageEntryField)}
  <label class="block text-sm font-medium" for="complete-entry-identity">Plot number</label>
  <input id="complete-entry-identity" class="w-full rounded border p-2" disabled
    data-column={field.column} value={owner.plot} />
{/snippet}

{#snippet fieldInput(control: PaperControl, _position: string)}
  {#if view.original && view.snapshot && control.column}
    {@const column = control.column}
    {@const policy = view.snapshot.Policies.find(field => field.column === column)}
    {@const draft = view.drafts[column]}
    {@const value = draft?.value ?? twoPageEntryCell(view.original, column)}
    {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
    {@const reference = view.snapshot.Fields.find(field => field.column === column)}
    {@const id = `complete-entry-${column}`}
    {@const unavailable = !editingEnabled || close.busy || view.blocked || childBlocked && !draft || column === 'BECSiteUnit' && !view.snapshot.MasterEditingAvailable}
    <div class="min-w-0 space-y-1" data-two-page-entry-field={column} data-source-control={control.controlId}>
      <label class="block text-sm font-medium" for={id}>{controlLabel(control, true)}</label>
      {#if column === 'ProjectID'}
        <select {id} class="w-full rounded border p-2" value={view.projectRowId ?? ''}
          disabled={unavailable || !view.snapshot.ProjectAssignmentAvailable}
          onchange={event => selectProject(event.currentTarget.value || null)}>
          <option value="">Keep original ({value.storage === 'null' ? 'NULL' : raw})</option>
          {#each view.snapshot.ProjectChoices.Choices.rows as row (row.rowId)}
            {@const code = row.cells[0]}
            {@const title = row.cells[1]}
            <option value={row.rowId} disabled={code.storage !== 'text' || !code.text || code.text.length > 30}>
              {metadataCellText(code)} — {title.storage === 'null' ? '(NULL title)' : title.text === '' ? '(empty title)' : metadataCellText(title)} [row {row.rowId}]
            </option>
          {/each}
        </select>
        {#if !view.snapshot.ProjectAssignmentAvailable}<p role="status" class="text-xs">{view.snapshot.ProjectAssignmentDiagnostic}</p>{/if}
      {:else if policy?.kind === 'boolean'}
        {@const selected = value.storage === 'null' ? '' : value.integer === '-1' ? 'true' : value.integer === '0' ? 'false' : 'historical'}
        <select {id} class="w-full rounded border p-2" value={selected} disabled={unavailable}
          onchange={event => stage(column, event.currentTarget.value === '' ? { kind: 'clear' }
            : { kind: 'boolean', value: event.currentTarget.value === 'true' })}>
          <option value="">NULL</option><option value="true">Yes</option><option value="false">No</option>
          {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
        </select>
      {:else if policy?.kind === 'option'}
        {@const selected = value.storage === 'null' ? '' : value.text === '1' || value.text === '2' ? value.text : 'historical'}
        {@const options = twoPageParentCommonFields(form).find(field => field.column === column)?.options ?? []}
        <select {id} class="w-full rounded border p-2" value={selected} disabled={unavailable}
          onchange={event => stage(column, { kind: 'option', option: event.currentTarget.value === '' ? null : Number(event.currentTarget.value) })}>
          <option value="">NULL</option>
          {#each options as option}<option value={String(option.value)}>{option.label}</option>{/each}
          {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
        </select>
      {:else if wideControl(control)}
        <textarea {id} class="w-full rounded border p-2" rows="3" value={raw} disabled={unavailable}
          aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
          oninput={event => stage(column, { kind: 'text', raw: event.currentTarget.value })}></textarea>
      {:else}
        <input {id} class="w-full rounded border p-2" type="text" value={raw} disabled={unavailable}
          list={reference?.available ? `${id}-choices` : undefined}
          aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
          oninput={event => stage(column, { kind: 'text', raw: event.currentTarget.value })} />
        {#if reference?.available}
          <datalist id={`${id}-choices`}>
            {#each referenceChoices(reference).filter(choice => choice.selectable && choice.code !== null) as choice (JSON.stringify([choice.rowId, choice.code, choice.description]))}
              <option value={choice.code ?? ''}>{choice.description === null ? '(NULL description)' : choice.description}</option>
            {/each}
          </datalist>
        {/if}
      {/if}
      {#if column !== 'ProjectID'}
        <div class="flex flex-wrap gap-2 text-xs">
          <button type="button" disabled={unavailable} onclick={() => stage(column, { kind: 'clear' })}>Clear to NULL</button>
          <button type="button" disabled={unavailable} onclick={() => stage(column, { kind: 'original' })}>Keep original</button>
        </div>
      {/if}
      {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-xs text-red-700">{draft.error}</p>{/if}
      {#if reference && !reference.available}<p role="status" class="text-xs">{reference.diagnostic}</p>{/if}
      {#if column === 'BECSiteUnit' && !view.snapshot.MasterEditingAvailable}
        <p role="status" class="text-xs">The owned current user is not authorized to change Master site units.</p>
      {/if}
      {#if view.advisories[column]}
        <p class="text-xs">{view.advisories[column]}</p>
        <button type="button" disabled={unavailable} onclick={() => acknowledge(column)}>Acknowledge literal {column}</button>
      {/if}
    </div>
  {/if}
{/snippet}
