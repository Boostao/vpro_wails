const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, loadTypeScript, srcPath } = require('./svelteTestHelpers.cjs');

const iconsProxy = new Proxy({}, { get: () => () => {} });
const help = loadTypeScript('helpSystem.ts', {
  '@wailsio/runtime': { Call: { ByName: () => Promise.resolve({}) } },
});

test('HomeWorkspaceHub compiles cleanly with BC Gov styling, zero emojis, and strict buildMode tier gating', () => {
  const source = readFileSync(srcPath('HomeWorkspaceHub.svelte'), 'utf8');
  assert.doesNotMatch(source, /[\u{1F300}-\u{1FAFF}]/u, 'must contain zero emojis');
  assert.doesNotMatch(source, /disposable|during migration|Not yet a complete Access replacement/i);
  assert.match(source, /#003366/);
  assert.match(source, /#FCBA19/);

  const compiled = compile(source, { filename: 'HomeWorkspaceHub.svelte', generate: 'client' });
  assert.deepEqual(compiled.warnings, []);

  const Hub = serverComponent(source, 'HomeWorkspaceHub.svelte', {
    '@lucide/svelte': iconsProxy,
    './helpSystem': help,
  });

  // 1. Release mode: clean workspace hub with zero dev or alpha banner
  const releaseHtml = render(Hub, {
    props: {
      activeProject: 'Sample',
      activeSU: 'Sample_SU',
      activeHierarchy: 'Sample_Hierarchy',
      plotCount: 42,
      buildMode: 'release',
      appVersion: '2.1.0',
      onnavigate: () => {},
      onhelpaction: () => {},
    },
  }).body;
  assert.doesNotMatch(releaseHtml, /data-home-tier-banner/);
  assert.match(releaseHtml, /Sample/);
  assert.match(releaseHtml, /Sample_SU/);
  assert.match(releaseHtml, /BECdb v13 · Taxa v19 · LMH25/);
  assert.equal((releaseHtml.match(/data-home-workflow=/g) ?? []).length, 6);

  // 2. Alpha mode: client evaluation banner + SHA256SUMS notice, zero dev path clutter
  const alphaHtml = render(Hub, {
    props: {
      activeProject: 'Sample',
      plotCount: 42,
      buildMode: 'alpha',
      appVersion: '2.1.0-alpha.1',
      onnavigate: () => {},
    },
  }).body;
  assert.match(alphaHtml, /data-home-tier-banner="alpha"/);
  assert.match(alphaHtml, /VPro Alpha Preview \(v2\.1\.0-alpha\.1\)/);
  assert.match(alphaHtml, /SHA256SUMS\.txt/);
  assert.doesNotMatch(alphaHtml, /DEV BUILD/);

  // 3. Dev mode: compact dev banner with version and short commit
  const devHtml = render(Hub, {
    props: {
      activeProject: 'Sample',
      plotCount: 42,
      buildMode: 'dev',
      appVersion: '2.0.1-dev',
      gitCommit: '30e86f6abcdef',
      releaseChannel: 'stable',
      onnavigate: () => {},
      onhelpaction: () => {},
    },
  }).body;
  assert.match(devHtml, /data-home-tier-banner="dev"/);
  assert.match(devHtml, /DEV BUILD · v2\.0\.1-dev · 30e86f6/);
});

test('App and Navigation embed HomeWorkspaceHub and clean BEC workflow terminology', () => {
  const app = readFileSync(srcPath('App.svelte'), 'utf8');
  const navigation = readFileSync(srcPath('Navigation.svelte'), 'utf8');
  assert.match(app, /HomeWorkspaceHub/);
  assert.doesNotMatch(app, /DemoReadiness/);
  assert.doesNotMatch(navigation, /Unavailable workflows are disabled/);
  assert.doesNotMatch(navigation, /only verified workflows are writable/);
});
