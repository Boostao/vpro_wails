import { wellFormedUTF16 } from './qualityEditor';
import { plotLabelFromHeader, type PlotLabelPreview } from './plotLabelPreview';

export interface PlotLabelRegionalEvidence {
  isoDate: string;
  locale: string;
  pattern: string;
  source: 'windows-user-long-date';
}

export interface RegionalPlotLabelPreview {
  label: PlotLabelPreview;
  labelDate: string;
  evidence: PlotLabelRegionalEvidence;
}

function exactObject(value: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value) ||
      Object.keys(value).length !== keys.length || keys.some(key => !Object.hasOwn(value, key))) {
    throw new Error('Native regional label is incomplete or inconsistent; no repairs applied.');
  }
  return value as Record<string, unknown>;
}

function nativeText(value: unknown, name: string, limit: number): string {
  if (typeof value !== 'string' || !value || !wellFormedUTF16(value) || value.includes('\0') || value.length > limit) {
    throw new Error(`Native regional label ${name} is unavailable or malformed; no locale fallback applied.`);
  }
  return value;
}

function calendarDate(value: unknown): string {
  const text = nativeText(value, 'calendar date', 10);
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text);
  if (!parts) throw new Error('Native regional label calendar date is malformed.');
  const year = Number(parts[1]), month = Number(parts[2]), day = Number(parts[3]);
  const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const limits = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (year < 1601 || month < 1 || month > 12 || day < 1 || day > limits[month - 1]) {
    throw new Error('Native regional label calendar date is invalid; no normalization applied.');
  }
  return text;
}

export function validatePlotLabelRegionalPreview(value: unknown, contextId: string, plot: string): RegionalPlotLabelPreview {
  const root = exactObject(value, ['contextId', 'plotNumber', 'header', 'labelDate', 'isoDate', 'locale', 'pattern', 'source']);
  if (!contextId || root.contextId !== contextId || root.plotNumber !== plot || root.source !== 'windows-user-long-date') {
    throw new Error('Native regional label belongs to another owner or unsupported date source.');
  }
  const header = exactObject(root.header, ['plotNumber', 'zone', 'subZone', 'siteSeries', 'plotRepresenting', 'projectId']);
  const label = Object.freeze(plotLabelFromHeader(header, plot));
  const evidence: PlotLabelRegionalEvidence = Object.freeze({
    isoDate: calendarDate(root.isoDate),
    locale: nativeText(root.locale, 'locale', 84),
    pattern: nativeText(root.pattern, 'Long Date pattern', 4095),
    source: 'windows-user-long-date',
  });
  return Object.freeze({ label, labelDate: nativeText(root.labelDate, 'date display', 4095), evidence });
}
