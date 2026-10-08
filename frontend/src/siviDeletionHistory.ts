import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { validSIVIRequestId } from './siviRequestId';
import type { SIVICreationForm } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVIHistoricalIdentity {
  historyId: string;
  form: SIVICreationForm;
  rowId: string;
  id: number;
  species: string | null;
  actor: string;
  editWhen: string;
}
export interface SIVIDeletionHistoryEvent extends SIVIHistoricalIdentity {
  restored: boolean;
  consumed: boolean;
}
export interface SIVIDeletionHistory {
  contextId: string;
  project: string;
  plot: string;
  historyPresent: boolean;
  events: SIVIDeletionHistoryEvent[];
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const only = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).every(key => keys.includes(key));
function historicalIdOrder(left: string, right: string): number {
  const a = Array.from(left, character => character.codePointAt(0)!);
  const b = Array.from(right, character => character.codePointAt(0)!);
  for (let index = 0; index < Math.min(a.length, b.length); index++) {
    if (a[index] !== b[index]) return a[index] - b[index];
  }
  return a.length - b.length;
}
export function siviHistoricalIdentityFromWire(event: Record<string, unknown>, requireUUID = true): SIVIHistoricalIdentity {
  const diagnostic = 'Historical identity, source form, physical row or literal provenance is incomplete.';
  if (typeof event.historyId !== 'string' || !event.historyId || event.historyId.length > 100 ||
    !wellFormedUTF16(event.historyId) || event.historyId.includes('\0') ||
    requireUUID && !validSIVIRequestId(event.historyId) ||
    event.form !== 'SubVegA-SIVI' && event.form !== 'SubVegA-SIVI_BC' && event.form !== 'SubVegC-SIVI' && event.form !== 'SubVegD-SIVI' ||
    typeof event.rowId !== 'string' || !exactSigned64(event.rowId) ||
    !Number.isInteger(event.id) || Number(event.id) < -2147483648 || Number(event.id) > 2147483647 ||
    !(event.species === null || typeof event.species === 'string' && wellFormedUTF16(event.species)) ||
    typeof event.actor !== 'string' || !event.actor || event.actor.length > 100 || !wellFormedUTF16(event.actor) || event.actor.includes('\0') ||
    typeof event.editWhen !== 'string' || event.editWhen.length !== 19 ||
    !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(event.editWhen) || siviDateTimestampError(event.editWhen) !== null) throw new Error(diagnostic);
  return { historyId: event.historyId, form: event.form, rowId: event.rowId, id: Number(event.id),
    species: event.species, actor: event.actor, editWhen: event.editWhen };
}
export function siviHistoricalListFromWire<Event extends SIVIHistoricalIdentity>(wire: unknown, owner: SIVISpeciesOwner,
  eventKeys: string[], parse: (event: Record<string, unknown>) => Event): SIVISpeciesOwner & { historyPresent: boolean; events: Event[] } {
  const diagnostic = 'Complete owned historical choices were not returned; no first event or current eligibility was inferred.';
  if (!record(wire) || !only(wire, ['contextId', 'project', 'plot', 'historyPresent', 'events']) ||
    wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
    typeof wire.historyPresent !== 'boolean' || !Array.isArray(wire.events) ||
    !wire.historyPresent && wire.events.length !== 0) throw new Error(diagnostic);
  const events: Event[] = [], seen = new Set<string>();
  for (const event of wire.events) {
    if (!record(event) || !only(event, eventKeys)) throw new Error(diagnostic);
    const parsed = parse(event);
    if (seen.has(parsed.historyId)) throw new Error(diagnostic);
    const previous = events.at(-1);
    if (previous && (previous.editWhen < parsed.editWhen ||
      previous.editWhen === parsed.editWhen && historicalIdOrder(previous.historyId, parsed.historyId) > 0)) throw new Error(diagnostic);
    seen.add(parsed.historyId);
    events.push(parsed);
  }
  return { ...owner, historyPresent: wire.historyPresent, events };
}
export function siviDeletionHistoryFromWire(wire: unknown, owner: SIVISpeciesOwner): SIVIDeletionHistory {
  return siviHistoricalListFromWire(wire, owner,
    ['historyId', 'form', 'rowId', 'id', 'species', 'actor', 'editWhen', 'restored', 'consumed'], event => {
      const identity = siviHistoricalIdentityFromWire(event);
      if (typeof event.restored !== 'boolean' || typeof event.consumed !== 'boolean' || event.restored !== event.consumed) {
        throw new Error('Deletion history must preserve exact restoration/consumption facts.');
      }
      return { ...identity, restored: event.restored, consumed: event.consumed };
    });
}
