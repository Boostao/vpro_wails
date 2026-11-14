import { spawn } from 'node:child_process';
import { readdirSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const testsRoot = path.join(frontendRoot, 'tests');

const BROWSER_CONDITION_SUITES = new Set([
  'HomeWorkspaceHub.test.cjs',
  'siteUnitSummaryWorkbookIntegration.test.cjs',
  'summaryVegetationEnvironmental.test.cjs',
  'summaryVegetationEnvironmentalCanonical.test.cjs',
  'summaryVegetationEnvironmentalLifeformPreview.test.cjs',
  'summaryVegetationEnvironmentalNumeric.test.cjs',
  'summaryVegetationEnvironmentalNumericPreview.test.cjs',
  'summaryVegetationEnvironmentalPanel.test.cjs',
  'summaryVegetationFieldList.test.cjs',
  'summaryVegetationFieldListDesktop.test.cjs',
  'summaryVegetationFieldListPane.test.cjs',
  'summaryVegetationWorkbook.test.cjs',
  'twoPageVegetationSpeciesList.test.cjs',
]);

function discoverTestFiles(dir) {
  const found = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      found.push(...discoverTestFiles(full));
    } else if (entry.name.endsWith('.test.cjs')) {
      found.push(path.relative(frontendRoot, full).replaceAll('\\', '/'));
    }
  }
  return found.sort();
}

function runBatch(label, nodeArgs) {
  if (nodeArgs.length === 0) {
    return Promise.resolve({ label, code: 0, stdout: '', stderr: '' });
  }
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, nodeArgs, {
      cwd: frontendRoot,
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', chunk => { stdout += chunk.toString('utf8'); });
    child.stderr.on('data', chunk => { stderr += chunk.toString('utf8'); });
    child.on('error', reject);
    child.on('close', code => resolve({ label, code: code ?? 1, stdout, stderr }));
  });
}

function parseTapCounts(output) {
  const get = key => {
    const matches = [...output.matchAll(new RegExp(`^# ${key} (\\d+)\\s*$`, 'gm'))];
    return matches.length > 0 ? Number(matches[matches.length - 1][1]) : 0;
  };
  return {
    tests: get('tests'),
    pass: get('pass'),
    fail: get('fail'),
  };
}

const filters = process.argv.slice(2).filter(Boolean);
const allDiscovered = discoverTestFiles(testsRoot);

const runnable = allDiscovered.filter(rel => {
  const base = path.posix.basename(rel);
  if (base === 'siviParentWire.test.cjs' && !process.env.VPRO_SIVI_PARENT_WIRE_RECEIPT && filters.length === 0) {
    return false;
  }
  if (filters.length === 0) return true;
  const lower = rel.toLowerCase();
  return filters.some(pattern => lower.includes(pattern.toLowerCase()));
});

if (runnable.length === 0) {
  console.error(`No frontend test suites matched filter: ${filters.join(', ')}`);
  process.exit(1);
}

const browserSuites = [];
const standardSuites = [];
const seen = new Set();

for (const rel of runnable) {
  if (seen.has(rel)) {
    console.error(`Duplicate test suite detected: ${rel}`);
    process.exit(1);
  }
  seen.add(rel);
  if (BROWSER_CONDITION_SUITES.has(path.posix.basename(rel))) {
    browserSuites.push(rel);
  } else {
    standardSuites.push(rel);
  }
}

const startedAt = Date.now();
const cpuCount = typeof os.availableParallelism === 'function' ? os.availableParallelism() : os.cpus().length;
const bothActive = standardSuites.length > 0 && browserSuites.length > 0;
const stdConcurrency = bothActive ? Math.max(2, Math.ceil(cpuCount * 0.65)) : Math.max(2, cpuCount - 1);
const brwConcurrency = bothActive ? Math.max(2, Math.floor(cpuCount * 0.35)) : Math.max(2, cpuCount - 1);

const [standardResult, browserResult] = await Promise.all([
  standardSuites.length > 0
    ? runBatch('standard', [`--test-concurrency=${stdConcurrency}`, '--test', ...standardSuites])
    : Promise.resolve({ label: 'standard', code: 0, stdout: '', stderr: '' }),
  browserSuites.length > 0
    ? runBatch('browser', ['--conditions=browser', `--test-concurrency=${brwConcurrency}`, '--test', ...browserSuites])
    : Promise.resolve({ label: 'browser', code: 0, stdout: '', stderr: '' }),
]);

for (const res of [standardResult, browserResult]) {
  if (res.stdout) process.stdout.write(res.stdout);
  if (res.stderr) process.stderr.write(res.stderr);
}

const stdCounts = parseTapCounts(standardResult.stdout + '\n' + standardResult.stderr);
const brwCounts = parseTapCounts(browserResult.stdout + '\n' + browserResult.stderr);
const totalTests = stdCounts.tests + brwCounts.tests;
const totalPass = stdCounts.pass + brwCounts.pass;
const totalFail = stdCounts.fail + brwCounts.fail;
const elapsedSec = ((Date.now() - startedAt) / 1000).toFixed(2);

console.log(
  `[run-tests] Executed ${seen.size} suite files strictly once ` +
  `(standard: ${standardSuites.length}, browser: ${browserSuites.length}) — ` +
  `subtests: ${totalPass}/${totalTests} passed, ${totalFail} failed in ${elapsedSec}s`
);

if (standardResult.code !== 0 || browserResult.code !== 0 || totalFail > 0) {
  process.exit(1);
}
