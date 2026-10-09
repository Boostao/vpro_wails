const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { transport, restoration, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const numeric = loadTypeScript('numericEditor.ts');
const soil = loadTypeScript('soilChildEditor.ts', { './qualityEditor': quality, './numericEditor': numeric });
const editor = loadTypeScript('twoPageSoilEditor.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './qualityEditor': quality, './numericEditor': numeric,
  './soilChildEditor': soil, './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const sourceAudit = loadTypeScript('siviDeletionReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration, './qualityEditor': quality,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'), './siviDeletionEditor': {},
});
const receipt = loadTypeScript('twoPageSoilReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration, './qualityEditor': quality,
  './siviDeletionReceipt': sourceAudit, './twoPageSoilEditor': editor,
  './siviParentTransport': transport, './siviRequestId': loadTypeScript('siviRequestId.ts'),
  './soilChildEditor': soil,
});
const { TwoPageSoilSession } = loadTypeScript('twoPageSoilSession.ts', {
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
  './projectMetadataEditor': metadata, './twoPageSoilEditor': editor, './twoPageSoilReceipt': receipt,
});
const fixture = require('./twoPageSoilWire.json');
const receipts = require('./twoPageSoilReceiptWire.json');
const outcomes = require('./twoPageSoilOutcomeWire.json');
const original = () => structuredClone(fixture.request.original);
const wire = (strength = 1) => structuredClone(receipts.find(entry => entry.name === String(strength)).receipt);
function outcome(json, status = 'committed', committed = wire()) {
  const result = structuredClone(outcomes.find(value => value.status === status));
  const request = JSON.parse(json);
  result.requestJSON = json;
  result.requestId = request.requestId;
  result.contextId = request.original.contextId;
  if (status === 'committed') result.receipt = { ...committed, didCommit: false, replayed: true };
  return result;
}
module.exports = { editor, receipt, TwoPageSoilSession, fixture, original, wire, cell, outcome, outcomes };
