const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const publication = loadTypeScript('publicationSession.ts');
const environment = loadTypeScript('environmentWorkbook.ts', {
  './lifeformSummary': lifeform, './publicationSession': publication, './longEnvironmentReport': {},
});
const shared = loadTypeScript('lifeformWorkbook.ts', {
  './lifeformSummary': lifeform, './speciesAttributeSummary': {},
  './environmentWorkbook': environment, './publicationSession': publication,
});
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-template.json'))),
});
const restoration = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': editor });
const names = loadTypeScript('reportUnitNames.ts', { './projectMetadataRestore': restoration });
const summary = loadTypeScript('siteUnitSummary.ts', { './projectMetadataRestore': restoration, './reportUnitNames': names });
const workbook = loadTypeScript('siteUnitSummaryWorkbook.ts', {
  './siteUnitSummary': summary, './projectMetadataRestore': restoration, './lifeformSummary': lifeform,
  './lifeformWorkbook': shared, './publicationSession': publication, './environmentWorkbook': environment,
});
const owner = { contextId: 'summary-workbook', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
const destination = 'C:\\output\\ exact name .xlsx';
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
function fixture(method = 1, owned = owner) {
  return {
    preview: { contextId: owned.contextId, projectPath: owned.projectPath, suPath: owned.suPath,
      report: { project: owned.project, su: owned.su, method, querySource: 'selected-su-filtered-env-admin',
        fields: structuredClone(workbook.siteUnitSummaryWorkbookFields),
        memberships: [
          { rowId: '1', plotNumber: cell('P'), siteUnit: cell('U:1'), joinedRows: 2, status: 'joined' },
          { rowId: '2', plotNumber: cell(null), siteUnit: cell('U'), joinedRows: 0, status: 'null-plot' },
          { rowId: '3', plotNumber: cell('Q'), siteUnit: cell(null), joinedRows: 0, status: 'null-unit' },
        ],
        units: [{ code: 'U:1', longName: ' Literal name ', nameStatus: 'unique',
          nameCandidates: [{ rowId: '1', value: cell(' Literal name ') }, { rowId: '2', value: cell(null) }],
          values: workbook.siteUnitSummaryWorkbookFields.map(() => ''),
          plots: [{ plotNumber: 'P', suRowId: '1', envRowId: '1', adminRowId: '1' },
            { plotNumber: 'P', suRowId: '1', envRowId: '1', adminRowId: '2' }] }],
      } },
    scope: { savedMethod: 2, siteUnitType: 1, orderBy: 3, includeSpecies: 0 },
    sheets: [{ unit: cell('U:1'), name: 'U-1' }],
    approvalHash: 'a'.repeat(64), workbookSHA256: 'b'.repeat(64), bytes: 1234,
  };
}
const validate = (value, method = 1, owned = owner) => workbook.validateSiteUnitSummaryWorkbookReview(value, owned, method);
function receipt(status = 'published') {
  return { status, requestedDestination: destination, path: status === 'not-published' ? '' : destination,
    sha256: status === 'not-published' ? '' : 'b'.repeat(64),
    errorMessage: status === 'published' ? '' : 'Explicit publication diagnostic' };
}
const source = () => readFileSync(path.join(__dirname, 'SiteUnitSummaryWorkbookPanel.svelte'), 'utf8');

test('exact 39-field source registry matches kernel and detached review retains method override, NULL and empty', () => {
  const kernel = readFileSync(path.join(__dirname, '..', '..', 'siteunitdetailreport.go'), 'utf8');
  const section = kernel.slice(kernel.indexOf('func siteUnitSummaryFields()'), kernel.indexOf('func planSiteUnitSummary'));
  const fields = [...section.matchAll(/\{"([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)"\}/g)]
    .map(([, source, key, label, section, kind]) => ({ source, key, label, section, kind }));
  assert.deepEqual(structuredClone(workbook.siteUnitSummaryWorkbookFields), fields);
  const value = fixture(), result = validate(value);
  assert.equal(result.scope.savedMethod, 2);
  assert.equal(result.preview.report.method, 1);
  assert.equal(result.preview.report.units[0].values.length, 39);
  value.preview.report.units[0].values[0] = 'changed';
  assert.equal(result.preview.report.units[0].values[0], '');
  assert.ok(Object.isFrozen(result.preview.report.units[0].plots[0]));
  assert.throws(() => result.sheets.push({}), TypeError);
  for (const method of [1, 2]) for (const savedMethod of [1, 2]) for (const orderBy of [1, 3]) {
    const v = fixture(method); v.scope.savedMethod = savedMethod; v.scope.orderBy = orderBy;
    assert.doesNotThrow(() => validate(v, method));
  }
});

test('rejects raw unsafe Unicode, null/missing/extra shapes, ownership, method, scope, hashes and mappings', () => {
  const mutations = [
    v => v.extra = true, v => delete v.bytes, v => v.bytes = 0, v => v.bytes = 1.5,
    v => v.bytes = Number.MAX_SAFE_INTEGER + 1, v => v.approvalHash = 'A'.repeat(64), v => v.workbookSHA256 = '',
    v => v.preview.contextId = 'foreign', v => v.preview.projectPath = 'foreign', v => v.preview.suPath = 'foreign',
    v => v.preview.report.project = 'foreign', v => v.preview.report.su = 'None', v => v.preview.report.method = 2,
    v => v.preview.report.querySource = 'layer', v => v.preview.extra = 1, v => v.preview.report.extra = 1,
    v => v.scope = null, v => v.scope.extra = 1, v => delete v.scope.savedMethod,
    v => v.scope.savedMethod = 0, v => v.scope.siteUnitType = 2, v => v.scope.orderBy = 2,
    v => v.scope.includeSpecies = 1, v => v.scope.savedMethod = '2',
    v => v.preview.report.fields[0].label = 'invented', v => v.preview.report.fields[0].kind = 'numeric',
    v => v.preview.report.fields[0].extra = 1, v => v.preview.report.fields.pop(),
    v => v.preview.report.units[0].values.pop(), v => v.preview.report.units[0].values[8] = 'changed',
    v => v.preview.report.units[0].values[0] = null, v => v.preview.report.units[0].values[0] = '\ud800',
    v => v.preview.report.units[0].values[0] = '\udc00', v => v.preview.report.units[0].values[0] = '\u000b',
    v => v.preview.report.units[0].nameCandidates[1].value.text = '\ufffe',
    v => v.preview.report.units[0].values[0] = 'x'.repeat(32768),
    v => v.preview.report.units[0].extra = 1, v => v.preview.report.memberships[0].extra = 1,
    v => v.preview.report.units[0].plots[0].extra = 1, v => v.preview.report.units[0].nameCandidates[0].extra = 1,
    v => v.preview.report.memberships[0].plotNumber.extra = 1,
    v => v.preview.report.units[0].nameCandidates[0].value.extra = 1,
    v => v.sheets = null, v => v.sheets = [], v => v.sheets[0].unit = cell(null),
    v => v.sheets[0].unit = 'U:1', v => v.sheets[0].unit.text = 'different',
    v => v.sheets[0].name = 'success fallback', v => v.sheets[0].extra = 1,
    v => v.sheets[0].unit.extra = 1,
  ];
  for (const mutate of mutations) { const v = fixture(); mutate(v); assert.throws(() => validate(v), String(mutate)); }
  for (const method of [null, 0, 3, '1']) assert.throws(() => validate(fixture(), method));
  for (const su of ['None', 'USysSuTableDynamic']) {
    const owned = { ...owner, su }; assert.throws(() => validate(fixture(1, owned), 1, owned));
  }
});

test('complete ordered physical memberships, Cartesian joins and original name ownership cannot be weakened', () => {
  for (const mutate of [
    v => v.preview.report.memberships.reverse(),
    v => v.preview.report.units[0].plots.reverse(),
    v => v.preview.report.units[0].nameCandidates.reverse(),
    v => v.preview.report.units[0].plots[1].envRowId = '2',
    v => v.preview.report.units[0].plots[1].plotNumber = 'Q',
    v => v.preview.report.units[0].plots[1].adminRowId = '1',
    v => v.preview.report.units[0].plots[0].envRowId = '01',
    v => v.preview.report.memberships[0].joinedRows = 1,
    v => v.preview.report.memberships[1].plotNumber = cell('P'),
    v => { v.preview.report.memberships[1].plotNumber = cell('P'); v.preview.report.memberships[1].status = 'missing-admin'; },
    v => v.preview.report.units[0].nameCandidates[1].value = cell('Other'),
    v => v.preview.report.units[0].longName = null,
    v => v.preview.report.units[0].plots = [],
  ]) { const v = fixture(); mutate(v); assert.throws(() => validate(v), String(mutate)); }
  const complete = fixture();
  complete.preview.report.memberships[0].joinedRows = 4;
  complete.preview.report.units[0].plots.push(
    { plotNumber: 'P', suRowId: '1', envRowId: '2', adminRowId: '1' },
    { plotNumber: 'P', suRowId: '1', envRowId: '2', adminRowId: '2' });
  assert.doesNotThrow(() => validate(complete));
  const extra = structuredClone(complete.preview.report.units[0]);
  extra.code = 'A'; extra.plots = extra.plots.map(p => ({ ...p, suRowId: '4' }));
  complete.preview.report.memberships.push({ rowId: '4', plotNumber: cell('P'), siteUnit: cell('A'), joinedRows: 4, status: 'joined' });
  complete.preview.report.units.push(extra); complete.sheets.push({ unit: cell('A'), name: 'A' });
  assert.throws(() => validate(complete), /inconsistent/);
  extra.nameCandidates = []; extra.longName = null; extra.nameStatus = 'missing';
  assert.doesNotThrow(() => validate(complete));
});

test('source worksheet names preserve literal empty codes and refuse collisions, reserved names and surrogate cuts', () => {
  for (const code of ["'bad", "bad'", '_vpro_source', 'x'.repeat(30) + '😀']) {
    const v = fixture(); v.preview.report.units[0].code = code; v.preview.report.memberships[0].siteUnit = cell(code);
    v.sheets[0].unit = cell(code); assert.throws(() => validate(v));
  }
  const empty = fixture(); empty.preview.report.units[0].code = ''; empty.preview.report.memberships[0].siteUnit = cell('');
  empty.sheets[0] = { unit: cell(''), name: 'NoName0' }; assert.doesNotThrow(() => validate(empty));
  const v = fixture(), other = structuredClone(v.preview.report.units[0]);
  other.code = 'u:1'; other.nameCandidates = []; other.longName = null; other.nameStatus = 'missing';
  other.plots = other.plots.map(p => ({ ...p, suRowId: '4' }));
  v.preview.report.memberships.push({ rowId: '4', plotNumber: cell('P'), siteUnit: cell('u:1'), joinedRows: 2, status: 'joined' });
  v.preview.report.units.unshift(other); v.sheets.unshift({ unit: cell('u:1'), name: 'u-1' });
  assert.throws(() => validate(v), /inconsistent/);
  const unicode = fixture(), first = unicode.preview.report.units[0];
  first.code = '😀'; unicode.preview.report.memberships[0].siteUnit = cell('😀');
  unicode.sheets[0] = { unit: cell('😀'), name: '😀' };
  const second = structuredClone(first);
  second.code = '\ue000'; second.nameCandidates = []; second.longName = null; second.nameStatus = 'missing';
  second.plots = second.plots.map(p => ({ ...p, suRowId: '4' }));
  unicode.preview.report.units.push(second);
  unicode.preview.report.memberships.push({ rowId: '4', plotNumber: cell('P'), siteUnit: cell('\ue000'), joinedRows: 2, status: 'joined' });
  unicode.sheets.push({ unit: cell('\ue000'), name: '\ue000' });
  assert.doesNotThrow(() => validate(unicode));
  unicode.preview.report.units.reverse(); unicode.sheets.reverse();
  assert.throws(() => validate(unicode));
});

test('deterministic publication is irreversible and known committed errors persist across remount until acknowledgement', async () => {
  const owned = { ...owner, contextId: 'known-remount' }, value = fixture(2, owned);
  const session = workbook.siteUnitSummaryWorkbookPublicationSession(owned);
  let resolve, request, held = false, calls = 0;
  const unsubscribe = session.subscribe(() => { const v = session.view(); held = v.busy || v.blocked; });
  const pending = session.publish(value, destination, input => {
    calls++; request = input; return new Promise(done => resolve = done);
  }, 2);
  assert.equal(held, true); assert.throws(() => session.acknowledge(), /cannot be cancelled/);
  assert.equal(workbook.siteUnitSummaryWorkbookPublicationSession({ ...owned }), session);
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() =>
    workbook.siteUnitSummaryWorkbookPublicationSession({ ...owned, [key]: 'different' }), /owned paths/);
  await assert.rejects(session.publish(value, destination, async () => receipt(), 2), /Acknowledge/);
  resolve(receipt('published-with-errors')); await pending;
  assert.deepEqual(structuredClone(request), { method: 2, approvalHash: 'a'.repeat(64), destination });
  assert.equal(session.view().outcome.status, 'published-with-errors'); assert.equal(held, true);
  assert.match(session.view().error, /Explicit/); assert.equal(calls, 1);
  session.acknowledge(); assert.equal(held, false);
  assert.equal(session.view().outcome.status, 'published-with-errors'); unsubscribe();
});

