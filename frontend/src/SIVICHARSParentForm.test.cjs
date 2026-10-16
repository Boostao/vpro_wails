const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { metadata, cell, setCell } = require('./siviParentTestHelpers.cjs');
const wire = require('../../testdata/chars-sivi-entry-client-wire.json');
const source = require('../../resources/fs1333-chars-sivi-layout.json');
const code = name => readFileSync(path.join(__dirname, name), 'utf8');

test('compiled CHARS presentation consumes the exact source and renders every binding once with associated visible labels', () => {
  for (const booleanValue of [cell(), cell('integer', '-1'), cell('integer', '0')]) {
    const original = structuredClone(wire.Original);
    setCell(original, 'SV_FloodPlain', booleanValue);
  for (const flag of [undefined, 'false', 'TRUE', 'true']) {
    let snapshots = 0;
    const component = serverComponent(code('SIVICHARSParentForm.svelte'), 'SIVICHARSParentForm.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { SIVICHARSParentEntryService: {},
        AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' } },
      './siviCHARSParentEntry': { siviCHARSParentFields: source.forms[0].fields.filter(field => field.binding),
        siviCHARSParentSpeciesAction: 'form:frmSIVIsite-CHARS/optSpeciesListComplete' },
      './referenceEditor': { siviReferencePolicies: [] },
      './projectMetadataEditor': metadata,
      '../../resources/fs1333-chars-sivi-layout.json': { default: source },
      './siviCHARSParentEntrySession': { SIVICHARSParentEntrySession: class {
        view() {
          snapshots++;
          return { original, snapshot: { policies: wire.Policies, fields: [] },
            drafts: {}, advisories: {}, busy: false, blocked: false, historyId: null, error: null };
        }
        closeState() { return { unsaved: false, busy: false, canSave: true, saveReason: '', error: null }; }
        dispose() {}
      } },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: flag });
    const props = { contextId: 'owned', project: 'Sample', plotNumber: '108050' };
    const html = render(component, { props }).body;
    assert.ok(snapshots > 0, 'consume the lazy render before claiming profile coverage');
    const fields = [...html.matchAll(/data-chars-field="([^"]+)"/g)].map(match => match[1]);
    assert.equal(fields.length, flag === 'true' ? 71 : 0);
    if (flag !== 'true') {
      assert.match(html, /disabled in this build/);
      continue;
    }
    for (const metadataAvailable of [false, true]) for (const onMetadata of [undefined, () => {}]) {
      const metadata = render(component, { props: { ...props, metadataAvailable, onMetadata } }).body;
      const button = metadata.match(/<button\b[^>]*data-chars-metadata-open[^>]*>[^<]*<\/button>/)?.[0];
      assert.equal(Boolean(button), Boolean(onMetadata));
      if (button) {
        assert.match(button, /data-source-control="form:frmSIVIsite-CHARS\/btnEditMetadata"/);
        assert.match(button, />Edit Metadata<\/button>/);
        assert.equal(/\sdisabled(?:=|\s|>)/.test(button), !metadataAvailable);
      }
    }
    for (const available of [false, true]) {
      const recovery = render(component, { props: { ...props, peerBlockedReason: 'Retained creator recovery',
        peerUnlockAvailable: available, creationAvailable: true } }).body;
      const lock = recovery.match(/<button\b[^>]*data-chars-lock[^>]*>/)?.[0];
      assert.ok(lock);
      assert.equal(/\sdisabled(?:=|\s|>)/.test(lock), !available, 'only recovery permits unlocking a clean remounted parent');
      assert.match(recovery, /data-chars-save[^>]*\sdisabled(?:=|\s|>)/);
      assert.match(recovery, /id="chars-RootZoneParticleSize"[^>]*\sdisabled(?:=|\s|>)/);
    }
    assert.equal(new Set(fields).size, 71);
    for (const field of source.forms[0].fields.filter(field => field.binding)) {
      assert.ok(html.includes(`data-source-control="${field.controlId}"`));
      assert.ok(html.includes(`for="chars-${field.binding}"`));
      assert.ok(html.includes(`id="chars-${field.binding}"`));
    }
    for (const group of ['Plot &amp; survey', 'Site description', 'Topography', 'STAND', 'TERRAIN', 'SOIL', 'Vegetation', 'Notes']) {
      assert.ok(html.includes(`aria-label="${group}"`), group);
    }
    assert.match(html, /Root Restricting Depth/);
    assert.match(html, /child vegetation workflows/);
    assert.match(html, /min-w-0/);
    assert.match(html, /grid-cols-1[^"]*sm:grid-cols-2[^"]*xl:grid-cols-3/);
    const binding = wire.Policies.find(policy => policy.column === 'SV_FloodPlain').binding;
    const boolean = [...html.matchAll(/<select\b[^>]*>[\s\S]*?<\/select>/g)]
      .map(match => match[0]).find(select => select.includes(`id="chars-${binding}"`));
    assert.ok(boolean, 'resolve the actual Boolean control before checking its commands');
    const nullOption = boolean.match(/<option\b[^>]*value="null"[^>]*>/)?.[0];
    assert.ok(nullOption);
    assert.equal(/\bdisabled(?:\s|=|>)/.test(nullOption), booleanValue.storage !== 'null');
    assert.match(boolean, /Retain historical NULL/);
    assert.match(boolean, /Yes \(-1\)/);
    assert.match(boolean, /No \(0\)/);
    const action = [...html.matchAll(/<select\b[^>]*>[\s\S]*?<\/select>/g)]
      .map(match => match[0]).find(select => select.includes('id="chars-SpeciesListComplete"'));
    assert.ok(action, 'resolve the actual implicit source choice before checking options');
    assert.match(action, /value="1">Comp\./);
    assert.match(action, /value="2">Part\./);
    assert.doesNotMatch(action, /value="null"/);
    assert.equal((html.match(/data-chars-action="SpeciesListComplete"/g) ?? []).length, 1);
  }
  }
});

