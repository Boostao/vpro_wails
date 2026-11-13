import { Call } from '@wailsio/runtime';
import type { ProjectState } from '../bindings/github.com/boostao/vpro-wails';

export type HelpDrawerTab = 'inspector' | 'keys' | 'guides';
export type HelpMenuAction = 'guide' | 'tours' | 'shortcuts' | 'whats-new' | 'reset-sample' | 'about';

export interface HelpCodeOption {
  code: string;
  label: string;
}

export interface HelpFieldTopic {
  id: string;
  code: string;
  section: string;
  title: string;
  column: string;
  unit: string;
  lmh25Page: number;
  keyRef?: string;
  summary: string;
  rules: string[];
  codes: HelpCodeOption[];
}

export interface HelpDecisionStep {
  step: number;
  prompt: string;
  yes?: string;
  no?: string;
}

export interface HelpDecisionKey {
  id: string;
  code: string;
  title: string;
  lmh25Page: number;
  steps: HelpDecisionStep[];
}

export interface HelpGuideSection {
  heading: string;
  body: string;
}

export interface HelpGuide {
  id: string;
  slug: string;
  category: string;
  title: string;
  summary: string;
  sections: HelpGuideSection[];
}

export interface HelpTourStep {
  target: string;
  view: string;
  title: string;
  body: string;
}

export interface HelpTour {
  id: string;
  title: string;
  duration: string;
  description: string;
  steps: HelpTourStep[];
}

export interface HelpShortcut {
  keys: string;
  scope: string;
  action: string;
}

export interface HelpWhatsNewEntry {
  version: string;
  date: string;
  channel: string;
  highlights: string[];
}

export interface HelpDataUseAgreement {
  title: string;
  authority: string;
  sourceUrl: string;
  citation: string;
  clauses: string[];
}

export interface StandardsSourceStamp {
  archiveName: string;
  archiveSha256: string;
  extractedName: string;
  extractedSha256: string;
  version: string;
}

export interface StandardsArtifactStamp {
  path: string;
  sha256: string;
  counts: Record<string, number>;
}

export interface HelpLibraryStamps {
  schemaVersion: number;
  generator: string;
  becVersion: string;
  becArchiveSha256: string;
  becExtractedSha256: string;
  taxonomyVersion: string;
  taxonomyArchiveSha256: string;
  taxonomyExtractedSha256: string;
  lmh25Edition: string;
  lmh25Pdf2023Sha256: string;
  sources: StandardsSourceStamp[];
  artifacts: StandardsArtifactStamp[];
}

export interface AboutInfo {
  productName: string;
  fullName: string;
  organization: string;
  appVersion: string;
  schemaVersion: string;
  releaseChannel: string;
  libraryStamps: HelpLibraryStamps;
}

export interface HelpBundle {
  schemaVersion: number;
  productName: string;
  fullName: string;
  organization: string;
  lmh25Edition: string;
  libraryStamps: HelpLibraryStamps;
  fields: HelpFieldTopic[];
  decisionKeys: HelpDecisionKey[];
  guides: HelpGuide[];
  tours: HelpTour[];
  shortcuts: HelpShortcut[];
  whatsNew: HelpWhatsNewEntry[];
  dataUseAgreement: HelpDataUseAgreement;
}

export interface HelpLookupResult {
  requestedId: string;
  resolvedId: string;
  matchKind: string;
  field?: HelpFieldTopic;
  decisionKey?: HelpDecisionKey;
  relatedGuides: HelpGuide[];
}

export interface HelpSearchHit {
  kind: 'field' | 'key' | 'guide' | 'shortcut' | 'tour' | string;
  id: string;
  code?: string;
  title: string;
  section?: string;
  snippet: string;
  lmh25Page?: number;
  score: number;
}

export interface WhatsNewStatus {
  latestVersion: string;
  lastSeenVersion: string;
  hasUnread: boolean;
  unreadCount: number;
  entries: HelpWhatsNewEntry[];
}

export interface HelpBackendPort {
  getHelpBundle(): Promise<HelpBundle>;
  lookupHelpTopic(helpId: string): Promise<HelpLookupResult>;
  searchHelp(query: string): Promise<HelpSearchHit[]>;
  getWhatsNewStatus(): Promise<WhatsNewStatus>;
  acknowledgeWhatsNew(version: string): Promise<WhatsNewStatus>;
  getAboutInfo(): Promise<AboutInfo>;
  resetWorkspaceToSample(contextId: string): Promise<ProjectState>;
}

