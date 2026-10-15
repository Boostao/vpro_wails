const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const source = readFileSync(path.join(__dirname, 'DemoReadiness.svelte'), 'utf8');
const flags = ['VITE_LONG_ENVIRONMENT_REPORT', 'VITE_LONG_VEGETATION_REPORT',
  'VITE_SITE_UNIT_SUMMARY', 'VITE_LIFEFORM_SUMMARY', 'VITE_SUMMARY_VEGETATION'];

test('actual Svelte rendering reports literal build availability without changing any gate', () => {
  for (const value of [undefined, '', 'false', 'TRUE', 'true']) {
    const environment = Object.fromEntries(flags.map(flag => [flag, value]));
    const html = render(serverComponent(source, 'DemoReadiness.svelte', {}, environment)).body;
    assert.equal((html.match(/data-demo-report=/g) ?? []).length, 5);
    assert.equal((html.match(/>Enabled in this build</g) ?? []).length, value === 'true' ? 5 : 0);
    assert.equal((html.match(/>Not enabled in this build</g) ?? []).length, value === 'true' ? 0 : 5);
    assert.match(html, /Use disposable projects for writes/);
    assert.match(html, /Not yet a complete Access replacement/);
    assert.match(html, /complete combined/);
    assert.doesNotMatch(html, /<button|<input|<select/);
  }
});

test('Home embeds capability explanation and FS882 tooltip does not claim a stale field subset', () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const navigation = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  assert.match(app, /import DemoReadiness from '\.\/DemoReadiness\.svelte'/);
  assert.match(app, /<DemoReadiness \/>/);
  assert.doesNotMatch(navigation, /saves only a subset of fields/);
  assert.match(navigation, /only verified workflows are writable/);
});
