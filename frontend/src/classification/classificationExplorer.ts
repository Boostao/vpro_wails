import { Call } from '@wailsio/runtime';
import type { CancellableRead } from '../shared/readRequests';

export type ExplorerMode = 'su_tree' | 'hierarchy_tree' | 'hierarchy_flat';

export const PHYTOSOCIOLOGICAL_RANK_COLUMNS = [
  { key: 'class', label: 'Class', level: 2 },
  { key: 'order', label: 'Order', level: 4 },
  { key: 'subOrder', label: 'SubOrder', level: 5 },
  { key: 'alliance', label: 'Alliance', level: 6 },
  { key: 'subAlliance', label: 'SubAlliance', level: 7 },
  { key: 'association', label: 'Association', level: 8 },
  { key: 'subAssociation', label: 'SubAssociation', level: 9 },
  { key: 'facies', label: 'Facies', level: 10 },
  { key: 'workingUnit', label: 'WorkingUnit', level: 10 },
  { key: 'siteSeries', label: 'SiteSeries', level: 11 },
] as const;

export type PhytosociologicalRankKey = (typeof PHYTOSOCIOLOGICAL_RANK_COLUMNS)[number]['key'];

export interface SUPlotLeaf {
  plotNumber: string;
  fieldNumber: string;
  zone: string;
  subZone: string;
  siteSeries: string;
  userSiteUnit: string;
  moistureRegime: string;
  nutrientRegime: string;
  elevation: number | null;
  siteUnit: string;
  presentInSu: boolean;
  inProjectEnv: boolean;
  locked: boolean;
}

export interface SUUnitBucket {
  siteUnit: string;
  longName: string;
  origin: string;
  maskValid: boolean;
  plotCount: number;
  plots: SUPlotLeaf[];
}

export interface SUPlotTree {
  contextId: string;
  project: string;
  projectPath: string;
  su: string;
  suPath: string;
  units: SUUnitBucket[];
  unassignedPlots: SUPlotLeaf[];
  orphanPlots: SUPlotLeaf[];
  totalProject: number;
  totalAssigned: number;
  totalUnassigned: number;
}

export interface PlotInspectorSpecies {
  species: string;
  scientificName: string;
  englishName: string;
  coverA: string;
  coverB: string;
  coverC: string;
  coverD: string;
  totalCover: number;
  totalFormatted: string;
}

export interface PlotInspectorSummary {
  contextId: string;
  plotNumber: string;
  fieldNumber: string;
  projectId: string;
  zone: string;
  subZone: string;
  siteSeries: string;
  userSiteUnit: string;
  becSiteUnit: string;
  currentSuUnit: string;
  moistureRegime: string;
  nutrientRegime: string;
  elevation: number | null;
  slope: string;
  aspect: number | null;
  latitude: number | null;
  longitude: number | null;
  topSpecies: PlotInspectorSpecies[];
}

export interface BECUnitPickerRequest {
  query: string;
  zone: string;
  subZone: string;
  limit: number;
}

export interface BECUnitPickerChoice {
  siteSeries: string;
  longName: string;
  scientificName: string;
  zone: string;
  subZone: string;
  alliance?: string;
  suballiance?: string;
  source: 'master' | 'user' | 'becdb_v13';
  maskValid: boolean;
}

export interface CreateSUTableRequest {
  name: string;
  destination: 'project' | 'new_file';
  filePath: string;
  populateAllProjectPlots: boolean;
  seedFromUserSiteUnit: boolean;
  activate: boolean;
}

export interface SaveSUTableAsRequest {
  sourceSu: string;
  newName: string;
  destination: 'project' | 'new_file';
  filePath: string;
  activate: boolean;
}

export interface SUTableLifecycleResult {
  contextId: string;
  name: string;
  tableName: string;
  filePath: string;
  plotCount: number;
  activated: boolean;
  newState?: unknown;
}

export interface AssignPlotsToSiteUnitRequest {
  plotNumbers: string[];
  targetSiteUnit: string | null;
  removeFromSuWhenUnassigned: boolean;
}

export interface AssignPlotsToSiteUnitResult {
  contextId: string;
  su: string;
  tableName: string;
  changedPlots: number;
  historyId: number;
}

export type SUFromEnvMode = 'overwrite' | 'fill_unassigned_only' | 'new_table';