export const defaultHelpPort: HelpBackendPort = {
  getHelpBundle() {
    return Call.ByName('main.HelpService.GetHelpBundle');
  },
  lookupHelpTopic(helpId: string) {
    return Call.ByName('main.HelpService.LookupHelpTopic', helpId);
  },
  searchHelp(query: string) {
    return Call.ByName('main.HelpService.SearchHelp', query);
  },
  getWhatsNewStatus() {
    return Call.ByName('main.HelpService.GetWhatsNewStatus');
  },
  acknowledgeWhatsNew(version: string) {
    return Call.ByName('main.HelpService.AcknowledgeWhatsNew', version);
  },
  getAboutInfo() {
    return Call.ByName('main.HelpService.GetAboutInfo');
  },
  resetWorkspaceToSample(contextId: string) {
    return Call.ByName('main.HelpService.ResetWorkspaceToSample', contextId);
  },
};

const COLUMN_TO_HELP_ID: Record<string, string> = {
  plotnumber: 'site.plot_number',
  plot: 'site.plot_number',
  fieldnumber: 'site.field_number',
  projectid: 'site.field_number',
  date: 'site.date',
  startdate: 'site.date',
  sitesurveyor: 'site.date',
  vegsurveyor: 'site.date',
  soilsurveyor: 'site.date',
  zone: 'site.bgc_zone',
  subzone: 'site.bgc_zone',
  siteseries: 'site.bgc_zone',
  becsiteunit: 'site.bgc_zone',
  usersiteunit: 'site.bgc_zone',
  mapunit: 'site.bgc_zone',
  plotrepresenting: 'site.bgc_zone',
  representing: 'site.bgc_zone',
  transdistrib: 'site.bgc_zone',
  ecosection: 'site.ecosection',
  fsregiondistrict: 'site.ecosection',
  forestregiondistrict: 'site.ecosection',
  latitude: 'site.coordinates',
  longitude: 'site.coordinates',
  utmzone: 'site.coordinates',
  utmeasting: 'site.coordinates',
  utmnorthing: 'site.coordinates',
  locationaccuracy: 'site.coordinates',
  ntsmapsheet: 'site.coordinates',
  location: 'site.coordinates',
  elevation: 'site.elevation',
  slopegradient: 'site.elevation',
  aspect: 'site.elevation',
  exposure1: 'site.elevation',
  exposure2: 'site.elevation',
  mesoslopeposition: 'site.meso_slope',
  surfaceshape: 'site.meso_slope',
  surfacetopographytype: 'site.meso_slope',
  surfacetopographysize: 'site.meso_slope',
  moistureregime: 'site.moisture_regime',
  nutrientregime: 'site.nutrient_regime',
  substratedecwood: 'site.substrate_cover',
  substratebedrock: 'site.substrate_cover',
  substraterocks: 'site.substrate_cover',
  substratemineralsoil: 'site.substrate_cover',
  substrateorganicmatter: 'site.substrate_cover',
  substratewater: 'site.substrate_cover',
  structuralstage: 'site.successional_structural',
  successionalstatus: 'site.successional_structural',
  standage: 'site.successional_structural',
  sitedisturbance1: 'site.successional_structural',
  sitedisturbance2: 'site.successional_structural',
  sitedisturbance3: 'site.successional_structural',
  floodingregimedur: 'soil.rooting_seepage',
  floodingregimefreq: 'soil.rooting_seepage',
  realmclass: 'site.realm_class',
  snowcoverregime: 'site.realm_class',
  activelayerdepth: 'site.realm_class',
  humusform: 'humus.humus_form',
  humusformph: 'humus.humus_form',
  vonpost: 'humus.von_post',
  texture: 'mineral.texture_cf',
  colour: 'mineral.texture_cf',
  percentcoarsefragstotal: 'mineral.texture_cf',
  percentcoarsefragsgravel: 'mineral.texture_cf',
  percentcoarsefragscobbles: 'mineral.texture_cf',
  percentcoarsefragsstones: 'mineral.texture_cf',
  gravel: 'mineral.texture_cf',
  cobbles: 'mineral.texture_cf',
  stones: 'mineral.texture_cf',
  rootingdepth: 'soil.rooting_seepage',
  rootrestrictingtype: 'soil.rooting_seepage',
  rootrestrictingdepth: 'soil.rooting_seepage',
  seepagedepth: 'soil.rooting_seepage',
  soildrainage: 'soil.rooting_seepage',
  species: 'veg.species',
  plottype: 'sivi.plot_type',
};

