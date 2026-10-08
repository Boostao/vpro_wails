import { siviHistoricalIdentityFromWire, siviHistoricalListFromWire, type SIVIHistoricalIdentity } from './siviDeletionHistory';
import { validSIVIRequestId } from './siviRequestId';
import { wellFormedUTF16 } from './qualityEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVICreationUndoHistoryEvent extends Omit<SIVIHistoricalIdentity, 'species'> {
  species: string;
  undone: boolean;
  consumed: boolean;
  reviewAvailable: boolean;
  unavailableReason: string | null;
}
export interface SIVICreationUndoHistory extends SIVISpeciesOwner {
  historyPresent: boolean;
  events: SIVICreationUndoHistoryEvent[];
}
export function siviCreationUndoHistoryFromWire(wire: unknown, owner: SIVISpeciesOwner): SIVICreationUndoHistory {
  const diagnostic = 'Complete owned creation Undo history was not returned; no creation or current Undo eligibility was inferred.';
  return siviHistoricalListFromWire(wire, owner,
    ['historyId', 'form', 'rowId', 'id', 'species', 'actor', 'editWhen', 'undone', 'consumed', 'reviewAvailable', 'unavailableReason'], event => {
      const identity = siviHistoricalIdentityFromWire(event, false);
      const reason = event.unavailableReason;
      if (reason !== null && typeof reason !== 'string') throw new Error(diagnostic);
      if (identity.id <= 0 || identity.species === null || !identity.species || identity.species.length > 8 || identity.species.includes('\0') ||
        typeof event.undone !== 'boolean' || typeof event.consumed !== 'boolean' || event.undone !== event.consumed ||
        typeof event.reviewAvailable !== 'boolean' ||
        event.reviewAvailable !== (!event.consumed && validSIVIRequestId(identity.historyId)) ||
        (event.reviewAvailable ? reason !== null : reason === null ||
          !reason || !wellFormedUTF16(reason) || reason.includes('\0'))) throw new Error(diagnostic);
      return { ...identity, species: identity.species, undone: event.undone, consumed: event.consumed,
        reviewAvailable: event.reviewAvailable, unavailableReason: reason };
    });
}
