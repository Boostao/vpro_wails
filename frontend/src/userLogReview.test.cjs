const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, metadata } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const vegetation = loadTypeScript('twoPageVegetationTable.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const log = loadTypeScript('userLogReview.ts', {
  './siviParentTransport': transport, './qualityEditor': quality, './twoPageVegetationTable': vegetation,
});
const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'userLogWire.json'), 'utf8'));
const validate = value => log.userLogFromWire(value, 'owned-context', value.mode, value.offset, value.limit);

test('shared actual-Go all/current/page/end wires retain source captions, raw storage and separate duplicate rows', () => {
  const wires = fixture(), before = JSON.stringify(wires);
  for (const [name, total, ids] of [
    ['all', 5, ['1', '2', '3', '4', '5']], ['current', 3, ['1', '4', '5']],
    ['page', 5, ['2', '3']], ['end', 3, []],
  ]) {
    const view = validate(wires[name]);
    assert.equal(view.total, total);
    assert.deepEqual(Array.from(view.table.rows, row => row.rowId), ids);
    assert.doesNotThrow(() => structuredClone(view));
    if (view.table.rows.length) view.table.rows[0].cells[0].text = 'caller only';
  }
  assert.equal(JSON.stringify(wires), before);
  assert.deepEqual(Array.from(log.userLogFields, field => [field.name, field.caption]),
    [['User', 'User'], ['InTime', 'Login'], ['OutTime', 'OutTime'], ['LocalMachine', 'Local Machine']]);
  const rows = validate(wires.all).table.rows;
  assert.equal(log.userLogCellLabel(rows[0].cells[2]), '(NULL)');
  assert.equal(log.userLogCellLabel(rows[1].cells[0]), '(empty text)');
  assert.equal(log.userLogCellLabel(rows[2].cells[1]), '0');
  assert.equal(log.userLogCellLabel(rows[2].cells[3]), 'hex:00ff');
  assert.equal(log.userLogCellLabel(rows[4].cells[1]), '1.125');
});
for (const [name, mutate] of [
  ['foreign context', value => value.contextId = 'foreign'],
  ['wrong mode', value => value.mode = 2],
  ['wrong offset', value => value.offset = 1],
  ['wrong limit', value => value.limit = 200],
  ['empty provenance', value => value.sourcePath = ''],
  ['malformed provenance', value => value.sourcePath = '\ud800'],
  ['NUL provenance', value => value.sourcePath += '\0'],
  ['negative total', value => value.total = -1],
  ['excessive total', value => value.total = 10001],
  ['fractional total', value => value.total = 5.5],
  ['short page', value => value.table.rows.pop()],
  ['sparse page', value => delete value.table.rows[0]],
  ['duplicate identity', value => value.table.rows[1].rowId = '1'],
  ['wrong order', value => value.table.rows.reverse()],
  ['wrong wire schema', value => { value.table.columns[0].type = 'TEXT'; delete value.table.columns[0].declaredType; }],
  ['extra column', value => value.table.columns.push({ name: 'Extra', declaredType: 'TEXT' })],
  ['duplicate column', value => value.table.columns[1].name = 'User'],
  ['unmapped column', value => value.table.columns[0].name = 'user'],
  ['malformed declared type', value => value.table.columns[0].declaredType = '\ud800'],
  ['malformed stored text', value => value.table.rows[0].cells[0].text = '\ud800'],
  ['integer overflow', value => value.table.rows[2].cells[1].integer = '9223372036854775808'],
  ['malformed blob', value => value.table.rows[2].cells[3].blobHex = '0g'],
]) test(`strict saved-log client rejects ${name}`, () => {
  const value = fixture().all; mutate(value);
  assert.throws(() => log.userLogFromWire(value, 'owned-context', 1, 0, 50));
});
test('Current rejects empty text as NULL; invalid request modes and bounds never default', () => {
  const current = fixture().current;
  current.table.rows[0].cells[2] = fixture().all.table.rows[1].cells[2];
  assert.throws(() => validate(current));
  for (const [mode, offset, limit] of [[0, 0, 50], [3, 0, 50], [1, -1, 50], [1, 0.5, 50], [1, 0, 0], [1, 0, 201]]) {
    assert.throws(() => log.userLogFromWire(fixture().all, 'owned-context', mode, offset, limit));
  }
});