export function mapColumnToHelpId(
  column: string | null | undefined,
  domain?: 'site' | 'humus' | 'mineral' | 'veg' | 'sivi'
): string | null {
  if (!column) return null;
  const normalized = column.trim().toLowerCase();
  if (!normalized) return null;

  if (normalized === 'horizon') {
    return domain === 'mineral' ? 'mineral.horizon' : 'humus.horizon';
  }
  if (normalized === 'upperdepth' || normalized === 'lowerdepth') {
    return domain === 'mineral' ? 'mineral.horizon' : 'humus.upper_lower_depth';
  }
  if (
    normalized.startsWith('cover') ||
    normalized.startsWith('height') ||
    normalized === 'totala' ||
    normalized === 'totalb'
  ) {
    return 'veg.covers';
  }
  if (normalized.startsWith('substrate')) {
    return 'site.substrate_cover';
  }
  if (normalized in COLUMN_TO_HELP_ID) {
    return COLUMN_TO_HELP_ID[normalized];
  }
  if (domain === 'sivi') {
    return 'sivi.plot_type';
  }
  return null;
}

export function defaultHelpTopicForView(view: string, entryForm: 'fs882' | 'sivi' = 'fs882'): string {
  if (view === 'fs882' && entryForm === 'sivi') return 'sivi.plot_type';
  switch (view) {
    case 'fs882':
      return 'site.moisture_regime';
    case 'datasheet':
      return 'veg.covers';
    case 'geospatial-map':
    case 'plot-locations':
    case 'google-earth-review':
      return 'site.coordinates';
    case 'diagnostic-synthesis':
    case 'classification-explorer':
    case 'hierarchy':
    case 'subzone-matrix':
      return 'site.bgc_zone';
    case 'lump-editor':
    case 'long-vegetation':
    case 'summary-vegetation':
    case 'lifeform-summary':
      return 'veg.species';
    case 'r-sidecar':
    case 'long-environment':
    case 'summary-environment':
      return 'site.moisture_regime';
    default:
      return 'site.plot_number';
  }
}

export interface HelpInspectableElement {
  getAttribute?(name: string): string | null;
  closest?(selector: string): HelpInspectableElement | null;
}

export function resolveContextualHelpTarget(
  activeElement: HelpInspectableElement | null | undefined,
  view: string,
  entryForm: 'fs882' | 'sivi' = 'fs882'
): string {
  if (activeElement && typeof activeElement.closest === 'function') {
    const directHolder = activeElement.closest('[data-help-id]');
    const directId = directHolder?.getAttribute?.('data-help-id')?.trim();
    if (directId) {
      return directId;
    }

    const soilColHolder = activeElement.closest('[data-soil-column]');
    const soilCol = soilColHolder?.getAttribute?.('data-soil-column');
    if (soilCol) {
      const panelHolder = activeElement.closest('[data-two-page-soil-panel]');
      const formName = panelHolder?.getAttribute?.('data-two-page-soil-panel') ?? '';
      const soilDomain = formName === 'SoilMineralXL' ? 'mineral' : 'humus';
      const mapped = mapColumnToHelpId(soilCol, soilDomain);
      if (mapped) return mapped;
    }

    for (const [attr, domain] of [
      ['data-two-page-entry-field', 'site'],
      ['data-sivi-shared-field', 'sivi'],
      ['data-sivi-parent-field', 'sivi'],
      ['data-chars-field', 'sivi'],
    ] as const) {
      const holder = activeElement.closest(`[${attr}]`);
      const col = holder?.getAttribute?.(attr);
      if (col) {
        const mapped = mapColumnToHelpId(col, domain);
        if (mapped) return mapped;
      }
    }

    const colAttr =
      activeElement.getAttribute?.('data-column') ??
      activeElement.closest('[data-column]')?.getAttribute?.('data-column');
    if (colAttr) {
      const mapped = mapColumnToHelpId(colAttr, entryForm === 'sivi' ? 'sivi' : 'site');
      if (mapped) return mapped;
    }
  }

  return defaultHelpTopicForView(view, entryForm);
}

