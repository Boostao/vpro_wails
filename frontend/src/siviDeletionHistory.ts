import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { validSIVIRequestId } from './siviRequestId';
import type { SIVICreationForm } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVIDeletionHistoryEvent {
  historyId: string;
  form: SIVICreationForm;
  rowId: string;
  id: number;
  species: string | null;
  actor: string;
  editWhen: string;
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
export function siviDeletionHistoryFromWire(wire: unknown, owner: SIVISpeciesOwner): SIVIDeletionHistory {
  const diagnostic = 'Complete owned deletion history was not returned; no remembered UUID or first event was inferred.';
  if (!record(wire) || !only(wire, ['contextId', 'project', 'plot', 'historyPresent', 'events']) ||
    wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
    typeof wire.historyPresent !== 'boolean' || !Array.isArray(wire.events) ||
    !wire.historyPresent && wire.events.length !== 0) throw new Error(diagnostic);
  const events: SIVIDeletionHistoryEvent[] = [], seen = new Set<string>();
  for (const event of wire.events) {
    if (!record(event) || !only(event, ['historyId', 'form', 'rowId', 'id', 'species', 'actor', 'editWhen', 'restored', 'consumed']) ||
      !validSIVIRequestId(event.historyId) || seen.has(event.historyId) ||
      event.form !== 'SubVegA-SIVI' && event.form !== 'SubVegA-SIVI_BC' && event.form !== 'SubVegC-SIVI' && event.form !== 'SubVegD-SIVI' ||
      typeof event.rowId !== 'string' || !exactSigned64(event.rowId) ||
      !Number.isInteger(event.id) || Number(event.id) < -2147483648 || Number(event.id) > 2147483647 ||
      !(event.species === null || typeof event.species === 'string' && wellFormedUTF16(event.species)) ||
      typeof event.actor !== 'string' || !event.actor || event.actor.length > 100 || !wellFormedUTF16(event.actor) || event.actor.includes('\0') ||
      typeof event.editWhen !== 'string' || event.editWhen.length !== 19 ||
      !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(event.editWhen) || siviDateTimestampError(event.editWhen) !== null ||
      typeof event.restored !== 'boolean' || typeof event.consumed !== 'boolean' || event.restored !== event.consumed) throw new Error(diagnostic);
    const previous = events.at(-1);
    if (previous && (previous.editWhen < event.editWhen ||
      previous.editWhen === event.editWhen && previous.historyId > event.historyId)) throw new Error(diagnostic);
    seen.add(event.historyId);
    events.push({ historyId: event.historyId, form: event.form, rowId: event.rowId, id: Number(event.id),
      species: event.species, actor: event.actor, editWhen: event.editWhen, restored: event.restored, consumed: event.consumed });
  }
  return { ...owner, historyPresent: wire.historyPresent, events };
}