function driver(getReview) {
  const activity = [];
  const values = {
    generation: 0, reads: new ReadRequests(), mode: 2, busy: false, error: '', review: null,
    contextId: 'owned-context', limit: 50, operationHeld: false,
    onBusyChange: value => activity.push(value), UserLogService: { GetReview: getReview },
    userLogFromWire: log.userLogFromWire,
  };
  const context = componentFunctions('UserLogReview.svelte', ['cancel', 'changeMode', 'show'], values);
  return { context, activity, functions: context.actions };
}
function deferred() {
  let resolve, reject, cancels = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancels++; };
  return { promise, resolve, reject, cancelled: () => cancels };
}
test('actual component read uses source Current, retains backend errors and releases activity', async () => {
  const calls = [], read = deferred(), d = driver((...args) => { calls.push(args); return read.promise; });
  const pending = d.functions.show();
  assert.equal(d.context.busy, true);
  assert.deepEqual(calls, [['owned-context', 2, 0, 50]]);
  read.resolve(fixture().current); await pending;
  assert.equal(d.context.review.total, 3); assert.equal(d.context.busy, false);
  assert.deepEqual(d.activity, [false, true, false]);
  const bad = driver(() => { throw new Error('source read failed'); });
  await bad.functions.show();
  assert.match(bad.context.error, /source read failed/); assert.equal(bad.context.review, null);
});
test('actual Cancel/mode correction rejects late old results and preserves a newer pending operation', async () => {
  const old = deferred(), fresh = deferred(); let call = 0;
  const d = driver(() => (++call === 1 ? old.promise : fresh.promise));
  const pendingOld = d.functions.show();
  d.functions.changeMode('1');
  assert.equal(old.cancelled(), 1); assert.equal(d.context.review, null);
  const pendingFresh = d.functions.show();
  old.resolve(fixture().current); await pendingOld;
  assert.equal(d.context.busy, true); assert.equal(d.context.review, null);
  fresh.resolve(fixture().all); await pendingFresh;
  assert.equal(d.context.review.mode, 1); assert.equal(d.context.busy, false);
  d.functions.changeMode('01');
  assert.equal(d.context.mode, 0); assert.match(d.context.error, /Choose/);
  await d.functions.show();
  assert.match(d.context.error, /source Show mode/);
});
test('held/root operations, context drift and cancelled late failures cannot populate the panel', async () => {
  const held = driver(() => { throw new Error('must not invoke'); });
  held.context.operationHeld = true; await held.functions.show();
  assert.match(held.context.error, /current operation/);
  const read = deferred(), changed = driver(() => read.promise);
  const pending = changed.functions.show(); changed.context.contextId = 'new-context';
  read.resolve(fixture().current); await pending;
  assert.equal(changed.context.review, null);
  const late = deferred(), cancelled = driver(() => late.promise);
  const waiting = cancelled.functions.show(); cancelled.functions.cancel();
  late.reject(new Error('old failure')); await waiting;
  assert.equal(cancelled.context.error, ''); assert.equal(cancelled.context.busy, false);
});
test('root refresh/list/hierarchy all retain the current user-log activity barrier', async () => {
  for (const name of ['refresh', 'loadPlots', 'loadHierarchy']) {
    const context = Object.fromEntries([
      'projectReportBusy', 'twoPageVegetationBusy', 'twoPageSoilBusy', 'pictureCreationBusy',
      'pictureLifecycleBusy', 'vegetationPublicationBusy', 'lifeformPublicationBusy', 'summaryPublicationBusy',
      'summaryPreferenceBusy', 'externalSUEnvironmentBusy',
    ].map(key => [key, false]));
    Object.assign(context, { view: 'user-log', editorBusy: true });
    const functions = componentFunctions('App.svelte', [name], context);
    await functions[name](0);
  }
});

async function renderComponent(source, props, populated = false) {
  if (populated) {
    const marker = 'let review = $state.raw<UserLogView | null>(null);';
    assert.equal(source.split(marker).length, 2);
    source = source.replace(marker, `let review = $state.raw<UserLogView | null>(${JSON.stringify(fixture().all)});`);
  }
  let code = compile(source, { filename: 'UserLogReview.svelte', generate: 'server' }).js.code;
  code = code.replace(/import\s*\{\s*UserLogService\s*\}\s*from\s*['"][^'"]+['"];?/,
    'const UserLogService = {GetReview(){throw new Error("SSR must not invoke reads")}};');
  code = code.replace(/import\s*\{\s*ReadRequests\s*\}\s*from\s*['"][^'"]+['"];?/,
    'class ReadRequests {cancelAll(){}}');
  code = code.replace(/import\s*\{[^}]*userLogFromWire[^}]*\}\s*from\s*['"][^'"]+['"];?/,
    `const userLogFields=${JSON.stringify(log.userLogFields)}; const userLogCellLabel=JSON.stringify;`);
  for (const name of ['svelte/internal/server', 'svelte']) {
    code = code.replaceAll(`'${name}'`, JSON.stringify(pathToFileURL(require.resolve(name)).href));
    code = code.replaceAll(`"${name}"`, JSON.stringify(pathToFileURL(require.resolve(name)).href));
  }
  const module = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`);
  const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
  return render(module.default, { props }).body;
}
test('actual compiled SSR consumes default Current, four populated column captions and explicit limitations', async () => {
  const source = readFileSync(path.join(__dirname, 'UserLogReview.svelte'), 'utf8');
  let destroyed = 0;
  const props = { contextId: 'owned-context', onBusyChange: () => destroyed++ };
  const initial = await renderComponent(source, props);
  assert.ok(destroyed > 0, 'lazy rendering must actually execute component callbacks');
  assert.match(initial, /value="2" selected/);
  assert.match(initial, /not a list of live Wails users/);
  assert.match(initial, /no login\/logout recording/);
  const body = await renderComponent(source, props, true);
  for (const caption of ['User', 'Login', 'OutTime', 'Local Machine']) assert.ok(body.includes(`>${caption}</th>`));
  assert.equal((body.match(/aria-label="Saved session row /g) ?? []).length, 5);
  assert.match(source, /\$state\.raw<UserLogView \| null>/);
  assert.doesNotMatch(source, /\b(?:Save|Delete|LogIn|LogOut)\s*\(/);
});

for (const flag of [undefined, false, true, 'false', 'TRUE', '1', 'true']) {
  test(`compiled normal User log menu requires the exact independent frontend literal ${String(flag)}`, async () => {
    const source = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
    const component = serverComponent(source, 'Navigation.svelte', {
      '@lucide/svelte': new Proxy({}, { get: () => () => {} }),
    }, { VITE_USER_LOG_REVIEW: flag });
    const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
    const body = render(component, { props: { view: 'home', onnavigate: () => {} } }).body;
    const button = [...body.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)].find(match => match[2].includes('User log'));
    assert.ok(button, 'planned selector must resolve');
    assert.equal(/\bdisabled(?:=|\s|$)/.test(button[1]), flag !== 'true');
  });
}