export interface CreateSUFromEnvFieldsRequest {
  fields: string[];
  separator: string;
  fieldSeparators?: string[];
  useBecSlashFormat: boolean;
  mode: SUFromEnvMode;
  targetSuName?: string;
  destination?: 'project' | 'new_file';
  filePath?: string;
  scopeToExistingSuRowsOnly: boolean;
  skipIncompleteRows: boolean;
  previewOnly: boolean;
  activate: boolean;
}

export interface SUFromEnvPlotPreview {
  plotNumber: string;
  beforeSiteUnit: string;
  afterSiteUnit: string;
  action: 'insert' | 'update' | 'preserve_existing' | 'skip_blank' | 'unchanged';
}

export interface CreateSUFromEnvFieldsResult {
  contextId: string;
  mode: SUFromEnvMode;
  targetSu: string;
  tableName: string;
  filePath: string;
  previewOnly: boolean;
  insertedCount: number;
  updatedCount: number;
  preservedCount: number;
  skippedCount: number;
  unchangedCount: number;
  items: SUFromEnvPlotPreview[];
  activated: boolean;
  newState?: unknown;
}

export interface HierarchyExplorerNode {
  id: number;
  name: string;
  cleanName: string;
  parent: number | null;
  level: number;
  rankName: string;
  tag?: string;
  isBreakpoint: boolean;
  depth: number;
  children: HierarchyExplorerNode[];
}

export interface FlatHierarchyRow {
  leafId: number;
  superclass?: string;
  class: string;
  subclass?: string;
  order: string;
  subOrder: string;
  alliance: string;
  subAlliance: string;
  association: string;
  subAssociation: string;
  facies: string;
  workingUnit: string;
  siteSeries: string;
  breakpointRank?: string;
}

export interface HierarchyDiagnostics {
  totalNodes: number;
  rootNodes: number;
  breakpointNodes: number;
  blankNameNodes: number;
  orphanNodes: number[];
  cycleNodeIds: number[];
}

export interface HierarchyExplorerView {
  contextId: string;
  project: string;
  hierarchy: string;
  hierarchyPath: string;
  tableName: string;
  roots: HierarchyExplorerNode[];
  flatNodes: HierarchyExplorerNode[];
  flatRankRows: FlatHierarchyRow[];
  diagnostics: HierarchyDiagnostics;
}

export interface CreateHierarchyTableRequest {
  name: string;
  destination: 'project' | 'new_file';
  filePath: string;
  activate: boolean;
}

export interface HierarchyTableLifecycleResult {
  contextId: string;
  name: string;
  tableName: string;
  filePath: string;
  activated: boolean;
  newState?: unknown;
}

export interface HierarchyAddNodeRequest {
  name: string;
  parentId: number | null;
  level: number;
  tag?: string;
  isBreakpoint: boolean;
}

export interface HierarchyRenameNodeRequest {
  nodeId: number;
  newName: string;
}

export interface HierarchyReparentNodeRequest {
  nodeId: number;
  newParentId: number | null;
  newLevel?: number | null;
}

export interface HierarchyDeleteNodeRequest {
  nodeId: number;
  cascadeChildren: boolean;
}

export interface HierarchyMutationResult {
  contextId: string;
  hierarchy: string;
  tableName: string;
  nodeId?: number;
  nodeName?: string;
  affectedRows: number;
}

export interface SaveFlatHierarchyRequest {
  rows: FlatHierarchyRow[];
}

