const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-template.json'))),
});
const restoration = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': editor });
const names = loadTypeScript('reportUnitNames.ts', { './projectMetadataRestore': restoration });
const report = loadTypeScript('longEnvironmentReport.ts', { './projectMetadataRestore': restoration, './reportUnitNames': names });
const workbook = loadTypeScript('environmentWorkbook.ts', {
  './longEnvironmentReport': report, './lifeformSummary': loadTypeScript('lifeformSummary.ts'),
  './publicationSession': loadTypeScript('publicationSession.ts'),
});
const owner = { contextId: 'workbook', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
const destination = 'C:\\output\\ exact.xlsx';
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
function fixture() {
  const fields = readFileSync(path.join(__dirname, '..', '..', 'testdata', 'long-environment-fields.txt'), 'utf8')
    .trimEnd().split(/\r?\n/).map(line => {
      if (line.startsWith('# ')) return { source: '', key: '', label: line.slice(2), heading: true };
      const [identity, label] = line.split('='), [source, key] = identity.split('.');
      return { source, key, label, heading: false };
    });
  const values = fields.map(() => cell(null)); values[0] = cell('P1');
  return { preview: { contextId: owner.contextId, projectPath: owner.projectPath, suPath: owner.suPath,
    report: { project: owner.project, su: owner.su, title: ' Title ', fields, diagnostics: [],
      units: [{ code: 'U:1', longName: null, nameStatus: 'missing', nameCandidates: [], plots: [{ plotNumber: 'P1', status: 'complete', values }] }] } },
    sheets: [{ unit: 'U:1', name: 'U-1' }], approvalHash: 'a'.repeat(64), workbookSHA256: 'b'.repeat(64), bytes: 12345 };
}
const receipt = (status = 'published') => ({ status, requestedDestination: destination, path: destination,
  sha256: 'b'.repeat(64), errorMessage: status === 'published' ? '' : 'explicit publication error' });

test('workbook review retains exact report cells, source approval and source worksheet mappings', () => {
  const input = fixture(), value = workbook.validateEnvironmentWorkbookReview(input, owner, ' Title ');
  input.preview.report.title = 'changed'; input.sheets[0].name = 'changed';
  assert.equal(value.preview.report.title, ' Title '); assert.equal(value.sheets[0].name, 'U-1');
  for (const mutate of [
    v => v.approvalHash = 'A'.repeat(64), v => v.workbookSHA256 = '', v => v.bytes = 0,
    v => v.preview.contextId = 'stale', v => v.preview.suPath = 'other',
    v => v.sheets = [], v => v.sheets[0].unit = 'U', v => v.sheets[0].name = 'U:1',
    v => v.sheets[0].name = '\ud800', v => v.extra = true,
  ]) {
    const value = fixture(); mutate(value);
    assert.throws(() => workbook.validateEnvironmentWorkbookReview(value, owner, ' Title '));
  }
});

test('workbook acknowledgement rejects contradictory byte/destination/outcome claims', () => {
  for (const status of ['published', 'published-with-errors', 'not-published']) {
    assert.equal(workbook.validateEnvironmentWorkbookOutcome(receipt(status), destination, 'b'.repeat(64)).status, status);
  }
  for (const mutate of [
    v => v.status = 'success', v => v.sha256 = 'c'.repeat(64), v => v.path = '',
    v => v.requestedDestination = 'other', v => v.errorMessage = 'hidden warning', v => v.extra = true,
  ]) {
    const value = receipt(); mutate(value);
    assert.throws(() => workbook.validateEnvironmentWorkbookOutcome(value, destination, 'b'.repeat(64)));
  }
});

test('shared publication barrier is noncancellable, freezes requests and survives remount until acknowledgement', async () => {
  const session = workbook.environmentWorkbookPublicationSession(owner), input = fixture();
  let resolve, captured, calls = 0;
  const pending = session.publish(input, destination, request => {
    calls++; captured = request;
    return new Promise(done => { resolve = done; });
  });
  input.approvalHash = 'changed'; input.preview.report.title = 'changed';
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.throws(() => session.acknowledge(), /cannot be cancelled/);
  await assert.rejects(session.publish(fixture(), destination, async () => receipt()), /Acknowledge/);
  assert.equal(workbook.environmentWorkbookPublicationSession({ ...owner }), session);
  assert.throws(() => workbook.environmentWorkbookPublicationSession({ ...owner, projectPath: 'different' }), /owned paths/);
  resolve(receipt()); await pending;
  assert.equal(calls, 1); assert.equal(captured.title, ' Title '); assert.equal(captured.approvalHash, 'a'.repeat(64));
  assert.equal(session.view().blocked, true); session.acknowledge();
  assert.equal(session.view().outcome.status, 'published');
});

test('lost or malformed workbook receipts stay unknown instead of becoming no-file defaults', async () => {
  for (const response of [null, { status: 'success' }, new Error('lost receipt')]) {
    const session = new workbook.EnvironmentWorkbookPublicationSession(owner);
    await session.publish(fixture(), destination, async () => {
      if (response instanceof Error) throw response;
      return response;
    });
    assert.equal(session.view().busy, false); assert.equal(session.view().blocked, true);
    assert.equal(session.view().outcome, null); assert.match(session.view().error, /outcome unknown/);
    session.acknowledge(); assert.match(session.view().error, /Do not repeat/);
  }
});

test('workbook panel keeps title/preferences/read/navigation/native-close barriers and receipts above guidance', () => {
  const source = readFileSync(path.join(__dirname, 'LongEnvironmentReport.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'LongEnvironmentReport.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_LONG_ENVIRONMENT_WORKBOOK === 'true'/);
  assert.match(source, /workbookReview = \$state\.raw<ValidatedEnvironmentWorkbookReview/);
  assert.match(source, /onBusyChange\(busy \|\| preferenceBarrier \|\| workbookBarrier\)/);
  assert.match(source, /reads\.track\(EnvironmentWorkbookService\.GetReview/);
  assert.doesNotMatch(source, /reads\.track\(EnvironmentWorkbookService\.ExportReviewed/);
  assert.match(source, /workbookSession\.publish\(workbookReview, destination/);
  assert.match(source, /generation\+\+; reads\.cancelAll/);
  assert.match(source, /function acknowledgeWorkbook\(\) \{\s*workbookSession\.acknowledge\(\);\s*workbookReview = null;/);
  assert.match(source, /input, textarea \{ box-sizing: border-box; min-width: 0;/);
  assert.match(source, /\.report \{[^}]*grid-template-columns: minmax\(0, 1fr\);[^}]*overflow-wrap: anywhere;/);
  assert.ok(source.indexOf('data-environment-workbook-receipt') < source.indexOf('class="guidance"'));
});
