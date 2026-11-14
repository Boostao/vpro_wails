import { validateSIVIParentOriginal, type SIVIParentOriginal } from './siviParentEditor';
import { SIVIParentSession } from './siviParentSession';
import { sourceParentOriginalFromWire, type SIVIParentActionDisplay, type SIVIParentOwner } from './siviMetadataWire';

export { completeMetadataCellFromWire, siviMetadataTableFromWire, sourceParentOriginalFromWire, type SIVIParentActionDisplay, type SIVIParentOwner } from './siviMetadataWire';

export function siviParentOriginalFromWire(wire: unknown, owner: SIVIParentOwner): SIVIParentOriginal {
  return sourceParentOriginalFromWire(wire, owner, validateSIVIParentOriginal);
}

export function siviParentSessionFromWire(editorId: string, wire: unknown, owner: SIVIParentOwner): SIVIParentSession {
  return new SIVIParentSession(editorId, siviParentOriginalFromWire(wire, owner));
}

// Display initialization never stages an option or repairs historical storage.
export function siviParentActionDisplay(original: SIVIParentOriginal): SIVIParentActionDisplay[] {
  const review = validateSIVIParentOriginal(original);
  return ['PlotType', 'SpeciesListComplete'].map(name => {
    const source = review.Bindings.find(binding => binding.Binding === name);
    if (!source) throw new Error('SIVI parent action source was not returned.');
    const expected = review.Rows[0][source.Table === review.EnvTable ? 'Env' : 'Admin'].cells[source.Column];
    let option: number | null = null;
    let diagnostic: string | null = null;
    if (expected.storage !== 'null') {
      if (name === 'PlotType' && expected.storage === 'text') {
        const index = ['Ground', 'Visual', 'Note', 'FS882', 'Other'].indexOf(expected.text ?? '');
        if (index >= 0) option = index + 1;
      } else if (name === 'SpeciesListComplete' && expected.storage === 'integer') {
        if (expected.integer === '-1') option = 1;
        else if (expected.integer === '0') option = 2;
      }
      if (option === null) diagnostic = `${name} original storage has no verified literal display mapping; retain it unchanged or select an explicit supported action.`;
    }
    return { controlId: `form:frmSIVIsite/${name === 'PlotType' ? 'optPlotType' : 'optSpeciesListComplete'}`,
      expected: structuredClone(expected), option, diagnostic };
  });
}