export interface ClassificationBackendPort {
  createSUTable(contextId: string, req: CreateSUTableRequest): Promise<SUTableLifecycleResult>;
  saveSUTableAs(contextId: string, req: SaveSUTableAsRequest): Promise<SUTableLifecycleResult>;
  getSUPlotTree(contextId: string, filterText: string): CancellableRead<SUPlotTree>;
  getPlotInspectorSummary(contextId: string, plotNumber: string): CancellableRead<PlotInspectorSummary>;
  searchBECUnitPicker(
    contextId: string,
    req: BECUnitPickerRequest
  ): CancellableRead<BECUnitPickerChoice[]>;
  assignPlotsToSiteUnit(
    contextId: string,
    req: AssignPlotsToSiteUnitRequest
  ): Promise<AssignPlotsToSiteUnitResult>;
  undoSUAssignment(contextId: string, historyId: number): Promise<AssignPlotsToSiteUnitResult>;
  createSUFromEnvFields(
    contextId: string,
    req: CreateSUFromEnvFieldsRequest
  ): Promise<CreateSUFromEnvFieldsResult>;
  getHierarchyExplorer(contextId: string): CancellableRead<HierarchyExplorerView>;
  createHierarchyTable(
    contextId: string,
    req: CreateHierarchyTableRequest
  ): Promise<HierarchyTableLifecycleResult>;
  addHierarchyNode(
    contextId: string,
    req: HierarchyAddNodeRequest
  ): Promise<HierarchyMutationResult>;
  renameHierarchyNode(
    contextId: string,
    req: HierarchyRenameNodeRequest
  ): Promise<HierarchyMutationResult>;
  reparentHierarchyNode(
    contextId: string,
    req: HierarchyReparentNodeRequest
  ): Promise<HierarchyMutationResult>;
  toggleHierarchyBreakpoint(
    contextId: string,
    nodeId: number
  ): Promise<HierarchyMutationResult>;
  deleteHierarchyNode(
    contextId: string,
    req: HierarchyDeleteNodeRequest
  ): Promise<HierarchyMutationResult>;
  saveFlatHierarchyRows(
    contextId: string,
    req: SaveFlatHierarchyRequest
  ): Promise<HierarchyMutationResult>;
}

export const defaultClassificationBackendPort: ClassificationBackendPort = {
  createSUTable: (contextId, req) =>
    Call.ByName('main.ClassificationService.CreateSUTable', contextId, req),
  saveSUTableAs: (contextId, req) =>
    Call.ByName('main.ClassificationService.SaveSUTableAs', contextId, req),
  getSUPlotTree: (contextId, filterText) =>
    Call.ByName('main.ClassificationService.GetSUPlotTree', contextId, filterText),
  getPlotInspectorSummary: (contextId, plotNumber) =>
    Call.ByName('main.ClassificationService.GetPlotInspectorSummary', contextId, plotNumber),
  searchBECUnitPicker: (contextId, req) =>
    Call.ByName('main.ClassificationService.SearchBECUnitPicker', contextId, req),
  assignPlotsToSiteUnit: (contextId, req) =>
    Call.ByName('main.ClassificationService.AssignPlotsToSiteUnit', contextId, req),
  undoSUAssignment: (contextId, historyId) =>
    Call.ByName('main.ClassificationService.UndoSUAssignment', contextId, historyId),
  createSUFromEnvFields: (contextId, req) =>
    Call.ByName('main.ClassificationService.CreateSUFromEnvFields', contextId, req),
  getHierarchyExplorer: contextId =>
    Call.ByName('main.ClassificationService.GetHierarchyExplorer', contextId),
  createHierarchyTable: (contextId, req) =>
    Call.ByName('main.ClassificationService.CreateHierarchyTable', contextId, req),
  addHierarchyNode: (contextId, req) =>
    Call.ByName('main.ClassificationService.AddHierarchyNode', contextId, req),
  renameHierarchyNode: (contextId, req) =>
    Call.ByName('main.ClassificationService.RenameHierarchyNode', contextId, req),
  reparentHierarchyNode: (contextId, req) =>
    Call.ByName('main.ClassificationService.ReparentHierarchyNode', contextId, req),
  toggleHierarchyBreakpoint: (contextId, nodeId) =>
    Call.ByName('main.ClassificationService.ToggleHierarchyBreakpoint', contextId, nodeId),
  deleteHierarchyNode: (contextId, req) =>
    Call.ByName('main.ClassificationService.DeleteHierarchyNode', contextId, req),
  saveFlatHierarchyRows: (contextId, req) =>
    Call.ByName('main.ClassificationService.SaveFlatHierarchyRows', contextId, req),
};

const BEC_UNIT_MASK_REGEX = /^[A-Z]{2,4}[a-z]{1,3}[0-9a-z]{0,3}\/[A-Za-z0-9.\-]{1,12}$/;

export function isValidBECUnitMask(code: string): boolean {
  return BEC_UNIT_MASK_REGEX.test(code.trim());
}

