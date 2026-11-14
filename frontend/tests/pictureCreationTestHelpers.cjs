const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const picture = loadTypeScript('pictureRead.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata,
});
const editor = loadTypeScript('pictureCreationEditor.ts', {
  './pictureRead': picture, './projectMetadataEditor': metadata, './qualityEditor': quality,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const owner = { contextId: 'owned', project: 'Sample', plotNumber: '00337' };
const requestId = '16fe3d56-d75d-4a74-9507-000000000001';
function selection() {
  return { ...owner, token: 'a'.repeat(64), selectedPath: 'Q:\\No image grant\\  Case #100% 😀.JpG', windowId: 0 };
}
function receipt(request = editor.pictureCreationRequest(selection(), owner, requestId)) {
  return { ...request, requestedSource: 'Q:\\Pictures.sqlite', source: 'Q:\\Pictures.sqlite',
    ownedFiles: { environment: 'Q:\\Env.sqlite', pictures: 'Q:\\Pictures.sqlite' }, actor: ' picture actor ',
    auditStrength: 0, id: 1, rowId: '9223372036854775807',
    columns: ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'].map(name => ({ name, declaredType: '' })),
    committed: { rowId: '9223372036854775807', cells: [
      cell('integer', '1'), cell('text', 'Q:\\No image grant\\'), cell('text', '  Case #100% 😀.JpG'),
      cell('text', owner.plotNumber), cell(),
    ] },
    createdWhen: '2026-10-08T08:00:00.123456789Z', historyId: '1', didCommit: true, replayed: false };
}
module.exports = { editor, picture, owner, selection, receipt, requestId, cell };