export function isEditableTarget(
  target: { tagName?: string; isContentEditable?: boolean; getAttribute?: (name: string) => string | null } | null | undefined
): boolean {
  if (!target) return false;
  if (target.isContentEditable) return true;
  const tag = (target.tagName ?? '').toUpperCase();
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
  const role = target.getAttribute?.('role');
  return role === 'textbox' || role === 'combobox' || role === 'searchbox';
}

export function classifyGlobalHelpShortcut(
  event: {
    key: string;
    ctrlKey?: boolean;
    metaKey?: boolean;
    altKey?: boolean;
    shiftKey?: boolean;
    defaultPrevented?: boolean;
  },
  activeElement: { tagName?: string; isContentEditable?: boolean; getAttribute?: (name: string) => string | null } | null | undefined
): 'f1' | 'shortcuts' | null {
  if (event.defaultPrevented) return null;
  if (event.key === 'F1') {
    return 'f1';
  }
  if ((event.ctrlKey || event.metaKey) && !event.altKey && (event.key === '/' || event.key === '?')) {
    return 'shortcuts';
  }
  if (event.key === '?' && !event.ctrlKey && !event.metaKey && !event.altKey) {
    if (!isEditableTarget(activeElement)) {
      return 'shortcuts';
    }
  }
  return null;
}

export interface DecisionKeyHistoryItem {
  step: number;
  prompt: string;
  choice: 'yes' | 'no';
  outcome: string;
}

export interface DecisionKeyEvalState {
  keyId: string;
  currentStep: number;
  completed: boolean;
  conclusion: string | null;
  history: DecisionKeyHistoryItem[];
}

export function isDichotomousDecisionKey(key: HelpDecisionKey | null | undefined): boolean {
  if (!key || !Array.isArray(key.steps) || key.steps.length === 0) return false;
  return key.steps.some((step) => Boolean(step.yes || step.no));
}

export function createDecisionKeyState(key: HelpDecisionKey): DecisionKeyEvalState {
  const firstStep = key.steps[0]?.step ?? 1;
  return {
    keyId: key.id,
    currentStep: firstStep,
    completed: false,
    conclusion: null,
    history: [],
  };
}

export function answerDecisionKeyStep(
  key: HelpDecisionKey,
  state: DecisionKeyEvalState,
  choice: 'yes' | 'no'
): DecisionKeyEvalState {
  if (state.completed) return state;
  const stepObj = key.steps.find((s) => s.step === state.currentStep);
  if (!stepObj) return state;
  const outcome = (choice === 'yes' ? stepObj.yes : stepObj.no) ?? '';
  const nextHistory: DecisionKeyHistoryItem[] = [
    ...state.history,
    {
      step: stepObj.step,
      prompt: stepObj.prompt,
      choice,
      outcome,
    },
  ];
  const gotoMatch = outcome.match(/^Go to Step\s+(\d+)/i);
  if (gotoMatch) {
    const nextStepNum = Number.parseInt(gotoMatch[1], 10);
    if (key.steps.some((s) => s.step === nextStepNum)) {
      return {
        keyId: key.id,
        currentStep: nextStepNum,
        completed: false,
        conclusion: null,
        history: nextHistory,
      };
    }
  }
  return {
    keyId: key.id,
    currentStep: stepObj.step,
    completed: true,
    conclusion: outcome || 'Completed',
    history: nextHistory,
  };
}

export interface CoverPatchRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface CoverComparisonBlock {
  percent: number;
  label: string;
  areaM2: string;
  equivalentSquare: string;
  patches: CoverPatchRect[];
}

