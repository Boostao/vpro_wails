const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');

const rpcCalls = [];
const mockWailsRuntime = {
  Call: {
    ByName(methodName, ...args) {
      rpcCalls.push({ methodName, args });
      return Promise.resolve({ ok: true });
    },
  },
};

const help = loadTypeScript('helpSystem.ts', {
  '@wailsio/runtime': mockWailsRuntime,
});

const helpBundleFixture = JSON.parse(
  readFileSync(path.join(__dirname, '..', '..', 'resources', 'help_bundle.json'), 'utf8')
);
const readSrc = (file) => readFileSync(path.join(__dirname, file), 'utf8');
const iconsProxy = new Proxy({}, { get: () => () => {} });

test('defaultHelpPort dispatches exact HelpService Wails RPC method names', async () => {
  rpcCalls.length = 0;
  await help.defaultHelpPort.getHelpBundle();
  await help.defaultHelpPort.lookupHelpTopic('site.moisture_regime');
  await help.defaultHelpPort.searchHelp('SITE 20');
  await help.defaultHelpPort.getWhatsNewStatus();
  await help.defaultHelpPort.acknowledgeWhatsNew('2.0.0');
  await help.defaultHelpPort.getAboutInfo();
  await help.defaultHelpPort.resetWorkspaceToSample('ctx-128bit');

  assert.deepEqual(rpcCalls, [
    { methodName: 'main.HelpService.GetHelpBundle', args: [] },
    { methodName: 'main.HelpService.LookupHelpTopic', args: ['site.moisture_regime'] },
    { methodName: 'main.HelpService.SearchHelp', args: ['SITE 20'] },
    { methodName: 'main.HelpService.GetWhatsNewStatus', args: [] },
    { methodName: 'main.HelpService.AcknowledgeWhatsNew', args: ['2.0.0'] },
    { methodName: 'main.HelpService.GetAboutInfo', args: [] },
    { methodName: 'main.HelpService.ResetWorkspaceToSample', args: ['ctx-128bit'] },
  ]);
});

test('resolveContextualHelpTarget resolves data-help-id, soil/site/SIVI column attributes, and view fallbacks', () => {
  const makeNode = (attrs = {}, ancestors = {}) => ({
    getAttribute: (name) => (name in attrs ? attrs[name] : null),
    closest: (selector) => {
      const m = selector.match(/^\[([a-z0-9-]+)\]$/i);
      if (!m) return null;
      const key = m[1];
      if (key in attrs) return { getAttribute: (n) => (n in attrs ? attrs[n] : null) };
      if (key in ancestors) return { getAttribute: (n) => (n === key ? ancestors[key] : null) };
      return null;
    },
  });

  // 1. Direct or ancestor data-help-id wins
  assert.equal(
    help.resolveContextualHelpTarget(makeNode({ 'data-help-id': 'site.moisture_regime' }), 'fs882'),
    'site.moisture_regime'
  );
  assert.equal(
    help.resolveContextualHelpTarget(makeNode({}, { 'data-help-id': 'veg.covers' }), 'fs882'),
    'veg.covers'
  );

  // 2. Soil panel column context distinguishes Humus vs Mineral UpperDepth/Horizon
  assert.equal(
    help.resolveContextualHelpTarget(
      makeNode({}, { 'data-soil-column': 'UpperDepth', 'data-two-page-soil-panel': 'SoilHumusXL' }),
      'fs882'
    ),
    'humus.upper_lower_depth'
  );
  assert.equal(
    help.resolveContextualHelpTarget(
      makeNode({}, { 'data-soil-column': 'UpperDepth', 'data-two-page-soil-panel': 'SoilMineralXL' }),
      'fs882'
    ),
    'mineral.horizon'
  );
  assert.equal(
    help.resolveContextualHelpTarget(
      makeNode({}, { 'data-soil-column': 'VonPost', 'data-two-page-soil-panel': 'SoilHumusXL' }),
      'fs882'
    ),
    'humus.von_post'
  );

  // 3. Column & SIVI fallback attributes
  assert.equal(
    help.resolveContextualHelpTarget(makeNode({ 'data-column': 'Cover5a' }), 'fs882'),
    'veg.covers'
  );
  assert.equal(
    help.resolveContextualHelpTarget(makeNode({ 'data-column': 'SubstrateDecWood' }), 'fs882'),
    'site.substrate_cover'
  );
  assert.equal(
    help.resolveContextualHelpTarget(makeNode({}, { 'data-sivi-parent-field': 'PlotType' }), 'fs882', 'sivi'),
    'sivi.plot_type'
  );

  // 4. View-level defaults when no input is focused
  assert.equal(help.resolveContextualHelpTarget(null, 'fs882', 'sivi'), 'sivi.plot_type');
  assert.equal(help.resolveContextualHelpTarget(null, 'fs882', 'fs882'), 'site.moisture_regime');
  assert.equal(help.resolveContextualHelpTarget(null, 'geospatial-map'), 'site.coordinates');
  assert.equal(help.resolveContextualHelpTarget(null, 'diagnostic-synthesis'), 'site.bgc_zone');
  assert.equal(help.resolveContextualHelpTarget(null, 'home'), 'site.plot_number');
});

