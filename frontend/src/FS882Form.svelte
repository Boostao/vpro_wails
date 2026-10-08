<script lang="ts">
  import {
    PlotService as LegacyPlotService,
    ContextService,
    SIVIParentSharedService,
    AuditRestoreAction,
    ReferenceService,
    type FS882Header,
    type VegRecord,
    type HumusRecord,
    type MineralRecord,
    type OtherRecord,
    type SoilSuggestion,
    type AuditEntry,
    type ListItem,
    type SpeciesItem,
    type VegetationDeletionReview,
    type ProjectPlotProfileFilterRequest
  } from '../bindings/github.com/boostao/vpro-wails';
  import { Lock, Unlock, Save, RotateCcw, AlertTriangle, Check, Plus, Trash2, Search, X } from '@lucide/svelte';
  import HeaderEditor from './HeaderEditor.svelte';
  import SourcePage from './SourcePage.svelte';
  import SoilCodeReference from './SoilCodeReference.svelte';
  import SoilCodeFields from './SoilCodeFields.svelte';
  import GeologyCodeFields from './GeologyCodeFields.svelte';
  import ParentCodeFields from './ParentCodeFields.svelte';
  import OrdinaryFields from './OrdinaryFields.svelte';
  import DrainageFields from './DrainageFields.svelte';
  import SourceChild from './SourceChild.svelte';
  import SIVIHeightPanel from './SIVIHeightPanel.svelte';
  import SIVIParentReadPanel from './SIVIParentReadPanel.svelte';
  import SIVIParentSharedFields from './SIVIParentSharedFields.svelte';
  import { SIVIParentSharedSession, type SIVIParentSharedColumn, type SIVIParentSharedInput } from './siviParentSharedSession';
  import { SIVIParentReadSession } from './siviParentReadSession';
  import { SIVIParentSourceSession } from './siviParentSourceSession';
  import { SIVIParentWriteSession, isSIVIParentDirectColumn } from './siviParentWriteSession';
  import { SIVIParentActionWriteSession, isSIVIParentActionColumn } from './siviParentActionWriteSession';
  import { SIVIProjectAssignmentSession, type SIVIProjectAssignmentInput } from './siviProjectAssignmentSession';
  import type { SIVIParentInput } from './siviParentEditor';
  import { SIVIHeightSession } from './siviHeightSession';
  import type { SIVIHeightColumn } from './siviHeightEditor';
  import NewChild from './NewChild.svelte';
  import PersonalSpeciesFields from './PersonalSpeciesFields.svelte';
  import SpeciesCodeCheck from './SpeciesCodeCheck.svelte';
  import ProjectMetadataEditor from './ProjectMetadataEditor.svelte';
  import EnvironmentSUTransfer from './EnvironmentSUTransfer.svelte';
  import ProjectPlotProfileReview from './ProjectPlotProfileReview.svelte';
  import { navigateSourceEnter } from './enterNavigation';
  import AuditRestore, { auditRestoreReason } from './AuditRestore.svelte';
  import { projectState } from './state';
  import { embeddedForm, paperChild, type VegetationMode } from './paperLayout';
  import { controlLabel } from './formPresentation';
  import type { EditorCloseState } from './closeLifecycle';
  import { heightField, stageHeight, heightDirty, heightErrors, heightSourceNotices, aCoverSourceNotice, aCoverSourceNotices, heightUpdates, vegetationNumberUpdates, type HeightDrafts } from './heightEditor';
  import { otherField, stageOther, otherDirty, otherErrors, otherUpdates, type OtherDrafts, type OtherValue } from './otherEditor';
  import { soilField, stageSoil, soilDirty, soilErrors, soilUpdates, type SoilDrafts, type SoilKind } from './soilChildEditor';
  import { vegetationAttributeField, stageVegetationAttribute, vegetationAttributeDirty, vegetationAttributeErrors, vegetationAttributeUpdates, type VegetationAttributeDrafts } from './vegetationAttributeEditor';
  import { stageCollected, collectedDirty, collectedUpdates, type CollectedDrafts } from './collectedEditor';
  import { speciesForms, stageSpecies, speciesDirty, speciesErrors, speciesUpdates, chooseSpecies, speciesEventError, type SpeciesDrafts, type SpeciesLists, type SpeciesChoices, type SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import type { WorkingUnitSession } from './workingUnitEditor';
  import { onDestroy, untrack, tick } from 'svelte';
  import { bindContextPlots } from './contextPlots';
  import { ReadRequests } from './readRequests';
  import { validateDeletionReview } from './vegetationDeletionEditor';
  import { beginVegetationCreation, stageVegetationCreation, stageVegetationCreationSpecies, chooseVegetationCreationSpecies, vegetationCreationSpeciesError, vegetationCreationErrors, vegetationCreationRequest, type VegetationCreationDraft } from './vegetationCreationEditor';
  import { beginPersonalSpecies, beginCreationPersonalSpecies, matchesPersonalSpeciesSource, matchesCreationPersonalSpeciesSource, stagePersonalText, personalSpeciesErrors, personalSpeciesRequest, personalSpeciesMatches, personalSpeciesCommittedError, personalLifeforms, personalTextFields, type PersonalSpeciesDraft } from './personalSpeciesEditor';

  let { plotNumber, contextId, onSaved, onClosed, onBusyChange, onProfileNavigation, onProfileSUReview, onFindPlot }: { plotNumber?: string; contextId: string; onSaved?: (p: string) => void | Promise<void>; onClosed?: () => void; onBusyChange?: (busy: boolean) => void; onFindPlot?: () => void;
    onProfileNavigation?: (proposal: ProjectPlotProfileFilterRequest, contextId: string) => Promise<void>;
    onProfileSUReview?: (proposal: ProjectPlotProfileFilterRequest, contextId: string) => Promise<void>;
  } = $props();
  const PlotService = { ...LegacyPlotService, ...bindContextPlots(untrack(() => contextId)) };
  const siviContextId = untrack(() => contextId);
  const siviHeightEnabled = import.meta.env.VITE_SIVI_HEIGHT_EDITING === 'true';
  const siviParentReviewEnabled = import.meta.env.VITE_SIVI_PARENT_REVIEW === 'true';
  const siviParentEditingEnabled = siviParentReviewEnabled && import.meta.env.VITE_SIVI_PARENT_EDITING === 'true';
  const siviParentSharedEnabled = siviParentReviewEnabled && import.meta.env.VITE_SIVI_PARENT_SHARED_EDITING === 'true';
  const siviParentSharedColumns: readonly SIVIParentSharedColumn[] = [
    'AirPhotoNum', 'XCoord', 'YCoord', 'StrataCoverTree', 'StrataCoverShrub', 'StrataCoverHerb', 'StrataCoverMoss', 'VegNotes',
  ];
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
  const siviParentActionEditingEnabled = siviParentEditingEnabled && import.meta.env.VITE_SIVI_PARENT_ACTION_EDITING === 'true';
  const siviParentActionReads = new ReadRequests();
  let siviParentActionSession = $state<SIVIParentActionWriteSession | null>(null);
  let siviParentActionRevision = $state(0);
  const siviParentActionView = $derived.by(() => { siviParentActionRevision; return siviParentActionSession?.view() ?? null; });
  const siviParentActionClose = $derived.by(() => { siviParentActionRevision; return siviParentActionSession?.closeState() ?? null; });
  const siviParentActionUnsaved = $derived(siviParentActionClose?.unsaved ?? false);
  const siviProjectAssignmentEnabled = siviParentEditingEnabled && import.meta.env.VITE_SIVI_PROJECT_ASSIGNMENT === 'true';
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
  const reads = new ReadRequests();
  const referenceReads = new ReadRequests();
  const speciesReads = new ReadRequests();
  const speciesReferenceReads = new ReadRequests();
  const speciesChoiceReads = new ReadRequests();
  const deletionReads = new ReadRequests();
  let deletionRequest = 0;
  let deletionBusy = $state(false);
  let deletionReview = $state<VegetationDeletionReview | null>(null);
  const deletionEnabled = import.meta.env.VITE_VEGETATION_DELETE_EDITING === 'true';
  const creationEnabled = import.meta.env.VITE_VEGETATION_CREATE_EDITING === 'true';
  let creationDraft = $state<VegetationCreationDraft | null>(null);
  let creationChoices = $state<SpeciesChoices | null>(null);
  const personalSpeciesEnabled = import.meta.env.VITE_PERSONAL_SPECIES_EDITING === 'true';
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
  const extendedShrubsEnabled = import.meta.env.VITE_EXTENDED_SHRUBS === 'true';
  let extendedShrubs = $state(false);
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
  const soilCodeEditingEnabled = import.meta.env.VITE_SOIL_CODES_EDITING !== 'false';
  const codeCheckEnabled = import.meta.env.VITE_SPECIES_CODE_CHECK_EDITING === 'true';
  let codeCheckOpen = $state(false);
  let codeCheckBusy = $state(false);
  let codeCheckEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const metadataEnabled = import.meta.env.VITE_PROJECT_METADATA_EDITING === 'true';
  const metadataCreationEnabled = import.meta.env.VITE_PROJECT_METADATA_CREATION === 'true';
  const metadataTemplateCreationEnabled = import.meta.env.VITE_PROJECT_METADATA_TEMPLATE_CREATION === 'true';
  const metadataRestorationEnabled = import.meta.env.VITE_PROJECT_METADATA_RESTORE === 'true';
  const sourceEnterEnabled = import.meta.env.VITE_SOURCE_ENTER_NAVIGATION === 'true';
  function sourceEnter(event: KeyboardEvent) {
    if (!sourceEnterEnabled || !container) return;
    try { navigateSourceEnter(event, container); }
    catch (cause) { error = `Source Enter navigation failed; drafts retained: ${String(cause)}`; }
  }
  let metadataOpen = $state(false);
  let metadataBusy = $state(false);
  const environmentSUEnabled = import.meta.env.VITE_SOURCE_ENV_SU_TRANSFER === 'true';
  const siteUnitEnvironmentEnabled = import.meta.env.VITE_SOURCE_SU_ENV_TRANSFER === 'true';
  let environmentSUOpen = $state(false);
  let environmentSUDirection = $state<'forward' | 'reverse'>('forward');
  let environmentSUEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const profileReviewEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_REVIEW === 'true';
  const profileRunEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_RUN === 'true';
  const profileEditingEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_EDITING === 'true';
  const profileCreationEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_CREATION === 'true';
  const profileDeletionEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_DELETION === 'true';
  const profileFilteringEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_FILTERING === 'true';
  const profileSaveSUEnabled = import.meta.env.VITE_PROJECT_PLOT_PROFILE_SAVE_SU === 'true';
  let profileReviewOpen = $state(false);
  let profileReviewBusy = $state(false);
  let profileReviewBlocked = $state(false);
  let profileEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  let metadataEditor = $state<{ getCloseState(): EditorCloseState; undo(): void }>();
  const headerWorkflowBusy = $derived(coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy || geologyCodeBusy || parentCodeBusy || drainageBusy || speciesDecisionBusy || deletionBusy || codeCheckBusy || metadataBusy || profileReviewBusy || environmentSUOpen || siviParentBusy || siviBusy);
  const workingUnitSession: WorkingUnitSession & { plot: string } = { mode: null, plot: '' };
  const heightEditingEnabled = import.meta.env.VITE_HEIGHT_EDITING !== 'false';
  const numberEditingEnabled = heightEditingEnabled && import.meta.env.VITE_VEGETATION_NUMBER_EDITING !== 'false';
  let heightDrafts = $state<HeightDrafts>({});
  const heightUnsaved = $derived(heightDirty(heightDrafts));
  const heightInvalid = $derived(heightErrors(heightDrafts));
  const otherEditingEnabled = import.meta.env.VITE_OTHER_EDITING !== 'false';
  let otherDrafts = $state<OtherDrafts>({});
  const otherUnsaved = $derived(otherDirty(otherDrafts));
  const otherInvalid = $derived(otherErrors(otherDrafts));
  const soilEditingEnabled = import.meta.env.VITE_SOIL_CHILD_EDITING !== 'false';
  let soilDrafts = $state<SoilDrafts>({});
  const soilUnsaved = $derived(soilDirty(soilDrafts));
  const soilInvalid = $derived(soilErrors(soilDrafts));
  let soilSuggestions = $state<SoilSuggestion[]>([]);
  let soilReferenceReady = $state(false);
  let soilReferenceBusy = $state(false);
  let soilReferenceError = $state<string | null>(null);
  const attributeEditingEnabled = import.meta.env.VITE_VEGETATION_ATTRIBUTE_EDITING !== 'false';
  let attributeDrafts = $state<VegetationAttributeDrafts>({});
  const attributeUnsaved = $derived(vegetationAttributeDirty(attributeDrafts));
  const attributeInvalid = $derived(vegetationAttributeErrors(attributeDrafts));
  let attributeSuggestions = $state<SoilSuggestion[]>([]);
  let attributeReferenceReady = $state(false);
  let attributeReferenceBusy = $state(false);
  let attributeReferenceError = $state<string | null>(null);
  const collectedEditingEnabled = import.meta.env.VITE_VEGETATION_COLLECTED_EDITING !== 'false';
  let collectedDrafts = $state<CollectedDrafts>({});
  const collectedUnsaved = $derived(collectedDirty(collectedDrafts));
  const speciesEditingEnabled = import.meta.env.VITE_VEGETATION_SPECIES_EDITING !== 'false';
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
  const nonParentChildUnsaved = $derived(heightUnsaved || siviUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || deletionReview !== null || creationDraft !== null || codeCheckOpen || metadataOpen || profileReviewBlocked || environmentSUOpen);
  const childUnsaved = $derived(nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved);
  let error = $state<string | null>(null);
  let successMsg = $state<string | null>(null);
  let dirty = $state(false);
  let headerValidation = $state<Record<string, string>>({});
  let capabilities = $state<Record<string, boolean | undefined>>({});
  let capabilitiesReady = $state(false);
  let masterAllowed = $state(false);
  type ChildKind = 'Veg' | 'Humus' | 'Mineral' | 'Other';
  let childCapabilities = $state<Record<ChildKind, Record<string, boolean | undefined>>>({ Veg: {}, Humus: {}, Mineral: {}, Other: {} });
  let loadRequest = 0;
  onDestroy(() => {
    siviParentSession?.dispose();
    siviParentSourceSession?.dispose();
    siviParentWriteSession?.dispose();
    siviParentActionSession?.dispose();
    siviProjectAssignmentSession?.dispose();
    siviParentSharedSession?.dispose();
    siviSession?.dispose();
    siviReads.cancelAll();
    loadRequest++; referenceRequest++; speciesRequest++;
    reads.cancelAll(); referenceReads.cancelAll(); speciesReads.cancelAll();
    speciesReferenceRequest++; speciesReferenceReads.cancelAll();
    speciesChoiceRequest++; speciesChoiceReads.cancelAll();
    deletionRequest++; deletionReads.cancelAll();
  });
  let container: HTMLDivElement;

  $effect(() => { onBusyChange?.(busy || headerWorkflowBusy); });

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
  const newHumusFields = { humusStructureDegree: null, humusStructureKind: null, mycelAbundance: null, fecalAbundance: null, rootsAbundance: null, rootsSize: null, vonPost: null };
  const newMineralFields = { pitDepthLimit: null, asp: null, percentCoarseFragsGravel: null, percentCoarseFragsCobbles: null, percentCoarseFragsStones: null, percentCoarseFragsTotal: null, percentCoarseFragsShape: null, rootsAbundance: null, rootsSize: null, mineralStructureClass: null, mineralStructureKind: null, mineralFormpH: null };
  const newOtherFields = { userItem1: null, userItem2: null, userItem3: null, userFlag1: null, userFlag2: null, userFlag3: null };
  const newVegetationFields = { height1: null, height2: null, height3: null, height4: null, height5: null, height6: null, ll: null, af: null, dc: null, ut: null, vi: null, pv: null, pg: null, ffa: null, cultural1: null, cultural2: null, other1: null, other2: null };
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
  let auditList = $state<AuditEntry[]>([]);
  let childRevision = $state(0);
  let headerRevision = $state(0);

  const treeColumns = ['cover1', 'cover2', 'cover3', 'totalA', 'cover4', 'cover5', 'cover5a', 'cover5b', 'cover5c', 'totalB'] as const;
  const heightTreeColumns = ['cover1', 'height1', 'cover2', 'height2', 'cover3', 'height3', 'totalA', 'cover4', 'height4', 'cover5', 'height5', 'totalB'] as const;
  function childValues(record: object, kind: ChildKind) {
    const values: Record<string, string | number | boolean | null | undefined> = {};
    const stored = new Map(Object.entries(record));
    for (const [key, supported] of Object.entries(childCapabilities[kind])) {
      if (supported) {
        const value = stored.get(key);
        if (value == null || typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
          values[key.toLowerCase()] = value ?? null;
        }
      }
    }
    return values;
  }
  function sourceRows(name: string) {
    if (['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL', 'SubVegAhtXL', 'SubVegChtXL'].includes(name)) {
      const records = vegList.filter(row => name === 'SubVegAXL_BC' || name === 'SubVegAXL'
        ? treeColumns.some(key => row[key] != null)
        : name === 'SubVegAhtXL' ? heightTreeColumns.some(key => row[key] != null)
        : name === 'SubVegCXL' || name === 'SubVegChtXL' ? row.cover6 != null
        : row.cover7 != null || row.cover8 != null || row.cover9 != null);
      return records.map(row => ({
        id: row.id,
        values: childValues(row, 'Veg')
      }));
    }
    if (name === 'USysVegOtherXL') return vegList.map(row => ({
      id: row.id, values: childValues(row, 'Veg')
    }));
    if (name === 'SoilHumusXL') return humusList.map(row => ({
      id: row.id, values: {
        ...childValues(row, 'Humus'),
        ...(childCapabilities.Humus.ph ? { humusformph: row.ph ?? null } : {})
      }
    }));
    if (name === 'SoilMineralXL') return mineralList.map(row => ({
      id: row.id, values: childValues(row, 'Mineral')
    }));
    if (name === 'SubOtherXL') return otherList.map(row => ({
      id: row.id, values: childValues(row, 'Other')
    }));
    throw new Error(`Child data binding is not migrated for ${name}`);
  }

  function newDraft(): FS882Header {
    return {
    plotNumber: '',
    projectId: null,
    surveyor: null,
    date: null,
    fieldNo: null,
    generalLocation: null,
    mapSheet: null,
    utmZone: null,
    easting: null,
    northing: null,
    accuracy: null,
    plotRepresenting: null,
    siteSeries: null,
    transition: null,
    mapUnit: null,
    moistureRegime: null,
    nutrientRegime: null,
    successional: null,
    structuralStage: null,
    standAge: null,
    elevation: null,
    slope: null,
    aspect: null,
    mesoSlopePos: null,
    surfaceShape: null,
    microtopType: null,
    microtopSize: null,
    fieldNotes: null,
    officeNotes: null,
    startDate: null,
    airPhotoNum: null,
    becSiteUnit: null,
    bedrockGeology1: null,
    bedrockGeology2: null,
    bedrockGeology3: null,
    coarseFragLith1: null,
    coarseFragLith2: null,
    coarseFragLith3: null,
    ecosection: null,
    enteredBy: null,
    exposure1: null,
    exposure2: null,
    fsRegionDistrict: null,
    floodingRegimeDur: null,
    floodingRegimeFreq: null,
    geoMorProSubSurf: null,
    geoMorProSurf: null,
    humusForm: null,
    humusFormPhase: null,
    humusThickness: null,
    hydroGeoSubSystem: null,
    hydroGeoSystem: null,
    latitude: null,
    longitude: null,
    photo: null,
    realmClass: null,
    rootRestrictingDepth: null,
    rootRestrictingType: null,
    rootZoneParticleSize: null,
    rootingDepth: null,
    seepageDepth: null,
    siteDisturbance1: null,
    siteDisturbance2: null,
    siteDisturbance3: null,
    sitePlotQuality: null,
    soilClassGroup: null,
    soilClassSubGroup: null,
    soilDrainage: null,
    soilNotes: null,
    soilPlotQuality: null,
    soilSurveyor: null,
    speciesListComplete: null,
    strataCoverHerb: null,
    strataCoverMoss: null,
    strataCoverShrub: null,
    strataCoverTree: null,
    subZone: null,
    substrateBedRock: null,
    substrateDecWood: null,
    substrateMineralSoil: null,
    substrateOrganicMatter: null,
    substrateRocks: null,
    substrateWater: null,
    surfaceExpSubSurf: null,
    surfaceExpSurf: null,
    surficialMaterialSubSurf: null,
    surficialMaterialSurf: null,
    terrainTextureSubSurf: null,
    terrainTextureSurf: null,
    updatedFromCards: null,
    userSiteUnit: null,
    vegNotes: null,
    vegPlotQuality: null,
    vegSurveyor: null,
    waterSource: null,
    xCoord: null,
    yCoord: null,
    zone: null,
    locked: false,
    };
  }

  let draft = $state<FS882Header>(newDraft());
  const sourceHeaderValues = $derived(new Map(Object.entries(draft).map(([key, value]) => [key.toLowerCase(), value])));

  let original = $state<FS882Header | null>(null);
  const childParentDisabled = $derived(!capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || original === null || deletionReview !== null || creationDraft !== null || personalDraft !== null || codeCheckOpen || metadataOpen || profileReviewBlocked);
  const siviParentSharedEditingDisabled = $derived(!siviParentSharedEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved ||
    personalDraft !== null || Object.keys(headerValidation).length > 0 || (siviParentSharedView?.blocked ?? false));
  const siviParentWriteEditingDisabled = $derived(!siviParentEditingEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || personalDraft !== null || Object.keys(headerValidation).length > 0 || (siviParentWriteView?.blocked ?? false));
  const siviParentActionEditingDisabled = $derived(!siviParentActionEditingEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || personalDraft !== null || Object.keys(headerValidation).length > 0 || (siviParentActionView?.blocked ?? false));
  const siviProjectAssignmentEditingDisabled = $derived(!siviProjectAssignmentEnabled || !capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty ||
    original === null || nonParentChildUnsaved || siviParentWriteUnsaved || siviParentActionUnsaved || siviParentSharedUnsaved || personalDraft !== null ||
    Object.keys(headerValidation).length > 0 || Boolean(siviParentSourceView?.error) || (siviProjectAssignmentView?.blocked ?? false));
  const heightEditingDisabled = $derived(!numberEditingEnabled || childParentDisabled || siviUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const otherEditingDisabled = $derived(!otherEditingEnabled || childParentDisabled || siviUnsaved || heightUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const soilEditingDisabled = $derived(!soilEditingEnabled || !soilReferenceReady || soilReferenceBusy || childParentDisabled || siviUnsaved || heightUnsaved || otherUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved);
  const attributeEditingDisabled = $derived(!attributeEditingEnabled || !attributeReferenceReady || attributeReferenceBusy || childParentDisabled || siviUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || collectedUnsaved || speciesUnsaved);
  const collectedEditingDisabled = $derived(!collectedEditingEnabled || childParentDisabled || siviUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || speciesUnsaved);
  const speciesEditingDisabled = $derived(!speciesEditingEnabled || !speciesReferenceReady || speciesReferenceBusy || childParentDisabled || siviUnsaved || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved);
  const siviEditingDisabled = $derived(!siviHeightEnabled || childParentDisabled || heightUnsaved || otherUnsaved || soilUnsaved || attributeUnsaved || collectedUnsaved || speciesUnsaved || Object.keys(headerValidation).length > 0 || (siviView?.blocked ?? false));
  const childEditingDisabled = $derived(childParentDisabled || childUnsaved);
  const auditRestoreEnabled = import.meta.env.VITE_AUDIT_RESTORE !== 'false';
  const auditProject = $derived($projectState?.activeProject ?? '');
  const auditBlocked = $derived(childEditingDisabled || Object.keys(headerValidation).length > 0);
  const auditGateReason = $derived(busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
    : draft.locked ? 'Unlock the plot before restoring fields.'
    : dirty || Object.keys(headerValidation).length > 0 ? 'Save valid header changes or Undo before restoring fields.'
    : 'Save the plot before restoring fields.');

  async function siviParentWriteOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviParentWriteSession || busy || headerWorkflowBusy || siviParentActionUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviParentWriteEditingDisabled) {
      error = 'SIVI parent edits require a clean unlocked parent and no conflicting operation; resolve acknowledgements explicitly.';
      return false;
    }
    error = null;
    successMsg = null;
    try {
      const ok = operation === 'load' ? await siviParentWriteSession.load()
        : operation === 'save' ? await siviParentWriteSession.save()
        : operation === 'undo' ? await siviParentWriteSession.undo()
        : await siviParentWriteSession.restore(operation === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) {
        error = siviParentWriteSession.view().error;
        return false;
      }
      successMsg = operation === 'save' ? 'SIVI directly bound parent fields saved and independently reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed parent restoration completed and reloaded.'
        : 'SIVI parent originals reloaded; no mutation was replayed.';
      return true;
    } catch (cause) {
      error = `SIVI parent operation failed; owner state retained: ${String(cause)}`;
      return false;
    }
  }

  async function siviParentActionOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviParentActionSession || busy || headerWorkflowBusy || siviParentWriteUnsaved || siviProjectAssignmentUnsaved || siviParentSharedUnsaved || operation !== 'undo' && siviParentActionEditingDisabled) {
      error = 'SIVI source actions require a clean unlocked parent and no conflicting drafts or operation; resolve acknowledgements explicitly.';
      return false;
    }
    error = null;
    successMsg = null;
    try {
      const ok = operation === 'load' ? await siviParentActionSession.load()
        : operation === 'save' ? await siviParentActionSession.save()
        : operation === 'undo' ? await siviParentActionSession.undo()
        : await siviParentActionSession.restore(operation === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviParentActionSession.view().error; return false; }
      successMsg = operation === 'save' ? 'SIVI source actions saved; callback directive verified and owned parent reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed source-action restoration completed and reloaded.'
        : 'SIVI action originals reloaded; no mutation was replayed.';
      return true;
    } catch (cause) {
      error = `SIVI source-action operation failed; owner state retained: ${String(cause)}`;
      return false;
    }
  }

  function stageSIVIParentAction(column: string, input: SIVIParentInput) {
    try {
      if (!siviParentActionSession || siviParentActionEditingDisabled || !isSIVIParentActionColumn(column)) {
        throw new Error('Only independently available SIVI source actions can be drafted.');
      }
      siviParentActionSession.stage(column, input);
      error = siviParentActionSession.view().error;
      successMsg = null;
    } catch (cause) {
      error = `SIVI source-action draft failed; existing state retained: ${String(cause)}`;
    }
  }

  async function siviProjectAssignmentOperation(operation: 'load' | 'save' | 'undo' | 'retain' | 'prune'): Promise<boolean> {
    if (!siviProjectAssignmentSession || busy || headerWorkflowBusy || siviParentWriteUnsaved || siviParentActionUnsaved || siviParentSharedUnsaved ||
      operation !== 'undo' && siviProjectAssignmentEditingDisabled) {
      error = 'ProjectID assignment requires a clean unlocked parent and no conflicting drafts or operation; resolve acknowledgements explicitly.';
      return false;
    }
    error = null;
    successMsg = null;
    try {
      const ok = operation === 'load' ? await siviProjectAssignmentSession.load()
        : operation === 'save' ? await siviProjectAssignmentSession.save()
        : operation === 'undo' ? await siviProjectAssignmentSession.undo()
        : await siviProjectAssignmentSession.restore(operation === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) { error = siviProjectAssignmentSession.view().error; return false; }
      successMsg = operation === 'save' ? 'Existing physical ProjectID definition assigned; owned parent and choices reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'Typed ProjectID restoration completed and reloaded; metadata was not changed.'
        : 'Owned ProjectID parent and choices reloaded; no mutation was replayed.';
      return true;
    } catch (cause) {
      error = `ProjectID assignment operation failed; owner state retained: ${String(cause)}`;
      return false;
    }
  }

  function stageSIVIProjectAssignment(input: SIVIProjectAssignmentInput) {
    try {
      if (!siviProjectAssignmentSession || siviProjectAssignmentEditingDisabled) {
        throw new Error('Only independently available existing ProjectID definitions can be selected.');
      }
      siviProjectAssignmentSession.stage('ProjectID', input);
      error = siviProjectAssignmentSession.view().error;
      successMsg = null;
    } catch (cause) {
      error = `ProjectID selection failed; existing state retained: ${String(cause)}`;
    }
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
      error = 'Shared SIVI edits require a clean unlocked parent and no conflicting operation; recover uncertain writes explicitly.';
      return false;
    }
    error = null; successMsg = null;
    try {
      const ok = operation === 'load' ? await siviParentSharedSession.load()
        : operation === 'save' ? await siviParentSharedSession.save()
        : operation === 'undo' ? await siviParentSharedSession.undo()
        : await siviParentSharedSession.restore(operation === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
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
    if (!siviSession || !siviHeightEnabled || busy || headerWorkflowBusy || operation !== 'undo' && siviEditingDisabled) {
      error = 'SIVI requires an available clean, unlocked parent and no conflicting operation or unresolved acknowledgement.';
      return false;
    }
    error = null;
    successMsg = null;
    try {
      const ok = operation === 'load' ? await siviSession.load()
        : operation === 'save' ? await siviSession.save()
        : operation === 'undo' ? await siviSession.undo()
        : await siviSession.restore(operation === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) {
        error = siviSession.view().error;
        return false;
      }
      successMsg = operation === 'save' ? 'SIVI aggregate heights saved; current rows and audits reloaded.'
        : operation === 'retain' || operation === 'prune' ? 'SIVI typed height restoration completed; current rows and audits reloaded.'
        : 'SIVI source rows reloaded. No height mutation was replayed.';
      return true;
    } catch (cause) {
      error = `SIVI operation failed; owner state retained: ${String(cause)}`;
      return false;
    }
  }

  function stageSIVICell(rowId: string, column: SIVIHeightColumn, raw: string, nullValue: boolean) {
    try {
      if (!siviSession || siviEditingDisabled) throw new Error('Unlock the clean available parent before editing SIVI heights.');
      siviSession.stage(rowId, column, raw, nullValue);
      const message = siviSession.view().error;
      if (message) error = `SIVI ${message}`;
      else if (error?.startsWith('SIVI ')) error = null;
      successMsg = null;
    } catch (cause) {
      error = `SIVI draft failed; original/raw state retained: ${String(cause)}`;
    }
  }

  async function showSIVIPanel() {
    if (siviPanelOpen) { siviPanelOpen = false; return; }
    if (siviEditingDisabled && !siviUnsaved) { error = 'SIVI requires a clean, unlocked available parent.'; return; }
    siviPanelOpen = true;
    if (!siviView?.review && !siviUnsaved) await siviOperation('load');
  }

  function openSpeciesCodeCheck() {
    if (!codeCheckEnabled || childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0) {
      error = 'Finish or Undo existing drafts and unlock the available plot before reviewing species codes.';
      return;
    }
    codeCheckOpen = true; error = null; successMsg = null;
  }

  function openProjectMetadata() {
    if (!metadataEnabled || childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0) {
      error = 'Finish or Undo existing drafts and unlock the available plot before reviewing project metadata.';
      return;
    }
    metadataOpen = true; error = null; successMsg = null;
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

  async function refreshAfterRestore(plot: string, request: number) {
    const header = await reads.track(PlotService.GetPlot(plot));
    if (!header) throw new Error('Restored plot could not be reloaded.');
    await loadChildData(plot);
    if (request !== loadRequest) return;
    const locked = draft.locked;
    draft = { ...header, locked };
    original = JSON.parse(JSON.stringify(header));
    dirty = false;
    headerValidation = {};
  }

  async function restoreAudit(rowIds: string[], action: AuditRestoreAction) {
    const plot = original?.plotNumber;
    if (!auditRestoreEnabled || auditBlocked || !plot || plot !== plotNumber) {
      error = auditGateReason;
      return;
    }
    if (action !== AuditRestoreAction.AuditRestoreCancel) {
      if (rowIds.length === 0 || new Set(rowIds).size !== rowIds.length) {
        error = 'Select distinct audit rows before restoring.';
        return;
      }
      for (const rowId of rowIds) {
        const entry = auditList.find(row => row.rowId === rowId);
        const reason = entry ? auditRestoreReason(entry, auditProject, plot, { header: capabilities, children: childCapabilities }) : 'Audit row disappeared; refresh before restoring.';
        if (reason) { error = reason; return; }
      }
    }
    const request = loadRequest;
    busy = true;
    error = null;
    successMsg = null;
    let marksWritten = false;
    try {
      if (action !== AuditRestoreAction.AuditRestoreCancel) {
        await PlotService.SetAuditRestoreSelection(plot, rowIds);
        marksWritten = true;
      }
      const result = await PlotService.RestoreSelectedAuditRecords(plot, rowIds, action);
      if (!result) throw new Error('Restoration result was not returned.');
      if (result.cleanedVegRows !== 0) throw new Error('Unexpected vegetation deletion reported; stop using this application binary.');
      if (request === loadRequest) successMsg = result.cancelled ? 'Restoration cancelled. Data and history were unchanged.'
        : `Restored ${result.restoredRows} fields; ${result.prunedAuditRows} selected audit rows pruned. No vegetation records deleted.`;
    } catch (cause) {
      if (request === loadRequest) error = `Restoration failed: ${String(cause)}`;
    } finally {
      if (marksWritten) {
        try { await PlotService.SetAuditRestoreSelection(plot, []); }
        catch (cause) { if (request === loadRequest) error = `${error ?? ''} Selection marks could not be cleared: ${String(cause)}`; }
      }
      try { await refreshAfterRestore(plot, request); }
      catch (cause) {
        if (request === loadRequest) {
          capabilitiesReady = false;
          childRevision++;
          error = `${error ?? ''} Restoration refresh failed: ${String(cause)}`;
        }
      }
      if (request === loadRequest) busy = false;
    }
  }

  async function load(p?: string) {
    if (siviParentSharedSession && (siviParentSharedSession.closeState().unsaved || siviParentSharedSession.closeState().busy)) {
      error = 'Finish or explicitly Undo shared SIVI drafts/recovery before replacing their plot owner.';
      return;
    }
    if (siviProjectAssignmentSession && (siviProjectAssignmentSession.closeState().unsaved || siviProjectAssignmentSession.closeState().busy)) {
      error = 'Finish or explicitly Undo ProjectID selection/recovery before replacing its plot owner.';
      return;
    }
    if (siviParentActionSession && (siviParentActionSession.closeState().unsaved || siviParentActionSession.closeState().busy)) {
      error = 'Finish or explicitly Undo SIVI source-action drafts/recovery before replacing their plot owner.';
      return;
    }
    if (siviParentWriteSession && (siviParentWriteSession.closeState().unsaved || siviParentWriteSession.closeState().busy)) {
      error = 'Finish or explicitly Undo SIVI parent drafts/recovery before replacing their plot owner.';
      return;
    }
    if (siviParentSourceView?.busy) {
      error = 'Wait for the current SIVI source operation before replacing its plot owner.';
      return;
    }
    if (siviSession && (siviSession.closeState().unsaved || siviSession.closeState().busy)) {
      error = 'SIVI drafts or operations must finish or be explicitly undone before replacing their plot owner.';
      return;
    }
    siviParentSession?.dispose();
    siviParentSession = null;
    siviParentSourceSession?.dispose();
    siviParentSourceSession = null;
    siviParentWriteSession?.dispose();
    siviParentWriteSession = null;
    siviParentActionSession?.dispose();
    siviParentActionSession = null;
    siviProjectAssignmentSession?.dispose();
    siviProjectAssignmentSession = null;
    siviParentSharedSession?.dispose();
    siviParentSharedSession = null;
    siviParentSharedRevision++;
    siviProjectAssignmentRevision++;
    siviParentActionRevision++;
    siviParentWriteRevision++;
    siviParentSourceRevision++;
    siviParentRevision++;
    siviParentOpen = false;
    if (activeTab === 'siviParent') activeTab = 'site';
    siviSession?.dispose();
    siviSession = null;
    siviReads.cancelAll();
    siviRevision++;
    siviPanelOpen = false;
    const request = ++loadRequest;
    reads.cancelAll();
    if (workingUnitSession.plot !== (p ?? '')) workingUnitSession.mode = null;
    workingUnitSession.plot = p ?? '';
    busy = true;
    error = null;
    capabilitiesReady = false;
    masterAllowed = false;
    heightDrafts = {};
    otherDrafts = {};
    soilDrafts = {};
    attributeDrafts = {};
    collectedDrafts = {};
    speciesDrafts = {};
    speciesChoices = {};
    deletionReview = null;
    creationDraft = null;
    creationChoices = null;
    personalDraft = null;
    savedPersonalCodes = [];
    vegList = [];
    humusList = [];
    mineralList = [];
    otherList = [];
    auditList = [];
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
                read: () => siviParentSharedReads.track(SIVIParentSharedService.GetOriginal(siviContextId, plot)),
                cancelRead: () => siviParentSharedReads.cancelAll(),
                save: request => SIVIParentSharedService.Save(siviContextId, plot, request),
                restore: (historyId, action) => SIVIParentSharedService.Restore(siviContextId, plot, historyId, action),
                refreshParent: () => refreshSIVIParent(plot, [siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession]),
              }, () => siviParentSharedRevision++);
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
            refreshParent: () => loadChildData(plot),
          }, () => siviRevision++, extendedShrubs);
          siviRevision++;
        }
      } else {
        draft = newDraft();
        original = null;
        dirty = false;
        headerValidation = {};
      }
      capabilitiesReady = true;
    } catch (e) {
      if (request === loadRequest) error = String(e);
    } finally {
      if (request === loadRequest) busy = false;
    }
  }

  async function loadChildData(p: string) {
    const request = loadRequest;
    try {
      const [v, h, m, o, a, vc, hc, mc, oc] = await Promise.all([
        reads.track(PlotService.ListVegRecords(p)),
        reads.track(PlotService.ListHumusRecords(p)),
        reads.track(PlotService.ListMineralRecords(p)),
        reads.track(PlotService.ListOtherRecords(p)),
        reads.track(PlotService.ListAuditEntries(p)),
        reads.track(PlotService.GetChildCapabilities('Veg')),
        reads.track(PlotService.GetChildCapabilities('Humus')),
        reads.track(PlotService.GetChildCapabilities('Mineral')),
        reads.track(PlotService.GetChildCapabilities('Other'))
      ]);
      if (request !== loadRequest) return;
      if (vc === null || hc === null || mc === null || oc === null) throw new Error('Child capabilities were not returned.');
      childCapabilities = { Veg: vc, Humus: hc, Mineral: mc, Other: oc };
      vegList = v ?? [];
      humusList = h ?? [];
      mineralList = m ?? [];
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
        referenceReads.track(ReferenceService.GetListItems('MoistureRegime')),
        referenceReads.track(ReferenceService.GetListItems('NutrientRegime')),
        referenceReads.track(ReferenceService.GetListItems('MesoSlopePosition')),
        referenceReads.track(ReferenceService.GetListItems('SurfaceShape')),
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
    vegList.reduce((sum, r) => sum + (r.totalA || 0), 0)
  );
  let totalShrubCover = $derived(
    vegList.reduce((sum, r) => sum + (r.totalB || 0), 0)
  );
  let totalHerbCover = $derived(
    vegList.reduce((sum, r) => sum + (r.cover6 || 0), 0)
  );
  let totalMossCover = $derived(
    vegList.reduce((sum, r) => sum + (r.cover7 || 0), 0)
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
      const res = await speciesReads.track(ReferenceService.SearchSpecies(speciesSearchQuery, 15));
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
    if (!soilEditingEnabled || soilReferenceBusy) return;
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
    const key = column.toLowerCase();
    function numeric() {
      if (raw === '') return undefined;
      const value = Number(raw);
      if (!Number.isFinite(value)) throw new Error(`Enter a finite number for ${column}.`);
      return value;
    }
    try {
      if (name.startsWith('SubVeg')) {
        const row = vegList.find(record => record.id === id);
        if (!row) throw new Error('Vegetation row disappeared; reload before editing.');
        const numbers: Record<string, 'cover1' | 'cover2' | 'cover3' | 'totalA' | 'cover4' | 'cover5' | 'totalB' | 'cover6' | 'cover7' | 'cover8' | 'cover9'> = {
          cover1: 'cover1', cover2: 'cover2', cover3: 'cover3', totala: 'totalA', cover4: 'cover4',
          cover5: 'cover5', totalb: 'totalB', cover6: 'cover6', cover7: 'cover7', cover8: 'cover8', cover9: 'cover9'
        };
        const numberKey = numbers[key];
        if (numberKey) await childOperation(() => PlotService.UpdateVegRecord({ ...row, [numberKey]: numeric() }));
        else if (key === 'species') {
          throw new Error('Species requires the source-list draft workflow; unrestricted source-grid edits are unavailable.');
        } else if (key === 'collected') {
          await childOperation(() => PlotService.UpdateVegRecord({ ...row, collected: raw || undefined }));
        } else throw new Error(`Vegetation column ${column} is not migrated.`);
      } else if (name === 'SoilHumusXL') {
        const row = humusList.find(record => record.id === id);
        if (!row) throw new Error('Humus row disappeared; reload before editing.');
        const numbers: Record<string, 'upperDepth' | 'lowerDepth' | 'ph'> = { upperdepth: 'upperDepth', lowerdepth: 'lowerDepth', humusformph: 'ph' };
        if (numbers[key]) await childOperation(() => PlotService.UpdateHumusRecord({ ...row, [numbers[key]]: numeric() }));
        else if (key === 'horizon' || key === 'comment') await childOperation(() => PlotService.UpdateHumusRecord({ ...row, [key]: raw || undefined }));
        else throw new Error(`Humus column ${column} is not migrated.`);
      } else if (name === 'SoilMineralXL') {
        const row = mineralList.find(record => record.id === id);
        if (!row) throw new Error('Mineral row disappeared; reload before editing.');
        const numbers: Record<string, 'upperDepth' | 'lowerDepth'> = { upperdepth: 'upperDepth', lowerdepth: 'lowerDepth' };
        if (numbers[key]) await childOperation(() => PlotService.UpdateMineralRecord({ ...row, [numbers[key]]: numeric() }));
        else if (key === 'horizon' || key === 'texture' || key === 'colour' || key === 'comments') {
          await childOperation(() => PlotService.UpdateMineralRecord({ ...row, [key]: raw || undefined }));
        } else throw new Error(`Mineral column ${column} is not migrated.`);
      } else if (name === 'SubOtherXL') {
        const row = otherList.find(record => record.id === id);
        if (!row) throw new Error('Other row disappeared; reload before editing.');
        const fields: Record<string, 'dataName' | 'dataItem'> = { dataname: 'dataName', dataitem: 'dataItem' };
        if (!fields[key]) throw new Error(`Other column ${column} is not migrated.`);
        await childOperation(() => PlotService.UpdateOtherRecord({ ...row, [fields[key]]: raw || undefined }));
      } else throw new Error(`Child form ${name} is not migrated.`);
    } catch (cause) {
      error = `Child input failed: ${String(cause)}`;
    }
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
    if (busy || headerWorkflowBusy) {
      error = 'Wait for the current operation to finish before saving.';
      return;
    }
    if (codeCheckOpen) { error = 'Save replacements or personal metadata explicitly, then close species-code review; ordinary plot Save never applies them.'; return; }
    if (metadataOpen) { error = 'Save or Undo metadata explicitly, then close metadata review; ordinary plot Save never applies it.'; return; }
    if (profileReviewBlocked) { error = 'Save or Undo profile drafts explicitly; ordinary plot Save never writes profile rules.'; return; }
    if (personalDraft !== null) { error = 'Save the personal definition explicitly or Cancel its entry; ordinary plot Save never writes the user database.'; return; }
    if (deletionReview !== null) { error = 'Confirm reviewed vegetation deletion explicitly or Cancel deletion review; ordinary Save never deletes rows.'; return; }
    if (creationDraft !== null) { await saveVegetationCreation(); return; }
    if (siviParentActionUnsaved) { await siviParentActionOperation('save'); return; }
    if (siviProjectAssignmentUnsaved) { await siviProjectAssignmentOperation('save'); return; }
    if (siviParentWriteUnsaved) { await siviParentWriteOperation('save'); return; }
    if (siviParentSharedUnsaved) { await siviParentSharedOperation('save'); return; }
    if (siviUnsaved) { await siviOperation('save'); return; }
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
      const payload = { ...draft, locked: capabilities.locked === true && draft.locked };
      if (original) await PlotService.UpdatePlot(payload);
      else await PlotService.CreatePlot(payload);
      original = JSON.parse(JSON.stringify(draft));
      workingUnitSession.plot = draft.plotNumber;
      dirty = false;
      headerValidation = {};
      successMsg = `Plot ${draft.plotNumber} saved successfully.`;
      await loadChildData(draft.plotNumber);
      if (onSaved) await onSaved(draft.plotNumber);
    } catch (e) {
      error = String(e);
    } finally {
      busy = false;
    }
  }

  export function getCloseState(): EditorCloseState {
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
    const invalid = Object.keys(headerValidation).length > 0 || heightInvalid.length > 0 || otherInvalid.length > 0 || soilInvalid.length > 0 || attributeInvalid.length > 0 || speciesInvalid.length > 0 || deletionReview !== null || creationInvalid.length > 0;
    const invalidChild = Boolean(container?.querySelector('.source-child input:invalid'));
    const saveReason = busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
      : draft.locked ? 'Unlock the plot before saving.'
      : siviParentActionUnsaved && !siviParentActionClose?.canSave ? siviParentActionClose?.saveReason ?? 'Correct or Undo unresolved SIVI source actions.'
      : siviParentWriteUnsaved && !siviParentWriteClose?.canSave ? siviParentWriteClose?.saveReason ?? 'Correct or Undo unresolved SIVI parent drafts.'
      : siviParentSharedUnsaved && !siviParentSharedClose?.canSave ? siviParentSharedClose?.saveReason ?? 'Correct or Undo unresolved shared SIVI drafts.'
      : siviProjectAssignmentUnsaved && !siviProjectAssignmentClose?.canSave ? siviProjectAssignmentClose?.saveReason ?? 'Correct or Undo unresolved ProjectID selections.'
      : siviUnsaved && !siviClose?.canSave ? siviClose?.saveReason ?? 'Correct or Undo unresolved SIVI heights.'
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
      blocked: (siviClose?.blocked ?? false) || (siviParentWriteClose?.blocked ?? false) || (siviParentActionClose?.blocked ?? false) || (siviProjectAssignmentClose?.blocked ?? false) || (siviParentSharedClose?.blocked ?? false), canSave: saveReason === '', saveReason, error };
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
    if (siviParentActionUnsaved) { void siviParentActionOperation('undo'); return; }
    if (siviProjectAssignmentUnsaved) { void siviProjectAssignmentOperation('undo'); return; }
    if (siviParentWriteUnsaved) { void siviParentWriteOperation('undo'); return; }
    if (siviParentSharedUnsaved) { void siviParentSharedOperation('undo'); return; }
    if (siviUnsaved) { void siviOperation('undo'); return; }
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
    dirty = false;
    headerValidation = {};
    error = null;
    headerRevision++;
    successMsg = 'Changes undone.';
  }

  async function toggleLock() {
    if (busy || headerWorkflowBusy) {
      error = 'Wait for the current operation to finish before changing the plot lock.';
      return;
    }
    if (codeCheckOpen) { error = 'Finish or close species-code review before changing the plot lock.'; return; }
    if (metadataOpen) { error = 'Finish or close metadata review before changing the plot lock.'; return; }
    if (profileReviewBlocked) { error = 'Finish profile drafts before changing the plot lock.'; return; }
    if (personalDraft !== null) {
      error = 'Save the personal definition explicitly or Cancel its entry before changing the plot lock.';
      return;
    }
    if (creationDraft !== null) {
      error = 'Save or Cancel vegetation creation before changing the plot lock.';
      return;
    }
    if (deletionReview !== null) {
      error = 'Confirm or Cancel vegetation deletion review before changing the plot lock.';
      return;
    }
    if (heightUnsaved) {
      error = 'Save or Cancel height drafts before changing the plot lock.';
      return;
    }
    if (siviParentWriteUnsaved) {
      error = 'Save or explicitly Undo SIVI parent drafts/recovery before changing the plot lock.';
      return;
    }
    if (siviParentSharedUnsaved) {
      error = 'Save or explicitly Undo shared SIVI drafts/recovery before changing the plot lock.';
      return;
    }
    if (siviParentActionUnsaved) {
      error = 'Save or explicitly Undo SIVI source-action drafts/recovery before changing the plot lock.';
      return;
    }
    if (siviProjectAssignmentUnsaved) {
      error = 'Save or explicitly Undo ProjectID selection/recovery before changing the plot lock.';
      return;
    }
    if (siviUnsaved) {
      error = 'SIVI heights or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock.';
      return;
    }
    if (otherUnsaved) {
      error = 'Save or Cancel Other drafts before changing the plot lock.';
      return;
    }
    if (soilUnsaved) {
      error = 'Save or Cancel soil drafts before changing the plot lock.';
      return;
    }
    if (attributeUnsaved) {
      error = 'Save or Cancel vegetation attribute drafts before changing the plot lock.';
      return;
    }
    if (collectedUnsaved) {
      error = 'Save or Cancel Collected drafts before changing the plot lock.';
      return;
    }
    if (speciesUnsaved) {
      error = 'Save or Cancel species drafts before changing the plot lock.';
      return;
    }
    if (!draft.locked && dirty) {
      await save();
      if (dirty) return;
    }
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

<div bind:this={container} class="fs882-container bg-white rounded-lg shadow border border-stone-200 flex flex-col h-full text-stone-800 text-sm">
  <!-- Header Bar -->
  <header class="fs882-toolbar p-3 bg-stone-100 border-b border-stone-200 flex justify-between items-center">
    <div class="flex items-center gap-3">
      <h2 class="font-bold text-stone-900 text-base">FS882-6x4XL: Ecosystem Field Form</h2>
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
      {#if environmentSUEnabled}
        <button type="button" data-source-control="btnEnvIntoSu" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0 || $projectState?.activeSU === 'None'}
          onclick={() => openEnvironmentSU()}>Env Into SU</button>
      {/if}
      {#if siteUnitEnvironmentEnabled}
        <button type="button" data-source-control="btnSuIntoEnv" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0 || $projectState?.activeSU === 'None'}
          onclick={() => openEnvironmentSU('reverse')}>SU Into Env</button>
      {/if}
      {#if onFindPlot}
        <button type="button" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={busy || headerWorkflowBusy || !capabilitiesReady} onclick={onFindPlot}>Find Plot</button>
      {/if}
      {#if profileReviewEnabled}
        <button type="button" class="px-3 py-1 rounded text-xs border border-stone-300 bg-white disabled:opacity-50"
          disabled={busy || headerWorkflowBusy || profileReviewOpen || !capabilitiesReady ||
            (profileEditingEnabled && $projectState?.plotProfile?.writable === true && (metadataOpen || codeCheckOpen || personalDraft !== null || deletionReview !== null || creationDraft !== null))}
          onclick={() => { profileReviewOpen = true; }}>
          Review project profile (read-only)
        </button>
      {/if}
      <button
        type="button"
        onclick={toggleLock}
        disabled={busy || headerWorkflowBusy || !capabilitiesReady}
        class="px-3 py-1 rounded text-xs font-medium border flex items-center gap-1 {draft.locked ? 'bg-stone-200 hover:bg-stone-300 border-stone-300' : 'bg-stone-100 hover:bg-stone-200 border-stone-300'}"
        title={draft.locked ? 'Unlock Form' : 'Lock Form'}
      >
        {#if draft.locked}
          <Unlock size={14} /> Unlock
        {:else}
          <Lock size={14} /> Lock
        {/if}
      </button>

      <button
        type="button"
        onclick={undo}
        disabled={(!dirty && !childUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady}
        class="px-3 py-1 rounded text-xs font-medium border border-stone-300 bg-white hover:bg-stone-50 disabled:opacity-50 flex items-center gap-1"
      >
        <RotateCcw size={14} /> Undo
      </button>

      <button
        type="button"
        onclick={save}
        disabled={(!dirty && !childUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady}
        class="px-3 py-1 rounded text-xs font-medium bg-emerald-700 hover:bg-emerald-800 text-white disabled:opacity-50 flex items-center gap-1"
      >
        <Save size={14} /> Save
      </button>

      {#if onClosed}
        <button
          type="button"
          aria-label="Close FS882 form"
          disabled={busy || headerWorkflowBusy}
          onclick={onClosed}
          class="px-2 py-1 text-stone-500 hover:text-stone-800 font-bold ml-2"
        >
          ✕
        </button>
      {/if}
    </div>
  </header>

  <!-- Banner Alerts -->
  {#if error}
    <div role="alert" class="p-2 bg-red-50 border-b border-red-200 text-red-800 text-xs flex items-center gap-2">
      <AlertTriangle size={14} class="text-red-600 shrink-0" />
      <span>{error}</span>
    </div>
  {/if}
  {#if successMsg}
    <div class="p-2 bg-emerald-50 border-b border-emerald-200 text-emerald-800 text-xs flex items-center gap-2">
      <Check size={14} class="text-emerald-600 shrink-0" />
      <span>{successMsg}</span>
    </div>
  {/if}

  <!-- Navigation Tabs -->
  <nav aria-label="FS882 sections" class="fs882-tabs flex border-b border-stone-200 bg-stone-50 px-3 text-xs font-medium">
    <button
      aria-current={activeTab === 'site' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'site' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'site'}
      disabled={headerWorkflowBusy}
    >
      Site & Location
    </button>
    <button
      aria-current={activeTab === 'veg' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'veg' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'veg'}
      disabled={headerWorkflowBusy}
    >
      Vegetation
    </button>
    <button
      aria-current={activeTab === 'vegOther' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'vegOther' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'vegOther'}
      disabled={headerWorkflowBusy}
    >
      Veg Other
    </button>
    <button
      aria-current={activeTab === 'soils' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'soils' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'soils'}
      disabled={headerWorkflowBusy}
    >
      Soils (Humus & Mineral)
    </button>
    <button
      aria-current={activeTab === 'other' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'other' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'other'}
      disabled={headerWorkflowBusy}
    >
      Other Data
    </button>
    <button
      aria-current={activeTab === 'audit' ? 'page' : undefined}
      class="px-4 py-2 border-b-2 {activeTab === 'audit' ? 'border-emerald-600 text-emerald-800 font-bold bg-white' : 'border-transparent text-stone-600 hover:text-stone-900'}"
      onclick={() => activeTab = 'audit'}
      disabled={headerWorkflowBusy}
    >
      Audit Trail
    </button>
  </nav>
  {#if siviParentReviewEnabled}
    <div class="border-b border-stone-200 px-4 py-2">
      <button type="button" data-sivi-parent-open aria-pressed={activeTab === 'siviParent'}
        class="rounded border px-3 py-1 text-sm disabled:opacity-50"
        disabled={busy || headerWorkflowBusy || dirty || nonParentChildUnsaved || !siviParentSession}
        onclick={() => {
          activeTab = 'siviParent';
          siviParentOpen = true;
          if (siviParentSession && !siviParentView?.original) void siviParentSession.load();
          if (!siviParentWriteUnsaved && !siviParentActionUnsaved && !siviProjectAssignmentUnsaved && !siviParentSharedUnsaved && siviParentSourceSession && !siviParentSourceView?.choices) void siviParentSourceSession.load();
        }}>SIVI parent (read-only)</button>
    </div>
  {/if}

  <!-- Tab Contents -->
  <div class="fs882-body overflow-y-auto flex-1">
    {#snippet sharedSIVIFields()}
      {#if siviParentSharedView}
        <SIVIParentSharedFields view={siviParentSharedView} disabled={siviParentSharedEditingDisabled}
          canSave={siviParentSharedClose?.canSave ?? false} onstage={stageSIVIParentShared}
          onoperation={operation => { void siviParentSharedOperation(operation); }}
          oncancel={() => siviParentSharedSession?.cancel()} />
      {/if}
    {/snippet}
    {#if activeTab === 'siviParent' && siviParentOpen && siviParentView && !metadataOpen}
      <SIVIParentReadPanel view={siviParentView} sourceView={siviParentSourceView}
        sharedEditor={siviParentSharedView ? {
          liveColumns: siviParentSharedView.original ? siviParentSharedColumns : [],
          content: sharedSIVIFields, busy: siviParentSharedView.busy,
          unsaved: siviParentSharedUnsaved || (siviParentSharedClose?.blocked ?? false),
        } : null}
        onMetadata={metadataEnabled ? openProjectMetadata : undefined}
        metadataDisabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
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
    {/if}
    {#if environmentSUOpen && $projectState?.projectPath}
      <EnvironmentSUTransfer bind:this={environmentSUEditor} client={PlotService} {contextId}
        project={$projectState.activeProject} su={$projectState.activeSU} path={$projectState.projectPath}
        direction={environmentSUDirection}
        onclosed={() => { environmentSUOpen = false; error = null; if (environmentSUDirection === 'reverse') void load(original?.plotNumber); }} />
    {/if}
    {#if profileReviewOpen}
      <ProjectPlotProfileReview bind:this={profileEditor} client={PlotService} allowRun={profileRunEnabled}
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
    {#if metadataOpen}
      <ProjectMetadataEditor bind:this={metadataEditor} client={PlotService} {contextId} plot={draft.plotNumber}
        allowCreation={metadataCreationEnabled} allowTemplateCreation={metadataTemplateCreationEnabled}
        allowRestoration={metadataRestorationEnabled}
        onbusy={value => metadataBusy = value}
        onclosed={() => { metadataOpen = false; error = null; }}
        oncommitted={refreshProjectMetadata} />
    {/if}
    {#if codeCheckOpen}
      <SpeciesCodeCheck bind:this={codeCheckEditor} client={PlotService}
        onbusy={value => codeCheckBusy = value}
        onclosed={() => { codeCheckOpen = false; error = null; }}
        oncommitted={() => loadChildData(draft.plotNumber)} />
    {/if}
    {#if speciesUnsaved}
      <div class="species-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Species draft controls">
        {#if savedPersonalCodes.length}<p role="status">Saved personal definitions: {savedPersonalCodes.join(', ')}. Plot Undo does not remove these user-database records.</p>{/if}
        <span>Species drafts are unsubmitted. Switching tabs or cover/height views never saves them; other editing waits.</span>
        {#each speciesNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={speciesEditingDisabled || speciesInvalid.length > 0} onclick={() => void saveSpeciesDrafts()}>Save species drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null} onclick={cancelSpeciesDrafts}>Cancel species drafts</button>
        </div>
        {#each speciesInvalid as message}<p role="alert">{message}</p>{/each}
        {#if speciesDecisionBusy}<p role="status">Looking up source species choices...</p>{/if}
        {#each Object.entries(speciesDrafts) as [id, cell] (id)}
          {#if cell.error}
            {@const choices = speciesChoices[id]}
            <section class="mt-2 border-t border-amber-300 pt-2" aria-label={`Species choices, row ${id}`} data-species-choice-row={id}>
              <p>Row {id}: entered code "{cell.raw}" ({cell.form}). No assignment is saved by reviewing choices.</p>
              <button type="button" class="mt-1 px-2 py-1 border rounded bg-white" disabled={speciesEditingDisabled}
                onclick={() => void reviewSpeciesChoices(Number(id))}>Review species code, row {id}</button>
              {#if choices && choices.entered === cell.raw && choices.form === cell.form}
                {@const hasAlias = choices.aliases.some(option => option.code !== null)}
                {#each choices.aliases as option, index (index)}
                  <div class="mt-1">
                    <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                    <button type="button" class="ml-2 px-2 py-1 border rounded bg-white"
                      disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null || speciesEventError(option.code) !== null}
                      title={speciesEventError(cell.raw) ?? speciesEventError(option.code) ?? 'Stage the explicit replacement; Save remains separate'}
                      onclick={() => option.code !== null && chooseSpeciesCode(Number(id), 'replace', option.code)}>Use replacement {option.code ?? 'NULL'}, row {id}</button>
                  </div>
                {/each}
                {#if hasAlias}
                  <button type="button" class="mt-1 px-2 py-1 border rounded bg-white"
                    disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null}
                    title={speciesEventError(cell.raw) ?? 'Stage the entered code using the source UCase event'}
                    onclick={() => chooseSpeciesCode(Number(id), 'keep')}>Keep entered code, row {id}</button>
                {:else}
                  {#each choices.users as option, index (index)}
                    <div class="mt-1">
                      <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                      <button type="button" class="ml-2 px-2 py-1 border rounded bg-white"
                        disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null || speciesEventError(option.code) !== null}
                        title={speciesEventError(cell.raw) ?? speciesEventError(option.code) ?? 'Stage the existing personal-list code; no personal-list write'}
                        onclick={() => option.code !== null && chooseSpeciesCode(Number(id), 'user', option.code)}>Use personal code {option.code ?? 'NULL'}, row {id}</button>
                    </div>
                  {:else}
                    {#if personalSpeciesEnabled}
                      <button type="button" class="mt-2 rounded border bg-white px-2 py-1" data-personal-create-row={id}
                        disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null}
                        onclick={() => startPersonalSpecies(Number(id))}>Create personal definition, row {id}</button>
                    {:else}
                      <p role="alert">No source replacement or existing personal code was found. Personal-list creation is unavailable; correct or Cancel the draft.</p>
                    {/if}
                  {/each}
                {/if}
                {#if speciesEventError(cell.raw)}<p role="alert">{speciesEventError(cell.raw)}</p>{/if}
              {/if}
            </section>
          {/if}
        {/each}
      </div>
    {/if}
    {#if personalDraft}
      <section class="personal-species-draft mb-3 rounded border border-amber-300 bg-amber-50 p-3 text-xs" aria-label="New personal species definition"
        data-personal-row-id={personalDraft.source.kind === 'existing' ? personalDraft.source.id : undefined} data-personal-editor={personalDraft.source.kind}>
        <h3 class="mb-2 font-semibold">New personal species definition - not yet saved</h3>
        <p class="mb-2" role="status">Saving here creates a reusable user-database definition and audit only. The plot assignment or new-row proposal remains unsaved. A later plot Undo does not delete a saved definition.</p>
        {#each personalInvalid as message}<p class="mb-2 text-red-700" role="alert">{message}</p>{/each}
        <PersonalSpeciesFields draft={personalDraft} disabled={busy || headerWorkflowBusy}
          onchange={value => { personalDraft = value; error = null; successMsg = null; }} />
        <div class="flex gap-2">
          <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalInvalid.length > 0}
            onclick={() => void savePersonalSpecies()}>Save personal definition only</button>
          <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy}
            onclick={cancelPersonalSpecies}>Cancel personal definition entry</button>
        </div>
        <p class="mt-2">Names preserve case and spacing, with a 255-UTF-16-unit bound. NULL and empty text remain distinct. Report, SppNumber and Codetype retain their original user-table defaults; no U/X classification is inferred.</p>
      </section>
    {/if}
    {#if collectedUnsaved}
      <div class="collected-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Collected draft controls">
        <span>Collected drafts are unsubmitted. Switching tabs or cover/height views never saves them; other editing waits.</span>
        {#each collectedNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={collectedEditingDisabled} onclick={() => void saveCollectedDrafts()}>Save Collected drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelCollectedDrafts}>Cancel Collected drafts</button>
        </div>
      </div>
    {/if}
    {#if attributeUnsaved}
      <div class="attribute-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Vegetation attribute draft controls">
        <span>Vegetation attribute drafts are unsubmitted. Switching tabs never saves them; other editing waits.</span>
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={attributeEditingDisabled || attributeInvalid.length > 0} onclick={() => void saveAttributeDrafts()}>Save vegetation attributes</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelAttributeDrafts}>Cancel vegetation attributes</button>
        </div>
        {#each attributeInvalid as message}<p role="alert">{message}</p>{/each}
      </div>
    {/if}
    {#if soilUnsaved}
      <div class="soil-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Soil draft controls">
        <span>Humus/Mineral drafts are unsubmitted. Switching tabs never saves them; other editing waits.</span>
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={soilEditingDisabled || soilInvalid.length > 0} onclick={() => void saveSoilDrafts()}>Save soil drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelSoilDrafts}>Cancel soil drafts</button>
        </div>
        {#each soilInvalid as message}<p role="alert">{message}</p>{/each}
      </div>
    {/if}
    {#if otherUnsaved}
      <div class="other-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Other draft controls">
        <span>Other drafts are unsubmitted. Switching tabs never saves them; header and other child editing wait.</span>
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={otherEditingDisabled || otherInvalid.length > 0} onclick={() => void saveOtherDrafts()}>Save Other drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelOtherDrafts}>Cancel Other drafts</button>
        </div>
        {#each otherInvalid as message}<p role="alert">{message}</p>{/each}
      </div>
    {/if}
    {#if creationDraft}
      <section class="vegetation-creation-draft mb-3 rounded border border-amber-300 bg-amber-50 p-3 text-xs" aria-label="New vegetation record">
        <h3 class="mb-2 font-semibold">New vegetation record - {creationDraft.form} - not yet saved</h3>
        {#if savedPersonalCodes.length}<p class="mb-2" role="status">Saved personal definitions: {savedPersonalCodes.join(', ')}. Cancelling this new-row proposal never removes these user-database records.</p>{/if}
        {#if creationInvalid.length}<p class="mb-2 text-red-700" role="alert">{creationInvalid[0]}</p>{/if}
        {#if creationNotice}<p class="mb-2 font-medium" role="status">{creationNotice}</p>{/if}
        <label class="mb-2 flex flex-col gap-1" for="creation-species">Species code
          <input id="creation-species" class="rounded border bg-white px-2 py-2" type="text" value={creationDraft.species} list="creation-species-list"
            disabled={busy || headerWorkflowBusy || personalDraft !== null} aria-invalid={creationSpeciesInvalid !== null}
            oninput={event => stageCreationSpecies(event.currentTarget.value)} />
        </label>
        <datalist id="creation-species-list">
          {#each speciesLists[creationDraft.form] ?? [] as option, index (index)}
            {#if option.code !== null}<option value={option.code}>{option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'}</option>{/if}
          {/each}
        </datalist>
        {#if creationDraft.decision}
          <p class="mb-2" role="status">Explicit {creationDraft.decision.kind} decision for "{creationDraft.decision.entered}" stages "{creationDraft.species}". No new row or personal definition is saved yet.</p>
        {:else if creationSpeciesInvalid}
          <section class="mb-3 border-t border-amber-300 pt-2" aria-label="New vegetation species choices">
            <button type="button" class="rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || !speciesReferenceReady || speciesReferenceBusy || speciesEventError(creationDraft.species) !== null}
              title={speciesEventError(creationDraft.species) ?? 'Review source definitions without saving'}
              onclick={() => void reviewCreationSpeciesChoices()}>Review new vegetation species code</button>
            {#if speciesDecisionBusy}<p role="status">Looking up source species choices...</p>{/if}
            {#if creationChoices && creationChoices.form === creationDraft.form && creationChoices.entered === creationDraft.species}
              {@const hasAlias = creationChoices.aliases.some(option => option.code !== null)}
              {#each creationChoices.aliases as option, index (index)}
                <div class="mt-1">
                  <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                  <button type="button" class="ml-2 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(option.code) !== null}
                    title={speciesEventError(option.code) ?? 'Stage explicit replacement; Save remains separate'}
                    onclick={() => option.code !== null && chooseCreationSpecies('replace', option.code)}>Use replacement {option.code ?? 'NULL'} for new row</button>
                </div>
              {/each}
              {#if hasAlias}
                <button type="button" class="mt-1 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null}
                  onclick={() => chooseCreationSpecies('keep')}>Keep entered code for new row</button>
              {:else}
                {#each creationChoices.users as option, index (index)}
                  <div class="mt-1">
                    <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                    <button type="button" class="ml-2 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(option.code) !== null}
                      title={speciesEventError(option.code) ?? 'Stage an existing personal code; no user-database write'}
                      onclick={() => option.code !== null && chooseCreationSpecies('user', option.code)}>Use personal code {option.code ?? 'NULL'} for new row</button>
                  </div>
                {:else}
                  {#if personalSpeciesEnabled}
                    <button type="button" class="mt-2 rounded border bg-white px-2 py-1" data-personal-create-source={creationDraft.form}
                      disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(creationDraft.species) !== null}
                      onclick={startCreationPersonalSpecies}>Create personal definition for new row</button>
                  {:else}
                    <p role="alert">No source replacement or existing personal code was found. Personal-list creation is unavailable; correct or Cancel this proposal. Personal definitions saved separately remain reusable.</p>
                  {/if}
                {/each}
              {/if}
            {/if}
          </section>
        {/if}
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {#each Object.entries(creationDraft.cells) as [column, cell] (column)}
            {@const control = paperChild(creationDraft.form).controls.find(control => control.column?.toLowerCase() === column.toLowerCase())}
            <label class="flex flex-col gap-1" for={`creation-${column}`}>{control ? controlLabel(control) : column}
              <input id={`creation-${column}`} class="rounded border bg-white px-2 py-2" type="text" inputmode="decimal" value={cell?.raw ?? ''}
                disabled={busy || headerWorkflowBusy || personalDraft !== null} aria-invalid={cell?.error != null} title={cell?.error ?? 'Blank is NULL; no zero cover is inferred.'}
                oninput={event => stageCreationCell(column, event.currentTarget.value)} />
            </label>
          {/each}
        </div>
        <div class="mt-3 flex gap-2">
          <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null || creationInvalid.length > 0 || !speciesReferenceReady || speciesReferenceBusy} onclick={() => void saveVegetationCreation()}>Save new vegetation record</button>
          <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null} onclick={() => void cancelVegetationCreation()}>Cancel vegetation creation</button>
        </div>
        <p class="mt-2">Select an exact canonical code or explicitly review replacement, keep or existing personal-code decisions. Enter source numeric values explicitly. Blank fields remain NULL; Layer, covers, totals and personal metadata are not inferred. Identity is allocated only by the transactional writer; saving a new row never writes the user database.</p>
      </section>
    {/if}
    {#if deletionReview}
      <section class="vegetation-deletion-review mb-3 rounded border border-red-300 bg-red-50 p-3 text-xs" aria-label="Vegetation deletion review">
        <p role="alert">Delete the entire vegetation record for {deletionReview.species ?? 'NULL'}, row {deletionReview.id}, plot {draft.plotNumber}? This removes every stored field from all cover/height/attribute views, not just {deletionReview.form}. The deleted identity remains reserved. Ordinary Save never confirms this action.</p>
        <details class="mt-2"><summary>Stored columns being removed</summary><p>{deletionReview.columns?.join(', ')}</p></details>
        <div class="mt-2 flex gap-2">
          <button type="button" class="rounded border bg-red-700 px-2 py-1 text-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void confirmVegetationDeletion()}>Confirm deletion of row {deletionReview.id}</button>
          <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelVegetationDeletion()}>Cancel deletion review</button>
        </div>
      </section>
    {/if}
    {#if heightUnsaved}
      <div class="height-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Height draft controls">
        <span>Height/cover drafts are unsubmitted. Switching views never saves them; header and other child editing wait.</span>
        {#each heightNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={heightEditingDisabled || heightInvalid.length > 0} onclick={() => void saveHeightDrafts()}>Save height/cover drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelHeightDrafts}>Cancel height/cover drafts</button>
        </div>
        {#each heightInvalid as message}<p role="alert">{message}</p>{/each}
        {#if Object.values(heightDrafts).some(row => row.cover6?.value === null && row.cover6?.expected !== null)}
          <p>Saving NULL Cover6 removes that row from the C-height view after refresh, but does not delete the vegetation record.</p>
        {/if}
      </div>
    {/if}
    {#if activeTab === 'site'}
      {#key headerRevision}
        <ParentCodeFields bind:draft {original} {capabilities} scope="site"
          disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
          onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
        {#snippet children(editor)}
        <OrdinaryFields bind:draft {original} {capabilities} scope="site"
          disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
          onchange={markDirty} onvalidation={validateHeader}>
        {#snippet children(ordinaryEditor)}
        <HeaderEditor bind:draft {original} {capabilities} disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
          onMetadata={metadataEnabled ? openProjectMetadata : undefined} metadataDisabled={childParentDisabled || childUnsaved || Object.keys(headerValidation).length > 0}
          {editor} additionalEditor={ordinaryEditor} {masterAllowed}
          existing={original !== null}
          lists={{ moistureRegime: moistureList, nutrientRegime: nutrientList, mesoSlopePos: mesoSlopeList, surfaceShape: surfaceShapeList }}
          onchange={markDirty} onerror={(message) => error = message} onvalidation={validateHeader}
          onCoordinateBusyChange={(pending) => coordinateBusy = pending}
          onWorkingUnitBusyChange={(pending) => workingUnitBusy = pending}
          onQualityBusyChange={(pending) => qualityBusy = pending}
          onSiteCodeBusyChange={(pending) => siteCodeBusy = pending}
          onRegionCodeBusyChange={(pending) => regionCodeBusy = pending} {workingUnitSession} />
        {/snippet}
        </OrdinaryFields>
        {/snippet}
        </ParentCodeFields>
      {/key}
    {:else if activeTab === 'veg'}
      {#if siviHeightEnabled}
        <button type="button" class="mb-3 rounded border px-3 py-1 disabled:opacity-50" data-sivi-open
          disabled={busy || headerWorkflowBusy || !siviPanelOpen && siviEditingDisabled && !siviUnsaved}
          onclick={() => void showSIVIPanel()}>{siviPanelOpen ? 'Return to FS882 vegetation' : 'Show SIVI aggregate heights (height-only)'}</button>
      {/if}
      {#if extendedShrubsEnabled}
        <label class="mb-3 flex items-center gap-2">
          <input type="checkbox" data-extended-shrubs checked={extendedShrubs} disabled={busy || headerWorkflowBusy}
            onchange={(event) => { extendedShrubs = event.currentTarget.checked; siviSession?.presentation(extendedShrubs); if (extendedShrubs) vegetationMode = 'cover'; }} />
          Show extended shrub layers (CHARS)
        </label>
        {#if extendedShrubs && !siviPanelOpen}
          <p class="mb-3 text-sm" role="status">Extended shrubs use cover mode. Turn this option off to use height mode; hidden values, drafts and errors are retained.</p>
        {/if}
      {/if}
      {#if siviPanelOpen && siviView}
        <SIVIHeightPanel view={siviView} disabled={siviEditingDisabled} canSave={siviClose?.canSave ?? false}
          onstage={stageSIVICell} onsave={() => void siviOperation('save')} onundo={() => void siviOperation('undo')}
          onreload={() => void siviOperation('load')} onrestore={(action) => void siviOperation(action === AuditRestoreAction.AuditRestoreRetain ? 'retain' : 'prune')} />
      {:else}
      {#if speciesEditingEnabled || creationEnabled}
        {#if speciesReferenceBusy}<p role="status">Loading source species lists...</p>{/if}
        {#if speciesReferenceError}<p role="alert">{speciesReferenceError}</p>{/if}
        <button type="button" class="mb-2 px-2 py-1 border rounded" disabled={busy || speciesReferenceBusy || headerWorkflowBusy}
          onclick={() => void loadSpeciesReferences()}>{speciesReferenceError ? 'Retry species references' : 'Reload species references'}</button>
      {/if}
      <OrdinaryFields bind:draft {original} {capabilities} scope="veg"
        disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
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
        <p class="mt-2 text-xs text-stone-500">{numberEditingEnabled ? 'Height/cover cells use explicit drafts and atomic Save/Cancel. No guessed height units; float64 precision is preserved within the source Single storage domain.' : 'Cover/height editing is read-only in this build.'} Switching view does not save or change data.</p>
      {/if}
      {#if collectedEditingEnabled}
        <p class="mt-2 text-xs text-stone-500">Collected buttons cycle NULL -> C -> V -> NULL using persistent drafts. Other historical values remain unchanged, as in the source click event. Use Save or Cancel; {creationEnabled ? 'new rows use a separate explicit source creation draft.' : 'new-row workflows remain unavailable in the source grids.'}</p>
      {/if}
      {#if numberEditingEnabled}
        <p class="mt-2 text-xs text-stone-500">All source cover/total and height fields share persistent drafts. Switching grids retains raw errors and original values; Save checks each field's original source row and commits changes/audits atomically. Clearing cover may remove a row from a source view, never delete the vegetation record.</p>
      {/if}
      <p class="mt-2 text-xs text-stone-500">{speciesEditingEnabled ? `Species selection uses canonical source lists and explicit old-code replace/keep or existing personal-code decisions. Decisions stage the source UCase event; Save remains separate. ${personalSpeciesEnabled ? 'Unknown-code personal definitions use a separate explicit user-database save; plot Save and Undo remain independent.' : 'Personal-list creation remains unavailable.'}` : 'Species editing remains read-only pending native verification.'}</p>
      <details class="mt-4 border border-stone-200 rounded p-3">
        <summary class="text-xs font-semibold cursor-pointer">Vegetation record preview (read-only)</summary>
      <div class="space-y-4">
        <!-- Live Stratum Cover Summaries -->
        <div class="grid grid-cols-4 gap-3 bg-stone-50 border border-stone-200 rounded p-3 text-xs">
          <div class="flex flex-col">
            <span class="text-stone-500 font-medium">Tree Layer (A)</span>
            <span class="font-bold text-stone-900 text-sm">{totalTreeCover.toFixed(1)}%</span>
          </div>
          <div class="flex flex-col">
            <span class="text-stone-500 font-medium">Shrub Layer (B)</span>
            <span class="font-bold text-stone-900 text-sm">{totalShrubCover.toFixed(1)}%</span>
          </div>
          <div class="flex flex-col">
            <span class="text-stone-500 font-medium">Herb Layer (C)</span>
            <span class="font-bold text-stone-900 text-sm">{totalHerbCover.toFixed(1)}%</span>
          </div>
          <div class="flex flex-col">
            <span class="text-stone-500 font-medium">Moss/Lichen (D)</span>
            <span class="font-bold text-stone-900 text-sm">{totalMossCover.toFixed(1)}%</span>
          </div>
        </div>

        <div class="flex justify-between items-center">
          <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Vegetation Entries ({vegList.length})</h3>
          <button
            type="button"
            onclick={() => showSpeciesModal = true}
            disabled
            class="px-2.5 py-1 bg-emerald-700 hover:bg-emerald-800 text-white rounded text-xs flex items-center gap-1"
          >
            <Plus size={14} /> Add Species (BC Catalog)
          </button>
        </div>

        <div class="border border-stone-200 rounded overflow-x-auto bg-white">
          <table class="w-full text-left text-xs">
            <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
              <tr>
                <th class="p-2">Species</th>
                <th class="p-2">Layer</th>
                <th class="p-2">Cover 1</th>
                <th class="p-2">Cover 2</th>
                <th class="p-2">Total A</th>
                <th class="p-2">Cover 4</th>
                <th class="p-2">Cover 5</th>
                <th class="p-2">Total B</th>
                <th class="p-2">Cover 6</th>
                <th class="p-2">Cover 7</th>
                <th class="p-2">Action</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-stone-100">
              {#each vegList as row (row.id)}
                <tr class="hover:bg-stone-50">
                  <td class="p-2 font-mono font-bold text-emerald-950">{row.species}</td>
                  <td class="p-2">
                    <input
                      type="text"
                      bind:value={row.layer}
                      disabled
                      class="w-10 border border-stone-200 rounded px-1 py-0.5 text-xs text-center"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover1}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover2}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2 font-bold text-stone-800 text-right">{row.totalA?.toFixed(1) ?? '—'}</td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover4}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover5}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2 font-bold text-stone-800 text-right">{row.totalB?.toFixed(1) ?? '—'}</td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover6}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover7}
                      disabled
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <button
                      type="button"
                      onclick={() => deleteVeg(row.id)}
                      disabled
                      class="text-red-600 hover:text-red-800 p-1"
                      title="Delete entry"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="11" class="p-4 text-center text-stone-400">No vegetation records for this plot.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

      </details>
      <!-- Species Search Modal -->
      {#if showSpeciesModal}
        <div class="fixed inset-0 bg-stone-900/40 backdrop-blur-xs flex items-center justify-center z-50 p-4">
          <div class="bg-white rounded-lg shadow-xl border border-stone-200 w-full max-w-lg flex flex-col max-h-[80vh]">
            <div class="p-3 border-b border-stone-200 flex justify-between items-center bg-stone-50 rounded-t-lg">
              <h4 class="font-bold text-sm text-stone-900">BC Master Species Search</h4>
              <button
                type="button"
                onclick={() => showSpeciesModal = false}
                class="text-stone-400 hover:text-stone-700 p-1"
              >
                <X size={16} />
              </button>
            </div>
            <div class="p-3 border-b border-stone-100 flex gap-2">
              <input
                type="text"
                placeholder="Search code (e.g. ABIE) or name (e.g. fir, spruce)..."
                bind:value={speciesSearchQuery}
                oninput={searchSpecies}
                class="flex-1 border border-stone-300 rounded px-3 py-1.5 text-xs bg-white"
              />
              <button
                type="button"
                onclick={searchSpecies}
                class="px-3 py-1.5 bg-emerald-700 text-white rounded text-xs font-medium hover:bg-emerald-800"
              >
                Search
              </button>
            </div>
            <div class="p-2 overflow-y-auto flex-1 divide-y divide-stone-100">
              {#if searchingSpecies}
                <div class="p-4 text-center text-xs text-stone-400">Searching catalog...</div>
              {:else if speciesSearchResults.length > 0}
                {#each speciesSearchResults as sp}
                  <button
                    type="button"
                    class="w-full p-2 hover:bg-emerald-50 rounded flex justify-between items-center text-left"
                    onclick={() => selectSpeciesToAdd(sp)}
                  >
                    <div>
                      <div class="flex items-center gap-2">
                        <span class="font-mono font-bold text-xs text-emerald-900 bg-emerald-100 px-1.5 py-0.5 rounded">{sp.code}</span>
                        <span class="font-medium text-xs text-stone-900 italic">{sp.scientificName}</span>
                      </div>
                      {#if sp.englishName}
                        <div class="text-[11px] text-stone-500 mt-0.5">{sp.englishName}</div>
                      {/if}
                    </div>
                    <span class="px-2 py-1 bg-emerald-700 hover:bg-emerald-800 text-white text-[11px] rounded font-medium">
                      Select
                    </span>
                  </button>
                {/each}
              {:else if speciesSearchQuery.trim()}
                <div class="p-4 text-center text-xs text-stone-400">No matching species found.</div>
              {:else}
                <div class="p-4 text-center text-xs text-stone-400">Type a code or species name above to search 10,500+ BC flora.</div>
              {/if}
            </div>
          </div>
        </div>
      {/if}
    {:else if activeTab === 'soils'}
      {#if soilEditingEnabled}
        {#if soilReferenceBusy}<p role="status">Loading soil reference suggestions...</p>{/if}
        {#if soilReferenceError}
          <p role="alert">{soilReferenceError}</p>
          <button type="button" disabled={busy || soilReferenceBusy || headerWorkflowBusy} onclick={() => void loadSoilSuggestions()}>Retry soil references</button>
        {/if}
      {/if}
      {#if !soilCodeEditingEnabled}<SoilCodeReference disabled={busy || headerWorkflowBusy} />{/if}
      <div class="flex gap-2 mb-3 text-xs">
        <button disabled={soilEditingDisabled || soilUnsaved} onclick={() => openNewChild('humus')}>Add Humus Layer</button>
        <button disabled={soilEditingDisabled || soilUnsaved} onclick={() => openNewChild('mineral')}>Add Mineral Layer</button>
      </div>
      <OrdinaryFields bind:draft {original} {capabilities} scope="soils"
        disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
        onchange={markDirty} onvalidation={validateHeader}>
      {#snippet children(ordinaryEditor)}
      <DrainageFields bind:draft {original} {capabilities}
        disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
        onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => drainageBusy = pending}>
      {#snippet children(drainageEditor)}
      <SoilCodeFields bind:draft {original} {capabilities}
        disabled={draft.locked || busy || !capabilitiesReady || childUnsaved || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || geologyCodeBusy}
        onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => soilCodeBusy = pending}>
        {#snippet children(editor)}
          <GeologyCodeFields bind:draft {original} {capabilities}
            disabled={draft.locked || busy || !capabilitiesReady || childUnsaved || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy}
            onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => geologyCodeBusy = pending}>
            {#snippet children(geologyEditor)}
              <ParentCodeFields bind:draft {original} {capabilities} scope="soils"
                disabled={draft.locked || busy || !capabilitiesReady || childUnsaved}
                onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
              {#snippet children(parentEditor)}
              <SourcePage name="Soil/Terrain" values={sourceHeaderValues} {editor} editors={[...(geologyEditor ? [geologyEditor] : []), ...(parentEditor ? [parentEditor] : []), ...(ordinaryEditor ? [ordinaryEditor] : []), ...(drainageEditor ? [drainageEditor] : [])]}>
                {#snippet embedded(control)}
                  {@const child = embeddedForm(control.controlId)}
                  <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={soilEditingDisabled}
                    deleteDisabled={soilUnsaved} onsoilstage={soilEditingEnabled ? stageSoilCell : undefined}
                    {soilDrafts} {soilSuggestions} ondelete={deleteSourceChild} />
                {/snippet}
              </SourcePage>
              {/snippet}
              </ParentCodeFields>
            {/snippet}
          </GeologyCodeFields>
        {/snippet}
      </SoilCodeFields>
      {/snippet}
      </DrainageFields>
      {/snippet}
      </OrdinaryFields>
      {#if soilEditingEnabled}
        <div class="mt-3 text-xs text-stone-600">
          <p>Suggestions are optional. Raw text is never completed or case-changed automatically; no soil totals are guessed.</p>
          <button type="button" class="mt-2 px-2 py-1 border rounded" disabled={busy || soilReferenceBusy || headerWorkflowBusy}
            onclick={() => void loadSoilSuggestions()}>Reload soil references</button>
        </div>
      {/if}
      <details class="mt-4 border border-stone-200 rounded p-3">
        <summary class="text-xs font-semibold cursor-pointer">Experimental soil record actions</summary>
      <div class="space-y-6">
        <!-- Humus Section -->
        <div class="space-y-3">
          <div class="flex justify-between items-center">
            <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Humus Layers ({humusList.length})</h3>
            <button
              type="button"
              onclick={() => openNewChild('humus')}
              disabled={soilEditingDisabled || soilUnsaved}
              class="px-2.5 py-1 bg-stone-700 hover:bg-stone-800 text-white rounded text-xs flex items-center gap-1"
            >
              <Plus size={14} /> Add Humus Layer
            </button>
          </div>
          <div class="border border-stone-200 rounded overflow-x-auto bg-white">
            <table class="w-full text-left text-xs">
              <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
                <tr>
                  <th class="p-2">Horizon</th>
                  <th class="p-2">Upper Depth</th>
                  <th class="p-2">Lower Depth</th>
                  <th class="p-2">pH</th>
                  <th class="p-2">Comment</th>
                  <th class="p-2">Action</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-stone-100">
                {#each humusList as h (h.id)}
                  <tr class="hover:bg-stone-50">
                    <td class="p-2 font-semibold">{h.horizon ?? '—'}</td>
                    <td class="p-2">{h.upperDepth ?? '—'}</td>
                    <td class="p-2">{h.lowerDepth ?? '—'}</td>
                    <td class="p-2">{h.ph ?? '—'}</td>
                    <td class="p-2">{h.comment ?? '—'}</td>
                    <td class="p-2">
                      <button
                        type="button"
                        onclick={() => deleteHumus(h.id)}
                        disabled={soilEditingDisabled || soilUnsaved}
                        class="text-red-600 hover:text-red-800 p-1"
                        title="Delete humus row"
                      >
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                {:else}
                  <tr>
                    <td colspan="6" class="p-4 text-center text-stone-400">No humus layers recorded.</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>

        <!-- Mineral Section -->
        <div class="space-y-3">
          <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Mineral Layers ({mineralList.length})</h3>
          <div class="border border-stone-200 rounded overflow-x-auto bg-white">
            <table class="w-full text-left text-xs">
              <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
                <tr>
                  <th class="p-2">Horizon</th>
                  <th class="p-2">Upper Depth</th>
                  <th class="p-2">Lower Depth</th>
                  <th class="p-2">Texture</th>
                  <th class="p-2">Colour</th>
                  <th class="p-2">Comments</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-stone-100">
                {#each mineralList as m (m.id)}
                  <tr class="hover:bg-stone-50">
                    <td class="p-2 font-semibold">{m.horizon ?? '—'}</td>
                    <td class="p-2">{m.upperDepth ?? '—'}</td>
                    <td class="p-2">{m.lowerDepth ?? '—'}</td>
                    <td class="p-2">{m.texture ?? '—'}</td>
                    <td class="p-2">{m.colour ?? '—'}</td>
                    <td class="p-2">{m.comments ?? '—'}</td>
                  </tr>
                {:else}
                  <tr>
                    <td colspan="6" class="p-4 text-center text-stone-400">No mineral layers recorded.</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      </details>
    {:else if activeTab === 'vegOther'}
      {#if attributeEditingEnabled}
        {#if attributeReferenceBusy}<p role="status">Loading vegetation attribute references...</p>{/if}
        {#if attributeReferenceError}
          <p role="alert">{attributeReferenceError}</p>
          <button type="button" disabled={busy || attributeReferenceBusy || headerWorkflowBusy} onclick={() => void loadAttributeSuggestions()}>Retry attribute references</button>
        {/if}
      {:else}
        <p class="mb-3 text-xs text-stone-500">Stored vegetation attributes are read-only; their editing workflow is not enabled.</p>
      {/if}
      <SourcePage name="Veg Other" values={sourceHeaderValues}>
        {#snippet embedded(control)}
          {@const child = embeddedForm(control.controlId)}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={attributeEditingDisabled}
            onattributestage={attributeEditingEnabled ? stageAttributeCell : undefined} {attributeDrafts} {attributeSuggestions} />
        {/snippet}
      </SourcePage>
      {#if attributeEditingEnabled}
        <div class="mt-3 text-xs text-stone-600">
          <p>Suggestions retain source definitions, including duplicates. AF is free numeric entry; species and row creation remain unavailable here.</p>
          <button type="button" class="mt-2 px-2 py-1 border rounded" disabled={busy || attributeReferenceBusy || headerWorkflowBusy}
            onclick={() => void loadAttributeSuggestions()}>Reload attribute references</button>
        </div>
      {/if}
    {:else if activeTab === 'other'}
      <button class="mb-3 text-xs" disabled={otherEditingDisabled || otherUnsaved} onclick={() => openNewChild('other')}>Add Other Data</button>
      <SourcePage name="Other" values={sourceHeaderValues}>
        {#snippet embedded(control)}
          {@const child = embeddedForm(control.controlId)}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={otherEditingDisabled}
            deleteDisabled={otherUnsaved} onotherstage={otherEditingEnabled ? stageOtherCell : undefined}
            {otherDrafts} ondelete={deleteSourceChild} />
        {/snippet}
      </SourcePage>
      <div class="space-y-3">
        <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Auxiliary Data ({otherList.length})</h3>
        <div class="border border-stone-200 rounded overflow-x-auto bg-white">
          <table class="w-full text-left text-xs">
            <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
              <tr>
                <th class="p-2">Data Name</th>
                <th class="p-2">Value</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-stone-100">
              {#each otherList as o (o.id)}
                <tr class="hover:bg-stone-50">
                  <td class="p-2 font-medium">{o.dataName ?? '—'}</td>
                  <td class="p-2">{o.dataItem ?? '—'}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="2" class="p-4 text-center text-stone-400">No auxiliary data items.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else if activeTab === 'audit'}
      <AuditRestore entries={auditList} project={auditProject} plot={original?.plotNumber ?? ''}
        capabilities={{ header: capabilities, children: childCapabilities }} enabled={auditRestoreEnabled}
        disabled={auditBlocked} gateReason={auditGateReason} revision={childRevision} onrestore={restoreAudit} />
    {/if}
    {#if newChild}
      <NewChild kind={newChild} {busy} {error} oncreate={createChild} oncancel={() => newChild = null} />
    {/if}
  </div>
</div>
