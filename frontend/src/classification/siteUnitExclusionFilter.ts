import { Call } from '@wailsio/runtime';
import type { CancellableRead } from '../shared/readRequests';

export const DEFAULT_SITE_UNIT_EXCLUSIONS: readonly string[] = [
  '.support',
  '.poor',
  '.nudum',
];

export type ExclusionMatchMode = 'auto' | 'suffix' | 'contains' | 'wildcard';

export interface SiteUnitExclusionRule {
  pattern: string;
  matchMode: ExclusionMatchMode;
  enabled: boolean;
}

export interface ExcludedSiteUnitInfo {
  siteUnit: string;
  matchedPattern: string;
  plotCount: number;
}

export interface RetainedSiteUnitInfo {
  siteUnit: string;
  plotCount: number;
}

export interface SiteUnitExclusionSummary {
  appliedRules: SiteUnitExclusionRule[];
  totalPlotsBefore: number;
  retainedPlots: number;
  excludedPlots: number;
  retainedUnits: RetainedSiteUnitInfo[];
  excludedUnits: ExcludedSiteUnitInfo[];
}

export type BreakpointPolicy = 'lowest' | 'highest';
export type UnbrokenUnitPolicy = 'keep_original' | 'exclude_unbroken';

export interface HierarchyBreakpointOptions {
  breakpointPolicy: BreakpointPolicy;
  unbrokenUnitPolicy: UnbrokenUnitPolicy;
  stripTildePrefix: boolean;
}

export interface HierarchyBreakpointUnitMapping {
  originalUnit: string;
  effectiveUnit: string;
  rolledUp: boolean;
  matchedInTree: boolean;
  excluded: boolean;
  plotCount: number;
}

export interface HierarchyBreakpointBucket {
  breakpointUnit: string;
  sourceUnits: string[];
  plotCount: number;
}

export interface HierarchyBreakpointResolution {
  contextId: string;
  project: string;
  su: string;
  hierarchy: string;
  breakpointPolicy: BreakpointPolicy;
  totalUnitsBefore: number;
  totalUnitsAfter: number;
  rolledUpPlotCount: number;
  unmappedUnits: string[];
  mappings: HierarchyBreakpointUnitMapping[];
  buckets: HierarchyBreakpointBucket[];
}

export interface SiteUnitExclusionPreviewRequest {
  rules: SiteUnitExclusionRule[];
  useDefaultPresets: boolean;
  useHierarchyBreakpoints: boolean;
  breakpointOptions: HierarchyBreakpointOptions;
}

export interface SiteUnitExclusionPreview {
  contextId: string;
  project: string;
  su: string;
  hierarchy: string;
  exclusions: SiteUnitExclusionSummary;
  breakpoints?: HierarchyBreakpointResolution;
}

export interface SummaryVegetationExclusionRequest {
  average: number;
  display: number;
  showClass: number;
  groupAndOrder: number;
  rules: SiteUnitExclusionRule[];
  useDefaultPresets: boolean;
  useHierarchyBreakpoints: boolean;
  breakpointOptions: HierarchyBreakpointOptions;
}

export interface SummaryVegetationExclusionPreview {
  preview: unknown;
  exclusions: SiteUnitExclusionSummary;
  breakpoints?: HierarchyBreakpointResolution;
}

export interface LongVegetationExclusionRequest {
  rules: SiteUnitExclusionRule[];
  useDefaultPresets: boolean;
  useHierarchyBreakpoints: boolean;
  breakpointOptions: HierarchyBreakpointOptions;
}

export interface LongVegetationExclusionPreview {
  preview: unknown;
  exclusions: SiteUnitExclusionSummary;
  breakpoints?: HierarchyBreakpointResolution;
}

export interface SiteUnitExclusionBackendPort {
  previewExclusions(
    contextId: string,
    request: SiteUnitExclusionPreviewRequest
  ): CancellableRead<SiteUnitExclusionPreview>;
  resolveHierarchyBreakpoints(
    contextId: string,
    request: {
      rules: SiteUnitExclusionRule[];
      useDefaultPresets: boolean;
      options: HierarchyBreakpointOptions;
    }
  ): CancellableRead<HierarchyBreakpointResolution>;
  previewSummaryWithExclusions(
    contextId: string,
    request: SummaryVegetationExclusionRequest
  ): CancellableRead<SummaryVegetationExclusionPreview>;
  previewLongWithExclusions(
    contextId: string,
    request: LongVegetationExclusionRequest
  ): CancellableRead<LongVegetationExclusionPreview>;
}

