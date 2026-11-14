<script lang="ts">
  import {
    PlotEntryService,
    ContextService,
    AuditRestoreAction,
    type FS882Header,
    type VegRecord,
    type HumusRecord,
    type MineralRecord,
    type OtherRecord,
    type SoilSuggestion,
    type OrdinaryAuditReadEntry,
    type ListItem,
    type SpeciesItem,
    type VegetationDeletionReview,
    type ProjectPlotProfileFilterRequest
  } from '../../bindings/github.com/boostao/vpro-wails';
  import { Lock, Unlock, Save, RotateCcw, AlertTriangle, Check } from '@lucide/svelte';
  import HeaderEditor from './HeaderEditor.svelte';
  import SourcePage from './SourcePage.svelte';
  import ParentCodeFields from './ParentCodeFields.svelte';
  import OrdinaryFields from './OrdinaryFields.svelte';
  import SourceChild from './SourceChild.svelte';
  import FS882SoilHorizonSection from './FS882SoilHorizonSection.svelte';
  import FS882OtherRecordsSection from './FS882OtherRecordsSection.svelte';
  import FS882AuditTrailSection from './FS882AuditTrailSection.svelte';
  import FS882StratumSummary from './FS882StratumSummary.svelte';
  import FS882ChildCreateModal from './FS882ChildCreateModal.svelte';
  import {
    buildDirectSourceChildWrite,
    executeReviewedSIVISessionOperation,
    historicalSoilReadError,
    newEmptyFS882Draft,
    newHumusFields,
    newMineralFields,
    newOtherFields,
    newVegetationFields,
    projectSourceRows,
    validateOrdinarySoilRecordIds,
    type FS882ChildKind,
  } from './fs882FormHelpers';
  import { subscribeTwoPageSoilActivity } from './twoPageSoilActivity';
  import { subscribeTwoPageVegetationActivity, twoPageVegetationActivity } from './twoPageVegetationActivity';
  import type { TwoPageVegetationForm } from './twoPageVegetationWire';
  import { siviCreationUndoSession as ownedSIVICreationUndoSession } from './siviCreationUndoSessions';
  import { siviDeletionSession as ownedSIVIDeletionSession,
    siviDeletionRestorationSession as ownedSIVIRestorationSession } from './siviDeletionSessions';
  import { siviCreationSession as ownedSIVICreationSession } from './siviCreationSessions';
  import type { SIVICreationDraft, SIVICreationForm } from './siviCreationEditor';
  import PicturePanel from '../pictures/PicturePanel.svelte';
  import { pictureMetadataSession as ownedPictureMetadataSession } from '../pictures/pictureMetadataSessions';
  import { SIVIParentSharedSession, siviParentSharedFieldsFor, type SIVIParentSharedColumn, type SIVIParentSharedInput } from './siviParentSharedSession';
  import { SIVIParentReadSession } from './siviParentReadSession';
  import { SIVIParentSourceSession } from './siviParentSourceSession';
  import { SIVIParentWriteSession, isSIVIParentDirectColumn } from './siviParentWriteSession';
  import { SIVIParentActionWriteSession, isSIVIParentActionColumn } from './siviParentActionWriteSession';
  import { SIVIProjectAssignmentSession, type SIVIProjectAssignmentInput } from './siviProjectAssignmentSession';
  import type { SIVIParentInput } from './siviParentEditor';
  import { SIVIHeightSession } from './siviHeightSession';
  import type { SIVIHeightColumn } from './siviHeightEditor';
  import { SIVICoverSession } from './siviCoverSession';
  import type { SIVICoverColumn } from './siviCoverEditor';
  import { SIVICombinedSession } from './siviCombinedSession';
  import { SIVICollectedSession } from './siviCollectedSession';
  import { SIVISpeciesSession } from './siviSpeciesSession';
  import { SIVIIdentitySession } from './siviIdentitySession';
  import { vegetationReadAvailability } from './vegetationReadAvailability';
  import type { SIVICombinedColumn } from './siviCombinedEditor';
  import type { TwoPageEntrySessionCache } from './twoPageEntrySession';
  import type { TwoPageParentSessionCache, TwoPageParentForm } from './twoPageParentSession';
  import type { TwoPageParentCommonSessionCache } from './twoPageParentCommonSession';
  import type { TwoPageEntryChild } from './twoPageEntryProjection';
  import { externalSUEnvironmentSession, subscribeExternalSUEnvironmentActivity } from '../classification/externalSUEnvironment';
  import { navigateSourceEnter } from './enterNavigation';
  import { projectState } from '../shared/state';
  import { embeddedForm, paperChild, type VegetationMode, type PaperControl } from './paperLayout';
  import type { EditorCloseState } from '../shell/closeLifecycle';
  import { heightField, stageHeight, heightDirty, heightErrors, heightSourceNotices, aCoverSourceNotice, aCoverSourceNotices, heightUpdates, vegetationNumberUpdates, type HeightDrafts } from './heightEditor';
  import { ordinaryTextError, ordinaryNumberValue } from './ordinaryEditor';
  import { otherField, stageOther, otherDirty, otherErrors, otherUpdates, type OtherDrafts, type OtherValue } from './otherEditor';
  import { soilField, stageSoil, soilDirty, soilErrors, soilUpdates, type SoilDrafts, type SoilKind } from './soilChildEditor';
  import { vegetationAttributeField, stageVegetationAttribute, vegetationAttributeDirty, vegetationAttributeErrors, vegetationAttributeUpdates, type VegetationAttributeDrafts } from './vegetationAttributeEditor';
  import { stageCollected, collectedDirty, collectedUpdates, type CollectedDrafts } from './collectedEditor';
  import { speciesForms, stageSpecies, speciesDirty, speciesErrors, speciesUpdates, chooseSpecies, speciesEventError, type SpeciesDrafts, type SpeciesLists, type SpeciesChoices, type SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import type { WorkingUnitSession } from './workingUnitEditor';
  import { onDestroy, untrack, tick } from 'svelte';
  import { bindContextPlots } from '../shared/contextPlots';
  import { ReadRequests } from '../shared/readRequests';
  import { validateDeletionReview } from './vegetationDeletionEditor';
  import { beginVegetationCreation, stageVegetationCreation, stageVegetationCreationSpecies, chooseVegetationCreationSpecies, vegetationCreationSpeciesError, vegetationCreationErrors, vegetationCreationRequest, type VegetationCreationDraft } from './vegetationCreationEditor';
  import { beginPersonalSpecies, beginCreationPersonalSpecies, matchesPersonalSpeciesSource, matchesCreationPersonalSpeciesSource, stagePersonalText, personalSpeciesErrors, personalSpeciesRequest, personalSpeciesMatches, personalSpeciesCommittedError, personalLifeforms, personalTextFields, type PersonalSpeciesDraft } from './personalSpeciesEditor';

  let { plotNumber, contextId, onSaved, onClosed, onBusyChange, onProfileNavigation, onProfileSUReview, onFindPlot, entryForm = 'fs882' }: { plotNumber?: string; contextId: string; onSaved?: (p: string) => void | Promise<void>; onClosed?: () => void; onBusyChange?: (busy: boolean) => void; onFindPlot?: () => void;
    entryForm?: 'fs882' | 'sivi';
    onProfileNavigation?: (proposal: ProjectPlotProfileFilterRequest, contextId: string) => Promise<void>;
    onProfileSUReview?: (proposal: ProjectPlotProfileFilterRequest, contextId: string) => Promise<void>;
  } = $props();
  const PlotService = { ...PlotEntryService, ...bindContextPlots(untrack(() => contextId)) };
  const siviContextId = untrack(() => contextId);
  const siviHeightEnabled = true;
  const siviCoverEnabled = true;
  const siviCombinedEnabled = true;
  const siviCollectedEnabled = true;
  const siviSpeciesEnabled = true;
  const siviIdentityEnabled = true;
  const siviCreationEnabled = true;
  const siviDeletionEnabled = true;
  const siviRestorationEnabled = true;
  const siviCreationUndoEnabled = true;
  const pictureReadingEnabled = true;
  const siviChildSourceNoticesEnabled = true;
  const siviParentReviewEnabled = true;
  const siviStandalone = untrack(() => entryForm === 'sivi');
  const siviStandaloneEnabled = true;
  const siviParentEditingEnabled = true;
  const siviParentSharedEnabled = true;
  const siviParentReferenceEnabled = true;
  const siviParentSharedColumns: readonly SIVIParentSharedColumn[] =
    siviParentSharedFieldsFor(siviParentReferenceEnabled).map(field => field.column);
  const siviParentSharedReads = new ReadRequests();
  let siviParentSharedSession = $state<SIVIParentSharedSession | null>(null);
  let siviParentSharedRevision = $state(0);
  const siviParentSharedView = $derived.by(() => { siviParentSharedRevision; return siviParentSharedSession?.view() ?? null; });
  const siviParentSharedClose = $derived.by(() => { siviParentSharedRevision; return siviParentSharedSession?.closeState() ?? null; });
  const siviParentSharedUnsaved = $derived(siviParentSharedClose?.unsaved ?? false);
  const siviParentWriteReads = new ReadRequests();
  let siviParentWriteSession = $state<SIVIParentWriteSession | null>(null);
  let siviParentWriteRevision = $state(0);
  const siviParentWriteView = $derived.by(() => { siviParentWriteRevision; return siviParentWriteSession?.view() ?? null; });
  const siviParentWriteClose = $derived.by(() => { siviParentWriteRevision; return siviParentWriteSession?.closeState() ?? null; });
  const siviParentWriteUnsaved = $derived(siviParentWriteClose?.unsaved ?? false);
  const siviParentActionEditingEnabled = true;
  const siviParentActionReads = new ReadRequests();
  let siviParentActionSession = $state<SIVIParentActionWriteSession | null>(null);
  let siviParentActionRevision = $state(0);
  const siviParentActionView = $derived.by(() => { siviParentActionRevision; return siviParentActionSession?.view() ?? null; });
  const siviParentActionClose = $derived.by(() => { siviParentActionRevision; return siviParentActionSession?.closeState() ?? null; });
  const siviParentActionUnsaved = $derived(siviParentActionClose?.unsaved ?? false);
  const siviProjectAssignmentEnabled = true;
  const siviProjectAssignmentReads = new ReadRequests();
  let siviProjectAssignmentSession = $state<SIVIProjectAssignmentSession | null>(null);
  let siviProjectAssignmentRevision = $state(0);
  const siviProjectAssignmentView = $derived.by(() => { siviProjectAssignmentRevision; return siviProjectAssignmentSession?.view() ?? null; });
  const siviProjectAssignmentClose = $derived.by(() => { siviProjectAssignmentRevision; return siviProjectAssignmentSession?.closeState() ?? null; });
  const siviProjectAssignmentUnsaved = $derived(siviProjectAssignmentClose?.unsaved ?? false);
  const siviParentReads = new ReadRequests();
  const siviParentSourceReads = new ReadRequests();
  let siviParentSourceSession = $state<SIVIParentSourceSession | null>(null);
  let siviParentSourceRevision = $state(0);
  const siviParentSourceView = $derived.by(() => { siviParentSourceRevision; return siviParentSourceSession?.view() ?? null; });
  const siviSourceAuthorityUnknown = $derived(siviParentSourceView?.authorityUnknown ?? false);
  const siviSourceBarrier = $derived(siviParentSourceView?.saving || siviSourceAuthorityUnknown);
  let siviParentSession = $state<SIVIParentReadSession | null>(null);
  let siviParentRevision = $state(0);
  let siviParentOpen = $state(false);
  const siviParentView = $derived.by(() => { siviParentRevision; return siviParentSession?.view() ?? null; });
  const siviParentBusy = $derived((siviParentView?.busy ?? false) || (siviParentSourceView?.busy ?? false) || (siviParentWriteView?.busy ?? false) || (siviParentActionView?.busy ?? false) || (siviProjectAssignmentView?.busy ?? false) || (siviParentSharedView?.busy ?? false));
  const siviReads = new ReadRequests();
  let siviSession: SIVIHeightSession | null = null;
  let siviRevision = $state(0);
  let siviPanelOpen = $state(false);
  const siviView = $derived.by(() => { siviRevision; return siviSession?.view() ?? null; });
  const siviClose = $derived.by(() => { siviRevision; return siviSession?.closeState() ?? null; });
  const siviUnsaved = $derived(siviClose?.unsaved ?? false);
  const siviBusy = $derived(siviClose?.busy ?? false);
  const siviCoverReads = new ReadRequests();
  let siviCoverSession: SIVICoverSession | null = null;
  let siviCoverRevision = $state(0);
  let siviCoverPanelOpen = $state(false);
  let siviCoverReading = $state(false);
  const siviCoverView = $derived.by(() => { siviCoverRevision; return siviCoverSession?.view() ?? null; });
  const siviCoverClose = $derived.by(() => { siviCoverRevision; return siviCoverSession?.closeState() ?? null; });
  const siviCoverUnsaved = $derived(siviCoverClose?.unsaved ?? false);
  const siviCoverBusy = $derived(siviCoverClose?.busy ?? false);
  const siviCombinedReads = new ReadRequests();
  let siviCombinedSession: SIVICombinedSession | null = null;
  let siviCombinedRevision = $state(0);
  let siviCombinedPanelOpen = $state(false);
  let siviCombinedReading = $state(false);
  const siviCombinedView = $derived.by(() => { siviCombinedRevision; return siviCombinedSession?.view() ?? null; });
  const siviCombinedClose = $derived.by(() => { siviCombinedRevision; return siviCombinedSession?.closeState() ?? null; });
  const siviCombinedUnsaved = $derived(siviCombinedClose?.unsaved ?? false);
  const siviCombinedBusy = $derived(siviCombinedClose?.busy ?? false);
  const siviCollectedReads = new ReadRequests();
  let siviCollectedSession: SIVICollectedSession | null = null;
  let siviCollectedRevision = $state(0);
  let siviCollectedPanelOpen = $state(false);
  let siviCollectedReading = $state(false);
  const siviCollectedView = $derived.by(() => { siviCollectedRevision; return siviCollectedSession?.view() ?? null; });
  const siviCollectedClose = $derived.by(() => { siviCollectedRevision; return siviCollectedSession?.closeState() ?? null; });
  const siviCollectedUnsaved = $derived(siviCollectedClose?.unsaved ?? false);
  const siviCollectedBusy = $derived(siviCollectedClose?.busy ?? false);
  const siviSpeciesReads = new ReadRequests();
  let siviSpeciesSession: SIVISpeciesSession | null = null;
  let siviSpeciesRevision = $state(0);
  let siviSpeciesPanelOpen = $state(false);
  let siviSpeciesReading = $state(false);
  const siviSpeciesView = $derived.by(() => { siviSpeciesRevision; return siviSpeciesSession?.view() ?? null; });
  const siviSpeciesClose = $derived.by(() => { siviSpeciesRevision; return siviSpeciesSession?.closeState() ?? null; });
  const siviSpeciesUnsaved = $derived(siviSpeciesClose?.unsaved ?? false);
  const siviSpeciesBusy = $derived(siviSpeciesClose?.busy ?? false);
  const siviIdentityReads = new ReadRequests();
  let siviIdentitySession: SIVIIdentitySession | null = null;
  let siviIdentityRevision = $state(0);
  let siviIdentityPanelOpen = $state(false);
  let siviIdentityReading = $state(false);
  const siviIdentityView = $derived.by(() => { siviIdentityRevision; return siviIdentitySession?.view() ?? null; });
  const siviIdentityClose = $derived.by(() => { siviIdentityRevision; return siviIdentitySession?.closeState() ?? null; });
  const siviIdentityUnsaved = $derived(siviIdentityClose?.unsaved ?? false);
  const siviIdentityBusy = $derived(siviIdentityClose?.busy ?? false);
  let vegetationReadUnavailable = $state<string | null>(null);
  let historicalSoilUnavailable = $state<string | null>(null);
  const reads = new ReadRequests();
  const referenceReads = new ReadRequests();
  const speciesReads = new ReadRequests();
  const speciesReferenceReads = new ReadRequests();
  const speciesChoiceReads = new ReadRequests();
  const deletionReads = new ReadRequests();
  let deletionRequest = 0;
  let deletionBusy = $state(false);
  let deletionReview = $state<VegetationDeletionReview | null>(null);
  const deletionEnabled = true;
  const creationEnabled = true;
  let creationDraft = $state<VegetationCreationDraft | null>(null);
  let creationChoices = $state<SpeciesChoices | null>(null);
  const personalSpeciesEnabled = true;
  let personalDraft = $state<PersonalSpeciesDraft | null>(null);
  let savedPersonalCodes = $state<string[]>([]);
  const personalInvalid = $derived(personalDraft ? personalSpeciesErrors(personalDraft) : []);
  let referenceRequest = 0;
  let speciesRequest = 0;
  let speciesReferenceRequest = 0;
  let speciesChoiceRequest = 0;
  let speciesDecisionBusy = $state(false);
  let speciesChoices = $state<Record<string, SpeciesChoices>>({});

  let activeTab = $state<'site' | 'veg' | 'vegOther' | 'soils' | 'other' | 'audit' | 'siviParent'>('site');
  async function vegetationNotesTab() {
    activeTab = 'soils';
    await tick();
    document.querySelector<HTMLInputElement>('#header-soilSurveyor')?.focus();
  }
  let vegetationMode = $state<VegetationMode>('initial');
  const extendedShrubsEnabled = true;
  let extendedShrubs = $state(false);
  function setExtendedShrubs(next: boolean) {
    extendedShrubs = next;
    siviSession?.presentation(next);
    siviCoverSession?.presentation(next);
    siviCombinedSession?.presentation(next);
    siviCollectedSession?.presentation(next);
    siviSpeciesSession?.presentation(next);
    siviIdentitySession?.presentation(next);
    if (next) vegetationMode = 'cover';
  }
  let busy = $state(false);
  let coordinateBusy = $state(false);
  let workingUnitBusy = $state(false);
  let qualityBusy = $state(false);
  let siteCodeBusy = $state(false);
  let regionCodeBusy = $state(false);
  let soilCodeBusy = $state(false);
  let geologyCodeBusy = $state(false);
  let parentCodeBusy = $state(false);
  let drainageBusy = $state(false);
  const soilCodeEditingEnabled = true;
  const codeCheckEnabled = true;
  let codeCheckOpen = $state(false);
  let codeCheckBusy = $state(false);
  let codeCheckEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const metadataEnabled = true;
  const metadataCreationEnabled = true;
  const metadataTemplateCreationEnabled = true;
  const metadataRestorationEnabled = true;
  const sourceEnterEnabled = true;
  function sourceEnter(event: KeyboardEvent) {
    if (!sourceEnterEnabled || !container) return;
    try { navigateSourceEnter(event, container); }
    catch (cause) { error = `Source Enter navigation failed; drafts retained: ${String(cause)}`; }
  }
  let metadataOpen = $state(false);
  let metadataBusy = $state(false);
  let pictureBusy = $state(false);
  const pictureMetadataWritingEnabled = true;
  const pictureMetadataSession = $derived.by(() => pictureMetadataWritingEnabled && siviContextId && $projectState?.activeProject && original?.plotNumber
    ? ownedPictureMetadataSession({ contextId: siviContextId, project: $projectState.activeProject, plotNumber: original.plotNumber }) : undefined);
  let pictureMetadataRevision = $state(0);
  $effect(() => pictureMetadataSession?.subscribe(() => untrack(() => pictureMetadataRevision++)));
  const pictureMetadataClose = $derived.by(() => { pictureMetadataRevision; return pictureMetadataSession?.closeState(); });
  const pictureMetadataPending = $derived(!!pictureMetadataClose &&
    (pictureMetadataClose.unsaved || pictureMetadataClose.blocked || pictureMetadataClose.busy));
  const siviCreationSession = $derived.by(() => siviCreationEnabled && siviContextId && $projectState?.activeProject && original?.plotNumber
    ? ownedSIVICreationSession({ contextId: siviContextId, project: $projectState.activeProject, plot: original.plotNumber }) : undefined);
  let siviCreationRevision = $state(0);
  let siviCreationHostBusy = $state(false);
  $effect(() => siviCreationSession?.subscribe(() => untrack(() => siviCreationRevision++)));
  const siviCreationView = $derived.by(() => { siviCreationRevision; return siviCreationSession?.view(); });
  const siviCreationClose = $derived.by(() => { siviCreationRevision; return siviCreationSession?.closeState(); });
  const siviCreationPending = $derived(!!siviCreationClose &&
    (siviCreationClose.unsaved || siviCreationClose.blocked || siviCreationClose.busy));
  const siviCreationBusy = $derived(siviCreationHostBusy || (siviCreationClose?.busy ?? false));
  const siviDeletionSession = $derived.by(() => siviDeletionEnabled && siviContextId && $projectState?.activeProject && original?.plotNumber
    ? ownedSIVIDeletionSession({ contextId: siviContextId, project: $projectState.activeProject, plot: original.plotNumber }) : undefined);
  const siviRestorationSession = $derived.by(() => siviRestorationEnabled && siviContextId && $projectState?.activeProject && original?.plotNumber
    ? ownedSIVIRestorationSession({ contextId: siviContextId, project: $projectState.activeProject, plot: original.plotNumber }) : undefined);
  const siviCreationUndoSession = $derived.by(() => siviCreationUndoEnabled && siviContextId && $projectState?.activeProject && original?.plotNumber
    ? ownedSIVICreationUndoSession({ contextId: siviContextId, project: $projectState.activeProject, plot: original.plotNumber }) : undefined);
  let siviDeletionRevision = $state(0);
  let siviRestorationRevision = $state(0);
  let siviCreationUndoRevision = $state(0);
  let siviLifecycleHostBusy = $state(false);
  $effect(() => siviDeletionSession?.subscribe(() => untrack(() => siviDeletionRevision++)));
  $effect(() => siviRestorationSession?.subscribe(() => untrack(() => siviRestorationRevision++)));
  $effect(() => siviCreationUndoSession?.subscribe(() => untrack(() => siviCreationUndoRevision++)));
  const siviDeletionView = $derived.by(() => { siviDeletionRevision; return siviDeletionSession?.view(); });
  const siviDeletionClose = $derived.by(() => { siviDeletionRevision; return siviDeletionSession?.closeState(); });
  const siviRestorationView = $derived.by(() => { siviRestorationRevision; return siviRestorationSession?.view(); });
  const siviRestorationClose = $derived.by(() => { siviRestorationRevision; return siviRestorationSession?.closeState(); });
  const siviCreationUndoView = $derived.by(() => { siviCreationUndoRevision; return siviCreationUndoSession?.view(); });
  const siviCreationUndoClose = $derived.by(() => { siviCreationUndoRevision; return siviCreationUndoSession?.closeState(); });
  const siviDeletionPending = $derived(!!siviDeletionClose &&
    (siviDeletionClose.unsaved || siviDeletionClose.blocked || siviDeletionClose.busy));
  const siviRestorationPending = $derived(!!siviRestorationClose &&
    (siviRestorationClose.unsaved || siviRestorationClose.blocked || siviRestorationClose.busy));
  const siviCreationUndoPending = $derived(!!siviCreationUndoClose &&
    (siviCreationUndoClose.unsaved || siviCreationUndoClose.blocked || siviCreationUndoClose.busy));
  const siviLifecycleBusy = $derived(siviLifecycleHostBusy || (siviDeletionClose?.busy ?? false) || (siviRestorationClose?.busy ?? false) || (siviCreationUndoClose?.busy ?? false));
  const siviRestorationTargets = $derived((siviRestorationView?.history?.events ?? []).map(event => ({
    historyId: event.historyId, restored: event.consumed,
    label: `${event.form}; Species ${event.species === null ? 'NULL' : event.species === '' ? '(empty text)' : event.species}; application ID ${event.id}; physical row ${event.rowId}; ${event.editWhen}`,
  })));
  const twoPageSoilReviewEnabled = true;
  const twoPageVegetationReviewEnabled = true;
  const twoPageReviewEnabled = true;
  const twoPageSourceLayoutEnabled = true;
  const twoPageCommonReviewEnabled = true;
  const twoPageEntryReviewEnabled = true;
  const ordinarySoilUnavailable: string | null = null;
  const ordinaryVegetationUnavailable: string | null = null;
  let twoPageOpen = $state(false);
  let twoPageScope = $state<'extra' | 'common' | 'entry'>('extra');
  let twoPageEntryInitialForm = $state<TwoPageParentForm>('FS882-8x6XL');
  let twoPageBusy = $state(false);
  let twoPageProject = $state<string | null>(null);
  const twoPageSessions: TwoPageParentSessionCache = new Map();
  const twoPageCommonSessions: TwoPageParentCommonSessionCache = new Map();
  const twoPageEntrySessions: TwoPageEntrySessionCache = new Map();
  let twoPageRevision = $state(0);
  let twoPageEditor = $state<{
    getCloseState(): EditorCloseState; getDraftState?(): EditorCloseState; undo(): void;
    canOpenMetadataEditor?(): boolean; openMetadataEditor?(): void;
  }>();
  let twoPageSoilBusy = $state(false);
  let twoPageSoilBlocked = $state(false);
  $effect(() => subscribeTwoPageSoilActivity(activity => {
    twoPageSoilBusy = activity.busy; twoPageSoilBlocked = activity.blocked;
  }));
  let twoPageVegetationBusy = $state(false);
  let twoPageVegetationBlocked = $state(false);
  let twoPageVegetationActivityRevision = $state(0);
  $effect(() => subscribeTwoPageVegetationActivity(activity => {
    twoPageVegetationBusy = activity.busy; twoPageVegetationBlocked = activity.blocked;
    untrack(() => twoPageVegetationActivityRevision++);
  }));
  const environmentSUEnabled = true;
  const siteUnitEnvironmentEnabled = true;
  let environmentSUOpen = $state(false);
  let environmentSUDirection = $state<'forward' | 'reverse' | 'external-reverse'>('forward');
  let environmentSUEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const externalSUEnvironmentEnabled = true;
  let externalSUEnvironmentBusy = $state(false);
  let externalSUEnvironmentBlocked = $state(false);
  const separateSUSelected = $derived(Boolean($projectState?.activeSU && $projectState.activeSU !== 'None' &&
    $projectState.suPath && $projectState.suPath !== $projectState.projectPath));
  $effect(() => {
    if (!externalSUEnvironmentEnabled || siviStandalone) return;
    const state = $projectState;
    const unsubscribe = subscribeExternalSUEnvironmentActivity(activity => {
      externalSUEnvironmentBusy = activity.busy; externalSUEnvironmentBlocked = activity.blocked;
    });
    if (state?.projectPath && state.suPath && separateSUSelected) {
      const retained = externalSUEnvironmentSession({ contextId, project: state.activeProject,
        projectPath: state.projectPath, su: state.activeSU, suPath: state.suPath });
      if (untrack(() => retained.view().blocked)) {
        const sourceForm = retained.requestSourceForm();
        if (sourceForm && twoPageEntryReviewEnabled) {
          twoPageEntryInitialForm = sourceForm; twoPageScope = 'entry'; twoPageProject = state.activeProject; twoPageOpen = true;
        } else { environmentSUDirection = 'external-reverse'; environmentSUOpen = true; }
      }
    }
    return unsubscribe;
  });
  const profileReviewEnabled = true;
  const profileRunEnabled = true;
  const profileSpeciesPresenceEnabled = true;
  const profileEditingEnabled = true;
  const profileCreationEnabled = true;
  const profileDeletionEnabled = true;
  const profileFilteringEnabled = true;
  const profileSaveSUEnabled = true;
  let profileReviewOpen = $state(false);
  let profileReviewBusy = $state(false);
  let profileReviewBlocked = $state(false);
  let profileEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  let metadataEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const independentHeaderWorkflowBusy = $derived(coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy || geologyCodeBusy || parentCodeBusy || drainageBusy || speciesDecisionBusy || deletionBusy || codeCheckBusy || metadataBusy || profileReviewBusy || environmentSUOpen || externalSUEnvironmentBusy || externalSUEnvironmentBlocked || siviParentBusy || siviBusy || siviCombinedBusy || siviCoverBusy || siviCollectedBusy || siviSpeciesBusy || siviIdentityBusy || siviCreationBusy || siviLifecycleBusy);
  const otherHeaderWorkflowBusy = $derived(independentHeaderWorkflowBusy || twoPageOpen || twoPageBusy);
  const headerWorkflowBusy = $derived(otherHeaderWorkflowBusy || pictureBusy || pictureMetadataPending || twoPageSoilBusy || twoPageSoilBlocked || twoPageVegetationBusy || twoPageVegetationBlocked);
  const tabWorkflowBusy = $derived(otherHeaderWorkflowBusy || pictureBusy || (pictureMetadataClose?.busy ?? false));
  const workingUnitSession: WorkingUnitSession & { plot: string } = { mode: null, plot: '' };
  const heightEditingEnabled = true;
  const numberEditingEnabled = true;
  let heightDrafts = $state<HeightDrafts>({});
  const heightUnsaved = $derived(heightDirty(heightDrafts));
  const heightInvalid = $derived(heightErrors(heightDrafts));
  const otherEditingEnabled = true;
  let otherDrafts = $state<OtherDrafts>({});
  const otherUnsaved = $derived(otherDirty(otherDrafts));
  const otherInvalid = $derived(otherErrors(otherDrafts));
  const soilEditingEnabled = true;
  let soilDrafts = $state<SoilDrafts>({});
  const soilUnsaved = $derived(soilDirty(soilDrafts));
  const soilInvalid = $derived(soilErrors(soilDrafts));
  let soilSuggestions = $state<SoilSuggestion[]>([]);
  let soilReferenceReady = $state(false);
  let soilReferenceBusy = $state(false);
  let soilReferenceError = $state<string | null>(null);
  const attributeEditingEnabled = true;
  let attributeDrafts = $state<VegetationAttributeDrafts>({});
  const attributeUnsaved = $derived(vegetationAttributeDirty(attributeDrafts));
  const attributeInvalid = $derived(vegetationAttributeErrors(attributeDrafts));
  let attributeSuggestions = $state<SoilSuggestion[]>([]);
  let attributeReferenceReady = $state(false);
  let attributeReferenceBusy = $state(false);
  let attributeReferenceError = $state<string | null>(null);
  const collectedEditingEnabled = true;
  let collectedDrafts = $state<CollectedDrafts>({});
  const collectedUnsaved = $derived(collectedDirty(collectedDrafts));
  const speciesEditingEnabled = true;
  let speciesDrafts = $state<SpeciesDrafts>({});
  let speciesLists = $state<SpeciesLists>({});
  let speciesReferenceReady = $state(false);
  let speciesReferenceBusy = $state(false);
  let speciesReferenceError = $state<string | null>(null);
  const speciesUnsaved = $derived(speciesDirty(speciesDrafts));
  const speciesInvalid = $derived(speciesErrors(speciesDrafts));
  const creationInvalid = $derived(creationDraft ? vegetationCreationErrors(creationDraft, speciesLists[creationDraft.form] ?? []) : []);
  const creationSpeciesInvalid = $derived(creationDraft ? vegetationCreationSpeciesError(creationDraft, speciesLists[creationDraft.form] ?? []) : null);
  const creationNotice = $derived(creationDraft && !Object.values(creationDraft.cells).some(cell => cell?.error !== null)
    ? aCoverSourceNotice(creationDraft.form, 'new', Object.fromEntries(Object.entries(creationDraft.cells).map(([field, cell]) => [field, cell?.value]))) : null);
  const siviCreationPeerUnsaved = $derived(heightUnsaved || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || deletionReview !== null || creationDraft !== null || codeCheckOpen || metadataOpen || profileReviewBlocked || environmentSUOpen);
  const nonParentChildUnsaved = $derived(siviCreationPeerUnsaved || siviCreationPending || siviDeletionPending || siviRestorationPending || siviCreationUndoPending);
  const childUnsaved = $derived(nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || twoPageOpen || twoPageSoilBusy || twoPageSoilBlocked || twoPageVegetationBusy || twoPageVegetationBlocked);
  let error = $state<string | null>(null);
  let successMsg = $state<string | null>(null);
  let dirty = $state(false);
  let headerValidation = $state<Record<string, string>>({});
  let capabilities = $state<Record<string, boolean | undefined>>({});
  let capabilitiesReady = $state(false);
  let masterAllowed = $state(false);
  type ChildKind = FS882ChildKind;
  let childCapabilities = $state<Record<ChildKind, Record<string, boolean | undefined>>>({ Veg: {}, Humus: {}, Mineral: {}, Other: {} });
  let loadRequest = 0;
  onDestroy(() => {
    for (const variants of twoPageSessions.values()) for (const owned of variants.values()) owned.dispose();
    for (const variants of twoPageCommonSessions.values()) for (const owned of variants.values()) owned.dispose();
    for (const variants of twoPageEntrySessions.values()) for (const owned of variants.values()) owned.dispose();
    siviParentSession?.dispose();
    siviParentSourceSession?.dispose();
    siviParentWriteSession?.dispose();
    siviParentActionSession?.dispose();
    siviProjectAssignmentSession?.dispose();
    siviParentSharedSession?.dispose();
    siviSession?.dispose();
    siviReads.cancelAll();
    siviCoverSession?.dispose();
    siviCoverReads.cancelAll();
    siviCombinedSession?.dispose();
    siviCombinedReads.cancelAll();
    siviCollectedSession?.dispose();
    siviCollectedReads.cancelAll();
    siviSpeciesSession?.dispose();
    siviSpeciesReads.cancelAll();
    siviIdentitySession?.dispose();
    siviIdentityReads.cancelAll();
    loadRequest++; referenceRequest++; speciesRequest++;
    reads.cancelAll(); referenceReads.cancelAll(); speciesReads.cancelAll();
    speciesReferenceRequest++; speciesReferenceReads.cancelAll();
    speciesChoiceRequest++; speciesChoiceReads.cancelAll();
    deletionRequest++; deletionReads.cancelAll();
    subEditorGeneration++;
  });
  let container: HTMLDivElement;
  let subEditorGeneration = 0;

  async function loadSubEditor<T>(
    loader: () => Promise<T>,
    label: string,
    onFailed: () => void,
  ): Promise<T> {
    const request = subEditorGeneration;
    try {
      return await loader();
    } catch (cause) {
      if (request === subEditorGeneration) {
        onFailed();
        error = `${label} could not load: ${String(cause)}`;
      }
      throw cause;
    }
  }

  $effect(() => { onBusyChange?.(busy || headerWorkflowBusy || siviSourceAuthorityUnknown); });

  // Reference lists for dropdowns
  let moistureList = $state<ListItem[]>([]);
  let nutrientList = $state<ListItem[]>([]);
  let mesoSlopeList = $state<ListItem[]>([]);
  let surfaceShapeList = $state<ListItem[]>([]);

  // Species search modal state
  let showSpeciesModal = $state(false);
  let speciesSearchQuery = $state('');
  let speciesSearchResults = $state<SpeciesItem[]>([]);
  let searchingSpecies = $state(false);
  let newChild = $state<'humus' | 'mineral' | 'other' | null>(null);
  function openNewChild(kind: 'humus' | 'mineral' | 'other') {
    if ((kind === 'other' ? otherEditingDisabled || otherUnsaved : soilEditingDisabled || soilUnsaved)) {
      error = 'Save or cancel drafts and wait for an unlocked, available child workflow before adding a row.';
      return;
    }
    error = null;
    newChild = kind;
  }

  async function deleteSourceChild(name: string, id: number) {
    if (childEditingDisabled || (name.startsWith('Soil') && soilEditingDisabled)) {
      error = 'Save or cancel drafts and wait for an unlocked, available child workflow before deleting a row.';
      return;
    }
    if (!window.confirm(`Delete this ${name} record?`)) return;
    if (name.startsWith('SubVeg')) await childOperation(() => PlotService.DeleteVegRecord(draft.plotNumber, id));
    else if (name === 'SoilHumusXL') await childOperation(() => PlotService.DeleteHumusRecord(draft.plotNumber, id));
    else if (name === 'SoilMineralXL') await childOperation(() => PlotService.DeleteMineralRecord(draft.plotNumber, id));
    else if (name === 'SubOtherXL') await childOperation(() => PlotService.DeleteOtherRecord(draft.plotNumber, id));
    else error = `Deletion is not migrated for ${name}.`;
  }

  async function createChild(first: string, second: string) {
    const kind = newChild;
    const soilUnavailable = ordinarySoilUnavailable ?? (typeof historicalSoilUnavailable !== 'undefined' ? historicalSoilUnavailable : null);
    if (soilUnavailable && (kind === 'humus' || kind === 'mineral')) {
      error = soilUnavailable;
      return;
    }
    let saved = false;
    if (kind === 'humus') saved = await childOperation(() => PlotService.SaveHumusRecord({
      ...newHumusFields,
      id: 0, plotNumber: draft.plotNumber, horizon: first || undefined, comment: second || undefined
    }));
    else if (kind === 'mineral') saved = await childOperation(() => PlotService.SaveMineralRecord({
      ...newMineralFields,
      id: 0, plotNumber: draft.plotNumber, horizon: first || undefined, comments: second || undefined
    }));
    else if (kind === 'other') saved = await childOperation(() => PlotService.SaveOtherRecord({
      ...newOtherFields,
      id: 0, plotNumber: draft.plotNumber, dataName: first || undefined, dataItem: second || undefined
    }));
    if (saved) newChild = null;
  }

  // Child lists
  let vegList = $state<VegRecord[]>([]);
  const heightNotices = $derived(heightSourceNotices(heightDrafts, vegList));
  const speciesNotices = $derived(aCoverSourceNotices(Object.entries(speciesDrafts).flatMap(([id, cell]) =>
    cell.error === null && cell.raw !== cell.expected ? [{ id: Number(id), form: cell.form }] : []), vegList));
  const collectedNotices = $derived(aCoverSourceNotices(Object.entries(collectedDrafts).flatMap(([id, cell]) =>
    cell.value !== cell.expected && cell.form ? [{ id: Number(id), form: cell.form }] : []), vegList));
  let humusList = $state<HumusRecord[]>([]);
  let mineralList = $state<MineralRecord[]>([]);
  let otherList = $state<OtherRecord[]>([]);
  let auditList = $state<OrdinaryAuditReadEntry[]>([]);
  let childRevision = $state(0);
  let headerRevision = $state(0);

  function sourceRows(name: string) {
    return projectSourceRows(name, { vegList, humusList, mineralList, otherList, childCapabilities });
  }

  function newDraft(): FS882Header {
    return newEmptyFS882Draft();
  }

  let draft = $state<FS882Header>(newDraft());
  const sourceHeaderValues = $derived(new Map(Object.entries(draft).map(([key, value]) => [key.toLowerCase(), value])));

  let original = $state<FS882Header | null>(null);
  let activeLayerDepthRaw = $state<string | null>(null);
  const snowCoverRegimeInput = $derived(draft.snowCoverRegime ?? '');
  const activeLayerDepthInput = $derived(
    activeLayerDepthRaw ?? (draft.activeLayerDepth == null ? '' : String(draft.activeLayerDepth)),
  );
  function stageSnowCoverRegime(raw: string) {
    if (headerInputsDisabled || capabilities.snowCoverRegime !== true) return;
    const value = raw === '' ? null : raw;
    draft.snowCoverRegime = value;
    const message = ordinaryTextError(
      { label: 'Snow cover regime', kind: 'text', maximum: 1 },
      value,
      original?.snowCoverRegime ?? null,
    );
    validateHeader('snowCoverRegime', message);
    markDirty();
  }
  function stageActiveLayerDepth(raw: string) {
    if (headerInputsDisabled || capabilities.activeLayerDepth !== true) return;
    activeLayerDepthRaw = raw;
    const cell = ordinaryNumberValue(
      { label: 'Active layer depth', kind: 'single' },
      raw,
      original?.activeLayerDepth ?? null,
    );
    if (cell.error === null) draft.activeLayerDepth = cell.value;
    validateHeader('activeLayerDepth', cell.error);
    markDirty();
  }
  const headerInputsDisabled = $derived(draft.locked || busy || siviSourceBarrier || !capabilitiesReady || childUnsaved || pictureBusy || pictureMetadataPending);
  const twoPageLinkedParentBlocked = $derived.by(() => {
    twoPageRevision;
    const parent = twoPageEditor?.getDraftState?.();
    return !twoPageOpen || twoPageScope !== 'entry' || !parent || parent.unsaved || parent.busy || !!parent.blocked ||
      draft.locked || busy || dirty || !capabilitiesReady || !original || siviSourceBarrier || independentHeaderWorkflowBusy ||
      pictureBusy || pictureMetadataPending || nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved ||
      siviProjectAssignmentUnsaved || siviParentSharedUnsaved || deletionReview !== null || creationDraft !== null || personalDraft !== null;
  });
  const twoPageSoilParentBlocked = $derived(twoPageLinkedParentBlocked || twoPageVegetationBusy || twoPageVegetationBlocked);
  const twoPageVegetationParentBlocked = $derived(twoPageLinkedParentBlocked || twoPageSoilBusy || twoPageSoilBlocked);
  function linkedVegetationParentBlocked(form: TwoPageVegetationForm): boolean {
    twoPageVegetationActivityRevision;
    if (twoPageVegetationParentBlocked || twoPageProject === null) return true;
    const others = twoPageVegetationActivity({ contextId, project: twoPageProject, plot: draft.plotNumber, form });
    return others.busy || others.blocked;
  }
  const childParentDisabled = $derived(!capabilitiesReady || draft.locked || busy || headerWorkflowBusy || siviSourceBarrier || dirty || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || original === null || deletionReview !== null || creationDraft !== null || personalDraft !== null || codeCheckOpen || metadataOpen || profileReviewBlocked || siviCreationPending || siviDeletionPending || siviRestorationPending || siviCreationUndoPending);
  const siviCreationRecoveryDisabled = $derived(!siviCreationEnabled || busy || siviCreationHostBusy || otherHeaderWorkflowBusy && !(siviCreationClose?.busy ?? false) ||
    pictureBusy || pictureMetadataPending || siviCreationPeerUnsaved || siviDeletionPending || siviRestorationPending || siviCreationUndoPending || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved);
  const siviDeletionRecoveryDisabled = $derived(!siviDeletionEnabled || busy || siviLifecycleHostBusy ||
    otherHeaderWorkflowBusy && !(siviDeletionClose?.busy ?? false) || pictureBusy || pictureMetadataPending ||
    siviCreationPeerUnsaved || siviCreationPending || siviRestorationPending || siviCreationUndoPending ||
    siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved);
  const siviRestorationRecoveryDisabled = $derived(!siviRestorationEnabled || busy || siviLifecycleHostBusy ||
    otherHeaderWorkflowBusy && !(siviRestorationClose?.busy ?? false) || pictureBusy || pictureMetadataPending ||
    siviCreationPeerUnsaved || siviCreationPending || siviDeletionPending || siviCreationUndoPending ||
    siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved);
  const siviCreationUndoRecoveryDisabled = $derived(!siviCreationUndoEnabled || busy || siviLifecycleHostBusy ||
    otherHeaderWorkflowBusy && !(siviCreationUndoClose?.busy ?? false) || pictureBusy || pictureMetadataPending ||
    siviCreationPeerUnsaved || siviCreationPending || siviDeletionPending || siviRestorationPending ||
    siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved);
  const siviCreationUndoEditingDisabled = $derived(siviCreationUndoRecoveryDisabled || !capabilitiesReady || draft.locked || siviSourceBarrier ||
    dirty || original === null || personalDraft !== null || Object.keys(headerValidation).length > 0);
  const siviDeletionEditingDisabled = $derived(siviDeletionRecoveryDisabled || !capabilitiesReady || draft.locked || siviSourceBarrier ||
    dirty || original === null || personalDraft !== null || Object.keys(headerValidation).length > 0);
  const siviRestorationEditingDisabled = $derived(siviRestorationRecoveryDisabled || !capabilitiesReady || draft.locked || siviSourceBarrier ||
    dirty || original === null || personalDraft !== null || Object.keys(headerValidation).length > 0);
  const siviCreationEditingDisabled = $derived(siviCreationRecoveryDisabled || !capabilitiesReady || draft.locked || siviSourceBarrier ||
    dirty || original === null || personalDraft !== null || Object.keys(headerValidation).length > 0);
  const siviParentSharedEditingDisabled = $derived(!siviParentSharedEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved ||
    personalDraft !== null || siviSourceAuthorityUnknown || Object.keys(headerValidation).length > 0 || (siviParentSharedView?.blocked ?? false));
  const siviParentWriteEditingDisabled = $derived(!siviParentEditingEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || personalDraft !== null || siviSourceAuthorityUnknown || Object.keys(headerValidation).length > 0 || (siviParentWriteView?.blocked ?? false));
  const siviParentActionEditingDisabled = $derived(!siviParentActionEditingEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || personalDraft !== null || siviSourceAuthorityUnknown || Object.keys(headerValidation).length > 0 || (siviParentActionView?.blocked ?? false));
  const siviProjectAssignmentEditingDisabled = $derived(!siviProjectAssignmentEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviParentSharedUnsaved || personalDraft !== null ||
    Object.keys(headerValidation).length > 0 || siviSourceAuthorityUnknown || Boolean(siviParentSourceView?.error) || (siviProjectAssignmentView?.blocked ?? false));
  const heightEditingDisabled = $derived(!numberEditingEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const otherEditingDisabled = $derived(!otherEditingEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const soilEditingDisabled = $derived(!!ordinarySoilUnavailable || !soilEditingEnabled || (typeof historicalSoilUnavailable !== 'undefined' && !!historicalSoilUnavailable) || !soilReferenceReady || soilReferenceBusy || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const attributeEditingDisabled = $derived(!attributeEditingEnabled || !attributeReferenceReady || attributeReferenceBusy || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || collectedUnsaved || speciesUnsaved);
  const collectedEditingDisabled = $derived(!collectedEditingEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || speciesUnsaved);
  const speciesEditingDisabled = $derived(!speciesEditingEnabled || !speciesReferenceReady || speciesReferenceBusy || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved);
  const siviEditingDisabled = $derived(!siviHeightEnabled || childParentDisabled || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviView?.blocked ?? false));
  const siviCoverEditingDisabled = $derived(!siviCoverEnabled || childParentDisabled || siviUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviCoverView?.blocked ?? false));
  const siviCombinedEditingDisabled = $derived(!siviCombinedEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviCombinedView?.blocked ?? false));
  const siviCollectedEditingDisabled = $derived(!siviCollectedEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviCollectedView?.blocked ?? false));
  const siviSpeciesEditingDisabled = $derived(!siviSpeciesEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviIdentityUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviSpeciesView?.blocked ?? false));
  const siviIdentityEditingDisabled = $derived(!siviIdentityEnabled || childParentDisabled || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviIdentityView?.blocked ?? false));
  const childEditingDisabled = $derived(childParentDisabled || childUnsaved);
  const auditRestoreEnabled = true;
  const auditProject = $derived($projectState?.activeProject ?? '');
  const auditBlocked = $derived(childEditingDisabled || Object.keys(headerValidation).length > 0);
  const auditGateReason = $derived(busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
    : draft.locked ? 'Unlock the plot before restoring fields.'
    : dirty || Object.keys(headerValidation).length > 0 ? 'Save valid header changes or Undo before restoring fields.'
    : 'Save the plot before restoring fields.');

  async function siviParentWriteOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviParentWriteSession || busy || headerWorkflowBusy || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviParentWriteEditingDisabled) {
      error = 'SIVI parent edits require a clean unlocked parent and no conflicting operation; resolve acknowledgements explicitly.'; return false;
    }
    error = null; successMsg = null;
    try {
      const ok = await executeReviewedSIVISessionOperation(siviParentWriteSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviParentWriteSession.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI directly bound parent fields saved and independently reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed parent restoration completed and reloaded.'
        : 'SIVI parent originals reloaded; no mutation was replayed.';
      return true;
    } catch (cause) { error = `SIVI parent operation failed; owner state retained: ${String(cause)}`; return false; }
  }

  async function siviParentActionOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviParentActionSession || busy || headerWorkflowBusy || siviParentWriteUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviParentActionEditingDisabled) {
      error = 'SIVI source actions require a clean unlocked parent and no conflicting drafts or operation; resolve acknowledgements explicitly.'; return false;
    }
    error = null; successMsg = null;
    try {
      const ok = await executeReviewedSIVISessionOperation(siviParentActionSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviParentActionSession.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI source actions saved; callback directive verified and owned parent reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed source-action restoration completed and reloaded.'
        : 'SIVI action originals reloaded; no mutation was replayed.';
      return true;
    } catch (cause) { error = `SIVI source-action operation failed; owner state retained: ${String(cause)}`; return false; }
  }

  function stageSIVIParentAction(column: string, input: SIVIParentInput) {
    try {
      if (!siviParentActionSession || siviParentActionEditingDisabled || !isSIVIParentActionColumn(column)) throw new Error('Only independently available SIVI source actions can be drafted.');
      siviParentActionSession.stage(column, input);
      error = siviParentActionSession.view().error; successMsg = null;
    } catch (cause) { error = `SIVI source-action draft failed; existing state retained: ${String(cause)}`; }
  }

  async function siviProjectAssignmentOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviProjectAssignmentSession || busy || headerWorkflowBusy || siviParentWriteUnsaved || siviParentActionUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviProjectAssignmentEditingDisabled) {
      error = 'ProjectID assignment requires a clean unlocked parent and no conflicting drafts or operation; resolve acknowledgements explicitly.'; return false;
    }
    error = null; successMsg = null;
    try {
      const ok = await executeReviewedSIVISessionOperation(siviProjectAssignmentSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviProjectAssignmentSession.view().error; return false; }
      successMsg = operation === 'save' ? 'Existing physical ProjectID definition assigned; owned parent and choices reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'Typed ProjectID restoration completed and reloaded; metadata was not changed.'
        : 'Owned ProjectID parent and choices reloaded; no mutation was replayed.';
      return true;
    } catch (cause) { error = `ProjectID assignment operation failed; owner state retained: ${String(cause)}`; return false; }
  }

  function stageSIVIProjectAssignment(input: SIVIProjectAssignmentInput) {
    try {
      if (!siviProjectAssignmentSession || siviProjectAssignmentEditingDisabled) throw new Error('Only independently available existing ProjectID definitions can be selected.');
      siviProjectAssignmentSession.stage('ProjectID', input);
      error = siviProjectAssignmentSession.view().error; successMsg = null;
    } catch (cause) { error = `ProjectID selection failed; existing state retained: ${String(cause)}`; }
  }

  async function refreshSIVIParent(plot: string, peers: (SIVIParentWriteSession | SIVIParentActionWriteSession | SIVIProjectAssignmentSession | SIVIParentSharedSession | null)[]) {
    for (const peer of peers) {
      if (peer && (peer.closeState().unsaved || peer.closeState().busy)) {
        throw new Error('Another SIVI parent editor has drafts or an operation; no peer state was replaced.');
      }
    }
    const header = await reads.track(ContextService.GetPlot(siviContextId, plot));
    if (!header || $projectState?.contextId !== siviContextId || header.plotNumber !== plot) {
      throw new Error('SIVI parent header refresh changed its context/plot owner.');
    }
    draft = { ...header };
    original = structuredClone(header);
    activeLayerDepthRaw = null;
    dirty = false;
    headerValidation = {};
    await loadChildData(plot);
    if (siviParentSession && !await siviParentSession.load()) {
      throw new Error(siviParentSession.view().error ?? 'Owned SIVI parent refresh did not finish.');
    }
    for (const peer of peers) {
      if (peer && (peer.view().original || peer.view().error) && !await peer.load()) {
        throw new Error(peer.view().error ?? 'Peer SIVI parent original refresh did not finish.');
      }
    }
  }

  function stageSIVIParentShared(column: SIVIParentSharedColumn, input: SIVIParentSharedInput) {
    try {
      if (!siviParentSharedSession || siviParentSharedEditingDisabled) throw new Error('Shared SIVI fields require their clean owned parent.');
      siviParentSharedSession.stage(column, input);
      error = siviParentSharedSession.view().error; successMsg = null;
    } catch (cause) { error = `Shared SIVI draft failed; owned state retained: ${String(cause)}`; }
  }

  async function siviParentSharedOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviParentSharedSession || busy || headerWorkflowBusy || siviParentWriteUnsaved || siviParentActionUnsaved ||
        siviProjectAssignmentUnsaved || operation !== 'undo' && siviParentSharedEditingDisabled) {
      error = 'Shared SIVI edits require a clean unlocked parent and no conflicting operation; recover uncertain writes explicitly.'; return false;
    }
    error = null; successMsg = null;
    try {
      const ok = await executeReviewedSIVISessionOperation(siviParentSharedSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviParentSharedSession.view().error; return false; }
      successMsg = 'Shared SIVI operation completed and owned originals reloaded; no mutation was replayed.';
      return true;
    } catch (cause) { error = `Shared SIVI operation failed; owner state retained: ${String(cause)}`; return false; }
  }

  async function changeSIVIProjectSource(source: number) {
    if (!siviParentSourceSession || busy || headerWorkflowBusy || dirty || childUnsaved) {
      error = 'Finish or Undo owned plot/selection drafts before changing ProjectID source.';
      return;
    }
    error = null;
    try {
      if (!await siviParentSourceSession.setSource(source)) {
        error = siviParentSourceSession.view().error;
        return;
      }
      if (siviProjectAssignmentSession?.view().original && !await siviProjectAssignmentSession.load()) {
        error = siviProjectAssignmentSession.view().error;
      }
    } catch (cause) {
      error = `ProjectID source change failed; owned state retained: ${String(cause)}`;
    }
  }

  async function reloadSIVIProjectSource() {
    if (!siviParentSourceSession || busy || headerWorkflowBusy || dirty || childUnsaved) {
      error = 'Finish or Undo owned plot/selection drafts before reloading ProjectID source.';
      return;
    }
    error = null;
    try {
      if (!await siviParentSourceSession.load()) {
        error = siviParentSourceSession.view().error;
        return;
      }
      if (siviProjectAssignmentSession?.view().original && !await siviProjectAssignmentSession.load()) {
        error = siviProjectAssignmentSession.view().error;
      }
    } catch (cause) {
      error = `ProjectID source reload failed; owned state retained: ${String(cause)}`;
    }
  }

  async function showSIVIParentPanel() {
    if (!siviParentReviewEnabled || busy || headerWorkflowBusy || dirty || nonParentChildUnsaved || !siviParentSession) {
      error = 'SIVI parent requires a clean available owned plot and its parent-review gate.';
      return;
    }
    activeTab = 'siviParent';
    siviParentOpen = true;
    await loadSIVIParentOriginals();
  }

  async function loadSIVIParentOriginals() {
    const parent = siviParentSession;
    if (!parent) throw new Error('SIVI parent originals have no current owned session.');
    if (!parent.view().original && !await parent.load()) return;
    if (siviParentSession !== parent) return;
    if (!siviParentWriteUnsaved && !siviParentActionUnsaved && !siviProjectAssignmentUnsaved && !siviParentSharedUnsaved &&
        siviParentSourceSession && !siviParentSourceView?.choices && !siviParentSourceView?.error &&
        !siviParentSourceView?.authorityUnknown) await siviParentSourceSession.load();
  }

  function stageSIVIParentCell(column: string, input: SIVIParentInput) {
    try {
      if (!siviParentWriteSession || siviParentWriteEditingDisabled || !isSIVIParentDirectColumn(column)) {
        throw new Error('Only available directly bound SIVI parent fields can be drafted.');
      }
      siviParentWriteSession.stage(column, input);
      error = siviParentWriteSession.view().error;
      successMsg = null;
    } catch (cause) {
      error = `SIVI parent draft failed; existing state retained: ${String(cause)}`;
    }
  }

  async function siviOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviSession || !siviHeightEnabled || busy || headerWorkflowBusy || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || operation !== 'undo' && siviEditingDisabled) {
      error = 'SIVI requires an available clean, unlocked parent and no conflicting operation or unresolved acknowledgement.'; return false;
    }
    error = null; successMsg = null;
    try {
      const ok = await executeReviewedSIVISessionOperation(siviSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviSession.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI aggregate heights saved; current rows and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed height restoration completed; current rows and audits reloaded.'
        : 'SIVI source rows reloaded. No height mutation was replayed.';
      return true;
    } catch (cause) { error = `SIVI operation failed; owner state retained: ${String(cause)}`; return false; }
  }

  function stageSIVICell(rowId: string, column: SIVIHeightColumn, raw: string, nullValue: boolean) {
    try {
      if (!siviSession || siviEditingDisabled) throw new Error('Unlock the clean available parent before editing SIVI heights.');
      siviSession.stage(rowId, column, raw, nullValue);
      const message = siviSession.view().error;
      if (message) error = `SIVI ${message}`; else if (error?.startsWith('SIVI ')) error = null;
      successMsg = null;
    } catch (cause) { error = `SIVI draft failed; original/raw state retained: ${String(cause)}`; }
  }

  async function showSIVIPanel() {
    if (siviPanelOpen) { siviPanelOpen = false; return; }
    if (siviEditingDisabled && !siviUnsaved) { error = 'SIVI requires a clean, unlocked available parent.'; return; }
    siviPanelOpen = true;
    siviCoverPanelOpen = false; siviCombinedPanelOpen = false; siviCollectedPanelOpen = false; siviSpeciesPanelOpen = false; siviIdentityPanelOpen = false;
    if (!siviView?.review && !siviUnsaved) await siviOperation('load');
  }

  async function siviCoverOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviCoverSession || !siviCoverEnabled || busy || headerWorkflowBusy || siviUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved || heightUnsaved ||
        otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved ||
        siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved ||
        operation !== 'undo' && siviCoverEditingDisabled) {
      error = 'SIVI covers require a clean, unlocked available parent and no conflicting drafts or operation.'; return false;
    }
    error = null; successMsg = null; siviCoverReading = operation === 'load';
    try {
      const ok = await executeReviewedSIVISessionOperation(siviCoverSession, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviCoverSession.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI source covers saved; current rows and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed cover restoration completed; current rows and audits reloaded.'
        : 'SIVI source cover rows reloaded. No mutation was replayed.';
      return true;
    } catch (cause) { error = `SIVI cover operation failed; owner state retained: ${String(cause)}`; return false; }
    finally { siviCoverReading = false; }
  }

  async function refreshSIVIChildEditors(plot: string, owner: 'height' | 'cover' | 'combined' | 'collected' | 'species' | 'identity' | 'creation' | 'deletion' | 'restoration' | 'creationUndo'): Promise<void> {
    if (owner !== 'creation' && siviCreationPending || owner !== 'deletion' && siviDeletionPending ||
      owner !== 'restoration' && siviRestorationPending || owner !== 'creationUndo' && siviCreationUndoPending) {
      throw new Error('SIVI lifecycle peer owns an unresolved review; no peer originals were reset.');
    }
    const peers = [owner !== 'height' ? siviSession : null, owner !== 'cover' ? siviCoverSession : null,
      owner !== 'combined' ? siviCombinedSession : null, owner !== 'collected' ? siviCollectedSession : null,
      owner !== 'species' ? siviSpeciesSession : null, owner !== 'identity' ? siviIdentitySession : null];
    for (const peer of peers) {
      if (peer?.closeState().unsaved || peer?.closeState().busy || peer?.closeState().blocked) {
        throw new Error('SIVI peer owns unresolved drafts or an operation; no peer originals were reset.');
      }

    }
    await loadChildData(plot);
    for (const peer of peers) {
      if (peer?.view().review && !await peer.refreshSource()) {
        throw new Error('SIVI peer originals failed to refresh; reload explicitly before continuing.');
      }
    }
  }

  async function refreshSIVILifecycleLists(owner: 'creation' | 'deletion' | 'restoration' | 'creationUndo'): Promise<void> {
    if (siviDeletionSession && (owner === 'deletion' || siviDeletionSession.view().targets !== null) &&
      !await siviDeletionSession.loadTargets()) throw new Error('Reload deletion source targets explicitly.');
    if (siviRestorationSession && (owner === 'restoration' || siviRestorationSession.view().history !== null) &&
      !await siviRestorationSession.loadHistory()) throw new Error('Reload owned deletion history explicitly.');
    if (siviCreationUndoSession && (owner === 'creationUndo' || siviCreationUndoSession.view().history !== null) &&
      !await siviCreationUndoSession.loadHistory()) throw new Error('Reload owned creation Undo history explicitly.');
  }

  async function siviCreationOperation(operation: 'load' | 'start' | 'save' | 'undo' | 'resolve' | 'cancel', form?: SIVICreationForm): Promise<boolean> {
    const session = siviCreationSession;
    if (!session || !siviCreationEnabled) { error = 'SIVI creation is unavailable for this owned parent.'; return false; }
    if (operation === 'cancel') { session.cancel(); return true; }
    if (siviCreationRecoveryDisabled || (operation === 'load' || operation === 'start' || operation === 'save') && siviCreationEditingDisabled) {
      error = 'SIVI creation requires its available owner and no conflicting drafts or operation.';
      return false;
    }
    const revision = session.view().revision;
    error = null;
    successMsg = null;
    try {
      if (operation === 'start') {
        if (!form) throw new Error('Choose one explicit SIVI source form before starting creation.');
        session.start(form);
      } else if (operation === 'undo') session.discard();
      else if (operation === 'load') {
        if (!await session.loadReferences()) { error = session.view().error; return false; }
      } else {
        const ok = operation === 'save' ? await session.save(crypto.randomUUID()) : await session.resolve();
        if (!ok) { error = session.view().error; return false; }
        const receipt = session.view().receipt;
        if (!receipt || session !== siviCreationSession) throw new Error('The verified creation owner changed before peer refresh.');
        siviCreationHostBusy = true;
        await refreshSIVIChildEditors(receipt.plot, 'creation');
        await refreshSIVILifecycleLists('creation');
        successMsg = `SIVI creation ${receipt.id} verified; ${receipt.replayed ? 'read-only resolution, no mutation repeated' : 'one row committed'}.`;
      }
      return true;
    } catch (cause) {
      if (session.view().revision > revision) {
        capabilitiesReady = false;
        error = `SIVI creation is already verified; peer refresh failed. Reload originals without repeating Create: ${String(cause)}`;
      } else error = `SIVI creation operation failed; retained owner state unchanged: ${String(cause)}`;
      return false;
    } finally { siviCreationHostBusy = false; }
  }

  function stageSIVICreation(draft: SIVICreationDraft) {
    try {
      if (!siviCreationSession || siviCreationEditingDisabled) throw new Error('Unlock the clean owned parent before correcting SIVI creation.');
      siviCreationSession.updateDraft(draft);
      error = siviCreationSession.view().error;
      successMsg = null;
    } catch (cause) { error = `SIVI creation correction failed; retained draft unchanged: ${String(cause)}`; }
  }

  async function siviLifecycleOperation(kind: 'deletion' | 'restoration',
    operation: 'load' | 'history' | 'review' | 'save' | 'undo' | 'resolve' | 'cancel',
    identity?: string, form?: SIVICreationForm): Promise<boolean> {
    const session = kind === 'deletion' ? siviDeletionSession : siviRestorationSession;
    const enabled = kind === 'deletion' ? siviDeletionEnabled : siviRestorationEnabled;
    if (!session || !enabled) { error = `SIVI ${kind} is unavailable for this owned parent.`; return false; }
    if (operation === 'cancel') { session.cancel(); return true; }
    const recoveryDisabled = kind === 'deletion' ? siviDeletionRecoveryDisabled : siviRestorationRecoveryDisabled;
    const editingDisabled = kind === 'deletion' ? siviDeletionEditingDisabled : siviRestorationEditingDisabled;
    if (recoveryDisabled || operation !== 'undo' && operation !== 'resolve' && editingDisabled) {
      error = `SIVI ${kind} requires its clean available owner and no conflicting drafts or operation.`;
      return false;
    }
    const revision = session.view().revision;
    error = null;
    successMsg = null;
    try {
      if (operation === 'undo') session.discard();
      else if (operation === 'load' && kind === 'deletion') {
        if (!siviDeletionSession || !await siviDeletionSession.loadTargets()) { error = session.view().error; return false; }
      } else if (operation === 'history' && kind === 'restoration') {
        if (!siviRestorationSession || !await siviRestorationSession.loadHistory()) { error = session.view().error; return false; }
      } else if (operation === 'review') {
        if (!identity) throw new Error('Select one explicit loaded source row or owned historical event.');
        const ok = kind === 'deletion'
          ? form && siviDeletionSession && await siviDeletionSession.review(form, identity)
          : siviRestorationSession && await siviRestorationSession.review(identity);
        if (!ok) { error = session.view().error ?? 'Choose an exact source form before reviewing deletion.'; return false; }
      } else if (operation === 'save' || operation === 'resolve') {
        if (!await (operation === 'save' ? session.save(crypto.randomUUID()) : session.resolve())) {
          error = session.view().error;
          return false;
        }
        const receipt = session.view().receipt;
        if (!receipt || session !== (kind === 'deletion' ? siviDeletionSession : siviRestorationSession)) {
          throw new Error('The verified lifecycle owner changed before peer refresh.');
        }
        siviLifecycleHostBusy = true;
        await refreshSIVIChildEditors(receipt.plot, kind);
        await refreshSIVILifecycleLists(kind);
        successMsg = `SIVI ${kind} ${receipt.id} verified; ${receipt.replayed ? 'read-only resolution, no mutation repeated' : 'one exact physical row committed'}.`;
      } else throw new Error('Unsupported source lifecycle operation; no action was inferred.');
      return true;
    } catch (cause) {
      if (session.view().revision > revision) {
        capabilitiesReady = false;
        error = `SIVI ${kind} is already verified; source refresh failed. Reload the owned plot without repeating the mutation: ${String(cause)}`;
      } else error = `SIVI ${kind} failed; retained owner state unchanged: ${String(cause)}`;
      return false;
    } finally { siviLifecycleHostBusy = false; }
  }

  function stageSIVILifecycle(kind: 'deletion' | 'restoration', confirmed: boolean, action?: 'retain' | 'prune') {
    try {
      if (kind === 'deletion') {
        if (!siviDeletionSession || siviDeletionEditingDisabled) throw new Error('Unlock the clean owned parent before confirming deletion.');
        siviDeletionSession.confirm(confirmed);
        error = siviDeletionSession.view().error;
      } else {
        if (!siviRestorationSession || siviRestorationEditingDisabled) throw new Error('Unlock the clean owned parent before confirming restoration.');
        if (action) siviRestorationSession.choose(action);
        else siviRestorationSession.confirm(confirmed);
        error = siviRestorationSession.view().error;
      }
      successMsg = null;
    } catch (cause) { error = `SIVI ${kind} confirmation failed; retained review unchanged: ${String(cause)}`; }
  }

  async function siviCreationUndoOperation(operation: 'history' | 'review' | 'undo' | 'discard' | 'resolve' | 'cancel',
    historyId?: string): Promise<boolean> {
    const session = siviCreationUndoSession;
    if (!session || !siviCreationUndoEnabled) { error = 'SIVI creation Undo is unavailable for this owned parent.'; return false; }
    if (operation === 'cancel') { session.cancel(); return true; }
    if (siviCreationUndoRecoveryDisabled || operation !== 'discard' && operation !== 'resolve' && siviCreationUndoEditingDisabled) {
      error = 'SIVI creation Undo requires its clean available owner and no conflicting drafts or operation.';
      return false;
    }
    const revision = session.view().revision;
    error = null;
    successMsg = null;
    try {
      if (operation === 'discard') session.discard();
      else if (operation === 'history') {
        if (!await session.loadHistory()) { error = session.view().error; return false; }
      } else if (operation === 'review') {
        if (!historyId) throw new Error('Choose an explicit available owned historical creation.');
        if (!await session.review(historyId)) { error = session.view().error; return false; }
      } else {
        if (!await (operation === 'undo' ? session.undo(crypto.randomUUID()) : session.resolve())) {
          error = session.view().error;
          return false;
        }
        const receipt = session.view().receipt;
        if (!receipt || session !== siviCreationUndoSession) throw new Error('The verified creation Undo owner changed before peer refresh.');
        siviLifecycleHostBusy = true;
        await refreshSIVIChildEditors(receipt.plot, 'creationUndo');
        await refreshSIVILifecycleLists('creationUndo');
        successMsg = `SIVI creation Undo ${receipt.id} verified; ${receipt.replayed ? 'read-only resolution, no mutation repeated' : 'one exact created row removed'}.`;
      }
      return true;
    } catch (cause) {
      if (session.view().revision > revision) {
        capabilitiesReady = false;
        error = `SIVI creation Undo is already verified; source refresh failed. Reload the owned plot without repeating Undo: ${String(cause)}`;
      } else error = `SIVI creation Undo failed; retained owner state unchanged: ${String(cause)}`;
      return false;
    } finally { siviLifecycleHostBusy = false; }
  }

  function stageSIVICreationUndo(confirmed: boolean, action?: 'retain' | 'prune') {
    try {
      if (!siviCreationUndoSession || siviCreationUndoEditingDisabled) throw new Error('Unlock the clean owned parent before confirming creation Undo.');
      if (action) siviCreationUndoSession.choose(action);
      else siviCreationUndoSession.confirm(confirmed);
      error = siviCreationUndoSession.view().error;
      successMsg = null;
    } catch (cause) { error = `SIVI creation Undo confirmation failed; retained review unchanged: ${String(cause)}`; }
  }

  function stageSIVICoverCell(rowId: string, column: SIVICoverColumn, raw: string, nullValue: boolean) {
    try {
      if (!siviCoverSession || siviCoverEditingDisabled) throw new Error('Unlock the clean available parent before editing SIVI covers.');
      siviCoverSession.stage(rowId, column, raw, nullValue);
      const message = siviCoverSession.view().error;
      if (message) error = `SIVI cover ${message}`;
      else if (error?.startsWith('SIVI cover ')) error = null;
      successMsg = null;
    } catch (cause) {
      error = `SIVI cover draft failed; original/raw state retained: ${String(cause)}`;
    }
  }

  async function showSIVICoverPanel() {
    if (siviCoverPanelOpen) { siviCoverPanelOpen = false; return; }
    if (siviCoverEditingDisabled && !siviCoverUnsaved) { error = 'SIVI covers require a clean, unlocked available parent.'; return; }
    siviCoverPanelOpen = true;
    siviPanelOpen = false;
    siviCombinedPanelOpen = false;
    siviCollectedPanelOpen = false;
    siviSpeciesPanelOpen = false;
    siviIdentityPanelOpen = false;
    if (!siviCoverView?.review && !siviCoverUnsaved) await siviCoverOperation('load');
  }

  async function siviCombinedOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviCombinedSession || !siviCombinedEnabled || busy || headerWorkflowBusy || siviUnsaved || siviCoverUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved ||
        heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved ||
        siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved ||
        operation !== 'undo' && siviCombinedEditingDisabled) {
      error = 'SIVI combined editing requires a clean, unlocked available parent and no conflicting drafts or operation.'; return false;
    }
    error = null; successMsg = null; siviCombinedReading = operation === 'load';
    const owner = siviCombinedSession;
    try {
      const ok = await executeReviewedSIVISessionOperation(owner, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (owner !== siviCombinedSession) return false;
      if (!ok) { error = owner.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI covers and heights saved atomically; current rows and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed combined restoration completed; current rows and audits reloaded.'
        : 'SIVI combined source rows reloaded. No mutation was replayed.';
      return true;
    } catch (cause) {
      if (owner !== siviCombinedSession) return false;
      error = `SIVI combined operation failed; owner state retained: ${String(cause)}`; return false;
    } finally { if (owner === siviCombinedSession) siviCombinedReading = false; }
  }

  function stageSIVICombinedCell(rowId: string, column: SIVICombinedColumn, raw: string, nullValue: boolean) {
    try {
      if (!siviCombinedSession || siviCombinedEditingDisabled) throw new Error('Unlock the clean available parent before editing combined SIVI rows.');
      siviCombinedSession.stage(rowId, column, raw, nullValue);
      const message = siviCombinedSession.view().error;
      if (message) error = `SIVI combined ${message}`; else if (error?.startsWith('SIVI combined ')) error = null;
      successMsg = null;
    } catch (cause) { error = `SIVI combined draft failed; original/raw state retained: ${String(cause)}`; }
  }

  async function showSIVICombinedPanel() {
    if (siviCombinedPanelOpen) { siviCombinedPanelOpen = false; return; }
    if (siviCombinedEditingDisabled && !siviCombinedUnsaved) { error = 'Combined SIVI rows require a clean, unlocked available parent.'; return; }
    siviCombinedPanelOpen = true;
    siviPanelOpen = false; siviCoverPanelOpen = false; siviCollectedPanelOpen = false; siviSpeciesPanelOpen = false; siviIdentityPanelOpen = false;
    if (!siviCombinedView?.review && !siviCombinedUnsaved) await siviCombinedOperation('load');
  }

  async function siviCollectedOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviCollectedSession || !siviCollectedEnabled || busy || headerWorkflowBusy || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviSpeciesUnsaved || siviIdentityUnsaved ||
        heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved ||
        siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved ||
        operation !== 'undo' && siviCollectedEditingDisabled) {
      error = 'SIVI Collected editing requires a clean, unlocked available parent and no conflicting drafts or operation.'; return false;
    }
    error = null; successMsg = null; siviCollectedReading = operation === 'load';
    const owner = siviCollectedSession;
    try {
      const ok = await executeReviewedSIVISessionOperation(owner, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (owner !== siviCollectedSession) return false;
      if (!ok) { error = owner.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI Collected cycle intent saved; current rows and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed Collected restoration completed; current rows and audits reloaded.'
        : 'SIVI Collected source rows reloaded. No mutation was replayed.';
      return true;
    } catch (cause) {
      if (owner !== siviCollectedSession) return false;
      error = `SIVI Collected operation failed; owner state retained: ${String(cause)}`; return false;
    } finally { if (owner === siviCollectedSession) siviCollectedReading = false; }
  }

  function cycleSIVICollected(rowId: string) {
    try {
      if (!siviCollectedSession || siviCollectedEditingDisabled) throw new Error('Unlock the clean available parent before cycling SIVI Collected.');
      siviCollectedSession.cycle(rowId);
      const message = siviCollectedSession.view().error;
      if (message) error = `SIVI Collected ${message}`; else if (error?.startsWith('SIVI Collected ')) error = null;
      successMsg = null;
    } catch (cause) { error = `SIVI Collected cycle failed; original state retained: ${String(cause)}`; }
  }

  async function showSIVICollectedPanel() {
    if (siviCollectedPanelOpen) { siviCollectedPanelOpen = false; return; }
    if (siviCollectedEditingDisabled && !siviCollectedUnsaved) { error = 'SIVI Collected requires a clean, unlocked available parent.'; return; }
    siviCollectedPanelOpen = true;
    siviPanelOpen = false; siviCoverPanelOpen = false; siviCombinedPanelOpen = false; siviSpeciesPanelOpen = false; siviIdentityPanelOpen = false;
    if (!siviCollectedView?.review && !siviCollectedUnsaved) await siviCollectedOperation('load');
  }

  async function siviSpeciesOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviSpeciesSession || !siviSpeciesEnabled || busy || headerWorkflowBusy || siviUnsaved || siviCoverUnsaved || siviCombinedUnsaved || siviCollectedUnsaved || siviIdentityUnsaved ||
        heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved ||
        siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved ||
        operation !== 'undo' && siviSpeciesEditingDisabled) {
      error = 'SIVI Species editing requires a clean, unlocked available parent and no conflicting drafts or operation.'; return false;
    }
    error = null; successMsg = null; siviSpeciesReading = operation === 'load';
    const owner = siviSpeciesSession;
    try {
      const ok = await executeReviewedSIVISessionOperation(owner, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (owner !== siviSpeciesSession) return false;
      if (!ok) { error = owner.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI Species source decisions saved; current rows, definitions and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed Species restoration completed; current rows, definitions and audits reloaded.'
        : 'SIVI Species rows and definitions reloaded. No mutation was replayed.';
      return true;
    } catch (cause) {
      if (owner !== siviSpeciesSession) return false;
      error = `SIVI Species operation failed; owner state retained: ${String(cause)}`; return false;
    } finally { if (owner === siviSpeciesSession) siviSpeciesReading = false; }
  }

  function stageSIVISpecies(rowId: string, command: 'Species' | 'context' | SpeciesDecisionKind, raw: string) {
    try {
      if (!siviSpeciesSession || siviSpeciesEditingDisabled) throw new Error('Unlock the clean available parent before editing SIVI Species.');
      siviSpeciesSession.stage(rowId, command, raw, false);
      const message = siviSpeciesSession.view().error;
      if (message) error = `SIVI Species ${message}`; else if (error?.startsWith('SIVI Species ')) error = null;
      successMsg = null;
    } catch (cause) { error = `SIVI Species draft failed; original/raw state retained: ${String(cause)}`; }
  }

  async function showSIVISpeciesPanel() {
    if (siviSpeciesPanelOpen) { siviSpeciesPanelOpen = false; return; }
    if (siviSpeciesEditingDisabled && !siviSpeciesUnsaved) { error = 'SIVI Species requires a clean, unlocked available parent.'; return; }
    siviSpeciesPanelOpen = true;
    siviPanelOpen = false; siviCoverPanelOpen = false; siviCombinedPanelOpen = false; siviCollectedPanelOpen = false; siviIdentityPanelOpen = false;
    if (!siviSpeciesView?.review && !siviSpeciesUnsaved) await siviSpeciesOperation('load');
  }

  async function siviIdentityOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviIdentitySession || !siviIdentityEnabled || busy || headerWorkflowBusy || siviUnsaved || siviCoverUnsaved ||
        siviCombinedUnsaved || siviCollectedUnsaved || siviSpeciesUnsaved || heightUnsaved || otherUnsaved || soilUnsaved ||
        attributeUnsaved || collectedUnsaved || speciesUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved ||
        siviProjectAssignmentUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviIdentityEditingDisabled) {
      error = 'SIVI ID editing requires a clean, unlocked available parent and no conflicting drafts or operation.'; return false;
    }
    error = null; successMsg = null; siviIdentityReading = operation === 'load';
    const owner = siviIdentitySession;
    try {
      const ok = await executeReviewedSIVISessionOperation(owner, operation, AuditRestoreAction.AuditRestoreRetain, AuditRestoreAction.AuditRestorePrune);
      if (owner !== siviIdentitySession) return false;
      if (!ok) { error = owner.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI IDs saved with technical restoration history; no source field audits were created.'
        : operation === 'retain' || operation === 'prune' ? 'Original IDs restored without source audit pruning or vegetation deletion.'
        : 'Owned SIVI ID source rows reloaded. No mutation was replayed.';
      return true;
    } catch (cause) {
      if (owner !== siviIdentitySession) return false;
      error = `SIVI ID operation failed; owner state retained: ${String(cause)}`; return false;
    } finally { if (owner === siviIdentitySession) siviIdentityReading = false; }
  }

  function stageSIVIIdentity(rowId: string, raw: string, nullValue: boolean) {
    try {
      if (!siviIdentitySession || siviIdentityEditingDisabled) throw new Error('Unlock the clean available parent before editing SIVI IDs.');
      siviIdentitySession.stage(rowId, 'ID', raw, nullValue);
      const message = siviIdentitySession.view().error;
      if (message) error = `SIVI ID ${message}`; else if (error?.startsWith('SIVI ID ')) error = null;
      successMsg = null;
    } catch (cause) { error = `SIVI ID draft failed; original/raw state retained: ${String(cause)}`; }
  }

  async function showSIVIIdentityPanel() {
    if (siviIdentityPanelOpen) { siviIdentityPanelOpen = false; return; }
    if (siviIdentityEditingDisabled && !siviIdentityUnsaved) { error = 'SIVI IDs require a clean, unlocked available parent.'; return; }
    siviIdentityPanelOpen = true;
    siviPanelOpen = false; siviCoverPanelOpen = false; siviCombinedPanelOpen = false; siviCollectedPanelOpen = false; siviSpeciesPanelOpen = false;
    if (!siviIdentityView?.review && !siviIdentityUnsaved) await siviIdentityOperation('load');
  }

  function openSpeciesCodeCheck() {
    if (!codeCheckEnabled || childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0) {
      error = 'Finish or Undo existing drafts and unlock the available plot before reviewing species codes.';
      return;
    }
    codeCheckOpen = true;
    if (typeof codeCheckBusy !== 'undefined') codeCheckBusy = true;
    error = null; successMsg = null;
  }

  export function canOpenProjectMetadata() {
    if (twoPageOpen) {
      return metadataEnabled && twoPageScope === 'entry' && Object.keys(headerValidation).length === 0 &&
        typeof twoPageEditor?.openMetadataEditor === 'function' && Boolean(twoPageEditor.canOpenMetadataEditor?.());
    }
    return metadataEnabled && !childParentDisabled && !childUnsaved && Object.keys(headerValidation).length === 0;
  }

  export function openProjectMetadata() {
    if (!canOpenProjectMetadata()) {
      error = 'Finish or Undo existing drafts and unlock the available plot before reviewing project metadata.';
      return;
    }
    if (twoPageOpen) twoPageEditor?.openMetadataEditor?.();
    else {
      metadataOpen = true;
      if (typeof metadataBusy !== 'undefined') metadataBusy = true;
    }
    error = null; successMsg = null;
  }

  export function canOpenMetadataInPlace() {
    return twoPageOpen && canOpenProjectMetadata();
  }

  function openTwoPageReview() {
    const project = $projectState?.activeProject;
    if (!project || $projectState?.contextId !== siviContextId || !twoPageReviewEnabled || siviStandalone || childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0) {
      error = 'Finish or Undo existing drafts and unlock the available plot before reviewing two-page additional fields.';
      return;
    }
    twoPageEntryInitialForm = extendedShrubs ? 'FS882-8x6XL-CHARS' : 'FS882-8x6XL';
    twoPageScope = 'extra';
    twoPageProject = project; twoPageOpen = true; error = null; successMsg = null;
  }

  function openTwoPageCommonReview() {
    if (twoPageOpen) {
      error = 'Finish and close the current two-page field review before changing its scope.';
      return;
    }
    if (!twoPageCommonReviewEnabled) {
      error = 'Two-page common-field review is independently disabled.';
      return;
    }
    openTwoPageReview();
    if (twoPageOpen) twoPageScope = 'common';
  }

  function openTwoPageEntryReview() {
    if (twoPageOpen || !twoPageEntryReviewEnabled) {
      error = 'Complete-entry review is disabled or another two-page owner is still open.';
      return;
    }
    openTwoPageReview();
    if (twoPageOpen) twoPageScope = 'entry';
  }

  async function refreshTwoPagePeers(scope: 'extra' | 'common' | 'entry') {
    await refreshSIVIParent(draft.plotNumber, [siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession]);
    for (const cohort of [{ scope: 'extra', cache: twoPageSessions }, { scope: 'common', cache: twoPageCommonSessions },
      { scope: 'entry', cache: twoPageEntrySessions }]) {
      if (cohort.scope === scope) continue;
      const variants = cohort.cache.get(JSON.stringify({ contextId, project: twoPageProject, plot: draft.plotNumber }));
      if (!variants) continue;
      for (const peer of variants.values()) {
        if (!peer.view().original && !peer.view().error) continue;
        if (peer.closeState().unsaved || peer.closeState().busy || !await peer.load()) {
          throw new Error(peer.view().error ?? 'Another two-page field scope could not be refreshed; no write was replayed.');
        }
      }
    }
  }

  async function refreshProjectMetadata() {
    if (!siviParentSession?.view().original && !siviParentSession?.view().error) {
      await loadChildData(draft.plotNumber);
      return;
    }
    await refreshSIVIParent(draft.plotNumber, [siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession]);
    if (siviParentSourceSession && (siviParentSourceSession.view().choices || siviParentSourceSession.view().error) &&
      !await siviParentSourceSession.load()) {
      throw new Error(siviParentSourceSession.view().error ?? 'ProjectID choices did not refresh after the metadata commit.');
    }
  }

  function openEnvironmentSU(direction: 'forward' | 'reverse' = 'forward') {
    if (!(direction === 'reverse' ? siteUnitEnvironmentEnabled : environmentSUEnabled) || childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0 ||
        !$projectState?.activeSU || $projectState.activeSU === 'None' || !original) {
      error = 'Select an owned-project SU, finish or Undo all drafts and unlock the available plot before reviewing Env Into SU.';
      return;
    }
    environmentSUDirection = direction; environmentSUOpen = true; error = null; successMsg = null;
  }

  function openExternalSUEnvironment() {
    if (!externalSUEnvironmentEnabled || siviStandalone || !separateSUSelected || childParentDisabled ||
        childUnsaved || Object.keys(headerValidation).length > 0 || !original) {
      error = 'Select a separate SU file and finish or Undo all drafts before reviewing SU Into Env.';
      return;
    }
    const state = $projectState;
    if (!state?.projectPath || !state.suPath) {
      error = 'Selected project and SU paths are unavailable; no transfer review was opened.';
      return;
    }
    externalSUEnvironmentSession({ contextId, project: state.activeProject, projectPath: state.projectPath,
      su: state.activeSU, suPath: state.suPath }).reviewSourceForm(null);
    environmentSUDirection = 'external-reverse'; environmentSUOpen = true; error = null; successMsg = null;
  }

  async function refreshAfterRestore(plot: string, request: number) {
    const header = await reads.track(PlotService.GetPlot(plot));
    if (!header) throw new Error('Restored plot could not be reloaded.');
    await loadChildData(plot);
    if (request !== loadRequest) return;
    const locked = draft.locked;
    draft = { ...header, locked };
    original = JSON.parse(JSON.stringify(header));
    activeLayerDepthRaw = null; dirty = false; headerValidation = {};
  }

  async function load(p?: string) {
    if (siviDeletionPending || siviRestorationPending || siviCreationUndoPending) { error = 'Finish the source deletion/restoration/creation Undo review or resolve its exact retained request before replacing the plot owner.'; return; }
    if (siviCreationPending) { error = 'Finish SIVI creation or resolve its exact retained request before replacing the plot owner.'; return; }
    if (pictureMetadataPending) { error = 'Finish or explicitly discard picture metadata drafts/recovery before replacing their plot owner.'; return; }
    if (siviParentSharedSession && (siviParentSharedSession.closeState().unsaved || siviParentSharedSession.closeState().busy)) { error = 'Finish or explicitly Undo shared SIVI drafts/recovery before replacing their plot owner.'; return; }
    if (siviProjectAssignmentSession && (siviProjectAssignmentSession.closeState().unsaved || siviProjectAssignmentSession.closeState().busy)) { error = 'Finish or explicitly Undo ProjectID selection/recovery before replacing its plot owner.'; return; }
    if (siviParentActionSession && (siviParentActionSession.closeState().unsaved || siviParentActionSession.closeState().busy)) { error = 'Finish or explicitly Undo SIVI source-action drafts/recovery before replacing their plot owner.'; return; }
    if (siviParentWriteSession && (siviParentWriteSession.closeState().unsaved || siviParentWriteSession.closeState().busy)) { error = 'Finish or explicitly Undo SIVI parent drafts/recovery before replacing their plot owner.'; return; }
    if (siviParentSourceView?.busy) { error = 'Wait for the current SIVI source operation before replacing its plot owner.'; return; }
    if (siviSession && (siviSession.closeState().unsaved || siviSession.closeState().busy)) { error = 'SIVI drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    if (siviCoverSession && (siviCoverSession.closeState().unsaved || siviCoverSession.closeState().busy)) { error = 'SIVI cover drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    if (siviCombinedSession && (siviCombinedSession.closeState().unsaved || siviCombinedSession.closeState().busy)) { error = 'Combined SIVI drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    if (siviCollectedSession && (siviCollectedSession.closeState().unsaved || siviCollectedSession.closeState().busy)) { error = 'SIVI Collected drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    if (siviSpeciesSession && (siviSpeciesSession.closeState().unsaved || siviSpeciesSession.closeState().busy)) { error = 'SIVI Species drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    if (siviIdentitySession && (siviIdentitySession.closeState().unsaved || siviIdentitySession.closeState().busy)) { error = 'SIVI ID drafts or operations must finish or be explicitly undone before replacing their plot owner.'; return; }
    siviParentSession?.dispose(); siviParentSession = null;
    siviParentSourceSession?.dispose(); siviParentSourceSession = null;
    siviParentWriteSession?.dispose(); siviParentWriteSession = null;
    siviParentActionSession?.dispose(); siviParentActionSession = null;
    siviProjectAssignmentSession?.dispose(); siviProjectAssignmentSession = null;
    siviParentSharedSession?.dispose(); siviParentSharedSession = null;
    siviParentSharedRevision++; siviProjectAssignmentRevision++; siviParentActionRevision++;
    siviParentWriteRevision++; siviParentSourceRevision++; siviParentRevision++;
    siviParentOpen = false;
    if (activeTab === 'siviParent') activeTab = 'site';
    siviSession?.dispose(); siviSession = null; siviReads.cancelAll(); siviRevision++; siviPanelOpen = false;
    siviCoverSession?.dispose(); siviCoverSession = null; siviCoverReads.cancelAll(); siviCoverRevision++; siviCoverPanelOpen = false;
    siviCombinedSession?.dispose(); siviCombinedSession = null; siviCombinedReads.cancelAll(); siviCombinedRevision++; siviCombinedPanelOpen = false;
    siviCollectedSession?.dispose(); siviCollectedSession = null; siviCollectedReads.cancelAll(); siviCollectedRevision++; siviCollectedPanelOpen = false;
    siviSpeciesSession?.dispose(); siviSpeciesSession = null; siviSpeciesReads.cancelAll(); siviSpeciesRevision++; siviSpeciesPanelOpen = false;
    siviIdentitySession?.dispose(); siviIdentitySession = null; siviIdentityReads.cancelAll(); siviIdentityRevision++; siviIdentityPanelOpen = false;
    const request = ++loadRequest;
    reads.cancelAll();
    if (workingUnitSession.plot !== (p ?? '')) workingUnitSession.mode = null;
    workingUnitSession.plot = p ?? '';
    busy = true; error = null; capabilitiesReady = false; masterAllowed = false;
    heightDrafts = {}; otherDrafts = {}; soilDrafts = {}; attributeDrafts = {}; collectedDrafts = {};
    speciesDrafts = {}; speciesChoices = {}; deletionReview = null; creationDraft = null; creationChoices = null;
    personalDraft = null; savedPersonalCodes = []; vegList = []; vegetationReadUnavailable = null;
    historicalSoilUnavailable = null; humusList = []; mineralList = []; otherList = []; auditList = [];
    childCapabilities = { Veg: {}, Humus: {}, Mineral: {}, Other: {} };
    try {
      const [fields, res, masterPolicy] = await Promise.all([
        reads.track(PlotService.GetHeaderCapabilities()),
        p ? reads.track(PlotService.GetPlot(p)) : Promise.resolve(null),
        reads.track(PlotService.CanEditMasterBEC())
      ]);
      if (request !== loadRequest) return;
      if (fields === null) throw new Error('Header capabilities were not returned.');
      capabilities = fields;
      masterAllowed = masterPolicy;
      if (p && !res) throw new Error(`Plot ${p} could not be loaded.`);
      if (p) await loadChildData(p);
      if (request !== loadRequest) return;
      if (res !== null) {
        draft = { ...res };
        original = JSON.parse(JSON.stringify(res));
        activeLayerDepthRaw = null;
        dirty = false;
        headerValidation = {};
        if (siviParentReviewEnabled && $projectState?.contextId === siviContextId) {
          const plot = res.plotNumber;
          siviParentSession = new SIVIParentReadSession(
            { contextId: siviContextId, project: $projectState.activeProject, plot },
            () => siviParentReads.track(ContextService.GetSIVIParentOriginal(siviContextId, plot)),
            () => siviParentReads.cancelAll(), () => siviParentRevision++);
          siviParentSourceSession = new SIVIParentSourceSession(
            { contextId: siviContextId, project: $projectState.activeProject, plot }, {
              join: () => siviParentSourceReads.track(ContextService.GetSIVIParentJoinReview(siviContextId, plot)),
              choices: () => siviParentSourceReads.track(ContextService.GetSIVIProjectIDChoices(siviContextId)),
              setSource: (expected, source) => ContextService.SetSIVIProjectIDSource(siviContextId, expected, source),
              cancel: () => siviParentSourceReads.cancelAll(),
            }, () => siviParentSourceRevision++);
          if (siviParentEditingEnabled) {
            siviParentWriteSession = new SIVIParentWriteSession(
              { contextId: siviContextId, project: $projectState.activeProject, plot }, {
                read: () => siviParentWriteReads.track(ContextService.GetSIVIParentDirectOriginal(siviContextId, plot)),
                cancelRead: () => siviParentWriteReads.cancelAll(),
                save: request => ContextService.SaveSIVIParentDirect(siviContextId, plot, request),
                restore: (historyId, action) => ContextService.RestoreSIVIParentDirect(siviContextId, plot, historyId, action),
                refreshParent: () => refreshSIVIParent(plot, [siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession]),
              }, () => siviParentWriteRevision++);
            siviParentWriteRevision++;
          }
          if (siviParentActionEditingEnabled) {
            siviParentActionSession = new SIVIParentActionWriteSession(
              { contextId: siviContextId, project: $projectState.activeProject, plot }, {
                read: () => siviParentActionReads.track(ContextService.GetSIVIParentActionOriginal(siviContextId, plot)),
                cancelRead: () => siviParentActionReads.cancelAll(),
                save: request => ContextService.SaveSIVIParentActions(siviContextId, plot, request),
                restore: (historyId, action) => ContextService.RestoreSIVIParentActions(siviContextId, plot, historyId, action),
                refreshParent: () => refreshSIVIParent(plot, [siviParentWriteSession, siviProjectAssignmentSession, siviParentSharedSession]),
              }, () => siviParentActionRevision++);
            siviParentActionRevision++;
          }
          if (siviProjectAssignmentEnabled) {
            siviProjectAssignmentSession = new SIVIProjectAssignmentSession(
              { contextId: siviContextId, project: $projectState.activeProject, plot }, {
                read: () => siviProjectAssignmentReads.track(ContextService.GetSIVIProjectAssignmentOriginal(siviContextId, plot)),
                cancelRead: () => siviProjectAssignmentReads.cancelAll(),
                save: request => ContextService.SaveSIVIProjectAssignment(siviContextId, plot, request),
                restore: (historyId, action) => ContextService.RestoreSIVIProjectAssignment(siviContextId, plot, historyId, action),
                refreshParent: () => refreshSIVIParent(plot, [siviParentWriteSession, siviParentActionSession, siviParentSharedSession]),
              }, () => siviProjectAssignmentRevision++);
            siviProjectAssignmentRevision++;
          }
          if (siviParentSharedEnabled) {
            siviParentSharedSession = new SIVIParentSharedSession(
              { contextId: siviContextId, project: $projectState.activeProject, plot }, {
                read: () => siviParentSharedReads.track(PlotEntryService.GetSIVIParentSharedOriginal(siviContextId, plot)),
                readReferences: zone => siviParentSharedReads.track(PlotEntryService.GetSIVIParentSharedReferences(siviContextId, plot, zone)),
                cancelRead: () => siviParentSharedReads.cancelAll(),
                save: request => PlotEntryService.SaveSIVIParentShared(siviContextId, plot, request),
                restore: (historyId, action) => PlotEntryService.RestoreSIVIParentShared(siviContextId, plot, historyId, action),
                refreshParent: () => refreshSIVIParent(plot, [siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession]),
              }, () => siviParentSharedRevision++, siviParentReferenceEnabled);
            siviParentSharedRevision++;
          }
          siviParentSourceRevision++;
          siviParentRevision++;
        }
        if (siviHeightEnabled) {
          const plot = res.plotNumber;
          siviSession = new SIVIHeightSession(plot, {
            read: extended => siviReads.track(ContextService.GetSIVIVegetation(siviContextId, plot, extended)),
            save: (extended, review, edits) => ContextService.SaveSIVIHeights(siviContextId, plot, extended, review, edits),
            restore: (historyId, action) => ContextService.RestoreSIVIHeights(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'height'),
          }, () => siviRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviRevision++;
        }
        if (siviCoverEnabled) {
          const plot = res.plotNumber;
          siviCoverSession = new SIVICoverSession(plot, {
            read: extended => siviCoverReads.track(PlotEntryService.GetSIVICoverOriginal(siviContextId, plot, extended)),
            save: (extended, request) => PlotEntryService.SaveSIVICoverReviewed(siviContextId, plot, extended, request),
            restore: (historyId, action) => PlotEntryService.RestoreSIVICoverReviewed(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'cover'),
          }, () => siviCoverRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviCoverRevision++;
        }
        if (siviCombinedEnabled) {
          const plot = res.plotNumber;
          siviCombinedSession = new SIVICombinedSession(plot, {
            read: extended => siviCombinedReads.track(PlotEntryService.GetSIVICombinedOriginal(siviContextId, plot, extended)),
            save: (extended, request) => PlotEntryService.SaveSIVICombinedReviewed(siviContextId, plot, extended, request),
            restore: (historyId, action) => PlotEntryService.RestoreSIVICombinedReviewed(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'combined'),
          }, () => siviCombinedRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviCombinedRevision++;
        }
        if (siviCollectedEnabled) {
          const plot = res.plotNumber;
          siviCollectedSession = new SIVICollectedSession(plot, {
            read: extended => siviCollectedReads.track(PlotEntryService.GetSIVICollectedOriginal(siviContextId, plot, extended)),
            save: (extended, request) => PlotEntryService.SaveSIVICollectedReviewed(siviContextId, plot, extended, request),
            restore: (historyId, action) => PlotEntryService.RestoreSIVICollectedReviewed(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'collected'),
          }, () => siviCollectedRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviCollectedRevision++;
        }
        if (siviSpeciesEnabled) {
          const plot = res.plotNumber;
          const project = $projectState?.activeProject;
          if (!project) throw new Error('SIVI Species requires the active owned project before creating its editor.');
          siviSpeciesSession = new SIVISpeciesSession({ contextId: siviContextId, project, plot }, {
            read: async extended => {
              const [original, references] = await Promise.all([
                siviSpeciesReads.track(PlotEntryService.GetSIVISpeciesOriginal(siviContextId, plot, extended)),
                siviSpeciesReads.track(PlotEntryService.GetSIVISpeciesReferences(siviContextId, plot)),
              ]);
              return { original, references };
            },
            save: (extended, request) => PlotEntryService.SaveSIVISpeciesReviewed(siviContextId, plot, extended, request),
            restore: (historyId, action) => PlotEntryService.RestoreSIVISpeciesReviewed(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'species'),
          }, () => siviSpeciesRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviSpeciesRevision++;
        }
        if (siviIdentityEnabled) {
          const plot = res.plotNumber;
          siviIdentitySession = new SIVIIdentitySession(plot, {
            read: extended => siviIdentityReads.track(PlotEntryService.GetSIVIIdentityOriginal(siviContextId, plot, extended)),
            save: (extended, request) => PlotEntryService.SaveSIVIIdentityReviewed(siviContextId, plot, extended, request),
            restore: (historyId, action) => PlotEntryService.RestoreSIVIIdentityReviewed(siviContextId, plot, historyId, action),
            refreshParent: () => refreshSIVIChildEditors(plot, 'identity'),
          }, () => siviIdentityRevision++, extendedShrubs, siviChildSourceNoticesEnabled);
          siviIdentityRevision++;
        }
      } else {
        draft = newDraft();
        original = null;
        activeLayerDepthRaw = null;
        dirty = false;
        headerValidation = {};
      }
      capabilitiesReady = true;
      if (siviStandalone && siviStandaloneEnabled && siviParentSession) {
        activeTab = 'siviParent';
        siviParentOpen = true;
        await loadSIVIParentOriginals();
        if (request !== loadRequest) return;
      }
    } catch (e) {
      if (request === loadRequest) error = String(e);
    } finally {
      if (request === loadRequest) busy = false;
    }
  }

  async function loadChildData(p: string) {
    const request = loadRequest;
    try {
      const [v, hOutcome, mOutcome, o, a, vc, hc, mc, oc] = await Promise.all([
        ordinaryVegetationUnavailable ? { records: [], reason: ordinaryVegetationUnavailable } : siviIdentityEnabled
          ? reads.track(ContextService.GetVegetationReadAvailability(siviContextId, p)).then(vegetationReadAvailability)
          : reads.track(PlotService.ListVegRecords(p)).then(records => ({ records: records ?? [], reason: null })),
        ordinarySoilUnavailable
          ? { records: [] as HumusRecord[], reason: null as string | null }
          : reads.track(PlotService.ListHumusRecords(p)).then(
              records => validateOrdinarySoilRecordIds('Humus', records ?? []),
              cause => {
                const reason = historicalSoilReadError('Humus', cause);
                if (reason) return { records: [] as HumusRecord[], reason };
                throw cause;
              }
            ),
        ordinarySoilUnavailable
          ? { records: [] as MineralRecord[], reason: null as string | null }
          : reads.track(PlotService.ListMineralRecords(p)).then(
              records => validateOrdinarySoilRecordIds('Mineral', records ?? []),
              cause => {
                const reason = historicalSoilReadError('Mineral', cause);
                if (reason) return { records: [] as MineralRecord[], reason };
                throw cause;
              }
            ),
        reads.track(PlotService.ListOtherRecords(p)),
        reads.track(PlotService.ListAuditReadEntries(p)),
        reads.track(PlotService.GetChildCapabilities('Veg')),
        reads.track(PlotService.GetChildCapabilities('Humus')),
        reads.track(PlotService.GetChildCapabilities('Mineral')),
        reads.track(PlotService.GetChildCapabilities('Other'))
      ]);
      if (request !== loadRequest) return;
      if (vc === null || hc === null || mc === null || oc === null) throw new Error('Child capabilities were not returned.');
      const soilHistoryReason = [hOutcome.reason, mOutcome.reason].filter(Boolean).join(' ') || null;
      childCapabilities = { Veg: vc, Humus: hc, Mineral: mc, Other: oc };
      vegList = v.records;
      vegetationReadUnavailable = v.reason;
      historicalSoilUnavailable = soilHistoryReason;
      humusList = soilHistoryReason ? [] : hOutcome.records;
      mineralList = soilHistoryReason ? [] : mOutcome.records;
      otherList = o ?? [];
      auditList = a ?? [];
      childRevision++;
    } catch (e) {
      throw new Error(`Child records could not be loaded: ${String(e)}`);
    }
  }

  $effect(() => {
    const plot = plotNumber;
    untrack(() => void load(plot));
  });

  async function loadReferenceLists() {
    const request = ++referenceRequest;
    referenceReads.cancelAll();
    try {
      const [m, n, ms, ss] = await Promise.all([
        referenceReads.track(PlotEntryService.GetListItems('MoistureRegime')),
        referenceReads.track(PlotEntryService.GetListItems('NutrientRegime')),
        referenceReads.track(PlotEntryService.GetListItems('MesoSlopePosition')),
        referenceReads.track(PlotEntryService.GetListItems('SurfaceShape')),
      ]);
      if (request !== referenceRequest) return;
      moistureList = m ?? [];
      nutrientList = n ?? [];
      mesoSlopeList = ms ?? [];
      surfaceShapeList = ss ?? [];
    } catch (e) {
      if (request === referenceRequest) error = `Reference lists could not be loaded: ${String(e)}`;
    }
  }

  $effect(() => {
    void loadReferenceLists();
  });

  let totalTreeCover = $derived(
    vegetationReadUnavailable ? null : vegList.reduce((sum, r) => sum + (r.totalA || 0), 0)
  );
  let totalShrubCover = $derived(
    vegetationReadUnavailable ? null : vegList.reduce((sum, r) => sum + (r.totalB || 0), 0)
  );
  let totalHerbCover = $derived(
    vegetationReadUnavailable ? null : vegList.reduce((sum, r) => sum + (r.cover6 || 0), 0)
  );
  let totalMossCover = $derived(
    vegetationReadUnavailable ? null : vegList.reduce((sum, r) => sum + (r.cover7 || 0), 0)
  );

  async function searchSpecies() {
    const request = ++speciesRequest;
    speciesReads.cancelAll();
    if (!speciesSearchQuery.trim()) {
      speciesSearchResults = [];
      searchingSpecies = false;
      return;
    }
    searchingSpecies = true;
    try {
      const res = await speciesReads.track(PlotEntryService.SearchSpecies(speciesSearchQuery, 15));
      if (request !== speciesRequest) return;
      speciesSearchResults = res ?? [];
    } catch (e) {
      if (request === speciesRequest) error = `Species search failed: ${String(e)}`;
    } finally {
      if (request === speciesRequest) searchingSpecies = false;
    }
  }

  async function selectSpeciesToAdd(sp: SpeciesItem) {
    const newVeg: VegRecord = {
      ...newVegetationFields,
      id: 0,
      plotNumber: draft.plotNumber,
      species: sp.code,
      layer: 'A',
      cover1: 0,
      totalA: 0
    };
    if (await childOperation(() => PlotService.SaveVegRecord(newVeg))) {
      showSpeciesModal = false;
      speciesSearchQuery = '';
      speciesSearchResults = [];
    }
  }

  async function loadSpeciesReferences() {
    if ((!speciesEditingEnabled && !creationEnabled) || speciesReferenceBusy) return;
    const request = speciesReferenceRequest;
    speciesReferenceBusy = true;
    speciesReferenceReady = false;
    speciesReferenceError = null;
    try {
      const rows = await Promise.all(speciesForms.map(form => speciesReferenceReads.track(PlotService.ListVegetationSpecies(form))));
      if (request !== speciesReferenceRequest) return;
      const lists: SpeciesLists = {};
      speciesForms.forEach((form, index) => {
        const options = rows[index];
        if (options === null) throw new Error(`Species metadata for ${form} was not returned.`);
        lists[form] = options;
      });
      speciesLists = lists;
      let drafts = speciesDrafts;
      for (const [id, cell] of Object.entries(drafts)) {
        if (!cell.decision) drafts = stageSpecies(drafts, cell.form, Number(id), cell.raw, cell.expected, lists);
      }
      speciesDrafts = drafts;
      speciesReferenceReady = true;
    } catch (cause) {
      if (request === speciesReferenceRequest) speciesReferenceError = `Species references unavailable: ${String(cause)}`;
    } finally {
      if (request === speciesReferenceRequest) speciesReferenceBusy = false;
    }
  }

  $effect(() => { untrack(() => void loadSpeciesReferences()); });

  function stageSpeciesCell(form: string, id: number, raw: string) {
    try {
      if (speciesEditingDisabled) throw new Error('Species editing is unavailable while loading, locked, busy or another draft is unsaved.');
      if (childCapabilities.Veg.species !== true) throw new Error('Species is unavailable in the active schema.');
      const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1 || !sourceRows(form).some(row => row.id === id)) throw new Error('Species source row is missing or ambiguous; reload before editing.');
      speciesDrafts = stageSpecies(speciesDrafts, form, id, raw, rows[0].species, speciesLists);
      const choices = { ...speciesChoices };
      delete choices[String(id)];
      speciesChoices = choices;
      successMsg = null;
      if (error?.startsWith('Species ')) error = null;
    } catch (cause) {
      error = `Species draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function reviewSpeciesChoices(id: number) {
    if (speciesEditingDisabled) { error = 'Wait for available, unlocked species editing before reviewing choices.'; return; }
    const cell = speciesDrafts[String(id)];
    if (!cell || !cell.error || cell.raw === '' || cell.raw.length > 8) { error = 'Enter a nonempty species code of at most eight UTF-16 units before reviewing choices.'; return; }
    const request = ++speciesChoiceRequest;
    speciesDecisionBusy = true;
    error = null;
    try {
      const aliases = await speciesChoiceReads.track(PlotService.ListVegetationSpeciesAliases({ code: cell.raw }));
      if (request !== speciesChoiceRequest) return;
      if (aliases === null) throw new Error('Species alias metadata was not returned.');
      const users = aliases.some(option => option.code !== null) ? []
        : await speciesChoiceReads.track(PlotService.ListVegetationSpeciesUsers({ code: cell.raw }));
      if (request !== speciesChoiceRequest) return;
      const current = speciesDrafts[String(id)];
      if (!current || current.raw !== cell.raw || current.form !== cell.form || current.expected !== cell.expected) {
        throw new Error('The species editor entry changed during lookup; review it again.');
      }
      if (users === null) throw new Error('Species personal-code metadata was not returned.');
      speciesChoices = { ...speciesChoices, [String(id)]: { form: cell.form, entered: cell.raw, aliases, users } };
    } catch (cause) {
      if (request === speciesChoiceRequest) error = `Species choices unavailable; draft retained: ${String(cause)}`;
    } finally {
      if (request === speciesChoiceRequest) speciesDecisionBusy = false;
    }
  }

  function chooseSpeciesCode(id: number, kind: SpeciesDecisionKind, selected?: string) {
    try {
      if (speciesEditingDisabled) throw new Error('Wait for available, unlocked species editing before selecting a decision.');
      const choices = speciesChoices[String(id)];
      if (!choices) throw new Error('Review the current species entry before selecting a decision.');
      speciesDrafts = chooseSpecies(speciesDrafts, id, choices, kind, selected);
      const next = { ...speciesChoices };
      delete next[String(id)];
      speciesChoices = next;
      error = null;
      successMsg = null;
      childRevision++;
    } catch (cause) {
      error = `Species decision failed; draft retained: ${String(cause)}`;
    }
  }

  async function cancelSpeciesDrafts() {
    if (personalDraft !== null) { error = 'Cancel personal definition entry before cancelling plot species drafts.'; return; }
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling species drafts.'; return; }
    speciesDrafts = {};
    speciesChoices = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = savedPersonalCodes.length
        ? 'Plot species drafts cancelled. No plot data or project history changed; explicitly saved personal definitions remain available.'
        : 'Species drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Species drafts cancelled, but rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  function startPersonalSpecies(id: number) {
    try {
      if (!personalSpeciesEnabled || speciesEditingDisabled) throw new Error('Personal definition creation is unavailable while loading, locked or another workflow is active.');
      const cell = speciesDrafts[String(id)], choices = speciesChoices[String(id)];
      if (!cell || !choices) throw new Error('Review the current unknown code before creating its personal definition.');
      personalDraft = beginPersonalSpecies(id, cell, choices);
      error = null;
      successMsg = null;
    } catch (cause) {
      error = `Personal definition could not start: ${String(cause)}`;
    }
  }

  function cancelPersonalSpecies() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the personal definition operation before cancelling entry.'; return; }
    personalDraft = null;
    error = null;
    successMsg = 'Personal definition entry cancelled without writes. The original plot species draft or new-row proposal is retained.';
  }

  function personalSourceMatches(proposed: PersonalSpeciesDraft): boolean {
    return proposed.source.kind === 'existing'
      ? matchesPersonalSpeciesSource(proposed, speciesDrafts[String(proposed.source.id)])
      : proposed.source.kind === 'creation' && creationEnabled && matchesCreationPersonalSpeciesSource(proposed, creationDraft);
  }

  async function savePersonalSpecies() {
    const proposed = personalDraft;
    if (!personalSpeciesEnabled || !proposed || busy || headerWorkflowBusy || dirty || draft.locked || !capabilitiesReady ||
        !personalSourceMatches(proposed)) {
      error = 'An available unlocked personal definition and its unchanged source species draft are required.';
      return;
    }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const request = personalSpeciesRequest(proposed);
      const definition = await PlotService.CreatePersonalSpeciesDefinition(request);
      committed = true;
      personalDraft = null;
      savedPersonalCodes = [...new Set([...savedPersonalCodes, proposed.entered.toUpperCase()])];
      if (!personalSpeciesMatches(definition, request)) throw new Error('Saved personal definition response differs from planned metadata.');
      const [users, aliases] = await Promise.all([
        speciesChoiceReads.track(PlotService.ListVegetationSpeciesUsers({ code: proposed.entered })),
        speciesChoiceReads.track(PlotService.ListVegetationSpeciesAliases({ code: proposed.entered })),
      ]);
      if (users === null || aliases === null || users.filter(option => personalSpeciesMatches(option, request)).length !== 1) {
        throw new Error('Saved personal definition differs from independently reloaded metadata.');
      }
      if (!personalSourceMatches(proposed)) throw new Error('The original plot species draft or new-row proposal changed during personal definition save.');
      const reloaded = { form: proposed.form, entered: proposed.entered, users, aliases };
      if (proposed.source.kind === 'existing') {
        speciesDrafts = chooseSpecies(speciesDrafts, proposed.source.id, reloaded, 'user', definition.code ?? undefined);
        const choices = { ...speciesChoices };
        delete choices[String(proposed.source.id)];
        speciesChoices = choices;
      } else {
        if (!creationDraft) throw new Error('The original new-row proposal is unavailable after definition save.');
        creationDraft = chooseVegetationCreationSpecies(creationDraft, reloaded, 'user', definition.code ?? undefined);
        creationChoices = null;
      }
      childRevision++;
      successMsg = `Personal definition ${definition.code} saved in the user database. The plot assignment is still unsaved; ${proposed.source.kind === 'existing' ? 'Save species drafts' : 'Save new vegetation record'} commits it. Undo never removes this explicitly saved definition.`;
    } catch (cause) {
      committed = committed || personalSpeciesCommittedError(cause);
      if (committed) { personalDraft = null; capabilitiesReady = false; }
      error = committed ? `Personal definition committed, but cleanup, refresh or source verification failed. The plot was not saved; reopen it before editing: ${String(cause)}`
        : `Personal definition save failed; metadata and plot drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveSpeciesDrafts() {
    if (speciesEditingDisabled) { error = 'Save a clean, unlocked header with available species references before saving.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = speciesUpdates(speciesDrafts);
      if (updates.length === 0) { speciesDrafts = {}; successMsg = 'No species values changed.'; return; }
      await PlotService.UpdateVegetationSpecies(draft.plotNumber, updates);
      committed = true;
      speciesDrafts = {};
      speciesChoices = {};
      await loadChildData(draft.plotNumber);
      successMsg = [`Saved ${updates.length} species row drafts atomically.`, ...aCoverSourceNotices(updates, vegList)].join(' ');
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Species changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Species save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  function stageCollectedCell(id: number, form: string) {
    try {
      if (collectedEditingDisabled) throw new Error('Collected editing is unavailable while loading, locked, busy or another draft is unsaved.');
      if (childCapabilities.Veg.collected !== true) throw new Error('Collected is unavailable in the active schema.');
      const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Collected row identity is missing or ambiguous; reload before editing.');
      if (!sourceRows(form).some(row => row.id === id)) throw new Error('Collected source row is unavailable; reload before editing.');
      // VegRecord omits nil Collected; the empty string remains a distinct value.
      const stored = rows[0].collected ?? null;
      collectedDrafts = stageCollected(collectedDrafts, id, stored, form);
      successMsg = null;
      if (error?.startsWith('Collected ')) error = null;
    } catch (cause) {
      error = `Collected draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function cancelCollectedDrafts() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling Collected drafts.'; return; }
    collectedDrafts = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Collected drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Collected drafts cancelled, but current rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveCollectedDrafts() {
    if (collectedEditingDisabled) { error = 'Save a clean, unlocked header before saving Collected drafts.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = collectedUpdates(collectedDrafts);
      const sources = updates.map(({ id }) => ({ id, form: collectedDrafts[String(id)].form ?? '' }));
      if (updates.length === 0) { collectedDrafts = {}; successMsg = 'No Collected values changed.'; return; }
      await PlotService.UpdateCollectedRecords(draft.plotNumber, updates);
      committed = true;
      collectedDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = [`Saved ${updates.length} Collected row drafts atomically.`, ...aCoverSourceNotices(sources, vegList)].join(' ');
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Collected changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Collected save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  function startVegetationCreation(form: string) {
    if (!creationEnabled || childParentDisabled || childUnsaved || !speciesReferenceReady || speciesReferenceBusy) {
      error = 'Finish or cancel other drafts and load canonical species references before starting vegetation creation.';
      return;
    }
    const columns = [...paperChild(form).controls].sort((a, b) => a.tabOrder - b.tabOrder).flatMap(control => control.column ? [control.column] : []);
    try {
      creationDraft = beginVegetationCreation(form, columns);
      creationChoices = null;
      error = null;
      successMsg = null;
    } catch (cause) {
      error = `Vegetation creation could not start: ${String(cause)}`;
    }
  }

  function stageCreationCell(column: string, raw: string) {
    if (!creationDraft || personalDraft !== null || busy || headerWorkflowBusy) { error = 'An available creation draft without personal metadata entry is required before editing.'; return; }
    try {
      creationDraft = stageVegetationCreation(creationDraft, column, raw);
      error = null;
      successMsg = null;
    } catch (cause) {
      error = `Vegetation creation input rejected: ${String(cause)}`;
    }
  }

  function stageCreationSpecies(raw: string) {
    if (!creationDraft || personalDraft !== null || busy || headerWorkflowBusy) { error = 'An available creation draft without personal metadata entry is required before editing species.'; return; }
    creationDraft = stageVegetationCreationSpecies(creationDraft, raw);
    creationChoices = null;
    error = null;
    successMsg = null;
  }

  async function reviewCreationSpeciesChoices() {
    const proposed = creationDraft;
    if (!creationEnabled || !proposed || proposed.decision || personalDraft !== null || busy || headerWorkflowBusy || draft.locked ||
        !capabilitiesReady || !speciesReferenceReady || speciesReferenceBusy || !creationSpeciesInvalid) {
      error = 'An available unresolved creation species entry is required before reviewing choices.';
      return;
    }
    if (speciesEventError(proposed.species)) { error = speciesEventError(proposed.species); return; }
    const request = ++speciesChoiceRequest;
    speciesDecisionBusy = true;
    creationChoices = null;
    error = null;
    try {
      const aliases = await speciesChoiceReads.track(PlotService.ListVegetationSpeciesAliases({ code: proposed.species }));
      if (request !== speciesChoiceRequest) return;
      if (aliases === null) throw new Error('Species alias metadata was not returned.');
      const users = aliases.some(option => option.code !== null) ? []
        : await speciesChoiceReads.track(PlotService.ListVegetationSpeciesUsers({ code: proposed.species }));
      if (request !== speciesChoiceRequest) return;
      if (!creationDraft || creationDraft.form !== proposed.form || creationDraft.species !== proposed.species || creationDraft.decision) {
        throw new Error('The original creation species entry changed during lookup; review it again.');
      }
      if (users === null) throw new Error('Species personal-code metadata was not returned.');
      creationChoices = { form: proposed.form, entered: proposed.species, aliases, users };
    } catch (cause) {
      if (request === speciesChoiceRequest) error = `Creation species choices unavailable; draft retained: ${String(cause)}`;
    } finally {
      if (request === speciesChoiceRequest) speciesDecisionBusy = false;
    }
  }

  function chooseCreationSpecies(kind: SpeciesDecisionKind, selected?: string) {
    try {
      if (!creationEnabled || !creationDraft || !creationChoices || personalDraft !== null || busy || headerWorkflowBusy || draft.locked || !capabilitiesReady) {
        throw new Error('Review the available unlocked creation entry before selecting a decision.');
      }
      creationDraft = chooseVegetationCreationSpecies(creationDraft, creationChoices, kind, selected);
      creationChoices = null;
      error = null;
      successMsg = null;
    } catch (cause) {
      error = `Creation species decision failed; draft retained: ${String(cause)}`;
    }
  }

  function startCreationPersonalSpecies() {
    try {
      if (!creationEnabled || !personalSpeciesEnabled || !creationDraft || !creationChoices || !creationSpeciesInvalid ||
          personalDraft !== null || busy || headerWorkflowBusy || draft.locked || !capabilitiesReady || !speciesReferenceReady || speciesReferenceBusy) {
        throw new Error('Review the available unlocked unknown creation code before creating its personal definition.');
      }
      personalDraft = beginCreationPersonalSpecies(creationDraft, creationChoices);
      error = null;
      successMsg = null;
    } catch (cause) {
      error = `Creation personal definition could not start: ${String(cause)}`;
    }
  }

  async function cancelVegetationCreation() {
    if (personalDraft !== null) { error = 'Cancel personal definition entry before cancelling the new-row proposal.'; return; }
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling creation.'; return; }
    creationDraft = null;
    creationChoices = null;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = savedPersonalCodes.length
        ? 'Vegetation creation cancelled. No project data or history changed; explicitly saved personal definitions remain available.'
        : 'Vegetation creation cancelled. No stored data or history changed; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Creation cancelled, but rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveVegetationCreation() {
    const proposed = creationDraft;
    if (!creationEnabled || !proposed || personalDraft !== null || busy || headerWorkflowBusy || dirty || draft.locked || !capabilitiesReady || original === null || !speciesReferenceReady || speciesReferenceBusy) {
      error = 'An unlocked available creation draft and canonical references are required before saving.';
      return;
    }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const request = vegetationCreationRequest(proposed, speciesLists[proposed.form] ?? []);
      const id = await PlotService.CreateSourceVegetation(draft.plotNumber, request);
      committed = true;
      creationDraft = null;
      creationChoices = null;
      if (!Number.isInteger(id) || id <= 0 || id > 2147483647) throw new Error('Creation returned an invalid allocated identity.');
      await loadChildData(draft.plotNumber);
      const observed = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (observed.length !== 1 || observed[0].species !== request.species) throw new Error('Committed creation differs from independently reloaded identity/species.');
      const notice = aCoverSourceNotice(proposed.form, id, observed[0]);
      successMsg = [`Vegetation row ${id} created in ${proposed.form}. No Layer or other cover values were inferred.`, ...(notice ? [notice] : [])].join(' ');
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Vegetation creation committed, but refresh or identity verification failed. Reopen the plot before editing: ${String(cause)}`
        : `Vegetation creation failed; draft retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function reviewVegetationDeletion(form: string, id: number) {
    if (!deletionEnabled || childParentDisabled || childUnsaved) {
      error = 'Finish or cancel other drafts before reviewing vegetation deletion.';
      return;
    }
    const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
    if (rows.length !== 1 || !sourceRows(form).some(row => row.id === id)) {
      error = 'Vegetation deletion source row is missing or ambiguous; reload before reviewing.';
      return;
    }
    const species = rows[0].species;
    const plot = draft.plotNumber;
    const request = ++deletionRequest;
    deletionBusy = true;
    error = null;
    successMsg = null;
    try {
      const reviewed = await deletionReads.track(PlotService.ReviewVegetationDeletion(plot, form, id));
      if (request !== deletionRequest) return;
      if (draft.plotNumber !== plot) throw new Error('Vegetation deletion plot changed during lookup; reload before reviewing.');
      deletionReview = validateDeletionReview(reviewed, form, id, species);
    } catch (cause) {
      if (request === deletionRequest) error = `Vegetation deletion review failed; no data changed: ${String(cause)}`;
    } finally {
      if (request === deletionRequest) deletionBusy = false;
    }
  }

  async function cancelVegetationDeletion() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling deletion review.'; return; }
    deletionReview = null;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Vegetation deletion cancelled. No stored data or history changed; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Deletion review cancelled, but rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function confirmVegetationDeletion() {
    const reviewed = deletionReview;
    if (!deletionEnabled || !reviewed || busy || headerWorkflowBusy || dirty || draft.locked || !capabilitiesReady || original === null) {
      error = 'Review an available, unlocked vegetation row before explicitly confirming deletion.';
      return;
    }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      await PlotService.DeleteReviewedVegetation(draft.plotNumber, { id: reviewed.id, form: reviewed.form, expected: reviewed.expected });
      committed = true;
      deletionReview = null;
      await loadChildData(draft.plotNumber);
      successMsg = `Vegetation row ${reviewed.id} deleted from all views. Its identity remains reserved.`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Vegetation deletion committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Vegetation deletion failed; review retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  function stageHeightCell(id: number, column: string, raw: string, form: string) {
    try {
      if (!heightEditingEnabled || heightEditingDisabled) throw new Error('Height editing is unavailable while loading, locked, busy or the header is unsaved.');
      const field = heightField(column);
      if (!field || childCapabilities.Veg[field] !== true) throw new Error(`Height column ${column} is unavailable in the active schema.`);
      const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Height row identity is missing or ambiguous; reload before editing.');
      if (!sourceRows(form).some(row => row.id === id)) throw new Error('Vegetation source row is unavailable; reload before editing.');
      heightDrafts = stageHeight(heightDrafts, id, field, raw, rows[0][field] ?? null, form);
      successMsg = null;
    } catch (cause) {
      error = `Height draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function cancelHeightDrafts() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling height drafts.'; return; }
    heightDrafts = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Height drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Height drafts cancelled, but current rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveHeightDrafts() {
    if (!heightEditingEnabled || heightEditingDisabled) { error = 'Save a clean, unlocked header before saving height drafts.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = heightUpdates(heightDrafts);
      const notices = heightSourceNotices(heightDrafts, vegList);
      if (updates.length === 0) { heightDrafts = {}; successMsg = 'No height values changed.'; return; }
      await PlotService.UpdateVegetationNumbers(draft.plotNumber, vegetationNumberUpdates(heightDrafts));
      committed = true;
      heightDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = `Saved ${updates.length} height/cover row drafts atomically.${notices.length ? ' ' + notices.join(' ') : ''}`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Height changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Height save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  function stageOtherCell(id: number, column: string, raw: OtherValue) {
    try {
      if (otherEditingDisabled) throw new Error('Other editing is unavailable while loading, locked, busy or another draft is unsaved.');
      const field = otherField(column);
      if (!field || childCapabilities.Other[field.key] !== true) throw new Error(`Other column ${column} is unavailable in the active schema.`);
      const rows = otherList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Other row identity is missing or ambiguous; reload before editing.');
      otherDrafts = stageOther(otherDrafts, id, field, raw, rows[0][field.key] ?? null);
      successMsg = null;
      if (error?.startsWith('Other ')) error = null;
    } catch (cause) {
      error = `Other draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function cancelOtherDrafts() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling Other drafts.'; return; }
    otherDrafts = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Other drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Other drafts cancelled, but current rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveOtherDrafts() {
    if (otherEditingDisabled) { error = 'Save a clean, unlocked header before saving Other drafts.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = otherUpdates(otherDrafts);
      if (updates.length === 0) { otherDrafts = {}; successMsg = 'No Other values changed.'; return; }
      await PlotService.UpdateOtherRecords(draft.plotNumber, updates);
      committed = true;
      otherDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = `Saved ${updates.length} Other row drafts atomically.`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Other changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Other save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function loadSoilSuggestions() {
    if (ordinarySoilUnavailable || !soilEditingEnabled || soilReferenceBusy) return;
    const request = referenceRequest;
    soilReferenceBusy = true;
    soilReferenceReady = false;
    soilReferenceError = null;
    try {
      const rows = await referenceReads.track(PlotService.ListSoilSuggestions());
      if (request !== referenceRequest) return;
      if (rows === null) throw new Error('Soil reference metadata was not returned.');
      soilSuggestions = rows;
      soilReferenceReady = true;
    } catch (cause) {
      if (request === referenceRequest) soilReferenceError = `Soil reference suggestions unavailable: ${String(cause)}`;
    } finally {
      if (request === referenceRequest) soilReferenceBusy = false;
    }
  }

  $effect(() => { untrack(() => void loadSoilSuggestions()); });

  function stageSoilCell(kind: SoilKind, id: number, column: string, raw: string) {
    try {
      if (soilEditingDisabled) throw new Error('Soil editing is unavailable while loading, locked, busy or another draft is unsaved.');
      const field = soilField(kind, column);
      if (!field || childCapabilities[kind][field.key] !== true) throw new Error(`Soil column ${column} is unavailable in the active schema.`);
      const rows = (kind === 'Humus' ? humusList : mineralList).filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Soil row identity is missing or ambiguous; reload before editing.');
      const stored = new Map(Object.entries(rows[0])).get(field.key);
      if (stored !== null && stored !== undefined && typeof stored !== 'string' && typeof stored !== 'number') throw new Error('Soil cell has an unsupported storage type.');
      soilDrafts = stageSoil(soilDrafts, kind, id, field, raw, stored ?? null);
      successMsg = null;
      if (error?.startsWith('Soil ')) error = null;
    } catch (cause) {
      error = `Soil draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function cancelSoilDrafts() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling soil drafts.'; return; }
    soilDrafts = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Soil drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Soil drafts cancelled, but current rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveSoilDrafts() {
    if (soilEditingDisabled) { error = 'Save a clean, unlocked header with available references before saving soil drafts.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = soilUpdates(soilDrafts);
      if (updates.length === 0) { soilDrafts = {}; successMsg = 'No soil values changed.'; return; }
      await PlotService.UpdateSoilRecords(draft.plotNumber, updates);
      committed = true;
      soilDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = `Saved ${updates.length} Humus/Mineral row drafts atomically.`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Soil changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Soil save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function loadAttributeSuggestions() {
    if (!attributeEditingEnabled || attributeReferenceBusy) return;
    const request = referenceRequest;
    attributeReferenceBusy = true;
    attributeReferenceReady = false;
    attributeReferenceError = null;
    try {
      const rows = await referenceReads.track(PlotService.ListVegetationAttributeSuggestions());
      if (request !== referenceRequest) return;
      if (rows === null) throw new Error('Vegetation attribute reference metadata was not returned.');
      attributeSuggestions = rows;
      attributeReferenceReady = true;
    } catch (cause) {
      if (request === referenceRequest) attributeReferenceError = `Vegetation attribute references unavailable: ${String(cause)}`;
    } finally {
      if (request === referenceRequest) attributeReferenceBusy = false;
    }
  }

  $effect(() => { untrack(() => void loadAttributeSuggestions()); });

  function stageAttributeCell(id: number, column: string, raw: string) {
    try {
      if (attributeEditingDisabled) throw new Error('Attribute editing is unavailable while loading, locked, busy or another draft is unsaved.');
      const field = vegetationAttributeField(column);
      if (!field || childCapabilities.Veg[field.key] !== true) throw new Error(`Attribute column ${column} is unavailable in the active schema.`);
      const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Vegetation row identity is missing or ambiguous; reload before editing.');
      const stored = new Map(Object.entries(rows[0])).get(field.key);
      if (stored !== null && stored !== undefined && typeof stored !== 'number') throw new Error('Attribute cell has an unsupported storage type.');
      attributeDrafts = stageVegetationAttribute(attributeDrafts, id, field, raw, stored ?? null);
      successMsg = null;
      if (error?.startsWith('Vegetation attribute ')) error = null;
    } catch (cause) {
      error = `Vegetation attribute draft failed: ${String(cause)}`;
      childRevision++;
    }
  }

  async function cancelAttributeDrafts() {
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation before cancelling attribute drafts.'; return; }
    attributeDrafts = {};
    childRevision++;
    error = null;
    successMsg = null;
    busy = true;
    try {
      await loadChildData(draft.plotNumber);
      successMsg = 'Vegetation attribute drafts cancelled. Stored data and history were unchanged; current rows reloaded.';
    } catch (cause) {
      capabilitiesReady = false;
      error = `Vegetation attribute drafts cancelled, but rows could not be reloaded. Reopen the plot: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function saveAttributeDrafts() {
    if (attributeEditingDisabled) { error = 'Save a clean, unlocked header with available references before saving attribute drafts.'; return; }
    let committed = false;
    busy = true;
    error = null;
    successMsg = null;
    try {
      const updates = vegetationAttributeUpdates(attributeDrafts);
      if (updates.length === 0) { attributeDrafts = {}; successMsg = 'No vegetation attribute values changed.'; return; }
      await PlotService.UpdateVegetationAttributes(draft.plotNumber, updates);
      committed = true;
      attributeDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = `Saved ${updates.length} vegetation attribute row drafts atomically.`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Vegetation attributes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Vegetation attribute save failed; drafts retained: ${String(cause)}`;
    } finally {
      busy = false;
    }
  }

  async function editSourceChild(name: string, id: number, column: string, raw: string) {
    if (ordinaryVegetationUnavailable && name.startsWith('SubVeg')) { error = ordinaryVegetationUnavailable; return; }
    const soilUnavailable = ordinarySoilUnavailable ?? (typeof historicalSoilUnavailable !== 'undefined' ? historicalSoilUnavailable : null);
    if (soilUnavailable && (name === 'SoilHumusXL' || name === 'SoilMineralXL')) { error = soilUnavailable; return; }
    try {
      if (name.startsWith('SubVeg') && column.toLowerCase() === 'species') {
        throw new Error('Species requires the source-list draft workflow; unrestricted source-grid edits are unavailable.');
      }
      const write = buildDirectSourceChildWrite(name, id, column, raw, { vegList, humusList, mineralList, otherList }, PlotService);
      await childOperation(write);
    } catch (cause) { error = `Child input failed: ${String(cause)}`; }
  }

  function markDirty() {
    dirty = true;
    successMsg = null;
  }

  const invalidHeaderMessage = 'Correct the invalid header input before saving.';
  function headerValidationError(message: string) {
    if (message === headerValidation['ordinary-veg']) return `Correct the invalid header input on Vegetation or Undo changes before saving. ${message}`;
    if (message === headerValidation['ordinary-soils']) return `Correct the invalid header input on Soils or Undo changes before saving. ${message}`;
    if (message === headerValidation['parentCodes-soils']) return `Correct the invalid header input on Soils or Undo changes before saving. ${message}`;
    if (message === headerValidation.soilDrainage) return `Correct the invalid header input on Soils or Undo changes before saving. ${message}`;
    if (message === headerValidation.soilCodes || message === headerValidation.geologyCodes) return `Correct the invalid header input on Soils or Undo changes before saving. ${message}`;
    return `Correct the invalid header input on Site or Undo changes before saving. ${message}`;
  }

  function validateHeader(key: string, message: string | null) {
    const next = { ...headerValidation };
    const previous = next[key];
    if (previous !== undefined && (error === previous || error === headerValidationError(previous) || error === invalidHeaderMessage)) error = null;
    if (message === null) {
      delete next[key];
    }
    else next[key] = message;
    headerValidation = next;
  }

  async function save() {
    if (siviSourceAuthorityUnknown) {
      error = 'Read owned ProjectID choices to observe the source preference outcome before saving.';
      return;
    }
    if (busy || headerWorkflowBusy) {
      error = 'Wait for the current operation to finish before saving.';
      return;
    }
    if (siviDeletionPending || siviRestorationPending || siviCreationUndoPending) {
      error = 'Confirm deletion/restoration/creation Undo explicitly in its reviewed panel or discard the unsubmitted review; ordinary Save never performs it.';
      return;
    }
    if (codeCheckOpen) { error = 'Save replacements or personal metadata explicitly, then close species-code review; ordinary plot Save never applies them.'; return; }
    if (metadataOpen) { error = 'Save or Undo metadata explicitly, then close metadata review; ordinary plot Save never applies it.'; return; }
    if (profileReviewBlocked) { error = 'Save or Undo profile drafts explicitly; ordinary plot Save never writes profile rules.'; return; }
    if (personalDraft !== null) { error = 'Save the personal definition explicitly or Cancel its entry; ordinary plot Save never writes the user database.'; return; }
    if (deletionReview !== null) { error = 'Confirm reviewed vegetation deletion explicitly or Cancel deletion review; ordinary Save never deletes rows.'; return; }
    if (creationDraft !== null) { await saveVegetationCreation(); return; }
    if (siviCreationPending) { await siviCreationOperation('save'); return; }
    if (siviParentActionUnsaved) { await siviParentActionOperation('save'); return; }
    if (siviProjectAssignmentUnsaved) { await siviProjectAssignmentOperation('save'); return; }
    if (siviParentWriteUnsaved) { await siviParentWriteOperation('save'); return; }
    if (siviParentSharedUnsaved) { await siviParentSharedOperation('save'); return; }
    if (siviUnsaved) { await siviOperation('save'); return; }
    if (siviCoverUnsaved) { await siviCoverOperation('save'); return; }
    if (siviCombinedUnsaved) { await siviCombinedOperation('save'); return; }
    if (siviCollectedUnsaved) { await siviCollectedOperation('save'); return; }
    if (siviSpeciesUnsaved) { await siviSpeciesOperation('save'); return; }
    if (siviIdentityUnsaved) { await siviIdentityOperation('save'); return; }
    if (heightUnsaved) { await saveHeightDrafts(); return; }
    if (otherUnsaved) { await saveOtherDrafts(); return; }
    if (soilUnsaved) { await saveSoilDrafts(); return; }
    if (attributeUnsaved) { await saveAttributeDrafts(); return; }
    if (collectedUnsaved) { await saveCollectedDrafts(); return; }
    if (speciesUnsaved) { await saveSpeciesDrafts(); return; }
    const invalid = container.querySelector<HTMLInputElement>('.header-editor input:invalid');
    if (invalid) {
      invalid.reportValidity();
      error = invalidHeaderMessage;
      return;
    }
    const validation = Object.values(headerValidation)[0];
    if (validation) {
      error = headerValidationError(validation);
      return;
    }
    if (!capabilitiesReady || draft.locked) {
      error = 'The header is not available for saving.';
      return;
    }
    if (!draft.plotNumber.trim()) {
      error = 'Plot Number is required.';
      return;
    }
    busy = true;
    error = null;
    successMsg = null;
    try {
      const payload = $state.snapshot({ ...draft, locked: capabilities.locked === true && draft.locked });
      if (original) await PlotService.UpdatePlot(payload);
      else await PlotService.CreatePlot(payload);
      original = JSON.parse(JSON.stringify(draft));
      activeLayerDepthRaw = null;
      workingUnitSession.plot = draft.plotNumber;
      dirty = false;
      headerValidation = {};
      successMsg = `Plot ${draft.plotNumber} saved successfully.`;
      await loadChildData(draft.plotNumber);
      if (siviParentSession?.view().original || siviParentSession?.view().error) {
        await refreshSIVIParent(draft.plotNumber, [siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession]);
      }
      if (onSaved) await onSaved(draft.plotNumber);
    } catch (e) {
      error = String(e);
    } finally {
      busy = false;
    }
  }

  export function getCloseState(): EditorCloseState {
    if (twoPageVegetationBusy || twoPageVegetationBlocked) return {
      unsaved: true, busy: twoPageVegetationBusy, blocked: true, canSave: false, error,
      saveReason: 'Finish retained linked-vegetation drafts/operations, recover unknown outcomes and acknowledge receipts before parent Save, Lock or close.',
    };
    if (twoPageSoilBusy || twoPageSoilBlocked) return {
      unsaved: true, busy: twoPageSoilBusy, blocked: true, canSave: false, error,
      saveReason: 'Finish retained linked-soil drafts/operations, recover unknown outcomes and acknowledge receipts before parent Save, Lock or close.',
    };
    if (pictureMetadataPending && pictureMetadataClose) return {
      ...pictureMetadataClose, canSave: false,
      saveReason: pictureMetadataClose.blocked || pictureMetadataClose.busy ? pictureMetadataClose.saveReason
        : 'Save picture metadata explicitly or discard its draft before parent Save, Lock or close.',
    };
    if (siviParentSourceView?.authorityUnknown) return {
      unsaved: true, busy: siviParentSourceView.busy, blocked: true, canSave: false,
      saveReason: 'Reload owned ProjectID choices to observe the source preference outcome before saving or closing.',
      error: siviParentSourceView.error ?? 'ProjectID source preference outcome is unknown.',
    };
    const lifecycle = siviCreationUndoPending && siviCreationUndoClose?.blocked ? siviCreationUndoClose
      : siviDeletionPending ? siviDeletionClose : siviRestorationPending ? siviRestorationClose
      : siviCreationUndoPending ? siviCreationUndoClose : null;
    if (lifecycle) return { ...lifecycle, canSave: false, busy: busy || headerWorkflowBusy || lifecycle.busy,
      saveReason: lifecycle.blocked || lifecycle.busy ? lifecycle.saveReason
        : 'Finish source deletion/restoration/creation Undo explicitly or discard its unsubmitted review before parent Save, Lock or close.' };
    if (twoPageOpen) return twoPageEditor?.getCloseState() ?? {
      unsaved: true, busy: true, blocked: true, canSave: false, error,
      saveReason: 'Wait for two-page field review, then finish and close it explicitly.',
    };
    if (environmentSUOpen) return environmentSUEditor?.getCloseState() ?? {
      unsaved: true, busy: true, canSave: false, error,
      saveReason: 'Wait for SU transfer review, then finish or dismiss and close it explicitly.',
    };
    if (profileReviewBlocked) return profileEditor?.getCloseState() ?? {
      unsaved: true, busy: true, canSave: false, error,
      saveReason: 'Finish profile rules explicitly before saving or closing the plot.',
    };
    if (metadataOpen) return metadataEditor?.getCloseState() ?? {
      unsaved: true, busy: true, canSave: false, error,
      saveReason: 'Wait for metadata review, then finish or close it before saving or closing the plot.',
    };
    if (codeCheckOpen) return codeCheckEditor?.getCloseState() ?? {
      unsaved: true, busy: true, canSave: false, error,
      saveReason: 'Wait for species-code review to load, then finish or close it before saving or closing the plot.',
    };
    if (siviCreationPending && siviCreationClose) return {
      ...siviCreationClose, busy: busy || headerWorkflowBusy || siviCreationClose.busy,
      canSave: !siviCreationEditingDisabled && siviCreationClose.canSave,
      saveReason: siviCreationEditingDisabled ? 'Finish conflicting work before creating the retained SIVI row.' : siviCreationClose.saveReason,
      error: siviCreationClose.error ?? error,
    };
    const invalid = Object.keys(headerValidation).length > 0 || heightInvalid.length > 0 || otherInvalid.length > 0 || soilInvalid.length > 0 || attributeInvalid.length > 0 || speciesInvalid.length > 0 || deletionReview !== null || creationInvalid.length > 0;
    const invalidChild = Boolean(container?.querySelector('.source-child input:invalid'));
    const saveReason = busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
      : draft.locked ? 'Unlock the plot before saving.'
      : siviParentActionUnsaved && !siviParentActionClose?.canSave ? siviParentActionClose?.saveReason ?? 'Correct or Undo unresolved SIVI source actions.'
      : siviParentWriteUnsaved && !siviParentWriteClose?.canSave ? siviParentWriteClose?.saveReason ?? 'Correct or Undo unresolved SIVI parent drafts.'
      : siviParentSharedUnsaved && !siviParentSharedClose?.canSave ? siviParentSharedClose?.saveReason ?? 'Correct or Undo unresolved shared SIVI drafts.'
      : siviProjectAssignmentUnsaved && !siviProjectAssignmentClose?.canSave ? siviProjectAssignmentClose?.saveReason ?? 'Correct or Undo unresolved ProjectID selections.'
      : siviUnsaved && !siviClose?.canSave ? siviClose?.saveReason ?? 'Correct or Undo unresolved SIVI heights.'
      : siviCoverUnsaved && !siviCoverClose?.canSave ? siviCoverClose?.saveReason ?? 'Correct or Undo unresolved SIVI covers.'
      : siviCombinedUnsaved && !siviCombinedClose?.canSave ? siviCombinedClose?.saveReason ?? 'Correct or Undo unresolved combined SIVI drafts.'
      : siviCollectedUnsaved && !siviCollectedClose?.canSave ? siviCollectedClose?.saveReason ?? 'Correct or Undo unresolved SIVI Collected cycles.'
      : siviSpeciesUnsaved && !siviSpeciesClose?.canSave ? siviSpeciesClose?.saveReason ?? 'Correct or Undo unresolved SIVI Species drafts.'
      : siviIdentityUnsaved && !siviIdentityClose?.canSave ? siviIdentityClose?.saveReason ?? 'Correct or Undo unresolved SIVI ID drafts.'
      : personalDraft !== null ? 'Save the personal definition explicitly or Cancel its entry before saving the plot or closing.'
      : deletionReview !== null ? 'Confirm reviewed vegetation deletion explicitly or Cancel deletion review before saving and closing.'
      : creationInvalid.length > 0 ? 'Correct invalid vegetation creation or Cancel creation before saving.'
      : creationDraft !== null && (!speciesReferenceReady || speciesReferenceBusy) ? 'Restore canonical species references before saving vegetation creation.'
      : newChild !== null || invalidChild ? 'Finish or cancel child entry before saving and closing.'
      : otherInvalid.length > 0 ? 'Correct invalid Other drafts or Cancel Other drafts before saving.'
      : soilInvalid.length > 0 ? 'Correct invalid soil drafts or Cancel soil drafts before saving.'
      : soilUnsaved && (!soilReferenceReady || soilReferenceBusy) ? 'Restore available soil references before saving drafts.'
      : attributeInvalid.length > 0 ? 'Correct invalid vegetation attributes or Cancel attribute drafts before saving.'
      : attributeUnsaved && (!attributeReferenceReady || attributeReferenceBusy) ? 'Restore available attribute references before saving drafts.'
      : speciesInvalid.length > 0 ? 'Correct invalid species drafts or Cancel species drafts before saving.'
      : speciesUnsaved && (!speciesReferenceReady || speciesReferenceBusy) ? 'Restore available species references before saving drafts.'
      : invalid ? 'Correct invalid header inputs or Undo before saving.'
      : !draft.plotNumber.trim() ? 'Plot Number is required before saving.' : '';
    return { unsaved: dirty || childUnsaved || invalid || newChild !== null || invalidChild, busy: busy || headerWorkflowBusy,
      blocked: (siviClose?.blocked ?? false) || (siviCoverClose?.blocked ?? false) || (siviCombinedClose?.blocked ?? false) || (siviCollectedClose?.blocked ?? false) || (siviSpeciesClose?.blocked ?? false) || (siviIdentityClose?.blocked ?? false) || (siviParentWriteClose?.blocked ?? false) || (siviParentActionClose?.blocked ?? false) || (siviProjectAssignmentClose?.blocked ?? false) || (siviParentSharedClose?.blocked ?? false), canSave: saveReason === '', saveReason, error };
  }

  export async function saveForClose(): Promise<boolean> {
    const state = getCloseState();
    if (!state.canSave) {
      error = state.saveReason;
      return false;
    }
    await save();
    return !dirty && !childUnsaved && Object.keys(headerValidation).length === 0 && error === null;
  }

  function undo() {
    if (twoPageOpen) { twoPageEditor?.undo(); return; }
    if (environmentSUOpen) { environmentSUEditor?.undo(); return; }
    if (busy || headerWorkflowBusy) {
      error = 'Wait for the current operation to finish before undoing changes.';
      return;
    }
    if (codeCheckOpen) { codeCheckEditor?.undo(); return; }
    if (metadataOpen) { metadataEditor?.undo(); return; }
    if (profileReviewBlocked) { profileEditor?.undo(); return; }
    if (personalDraft !== null) { cancelPersonalSpecies(); return; }
    if (deletionReview !== null) { void cancelVegetationDeletion(); return; }
    if (creationDraft !== null) { void cancelVegetationCreation(); return; }
    if (siviDeletionPending) { void siviLifecycleOperation('deletion', 'undo'); return; }
    if (siviRestorationPending) { void siviLifecycleOperation('restoration', 'undo'); return; }
    if (siviCreationUndoPending) { void siviCreationUndoOperation('discard'); return; }
    if (siviCreationPending) { void siviCreationOperation('undo'); return; }
    if (siviParentActionUnsaved) { void siviParentActionOperation('undo'); return; }
    if (siviProjectAssignmentUnsaved) { void siviProjectAssignmentOperation('undo'); return; }
    if (siviParentWriteUnsaved) { void siviParentWriteOperation('undo'); return; }
    if (siviParentSharedUnsaved) { void siviParentSharedOperation('undo'); return; }
    if (siviUnsaved) { void siviOperation('undo'); return; }
    if (siviCoverUnsaved) { void siviCoverOperation('undo'); return; }
    if (siviCombinedUnsaved) { void siviCombinedOperation('undo'); return; }
    if (siviCollectedUnsaved) { void siviCollectedOperation('undo'); return; }
    if (siviSpeciesUnsaved) { void siviSpeciesOperation('undo'); return; }
    if (siviIdentityUnsaved) { void siviIdentityOperation('undo'); return; }
    if (heightUnsaved) { void cancelHeightDrafts(); return; }
    if (otherUnsaved) { void cancelOtherDrafts(); return; }
    if (soilUnsaved) { void cancelSoilDrafts(); return; }
    if (attributeUnsaved) { void cancelAttributeDrafts(); return; }
    if (collectedUnsaved) { void cancelCollectedDrafts(); return; }
    if (speciesUnsaved) { void cancelSpeciesDrafts(); return; }
    if (original) {
      draft = JSON.parse(JSON.stringify(original));
    } else {
      draft = newDraft();
    }
    activeLayerDepthRaw = null;
    dirty = false;
    headerValidation = {};
    error = null;
    headerRevision++;
    successMsg = 'Changes undone.';
  }

  async function toggleLock() {
    if (siviDeletionPending || siviRestorationPending || siviCreationUndoPending) { error = 'Finish or resolve source deletion/restoration/creation Undo before changing the plot lock.'; return; }
    if (siviSourceAuthorityUnknown) { error = 'Read owned ProjectID choices to observe the source preference outcome before changing the plot lock.'; return; }
    if (busy || headerWorkflowBusy) { error = 'Wait for the current operation to finish before changing the plot lock.'; return; }
    if (codeCheckOpen) { error = 'Finish or close species-code review before changing the plot lock.'; return; }
    if (metadataOpen) { error = 'Finish or close metadata review before changing the plot lock.'; return; }
    if (profileReviewBlocked) { error = 'Finish profile drafts before changing the plot lock.'; return; }
    if (personalDraft !== null) { error = 'Save the personal definition explicitly or Cancel its entry before changing the plot lock.'; return; }
    if (creationDraft !== null) { error = 'Save or Cancel vegetation creation before changing the plot lock.'; return; }
    if (deletionReview !== null) { error = 'Confirm or Cancel vegetation deletion review before changing the plot lock.'; return; }
    if (heightUnsaved) {
      error = 'Save or Cancel height drafts before changing the plot lock.';
      return;
    }
    if (siviParentWriteUnsaved) { error = 'Save or explicitly Undo SIVI parent drafts/recovery before changing the plot lock.'; return; }
    if (siviParentSharedUnsaved) { error = 'Save or explicitly Undo shared SIVI drafts/recovery before changing the plot lock.'; return; }
    if (siviParentActionUnsaved) { error = 'Save or explicitly Undo SIVI source-action drafts/recovery before changing the plot lock.'; return; }
    if (siviProjectAssignmentUnsaved) { error = 'Save or explicitly Undo ProjectID selection/recovery before changing the plot lock.'; return; }
    if (siviUnsaved) { error = 'SIVI heights or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviCoverUnsaved) { error = 'SIVI covers or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviCombinedUnsaved) { error = 'Combined SIVI drafts or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviCollectedUnsaved) { error = 'SIVI Collected or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviSpeciesUnsaved) { error = 'SIVI Species or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviIdentityUnsaved) { error = 'SIVI IDs or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.'; return; }
    if (siviCreationPending) { error = 'SIVI creation drafts or unknown acknowledgements must be resolved before changing the plot lock.'; return; }
    if (otherUnsaved) { error = 'Save or Cancel Other drafts before changing the plot lock.'; return; }
    if (soilUnsaved) { error = 'Save or Cancel soil drafts before changing the plot lock.'; return; }
    if (attributeUnsaved) { error = 'Save or Cancel vegetation attribute drafts before changing the plot lock.'; return; }
    if (collectedUnsaved) { error = 'Save or Cancel Collected drafts before changing the plot lock.'; return; }
    if (speciesUnsaved) { error = 'Save or Cancel species drafts before changing the plot lock.'; return; }
    if (!draft.locked && dirty) { await save(); if (dirty) return; }
    draft.locked = !draft.locked;
  }

  async function childOperation(operation: () => Promise<void>) {
    if (!capabilitiesReady || busy || headerWorkflowBusy || draft.locked) {
      error = 'Child editing is unavailable while the plot is loading, busy or locked.';
      return false;
    }
    if (!original || !draft.plotNumber || dirty || childUnsaved) {
      error = 'Save the plot header before editing child records.';
      return false;
    }
    if (draft.locked || busy) {
      error = 'Child records cannot be changed while locked or loading.';
      return false;
    }
    busy = true;
    error = null;
    successMsg = null;
    let committed = false;
    try {
      await operation();
      committed = true;
      await loadChildData(draft.plotNumber);
      return true;
    } catch (cause) {
      if (committed) {
        capabilitiesReady = false;
        error = `Child changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`;
        return true;
      }
      error = `Child operation failed: ${String(cause)}`;
      try {
        await loadChildData(draft.plotNumber);
      } catch (refreshError) {
        error += `; refresh failed: ${String(refreshError)}`;
      }
      return false;
    } finally {
      busy = false;
    }
  }

  async function deleteVeg(id: number) {
    await deleteSourceChild('SubVegAXL_BC', id);
  }

  async function deleteHumus(id: number) {
    await deleteSourceChild('SoilHumusXL', id);
  }

</script>

<svelte:window onkeydown={sourceEnter} />

<div bind:this={container} data-help-id={siviStandalone ? 'sivi.plot_type' : 'site.plot_number'} class="fs882-container bg-white rounded-lg shadow border border-stone-200 flex flex-col h-full text-stone-800 text-sm">
  <!-- Header Bar -->
  <header class="fs882-toolbar p-3 bg-stone-100 border-b border-stone-200 flex justify-between items-center">
    <div class="flex items-center gap-3">
      <h2 class="font-bold text-stone-900 text-base">{siviStandalone ? 'SIVI / FS1333: Site Visit' : 'FS882-6x4XL: Ecosystem Field Form'}</h2>
      <span class="px-2 py-0.5 rounded text-xs font-semibold {dirty || childUnsaved ? 'bg-amber-100 text-amber-800' : 'bg-stone-200 text-stone-600'}">
        {dirty || childUnsaved ? 'Draft Unsaved' : 'Clean'}
      </span>
      {#if draft.locked}
        <span class="px-2 py-0.5 rounded text-xs font-semibold bg-red-100 text-red-700 flex items-center gap-1">
          <Lock size={12} /> Locked (Read-Only)
        </span>
      {/if}
    </div>

    <div class="fs882-actions flex items-center gap-2">
      {#if twoPageReviewEnabled && !siviStandalone}
        <button type="button" data-two-page-review class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
          onclick={openTwoPageReview}>Two-page additional fields</button>
      {/if}
      {#if twoPageCommonReviewEnabled && !siviStandalone}
        <button type="button" data-two-page-common-review class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
          onclick={openTwoPageCommonReview}>Two-page common fields</button>
      {/if}
      {#if twoPageEntryReviewEnabled && !siviStandalone}
        <button type="button" data-two-page-entry-review class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
          onclick={openTwoPageEntryReview}>Complete two-page entry</button>
      {/if}
      {#if environmentSUEnabled && !siviStandalone}
        <button type="button" data-source-control="btnEnvIntoSu" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0 || $projectState?.activeSU === 'None'}
          onclick={() => openEnvironmentSU()}>Env Into SU</button>
      {/if}
      {#if siteUnitEnvironmentEnabled && !siviStandalone && !(externalSUEnvironmentEnabled && separateSUSelected)}
        <button type="button" data-source-control="btnSuIntoEnv" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0 || $projectState?.activeSU === 'None'}
          onclick={() => openEnvironmentSU('reverse')}>SU Into Env</button>
      {/if}
      {#if externalSUEnvironmentEnabled && separateSUSelected && !siviStandalone && !twoPageOpen}
        <button type="button" data-source-control="btnSuIntoEnv" data-external-su-environment-open class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
          onclick={openExternalSUEnvironment}>SU Into Env: separate file</button>
      {/if}
      {#if onFindPlot}
        <button type="button" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={busy || headerWorkflowBusy || !capabilitiesReady} onclick={onFindPlot}>Find Plot</button>
      {/if}
      {#if profileReviewEnabled}
        <button type="button" data-project-profile-review-open class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={busy || headerWorkflowBusy || siviSourceBarrier || profileReviewOpen || !capabilitiesReady ||
            (profileEditingEnabled && $projectState?.plotProfile?.writable === true && (metadataOpen || codeCheckOpen || personalDraft !== null || deletionReview !== null || creationDraft !== null))}
          onclick={() => {
            if (siviSourceBarrier) { error = 'Observe the ProjectID source preference outcome before opening profile rules.'; return; }
            profileReviewOpen = true;
            profileReviewBusy = true;
          }}>
          {profileEditingEnabled && $projectState?.plotProfile?.writable === true ? 'Review project profile rules' : 'Review project profile (read-only)'}
        </button>
      {/if}
      <button type="button" onclick={toggleLock}
        disabled={busy || headerWorkflowBusy || !capabilitiesReady || siviStandalone &&
          (!siviStandaloneEnabled || !siviParentView?.original || Boolean(siviParentView?.error) || childUnsaved || siviSourceBarrier)}
        class="px-3 py-1 rounded text-xs font-medium border flex items-center gap-1 {draft.locked ? 'bg-stone-200 hover:bg-stone-300 border-stone-300' : 'bg-stone-100 hover:bg-stone-200 border-stone-300'}"
        title={draft.locked ? 'Unlock Form' : 'Lock Form'}>
        {#if draft.locked}<Unlock size={14} /> Unlock{:else}<Lock size={14} /> Lock{/if}
      </button>
      <button type="button" onclick={undo}
        disabled={(!dirty && !childUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady}
        class="px-3 py-1 rounded text-xs font-medium border border-stone-300 bg-white hover:bg-stone-50 disabled:opacity-50 flex items-center gap-1">
        <RotateCcw size={14} /> Undo
      </button>
      <button type="button" onclick={save}
        disabled={(!dirty && !childUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady ||
          siviStandalone && !getCloseState().canSave}
        class="px-3 py-1 rounded text-xs font-medium bg-emerald-700 hover:bg-emerald-800 text-white disabled:opacity-50 flex items-center gap-1">
        <Save size={14} /> Save
      </button>
      {#if onClosed}
        <button type="button" aria-label={siviStandalone ? 'Close SIVI form' : 'Close FS882 form'}
          disabled={busy || headerWorkflowBusy} onclick={onClosed}
          class="px-2 py-1 text-stone-500 hover:text-stone-800 font-bold ml-2">✕</button>
      {/if}
    </div>
  </header>

  <!-- Banner Alerts -->
  {#if error}
    <div role="alert" class="p-2 bg-red-50 border-b border-red-200 text-red-800 text-xs flex items-center gap-2">
      <AlertTriangle size={14} class="text-red-600 shrink-0" /><span>{error}</span>
    </div>
  {/if}
  {#if vegetationReadUnavailable}
    <p role="alert" class="p-2 bg-amber-50 text-amber-900 text-sm" data-vegetation-unavailable>
      Ordinary ID-keyed vegetation is unavailable: {vegetationReadUnavailable}
      Use independently enabled physical-row source views. Ordinary vegetation rows, totals and actions are not presented as an empty result.
    </p>
  {/if}
  {#if successMsg}
    <div class="p-2 bg-emerald-50 border-b border-emerald-200 text-emerald-800 text-xs flex items-center gap-2">
      <Check size={14} class="text-emerald-600 shrink-0" /><span>{successMsg}</span>
    </div>
  {/if}

  <!-- Navigation Tabs -->
  <nav aria-label={siviStandalone ? 'SIVI sections' : 'FS882 sections'} class="fs882-tabs flex border-b border-stone-200 bg-stone-50 px-3 text-xs font-medium">
    {#if siviStandalone}
      <button type="button" data-sivi-site-tab data-help-id="sivi.plot_type" aria-current={activeTab === 'siviParent' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'siviParent' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        disabled={tabWorkflowBusy || !siviStandaloneEnabled} onclick={() => { activeTab = 'siviParent'; siviParentOpen = true; }}>Plot &amp; Site Description</button>
    {:else}
      <button data-help-id="site.moisture_regime" aria-current={activeTab === 'site' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'site' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        onclick={() => activeTab = 'site'} disabled={tabWorkflowBusy}>Site & Location</button>
    {/if}
    <button data-help-id="veg.covers" aria-current={activeTab === 'veg' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'veg' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'veg'} disabled={tabWorkflowBusy || siviStandalone && !siviStandaloneEnabled}>Vegetation</button>
    {#if !siviStandalone}
      <button data-help-id="veg.species" aria-current={activeTab === 'vegOther' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'vegOther' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        onclick={() => activeTab = 'vegOther'} disabled={tabWorkflowBusy}>Veg Other</button>
      <button data-help-id="humus.upper_lower_depth" aria-current={activeTab === 'soils' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'soils' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        onclick={() => activeTab = 'soils'} disabled={tabWorkflowBusy}>Soils (Humus & Mineral)</button>
      <button aria-current={activeTab === 'other' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'other' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        onclick={() => activeTab = 'other'} disabled={tabWorkflowBusy}>Other Data</button>
      <button aria-current={activeTab === 'audit' ? 'page' : undefined}
        class="px-4 py-2 border-b-2 {activeTab === 'audit' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
        onclick={() => activeTab = 'audit'} disabled={tabWorkflowBusy}>Audit Trail</button>
    {/if}
  </nav>
  {#if (extendedShrubsEnabled && !twoPageOpen) || (siviParentReviewEnabled && !siviStandalone)}
    <div class="border-b border-stone-200 px-4 py-2 flex flex-wrap items-center gap-4">
      {#if extendedShrubsEnabled && !twoPageOpen}
        <label class="inline-flex items-center gap-2 text-xs font-medium cursor-pointer" data-help-id="veg.covers">
          <input type="checkbox" data-extended-shrubs checked={extendedShrubs} disabled={busy || headerWorkflowBusy}
            onchange={(event) => setExtendedShrubs(event.currentTarget.checked)} />
          Show extended shrub layers
        </label>
      {/if}
      {#if siviParentReviewEnabled && !siviStandalone}
        <button type="button" data-sivi-parent-open aria-pressed={activeTab === 'siviParent'}
          class="rounded border px-3 py-1 text-sm disabled:opacity-50"
          disabled={busy || headerWorkflowBusy || dirty || nonParentChildUnsaved || !siviParentSession}
          onclick={() => void showSIVIParentPanel()}>SIVI parent (read-only)</button>
      {/if}
    </div>
  {/if}

  <!-- Tab Contents -->
  <div class="fs882-body overflow-y-auto flex-1">
    {#if siviStandalone && !siviStandaloneEnabled}
      <p role="alert" class="p-4">Standalone SIVI is unavailable. Its independent entry gate and parent-review gate are required; no FS882 fallback is presented.</p>
    {/if}
    {#snippet sharedSIVIFields()}
      {#if siviParentSharedView}
        {#await import('./SIVIParentSharedFields.svelte')}
          <p role="status" class="p-2 text-xs">Loading shared SIVI fields...</p>
        {:then { default: SIVIParentSharedFields }}
          <SIVIParentSharedFields view={siviParentSharedView} disabled={siviParentSharedEditingDisabled}
            canSave={siviParentSharedClose?.canSave ?? false} onstage={stageSIVIParentShared}
            onoperation={operation => { void siviParentSharedOperation(operation); }}
            onreferences={() => { void siviParentSharedSession?.refreshReferences(); }}
            oncancel={() => siviParentSharedSession?.cancel()}
            {extendedShrubs}
            onExtendedShrubsChange={extendedShrubsEnabled ? setExtendedShrubs : undefined}
            snowCoverRegime={snowCoverRegimeInput}
            activeLayerDepth={activeLayerDepthInput}
            snowCoverRegimeDisabled={headerInputsDisabled || capabilities.snowCoverRegime !== true}
            activeLayerDepthDisabled={headerInputsDisabled || capabilities.activeLayerDepth !== true}
            snowCoverRegimeError={headerValidation.snowCoverRegime ?? null}
            activeLayerDepthError={headerValidation.activeLayerDepth ?? null}
            onSnowCoverRegimeChange={stageSnowCoverRegime}
            onActiveLayerDepthChange={stageActiveLayerDepth} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">Shared SIVI fields could not load: {String(loadError)}</p>
        {/await}
      {/if}
    {/snippet}
    {#if activeTab === 'siviParent' && siviParentOpen && siviParentView && !metadataOpen && (!siviStandalone || siviStandaloneEnabled)}
      {#await import('./SIVIParentReadPanel.svelte')}
        <p role="status" class="p-3 text-xs">Loading SIVI parent panel...</p>
      {:then { default: SIVIParentReadPanel }}
        <SIVIParentReadPanel view={siviParentView} sourceView={siviParentSourceView}
          sharedEditor={siviParentSharedView ? {
            liveColumns: siviParentSharedView.original ? siviParentSharedColumns : [],
            content: sharedSIVIFields, busy: siviParentSharedView.busy,
            unsaved: siviParentSharedUnsaved || (siviParentSharedClose?.blocked ?? false),
          } : null}
          onMetadata={metadataEnabled ? openProjectMetadata : undefined}
          metadataDisabled={!canOpenProjectMetadata()}
          writeView={siviParentWriteView} writeDisabled={siviParentWriteEditingDisabled}
          writeCanSave={siviParentWriteClose?.canSave ?? false}
          onWriteOperation={operation => { void siviParentWriteOperation(operation); }}
          onWriteStage={stageSIVIParentCell}
          onWriteCancel={() => siviParentWriteSession?.cancel()}
          actionView={siviParentActionView} actionDisabled={siviParentActionEditingDisabled}
          actionCanSave={siviParentActionClose?.canSave ?? false}
          onActionOperation={operation => { void siviParentActionOperation(operation); }}
          onActionStage={stageSIVIParentAction}
          onActionCancel={() => siviParentActionSession?.cancel()}
          assignmentView={siviProjectAssignmentView} assignmentDisabled={siviProjectAssignmentEditingDisabled}
          assignmentCanSave={siviProjectAssignmentClose?.canSave ?? false}
          onAssignmentOperation={operation => { void siviProjectAssignmentOperation(operation); }}
          onAssignmentStage={stageSIVIProjectAssignment}
          onAssignmentCancel={() => siviProjectAssignmentSession?.cancel()}
          reloadDisabled={busy || dirty || childUnsaved || siviParentBusy}
          onreload={() => { if (siviParentSession) void siviParentSession.load(); }}
          onSourceReload={() => { void reloadSIVIProjectSource(); }}
          onSourceCancel={() => siviParentSourceSession?.cancel()}
          onSourceChange={source => { void changeSIVIProjectSource(source); }}
          oncancel={() => siviParentSession?.cancel()} />
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">SIVI parent panel could not load: {String(loadError)}</p>
      {/await}
    {/if}
    {#if environmentSUOpen && environmentSUDirection !== 'external-reverse' && $projectState?.projectPath}
      {#await loadSubEditor(() => import('../classification/EnvironmentSUTransfer.svelte'), 'SU transfer review', () => { environmentSUOpen = false; })}
        <p role="status" class="p-3 text-xs">Loading SU transfer review...</p>
      {:then { default: EnvironmentSUTransfer }}
        <EnvironmentSUTransfer bind:this={environmentSUEditor} client={PlotService} {contextId}
          project={$projectState.activeProject} su={$projectState.activeSU} path={$projectState.projectPath}
          direction={environmentSUDirection === 'reverse' ? 'reverse' : 'forward'}
          onclosed={() => { environmentSUOpen = false; error = null; if (environmentSUDirection === 'reverse') void load(original?.plotNumber); }} />
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">SU transfer review could not load: {String(loadError)}</p>
      {/await}
    {/if}
    {#if environmentSUOpen && environmentSUDirection === 'external-reverse' && $projectState?.projectPath && $projectState.suPath}
      {#await loadSubEditor(() => import('../classification/ExternalSUEnvironmentTransfer.svelte'), 'External SU transfer review', () => { environmentSUOpen = false; })}
        <p role="status" class="p-3 text-xs">Loading external SU transfer review...</p>
      {:then { default: ExternalSUEnvironmentTransfer }}
        <ExternalSUEnvironmentTransfer bind:this={environmentSUEditor} {contextId}
          project={$projectState.activeProject} projectPath={$projectState.projectPath}
          su={$projectState.activeSU} suPath={$projectState.suPath}
          onclosed={() => { environmentSUOpen = false; error = null; void load(original?.plotNumber); }} />
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">External SU transfer review could not load: {String(loadError)}</p>
      {/await}
    {/if}
    {#if profileReviewOpen}
      {#await loadSubEditor(() => import('../classification/ProjectPlotProfileReview.svelte'), 'Project plot profile review', () => { profileReviewOpen = false; profileReviewBusy = false; profileReviewBlocked = false; })}
        <p role="status" class="p-3 text-xs">Loading project plot profile review...</p>
      {:then { default: ProjectPlotProfileReview }}
        {#if profileReviewOpen}
          <ProjectPlotProfileReview bind:this={profileEditor} client={PlotService} allowRun={profileRunEnabled}
            allowPresence={profileSpeciesPresenceEnabled && $projectState !== null} {contextId} activeSU={$projectState?.activeSU}
            authorityBlocked={siviSourceBarrier}
            allowEditing={profileEditingEnabled && $projectState?.plotProfile?.writable === true} allowCreation={profileCreationEnabled} allowDeletion={profileDeletionEnabled}
            allowSaveSU={profileSaveSUEnabled} onsureview={async proposal => {
              if (!onProfileSUReview) throw new Error('Save as SU review is unavailable.');
              await onProfileSUReview(proposal, contextId);
            }} allowFiltering={profileFilteringEnabled} onapply={async proposal => {
              if (!onProfileNavigation) throw new Error('Profile navigation publication is unavailable.');
              await onProfileNavigation(proposal, contextId);
            }}
            onblocked={value => profileReviewBlocked = value} onbusy={value => profileReviewBusy = value}
            onclosed={() => { profileReviewOpen = false; }} />
        {/if}
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">Project plot profile review could not load: {String(loadError)}</p>
      {/await}
    {/if}
    {#snippet twoPageLinkedChild(child: TwoPageEntryChild, _control: PaperControl)}
      {#if twoPageSoilReviewEnabled && twoPageScope === 'entry' && twoPageProject !== null &&
        (child.form === 'SoilHumusXL' || child.form === 'SoilMineralXL')}
        {#await import('./TwoPageSoilPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading linked soil panel...</p>
        {:then { default: TwoPageSoilPanel }}
          <TwoPageSoilPanel {contextId} project={twoPageProject} plot={draft.plotNumber} form={child.form}
            parentBlocked={twoPageSoilParentBlocked} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">Linked soil panel could not load: {String(loadError)}</p>
        {/await}
      {:else if twoPageVegetationReviewEnabled && twoPageScope === 'entry' && twoPageProject !== null &&
        (child.form === 'SubVegAXL_BC' || child.form === 'SubVegAXL' || child.form === 'SubVegCXL' || child.form === 'SubVegDXL')}
        {#await import('./TwoPageVegetationPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading linked vegetation panel...</p>
        {:then { default: TwoPageVegetationPanel }}
          {#key JSON.stringify([contextId, twoPageProject, draft.plotNumber, child.form])}
            <TwoPageVegetationPanel {contextId} project={twoPageProject} plot={draft.plotNumber} form={child.form}
              parentBlocked={linkedVegetationParentBlocked(child.form)} />
          {/key}
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">Linked vegetation panel could not load: {String(loadError)}</p>
        {/await}
      {:else if pictureReadingEnabled && twoPageScope === 'entry' && twoPageProject !== null && child.form === 'frmVPicsXL'}
        <div data-two-page-picture-child={child.form} data-source-control={_control.controlId}
          data-source-image-control="form:frmVPicsXL/imgCtl">
          <PicturePanel {contextId} project={twoPageProject} plotNumber={draft.plotNumber} view="child"
            observeMutationReceipts={false}
            disabled={twoPageLinkedParentBlocked || twoPageSoilBusy || twoPageSoilBlocked || twoPageVegetationBusy || twoPageVegetationBlocked}
            onBusyChange={(pending) => pictureBusy = pending} />
        </div>
      {:else if ordinaryVegetationUnavailable && child.form.startsWith('SubVeg')}
        <p role="status" data-vegetation-ordinary-unavailable={child.form}>{ordinaryVegetationUnavailable}</p>
      {:else if ordinarySoilUnavailable && (child.form === 'SoilHumusXL' || child.form === 'SoilMineralXL')}
        <p role="status" data-soil-ordinary-unavailable={child.form}>{ordinarySoilUnavailable}</p>
      {:else}
        <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={true} />
      {/if}
    {/snippet}
    {#if twoPageOpen && twoPageProject !== null}
      {#if twoPageScope === 'entry'}
        {#await loadSubEditor(() => import('./TwoPageEntryEditor.svelte'), 'Two-page complete entry', () => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; })}
          <p role="status" data-two-page-loading="entry" class="p-3 text-xs">Loading two-page complete entry...</p>
        {:then { default: TwoPageEntryEditor }}
          <TwoPageEntryEditor bind:this={twoPageEditor} {contextId} project={twoPageProject} plot={draft.plotNumber}
            initialForm={twoPageEntryInitialForm}
            onExtendedShrubsChange={extendedShrubsEnabled ? setExtendedShrubs : undefined}
            pictureReaderAvailable={pictureReadingEnabled}
            auditReaderAvailable={true}
            googleEarthReaderAvailable={true}
            vegetationTableReaderAvailable={true}
            environmentTableReaderAvailable={true}
            projectMetadataReaderAvailable={true}
            projectMetadataClient={PlotService} metadataRestorationAvailable={metadataRestorationEnabled}
            profilingReaderAvailable={true}
            profilingSource={$projectState?.plotProfile?.source ?? null}
            vegetationTableSU={$projectState?.activeSU ?? ''}
            vegetationTableSUPath={$projectState?.suPath ?? ''}
            sourceProjectPath={$projectState?.projectPath ?? ''}
            externalScope={externalSUEnvironmentEnabled && separateSUSelected && !siviStandalone && $projectState?.projectPath && $projectState.suPath
              ? { contextId, project: $projectState.activeProject, projectPath: $projectState.projectPath, su: $projectState.activeSU, suPath: $projectState.suPath } : null}
            sourceBlocked={twoPageLinkedParentBlocked || Object.keys(headerValidation).length > 0 || twoPageSoilBusy || twoPageSoilBlocked || twoPageVegetationBusy || twoPageVegetationBlocked}
            cache={twoPageEntrySessions} revision={twoPageRevision} onchange={() => twoPageRevision++}
            childBlocked={twoPageSoilBusy || twoPageSoilBlocked || twoPageVegetationBusy || twoPageVegetationBlocked || pictureBusy || pictureMetadataPending}
            onbusy={value => twoPageBusy = value}
            onclosed={() => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; error = null; }}
            oncommitted={() => refreshTwoPagePeers('entry')} embedded={twoPageLinkedChild} />
        {:catch loadError}
          <p role="alert" data-two-page-load-error="entry" class="p-3 text-xs text-red-700">Two-page complete entry could not load: {String(loadError)}</p>
        {/await}
      {:else if twoPageScope === 'common'}
        {#await loadSubEditor(() => import('./TwoPageParentCommonEditor.svelte'), 'Two-page common fields', () => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; })}
          <p role="status" data-two-page-loading="common" class="p-3 text-xs">Loading two-page common fields...</p>
        {:then { default: TwoPageParentCommonEditor }}
          <TwoPageParentCommonEditor bind:this={twoPageEditor} {contextId} project={twoPageProject} plot={draft.plotNumber}
            initialForm={twoPageEntryInitialForm}
            onExtendedShrubsChange={extendedShrubsEnabled ? setExtendedShrubs : undefined}
            cache={twoPageCommonSessions} revision={twoPageRevision} onchange={() => twoPageRevision++}
            onbusy={value => twoPageBusy = value}
            onclosed={() => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; error = null; }}
            oncommitted={() => refreshTwoPagePeers('common')} embedded={twoPageLinkedChild} />
        {:catch loadError}
          <p role="alert" data-two-page-load-error="common" class="p-3 text-xs text-red-700">Two-page common fields could not load: {String(loadError)}</p>
        {/await}
      {:else}
        {#await loadSubEditor(() => import('./TwoPageParentEditor.svelte'), 'Two-page additional fields', () => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; })}
          <p role="status" data-two-page-loading="extra" class="p-3 text-xs">Loading two-page additional fields...</p>
        {:then { default: TwoPageParentEditor }}
          <TwoPageParentEditor bind:this={twoPageEditor} {contextId} project={twoPageProject} plot={draft.plotNumber}
            initialForm={twoPageEntryInitialForm}
            onExtendedShrubsChange={extendedShrubsEnabled ? setExtendedShrubs : undefined}
            cache={twoPageSessions} revision={twoPageRevision} onchange={() => twoPageRevision++}
            onbusy={value => twoPageBusy = value}
            onclosed={() => { twoPageOpen = false; twoPageBusy = false; twoPageProject = null; error = null; }}
            oncommitted={() => refreshTwoPagePeers('extra')} embedded={twoPageLinkedChild} />
        {:catch loadError}
          <p role="alert" data-two-page-load-error="extra" class="p-3 text-xs text-red-700">Two-page additional fields could not load: {String(loadError)}</p>
        {/await}
      {/if}
    {/if}
    {#if metadataOpen}
      {#await loadSubEditor(() => import('../admin/ProjectMetadataEditor.svelte'), 'Project metadata editor', () => { metadataOpen = false; metadataBusy = false; })}
        <p role="status" class="p-3 text-xs">Loading project metadata editor...</p>
      {:then { default: ProjectMetadataEditor }}
        <ProjectMetadataEditor bind:this={metadataEditor} client={PlotService} {contextId} plot={draft.plotNumber}
          allowCreation={metadataCreationEnabled} allowTemplateCreation={metadataTemplateCreationEnabled}
          allowRestoration={metadataRestorationEnabled}
          onbusy={value => metadataBusy = value}
          onclosed={() => { metadataOpen = false; error = null; }}
          oncommitted={refreshProjectMetadata} />
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">Project metadata editor could not load: {String(loadError)}</p>
      {/await}
    {/if}
    {#if codeCheckOpen}
      {#await loadSubEditor(() => import('./SpeciesCodeCheck.svelte'), 'Species code check', () => { codeCheckOpen = false; codeCheckBusy = false; })}
        <p role="status" class="p-3 text-xs">Loading species code check...</p>
      {:then { default: SpeciesCodeCheck }}
        <SpeciesCodeCheck bind:this={codeCheckEditor} client={PlotService}
          onbusy={value => codeCheckBusy = value}
          onclosed={() => { codeCheckOpen = false; error = null; }}
          oncommitted={() => loadChildData(draft.plotNumber)} />
      {:catch loadError}
        <p role="alert" class="p-3 text-xs text-red-700">Species code check could not load: {String(loadError)}</p>
      {/await}
    {/if}
    <FS882ChildCreateModal region="toolbars" {busy} {headerWorkflowBusy} plotNumber={draft.plotNumber}
      {savedPersonalCodes} {personalSpeciesEnabled} {speciesUnsaved} {speciesEditingDisabled} {speciesInvalid}
      {speciesNotices} {speciesDecisionBusy} {speciesDrafts} {speciesChoices} {speciesLists} {speciesReferenceReady}
      {speciesReferenceBusy} {personalDraft} {personalInvalid} {collectedUnsaved} {collectedEditingDisabled}
      {collectedNotices} {attributeUnsaved} {attributeEditingDisabled} {attributeInvalid} {soilUnsaved}
      {soilEditingDisabled} {soilInvalid} {otherUnsaved} {otherEditingDisabled} {otherInvalid} {creationDraft}
      {creationInvalid} {creationSpeciesInvalid} {creationNotice} {creationChoices} {deletionReview} {heightUnsaved}
      {heightEditingDisabled} {heightInvalid} {heightNotices} {heightDrafts} {saveSpeciesDrafts} {cancelSpeciesDrafts}
      {reviewSpeciesChoices} {chooseSpeciesCode} {startPersonalSpecies}
      onPersonalDraftChange={(value) => { personalDraft = value; error = null; successMsg = null; }}
      {savePersonalSpecies} {cancelPersonalSpecies} {saveCollectedDrafts} {cancelCollectedDrafts}
      {saveAttributeDrafts} {cancelAttributeDrafts} {saveSoilDrafts} {cancelSoilDrafts} {saveOtherDrafts}
      {cancelOtherDrafts} {stageCreationSpecies} {reviewCreationSpeciesChoices} {chooseCreationSpecies}
      {startCreationPersonalSpecies} {stageCreationCell} {saveVegetationCreation} {cancelVegetationCreation}
      {confirmVegetationDeletion} {cancelVegetationDeletion} {saveHeightDrafts} {cancelHeightDrafts} />
    <div hidden={twoPageOpen && (twoPageScope === 'entry' || twoPageSourceLayoutEnabled)} data-two-page-ordinary-host>
    {#if activeTab === 'site' && !siviStandalone}
      {#key headerRevision}
        <ParentCodeFields bind:draft {original} {capabilities} scope="site"
          disabled={headerInputsDisabled}
          onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
        {#snippet children(editor)}
        <OrdinaryFields bind:draft {original} {capabilities} scope="site"
          disabled={headerInputsDisabled}
          onchange={markDirty} onvalidation={validateHeader}>
        {#snippet children(ordinaryEditor)}
        <HeaderEditor bind:draft {original} {capabilities} disabled={headerInputsDisabled}
          onMetadata={metadataEnabled ? openProjectMetadata : undefined} metadataDisabled={!canOpenProjectMetadata()}
          {editor} additionalEditor={ordinaryEditor} {masterAllowed} {extendedShrubs}
          snowCoverRegime={snowCoverRegimeInput}
          activeLayerDepth={activeLayerDepthInput}
          snowCoverRegimeError={headerValidation.snowCoverRegime ?? null}
          activeLayerDepthError={headerValidation.activeLayerDepth ?? null}
          onSnowCoverRegimeChange={stageSnowCoverRegime}
          onActiveLayerDepthChange={stageActiveLayerDepth}
          existing={original !== null}
          lists={{ moistureRegime: moistureList, nutrientRegime: nutrientList, mesoSlopePos: mesoSlopeList, surfaceShape: surfaceShapeList }}
          onchange={markDirty} onerror={(message) => error = message} onvalidation={validateHeader}
          onCoordinateBusyChange={(pending) => coordinateBusy = pending}
          onWorkingUnitBusyChange={(pending) => workingUnitBusy = pending}
          onQualityBusyChange={(pending) => qualityBusy = pending}
          onSiteCodeBusyChange={(pending) => siteCodeBusy = pending}
          onRegionCodeBusyChange={(pending) => regionCodeBusy = pending} {workingUnitSession}
          pictureEditor={pictureReadingEnabled && !twoPageOpen ? linkedPictures : undefined} />
        {/snippet}
        </OrdinaryFields>
        {#snippet linkedPictures()}
          {#if $projectState && original}
            <PicturePanel contextId={siviContextId} project={$projectState.activeProject} plotNumber={original.plotNumber}
              disabled={busy || otherHeaderWorkflowBusy || !capabilitiesReady || dirty || childUnsaved}
              metadataDisabled={busy || otherHeaderWorkflowBusy || !capabilitiesReady || !pictureMetadataPending && (dirty || childUnsaved)}
              onBusyChange={(pending) => pictureBusy = pending} metadataSession={pictureMetadataSession} />
          {:else}
            <p role="status">Save and reload an owned plot before reading linked pictures.</p>
          {/if}
        {/snippet}
        {/snippet}
        </ParentCodeFields>
      {/key}
    {:else if activeTab === 'veg' && (!siviStandalone || siviStandaloneEnabled)}
      {#if siviDeletionEnabled && siviDeletionView && siviDeletionClose}
        {#await import('./SIVIDeletionPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI deletion panel...</p>
        {:then { default: SIVIDeletionPanel }}
          <SIVIDeletionPanel view={siviDeletionView} close={siviDeletionClose} targets={siviDeletionView.targets ?? []}
            disabled={siviDeletionEditingDisabled} recoveryDisabled={siviDeletionRecoveryDisabled}
            onreview={(form, rowId) => void siviLifecycleOperation('deletion', 'review', rowId, form)}
            onconfirm={confirmed => stageSIVILifecycle('deletion', confirmed)}
            onoperation={operation => void siviLifecycleOperation('deletion', operation)} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI deletion panel could not load: {String(loadError)}</p>
        {/await}
      {/if}
      {#if siviRestorationEnabled && siviRestorationView && siviRestorationClose}
        {#await import('./SIVIDeletionRestorationPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI restoration panel...</p>
        {:then { default: SIVIDeletionRestorationPanel }}
          <SIVIDeletionRestorationPanel view={siviRestorationView} close={siviRestorationClose} targets={siviRestorationTargets}
            disabled={siviRestorationEditingDisabled} recoveryDisabled={siviRestorationRecoveryDisabled}
            onreview={historyId => void siviLifecycleOperation('restoration', 'review', historyId)}
            onchoose={action => stageSIVILifecycle('restoration', false, action)}
            onconfirm={confirmed => stageSIVILifecycle('restoration', confirmed)}
            onoperation={operation => void siviLifecycleOperation('restoration', operation)} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI restoration panel could not load: {String(loadError)}</p>
        {/await}
      {/if}
      {#if siviCreationUndoEnabled && siviCreationUndoView && siviCreationUndoClose}
        {#await import('./SIVICreationUndoPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI creation undo panel...</p>
        {:then { default: SIVICreationUndoPanel }}
          <SIVICreationUndoPanel view={siviCreationUndoView} close={siviCreationUndoClose}
            disabled={siviCreationUndoEditingDisabled} recoveryDisabled={siviCreationUndoRecoveryDisabled}
            onreview={historyId => void siviCreationUndoOperation('review', historyId)}
            onchoose={action => stageSIVICreationUndo(false, action)}
            onconfirm={confirmed => stageSIVICreationUndo(confirmed)}
            onoperation={operation => void siviCreationUndoOperation(operation)} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI creation undo panel could not load: {String(loadError)}</p>
        {/await}
      {/if}
      {#if siviCreationEnabled && siviCreationSession && siviCreationView && siviCreationClose}
        {#await import('./SIVICreationPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI creation panel...</p>
        {:then { default: SIVICreationPanel }}
          <SIVICreationPanel view={siviCreationView} close={siviCreationClose}
            owner={{ contextId: siviContextId, project: siviCreationView.references?.project ?? $projectState?.activeProject ?? '', plot: original?.plotNumber ?? '' }}
            disabled={siviCreationEditingDisabled} recoveryDisabled={siviCreationRecoveryDisabled}
            onoperation={(operation, form) => void siviCreationOperation(operation, form)} onchange={stageSIVICreation} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI creation panel could not load: {String(loadError)}</p>
        {/await}
      {/if}
      {#if siviHeightEnabled}
        <button type="button" class="mb-3 rounded border px-3 py-1 disabled:opacity-50" data-sivi-open
          disabled={busy || headerWorkflowBusy || !siviPanelOpen && siviEditingDisabled && !siviUnsaved}
          onclick={() => void showSIVIPanel()}>{siviPanelOpen ? (siviStandalone ? 'Close height view' : 'Return to FS882 vegetation') : 'Show SIVI aggregate heights (height-only)'}</button>
      {/if}
      {#if siviCoverEnabled}
        <button type="button" class="mb-3 rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-open
          disabled={busy || headerWorkflowBusy || !siviCoverPanelOpen && siviCoverEditingDisabled && !siviCoverUnsaved}
          onclick={() => void showSIVICoverPanel()}>{siviCoverPanelOpen ? (siviStandalone ? 'Close cover view' : 'Return to FS882 vegetation') : 'Show SIVI source covers'}</button>
      {/if}
      {#if siviCombinedEnabled}
        <button type="button" class="mb-3 rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-open
          disabled={busy || headerWorkflowBusy || !siviCombinedPanelOpen && siviCombinedEditingDisabled && !siviCombinedUnsaved}
          onclick={() => void showSIVICombinedPanel()}>{siviCombinedPanelOpen ? 'Close combined view' : 'Show combined SIVI covers and heights'}</button>
      {/if}
      {#if siviCollectedEnabled}
        <button type="button" class="mb-3 rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-open
          disabled={busy || headerWorkflowBusy || !siviCollectedPanelOpen && siviCollectedEditingDisabled && !siviCollectedUnsaved}
          onclick={() => void showSIVICollectedPanel()}>{siviCollectedPanelOpen ? 'Close Collected view' : 'Show SIVI Collected cycles'}</button>
      {/if}
      {#if siviSpeciesEnabled}
        <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-open
          disabled={busy || headerWorkflowBusy || !siviSpeciesPanelOpen && siviSpeciesEditingDisabled && !siviSpeciesUnsaved}
          onclick={() => void showSIVISpeciesPanel()}>{siviSpeciesPanelOpen ? 'Close Species view' : 'Show SIVI Species'}</button>
      {/if}
      {#if siviIdentityEnabled}
        <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-open
          disabled={busy || headerWorkflowBusy || !siviIdentityPanelOpen && siviIdentityEditingDisabled && !siviIdentityUnsaved}
          onclick={() => void showSIVIIdentityPanel()}>{siviIdentityPanelOpen ? 'Close ID view' : 'Show SIVI IDs'}</button>
      {/if}
      {#if extendedShrubs && !siviPanelOpen && !siviCoverPanelOpen && !siviCombinedPanelOpen && !siviCollectedPanelOpen && !siviSpeciesPanelOpen && !siviIdentityPanelOpen}
        <p class="mb-3 text-sm" role="status">Extended shrubs use cover mode (Cover5a–Cover5c / Height5a–Height5c). Turn this option off to use height mode; hidden values, drafts and errors are retained.</p>
      {/if}
      {#if siviIdentityPanelOpen && siviIdentityView}
        {#if siviIdentityReading && siviIdentityBusy}
          <button type="button" class="rounded border px-3 py-1" data-sivi-identity-cancel-read
            onclick={() => siviIdentityReads.cancelAll()}>Cancel ID source read</button>
        {/if}
        {#await import('./SIVIIdentityPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI ID panel...</p>
        {:then { default: SIVIIdentityPanel }}
          <SIVIIdentityPanel view={siviIdentityView} disabled={siviIdentityEditingDisabled} canSave={siviIdentityClose?.canSave ?? false}
            onstage={stageSIVIIdentity} onsave={() => void siviIdentityOperation('save')} onundo={() => void siviIdentityOperation('undo')}
            onreload={() => void siviIdentityOperation('load')}
            onrestore={(action) => void siviIdentityOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI ID panel could not load: {String(loadError)}</p>
        {/await}
      {:else if siviSpeciesPanelOpen && siviSpeciesView}
        {#if siviSpeciesReading && siviSpeciesBusy}
          <button type="button" class="rounded border px-3 py-1" data-sivi-species-cancel-read
            onclick={() => siviSpeciesReads.cancelAll()}>Cancel Species source/reference read</button>
        {/if}
        {#await import('./SIVISpeciesPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI Species panel...</p>
        {:then { default: SIVISpeciesPanel }}
          <SIVISpeciesPanel view={siviSpeciesView} disabled={siviSpeciesEditingDisabled} canSave={siviSpeciesClose?.canSave ?? false}
            onstage={(rowId, raw) => stageSIVISpecies(rowId, 'Species', raw)}
            oncontext={(rowId, group) => stageSIVISpecies(rowId, 'context', String(group))}
            onchoose={(rowId, kind, selected) => stageSIVISpecies(rowId, kind, selected ?? '')}
            onsave={() => void siviSpeciesOperation('save')} onundo={() => void siviSpeciesOperation('undo')}
            onreload={() => void siviSpeciesOperation('load')} onrestore={(action) => void siviSpeciesOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI Species panel could not load: {String(loadError)}</p>
        {/await}
      {:else if siviCollectedPanelOpen && siviCollectedView}
        {#if siviCollectedReading && siviCollectedBusy}
          <button type="button" class="mb-3 rounded border px-3 py-1" data-sivi-collected-cancel-read
            onclick={() => siviCollectedReads.cancelAll()}>Cancel Collected source read</button>
        {/if}
        {#await import('./SIVICollectedPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI Collected panel...</p>
        {:then { default: SIVICollectedPanel }}
          <SIVICollectedPanel view={siviCollectedView} disabled={siviCollectedEditingDisabled} canSave={siviCollectedClose?.canSave ?? false}
            oncycle={cycleSIVICollected} onsave={() => void siviCollectedOperation('save')} onundo={() => void siviCollectedOperation('undo')}
            onreload={() => void siviCollectedOperation('load')} onrestore={(action) => void siviCollectedOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI Collected panel could not load: {String(loadError)}</p>
        {/await}
      {:else if siviCombinedPanelOpen && siviCombinedView}
        {#if siviCombinedReading && siviCombinedBusy}
          <button type="button" class="mb-3 rounded border px-3 py-1" data-sivi-combined-cancel-read
            onclick={() => siviCombinedReads.cancelAll()}>Cancel combined source read</button>
        {/if}
        {#await import('./SIVICombinedPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI combined panel...</p>
        {:then { default: SIVICombinedPanel }}
          <SIVICombinedPanel view={siviCombinedView} disabled={siviCombinedEditingDisabled} canSave={siviCombinedClose?.canSave ?? false}
            onstage={stageSIVICombinedCell} onsave={() => void siviCombinedOperation('save')} onundo={() => void siviCombinedOperation('undo')}
            onreload={() => void siviCombinedOperation('load')} onrestore={(action) => void siviCombinedOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI combined panel could not load: {String(loadError)}</p>
        {/await}
      {:else if siviPanelOpen && siviView}
        {#await import('./SIVIHeightPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI height panel...</p>
        {:then { default: SIVIHeightPanel }}
          <SIVIHeightPanel view={siviView} disabled={siviEditingDisabled} canSave={siviClose?.canSave ?? false}
            onstage={stageSIVICell} onsave={() => void siviOperation('save')} onundo={() => void siviOperation('undo')}
            onreload={() => void siviOperation('load')} onrestore={(action) => void siviOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI height panel could not load: {String(loadError)}</p>
        {/await}
      {:else if siviCoverPanelOpen && siviCoverView}
        {#if siviCoverReading && siviCoverBusy}
          <button type="button" class="mb-3 rounded border px-3 py-1" data-sivi-cover-cancel-read
            onclick={() => siviCoverReads.cancelAll()}>Cancel source cover read</button>
        {/if}
        {#await import('./SIVICoverPanel.svelte')}
          <p role="status" class="p-2 text-xs">Loading SIVI cover panel...</p>
        {:then { default: SIVICoverPanel }}
          <SIVICoverPanel view={siviCoverView} disabled={siviCoverEditingDisabled} canSave={siviCoverClose?.canSave ?? false}
            onstage={stageSIVICoverCell} onsave={() => void siviCoverOperation('save')} onundo={() => void siviCoverOperation('undo')}
            onreload={() => void siviCoverOperation('load')} onrestore={(action) => void siviCoverOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
        {:catch loadError}
          <p role="alert" class="p-2 text-xs text-red-700">SIVI cover panel could not load: {String(loadError)}</p>
        {/await}
      {:else if vegetationReadUnavailable}
        <p role="alert" class="p-4">Ordinary ID-keyed vegetation is unavailable: {vegetationReadUnavailable}
          Independently enabled physical-row SIVI views remain available; no NULL identity is converted to zero and no row is omitted from an ordinary result.</p>
      {:else if siviStandalone}
        <p role="status" class="p-4">SIVI vegetation preserves the three PlotNumber-linked source groups: A (trees/shrubs), C and D.
          Choose an independently enabled source view above. Species creation/deletion and original species-list callbacks remain unavailable.</p>
      {:else}
      {#if speciesEditingEnabled || creationEnabled}
        {#if speciesReferenceBusy}<p role="status">Loading source species lists...</p>{/if}
        {#if speciesReferenceError}<p role="alert">{speciesReferenceError}</p>{/if}
        <button type="button" class="mb-2 px-2 py-1 border rounded" disabled={busy || speciesReferenceBusy || headerWorkflowBusy}
          onclick={() => void loadSpeciesReferences()}>{speciesReferenceError ? 'Retry species references' : 'Reload species references'}</button>
      {/if}
      <OrdinaryFields bind:draft {original} {capabilities} scope="veg"
        disabled={headerInputsDisabled}
        onchange={markDirty} onvalidation={validateHeader} onVegNotesTab={vegetationNotesTab}>
      {#snippet children(ordinaryEditor)}
      <SourcePage name="Vegetation" values={sourceHeaderValues} {vegetationMode} editor={ordinaryEditor} onHeightToggle={() => vegetationMode = vegetationMode === 'height' ? 'cover' : 'height'}
        heightToggleDisabled={extendedShrubs || busy || headerWorkflowBusy}
        {onFindPlot} findPlotDisabled={busy || headerWorkflowBusy || !capabilitiesReady}
        onSpeciesCheck={codeCheckEnabled ? openSpeciesCodeCheck : undefined} speciesCheckDisabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}>
        {#snippet embedded(control)}
          {@const originalChild = embeddedForm(control.controlId)}
          {@const child = { ...originalChild, form: extendedShrubs && originalChild.form === 'SubVegAXL_BC' ? 'SubVegAXL' : originalChild.form }}
          {@const heightGrid = child.form === 'SubVegAhtXL' || child.form === 'SubVegChtXL'}
          {#if creationEnabled}
            <button type="button" class="mb-2 rounded border px-2 py-1 text-xs disabled:opacity-50" data-create-source={child.form}
              disabled={childParentDisabled || childUnsaved || !speciesReferenceReady || speciesReferenceBusy}
              onclick={() => startVegetationCreation(child.form)}>New vegetation record - {child.form}</button>
          {/if}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={heightGrid || numberEditingEnabled ? heightEditingDisabled : childEditingDisabled}
            drafts={heightDrafts} onstage={numberEditingEnabled ? (id, column, raw) => stageHeightCell(id, column, raw, child.form) : undefined}
            {collectedDrafts} collectedDisabled={collectedEditingDisabled}
            oncollectedstage={collectedEditingEnabled ? stageCollectedCell : undefined}
            {speciesDrafts} {speciesLists} speciesDisabled={speciesEditingDisabled}
            onspeciesstage={speciesEditingEnabled ? stageSpeciesCell : undefined}
            onedit={undefined} ondelete={deletionEnabled ? reviewVegetationDeletion : undefined} deleteDisabled={childUnsaved} />
        {/snippet}
      </SourcePage>
      {/snippet}
      </OrdinaryFields>
      {/if}
      {#if vegetationMode === 'height'}
        <p class="mt-2 text-xs text-stone-500">{numberEditingEnabled ? 'Cover and height values validate against LMH25 (< 100%) single-precision bounds; use Save or Cancel to commit or discard drafts.' : 'Cover/height cells are read-only.'} Switching view does not save or alter data.</p>
      {/if}
      {#if collectedEditingEnabled}
        <p class="mt-2 text-xs text-stone-500">Collected buttons cycle NULL -> C -> V -> NULL using persistent drafts. Other historical values remain unchanged, as in the source click event. Use Save or Cancel; {creationEnabled ? 'new rows use a separate explicit source creation draft.' : 'new-row workflows remain unavailable in the source grids.'}</p>
      {/if}
      {#if numberEditingEnabled}
        <p class="mt-2 text-xs text-stone-500">All source cover/total and height fields share persistent drafts. Switching grids retains raw errors and original values; Save checks each field's original source row and commits changes/audits atomically. Clearing cover may remove a row from a source view, never delete the vegetation record.</p>
      {/if}
      <p class="mt-2 text-xs text-stone-500">{speciesEditingEnabled ? `Species selection uses canonical source lists and explicit old-code replace/keep or existing personal-code decisions. Decisions stage the source UCase event; Save remains separate. ${personalSpeciesEnabled ? 'Unknown-code personal definitions use a separate explicit user-database save; plot Save and Undo remain independent.' : 'Personal-list creation remains unavailable.'}` : 'Species editing remains read-only pending native verification.'}</p>
      <FS882StratumSummary {vegList} {totalTreeCover} {totalShrubCover} {totalHerbCover} {totalMossCover}
        onopenspeciesmodal={() => { showSpeciesModal = true; }} ondeleteveg={deleteVeg} />
      <FS882ChildCreateModal region="speciesModal" bind:showSpeciesModal bind:speciesSearchQuery
        {speciesSearchResults} {searchingSpecies} {searchSpecies} {selectSpeciesToAdd} />
    {:else if activeTab === 'soils'}
      <FS882SoilHorizonSection bind:draft {original} {capabilities} {sourceHeaderValues} {ordinarySoilUnavailable}
        {historicalSoilUnavailable} {soilEditingEnabled} {soilCodeEditingEnabled} {soilReferenceBusy}
        {soilReferenceError} {soilEditingDisabled} {soilUnsaved} {headerInputsDisabled} {busy} {headerWorkflowBusy}
        {coordinateBusy} {workingUnitBusy} {qualityBusy} {siteCodeBusy} {regionCodeBusy}
        bind:soilCodeBusy bind:geologyCodeBusy bind:parentCodeBusy bind:drainageBusy
        {childRevision} {soilDrafts} {soilSuggestions} {humusList} {mineralList} {sourceRows} {markDirty}
        {validateHeader} {loadSoilSuggestions} {openNewChild} {stageSoilCell} {deleteSourceChild} {deleteHumus} />
    {:else if activeTab === 'vegOther'}
      <FS882OtherRecordsSection mode="vegOther" {sourceHeaderValues} {childRevision} {busy} {headerWorkflowBusy}
        {sourceRows} {vegetationReadUnavailable} {attributeEditingEnabled} {attributeReferenceBusy}
        {attributeReferenceError} {attributeEditingDisabled} {attributeDrafts} {attributeSuggestions}
        {loadAttributeSuggestions} {stageAttributeCell} />
    {:else if activeTab === 'other'}
      <FS882OtherRecordsSection mode="other" {sourceHeaderValues} {childRevision} {busy} {headerWorkflowBusy}
        {sourceRows} {otherEditingEnabled} {otherEditingDisabled} {otherUnsaved} {otherDrafts} {otherList}
        {openNewChild} {stageOtherCell} {deleteSourceChild} />
    {:else if activeTab === 'audit'}
      <FS882AuditTrailSection entries={auditList} project={auditProject} plot={original?.plotNumber ?? ''}
        expectedPlotNumber={plotNumber} {capabilities} {childCapabilities} enabled={auditRestoreEnabled}
        disabled={auditBlocked} gateReason={auditGateReason} revision={childRevision}
        getLoadRequest={() => loadRequest} {PlotService}
        onBusyChange={(val) => { busy = val; }}
        onFeedback={(err, msg) => { error = err; if (msg !== null || err === null) successMsg = msg; }}
        onRefreshAfterRestore={refreshAfterRestore}
        onRefreshFailure={(msg) => { capabilitiesReady = false; childRevision++; error = msg; }} />
    {/if}
    </div>
    <FS882ChildCreateModal region="newChild" {newChild} {busy} {error} {createChild} cancelNewChild={() => { newChild = null; }} />
  </div>
</div>