test('compiled linked recovery remains rendered exactly once when parent originals are unavailable', () => {
  for (const available of [false, true]) {
    let rendered = 0;
    const parent = serverComponent(code('SIVICHARSParentForm.svelte'), 'SIVICHARSParentForm.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { SIVICHARSParentEntryService: {}, AuditRestoreAction: {} },
      './siviCHARSParentEntry': { siviCHARSParentFields: source.forms[0].fields.filter(field => field.binding),
        siviCHARSParentSpeciesAction: 'form:frmSIVIsite-CHARS/optSpeciesListComplete' },
      './referenceEditor': { siviReferencePolicies: [] }, './projectMetadataEditor': metadata,
      '../../resources/fs1333-chars-sivi-layout.json': { default: source },
      './siviCHARSParentEntrySession': { SIVICHARSParentEntrySession: class {
        view() { return { original: available ? wire.Original : null,
          snapshot: available ? { policies: wire.Policies, fields: [] } : null,
          drafts: {}, advisories: {}, busy: false, blocked: false, historyId: null, error: null }; }
        closeState() { return { unsaved: false, busy: false, blocked: false, canSave: available, saveReason: '', error: null }; }
        dispose() {}
      } },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: 'true' });
    const host = serverComponent(`<script>import Parent from 'parent'; import { record } from 'proof';</script>
      {#snippet linked()}{record()}<span data-linked-recovery>Retained linked recovery</span>{/snippet}
      <Parent contextId="owned" project="Sample" plotNumber="108050" linkedVegetation={linked} />`,
    'linked-recovery-host.svelte', { parent: { default: parent }, proof: { record: () => { rendered++; return ''; } } });
    const html = render(host).body;
    assert.equal(rendered, 1);
    assert.equal((html.match(/data-linked-recovery/g) ?? []).length, 1);
    assert.equal((html.match(/data-chars-field=/g) ?? []).length, available ? 71 : 0);
    if (available) {
      assert.ok(html.indexOf('aria-label="Vegetation"') < html.indexOf('data-linked-recovery'));
      assert.ok(html.indexOf('data-linked-recovery') < html.indexOf('aria-label="Notes"'));
    }
  }
});