export const defaultSiteUnitExclusionPort: SiteUnitExclusionBackendPort = {
  previewExclusions: (contextId, request) =>
    Call.ByName('main.VegetationReportService.PreviewSiteUnitExclusions', contextId, request),
  resolveHierarchyBreakpoints: (contextId, request) =>
    Call.ByName(
      'main.VegetationReportService.ResolveHierarchyBreakpointUnits',
      contextId,
      request
    ),
  previewSummaryWithExclusions: (contextId, request) =>
    Call.ByName(
      'main.VegetationReportService.PreviewSummaryVegetationWithExclusions',
      contextId,
      request
    ),
  previewLongWithExclusions: (contextId, request) =>
    Call.ByName(
      'main.VegetationReportService.PreviewLongVegetationWithExclusions',
      contextId,
      request
    ),
};

export function createDefaultSiteUnitExclusionRules(): SiteUnitExclusionRule[] {
  return DEFAULT_SITE_UNIT_EXCLUSIONS.map(pattern => ({
    pattern,
    matchMode: 'auto',
    enabled: true,
  }));
}

export function createDefaultBreakpointOptions(): HierarchyBreakpointOptions {
  return {
    breakpointPolicy: 'lowest',
    unbrokenUnitPolicy: 'keep_original',
    stripTildePrefix: false,
  };
}

export function wildcardMatchCaseInsensitive(pattern: string, target: string): boolean {
  const p = Array.from(pattern.toLowerCase());
  const s = Array.from(target.toLowerCase());
  let pi = 0;
  let si = 0;
  let starIdx = -1;
  let matchIdx = 0;

  while (si < s.length) {
    if (pi < p.length && (p[pi] === '?' || p[pi] === s[si])) {
      pi++;
      si++;
    } else if (pi < p.length && p[pi] === '*') {
      starIdx = pi;
      matchIdx = si;
      pi++;
    } else if (starIdx !== -1) {
      pi = starIdx + 1;
      matchIdx++;
      si = matchIdx;
    } else {
      return false;
    }
  }
  while (pi < p.length && p[pi] === '*') {
    pi++;
  }
  return pi === p.length;
}

export function matchSiteUnitAgainstRules(
  siteUnit: string,
  rules: readonly SiteUnitExclusionRule[]
): { excluded: boolean; matchedPattern: string } {
  const trimmed = siteUnit.trim();
  if (!trimmed) return { excluded: false, matchedPattern: '' };
  const lowerUnit = trimmed.toLowerCase();

  for (const rule of rules) {
    if (!rule.enabled) continue;
    const pat = rule.pattern.trim();
    if (!pat) continue;
    const lowerPat = pat.toLowerCase();
    let mode: ExclusionMatchMode = rule.matchMode || 'auto';

    if (mode === 'auto') {
      if (pat.includes('*') || pat.includes('?')) {
        mode = 'wildcard';
      } else if (pat.startsWith('.')) {
        if (
          lowerUnit.endsWith(lowerPat) ||
          lowerUnit.includes(`${lowerPat}.`) ||
          lowerUnit.includes(`${lowerPat}-`) ||
          lowerUnit.includes(`${lowerPat}_`) ||
          lowerUnit.includes(`${lowerPat} `)
        ) {
          return { excluded: true, matchedPattern: rule.pattern };
        }
        continue;
      } else {
        mode = 'contains';
      }
    }

    if (mode === 'suffix' && lowerUnit.endsWith(lowerPat)) {
      return { excluded: true, matchedPattern: rule.pattern };
    }
    if (mode === 'contains' && lowerUnit.includes(lowerPat)) {
      return { excluded: true, matchedPattern: rule.pattern };
    }
    if (mode === 'wildcard' && wildcardMatchCaseInsensitive(lowerPat, lowerUnit)) {
      return { excluded: true, matchedPattern: rule.pattern };
    }
  }
  return { excluded: false, matchedPattern: '' };
}