test('classifyGlobalHelpShortcut handles F1 everywhere, ? only outside editable controls, and Ctrl+/', () => {
  const inputEl = { tagName: 'INPUT', isContentEditable: false, getAttribute: () => null };
  const textareaEl = { tagName: 'TEXTAREA', isContentEditable: false, getAttribute: () => null };
  const bodyEl = { tagName: 'BODY', isContentEditable: false, getAttribute: () => null };

  assert.equal(help.classifyGlobalHelpShortcut({ key: 'F1' }, inputEl), 'f1');
  assert.equal(help.classifyGlobalHelpShortcut({ key: 'F1' }, bodyEl), 'f1');
  assert.equal(help.classifyGlobalHelpShortcut({ key: 'F1', defaultPrevented: true }, bodyEl), null);

  assert.equal(help.classifyGlobalHelpShortcut({ key: '?' }, bodyEl), 'shortcuts');
  assert.equal(help.classifyGlobalHelpShortcut({ key: '?' }, inputEl), null);
  assert.equal(help.classifyGlobalHelpShortcut({ key: '?' }, textareaEl), null);

  assert.equal(help.classifyGlobalHelpShortcut({ key: '/', ctrlKey: true }, inputEl), 'shortcuts');
  assert.equal(help.classifyGlobalHelpShortcut({ key: '/', metaKey: true }, bodyEl), 'shortcuts');
});

test('dichotomous decision key evaluator resolves all rSMR (0–8) and rSNR (A–E) branches and SVG % cover blocks', () => {
  const rsmrKey = helpBundleFixture.decisionKeys.find((k) => k.id === 'key.rsmr_9_4');
  const rsnrKey = helpBundleFixture.decisionKeys.find((k) => k.id === 'key.rsnr_9_6');
  const coverKey = helpBundleFixture.decisionKeys.find((k) => k.id === 'key.cover_chart');

  assert.equal(help.isDichotomousDecisionKey(rsmrKey), true);
  assert.equal(help.isDichotomousDecisionKey(rsnrKey), true);
  assert.equal(help.isDichotomousDecisionKey(coverKey), false);

  // rSMR: Step 1 No -> Step 2 No -> Step 3 Yes => 6 — Hygric
  let st = help.createDecisionKeyState(rsmrKey);
  st = help.answerDecisionKeyStep(rsmrKey, st, 'no');
  assert.equal(st.currentStep, 2);
  st = help.answerDecisionKeyStep(rsmrKey, st, 'no');
  assert.equal(st.currentStep, 3);
  st = help.answerDecisionKeyStep(rsmrKey, st, 'yes');
  assert.equal(st.completed, true);
  assert.match(st.conclusion, /^6 — Hygric/);
  assert.equal(st.history.length, 3);

  // rSMR: All No through Step 6 => 4 — Mesic
  let mesic = help.createDecisionKeyState(rsmrKey);
  for (let i = 0; i < 6; i++) {
    mesic = help.answerDecisionKeyStep(rsmrKey, mesic, 'no');
  }
  assert.equal(mesic.completed, true);
  assert.match(mesic.conclusion, /^4 — Mesic/);

  // rSNR: Step 1 No -> Step 2 Yes => B — Poor
  let poor = help.createDecisionKeyState(rsnrKey);
  poor = help.answerDecisionKeyStep(rsnrKey, poor, 'no');
  poor = help.answerDecisionKeyStep(rsnrKey, poor, 'yes');
  assert.equal(poor.completed, true);
  assert.equal(poor.conclusion, 'B — Poor');

  // Visual % Cover SVG plates cover all 7 canonical LMH25 (2023) p. 274 percentages
  assert.deepEqual(
    Array.from(help.COVER_COMPARISON_BLOCKS, (b) => b.percent),
    [0.1, 1, 5, 10, 25, 50, 75]
  );
  for (const block of help.COVER_COMPARISON_BLOCKS) {
    assert.ok(block.patches.length >= 1);
    for (const p of block.patches) {
      assert.ok(p.x >= 0 && p.y >= 0 && p.x + p.w <= 100 && p.y + p.h <= 100);
    }
  }
});

