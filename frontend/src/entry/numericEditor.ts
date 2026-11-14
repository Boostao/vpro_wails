export const singleMaximum = 3.4028234663852886e38;
export function finiteSingleValue(field: string, raw: string, unchanged?: number | null): { value: number | null; error: string | null } {
  const text = raw.trim();
  if (text === '') return { value: null, error: null };
  if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(text)) {
    return { value: null, error: `Enter a finite number or clear ${field} to NULL.` };
  }
  const value = Number(text);
  if (!Number.isFinite(value) || (value !== unchanged && Math.abs(value) > singleMaximum)) {
    return { value: null, error: `${field} is outside the finite Access Single storage domain.` };
  }
  return { value, error: null };
}