test('actual CHARS route needs its exact independent gate and defers ownership replacement until transition completion', async () => {
  let pending;
  const state = componentFunctions('App.svelte', ['openSIVI'], {
    siviStandaloneEnabled: false, siviCHARSParentEntryEnabled: false,
    selected: { plotNumber: ' P ' }, $projectState: { contextId: 'owned', activeProject: 'Project' },
    error: '', editorPlotNumber: 'original', editorEntryForm: 'fs882', view: 'plots',
    requestTransition: async run => { pending = run; },
  });
  await state.actions.openSIVI('sivi-chars');
  assert.equal(pending, undefined);
  assert.match(state.error, /independent gate/);
  state.siviCHARSParentEntryEnabled = true;
  await state.actions.openSIVI('sivi-chars');
  assert.equal(state.editorEntryForm, 'fs882');
  state.$projectState.contextId = 'new owner';
  await assert.rejects(pending(), /context\/project changed/);
  assert.equal(state.editorEntryForm, 'fs882');
  state.$projectState.contextId = 'owned';
  await state.actions.openSIVI('sivi-chars');
  await pending();
  assert.equal(state.editorEntryForm, 'sivi-chars');
  assert.equal(state.editorPlotNumber, ' P ');
});

test('compiled CHARS menu is independently gated and discloses its exact parent/linked scope', () => {
  for (const combined of [undefined, 'false', 'TRUE', 'true']) {
  const component = serverComponent(code('Navigation.svelte'), 'Navigation.svelte', {
    '@lucide/svelte': new Proxy({}, { get: () => () => {} }),
  });
  for (const available of [false, true]) {
    for (const callback of [undefined, () => {}]) {
      for (const collected of [false, true]) for (const species of [false, true]) for (const identity of [false, true]) {
      const html = render(component, { props: { view: 'plots', onnavigate: () => {},
        charsAvailable: available, onopenchars: callback, charsLinkedVegetation: combined === 'true',
        charsCollected: collected, charsSpecies: species, charsIdentity: identity } }).body;
      const match = html.match(/<button\b[^>]*>[\s\S]*?<span>CHARS SIVI parent — Experimental<\/span>\s*<\/button>/)?.[0];
      assert.ok(match);
      const button = match.slice(match.lastIndexOf('<button'));
      assert.equal(/\bdisabled(?:\s|=|>)/.test(button), !(available && callback));
      assert.match(button, available && callback
        ? combined === 'true' ? /separately gated linked covers\/heights/
          : collected ? /separately gated linked Collected cycles/
            : species ? /separately gated linked Species selections/
              : identity ? /separately gated linked vegetation IDs/ : /parent-only entry/
        : /independent CHARS parent-entry gate/);
      assert.equal(button.includes('Collected cycles'), Boolean(available && callback && collected));
      assert.equal(button.includes('Species selections'), Boolean(available && callback && species));
      assert.equal(button.includes('vegetation IDs'), Boolean(available && callback && identity));
      }
    }
  }
  }
});

test('actual CHARS menu delegates once only after restoring owned navigation focus', () => {
  const calls = [];
  const state = componentFunctions('Navigation.svelte', ['itemEnabled', 'activate', 'closeMenu', 'navigate'], {
    charsAvailable: false, onopenchars: undefined, openMenu: 'Forms',
    navigation: { querySelector: () => ({ querySelector: () => ({ focus: () => calls.push('focus') }) }) },
    onnavigate: view => calls.push(view),
  });
  state.actions.activate({ action: 'sivi-chars' });
  state.charsAvailable = true;
  state.actions.activate({ action: 'sivi-chars' });
  assert.deepEqual(calls, []);
  state.onopenchars = () => { assert.equal(state.openMenu, null); calls.push('chars'); };
  state.actions.activate({ action: 'sivi-chars' });
  assert.deepEqual(calls, ['focus', 'chars']);
});

test('CHARS shares the root native-close and navigation interface without replacing the normal FS882 host', () => {
  const app = code('App.svelte'), form = code('SIVICHARSParentForm.svelte');
  assert.match(app, /siviCHARSParentEntryEnabled = import\.meta\.env\.VITE_SIVI_CHARS_PARENT_ENTRY === 'true'/);
  assert.match(app, /\{#if editorEntryForm === 'sivi-chars'\}[\s\S]*?<SIVICHARSForm bind:this=\{editor\}[\s\S]*?\{:else\}[\s\S]*?<FS882Form/);
  assert.match(form, /\$state\.raw<ReturnType<SIVICHARSParentEntrySession\['view'\]>/);
  for (const method of ['getCloseState', 'saveForClose', 'canOpenProjectMetadata', 'openProjectMetadata']) {
    assert.match(form, new RegExp(`export (?:async )?function ${method}\\(`));
  }
  assert.match(form, /currentTarget\.value/);
  assert.doesNotMatch(form, /window\.close|ConfirmClose|CancelClose/);
});
