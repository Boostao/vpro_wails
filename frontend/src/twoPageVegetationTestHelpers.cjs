const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { transport, restoration, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const wire = loadTypeScript('twoPageVegetationWire.ts', {
  './twoPageVegetationIdentifierFold.json': require('./twoPageVegetationIdentifierFold.json'),
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const editor = loadTypeScript('twoPageVegetationEditor.ts', {
  './twoPageVegetationWire': wire, './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './qualityEditor': quality,
  './numericEditor': loadTypeScript('numericEditor.ts'), './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const fixture = require('./twoPageVegetationWire.json');
const original = () => structuredClone(fixture.request.original);
const sourceAudit = loadTypeScript('siviDeletionReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration, './qualityEditor': quality,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'), './siviDeletionEditor': {},
});
const receipt = loadTypeScript('twoPageVegetationReceipt.ts', {
  './projectMetadataEditor': metadata,
  './twoPageVegetationWire': wire, './twoPageVegetationEditor': editor,
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
  './siviRequestId': loadTypeScript('siviRequestId.ts'), './siviDeletionReceipt': sourceAudit,
});
const references = loadTypeScript('twoPageVegetationReferences.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration, './qualityEditor': quality,
}, { setTimeout });
const { TwoPageVegetationSession } = loadTypeScript('twoPageVegetationSession.ts', {
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
  './twoPageVegetationWire': wire, './twoPageVegetationEditor': editor, './twoPageVegetationReceipt': receipt,
  './twoPageVegetationReferences': references,
});
module.exports = { wire, editor, fixture, original, cell, receipt, references, TwoPageVegetationSession };
