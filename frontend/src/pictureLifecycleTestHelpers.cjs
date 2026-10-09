const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { picture, cell } = require('./pictureCreationTestHelpers.cjs');
const { metadata } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('pictureLifecycleEditor.ts', {
  './pictureRead': picture, './projectMetadataEditor': metadata, './qualityEditor': quality,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const fixture = require('./pictureLifecycleWire.json');
const owner = fixture.owner;
const requestId = fixture.deletion.request.requestId;
function deletion() { return structuredClone(fixture.deletion); }
function review() {
  const request = deletion().request;
  return { ...owner, records: { columns: request.columns, rows: [request.original] } };
}
function restorationReview(currentOwner = owner) {
  return { ...currentOwner, historyId: '1', deletion: deletion(), peers: [] };
}
function restoration(request = editor.pictureRestorationRequest(restorationReview(), owner, fixture.restorationRequestId)) {
  return { request, deletion: deletion(), actor: fixture.restoringActor, auditStrength: fixture.restoringAuditStrength,
    restoredWhen: fixture.restoredWhen, didCommit: true, replayed: false };
}
module.exports = { editor, fixture, owner, requestId, deletion, review, restorationReview, restoration, cell, loadTypeScript };
