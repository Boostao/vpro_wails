const assert = require('node:assert/strict');
const vm = require('node:vm');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');

function serverComponent(source, filename, dependencies) {
  const compiled = compile(source, { filename: path.resolve(__dirname, filename), generate: 'server' });
  assert.equal(compiled.warnings.length, 0);
  const module = { exports: {} };
  const code = ts.transpileModule(compiled.js.code, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
  }).outputText;
  vm.runInNewContext(code, {
    module, exports: module.exports,
    require(name) {
      if (name in dependencies) return dependencies[name];
      if (name === './catalogueLookup' && './qualityEditor' in dependencies) {
        return loadTypeScript('catalogueLookup.ts', {
          './qualityEditor': dependencies['./qualityEditor'],
          './readRequests': { ReadRequests: class {
            track(request) { return request; }
            cancelAll() {}
          } }
        });
      }
      if (name === 'svelte' || name.startsWith('svelte/')) return require(name);
      if (name === './FieldGuidance.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'FieldGuidance.svelte'), 'utf8'), 'FieldGuidance.svelte', {}) };
      throw new Error(`Unexpected renderer dependency ${name}`);
    }
  });
  return module.exports.default;
}

function loadTypeScript(filename, dependencies = {}) {
  const module = { exports: {} };
  vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, filename), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true }
  }).outputText, { module, exports: module.exports, structuredClone, require(name) {
    if (name in dependencies) return dependencies[name];
    throw new Error(`Unexpected TypeScript dependency ${name}`);
  } });
  return module.exports;
}
function presentationHelpers() {
  const paper = loadTypeScript('paperLayout.ts', {
    '../../resources/fs882-xl-layout.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'fs882-xl-layout.json'), 'utf8'))
  });
  return { paper, presentation: loadTypeScript('formPresentation.ts', { './paperLayout': paper }) };
}

module.exports = { serverComponent, loadTypeScript, presentationHelpers };