function generateDeterministicPatches(percent: number): CoverPatchRect[] {
  // 100 x 100 coordinate frame = 100 m2 (10 m x 10 m plot).
  // Total patch area in SVG units^2 equals percent * 100 (within visual precision).
  switch (percent) {
    case 0.1:
      return [{ x: 47, y: 47, w: 3.2, h: 3.2 }];
    case 1:
      return [
        { x: 14, y: 18, w: 4, h: 4 },
        { x: 68, y: 22, w: 3.5, h: 3.5 },
        { x: 38, y: 44, w: 4.5, h: 4.5 },
        { x: 76, y: 64, w: 4, h: 4 },
        { x: 22, y: 74, w: 4, h: 4 },
        { x: 54, y: 80, w: 3.2, h: 3.2 },
      ];
    case 5:
      return [
        { x: 10, y: 12, w: 10, h: 10 },
        { x: 58, y: 14, w: 9, h: 9 },
        { x: 34, y: 38, w: 11, h: 10 },
        { x: 72, y: 52, w: 9, h: 9 },
        { x: 16, y: 68, w: 9, h: 8 },
        { x: 50, y: 76, w: 8, h: 7 },
      ];
    case 10:
      return [
        { x: 8, y: 8, w: 14, h: 14 },
        { x: 46, y: 10, w: 12, h: 12 },
        { x: 76, y: 16, w: 12, h: 12 },
        { x: 24, y: 38, w: 15, h: 14 },
        { x: 62, y: 44, w: 13, h: 13 },
        { x: 12, y: 68, w: 13, h: 12 },
        { x: 46, y: 72, w: 12, h: 11 },
      ];
    case 25:
      return [
        { x: 6, y: 6, w: 24, h: 22 },
        { x: 44, y: 8, w: 20, h: 18 },
        { x: 72, y: 14, w: 20, h: 20 },
        { x: 18, y: 38, w: 26, h: 22 },
        { x: 56, y: 44, w: 24, h: 20 },
        { x: 8, y: 70, w: 22, h: 18 },
        { x: 48, y: 72, w: 18, h: 15 },
      ];
    case 50:
      return [
        { x: 4, y: 4, w: 34, h: 30 },
        { x: 44, y: 6, w: 26, h: 26 },
        { x: 74, y: 8, w: 22, h: 32 },
        { x: 8, y: 40, w: 30, h: 28 },
        { x: 44, y: 38, w: 32, h: 28 },
        { x: 6, y: 72, w: 36, h: 22 },
        { x: 52, y: 70, w: 40, h: 24 },
      ];
    case 75:
      return [
        { x: 2, y: 2, w: 46, h: 44 },
        { x: 52, y: 2, w: 46, h: 38 },
        { x: 2, y: 50, w: 42, h: 48 },
        { x: 48, y: 44, w: 50, h: 32 },
        { x: 56, y: 78, w: 40, h: 20 },
      ];
    default:
      return [];
  }
}

export const COVER_COMPARISON_BLOCKS: readonly CoverComparisonBlock[] = [
  {
    percent: 0.1,
    label: '0.1%',
    areaM2: '0.1 m² / 100 m²',
    equivalentSquare: '0.32 m × 0.32 m',
    patches: generateDeterministicPatches(0.1),
  },
  {
    percent: 1,
    label: '1%',
    areaM2: '1 m² / 100 m²',
    equivalentSquare: '1.00 m × 1.00 m',
    patches: generateDeterministicPatches(1),
  },
  {
    percent: 5,
    label: '5%',
    areaM2: '5 m² / 100 m²',
    equivalentSquare: '2.24 m × 2.24 m',
    patches: generateDeterministicPatches(5),
  },
  {
    percent: 10,
    label: '10%',
    areaM2: '10 m² / 100 m²',
    equivalentSquare: '3.16 m × 3.16 m',
    patches: generateDeterministicPatches(10),
  },
  {
    percent: 25,
    label: '25%',
    areaM2: '25 m² / 100 m²',
    equivalentSquare: '5.00 m × 5.00 m (1 quadrant)',
    patches: generateDeterministicPatches(25),
  },
  {
    percent: 50,
    label: '50%',
    areaM2: '50 m² / 100 m²',
    equivalentSquare: '7.07 m × 7.07 m (half plot)',
    patches: generateDeterministicPatches(50),
  },
  {
    percent: 75,
    label: '75%',
    areaM2: '75 m² / 100 m²',
    equivalentSquare: '8.66 m × 8.66 m (3 quadrants)',
    patches: generateDeterministicPatches(75),
  },
];
