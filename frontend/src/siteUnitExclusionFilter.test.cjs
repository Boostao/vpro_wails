const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const wailsStub = {
  Call: {
    ByName: () => Promise.resolve(null),
  },
};

const filter = loadTypeScript('siteUnitExclusionFilter.ts', {
  '@wailsio/runtime': wailsStub,
});
const plain = value => JSON.parse(JSON.stringify(value));

test('default BEC SiteUnit exclusion presets are .support, .poor, and .nudum', () => {
  assert.deepEqual(plain(filter.DEFAULT_SITE_UNIT_EXCLUSIONS), ['.support', '.poor', '.nudum']);
  const defaults = filter.createDefaultSiteUnitExclusionRules();
  assert.equal(defaults.length, 3);
  assert.equal(defaults.every(r => r.enabled && r.matchMode === 'auto'), true);
});

test('matchSiteUnitAgainstRules matches case-insensitive suffixes, dot-segments, phrases, and wildcards without false positives', () => {
  const rules = filter.createDefaultSiteUnitExclusionRules();
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('CWHvm1/01.support', rules)), {
    excluded: true,
    matchedPattern: '.support',
  });
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('CWHvm1/01.SUPPORT-a', rules)), {
    excluded: true,
    matchedPattern: '.support',
  });
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('SBSmk1/05.POOR', rules)), {
    excluded: true,
    matchedPattern: '.poor',
  });
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('IDFdk1/03.nudum', rules)), {
    excluded: true,
    matchedPattern: '.nudum',
  });
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('SupportCreek/01', rules)), {
    excluded: false,
    matchedPattern: '',
  });
  assert.deepEqual(plain(filter.matchSiteUnitAgainstRules('CWHvm1/01', rules)), {
    excluded: false,
    matchedPattern: '',
  });

  const custom = [
    { pattern: 'CWH*/99*', matchMode: 'wildcard', enabled: true },
    { pattern: 'transitional', matchMode: 'contains', enabled: true },
    { pattern: '.draft', matchMode: 'suffix', enabled: false },
  ];
  assert.equal(filter.matchSiteUnitAgainstRules('CWHvm2/99b', custom).excluded, true);
  assert.equal(filter.matchSiteUnitAgainstRules('MHmm1/01 Transitional', custom).excluded, true);
  assert.equal(filter.matchSiteUnitAgainstRules('MHmm1/01.draft', custom).excluded, false);
});

test('SiteUnitExclusionFilter.svelte compiles with zero warnings, renders default chips and ~ breakpoint controls, and has zero emojis', () => {
  const svelteSource = readFileSync(path.join(__dirname, 'SiteUnitExclusionFilter.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(svelteSource), false, 'must contain zero emojis');

  const Component = serverComponent(svelteSource, 'SiteUnitExclusionFilter.svelte', {
    './siteUnitExclusionFilter': filter,
  });

  const rendered = render(Component, {
    props: {
      contextId: 'ctx-1',
      su: 'Sample',
      hierarchy: 'Sample',
      reportMode: 'summary',
      initialUseBreakpoints: true,
    },
  });
  assert.match(rendered.body, /\.support/);
  assert.match(rendered.body, /\.poor/);
  assert.match(rendered.body, /\.nudum/);
  assert.match(rendered.body, /Nested '~' Breakpoint Policy/);
});
