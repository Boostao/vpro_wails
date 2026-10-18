const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');

const frontend = path.resolve(__dirname, '..');
const baselineBytes = 5_202_547;
const previewFlags = [
  'LONG_ENVIRONMENT_PREFERENCES', 'LONG_ENVIRONMENT_REPORT', 'LONG_ENVIRONMENT_WORKBOOK',
  'LONG_VEGETATION_CODE', 'LONG_VEGETATION_PREFERENCES', 'LONG_VEGETATION_REPORT',
  'LONG_VEGETATION_STRATA', 'LONG_VEGETATION_WORKBOOK',
  'PROJECT_METADATA_EDITING', 'PROJECT_METADATA_RESTORE',
  'SIVI_CHARS_PARENT_ENTRY', 'SIVI_COLLECTED_EDITING', 'SIVI_COMBINED_EDITING',
  'SIVI_CREATION_UNDO', 'SIVI_CREATION_WRITING', 'SIVI_DELETION_RESTORING',
  'SIVI_DELETION_WRITING', 'SIVI_IDENTITY_EDITING', 'SIVI_PARENT_REVIEW',
  'SIVI_SPECIES_EDITING', 'SIVI_STANDALONE', 'SUBZONE_MATRIX_WORKBOOK',
  'SUMMARY_VEGETATION', 'SUMMARY_VEGETATION_ENVIRONMENT',
  'SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM', 'SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC',
  'SUMMARY_VEGETATION_FIELD_LIST', 'SUMMARY_VEGETATION_PREFERENCES',
  'SUMMARY_VEGETATION_PRESENTATION_PREFERENCES', 'SUMMARY_VEGETATION_QUALITY_PREFERENCES',
  'USER_LOG_REVIEW',
];

test('entry panels compile as guarded deferred components with explicit loading and failure states', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const compiled = compile(source, { filename: 'App.svelte', generate: 'client', dev: false });
  assert.deepEqual(compiled.warnings, []);
  for (const [component, owner] of [['FS882Form', 'fs882'], ['SIVICHARSForm', 'sivi-chars']]) {
    assert.doesNotMatch(source, new RegExp(`import ${component} from`));
    assert.match(compiled.js.code, new RegExp(`import\\(['"]\\./${component}\\.svelte['"]\\)`));
    assert.match(source, new RegExp(`role="status" data-entry-loading="${owner}"`));
    assert.match(source, new RegExp(`role="alert" data-entry-load-error="${owner}"`));
  }
  assert.match(source, /\{#key \$projectState\?\.contextId\}[\s\S]*?\{#key editorEntryForm\}[\s\S]*?\{#key editorPlotNumber\}/);
  assert.equal((source.match(/bind:this=\{editor\}/g) ?? []).length, 2);
});

test('only a resolved module awaiting its editor binding blocks navigation and close', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const expression = source.match(/const editorInitializing = \$derived\(([\s\S]*?)\);/);
  assert.ok(expression);
  const initializing = new Function('view', 'editor', 'editorEntryForm',
    'fs882ModuleLoaded', 'siviCHARSModuleLoaded', `return (${expression[1]});`);
  for (const form of ['xl', 'sivi', 'sivi-chars']) {
    for (const loaded of [false, true]) {
      const fs882 = form === 'sivi-chars' ? !loaded : loaded;
      const chars = form === 'sivi-chars' ? loaded : !loaded;
      assert.equal(initializing('fs882', undefined, form, fs882, chars), loaded);
      assert.equal(initializing('fs882', {}, form, fs882, chars), false);
      assert.equal(initializing('home', undefined, form, fs882, chars), false);
    }
  }
  assert.equal((source.match(/closeDisposition\([^\n]*editorInitializing/g) ?? []).length, 3);
  assert.match(source, /await import\('\.\/FS882Form\.svelte'\);\s*fs882ModuleLoaded = true/);
  assert.match(source, /await import\('\.\/SIVICHARSForm\.svelte'\);\s*siviCHARSModuleLoaded = true/);
});

test('actual preview build halves initial transitive JS without removing entry layouts or inflating total JS', async t => {
  const saved = new Map(Object.entries(process.env).filter(([name]) => name.startsWith('VITE_')));
  for (const name of saved.keys()) delete process.env[name];
  for (const name of previewFlags) process.env[`VITE_${name}`] = 'true';
  try {
    assert.equal(previewFlags.length, 31);
    const { build } = await import('vite');
    const result = await build({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      mode: 'production',
      logLevel: 'warn',
      build: { write: false, emptyOutDir: false, sourcemap: false },
    });
    assert.ok(!Array.isArray(result) && Array.isArray(result.output));
    const chunks = new Map(result.output.filter(output => output.type === 'chunk')
      .map(chunk => [chunk.fileName, chunk]));
    const entries = [...chunks.values()].filter(chunk => chunk.isEntry);
    assert.equal(entries.length, 1);
    const initial = new Set();
    function visit(name) {
      if (initial.has(name)) return;
      const chunk = chunks.get(name);
      assert.ok(chunk, `Missing static JavaScript dependency ${name}`);
      initial.add(name);
      for (const dependency of chunk.imports) visit(dependency);
    }
    visit(entries[0].fileName);
    const bytes = names => [...names].reduce((sum, name) =>
      sum + Buffer.byteLength(chunks.get(name).code, 'utf8'), 0);
    const initialBytes = bytes(initial);
    const totalBytes = bytes(chunks.keys());
    t.diagnostic(JSON.stringify({ baselineBytes, initialBytes, totalBytes,
      initialChunks: [...initial], deferredChunks: chunks.size - initial.size }));
    assert.ok(initialBytes <= baselineBytes * .5, `Initial transitive JS ${initialBytes} exceeds half of ${baselineBytes}`);
    assert.ok(totalBytes <= baselineBytes * 1.1, `Total JS ${totalBytes} exceeds 110% of ${baselineBytes}`);
    const modules = names => new Set([...names].flatMap(name =>
      Object.keys(chunks.get(name).modules).map(id => id.replaceAll('\\', '/'))));
    const eager = modules(initial), all = modules(chunks.keys());
    for (const filename of ['FS882Form.svelte', 'SIVICHARSForm.svelte',
      'fs882-xl-layout.json', 'fs882-two-page-layout.json', 'fs882-two-page-chars-layout.json',
      'fs1333-sivi-layout.json', 'fs1333-chars-sivi-layout.json', 'project-metadata-layout.json']) {
      const suffix = '/' + filename;
      assert.ok([...all].some(id => id.endsWith(suffix)), `Required entry source disappeared: ${filename}`);
      if (filename !== 'fs1333-sivi-layout.json') {
        assert.ok(![...eager].some(id => id.endsWith(suffix)), `Entry source still loads at startup: ${filename}`);
      }
    }
    assert.ok([...eager].some(id => id.endsWith('/fs1333-sivi-layout.json')),
      'Retain the source parent validator shared by existing log/picture wire readers');
  } finally {
    for (const name of Object.keys(process.env).filter(name => name.startsWith('VITE_'))) delete process.env[name];
    for (const [name, value] of saved) process.env[name] = value;
  }
});