export interface FlattenedSUTreeItem {
  id: string;
  kind: 'unit_folder' | 'unassigned_folder' | 'plot_leaf';
  label: string;
  subtitle: string;
  level: 1 | 2;
  parentId: string | null;
  expandable: boolean;
  expanded: boolean;
  unitCode: string | null; // null represents Unassigned bucket
  plot?: SUPlotLeaf;
  count?: number;
  maskValid?: boolean;
}

export const UNASSIGNED_BUCKET_ID = 'bucket::__unassigned__';

export function flattenVisibleSUTree(
  tree: SUPlotTree | null,
  expandedIds: ReadonlySet<string>
): FlattenedSUTreeItem[] {
  if (!tree) return [];
  const items: FlattenedSUTreeItem[] = [];

  for (const bucket of tree.units) {
    const folderId = `unit::${bucket.siteUnit}`;
    const expanded = expandedIds.has(folderId);
    items.push({
      id: folderId,
      kind: 'unit_folder',
      label: bucket.siteUnit,
      subtitle: bucket.longName,
      level: 1,
      parentId: null,
      expandable: bucket.plots.length > 0,
      expanded,
      unitCode: bucket.siteUnit,
      count: bucket.plotCount,
      maskValid: bucket.maskValid,
    });
    if (expanded) {
      for (const p of bucket.plots) {
        items.push({
          id: `plot::${bucket.siteUnit}::${p.plotNumber}`,
          kind: 'plot_leaf',
          label: p.plotNumber,
          subtitle: [p.zone + p.subZone + (p.siteSeries ? `/${p.siteSeries}` : ''), p.moistureRegime ? `SMR ${p.moistureRegime}` : '']
            .filter(Boolean)
            .join(' · '),
          level: 2,
          parentId: folderId,
          expandable: false,
          expanded: false,
          unitCode: bucket.siteUnit,
          plot: p,
        });
      }
    }
  }

  const unExpanded = expandedIds.has(UNASSIGNED_BUCKET_ID);
  items.push({
    id: UNASSIGNED_BUCKET_ID,
    kind: 'unassigned_folder',
    label: 'Unassigned Plots',
    subtitle: 'Absent from SU or blank SiteUnit',
    level: 1,
    parentId: null,
    expandable: tree.unassignedPlots.length > 0,
    expanded: unExpanded,
    unitCode: null,
    count: tree.unassignedPlots.length,
  });
  if (unExpanded) {
    for (const p of tree.unassignedPlots) {
      items.push({
        id: `plot::__unassigned__::${p.plotNumber}`,
        kind: 'plot_leaf',
        label: p.plotNumber,
        subtitle: [p.zone + p.subZone + (p.siteSeries ? `/${p.siteSeries}` : ''), p.userSiteUnit]
          .filter(Boolean)
          .join(' · '),
        level: 2,
        parentId: UNASSIGNED_BUCKET_ID,
        expandable: false,
        expanded: false,
        unitCode: null,
        plot: p,
      });
    }
  }

  return items;
}

export interface FlattenedHierarchyTreeItem {
  id: string;
  nodeId: number;
  name: string;
  cleanName: string;
  parentNodeId: number | null;
  parentItemId: string | null;
  level: number;
  rankName: string;
  depth: number;
  isBreakpoint: boolean;
  expandable: boolean;
  expanded: boolean;
  childCount: number;
}

export function flattenVisibleHierarchyTree(
  roots: readonly HierarchyExplorerNode[],
  expandedNodeIds: ReadonlySet<number>
): FlattenedHierarchyTreeItem[] {
  const out: FlattenedHierarchyTreeItem[] = [];

  function walk(node: HierarchyExplorerNode, depth: number, parentItemId: string | null) {
    const itemId = `hnode::${node.id}`;
    const expandable = node.children.length > 0;
    const expanded = expandable && expandedNodeIds.has(node.id);
    out.push({
      id: itemId,
      nodeId: node.id,
      name: node.name,
      cleanName: node.cleanName,
      parentNodeId: node.parent,
      parentItemId,
      level: node.level,
      rankName: node.rankName,
      depth,
      isBreakpoint: node.isBreakpoint,
      expandable,
      expanded,
      childCount: node.children.length,
    });
    if (expanded) {
      for (const child of node.children) {
        walk(child, depth + 1, itemId);
      }
    }
  }

  for (const r of roots) {
    walk(r, 1, null);
  }
  return out;
}