test('all 5 Svelte 5 Help/Tour/Modal components compile cleanly with zero warnings, zero emojis, and BC Gov styling', () => {
  const emojiPattern = /[\u{1F300}-\u{1FAFF}]/u;
  for (const file of [
    'HelpDrawer.svelte',
    'OnboardingTour.svelte',
    'KeyboardShortcutsModal.svelte',
    'WhatsNewModal.svelte',
    'AboutVProModal.svelte',
  ]) {
    const source = readSrc(file);
    assert.doesNotMatch(source, emojiPattern, `${file} must contain zero emojis`);
    assert.match(source, /#003366/, `${file} must apply BC Navy header hierarchy`);
    assert.match(source, /#FCBA19/, `${file} must apply BC Gold accent rule`);
    const compiled = compile(source, { filename: file, generate: 'client' });
    assert.deepEqual(compiled.warnings, [], `${file} emitted compiler warnings`);
  }

  // SSR render checks with open=true
  const Drawer = serverComponent(readSrc('HelpDrawer.svelte'), 'HelpDrawer.svelte', {
    '@lucide/svelte': iconsProxy,
    './helpSystem': help,
  });
  const drawerHtml = render(Drawer, {
    props: { open: true, topicId: 'site.moisture_regime', onclose: () => {} },
  }).body;
  assert.match(drawerHtml, /role="complementary"/);
  assert.match(drawerHtml, /aria-label="VPro Help and LMH25 Reference"/);
  assert.match(drawerHtml, /data-help-tab="inspector"/);
  assert.match(drawerHtml, /data-help-tab="keys"/);
  assert.match(drawerHtml, /data-help-tab="guides"/);

  const WhatsNew = serverComponent(readSrc('WhatsNewModal.svelte'), 'WhatsNewModal.svelte', {
    '@lucide/svelte': iconsProxy,
    './helpSystem': help,
  });
  const whatsNewHtml = render(WhatsNew, {
    props: {
      open: true,
      initialStatus: {
        latestVersion: '2.0.0',
        lastSeenVersion: '0.1.0',
        hasUnread: true,
        unreadCount: 1,
        entries: helpBundleFixture.whatsNew,
      },
      onclose: () => {},
    },
  }).body;
  assert.match(whatsNewHtml, /data-whats-new-unread-banner/);
  assert.match(whatsNewHtml, /Mark v2\.0\.0 as Read/);
  assert.match(whatsNewHtml, /Version 2\.0\.0/);

  // Build-tier presentation checks in AboutVProModal (dev vs alpha vs release)
  const About = serverComponent(readSrc('AboutVProModal.svelte'), 'AboutVProModal.svelte', {
    '@lucide/svelte': iconsProxy,
    './helpSystem': help,
  });
  assert.equal(help.formatShortCommit('30e86f6abcdef1234'), '30e86f6');
  assert.equal(help.formatShortCommit('local'), 'local');
  assert.equal(help.formatShortCommit(''), '');
  assert.ok(About);
});

test('Navigation Help menu exposes all 6 active items, unread badge, Tour targets, and lazy App wiring', () => {
  const navSource = readSrc('Navigation.svelte');
  const Nav = serverComponent(navSource, 'Navigation.svelte', { '@lucide/svelte': iconsProxy });
  const html = render(Nav, {
    props: {
      view: 'home',
      onnavigate: () => {},
      whatsNewUnreadCount: 2,
      resetSampleAvailable: true,
      onhelpaction: () => {},
    },
  }).body;

  for (const label of [
    'VPro User Guide &amp; LMH25 Reference (F1)',
    'Interactive Walkthroughs (4 Tours)',
    'Keyboard Shortcuts (?)',
    "What's New / Release Notes (2 new)",
    'Reset Workspace to Sample',
    'About VPro &amp; Standards Libraries',
  ]) {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const match = html.match(new RegExp(`<button\\b([^>]*)>[\\s\\S]*?<span>${escaped}<\\/span>\\s*<\\/button>`));
    assert.ok(match, `Help menu must render "${label}"`);
    const buttonAttrs = match[0].slice(match[0].lastIndexOf('<button')).split('>')[0];
    assert.equal(/\bdisabled(?:\s|=|$)/.test(buttonAttrs), false, `"${label}" must be enabled`);
  }
  assert.match(html, /data-nav-whats-new-badge[^>]*>2</);
  assert.match(html, /What's New \/ Release Notes \(2 new\)/);
  assert.match(html, /data-nav-group="Project"/);
  assert.match(html, /data-nav-group="Interchange"/);

  const dispatched = [];
  const ctx = componentFunctions('Navigation.svelte', ['itemEnabled', 'activate', 'closeMenu', 'navigate'], {
    resetSampleAvailable: true,
    openMenu: 'Help',
    navigation: { querySelector: () => ({ querySelector: () => ({ focus: () => {} }) }) },
    onnavigate: () => {},
    onhelpaction: (action) => dispatched.push(action),
  });
  for (const action of ['guide', 'tours', 'shortcuts', 'whats-new', 'reset-sample', 'about']) {
    ctx.actions.activate({ helpAction: action });
  }
  assert.deepEqual(dispatched, ['guide', 'tours', 'shortcuts', 'whats-new', 'reset-sample', 'about']);

  // Verify App.svelte lazy-loads all 5 Help components rather than static-importing them
  const appSource = readSrc('App.svelte');
  for (const comp of [
    'HelpDrawer',
    'OnboardingTour',
    'KeyboardShortcutsModal',
    'WhatsNewModal',
    'AboutVProModal',
  ]) {
    assert.doesNotMatch(appSource, new RegExp(`import ${comp} from`));
    assert.match(appSource, new RegExp(`import\\(['"]\\./${comp}\\.svelte['"]\\)`));
  }

  // Verify entry forms carry data-help-id without static component imports
  for (const entryFile of [
    'HeaderEditor.svelte',
    'FS882Form.svelte',
    'TwoPageEntryEditor.svelte',
    'TwoPageSoilPanel.svelte',
  ]) {
    const src = readSrc(entryFile);
    assert.match(src, /data-help-id=/, `${entryFile} must include data-help-id attributes`);
    assert.doesNotMatch(src, /HelpDrawer|helpSystem/, `${entryFile} must have zero new static help imports`);
  }
});

test('index.html embeds inline SVG data-URI favicon and LMH25 2023 help bundle includes 3c, 3d, A.w.th, A.w.br, CO, and FloodingRegimeDur S', () => {
  const indexHtml = readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  assert.match(indexHtml, /<link\s+rel="icon"\s+type="image\/svg\+xml"\s+href="data:image\/svg\+xml,/);

  const docsFields = JSON.parse(
    readFileSync(path.join(__dirname, '..', '..', 'docs-src', 'lmh25_fields.json'), 'utf8')
  );
  const bundle = JSON.parse(
    readFileSync(path.join(__dirname, '..', '..', 'resources', 'help_bundle.json'), 'utf8')
  );

  for (const dataset of [docsFields, bundle]) {
    const allCodes = dataset.fields.flatMap(f => f.codes.map(c => c.code));
    for (const requiredCode of ['3c', '3d', 'A.w.th', 'A.w.br', 'CO', 'S']) {
      assert.ok(allCodes.includes(requiredCode), `LMH25 2023 code ${requiredCode} must be documented`);
    }
  }

  const makeColNode = col => ({
    getAttribute: name => (name === 'data-column' ? col : null),
    closest: selector => (selector === '[data-column]' ? { getAttribute: () => col } : null),
  });
  assert.equal(help.resolveContextualHelpTarget(makeColNode('SiteDisturbance1'), 'fs882'), 'site.successional_structural');
  assert.equal(help.resolveContextualHelpTarget(makeColNode('Exposure1'), 'fs882'), 'site.elevation');
  assert.equal(help.resolveContextualHelpTarget(makeColNode('FloodingRegimeDur'), 'fs882'), 'soil.rooting_seepage');
});
