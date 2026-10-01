import { becGroups, equalCode } from './becEditor';
import { wellFormedUTF16, type PlotQualityChoice } from './qualityEditor';

export function createReferenceCodeEditor<Key extends string, List extends string>(config: {
  keys: readonly Key[];
  labels: Record<Key, string>;
  list: (key: Key) => List;
  limits: Record<List, number>;
}) {
  type Codes = Record<Key, string | null>;
  function valueError(key: Key, value: string | null, original: string | null): string | null {
    if (value === null || value === original) return null;
    const label = config.labels[key], limit = config.limits[config.list(key)];
    if (value === '') return `Clear ${label} to NULL instead of an empty string.`;
    if (!wellFormedUTF16(value)) return `${label} contains an incomplete Unicode character; it has not been replaced.`;
    if (value.length > limit) return `${label} must be at most ${limit} UTF-16 characters; the raw entry has not been truncated.`;
    return null;
  }
  function lengthError(codes: Codes, original: Codes | null): string | null {
    for (const key of config.keys) {
      const error = valueError(key, codes[key], original?.[key] ?? null);
      if (error) return error;
    }
    return null;
  }
  function changed(codes: Codes, original: Codes | null): boolean {
    return config.keys.some(key => codes[key] !== (original?.[key] ?? null));
  }
  function selectable(row: PlotQualityChoice, list: List): boolean {
    return row.selectable && row.listName === list && row.code !== null && row.code !== '' &&
      row.code.length <= config.limits[list] && wellFormedUTF16(row.code);
  }
  function groups(rows: PlotQualityChoice[], list: List) {
    return becGroups(rows, row => selectable(row, list) ? row.code : null);
  }
  function definitions(rows: PlotQualityChoice[], list: List, code: string | null): PlotQualityChoice[] {
    return rows.filter(row => selectable(row, list) && equalCode(row.code, code));
  }
  function suggestions(rows: PlotQualityChoice[], key: Key, code: string | null) {
    if (code === null || code === '') return [];
    const fold = (value: string) => value.replace(/[A-Z]/g, letter => letter.toLowerCase());
    return groups(rows, config.list(key)).filter(group => group.code !== code && fold(group.code).startsWith(fold(code)));
  }
  const acknowledgements = new WeakMap<object, string>();
  function fingerprint(codes: Codes): string { return JSON.stringify(config.keys.map(key => codes[key])); }
  function rememberAcknowledgement(draft: object, codes: Codes, accepted: boolean): void {
    if (accepted) acknowledgements.set(draft, fingerprint(codes));
    else acknowledgements.delete(draft);
  }
  function acknowledged(draft: object, codes: Codes): boolean {
    return acknowledgements.get(draft) === fingerprint(codes);
  }
  return { valueError, lengthError, changed, selectable, groups, definitions, suggestions,
    rememberAcknowledgement, acknowledged };
}
