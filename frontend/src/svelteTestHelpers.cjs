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

function serverComponent(source, filename, dependencies, environment = {}) {
  const compiled = compile(source, { filename: path.resolve(__dirname, filename), generate: 'server' });
  assert.equal(compiled.warnings.length, 0);
  const module = { exports: {} };
  const code = ts.transpileModule(compiled.js.code.replaceAll('import.meta.env', '__testEnvironment'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
  }).outputText;
  vm.runInNewContext(code, {
    module, exports: module.exports, structuredClone, __testEnvironment: environment,
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
      if (name === './ProjectMetadataEditor.svelte' && environment.VITE_PROJECT_METADATA_EDITING !== 'true') {
        return { default: () => { throw new Error('Metadata editor rendered while its explicit test profile is disabled.'); } };
      }
      if (name === './summaryVegetationPreferenceSessions' && environment.VITE_SUMMARY_VEGETATION_PREFERENCES !== 'true') {
        return { summaryVegetationPreferenceSession: null };
      }
      if (name === './summaryVegetationQualityPreferenceSessions' && environment.VITE_SUMMARY_VEGETATION_QUALITY_PREFERENCES !== 'true') {
        return { summaryVegetationQualityPreferenceSession: null };
      }
      if (name === './summaryVegetationPresentationPreferenceSessions' && environment.VITE_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES !== 'true') {
        return { summaryVegetationPresentationPreferenceSession: null };
      }
      if (name === './summaryVegetationFieldListDesktop' && environment.VITE_SUMMARY_VEGETATION_FIELD_LIST !== 'true') {
        return { summaryVegetationFieldListDesktop: { enabled: false, contextAvailable: () => false } };
      }
      if (name === './summaryVegetationEnvironmentalDesktop' && environment.VITE_SUMMARY_VEGETATION_ENVIRONMENT !== 'true') {
        return { summaryEnvironmentalEnabled: false, summaryEnvironmentalDesktop: {}, summaryEnvironmentalPort: {} };
      }
      if (name === './SummaryVegetationWorkbookPanel.svelte' && environment.VITE_SUMMARY_VEGETATION_WORKBOOK !== 'true') {
        return { default: () => { throw new Error('Inspection workbook rendered while its explicit test profile is disabled.'); } };
      }
      if (name === './SummaryVegetationEnvironmental.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'SummaryVegetationEnvironmental.svelte'), 'utf8'),
        'SummaryVegetationEnvironmental.svelte', dependencies, environment) };
      if (name === './SummaryVegetationFieldList.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'SummaryVegetationFieldList.svelte'), 'utf8'),
        'SummaryVegetationFieldList.svelte', dependencies, environment) };
      if (name === './summaryVegetationFieldList') return loadTypeScript('summaryVegetationFieldList.ts');
      if (name === './SummaryVegetationPresentationPreferences.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'SummaryVegetationPresentationPreferences.svelte'), 'utf8'),
        'SummaryVegetationPresentationPreferences.svelte', dependencies, environment) };
      if (name === './SummaryVegetationQualityPreferences.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'SummaryVegetationQualityPreferences.svelte'), 'utf8'),
        'SummaryVegetationQualityPreferences.svelte', {
          ...dependencies, './summaryVegetationQualityPreferences': qualityPreferenceHelpers().preferences,
        }, environment) };
      if (name === './longVegetationProjectWorkspaces' && environment.VITE_LONG_VEGETATION_PROJECT_SCOPE !== 'true') {
        return { projectReportWorkspace: null };
      }
      if (name === './LongVegetationProjectScopeSelection.svelte' || name === './LongVegetationProjectUnitPreferences.svelte') {
        const filename = name.slice(2);
        return { default: serverComponent(readFileSync(path.join(__dirname, filename), 'utf8'), filename, dependencies, environment) };
      }
      if (name === './FieldGuidance.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'FieldGuidance.svelte'), 'utf8'), 'FieldGuidance.svelte', {}) };
      if (name === './SIVIChildSourceNotices.svelte') return { default: serverComponent(
        readFileSync(path.join(__dirname, 'SIVIChildSourceNotices.svelte'), 'utf8'), 'SIVIChildSourceNotices.svelte', {}) };
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
    if (name === './qualityMinimumReference') return qualityPreferenceHelpers().reference;
    if (name === './siviChildSourceNotices') {
      const quality = dependencies['./qualityEditor'] || loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
      const metadata = dependencies['./projectMetadataEditor'] || loadTypeScript('projectMetadataEditor.ts', {
        './qualityEditor': quality, '../../resources/project-metadata-standard.json': [], '../../resources/project-metadata-template.json': [],
      });
      return loadTypeScript('siviChildSourceNotices.ts', {
        './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
        './projectMetadataEditor': metadata,
        './projectMetadataRestore': dependencies['./projectMetadataRestore'] || loadTypeScript('projectMetadataRestore.ts', {
          './qualityEditor': quality, './projectMetadataEditor': metadata,
        }),
      });
    }
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
  const defaultSoilReviewProfile = filename === 'App.svelte' || filename === 'FS882Form.svelte'
    ? { twoPageSoilBusy: false, twoPageSoilBlocked: false, ordinarySoilUnavailable: null,
      twoPageVegetationBusy: false, twoPageVegetationBlocked: false, ordinaryVegetationUnavailable: null } : {};
  const context = vm.createContext({ exports: {}, __testEnvironment: {},
    ...(filename === 'TwoPageEntryEditor.svelte' ? { vegetationTableOpen: false, vegetationTableBusy: false, environmentTableOpen: false, environmentTableBusy: false, projectMetadataOpen: false, projectMetadataBusy: false, profilingOpen: false, profilingBusy: false } : {}),
    ...(filename === 'SummaryVegetationReport.svelte' ? {
      preferences: null, quality: null, qualityView: null, qualityReadBusy: false,
      qualityMutationHeld: false, qualityInvalid: false, qualityUnavailable: false,
      presentation: null, presentationView: null, presentationReadBusy: false,
      presentationMutationHeld: false, presentationInvalid: false,
      fieldListHeld: false, fieldListBusy: false, environmentalHeld: false,
    } : {}),
    ...(filename === 'App.svelte' ? { projectReportBusy: false, summaryPreferenceBusy: false, summaryFieldListBusy: false, summaryEnvironmentalBusy: false, externalSUEnvironmentBusy: false } : {}), ...defaultSoilReviewProfile, ...values });
  const code = selected.map(node => node.getText(ast)).join('\n').replaceAll('import.meta.env', '__testEnvironment') +
    `\nthis.actions = {${names.join(',')}};`;
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

function qualityPreferenceHelpers() {
  const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
  const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
  const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
  const earth = loadTypeScript('googleEarthReview.ts', {
    './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
  });
  const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
  const lifeform = loadTypeScript('lifeformSummary.ts');
  const transport = loadTypeScript('siviParentTransport.ts', {
    './projectMetadataRestore': cells, './siviParentEditor': {}, './siviParentSession': {},
  });
  const reference = loadTypeScript('qualityMinimumReference.ts', {
    './lifeformSummary': lifeform, './projectMetadataRestore': cells, './siviParentTransport': transport,
  });
  return { reference, preferences: loadTypeScript('summaryVegetationQualityPreferences.ts', {
    './googleEarthReview': earth, './lifeformSummary': lifeform, './qualityEditor': quality,
    './qualityMinimumReference': reference, './reportTitlePreferences': shared,
  }) };
}

module.exports = { serverComponent, loadTypeScript, componentFunctions, presentationHelpers };