export interface AriaTreeAction {
  type: 'focus' | 'expand' | 'collapse' | 'select';
  targetId: string;
}

export function resolveAriaTreeKeyAction(
  items: ReadonlyArray<{
    id: string;
    parentId: string | null;
    expandable: boolean;
    expanded: boolean;
  }>,
  focusedId: string,
  key: string
): AriaTreeAction | null {
  if (items.length === 0) return null;
  const idx = items.findIndex(i => i.id === focusedId);
  const currentIdx = idx >= 0 ? idx : 0;
  const current = items[currentIdx];

  switch (key) {
    case 'ArrowDown':
      if (currentIdx + 1 < items.length) {
        return { type: 'focus', targetId: items[currentIdx + 1].id };
      }
      return null;
    case 'ArrowUp':
      if (currentIdx - 1 >= 0) {
        return { type: 'focus', targetId: items[currentIdx - 1].id };
      }
      return null;
    case 'Home':
      return { type: 'focus', targetId: items[0].id };
    case 'End':
      return { type: 'focus', targetId: items[items.length - 1].id };
    case 'ArrowRight':
      if (current.expandable && !current.expanded) {
        return { type: 'expand', targetId: current.id };
      }
      if (current.expandable && current.expanded && currentIdx + 1 < items.length) {
        return { type: 'focus', targetId: items[currentIdx + 1].id };
      }
      return null;
    case 'ArrowLeft':
      if (current.expandable && current.expanded) {
        return { type: 'collapse', targetId: current.id };
      }
      if (current.parentId) {
        return { type: 'focus', targetId: current.parentId };
      }
      return null;
    case 'Enter':
    case ' ':
      return { type: 'select', targetId: current.id };
    default:
      return null;
  }
}

export function wouldIntroduceHierarchyCycle(
  flatNodes: ReadonlyArray<{ id: number; parent: number | null }>,
  nodeId: number,
  candidateParentId: number | null
): boolean {
  if (candidateParentId === null) return false;
  if (candidateParentId === nodeId) return true;
  const parentById = new Map<number, number | null>();
  for (const n of flatNodes) {
    parentById.set(n.id, n.parent);
  }
  const visited = new Set<number>([nodeId]);
  let curr: number | null | undefined = candidateParentId;
  while (curr !== null && curr !== undefined) {
    if (visited.has(curr)) return true;
    visited.add(curr);
    curr = parentById.get(curr) ?? null;
  }
  return false;
}

export function toggleBreakpointName(rawName: string): string {
  const trimmed = rawName.trim();
  if (trimmed.startsWith('~')) {
    return trimmed.slice(1).trim();
  }
  return `~${trimmed}`;
}

export function validateFlatHierarchyRows(rows: readonly FlatHierarchyRow[]): string {
  const parentByNode = new Map<string, string>();
  for (let i = 0; i < rows.length; i++) {
    const r = rows[i];
    const chain = [
      r.superclass,
      r.class,
      r.subclass,
      r.order,
      r.subOrder,
      r.alliance,
      r.subAlliance,
      r.association,
      r.subAssociation,
      r.facies,
      r.workingUnit,
      r.siteSeries,
    ]
      .map(v => (v ?? '').trim())
      .filter(Boolean);

    let prev = '__root__';
    const seenInRow = new Set<string>();
    for (const rawName of chain) {
      const clean = rawName.replace(/^~/, '').trim().toLowerCase();
      if (!clean) {
        return `Row ${i + 1} has an empty breakpoint node ('~').`;
      }
      if (seenInRow.has(clean)) {
        return `Row ${i + 1} repeats "${clean}" at multiple ranks (cycle).`;
      }
      seenInRow.add(clean);
      const existingParent = parentByNode.get(clean);
      if (existingParent !== undefined && existingParent !== prev) {
        return `Row ${i + 1} places "${clean}" under conflicting parents.`;
      }
      parentByNode.set(clean, prev);
      prev = clean;
    }
  }
  return '';
}

export function createEmptyFlatHierarchyRow(): FlatHierarchyRow {
  return {
    leafId: 0,
    class: '',
    order: '',
    subOrder: '',
    alliance: '',
    subAlliance: '',
    association: '',
    subAssociation: '',
    facies: '',
    workingUnit: '',
    siteSeries: '',
  };
}