test('lost, incomplete and contradictory receipts stay unknown with replay barrier and no identity inference', async () => {
  for (const result of [null, new Error('transport lost'), {}, { ...receipt(), extra: 1 },
    { ...receipt(), requestedDestination: 'elsewhere' }, { ...receipt(), sha256: 'c'.repeat(64) },
    { ...receipt(), path: '' }, { ...receipt(), errorMessage: 'hidden error' }]) {
    const session = new workbook.SiteUnitSummaryWorkbookPublicationSession(owner);
    let calls = 0;
    await session.publish(fixture(), destination, async () => {
      calls++; if (result instanceof Error) throw result; return result;
    }, 1);
    assert.equal(session.view().outcome, null); assert.equal(session.view().blocked, true);
    assert.equal(session.view().requestedDestination, destination); assert.match(session.view().error, /outcome unknown/);
    await assert.rejects(session.publish(fixture(), destination, async () => { calls++; return receipt(); }, 1), /Acknowledge/);
    assert.equal(calls, 1); session.acknowledge(); assert.equal(session.view().blocked, false);
  }
  for (const status of ['published', 'not-published']) {
    const session = new workbook.SiteUnitSummaryWorkbookPublicationSession(owner);
    await session.publish(fixture(), destination, async () => receipt(status), 1);
    assert.equal(session.view().outcome.status, status); assert.equal(session.view().blocked, true);
  }
  const owned = { ...owner, contextId: 'unknown-remount' };
  const session = workbook.siteUnitSummaryWorkbookPublicationSession(owned);
  await session.publish(fixture(1, owned), destination, async () => null, 1);
  assert.equal(workbook.siteUnitSummaryWorkbookPublicationSession({ ...owned }).view().blocked, true);
});

