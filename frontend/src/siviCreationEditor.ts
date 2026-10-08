import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { parseSIVICover, siviCoverGroups, type SIVICoverColumn, type SIVICoverCell } from './siviCoverEditor';
import { validateSIVISpeciesReferences, siviSpeciesListed, siviSpeciesChoices,
  type SIVISpeciesOwner, type SIVISpeciesReferences } from './siviSpeciesEditor';
import { resolveSpeciesDecision, type SpeciesDecision, type SpeciesDecisionKind } from './vegetationSpeciesEditor';

export type SIVICreationForm = 'SubVegA-SIVI' | 'SubVegA-SIVI_BC' | 'SubVegC-SIVI' | 'SubVegD-SIVI';
export interface SIVICreationDraft extends SIVISpeciesOwner {
  form: SIVICreationForm;
  species: string;
  decision?: SpeciesDecision;
  cells: Partial<Record<SIVICoverColumn, SIVICoverCell>>;
}
export interface SIVICreationPlan extends SIVISpeciesOwner {
  form: SIVICreationForm;
  species: string;
  decision?: SpeciesDecision;
  covers: { column: SIVICoverColumn; value: ProjectMetadataCell }[];
}
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

function scope(form: SIVICreationForm) {
  const group = form === 'SubVegA-SIVI' || form === 'SubVegA-SIVI_BC' ? 0
    : form === 'SubVegC-SIVI' ? 1 : form === 'SubVegD-SIVI' ? 2 : -1;
  if (group < 0) throw new Error('SIVI creation requires an explicit A, C or D source form.');
  return { group, columns: siviCoverGroups(group, form === 'SubVegA-SIVI').flat() };
}

function requireOwner(draft: SIVISpeciesOwner, owner: SIVISpeciesOwner) {
  if (!owner.contextId || !owner.project || !owner.plot || owner.plot.length > 7 ||
      !wellFormedUTF16(owner.plot) || owner.plot.includes('\0') ||
      draft.contextId !== owner.contextId || draft.project !== owner.project || draft.plot !== owner.plot) {
    throw new Error('SIVI creation requires the active context, project and exact nonempty source parent; no parent was inferred.');
  }
}

export function beginSIVICreation(form: SIVICreationForm, owner: SIVISpeciesOwner): SIVICreationDraft {
  scope(form);
  requireOwner(owner, owner);
  return { contextId: owner.contextId, project: owner.project, plot: owner.plot, form, species: '', cells: {} };
}

export function stageSIVICreationSpecies(draft: SIVICreationDraft, species: string): SIVICreationDraft {
  if (typeof species !== 'string') throw new Error('SIVI creation requires literal Species text.');
  return { ...draft, species, decision: undefined };
}

export function chooseSIVICreationSpecies(draft: SIVICreationDraft, references: SIVISpeciesReferences,
  owner: SIVISpeciesOwner, kind: SpeciesDecisionKind, selected?: string): SIVICreationDraft {
  requireOwner(draft, owner);
  validateSIVISpeciesReferences(references, owner);
  if (draft.decision) throw new Error('Correct the original Species entry before making another creation decision.');
  const resolved = resolveSpeciesDecision(siviSpeciesChoices(references, draft.form, draft.species), kind, selected);
  return { ...draft, species: resolved.value, decision: resolved.decision };
}

export function stageSIVICreationCover(draft: SIVICreationDraft, column: SIVICoverColumn,
  raw: string, nullValue: boolean): SIVICreationDraft {
  if (!scope(draft.form).columns.includes(column)) {
    throw new Error('The creation field is hidden or belongs to another source form; identity, heights and defaults are not assigned.');
  }
  const cell = parseSIVICover(column, raw, nullValue, nullCell());
  return { ...draft, cells: { ...draft.cells, [column]: cell } };
}

function speciesError(draft: SIVICreationDraft, references: SIVISpeciesReferences): string | null {
  if (!draft.species || !wellFormedUTF16(draft.species) || draft.species.includes('\0') || draft.species.length > 8) {
    return 'New Species requires nonempty literal text within 8 UTF-16 units; raw input was not repaired.';
  }
  if (draft.decision) {
    const decision = draft.decision;
    const resolved = resolveSpeciesDecision(siviSpeciesChoices(references, draft.form, decision.entered),
      decision.kind, decision.selected);
    return resolved.value === draft.species ? null : 'Creation Species differs from its explicit source decision.';
  }
  return siviSpeciesListed(references, scope(draft.form).group).some(option => option.code === draft.species)
    ? null : 'Select an exact source-list Species code or review its old-code/personal choices before creating.';
}

export function siviCreationErrors(draft: SIVICreationDraft, references: SIVISpeciesReferences,
  owner: SIVISpeciesOwner): string[] {
  requireOwner(draft, owner);
  const source = scope(draft.form);
  const approved = validateSIVISpeciesReferences(references, owner);
  const errors: string[] = [];
  const invalidSpecies = speciesError(draft, approved);
  if (invalidSpecies) errors.push(invalidSpecies);
  let member = false;
  for (const key of Object.keys(draft.cells)) {
    const column = source.columns.find(column => column === key);
    if (!column) throw new Error('Creation contains a hidden or foreign source field; no partial plan is available.');
    const staged = draft.cells[column];
    if (!staged || !equalCell(staged.expected, nullCell())) {
      throw new Error('Creation requires a complete new-row draft, not a historical original or implicit identity.');
    }
    const parsed = parseSIVICover(column, staged.raw, staged.nullValue, nullCell());
    if (parsed.error) errors.push(parsed.error);
    else if (!equalCell(staged.value, parsed.value)) errors.push(`${column} differs from its literal creation draft.`);
    else if (parsed.value.storage !== 'null') member = true;
  }
  if (!member) errors.push('Enter an explicit non-NULL cover or total belonging to this source view; no zero or automatic sum is inferred.');
  return errors;
}

export function siviCreationPlan(draft: SIVICreationDraft, references: SIVISpeciesReferences,
  owner: SIVISpeciesOwner): SIVICreationPlan {
  const errors = siviCreationErrors(draft, references, owner);
  if (errors.length) throw new Error(errors[0]);
  const covers = scope(draft.form).columns.flatMap(column => {
    const cell = draft.cells[column];
    return cell && cell.value.storage !== 'null' ? [{ column, value: { ...cell.value } }] : [];
  });
  return { contextId: owner.contextId, project: owner.project, plot: owner.plot, form: draft.form, species: draft.species,
    ...(draft.decision ? { decision: { ...draft.decision } } : {}), covers };
}
