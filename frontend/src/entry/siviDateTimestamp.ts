const timestampPattern = /^([0-9]{4})-([0-9]{2})-([0-9]{2}) ([0-9]{2}):([0-9]{2}):([0-9]{2})(?:\.[0-9]{1,9})?$/;

export function siviDateTimestampError(raw: string): string | null {
  const match = timestampPattern.exec(raw);
  if (!match || match[0] !== raw) return 'Date requires YYYY-MM-DD HH:MM:SS[.fraction] without a timezone';
  const [year, month, day, hour, minute, second] = match.slice(1, 7).map(Number);
  if (year < 100) return 'Date year must be between 0100 and 9999';
  const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (month < 1 || month > 12 || day < 1 || day > days[month - 1] ||
      hour > 23 || minute > 59 || second > 59) return 'Date must be a valid calendar/time value';
  return null;
}