test('malformed destinations and stale current methods never dispatch; literal valid destination is preserved', async () => {
  const session = new workbook.SiteUnitSummaryWorkbookPublicationSession(owner);
  let calls = 0;
  for (const bad of ['', null, 'x.csv', 'x.xlsx ', 'x.xlsx\0', '\ud800.xlsx']) {
    await assert.rejects(session.publish(fixture(), bad, async () => { calls++; return receipt(); }, 1), /literal .xlsx/);
  }
  await assert.rejects(session.publish(fixture(), destination, async () => { calls++; return receipt(); }, 2));
  assert.equal(calls, 0); assert.equal(session.view().blocked, false);
  await session.publish(fixture(), destination, async request => {
    calls++; assert.equal(request.destination, destination); return receipt();
  }, 1);
  const view = session.view(); view.outcome.path = 'changed';
  assert.equal(session.view().outcome.path, destination); assert.equal(calls, 1);
});

test('panel compiles with accessible scoped source tables, default-off gate, current-method guard and cancellation isolation', () => {
  const text = source();
  assert.equal(compile(text, { filename: 'SiteUnitSummaryWorkbookPanel.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(text, /VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true'/);
  assert.match(text, /onBusyChange\(busy \|\| publication.busy \|\| publication.blocked\)/);
  assert.match(text, /method !== requestedMethod/);
  assert.match(text, /review\.preview\.report\.method === method/);
  assert.match(text, /!currentReview/);
  assert.match(text, /reads\.track\(SiteUnitSummaryWorkbookService\.GetReview/);
  assert.doesNotMatch(text, /reads\.track\(SiteUnitSummaryWorkbookService\.ExportReviewed/);
  assert.match(text, /disposed \|\| requestGeneration !== generation/);
  assert.match(text, /session\.publish\(review, destination/);
  assert.match(text, /session\.acknowledge\(\); review = null/);
  assert.match(text, /<caption>/); assert.match(text, /scope="row"/); assert.match(text, /scope="col"/);
  assert.match(text, /'SITE', 'VEGETATION', 'SOILS'/);
  assert.ok(text.indexOf('publication.error') < text.indexOf('class="destination"'));
  assert.ok(text.indexOf('data-site-unit-workbook-export') < text.indexOf('class="guidance"'));
  for (const selector of ['', '-review', '-cancel', '-destination', '-export', '-acknowledge', '-receipt', '-unknown']) {
    assert.ok(text.includes(`data-site-unit-workbook${selector}`));
  }
  const cancellation = text.slice(text.indexOf('function cancel()'), text.indexOf('async function getReview()'));
  assert.doesNotMatch(cancellation, /acknowledge|publication\s*=|session\./);
});

test('actual server-rendered panel exposes label, disabled gate and explicit remount unknown alert above controls', () => {
  const owned = { ...owner, contextId: 'unknown-remount' };
  const Panel = serverComponent(source().replace("import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK", "'false'")
    .replace('structuredClone(owner)', '({ ...owner })'),
    'SiteUnitSummaryWorkbookPanel.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { SiteUnitSummaryWorkbookService: {} },
      './siteUnitSummaryWorkbook': workbook,
    });
  let held;
  const html = render(Panel, { props: { owner: owned, method: 1, disabled: false, onBusyChange: v => held = v } }).body;
  assert.equal(held, true);
  assert.match(html, /data-site-unit-workbook-unknown[^>]*role="alert"/);
  assert.match(html, /<label[^>]*>Workbook destination[\s\S]*?<input[^>]*data-site-unit-workbook-destination/);
  assert.match(html, /data-site-unit-workbook-review[^>]*disabled/);
  assert.match(html, /data-site-unit-workbook-export[^>]*disabled/);
  assert.ok(html.indexOf('outcome unknown') < html.indexOf('Workbook destination'));
});

function panelScript(owned) {
  let props = { owner: owned, method: 1, disabled: false, onBusyChange: held => holds.push(held) };
  const holds = [], requests = [];
  let destroy, exports;
  let script = source().match(/<script lang="ts">([\s\S]*?)<\/script>/)[1]
    .replace(/\r/g, '')
    .replace(/^\s*import[\s\S]*?;\n/gm, '')
    .replace("import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK", "'true'");
  script = ts.transpileModule(script, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
  const getters = [];
  script = script.replace(/const (\w+) = \$derived\((.*)\);/g, (_, name, expression) => {
    getters.push([name, expression]); return '';
  });
  for (const [name] of getters) script = script.replace(new RegExp(`\\b${name}\\b`, 'g'), `derived.${name}`);
  const gettersCode = getters.map(([name, expression]) => {
    for (const [other] of getters) expression = expression.replace(new RegExp(`\\b${other}\\b`, 'g'), `derived.${other}`);
    return `Object.defineProperty(derived, '${name}', {get:()=>(${expression})});`;
  }).join('\n');
  // Props are simple owned strings; emulate live prop and derived reads, not DOM layout.
  script = script.replace('owner, method, disabled,', 'owner, method: initialMethod, disabled,');
  script = ts.transpileModule(script, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText
    .replace(/(?<![.\w])method\b(?!\s*:)/g, 'props.method');
  const rewrittenGetters = gettersCode.replace(/(?<![.\w])method\b(?!\s*:)/g, 'props.method');
  const code = `const derived = {};\n${script}\n${rewrittenGetters}\nexpose({
    getReview, cancel, publish, destroy:()=>destroyCallback(), acknowledge,
    review:()=>review, busy:()=>busy, publication:()=>publication, destination:value=>destination=value
  });`;
  const state = value => value; state.raw = state;
  vm.runInNewContext(ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, {
    props, $props: () => props, $state: state, structuredClone, untrack: callback => callback(),
    onDestroy: callback => destroy = callback, destroyCallback: () => destroy(),
    expose: value => exports = value,
    ReadRequests: class {
      track(request) { requests.push(request); return request; }
      cancelAll() { for (const request of requests) request.cancel(); }
    },
    siteUnitSummaryWorkbookPublicationSession: workbook.siteUnitSummaryWorkbookPublicationSession,
    validateSiteUnitSummaryWorkbookReview: workbook.validateSiteUnitSummaryWorkbookReview,
    SiteUnitSummaryWorkbookService: {
      GetReview(contextId, json) {
        let resolve; const promise = new Promise(done => resolve = done);
        promise.cancelled = false; promise.cancel = () => promise.cancelled = true;
        promise.resolve = resolve; promise.contextId = contextId; promise.request = JSON.parse(json);
        return promise;
      },
      ExportReviewed: async () => { throw new Error('lost receipt'); },
    },
  });
  return { ...exports, props, holds, requests };
}

test('actual panel review script cancels late delivery and suppresses parent-method shifts without clearing receipt authority', async () => {
  const owned = { ...owner, contextId: 'panel-live' }, panel = panelScript(owned);
  const cancelled = panel.getReview();
  const first = panel.requests[0]; assert.equal(first.contextId, owned.contextId);
  assert.deepEqual(first.request, { method: 1 });
  panel.cancel(); assert.equal(first.cancelled, true);
  first.resolve(fixture(1, owned)); await cancelled;
  assert.equal(panel.review(), null); assert.equal(panel.busy(), false);
  const stale = panel.getReview(), second = panel.requests[1];
  panel.props.method = 2; second.resolve(fixture(1, owned)); await stale;
  assert.equal(panel.review(), null); assert.equal(panel.busy(), false);
  const current = panel.getReview(), third = panel.requests[2];
  third.resolve(fixture(2, owned)); await current;
  assert.equal(panel.review().preview.report.method, 2);
  panel.destination(destination); panel.props.method = 1;
  await panel.publish(); assert.equal(panel.publication().blocked, false);
  panel.props.method = 2; await panel.publish();
  assert.equal(panel.publication().blocked, true); assert.equal(panel.review(), null);
  panel.cancel(); assert.equal(panel.publication().blocked, true);
  panel.destroy(); assert.equal(panel.holds.at(-1), true);
  const remount = panelScript(owned);
  assert.equal(remount.publication().blocked, true); assert.equal(remount.holds.at(-1), true);
  remount.acknowledge(); assert.equal(remount.publication().blocked, false); remount.destroy();
});
