<script lang="ts">
  import {
    PlotService as LegacyPlotService,
    AuditRestoreAction,
    ReferenceService,
    type FS882Header,
    type VegRecord,
    type HumusRecord,
    type MineralRecord,
    type OtherRecord,
    type AuditEntry,
    type ListItem,
    type SpeciesItem
  } from '../bindings/github.com/boostao/vpro-wails';
  import { Lock, Unlock, Save, RotateCcw, AlertTriangle, Check, Plus, Trash2, Search, X } from '@lucide/svelte';
  import HeaderEditor from './HeaderEditor.svelte';
  import SourcePage from './SourcePage.svelte';
  import SoilCodeReference from './SoilCodeReference.svelte';
  import SoilCodeFields from './SoilCodeFields.svelte';
  import GeologyCodeFields from './GeologyCodeFields.svelte';
  import ParentCodeFields from './ParentCodeFields.svelte';
  import OrdinaryFields from './OrdinaryFields.svelte';
  import SourceChild from './SourceChild.svelte';
  import NewChild from './NewChild.svelte';
  import AuditRestore, { auditRestoreReason } from './AuditRestore.svelte';
  import { projectState } from './state';
  import { embeddedForm, type VegetationMode } from './paperLayout';
  import type { EditorCloseState } from './closeLifecycle';
  import { heightField, stageHeight, heightDirty, heightErrors, heightUpdates, type HeightDrafts } from './heightEditor';
  import type { WorkingUnitSession } from './workingUnitEditor';
  import { onDestroy, untrack, tick } from 'svelte';
  import { bindContextPlots } from './contextPlots';
  import { ReadRequests } from './readRequests';

  let { plotNumber, contextId, onSaved, onClosed, onBusyChange }: { plotNumber?: string; contextId: string; onSaved?: (p: string) => void | Promise<void>; onClosed?: () => void; onBusyChange?: (busy: boolean) => void } = $props();
  const PlotService = { ...LegacyPlotService, ...bindContextPlots(untrack(() => contextId)) };
  const reads = new ReadRequests();
  const referenceReads = new ReadRequests();
  const speciesReads = new ReadRequests();
  let referenceRequest = 0;
  let speciesRequest = 0;

  let activeTab = $state<'site' | 'veg' | 'vegOther' | 'soils' | 'other' | 'audit'>('site');
  async function vegetationNotesTab() {
    activeTab = 'soils';
    await tick();
    document.querySelector<HTMLInputElement>('#header-soilSurveyor')?.focus();
  }
  let vegetationMode = $state<VegetationMode>('initial');
  let busy = $state(false);
  let coordinateBusy = $state(false);
  let workingUnitBusy = $state(false);
  let qualityBusy = $state(false);
  let siteCodeBusy = $state(false);
  let regionCodeBusy = $state(false);
  let soilCodeBusy = $state(false);
  let geologyCodeBusy = $state(false);
  let parentCodeBusy = $state(false);
  const soilCodeEditingEnabled = import.meta.env.VITE_SOIL_CODES_EDITING !== 'false';
  const headerWorkflowBusy = $derived(coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy || geologyCodeBusy || parentCodeBusy);
  const workingUnitSession: WorkingUnitSession & { plot: string } = { mode: null, plot: '' };
  const heightEditingEnabled = import.meta.env.VITE_HEIGHT_EDITING !== 'false';
  let heightDrafts = $state<HeightDrafts>({});
  const heightUnsaved = $derived(heightDirty(heightDrafts));
  const heightInvalid = $derived(heightErrors(heightDrafts));
  let error = $state<string | null>(null);
  let successMsg = $state<string | null>(null);
  let dirty = $state(false);
  let headerValidation = $state<Record<string, string>>({});
  let capabilities = $state<Record<string, boolean | undefined>>({});
  let capabilitiesReady = $state(false);
  type ChildKind = 'Veg' | 'Humus' | 'Mineral' | 'Other';
  let childCapabilities = $state<Record<ChildKind, Record<string, boolean | undefined>>>({ Veg: {}, Humus: {}, Mineral: {}, Other: {} });
  let loadRequest = 0;
  onDestroy(() => {
    loadRequest++; referenceRequest++; speciesRequest++;
    reads.cancelAll(); referenceReads.cancelAll(); speciesReads.cancelAll();
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
    error = null;
    newChild = kind;
  }

  async function deleteSourceChild(name: string, id: number) {
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
    if (['SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL', 'SubVegAhtXL', 'SubVegChtXL'].includes(name)) {
      const records = vegList.filter(row => name === 'SubVegAXL_BC'
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
  const heightEditingDisabled = $derived(!capabilitiesReady || draft.locked || busy || headerWorkflowBusy || dirty || original === null);
  const childEditingDisabled = $derived(heightEditingDisabled || heightUnsaved);
  const auditRestoreEnabled = import.meta.env.VITE_AUDIT_RESTORE !== 'false';
  const auditProject = $derived($projectState?.activeProject ?? '');
  const auditBlocked = $derived(childEditingDisabled || Object.keys(headerValidation).length > 0);
  const auditGateReason = $derived(busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
    : draft.locked ? 'Unlock the plot before restoring fields.'
    : dirty || Object.keys(headerValidation).length > 0 ? 'Save valid header changes or Undo before restoring fields.'
    : 'Save the plot before restoring fields.');

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
    const request = ++loadRequest;
    reads.cancelAll();
    if (workingUnitSession.plot !== (p ?? '')) workingUnitSession.mode = null;
    workingUnitSession.plot = p ?? '';
    busy = true;
    error = null;
    capabilitiesReady = false;
    heightDrafts = {};
    vegList = [];
    humusList = [];
    mineralList = [];
    otherList = [];
    auditList = [];
    childCapabilities = { Veg: {}, Humus: {}, Mineral: {}, Other: {} };
    try {
      const [fields, res] = await Promise.all([
        reads.track(PlotService.GetHeaderCapabilities()),
        p ? reads.track(PlotService.GetPlot(p)) : Promise.resolve(null)
      ]);
      if (request !== loadRequest) return;
      if (fields === null) throw new Error('Header capabilities were not returned.');
      capabilities = fields;
      if (p && !res) throw new Error(`Plot ${p} could not be loaded.`);
      if (p) await loadChildData(p);
      if (request !== loadRequest) return;
      if (res !== null) {
        draft = { ...res };
        original = JSON.parse(JSON.stringify(res));
        dirty = false;
        headerValidation = {};
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
    void load(plotNumber);
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

  async function updateVegCover(row: VegRecord) {
    await childOperation(() => PlotService.UpdateVegRecord(row));
  }

  function stageHeightCell(id: number, column: string, raw: string) {
    try {
      if (!heightEditingEnabled || heightEditingDisabled) throw new Error('Height editing is unavailable while loading, locked, busy or the header is unsaved.');
      const field = heightField(column);
      if (!field || childCapabilities.Veg[field] !== true) throw new Error(`Height column ${column} is unavailable in the active schema.`);
      const rows = vegList.filter(row => row.id === id && row.plotNumber === draft.plotNumber);
      if (rows.length !== 1) throw new Error('Height row identity is missing or ambiguous; reload before editing.');
      heightDrafts = stageHeight(heightDrafts, id, field, raw, rows[0][field] ?? null);
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
      if (updates.length === 0) { heightDrafts = {}; successMsg = 'No height values changed.'; return; }
      await PlotService.UpdateHeightRecords(draft.plotNumber, updates);
      committed = true;
      heightDrafts = {};
      await loadChildData(draft.plotNumber);
      successMsg = `Saved ${updates.length} height/cover row drafts atomically.`;
    } catch (cause) {
      if (committed) capabilitiesReady = false;
      error = committed ? `Height changes committed, but refresh failed. Reopen the plot before editing: ${String(cause)}`
        : `Height save failed; drafts retained: ${String(cause)}`;
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
        else if (key === 'species' || key === 'collected') {
          await childOperation(() => PlotService.UpdateVegRecord({ ...row, [key]: raw || (key === 'species' ? '' : undefined) }));
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
    if (heightUnsaved) { await saveHeightDrafts(); return; }
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
    const invalid = Object.keys(headerValidation).length > 0 || heightInvalid.length > 0;
    const invalidChild = Boolean(container?.querySelector('.source-child input:invalid'));
    const saveReason = busy || headerWorkflowBusy || !capabilitiesReady ? 'Wait for the plot to finish loading or header workflow.'
      : draft.locked ? 'Unlock the plot before saving.'
      : newChild !== null || invalidChild ? 'Finish or cancel child entry before saving and closing.'
      : invalid ? 'Correct invalid header inputs or Undo before saving.'
      : !draft.plotNumber.trim() ? 'Plot Number is required before saving.' : '';
    return { unsaved: dirty || heightUnsaved || invalid || newChild !== null || invalidChild, busy: busy || headerWorkflowBusy, canSave: saveReason === '', saveReason, error };
  }

  export async function saveForClose(): Promise<boolean> {
    const state = getCloseState();
    if (!state.canSave) {
      error = state.saveReason;
      return false;
    }
    await save();
    return !dirty && !heightUnsaved && Object.keys(headerValidation).length === 0 && error === null;
  }

  function undo() {
    if (busy || headerWorkflowBusy) {
      error = 'Wait for the current operation to finish before undoing changes.';
      return;
    }
    if (heightUnsaved) { void cancelHeightDrafts(); return; }
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
    if (heightUnsaved) {
      error = 'Save or Cancel height drafts before changing the plot lock.';
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
    if (!original || !draft.plotNumber || dirty || heightUnsaved) {
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
    try {
      await operation();
      await loadChildData(draft.plotNumber);
      return true;
    } catch (cause) {
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

<div bind:this={container} class="fs882-container bg-white rounded-lg shadow border border-stone-200 flex flex-col h-full text-stone-800 text-sm">
  <!-- Header Bar -->
  <header class="fs882-toolbar p-3 bg-stone-100 border-b border-stone-200 flex justify-between items-center">
    <div class="flex items-center gap-3">
      <h2 class="font-bold text-stone-900 text-base">FS882-6x4XL: Ecosystem Field Form</h2>
      <span class="px-2 py-0.5 rounded text-xs font-semibold {dirty || heightUnsaved ? 'bg-amber-100 text-amber-800' : 'bg-stone-200 text-stone-600'}">
        {dirty || heightUnsaved ? 'Draft Unsaved' : 'Clean'}
      </span>
      {#if draft.locked}
        <span class="px-2 py-0.5 rounded text-xs font-semibold bg-red-100 text-red-700 flex items-center gap-1">
          <Lock size={12} /> Locked (Read-Only)
        </span>
      {/if}
    </div>

    <div class="fs882-actions flex items-center gap-2">
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
        disabled={(!dirty && !heightUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady}
        class="px-3 py-1 rounded text-xs font-medium border border-stone-300 bg-white hover:bg-stone-50 disabled:opacity-50 flex items-center gap-1"
      >
        <RotateCcw size={14} /> Undo
      </button>

      <button
        type="button"
        onclick={save}
        disabled={(!dirty && !heightUnsaved) || draft.locked || busy || headerWorkflowBusy || !capabilitiesReady}
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

  <!-- Tab Contents -->
  <div class="fs882-body overflow-y-auto flex-1">
    {#if heightUnsaved}
      <div class="height-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Height draft controls">
        <span>Height/cover drafts are unsubmitted. Switching views never saves them; header and other child editing wait.</span>
        <div class="flex gap-2 mt-2">
          <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={heightEditingDisabled || heightInvalid.length > 0} onclick={() => void saveHeightDrafts()}>Save height drafts</button>
          <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={cancelHeightDrafts}>Cancel height drafts</button>
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
          disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
          onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
        {#snippet children(editor)}
        <OrdinaryFields bind:draft {original} {capabilities} scope="site"
          disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
          onchange={markDirty} onvalidation={validateHeader}>
        {#snippet children(ordinaryEditor)}
        <HeaderEditor bind:draft {original} {capabilities} disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
          {editor} additionalEditor={ordinaryEditor}
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
      <OrdinaryFields bind:draft {original} {capabilities} scope="veg"
        disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
        onchange={markDirty} onvalidation={validateHeader} onVegNotesTab={vegetationNotesTab}>
      {#snippet children(ordinaryEditor)}
      <SourcePage name="Vegetation" values={sourceHeaderValues} {vegetationMode} editor={ordinaryEditor} onHeightToggle={() => vegetationMode = vegetationMode === 'height' ? 'cover' : 'height'}>
        {#snippet embedded(control)}
          {@const child = embeddedForm(control.controlId)}
          {@const heightGrid = child.form === 'SubVegAhtXL' || child.form === 'SubVegChtXL'}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={heightGrid ? heightEditingDisabled : childEditingDisabled}
            drafts={heightDrafts} onstage={heightGrid && heightEditingEnabled ? stageHeightCell : undefined}
            onedit={heightGrid ? undefined : editSourceChild} ondelete={heightGrid ? undefined : deleteSourceChild} />
        {/snippet}
      </SourcePage>
      {/snippet}
      </OrdinaryFields>
      {#if vegetationMode === 'height'}
        <p class="mt-2 text-xs text-stone-500">{heightEditingEnabled ? 'Height/cover cells use explicit drafts and atomic Save/Cancel. No guessed height units; float64 precision is preserved within the source Single storage domain. Species and collected workflows remain read-only here.' : 'Cover/height editing is gated until native Wails verification.'} Switching view does not save or change data.</p>
      {/if}
      <details class="mt-4 border border-stone-200 rounded p-3">
        <summary class="text-xs font-semibold cursor-pointer">Experimental vegetation editing grid</summary>
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
            disabled={childEditingDisabled || ['species', 'layer', 'cover1', 'totalA'].some(key => childCapabilities.Veg[key] !== true)}
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
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.layer !== true}
                      class="w-10 border border-stone-200 rounded px-1 py-0.5 text-xs text-center"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover1}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover1 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover2}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover2 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2 font-bold text-stone-800 text-right">{row.totalA?.toFixed(1) ?? '—'}</td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover4}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover4 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover5}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover5 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2 font-bold text-stone-800 text-right">{row.totalB?.toFixed(1) ?? '—'}</td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover6}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover6 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <input
                      type="number"
                      step="0.1"
                      bind:value={row.cover7}
                      onchange={() => updateVegCover(row)}
                      disabled={childEditingDisabled || childCapabilities.Veg.cover7 !== true}
                      class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right"
                    />
                  </td>
                  <td class="p-2">
                    <button
                      type="button"
                      onclick={() => deleteVeg(row.id)}
                      disabled={childEditingDisabled}
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
      {#if !soilCodeEditingEnabled}<SoilCodeReference disabled={busy || headerWorkflowBusy} />{/if}
      <div class="flex gap-2 mb-3 text-xs">
        <button disabled={childEditingDisabled} onclick={() => openNewChild('humus')}>Add Humus Layer</button>
        <button disabled={childEditingDisabled} onclick={() => openNewChild('mineral')}>Add Mineral Layer</button>
      </div>
      <OrdinaryFields bind:draft {original} {capabilities} scope="soils"
        disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
        onchange={markDirty} onvalidation={validateHeader}>
      {#snippet children(ordinaryEditor)}
      <SoilCodeFields bind:draft {original} {capabilities}
        disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || geologyCodeBusy}
        onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => soilCodeBusy = pending}>
        {#snippet children(editor)}
          <GeologyCodeFields bind:draft {original} {capabilities}
            disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy}
            onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => geologyCodeBusy = pending}>
            {#snippet children(geologyEditor)}
              <ParentCodeFields bind:draft {original} {capabilities} scope="soils"
                disabled={draft.locked || busy || !capabilitiesReady || heightUnsaved}
                onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
              {#snippet children(parentEditor)}
              <SourcePage name="Soil/Terrain" values={sourceHeaderValues} {editor} editors={[...(geologyEditor ? [geologyEditor] : []), ...(parentEditor ? [parentEditor] : []), ...(ordinaryEditor ? [ordinaryEditor] : [])]}>
                {#snippet embedded(control)}
                  {@const child = embeddedForm(control.controlId)}
                  <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={childEditingDisabled} onedit={editSourceChild} ondelete={deleteSourceChild} />
                {/snippet}
              </SourcePage>
              {/snippet}
              </ParentCodeFields>
            {/snippet}
          </GeologyCodeFields>
        {/snippet}
      </SoilCodeFields>
      {/snippet}
      </OrdinaryFields>
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
              disabled={childEditingDisabled}
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
                        disabled={childEditingDisabled}
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
      <p class="mb-3 text-xs text-stone-500">Stored vegetation attributes are read-only; their reference and editing workflows are not yet verified.</p>
      <SourcePage name="Veg Other" values={sourceHeaderValues}>
        {#snippet embedded(control)}
          {@const child = embeddedForm(control.controlId)}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} />
        {/snippet}
      </SourcePage>
    {:else if activeTab === 'other'}
      <button class="mb-3 text-xs" disabled={childEditingDisabled} onclick={() => openNewChild('other')}>Add Other Data</button>
      <SourcePage name="Other" values={sourceHeaderValues}>
        {#snippet embedded(control)}
          {@const child = embeddedForm(control.controlId)}
          <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={childEditingDisabled} onedit={editSourceChild} ondelete={deleteSourceChild} />
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
