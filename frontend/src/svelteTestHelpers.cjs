const assert = require('node:assert/strict');
const vm = require('node:vm');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const serverReads = { ReadRequests: class {
  track(request) { return request; }
  cancelAll() {}
} };

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
      if (name === './readRequests') return serverReads;
      if (name === './ordinaryEditor') return loadTypeScript('ordinaryEditor.ts', {
        './qualityEditor': dependencies['./qualityEditor'] || loadTypeScript('qualityEditor.ts', {
          './becEditor': loadTypeScript('becEditor.ts')
        }),
        './numericEditor': loadTypeScript('numericEditor.ts')
      });
      if (name === './otherEditor') return loadTypeScript('otherEditor.ts', {
        './qualityEditor': dependencies['./qualityEditor'] || loadTypeScript('qualityEditor.ts', {
          './becEditor': loadTypeScript('becEditor.ts')
        })
      });
      if (name === './soilChildEditor') return loadTypeScript('soilChildEditor.ts', {
        './qualityEditor': dependencies['./qualityEditor'] || loadTypeScript('qualityEditor.ts', {
          './becEditor': loadTypeScript('becEditor.ts')
        }),
        './numericEditor': loadTypeScript('numericEditor.ts')
      });
      if (name === './vegetationAttributeEditor') return loadTypeScript('vegetationAttributeEditor.ts', {
        './numericEditor': loadTypeScript('numericEditor.ts')
      });
      if (name === './catalogueLookup' && './qualityEditor' in dependencies) {
        return loadTypeScript('catalogueLookup.ts', {
          './qualityEditor': dependencies['./qualityEditor'],
          './readRequests': serverReads
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

function loadTypeScript(filename, dependencies = {}, globals = {}) {
  const module = { exports: {} };
  vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, filename), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true }
  }).outputText, { ...globals, module, exports: module.exports, structuredClone, require(name) {
    if (name in dependencies) return dependencies[name];
    throw new Error(`Unexpected TypeScript dependency ${name}`);
  } });
  return module.exports;
}
function componentFunctions(filename, names, values) {
  const source = readFileSync(path.join(__dirname, filename), 'utf8');
  const script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile(filename + '.ts', script, ts.ScriptTarget.Latest, true);
  const selected = ast.statements.filter(node => ts.isFunctionDeclaration(node) && names.includes(node.name.text));
  assert.equal(selected.length, names.length);
  const context = vm.createContext({ exports: {}, ...values });
  const code = selected.map(node => node.getText(ast)).join('\n') + `\nthis.actions = {${names.join(',')}};`;
  vm.runInContext(ts.transpileModule(code, { compilerOptions: {
    module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022
  } }).outputText, context);
  return context;
}
function presentationHelpers() {
  const paper = loadTypeScript('paperLayout.ts', {
    '../../resources/fs882-xl-layout.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'fs882-xl-layout.json'), 'utf8')),
    '../../resources/fs882-extended-shrub-layout.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'fs882-extended-shrub-layout.json'), 'utf8'))
  });
  return { paper, presentation: loadTypeScript('formPresentation.ts', { './paperLayout': paper }) };
}

module.exports = { serverComponent, loadTypeScript, componentFunctions, presentationHelpers };
