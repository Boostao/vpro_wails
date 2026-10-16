const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync, existsSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compileModule, compile } = require('svelte/compiler');
const runtime = require('svelte/internal/client');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { review, cell } = require('./siviSourceFixture.cjs');
const wire = require('../../testdata/chars-sivi-entry-client-wire.json');
const speciesWire = require('./siviSpeciesReferenceFixture.json');
const creationReceiptTemplate = require('./siviCreationUndoTestHelpers.cjs').review;
const deletionProof = require('./siviDeletionTestHelpers.cjs');
const creationUndoProof = require('./siviCreationUndoTestHelpers.cjs');
const lifecycleOff = {
  './SIVIDeletionPanel.svelte': { default() {} },
  './SIVIDeletionRestorationPanel.svelte': { default() {} },
  './SIVICreationUndoPanel.svelte': { default() {} },
  './siviDeletionSessions': {
    siviDeletionSession() { throw new Error('Deletion must remain independently gated.'); },
    siviDeletionRestorationSession() { throw new Error('Restoration must remain independently gated.'); },
  },
  './siviCreationUndoSessions': { siviCreationUndoSession() { throw new Error('Creation Undo must remain independently gated.'); } },
};
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const clone = value => structuredClone(value);
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

function fixture(options = {}) {
  const calls = { parentRead: 0, childRead: 0, collectedRead: 0, parentSave: 0, childSave: 0,
    collectedSave: 0, speciesRead: 0, speciesReferences: 0, speciesSave: 0,
    identityRead: 0, identitySave: 0, identityRestore: 0,
    creationReferences: 0, create: 0, creationLookup: 0,
    lifecycle: Object.fromEntries(['deletion', 'restoration', 'creationUndo'].map(kind =>
      [kind, { read: 0, review: 0, save: 0, lookup: 0 }])),
    restore: 0, collectedRestore: 0, speciesRestore: 0, refresh: 0, cancelled: 0 };
  const originalParent = clone(wire);
  let currentParent = clone(wire);
  const originalChild = review({ extended: true });
  for (const group of originalChild) for (const row of group.Rows) {
    row.cells[group.Columns.indexOf('PlotNumber')] = cell('text', wire.Original.Plot);
    if (options.collectedValue) row.cells[group.Columns.indexOf('Collected')] = clone(options.collectedValue);
    if (options.speciesValue) row.cells[group.Columns.indexOf('Species')] = clone(options.speciesValue);
    if (options.identityValue) row.cells[group.Columns.indexOf('ID')] = clone(options.identityValue);
  }
  let currentChild = clone(originalChild);
  const references = { ...clone(speciesWire), contextId: wire.Original.ContextID, project: wire.Original.Project, plot: wire.Original.Plot };
  function creationReceipt(request, lookup = false) {
    const template = clone(creationReceiptTemplate().creation);
    const receipt = { ...template, ...parentOwner(), form: request.form, request: clone(request), covers: clone(request.covers),
      id: 2147483000 + calls.create, rowId: String(9000 + calls.create),
      requestId: request.requestId, historyId: request.requestId, didCommit: !lookup, replayed: lookup };
    receipt.committed.rowId = receipt.rowId;
    for (const [index, column] of receipt.columns.entries()) {
      receipt.committed.cells[index] = column.name === 'ID' ? cell('integer', String(receipt.id))
        : column.name === 'Flag' ? cell('integer', '0')
        : column.name === 'PlotNumber' ? cell('text', request.plot)
        : column.name === 'Species' ? cell('text', request.species)
        : clone(request.covers.find(cover => cover.column === column.name)?.value ?? cell('null'));
    }
    return receipt;
  }
  function parentOwner() { return { contextId: wire.Original.ContextID, project: wire.Original.Project, plot: wire.Original.Plot }; }
  const lifecycleRequests = {}, lifecycleReceipts = {};
  const deletionOriginal = { ...deletionProof.original(), ...parentOwner(), form: 'SubVegA-SIVI' };
  for (const [index, column] of deletionOriginal.columns.entries()) {
    if (column.name === 'PlotNumber') deletionOriginal.original.cells[index] = cell('text', parentOwner().plot);
    if (['Cover6', 'Cover7'].includes(column.name)) deletionOriginal.original.cells[index] = cell('real', 0);
  }
  if (options.deletionCell) {
    deletionOriginal.original.cells[deletionOriginal.columns.findIndex(column => column.name === options.deletionCell.column)] =
      clone(options.deletionCell.value);
  }
  const additionalDeletionRows = [];
  const historicalCreation = creationUndoProof.review();
  Object.assign(historicalCreation, parentOwner());
  Object.assign(historicalCreation.creation, parentOwner());
  Object.assign(historicalCreation.creation.request, parentOwner());
  if (options.historicalCreationForm) {
    historicalCreation.form = historicalCreation.creation.form = historicalCreation.creation.request.form = options.historicalCreationForm;
  }
  historicalCreation.committed.cells[historicalCreation.columns.findIndex(column => column.name === 'PlotNumber')] = cell('text', parentOwner().plot);
  historicalCreation.creation.committed = clone(historicalCreation.committed);
  for (const row of historicalCreation.auditsBefore) {
    row.cells[historicalCreation.auditColumns.findIndex(column => column.name === 'PlotNumber')] = cell('text', parentOwner().plot);
    row.cells[historicalCreation.auditColumns.findIndex(column => column.name === 'Project')] = cell('text', parentOwner().project);
  }
  let deletionRecord = options.seedDeletion ? { ...deletionProof.receipt({ ...clone(deletionOriginal),
    requestId: deletionProof.token }, true), ...parentOwner() } : null;
  let deletionConsumed = false, creationConsumed = false;
  function projectPhysical(columns, row, remove) {
    for (const group of currentChild) {
      group.Rows = group.Rows.filter(value => value.rowId !== row.rowId);
      const membership = group.Form.startsWith('SubVegA-')
        ? ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB']
        : group.Form === 'SubVegC-SIVI' ? ['Cover6'] : ['Cover7', 'Cover8', 'Cover9'];
      if (!remove && membership.some(name => row.cells[columns.findIndex(column => column.name === name)].storage !== 'null')) {
        group.Rows.push({ rowId: row.rowId, cells: group.Columns.map(name => clone(row.cells[columns.findIndex(column => column.name === name)])) });
      }
    }
  }
  if (options.lifecycle) {
    if (!options.seedDeletion) projectPhysical(deletionOriginal.columns, deletionOriginal.original, false);
    projectPhysical(historicalCreation.columns, historicalCreation.committed, false);
  }
  function restorationReview() {
    assert.ok(deletionRecord);
    const auditColumns = historicalCreation.auditColumns;
    return { ...parentOwner(), historyId: deletionRecord.historyId, expected: 'a'.repeat(64),
      form: deletionRecord.form, rowId: deletionRecord.rowId, id: deletionRecord.id,
      columns: clone(deletionRecord.columns), original: clone(deletionRecord.original),
      deletion: { ...clone(deletionRecord), didCommit: false, replayed: true },
      auditColumns: clone(auditColumns), auditsBefore: deletionRecord.audits.map(audit => ({
        rowId: audit.rowId, cells: auditColumns.map(column => column.name === 'ID' ? cell('integer', String(audit.id))
          : ['Restore', 'Flag'].includes(column.name) ? cell('integer', '0')
          : column.name === 'AfterEdit' ? cell('null')
          : cell('text', { Project: audit.project, User: audit.user, PlotNumber: audit.plotNumber, Table: 'sAmPlE_vEg',
            EditField: audit.editField, EditWhen: audit.editWhen, BeforeEdit: audit.beforeEdit }[column.name])),
      })) };
  }
  function restorationResult(request) {
    const reviewed = restorationReview(), after = request.action === 'prune' ? [] : clone(reviewed.auditsBefore);
    for (const row of after) row.cells[reviewed.auditColumns.findIndex(column => column.name === 'Restore')] = cell('integer', '-1');
    return { ...reviewed, requestId: request.requestId, restorationId: request.requestId, action: request.action,
      actor: ' restoring user ', auditStrength: 2, editWhen: '2026-10-07 19:31:54', restored: clone(reviewed.original),
      request: clone(request), auditsAfter: after, cancelled: false, restoredRows: 1,
      prunedAuditRows: request.action === 'prune' ? reviewed.auditsBefore.length : 0, didCommit: true, replayed: false };
  }
  async function lifecycleRead(kind, value) {
    const count = ++calls.lifecycle[kind].read;
    return options.lifecycleRead ? options.lifecycleRead(kind, count, clone(value)) : clone(value);
  }
  async function lifecycleMutation(kind, raw) {
    calls.lifecycle[kind].save++;
    const request = JSON.parse(raw);
    lifecycleRequests[kind] = clone(request);
    let result;
    if (kind === 'deletion') {
      result = { ...deletionProof.receipt(request), ...parentOwner() };
      deletionRecord = clone(result);
      projectPhysical(request.columns, request.original, true);
    } else if (kind === 'restoration') {
      result = restorationResult(request);
      deletionConsumed = true;
      projectPhysical(result.columns, result.original, false);
    } else {
      result = creationUndoProof.result(historicalCreation, request);
      creationConsumed = true;
      projectPhysical(historicalCreation.columns, historicalCreation.committed, true);
    }
    lifecycleReceipts[kind] = clone(result);
    return options.lifecycleMutation ? options.lifecycleMutation(kind, request, result) : result;
  }
  async function lifecycleLookup(kind, raw) {
    calls.lifecycle[kind].lookup++;
    assert.deepEqual(JSON.parse(raw), lifecycleRequests[kind]);
    return { ...clone(lifecycleReceipts[kind]), didCommit: false, replayed: true };
  }
  function requireCommittedRows(edits, committed) {
    const ids = new Set(edits.map(edit => edit.rowId));
    for (let index = 0; index < currentChild.length; index++) {
      const actual = currentChild[index].Rows.filter(row => ids.has(row.rowId));
      const expected = committed[index].Rows.filter(row => ids.has(row.rowId));
      if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error('SIVI physical row changed since the typed edit; no restoration or pruning');
      }
    }
  }
  const cleanups = [];
  const parentProps = { contextId: wire.Original.ContextID, project: wire.Original.Project, plotNumber: wire.Original.Plot };
  const environment = { VITE_SIVI_CHARS_PARENT_ENTRY: 'true', VITE_SIVI_COMBINED_EDITING: 'true', ...options.environment };
  const api = {
    SIVIDeletionService: {
      GetTargets: async () => lifecycleRead('deletion', { ...parentOwner(), columns: deletionOriginal.columns,
        rows: [...(deletionRecord && !deletionConsumed ? [] : [deletionOriginal.original]), ...additionalDeletionRows] }),
      GetOriginal: async (_context, _plot, form, rowId) => {
        calls.lifecycle.deletion.review++;
        const row = rowId === deletionOriginal.original.rowId ? deletionOriginal.original
          : additionalDeletionRows.find(row => row.rowId === rowId);
        assert.ok(row);
        return { ...clone(deletionOriginal), form, original: clone(row) };
      },
      Delete: async (_context, raw) => lifecycleMutation('deletion', raw),
      LookupReceipt: async (_context, raw) => lifecycleLookup('deletion', raw),
      GetHistory: async () => lifecycleRead('restoration', { ...parentOwner(), historyPresent: deletionRecord !== null,
        events: deletionRecord ? [{ historyId: deletionRecord.historyId, form: deletionRecord.form, rowId: deletionRecord.rowId,
          id: deletionRecord.id, species: deletionOriginal.original.cells[deletionOriginal.columns.findIndex(column => column.name === 'Species')].text,
          actor: deletionRecord.actor, editWhen: deletionRecord.editWhen, restored: deletionConsumed, consumed: deletionConsumed }] : [] }),
      ReviewRestoration: async () => { calls.lifecycle.restoration.review++; return restorationReview(); },
      Restore: async (_context, raw) => lifecycleMutation('restoration', raw),
      LookupRestorationReceipt: async (_context, raw) => lifecycleLookup('restoration', raw),
    },
    SIVICreationUndoService: {
      GetHistory: async () => lifecycleRead('creationUndo', { ...parentOwner(), historyPresent: true, events: [{
        historyId: historicalCreation.historyId, form: historicalCreation.form, rowId: historicalCreation.rowId, id: historicalCreation.id,
        species: 'TREE', actor: historicalCreation.creation.actor, editWhen: historicalCreation.creation.editWhen,
        undone: creationConsumed, consumed: creationConsumed, reviewAvailable: !creationConsumed,
        unavailableReason: creationConsumed ? 'This creation history has already been consumed.' : null }] }),
      Review: async () => { calls.lifecycle.creationUndo.review++; return clone(historicalCreation); },
      Undo: async (_context, raw) => lifecycleMutation('creationUndo', raw),
      LookupReceipt: async (_context, raw) => lifecycleLookup('creationUndo', raw),
    },
    SIVICreationService: {
      GetReferences: async (context, plot) => {
        calls.creationReferences++;
        assert.equal(context, wire.Original.ContextID);
        assert.equal(plot, wire.Original.Plot);
        return options.creationReferences ? options.creationReferences(calls.creationReferences, clone(references)) : clone(references);
      },
      Create: async (context, raw) => {
        calls.create++;
        assert.equal(context, wire.Original.ContextID);
        calls.creationRequest = JSON.parse(raw);
        const result = creationReceipt(calls.creationRequest);
        additionalDeletionRows.push({ rowId: result.rowId, cells: deletionOriginal.columns.map(column =>
          clone(result.committed.cells[result.columns.findIndex(value => value.name === column.name)])) });
        for (const group of currentChild) if (group.Form === result.form) {
          group.Rows.push({ rowId: result.rowId, cells: group.Columns.map(name =>
            clone(result.committed.cells[result.columns.findIndex(column => column.name === name)])) });
        }
        return options.create ? options.create(calls.creationRequest, result) : result;
      },
      LookupReceipt: async (context, raw) => {
        calls.creationLookup++;
        assert.equal(context, wire.Original.ContextID);
        assert.deepEqual(JSON.parse(raw), calls.creationRequest);
        return options.creationLookup ? options.creationLookup(calls.creationRequest) : creationReceipt(calls.creationRequest, true);
      },
    },
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
    SIVICHARSParentEntryService: {
      Review: async () => {
        calls.parentRead++;
        return options.parentRead ? options.parentRead(calls.parentRead, clone(currentParent)) : clone(currentParent);
      },
      Save: async (_context, _plot, request) => {
        calls.parentSave++;
        for (const edit of request.edits) {
          const binding = currentParent.Original.Bindings.find(row => row.Binding === edit.column);
          const target = currentParent.Original.Rows[0][binding.Table.endsWith('_Env') ? 'Env' : 'Admin'];
          target.cells[binding.Column] = clone(edit.value);
        }
        return { ChangedCells: request.edits.length + (request.actions?.length ?? 0), HistoryID: '101' };
      },
      Restore: async () => { currentParent = clone(originalParent); return { restoredRows: 1, prunedAuditRows: 1,
        cleanedVegRows: 0, cancelled: false }; },
    },
    SIVICombinedService: {
      GetOriginal: async (context, plot, extended) => {
        calls.childRead++;
        assert.equal(context, wire.Original.ContextID);
        assert.equal(plot, wire.Original.Plot);
        assert.equal(extended, true);
        if (options.read) return options.read(calls.childRead, clone(currentChild));
        return clone(currentChild);
      },
      SaveReviewed: async (_context, _plot, extended, raw) => {
        calls.childSave++;
        assert.equal(extended, true);
        const request = JSON.parse(raw);
        calls.request = request;
        for (const edit of request.edits) {
          for (const group of currentChild) for (const row of group.Rows) {
            const index = group.Columns.indexOf(edit.column);
            if (row.rowId === edit.rowId && index >= 0) row.cells[index] = clone(edit.value);
          }
        }
        calls.committedChild = clone(currentChild);
        if (options.save) return options.save(request);
        return { ChangedCells: request.edits.length, HistoryID: '201' };
      },
      RestoreReviewed: async (_context, _plot, history, action) => {
        calls.restore++;
        assert.equal(history, '201');
        requireCommittedRows(calls.request.edits, calls.committedChild);
        for (const edit of calls.request.edits) for (const group of currentChild) for (const row of group.Rows) {
          const index = group.Columns.indexOf(edit.column);
          if (row.rowId === edit.rowId && index >= 0) row.cells[index] = clone(edit.expected);
        }
        return { restoredRows: calls.request.edits.length, prunedAuditRows: action === 'prune' ? calls.request.edits.length : 0,
          cleanedVegRows: 0, cancelled: false };
      },
    },
    SIVICollectedService: {
      GetOriginal: async (context, plot, extended) => {
        calls.collectedRead++;
        assert.equal(context, wire.Original.ContextID);
        assert.equal(plot, wire.Original.Plot);
        assert.equal(extended, true);
        return options.collectedRead ? options.collectedRead(calls.collectedRead, clone(currentChild)) : clone(currentChild);
      },
      SaveReviewed: async (_context, _plot, extended, raw) => {
        calls.collectedSave++;
        assert.equal(extended, true);
        calls.collectedRequest = JSON.parse(raw);
        for (const edit of calls.collectedRequest.edits) {
          const before = edit.expected.storage === 'null' ? null : edit.expected.text;
          const outcomes = new Map([[null, ['C', 'V', null]], ['C', ['V', null, 'C']], ['V', [null, 'C', 'V']]]);
          const value = outcomes.has(before) ? outcomes.get(before)[edit.clicks - 1] : before;
          for (const group of currentChild) for (const row of group.Rows) {
            if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('Collected')] = cell(value === null ? 'null' : 'text', value);
          }
        }
        calls.committedCollected = clone(currentChild);
        return options.collectedSave ? options.collectedSave(calls.collectedRequest)
          : { ChangedCells: calls.collectedRequest.edits.length, HistoryID: '301' };
      },
      RestoreReviewed: async (_context, _plot, history, action) => {
        calls.collectedRestore++;
        assert.equal(history, '301');
        requireCommittedRows(calls.collectedRequest.edits, calls.committedCollected);
        for (const edit of calls.collectedRequest.edits) for (const group of currentChild) for (const row of group.Rows) {
          if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('Collected')] = clone(edit.expected);
        }
        return { restoredRows: calls.collectedRequest.edits.length,
          prunedAuditRows: action === 'prune' ? calls.collectedRequest.edits.length : 0, cleanedVegRows: 0, cancelled: false };
      },
    },
  };
  api.SIVISpeciesService = {
    GetOriginal: async (context, plot, extended) => {
      calls.speciesRead++;
      assert.equal(context, wire.Original.ContextID);
      assert.equal(plot, wire.Original.Plot);
      assert.equal(extended, true);
      return options.speciesRead ? options.speciesRead(calls.speciesRead, clone(currentChild)) : clone(currentChild);
    },
    GetReferences: async (context, plot) => {
      calls.speciesReferences++;
      assert.equal(context, wire.Original.ContextID);
      assert.equal(plot, wire.Original.Plot);
      return options.speciesReferences ? options.speciesReferences(calls.speciesReferences, clone(references)) : clone(references);
    },
    SaveReviewed: async (_context, _plot, extended, raw) => {
      calls.speciesSave++;
      assert.equal(extended, true);
      calls.speciesRequest = JSON.parse(raw);
      assert.deepEqual(calls.speciesRequest.references, references);
      for (const edit of calls.speciesRequest.edits) for (const group of currentChild) for (const row of group.Rows) {
        if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('Species')] = cell('text', edit.value);
      }
      calls.committedSpecies = clone(currentChild);
      return options.speciesSave ? options.speciesSave(calls.speciesRequest)
        : { ChangedCells: calls.speciesRequest.edits.length, HistoryID: '401' };
    },
    RestoreReviewed: async (_context, _plot, history, action) => {
      calls.speciesRestore++;
      assert.equal(history, '401');
      requireCommittedRows(calls.speciesRequest.edits, calls.committedSpecies);
      for (const edit of calls.speciesRequest.edits) for (const group of currentChild) for (const row of group.Rows) {
        if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('Species')] = clone(edit.expected);
      }
      return { restoredRows: calls.speciesRequest.edits.length,
        prunedAuditRows: action === 'prune' ? calls.speciesRequest.edits.length : 0, cleanedVegRows: 0, cancelled: false };
    },
  };
  api.SIVIIdentityService = {
    GetOriginal: async (context, plot, extended) => {
      calls.identityRead++;
      assert.equal(context, wire.Original.ContextID);
      assert.equal(plot, wire.Original.Plot);
      assert.equal(extended, true);
      return options.identityRead ? options.identityRead(calls.identityRead, clone(currentChild)) : clone(currentChild);
    },
    SaveReviewed: async (_context, _plot, extended, raw) => {
      calls.identitySave++;
      assert.equal(extended, true);
      calls.identityRequest = JSON.parse(raw);
      for (const edit of calls.identityRequest.edits) for (const group of currentChild) for (const row of group.Rows) {
        assert.equal(edit.column, 'ID');
        if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('ID')] = clone(edit.value);
      }
      calls.committedIdentity = clone(currentChild);
      return options.identitySave ? options.identitySave(calls.identityRequest)
        : { ChangedCells: calls.identityRequest.edits.length, HistoryID: '501' };
    },
    RestoreReviewed: async (_context, _plot, history, action) => {
      calls.identityRestore++;
      assert.equal(history, '501');
      assert.ok(['retain', 'prune'].includes(action));
      requireCommittedRows(calls.identityRequest.edits, calls.committedIdentity);
      for (const edit of calls.identityRequest.edits) for (const group of currentChild) for (const row of group.Rows) {
        if (row.rowId === edit.rowId) row.cells[group.Columns.indexOf('ID')] = clone(edit.expected);
      }
      return options.identityRestore ? options.identityRestore()
        : { restoredRows: calls.identityRequest.edits.length, prunedAuditRows: 0, cleanedVegRows: 0, cancelled: false };
    },
  };
  const cache = new Map();
  function load(filename) {
    if (cache.has(filename)) return cache.get(filename);
    const module = { exports: {} };
    cache.set(filename, module.exports);
    const code = ts.transpileModule(readFileSync(filename, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
    }).outputText;
    new Function('module', 'exports', 'require', code)(module, module.exports, request => dependency(request, filename));
    cache.set(filename, module.exports);
    return module.exports;
  }
  function dependency(request, filename) {
    if (request === '../bindings/github.com/boostao/vpro-wails') return api;
    if (request === './readRequests') return { ReadRequests: class {
      track(promise) { return promise; }
      cancelAll() { calls.cancelled++; options.cancel?.(); }
    } };
    if (request === 'svelte') return { untrack: runtime.untrack, onMount() {}, onDestroy: callback => cleanups.push(callback) };
    if (request === 'svelte/internal/client') return runtime;
    if (request === '@wailsio/runtime') return {};
    if (request.endsWith('.svelte')) return { default() {} };
    const target = path.resolve(path.dirname(filename), request);
    if (target.endsWith('.json')) return JSON.parse(readFileSync(target, 'utf8'));
    if (existsSync(target + '.ts')) return load(target + '.ts');
    throw new Error('Unexpected client test dependency: ' + request);
  }
  function component(name, props, extra) {
    const filename = path.join(__dirname, name);
    let script = source(name).match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
    const ast = ts.createSourceFile(name + '.ts', script, ts.ScriptTarget.Latest, true);
    const declaration = ast.statements.find(node => ts.isVariableStatement(node) && node.getText(ast).includes('$props()'));
    assert.ok(declaration);
    const properties = name === 'SIVICHARSForm.svelte' ? 'contextId, project, plotNumber, onSaved, onBusyChange'
      : `contextId, project, plotNumber, onSaved, onBusyChange, linkedVegetation,
        collectedAvailable = false, speciesAvailable = false, identityAvailable = false, creationAvailable = false,
        deletionAvailable = false, restorationAvailable = false, creationUndoAvailable = false`;
    script = script.slice(0, declaration.getStart(ast)) + `const { ${properties} } = __props;` +
      script.slice(declaration.end);
    if (name === 'SIVICHARSParentForm.svelte') {
      for (const prop of ['peerBlockedReason', 'peerUnlockAvailable']) script = script.replaceAll(prop, '__props.' + prop);
    }
    const javascript = ts.transpileModule(script.replaceAll('import.meta.env', '__environment'), {
      compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
    }).outputText;
    const compiled = compileModule(`import { props as __props, environment as __environment } from 'test-owner';
      ${javascript}\n${extra}`, { filename: name + '.svelte.js', generate: 'client' });
    assert.equal(compiled.warnings.length, 0);
    const module = { exports: {} };
    const code = ts.transpileModule(compiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText;
    new Function('module', 'exports', 'require', code)(module, module.exports, request =>
      request === 'test-owner' ? { props, environment } : dependency(request, filename));
    return module.exports;
  }
  let parent, form, disposeEffects;
  Object.defineProperty(parentProps, 'peerBlockedReason', { get: () => form.peerReason() });
  Object.defineProperty(parentProps, 'peerUnlockAvailable', { get: () => form.creationCanUnlock() });
  function mount() { disposeEffects = runtime.effect_root(() => {
    form = component('SIVICHARSForm.svelte', {
      ...parentProps, onSaved: async () => { calls.refresh++; await options.refresh?.(); },
    }, `export function bindParent(value) { parent = value; }
      export function perform(kind) { return operate(kind); }
      export function draft(row, column, value, nil = false) { stage(row, column, value, nil); }
      export function snapshot() { return childView; }
      export function peerReason() { return peerBlockedReason; }
      export function parentSaved() { return refreshAfterParentSave(); }
      export function cancelSource() { cancelRead(); }
      export function collect(kind) { return operate(kind, 'collected'); }
      export function cycle(row) { cycleCollected(row); }
      export function collectedSnapshot() { return collectedView; }
      export function cancelCollected() { cancelRead('collected'); }
      export function select(kind) { return operate(kind, 'species'); }
      export function speciesDraft(row, raw) { stageSpecies(row, raw); }
      export function speciesContext(row, group) { contextSpecies(row, group); }
      export function speciesChoose(row, kind, selected) { chooseSpecies(row, kind, selected); }
      export function speciesSnapshot() { return speciesView; }
      export function cancelSpecies() { cancelRead('species'); }
      export function identify(kind) { return operate(kind, 'identity'); }
      export function identityDraft(row, raw, nil = false) { stageIdentity(row, raw, nil); }
      export function identitySnapshot() { return identityView; }
      export function cancelIdentity() { cancelRead('identity'); }
      export function createRow(kind, source) { return creationOperation(kind, source); }
      export function creationDraft(value) { stageCreation(value); }
      export function creationSnapshot() { return creationView; }
      export function creationCanUnlock() { return (creationPending || lifecyclePending) && !creationClose?.busy &&
        !lifecycleKinds.some(kind => lifecycleStates[kind]?.busy); }
      export function retainedCreation() { return creation; }
      export function creationNeedsRefresh() { return creationRefreshNeeded; }
      export function lifecycle(kind, operation, identity, source) { return lifecycleOperation(kind, operation, identity, source); }
      export function lifecycleConfirm(kind, confirmed, action) { stageLifecycle(kind, confirmed, action); }
      export function lifecycleSnapshot(kind) { return lifecycleEntry(kind)?.view(); }
      export function retainedLifecycle(kind) { return lifecycleEntry(kind); }
      export function lifecycleTargets() { return deletionTargets; }
      export function lifecycleNeedsRefresh(kind) { return lifecycleRefreshNeeded(kind); }`);
    parentProps.onSaved = form.parentSaved;
    parent = component('SIVICHARSParentForm.svelte', parentProps, `
      export function loadSource() { return operation('load'); }
      export function unlock() { return locked ? toggleParentLock() : true; }
      export function clickLock() { return toggleParentLock(); }
      export function lockEligibility() {
        return { locked, disabled: lockToggleDisabled, hasOriginal: !!view?.original, fieldsDisabled: disabled };
      }
      export function draft(column, input) { stage(column, input); }
      export function snapshot() { return view; }
      export function undoSource() { return operation('undo'); }
      export function restoreSource() { return operation('prune'); }`);
    form.bindParent(parent);
  }); }
  mount();
  return { calls, get parent() { return parent; }, get form() { return form; }, rowId: originalChild[0].Rows[0].rowId,
    lifecycleOriginal: clone(deletionOriginal), lifecycleCreation: clone(historicalCreation),
    creationEditor: () => load(path.join(__dirname, 'siviCreationEditor.ts')),
    creationFields(form) {
      const fields = serverComponent(source('SIVICreationFields.svelte'), 'SIVICreationFields.svelte',
        Object.fromEntries(['siviCoverEditor', 'siviSpeciesEditor', 'vegetationSpeciesEditor', 'siviCreationEditor']
          .map(name => ['./' + name, load(path.join(__dirname, name + '.ts'))])));
      const draft = load(path.join(__dirname, 'siviCreationEditor.ts')).beginSIVICreation(form, parentOwner());
      return render(fields, { props: { draft, owner: parentOwner(), references: clone(references), onchange() {} } }).body;
    },
    remount() { for (const cleanup of cleanups.splice(0).reverse()) cleanup(); disposeEffects(); mount(); },
    dispose() { for (const cleanup of cleanups.reverse()) cleanup(); disposeEffects(); } };
}

test('compiled linked snippet requires exact parent and independent combined/Collected/Species/identity literal flags', () => {
  for (const parent of [undefined, 'false', 'TRUE', 'true']) for (const combined of [undefined, 'false', 'TRUE', 'true'])
  for (const collected of [undefined, 'false', 'TRUE', 'true']) for (const species of [undefined, 'false', 'TRUE', 'true'])
  for (const identity of [undefined, 'false', 'TRUE', 'true']) {
    let rendered = 0;
    const parentStub = serverComponent(`<script lang="ts">
      import { record } from 'proof'; let { linkedVegetation } = $props();
      </script><section>{record(Boolean(linkedVegetation))}{#if linkedVegetation}{@render linkedVegetation()}{/if}</section>`, 'linked-parent.svelte',
      { proof: { record: value => { if (value) rendered++; return ''; } } });
    const component = serverComponent(source('SIVICHARSForm.svelte'), 'SIVICHARSForm.svelte', {
      ...lifecycleOff,
      './SIVICHARSParentForm.svelte': { default: parentStub }, './SIVICombinedPanel.svelte': { default() {} },
      './SIVICollectedPanel.svelte': { default() {} },
      './SIVIIdentityPanel.svelte': { default() {} },
      './SIVISpeciesPanel.svelte': { default() {} },
      './SIVICreationPanel.svelte': { default() {} },
      './siviCreationSessions': { siviCreationSession() { throw new Error('Creation must remain independently gated.'); } },
      '../bindings/github.com/boostao/vpro-wails': { SIVICombinedService: {}, SIVICollectedService: {}, SIVISpeciesService: {}, SIVIIdentityService: {}, AuditRestoreAction: {} },
      './siviCombinedSession': { SIVICombinedSession: class {
        view() { return { busy: false }; }
        closeState() { return { busy: false, unsaved: false, blocked: false }; }
        dispose() {}
      } },
      './siviCollectedSession': { SIVICollectedSession: class {
        view() { return { busy: false }; }
        closeState() { return { busy: false, unsaved: false, blocked: false }; }
        dispose() {}
      } },
      './siviSpeciesSession': { SIVISpeciesSession: class {
        view() { return { busy: false }; }
        closeState() { return { busy: false, unsaved: false, blocked: false }; }
        dispose() {}
      } },
      './siviIdentitySession': { SIVIIdentitySession: class {
        view() { return { busy: false }; }
        closeState() { return { busy: false, unsaved: false, blocked: false }; }
        dispose() {}
      } },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: parent, VITE_SIVI_COMBINED_EDITING: combined,
      VITE_SIVI_COLLECTED_EDITING: collected, VITE_SIVI_SPECIES_EDITING: species, VITE_SIVI_IDENTITY_EDITING: identity });
    const html = render(component, { props: { contextId: 'owned', project: 'Sample', plotNumber: 'P' } }).body;
    const combinedEnabled = parent === 'true' && combined === 'true';
    const collectedEnabled = parent === 'true' && collected === 'true';
    const speciesEnabled = parent === 'true' && species === 'true';
    const identityEnabled = parent === 'true' && identity === 'true';
    const enabled = combinedEnabled || collectedEnabled || speciesEnabled || identityEnabled;
    assert.equal(rendered, enabled ? 1 : 0);
    assert.equal(html.includes('data-chars-linked-vegetation'), enabled);
    assert.equal((html.match(/data-chars-linked-open/g) ?? []).length, combinedEnabled ? 1 : 0);
    assert.equal((html.match(/data-chars-collected-open/g) ?? []).length, collectedEnabled ? 1 : 0);
    assert.equal((html.match(/data-chars-species-open/g) ?? []).length, speciesEnabled ? 1 : 0);
    assert.equal((html.match(/data-chars-identity-open/g) ?? []).length, identityEnabled ? 1 : 0);
    assert.doesNotMatch(html, /data-sivi-combined-column/);
  }
  assert.equal(compile(source('SIVICHARSForm.svelte'), { filename: 'SIVICHARSForm.svelte', generate: 'client' }).warnings.length, 0);
  const parent = source('SIVICHARSParentForm.svelte');
  assert.match(parent, /group\.name === 'Vegetation'\}\{@render linkedVegetation\?\.\(\)\}/);
  assert.ok(parent.indexOf("name: 'Vegetation'") < parent.indexOf("name: 'Notes'"));
  const app = source('App.svelte');
  assert.match(app, /const siviCHARSLinkedVegetationEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_COMBINED_EDITING === 'true'/);
  assert.match(app, /charsLinkedVegetation=\{siviCHARSLinkedVegetationEnabled\}/);
  assert.match(app, /const siviCHARSCollectedEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_COLLECTED_EDITING === 'true'/);
  assert.match(app, /charsCollected=\{siviCHARSCollectedEnabled\}/);
  assert.match(app, /const siviCHARSSpeciesEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_SPECIES_EDITING === 'true'/);
  assert.match(app, /charsSpecies=\{siviCHARSSpeciesEnabled\}/);
  assert.match(app, /const siviCHARSIdentityEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_IDENTITY_EDITING === 'true'/);
  assert.match(app, /charsIdentity=\{siviCHARSIdentityEnabled\}/);
});

test('compiled creator gate is independent of all four child gates and exposes only actual CHARS source forms', () => {
  for (const parent of [undefined, 'false', 'TRUE', 'true']) for (const creation of [undefined, 'false', 'TRUE', 'true'])
  for (const peers of ['false', 'true']) {
    let panels = 0;
    const parentStub = serverComponent('<script>let { linkedVegetation } = $props();</script>{@render linkedVegetation?.()}',
      'parent.svelte', {});
    const panelStub = serverComponent(`<script>import { record } from 'proof';
      let { allowedForms } = $props();</script>{record(allowedForms)}<div data-creator-proof></div>`, 'creator.svelte',
      { proof: { record: forms => {
        panels++;
        assert.deepEqual(Array.from(forms), ['SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI']);
        return '';
      } } });
    class Child {
      view() { return { busy: false }; }
      closeState() { return { busy: false, unsaved: false, blocked: false }; }
      dispose() {}
    }
    const component = serverComponent(source('SIVICHARSForm.svelte'), 'SIVICHARSForm.svelte', {
      ...lifecycleOff,
      '../bindings/github.com/boostao/vpro-wails': { SIVICombinedService: {}, SIVICollectedService: {},
        SIVISpeciesService: {}, SIVIIdentityService: {}, AuditRestoreAction: {} },
      './SIVICHARSParentForm.svelte': { default: parentStub },
      ...Object.fromEntries(['Combined', 'Collected', 'Species', 'Identity'].map(name => ['./SIVI' + name + 'Panel.svelte', { default() {} }])),
      ...Object.fromEntries(['Combined', 'Collected', 'Species', 'Identity']
        .map(name => ['./sivi' + name + 'Session', { ['SIVI' + name + 'Session']: Child }])),
      './SIVICreationPanel.svelte': { default: panelStub },
      './siviCreationSessions': { siviCreationSession: () => ({
        view: () => ({ receipt: null, draft: null, revision: 0 }),
        closeState: () => ({ unsaved: false, busy: false, blocked: false, canSave: false }),
      }) },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: parent, VITE_SIVI_CREATION_WRITING: creation,
      VITE_SIVI_COMBINED_EDITING: peers, VITE_SIVI_COLLECTED_EDITING: peers,
      VITE_SIVI_SPECIES_EDITING: peers, VITE_SIVI_IDENTITY_EDITING: peers });
    const html = render(component, { props: { contextId: 'owned', project: 'Sample', plotNumber: 'P' } }).body;
    const enabled = parent === 'true' && creation === 'true';
    assert.equal(panels, enabled ? 1 : 0);
    assert.equal(html.includes('data-creator-proof'), enabled);
  }
  assert.match(source('App.svelte'), /const siviCHARSCreationEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_CREATION_WRITING === 'true'/);
  assert.match(source('App.svelte'), /charsCreation=\{siviCHARSCreationEnabled\}/);
});

test('compiled lifecycle gates remain literal and independent of creator and typed child flags', () => {
  const flags = [undefined, 'false', 'TRUE', 'true'];
  for (const parent of flags) for (const deletion of flags) for (const restoration of flags) for (const undo of flags) {
    const counts = { deletion: 0, restoration: 0, creationUndo: 0 };
    class Child {
      view() { return { busy: false }; }
      closeState() { return { busy: false, unsaved: false, blocked: false }; }
      dispose() {}
    }
    const parentStub = serverComponent('<script>let { linkedVegetation } = $props();</script>{@render linkedVegetation?.()}',
      'parent.svelte', {});
    function panel(kind) {
      return { default: serverComponent(`<script>import { record } from 'proof';</script>{record()}`,
        kind + '.svelte', { proof: { record() { counts[kind]++; return ''; } } }) };
    }
    const session = () => ({
      view: () => ({ receipt: null, targets: null, original: null, history: null, review: null, revision: 0 }),
      closeState: () => ({ unsaved: false, busy: false, blocked: false, canSave: false }),
    });
    const component = serverComponent(source('SIVICHARSForm.svelte'), 'SIVICHARSForm.svelte', {
      ...lifecycleOff,
      './SIVICHARSParentForm.svelte': { default: parentStub },
      '../bindings/github.com/boostao/vpro-wails': { SIVICombinedService: {}, SIVICollectedService: {},
        SIVISpeciesService: {}, SIVIIdentityService: {}, AuditRestoreAction: {} },
      ...Object.fromEntries(['Combined', 'Collected', 'Species', 'Identity'].map(name => ['./SIVI' + name + 'Panel.svelte', { default() {} }])),
      ...Object.fromEntries(['Combined', 'Collected', 'Species', 'Identity'].map(name =>
        ['./sivi' + name + 'Session', { ['SIVI' + name + 'Session']: Child }])),
      './SIVICreationPanel.svelte': { default() {} },
      './siviCreationSessions': { siviCreationSession() { throw new Error('Creator is not a lifecycle prerequisite.'); } },
      './SIVIDeletionPanel.svelte': panel('deletion'), './SIVIDeletionRestorationPanel.svelte': panel('restoration'),
      './SIVICreationUndoPanel.svelte': panel('creationUndo'),
      './siviDeletionSessions': { siviDeletionSession: session, siviDeletionRestorationSession: session },
      './siviCreationUndoSessions': { siviCreationUndoSession: session },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: parent, VITE_SIVI_DELETION_WRITING: deletion,
      VITE_SIVI_DELETION_RESTORING: restoration, VITE_SIVI_CREATION_UNDO: undo });
    render(component, { props: { contextId: 'owned', project: 'Sample', plotNumber: 'P' } }).body;
    for (const [index, kind] of ['deletion', 'restoration', 'creationUndo'].entries()) {
      assert.equal(counts[kind], parent === 'true' && [deletion, restoration, undo][index] === 'true' ? 1 : 0);
    }
  }
  for (const name of ['Deletion', 'Restoration', 'CreationUndo']) {
    assert.match(source('App.svelte'), new RegExp(`chars${name}=\\{siviCHARS${name}Enabled\\}`));
  }
});

async function lifecycleFixture(options = {}) {
  const f = fixture({ ...options, lifecycle: true, environment: {
    VITE_SIVI_CREATION_WRITING: 'true', VITE_SIVI_DELETION_WRITING: 'true',
    VITE_SIVI_DELETION_RESTORING: 'true', VITE_SIVI_CREATION_UNDO: 'true', ...options.environment,
  } });
  assert.equal(await f.parent.loadSource(), true);
  f.parent.unlock();
  return f;
}
async function lifecycleReview(f, kind, source = 'SubVegA-SIVI') {
  const load = kind === 'deletion' ? 'load' : 'history';
  assert.equal(await f.form.lifecycle(kind, load), true, f.form.getCloseState().error);
  const id = kind === 'deletion' ? f.lifecycleOriginal.original.rowId
    : f.form.lifecycleSnapshot(kind).history.events[0].historyId;
  assert.equal(await f.form.lifecycle(kind, 'review', id, source), true, f.form.getCloseState().error);
  return id;
}
function lifecycleConfirm(f, kind, action = 'prune') {
  if (kind !== 'deletion') f.form.lifecycleConfirm(kind, false, action);
  f.form.lifecycleConfirm(kind, true);
}

test('three CHARS deletion links share one complete physical original and ordinary Save never deletes it', async () => {
  const f = await lifecycleFixture();
  try {
    assert.equal(await f.form.lifecycle('deletion', 'load'), true);
    assert.deepEqual(f.form.lifecycleTargets().map(value => [value.form, value.rowId]),
      ['SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI'].map(form => [form, f.lifecycleOriginal.original.rowId]));
    assert.equal(await f.form.lifecycle('deletion', 'review', f.lifecycleOriginal.original.rowId, 'SubVegA-SIVI_BC'), false);
    assert.equal(f.calls.lifecycle.deletion.review, 0);
    await lifecycleReview(f, 'deletion');
    assert.deepEqual(f.form.lifecycleSnapshot('deletion').original.original, f.lifecycleOriginal.original);
    assert.equal(f.form.lifecycleSnapshot('deletion').original.columns.length, 44);
    lifecycleConfirm(f, 'deletion');
    assert.equal(f.form.getCloseState().canSave, false);
    assert.match(f.form.getCloseState().saveReason, /explicitly|Save-and-close/);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.parent.saveForClose(), false);
    assert.equal(f.calls.lifecycle.deletion.save, 0);
    assert.equal(await f.form.createRow('load'), false);
    assert.equal(await f.form.lifecycle('restoration', 'history'), false);
    for (const read of [() => f.form.perform('load'), () => f.form.collect('load'),
      () => f.form.select('load'), () => f.form.identify('load')]) assert.equal(await read(), false);
    f.remount();
    assert.equal(f.form.lifecycleSnapshot('deletion').confirmed, true);
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.lifecycle('deletion', 'undo'), true);
    assert.equal(f.form.getCloseState().unsaved, false);
  } finally { f.dispose(); }
});

test('a retained normal-A deletion review is neither rebound nor submitted, but owning discard survives remount', async () => {
  const f = await lifecycleFixture();
  try {
    assert.equal(await f.form.lifecycle('deletion', 'load'), true);
    const retained = f.form.retainedLifecycle('deletion');
    assert.equal(await retained.review('SubVegA-SIVI_BC', f.lifecycleOriginal.original.rowId), true);
    retained.confirm(true);
    f.remount();
    assert.equal(f.form.retainedLifecycle('deletion'), retained);
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(f.form.lifecycleSnapshot('deletion').original.form, 'SubVegA-SIVI_BC');
    assert.equal(await f.form.lifecycle('deletion', 'save'), false);
    assert.equal(f.calls.lifecycle.deletion.save, 0);
    assert.equal(await f.form.lifecycle('deletion', 'undo'), true);
  } finally { f.dispose(); }
});

test('Delete refreshes every loaded physical-row peer and history, then typed restoration preserves all44 cells', async () => {
  const f = await lifecycleFixture({ environment: {
    VITE_SIVI_COLLECTED_EDITING: 'true', VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_IDENTITY_EDITING: 'true' } });
  try {
    for (const load of [() => f.form.perform('load'), () => f.form.collect('load'),
      () => f.form.select('load'), () => f.form.identify('load')]) assert.equal(await load(), true);
    assert.equal(await f.form.lifecycle('restoration', 'history'), true);
    assert.equal(f.form.lifecycleSnapshot('restoration').history.historyPresent, false);
    assert.equal(await f.form.lifecycle('creationUndo', 'history'), true);
    await lifecycleReview(f, 'deletion', 'SubVegC-SIVI');
    lifecycleConfirm(f, 'deletion');
    assert.equal(await f.form.lifecycle('deletion', 'save'), true, f.form.getCloseState().error);
    assert.equal(f.calls.lifecycle.deletion.save, 1);
    assert.equal(f.form.lifecycleNeedsRefresh('deletion'), false);
    for (const view of [f.form.snapshot(), f.form.collectedSnapshot(), f.form.speciesSnapshot(), f.form.identitySnapshot()]) {
      assert.equal(view.review.some(group => group.Rows.some(row => row.rowId === f.lifecycleOriginal.original.rowId)), false);
    }
    assert.equal(f.form.lifecycleSnapshot('deletion').targets.length, 0);
    assert.equal(f.form.lifecycleSnapshot('restoration').history.events.length, 1);
    assert.equal(f.calls.lifecycle.creationUndo.read, 2);
    await lifecycleReview(f, 'restoration');
    lifecycleConfirm(f, 'restoration', 'retain');
    f.form.lifecycleConfirm('restoration', false, 'prune');
    assert.equal(f.form.lifecycleSnapshot('restoration').confirmed, false);
    assert.equal(await f.form.lifecycle('restoration', 'save'), false);
    lifecycleConfirm(f, 'restoration');
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.lifecycle('restoration', 'save'), true, f.form.getCloseState().error);
    assert.deepEqual(f.form.lifecycleSnapshot('restoration').receipt.restored, f.lifecycleOriginal.original);
    assert.equal(f.calls.lifecycle.restoration.save, 1);
    assert.equal(f.form.lifecycleSnapshot('restoration').history.events[0].consumed, true);
    assert.equal(f.form.lifecycleSnapshot('deletion').targets.length, 4);
    assert.equal(f.form.lifecycleTargets().length, 3);
    for (const view of [f.form.snapshot(), f.form.collectedSnapshot(), f.form.speciesSnapshot(), f.form.identitySnapshot()]) {
      for (const group of view.review) assert.ok(group.Rows.some(row => row.rowId === f.lifecycleOriginal.original.rowId));
    }
  } finally { f.dispose(); }
});

test('historical creation Undo retains explicit normal-A provenance instead of inferring a new CHARS creation form', async () => {
  const f = await lifecycleFixture({ historicalCreationForm: 'SubVegA-SIVI_BC' });
  try {
    await lifecycleReview(f, 'creationUndo');
    assert.equal(f.form.lifecycleSnapshot('creationUndo').review.form, 'SubVegA-SIVI_BC');
    lifecycleConfirm(f, 'creationUndo', 'retain');
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.lifecycle.creationUndo.save, 0);
    assert.equal(await f.form.lifecycle('creationUndo', 'undo'), true, f.form.getCloseState().error);
    const receipt = f.form.lifecycleSnapshot('creationUndo').receipt;
    assert.equal(receipt.form, 'SubVegA-SIVI_BC');
    assert.equal(receipt.committed.cells.length, 44);
    assert.equal(receipt.auditsAfter.length, 2);
    assert.equal(f.form.lifecycleSnapshot('creationUndo').history.events[0].consumed, true);
    assert.equal(f.calls.lifecycle.creationUndo.save, 1);
  } finally { f.dispose(); }
});

test('cancelled lifecycle acknowledgements resolve the same retained requests after remount without repeating mutations', async () => {
  for (const kind of ['deletion', 'restoration', 'creationUndo']) {
    const held = deferred();
    const f = await lifecycleFixture({ seedDeletion: kind === 'restoration',
      lifecycleMutation: (_kind, _request, value) => { held.result = value; return held.promise; } });
    try {
      await lifecycleReview(f, kind);
      lifecycleConfirm(f, kind);
      const pending = f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save');
      assert.equal(f.form.getCloseState().busy, true);
      assert.equal(await f.form.lifecycle(kind, 'cancel'), true);
      const retained = f.form.lifecycleSnapshot(kind).requestId;
      assert.equal(f.form.lifecycleSnapshot(kind).blocked, true);
      f.remount();
      assert.equal(await f.parent.loadSource(), true);
      f.parent.unlock();
      assert.equal(f.form.lifecycleSnapshot(kind).requestId, retained);
      assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'discard' : 'undo'), false);
      assert.equal(await f.form.lifecycle(kind, 'resolve'), true, f.form.getCloseState().error);
      assert.equal(f.calls.lifecycle[kind].save, 1);
      assert.equal(f.calls.lifecycle[kind].lookup, 1);
      assert.equal(f.form.lifecycleSnapshot(kind).receipt.replayed, true);
      held.resolve(held.result);
      assert.equal(await pending, false);
      assert.equal(f.form.lifecycleSnapshot(kind).receipt.replayed, true);
    } finally { f.dispose(); }
  }
});

test('verified lifecycle receipt and missing-parent recovery survive remount without mutation replay', async () => {
  for (const kind of ['deletion', 'restoration', 'creationUndo']) {
    let fail = true;
    const f = await lifecycleFixture({ seedDeletion: kind === 'restoration',
      parentRead: (count, value) => { if (count > 1 && fail) throw new Error('missing parent projection'); return value; } });
    try {
      await lifecycleReview(f, kind);
      lifecycleConfirm(f, kind);
      assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save'), false);
      const receipt = f.form.lifecycleSnapshot(kind).receipt;
      assert.ok(receipt);
      assert.equal(f.form.lifecycleNeedsRefresh(kind), true);
      assert.equal(f.parent.snapshot().original, null);
      assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save'), false);
      fail = false;
      f.remount();
      assert.equal(f.form.lifecycleNeedsRefresh(kind), true);
      assert.equal(await f.parent.loadSource(), true);
      f.parent.unlock();
      assert.equal(await f.form.lifecycle(kind, 'refresh'), true, f.form.getCloseState().error);
      assert.deepEqual(f.form.lifecycleSnapshot(kind).receipt, receipt);
      assert.equal(f.calls.lifecycle[kind].save, 1);
      assert.equal(f.form.getCloseState().unsaved, false);
    } finally { f.dispose(); }
  }
});

test('loaded empty lifecycle lists retry postcommit failure without initializing optional peers', async () => {
  let fail = false;
  const f = await lifecycleFixture({ environment: { VITE_SIVI_COMBINED_EDITING: 'false',
    VITE_SIVI_COLLECTED_EDITING: 'false', VITE_SIVI_SPECIES_EDITING: 'false', VITE_SIVI_IDENTITY_EDITING: 'false' },
    lifecycleRead: (kind, count, value) => { if (kind === 'restoration' && count > 1 && fail) throw new Error('history refresh failed'); return value; } });
  try {
    assert.equal(await f.form.lifecycle('restoration', 'history'), true);
    assert.equal(f.form.lifecycleSnapshot('restoration').history.events.length, 0);
    await lifecycleReview(f, 'deletion');
    lifecycleConfirm(f, 'deletion');
    fail = true;
    assert.equal(await f.form.lifecycle('deletion', 'save'), false);
    const receipt = f.form.lifecycleSnapshot('deletion').receipt;
    assert.equal(f.form.lifecycleNeedsRefresh('deletion'), true);
    fail = false;
    assert.equal(await f.form.lifecycle('deletion', 'refresh'), true);
    assert.deepEqual(f.form.lifecycleSnapshot('deletion').receipt, receipt);
    assert.equal(f.form.lifecycleSnapshot('restoration').history.events.length, 1);
    assert.equal(f.calls.lifecycle.deletion.save, 1);
    assert.equal(f.calls.childRead + f.calls.collectedRead + f.calls.speciesRead + f.calls.identityRead, 0);
  } finally { f.dispose(); }
});

test('each lifecycle review blocks every other owner while explicit known discard stays reachable', async () => {
  for (const kind of ['deletion', 'restoration', 'creationUndo']) {
    const f = await lifecycleFixture({ seedDeletion: kind === 'restoration', environment: {
      VITE_SIVI_COLLECTED_EDITING: 'true', VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_IDENTITY_EDITING: 'true' } });
    try {
      await lifecycleReview(f, kind);
      lifecycleConfirm(f, kind);
      for (const read of [() => f.form.perform('load'), () => f.form.collect('load'),
        () => f.form.select('load'), () => f.form.identify('load'), () => f.form.createRow('load')]) assert.equal(await read(), false);
      for (const peer of ['deletion', 'restoration', 'creationUndo']) if (peer !== kind) {
        assert.equal(await f.form.lifecycle(peer, peer === 'deletion' ? 'load' : 'history'), false);
        assert.equal(f.calls.lifecycle[peer].read, 0);
      }
      f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
      assert.equal(f.parent.getCloseState().unsaved, false);
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'discard' : 'undo'), true);
      assert.equal(f.calls.lifecycle[kind].save, 0);
    } finally { f.dispose(); }
  }
});

test('cancelled lifecycle reads reject stale foreign results after remount and a fresh owning read', async () => {
  for (const kind of ['deletion', 'restoration', 'creationUndo']) {
    const held = deferred();
    const f = await lifecycleFixture({ seedDeletion: kind === 'restoration', lifecycleRead: (current, count, value) => {
      if (current === kind && count === 1) { held.result = value; return held.promise; }
      return value;
    } });
    try {
      const operation = kind === 'deletion' ? 'load' : 'history';
      const reading = f.form.lifecycle(kind, operation);
      assert.equal(f.form.lifecycleSnapshot(kind).busy, true);
      assert.equal(await f.form.lifecycle(kind, 'cancel'), true);
      assert.equal(f.form.lifecycleSnapshot(kind).blocked, false);
      assert.match(f.form.lifecycleSnapshot(kind).error, /cancelled/);
      f.remount();
      assert.equal(await f.parent.loadSource(), true);
      f.parent.unlock();
      assert.equal(await f.form.lifecycle(kind, operation), true);
      held.resolve({ ...held.result, plot: 'FOREIGN' });
      assert.equal(await reading, false);
      assert.equal(f.form.lifecycleSnapshot(kind).error, null);
      assert.equal(f.calls.lifecycle[kind].read, 2);
      assert.equal(f.calls.lifecycle[kind].save, 0);
    } finally { f.dispose(); }
  }
});

test('new creator commits refresh already loaded deletion and historical lists with the new physical target', async () => {
  const f = await lifecycleFixture();
  try {
    for (const kind of ['deletion', 'restoration', 'creationUndo']) {
      assert.equal(await f.form.lifecycle(kind, kind === 'deletion' ? 'load' : 'history'), true);
    }
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f);
    assert.equal(await f.form.createRow('save'), true);
    const receipt = f.form.creationSnapshot().receipt;
    assert.equal(f.form.lifecycleTargets().some(target => target.rowId === receipt.rowId && target.form === receipt.form), true);
    for (const kind of ['deletion', 'restoration', 'creationUndo']) assert.equal(f.calls.lifecycle[kind].read, 2);
    assert.equal(await f.form.lifecycle('deletion', 'review', receipt.rowId, receipt.form), true);
    const original = f.form.lifecycleSnapshot('deletion').original;
    for (const [index, column] of original.columns.entries()) {
      assert.deepEqual(original.original.cells[index], receipt.committed.cells[receipt.columns.findIndex(value => value.name === column.name)]);
    }
    assert.equal(await f.form.lifecycle('deletion', 'undo'), true);
    assert.equal(f.calls.create, 1);
    assert.equal(f.calls.lifecycle.deletion.save, 0);
  } finally { f.dispose(); }
});

test('historical unavailable targets and freshly reviewed invalid originals stay explicit without deletion or repair', async () => {
  for (const [column, value, reason] of [
    ['ID', cell('null'), /NULL or non-signed32/],
    ['ID', cell('integer', '2147483648'), /NULL or non-signed32/],
    ['Species', cell('blob', 'ff'), /BLOB/],
    ['Flag', cell('real', 1), /BOOLEAN/],
  ]) {
    const f = await lifecycleFixture({ deletionCell: { column, value } });
    try {
      assert.equal(await f.form.lifecycle('deletion', 'load'), true, f.form.getCloseState().error);
      assert.equal(f.form.lifecycleTargets().length, 3);
      for (const target of f.form.lifecycleTargets()) assert.match(target.unavailable, reason);
      assert.equal(await f.form.lifecycle('deletion', 'review', f.lifecycleOriginal.original.rowId, 'SubVegA-SIVI'), true);
      assert.match(f.form.lifecycleSnapshot('deletion').error, reason);
      f.form.lifecycleConfirm('deletion', true);
      assert.equal(f.form.getCloseState().blocked, true);
      assert.equal(await f.form.lifecycle('deletion', 'save'), false);
      assert.equal(f.calls.lifecycle.deletion.save, 0);
      assert.equal(await f.form.lifecycle('deletion', 'undo'), true);
    } finally { f.dispose(); }
  }
});

test('one read-only refresh reconciles several retained verified receipts after remount without a peer deadlock', async () => {
  const f = await lifecycleFixture();
  try {
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f);
    assert.equal(await f.form.createRow('save'), true);
    for (const kind of ['deletion', 'restoration', 'creationUndo']) {
      await lifecycleReview(f, kind);
      lifecycleConfirm(f, kind);
      assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save'), true, f.form.getCloseState().error);
    }
    const receipts = Object.fromEntries(['deletion', 'restoration', 'creationUndo'].map(kind =>
      [kind, f.form.lifecycleSnapshot(kind).receipt]));
    const created = f.form.creationSnapshot().receipt;
    assert.equal(f.form.getCloseState().unsaved, false);
    f.remount();
    assert.equal(f.form.creationNeedsRefresh(), true);
    for (const kind of Object.keys(receipts)) assert.equal(f.form.lifecycleNeedsRefresh(kind), true);
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.lifecycle('restoration', 'refresh'), true, f.form.getCloseState().error);
    for (const kind of Object.keys(receipts)) {
      assert.equal(f.form.lifecycleNeedsRefresh(kind), false);
      assert.deepEqual(f.form.lifecycleSnapshot(kind).receipt, receipts[kind]);
      assert.equal(f.calls.lifecycle[kind].save, 1);
    }
    assert.deepEqual(f.form.creationSnapshot().receipt, created);
    assert.equal(f.form.creationNeedsRefresh(), false);
    assert.equal(f.calls.create, 1);
    assert.equal(f.form.getCloseState().unsaved, false);
    assert.equal(await f.form.createRow('start', 'SubVegC-SIVI'), true);
    f.remount();
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.lifecycle('restoration', 'refresh'), false, 'a retained unsent draft is not a receipt-only peer');
    assert.equal(await f.form.createRow('undo'), true, 'explicit owning discard remains reachable beside verified receipts');
    assert.equal(await f.form.lifecycle('restoration', 'refresh'), true);
    assert.equal(f.calls.create, 1);
    assert.equal(f.form.getCloseState().unsaved, false);
  } finally { f.dispose(); }
});

for (const kind of ['deletion', 'restoration', 'creationUndo']) {
test(`retained unknown ${kind} remount uses production recovery unlock without a parent original or write grant`, async () => {
  const held = deferred();
  const parentEntered = deferred(), parentReleased = deferred();
  let failParent = false, holdParent = false;
  const f = await lifecycleFixture({
    parentRead: async (n, value) => {
      if (failParent) {
        if (holdParent) { parentEntered.resolve(); await parentReleased.promise; }
        throw new Error('parent projection unavailable');
      }
      return value;
    },
    lifecycleMutation: (target, request, result) => {
      if (target !== kind) return result;
      held.result = result;
      return held.promise;
    },
  });
  try {
    assert.match(source('SIVICHARSParentForm.svelte'),
      /data-chars-lock\s+disabled=\{lockToggleDisabled\}\s+onclick=\{\(\) => \{ toggleParentLock\(\); \}\}/);
    if (kind === 'restoration') {
      await lifecycleReview(f, 'deletion');
      lifecycleConfirm(f, 'deletion');
      assert.equal(await f.form.lifecycle('deletion', 'save'), true);
    }
    await lifecycleReview(f, kind);
    lifecycleConfirm(f, kind);
    const pending = f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save');
    assert.equal(f.form.lifecycleSnapshot(kind).busy, true);
    assert.equal(await f.form.lifecycle(kind, 'cancel'), true);
    const requestId = f.form.lifecycleSnapshot(kind).requestId;
    failParent = true;
    holdParent = true;
    f.remount();
    const loading = f.parent.loadSource();
    await parentEntered.promise;
    assert.equal(f.parent.lockEligibility().disabled, true);
    assert.equal(f.parent.clickLock(), false, 'busy parent cannot acquire recovery unlock');
    assert.equal(f.parent.canReloadLinkedParent(), false);
    holdParent = false;
    parentReleased.resolve();
    assert.equal(await loading, false);
    assert.deepEqual(f.parent.lockEligibility(), { locked: true, disabled: false, hasOriginal: false, fieldsDisabled: true });
    assert.equal(f.parent.clickLock(), true);
    assert.equal(f.parent.canReloadLinkedParent(), true);
    assert.equal(f.parent.canOpenLinkedVegetation(), false);
    assert.equal(await f.parent.saveForClose(), false);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.perform('load'), false);
    assert.equal(await f.form.lifecycle(kind, 'resolve'), false, 'lookup can succeed while subsequent host refresh fails');
    const resolved = f.form.lifecycleSnapshot(kind);
    assert.equal(f.calls.lifecycle[kind].lookup, 1);
    assert.equal(f.calls.lifecycle[kind].save, 1);
    assert.equal(resolved.blocked, false);
    assert.equal(resolved.requestId, null);
    assert.equal(resolved.receipt.requestId, requestId);
    assert.equal(resolved.receipt.replayed, true);
    assert.equal(f.form.lifecycleNeedsRefresh(kind), true);
    assert.equal(f.form.getCloseState().canSave, false);
    held.resolve(held.result);
    assert.equal(await pending, false);
    failParent = false;
    assert.equal(await f.form.lifecycle(kind, 'refresh'), true);
    assert.equal(f.form.lifecycleNeedsRefresh(kind), false);
    assert.equal(f.calls.lifecycle[kind].save, 1);
  } finally { held.resolve(held.result); parentReleased.resolve(); f.dispose(); }
});
}

test('missing parent without a retained peer cannot use the production recovery-only unlock', async () => {
  let failParent = false;
  const f = await fixture({ parentRead: (n, value) => {
    if (failParent) throw new Error('parent projection unavailable');
    return value;
  } });
  try {
    failParent = true;
    f.remount();
    assert.equal(await f.parent.loadSource(), false);
    assert.equal(f.parent.lockEligibility().disabled, true);
    assert.equal(f.parent.clickLock(), false);
    assert.equal(f.parent.canReloadLinkedParent(), false);
    assert.equal(f.parent.canOpenLinkedVegetation(), false);
  } finally { f.dispose(); }
});

for (const disturbance of ['revision', 'review', 'unknown', 'discarded-review']) {
  test(`awaited common refresh preserves newer ${disturbance} authority and requires explicit recovery`, async () => {
    const entered = deferred(), release = deferred(), acknowledgement = deferred();
    let holdRefresh = false, holdMutation = false;
    const f = await lifecycleFixture({
      refresh: async () => { if (holdRefresh) { entered.resolve(); await release.promise; } },
      lifecycleMutation: (kind, request, result) => {
        if (!holdMutation) return result;
        acknowledgement.result = result;
        return acknowledgement.promise;
      },
    });
    try {
      for (const kind of ['deletion', 'restoration', 'creationUndo']) {
        await lifecycleReview(f, kind);
        lifecycleConfirm(f, kind);
        assert.equal(await f.form.lifecycle(kind, kind === 'creationUndo' ? 'undo' : 'save'), true);
      }
      const original = f.form.lifecycleSnapshot('deletion').receipt;
      f.remount();
      assert.equal(await f.parent.loadSource(), true);
      assert.equal(f.parent.unlock(), true);
      holdRefresh = true;
      const refreshing = f.form.lifecycle('restoration', 'refresh');
      await entered.promise;
      const session = f.form.retainedLifecycle('deletion');
      assert.equal(await session.loadTargets(), true);
      assert.equal(await session.review(original.form, original.rowId), true);
      let saving;
      if (disturbance === 'discarded-review') session.discard();
      else if (disturbance !== 'review') {
        session.confirm(true);
        holdMutation = disturbance === 'unknown';
        saving = session.save(require('node:crypto').randomUUID());
        if (disturbance === 'unknown') {
          session.cancel();
          assert.equal(session.view().blocked, true);
        } else {
          assert.equal(await saving, true);
          assert.equal(session.view().revision, 2);
        }
      }
      const pendingRequest = session.view().requestId;
      release.resolve();
      assert.equal(await refreshing, true);
      holdRefresh = false;
      if (disturbance !== 'discarded-review') {
        assert.equal(f.form.getCloseState().canSave, false);
        assert.equal(await f.form.perform('load'), false);
      }
      if (disturbance === 'revision') {
        assert.equal(f.form.lifecycleNeedsRefresh('deletion'), true);
        assert.equal(await f.form.lifecycle('deletion', 'refresh'), true);
      } else if (disturbance === 'review') {
        assert.equal(session.view().original.original.rowId, original.rowId);
        assert.equal(session.closeState().unsaved, true);
        assert.equal(await f.form.lifecycle('deletion', 'undo'), true);
      } else if (disturbance === 'unknown') {
        assert.equal(session.view().requestId, pendingRequest);
        assert.equal(session.view().blocked, true);
        assert.equal(await f.form.lifecycle('deletion', 'undo'), false);
        assert.equal(await f.form.lifecycle('deletion', 'resolve'), true);
        acknowledgement.resolve(acknowledgement.result);
        assert.equal(await saving, false);
        assert.equal(session.view().receipt.requestId, pendingRequest);
        assert.equal(session.view().receipt.replayed, true);
      } else {
        assert.equal(session.view().original, null);
        assert.equal(session.view().receipt, null, 'a discarded review cannot resurrect its earlier receipt');
        assert.equal(f.form.lifecycleNeedsRefresh('restoration'), false);
        assert.equal(f.form.lifecycleNeedsRefresh('creationUndo'), false);
      }
      assert.equal(f.calls.lifecycle.deletion.save, disturbance === 'review' || disturbance === 'discarded-review' ? 1 : 2);
      assert.equal(f.form.getCloseState().unsaved, false);
    } finally { release.resolve(); acknowledgement.resolve(acknowledgement.result); f.dispose(); }
  });
}

test('actual CHARS creation fields expose fourteen labelled covers/totals and no height or identity assignment', () => {
  const f = fixture();
  try {
    const expected = {
      'SubVegA-SIVI': ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB'],
      'SubVegC-SIVI': ['Cover6'], 'SubVegD-SIVI': ['Cover7', 'Cover8', 'Cover9'],
    };
    let controls = 0;
    for (const [form, columns] of Object.entries(expected)) {
      const html = f.creationFields(form);
      for (const column of columns) {
        const id = `sivi-creation-${form}-${column}`;
        assert.equal(html.split(`id="${id}"`).length - 1, 1);
        assert.match(html, new RegExp(`for="${id}"`));
        controls++;
      }
      assert.equal([...html.matchAll(/<input[^>]*id="sivi-creation-/g)].length, columns.length + 1);
      assert.doesNotMatch(html, /id="[^"]*(?:Height|Layer|Collected|PlotNumber|\bID)"/);
    }
    assert.equal(controls, 14);
  } finally { f.dispose(); }
});

async function creationFixture(options = {}) {
  const f = fixture({ ...options, environment: { VITE_SIVI_CREATION_WRITING: 'true', ...options.environment } });
  assert.equal(await f.parent.loadSource(), true);
  f.parent.unlock();
  return f;
}
function validCreation(f, form = 'SubVegA-SIVI', raw = '0') {
  const editor = f.creationEditor();
  const original = f.form.creationSnapshot().draft;
  assert.equal(original.form, form);
  const species = form === 'SubVegD-SIVI' ? 'MOSS' : form === 'SubVegC-SIVI' ? 'MINE' : 'TREE';
  const column = form === 'SubVegC-SIVI' ? 'Cover6' : form === 'SubVegD-SIVI' ? 'Cover7' : 'Cover5a';
  let draft = editor.stageSIVICreationSpecies(original, species);
  if (form === 'SubVegC-SIVI') draft = editor.chooseSIVICreationSpecies(draft, f.form.creationSnapshot().references,
    { contextId: draft.contextId, project: draft.project, plot: draft.plot }, 'user', 'MINE');
  f.form.creationDraft(editor.stageSIVICreationCover(draft, column, raw, false));
}

test('actual CHARS creator requires its own gate, source parent and exact three forms without a second owner', async () => {
  const off = fixture();
  try {
    assert.equal(await off.form.createRow('load'), false);
    assert.equal(off.calls.creationReferences, 0);
  } finally { off.dispose(); }
  const f = fixture({ environment: { VITE_SIVI_CREATION_WRITING: 'true', VITE_SIVI_COMBINED_EDITING: 'false' } });
  try {
    assert.equal(await f.form.createRow('load'), false);
    assert.equal(f.calls.creationReferences, 0);
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI_BC'), false);
    for (const form of ['SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI']) {
      assert.equal(await f.form.createRow('start', form), true);
      validCreation(f, form);
      assert.equal(f.form.getCloseState().canSave, true, f.form.getCloseState().saveReason);
      assert.equal(await f.form.saveForClose(), true);
      const receipt = f.form.creationSnapshot().receipt;
      assert.equal(receipt.form, form);
      assert.equal(receipt.original.cells.length, 44);
      assert.equal(receipt.original.cells.filter(value => value.storage === 'null').length, 43);
      assert.equal(receipt.committed.cells.filter(value => value.storage === 'null').length, 39);
      assert.deepEqual(receipt.covers.map(cover => [cover.column, cover.value.real]),
        [[form === 'SubVegC-SIVI' ? 'Cover6' : form === 'SubVegD-SIVI' ? 'Cover7' : 'Cover5a', 0]]);
      assert.equal(Object.hasOwn(f.calls.creationRequest, 'id'), false);
      assert.equal(f.form.creationNeedsRefresh(), false);
    }
    assert.equal(f.calls.create, 3);
    assert.equal(f.calls.parentSave, 0);
    assert.equal(f.calls.creationLookup, 0);
  } finally { f.dispose(); }
});

test('creator raw validation and owning correction survive remount and block every child/parent and close Save', async () => {
  const f = await creationFixture({ environment: {
    VITE_SIVI_COLLECTED_EDITING: 'true', VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_IDENTITY_EDITING: 'true' } });
  try {
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f, 'SubVegA-SIVI', '100');
    const retained = f.form.retainedCreation();
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.perform('load'), false);
    assert.equal(await f.form.collect('load'), false);
    assert.equal(await f.form.select('load'), false);
    assert.equal(await f.form.identify('load'), false);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    f.remount();
    assert.equal(f.form.retainedCreation(), retained);
    assert.equal(f.form.creationSnapshot().draft.cells.Cover5a.raw, '100');
    assert.equal(f.form.getCloseState().canSave, false);
    assert.equal(await f.parent.loadSource(), true, 'fresh parent read remains available for retained peer recovery');
    f.parent.unlock();
    validCreation(f);
    assert.equal(f.form.creationSnapshot().error, null);
    assert.equal(f.form.getCloseState().canSave, true);
    assert.equal(await f.form.createRow('undo'), true);
    assert.equal(f.calls.create, 0);
    assert.equal(f.form.getCloseState().unsaved, false);
  } finally { f.dispose(); }
});

test('a retained normal standard-A draft is not silently rebound, submitted or erased by CHARS remount', async () => {
  const f = await creationFixture();
  try {
    assert.equal(await f.form.createRow('load'), true);
    f.form.retainedCreation().start('SubVegA-SIVI_BC');
    f.remount();
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(f.form.creationSnapshot().draft.form, 'SubVegA-SIVI_BC');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.form.createRow('save'), false);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.create, 0);
    assert.equal(await f.form.createRow('undo'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
  } finally { f.dispose(); }
});

test('verified creation refresh failure preserves receipt and reachable read-only recovery without originals or Create replay', async () => {
  let fail = true;
  const f = await creationFixture({ parentRead: (count, value) => {
    if (count > 1 && fail) throw new Error('parent projection unavailable');
    return value;
  } });
  try {
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.create, 1);
    assert.equal(f.form.creationSnapshot().draft, null);
    assert.ok(f.form.creationSnapshot().receipt);
    assert.equal(f.form.creationNeedsRefresh(), true);
    assert.equal(f.parent.snapshot().original, null);
    assert.equal(await f.form.createRow('save'), false);
    assert.equal(f.calls.create, 1);
    fail = false;
    assert.equal(await f.form.createRow('refresh'), true);
    assert.equal(f.form.creationNeedsRefresh(), false);
    assert.equal(f.calls.childRead, 2);
    const receipt = f.form.creationSnapshot().receipt;
    f.remount();
    assert.deepEqual(f.form.creationSnapshot().receipt, receipt);
    assert.equal(f.form.creationNeedsRefresh(), true, 'new host independently refreshes the retained irreversible receipt');
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.createRow('refresh'), true);
    assert.equal(f.calls.create, 1);
  } finally { f.dispose(); }
});

test('cancelled actual Create acknowledgement retains exact request; remount resolves read-only and rejects old completion', async () => {
  const held = deferred();
  const f = await creationFixture({ create: (_request, result) => { held.result = result; return held.promise; } });
  try {
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f);
    const saving = f.form.saveForClose();
    assert.equal(f.form.getCloseState().busy, true);
    assert.equal(await f.form.createRow('cancel'), true);
    const requestId = f.form.creationSnapshot().requestId;
    assert.equal(f.form.creationSnapshot().blocked, true);
    f.remount();
    assert.equal(f.form.creationSnapshot().requestId, requestId);
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.createRow('resolve'), true);
    assert.equal(f.calls.create, 1);
    assert.equal(f.calls.creationLookup, 1);
    assert.equal(f.form.creationSnapshot().receipt.replayed, true);
    held.resolve(held.result);
    assert.equal(await saving, false);
    assert.equal(f.form.creationSnapshot().receipt.replayed, true);
    assert.equal(f.form.getCloseState().unsaved, false);
  } finally { f.dispose(); }
});

test('creator definition Cancel and stale completion retain the current owner and explicit read retry', async () => {
  const held = deferred();
  const f = await creationFixture({ creationReferences: (count, value) => {
    if (count === 1) { held.result = value; return held.promise; }
    return value;
  } });
  try {
    const loading = f.form.createRow('load');
    assert.equal(f.form.creationSnapshot().busy, true);
    assert.equal(await f.form.createRow('cancel'), true);
    assert.match(f.form.creationSnapshot().error, /cancelled/);
    assert.equal(await f.form.createRow('start', 'SubVegC-SIVI'), false);
    f.remount();
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.createRow('load'), true);
    held.resolve({ ...held.result, plot: 'FOREIGN' });
    assert.equal(await loading, false);
    assert.equal(f.form.creationSnapshot().error, null);
    assert.equal(await f.form.createRow('start', 'SubVegC-SIVI'), true);
  } finally { f.dispose(); }
});

test('every loaded peer refreshes after Create, and failed sibling refresh keeps the same irreversible receipt', async () => {
  let fail = false;
  const f = await creationFixture({ environment: {
    VITE_SIVI_COLLECTED_EDITING: 'true', VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_IDENTITY_EDITING: 'true' },
    collectedRead: (_count, value) => { if (fail) throw new Error('Collected source unavailable'); return value; } });
  try {
    for (const load of [() => f.form.perform('load'), () => f.form.collect('load'),
      () => f.form.select('load'), () => f.form.identify('load')]) assert.equal(await load(), true);
    assert.equal(await f.form.createRow('load'), true);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), true);
    validCreation(f);
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    const receipt = f.form.creationSnapshot().receipt;
    assert.equal(f.form.creationNeedsRefresh(), true);
    fail = false;
    assert.equal(await f.form.createRow('refresh'), true);
    assert.deepEqual(f.form.creationSnapshot().receipt, receipt);
    assert.equal(f.calls.create, 1);
    assert.equal(f.calls.childRead, 3);
    assert.equal(f.calls.collectedRead, 3);
    assert.equal(f.calls.speciesRead, 2);
    assert.equal(f.calls.identityRead, 2);
    for (const view of [f.form.snapshot(), f.form.collectedSnapshot(), f.form.speciesSnapshot(), f.form.identitySnapshot()]) {
      const added = view.review[0].Rows.find(row => row.rowId === receipt.rowId);
      assert.ok(added, 'every loaded peer sees the newly created physical row after explicit recovery');
    }
  } finally { f.dispose(); }
});

test('dirty child peers prevent creator reference reads, start, corrections and mutations before transport', async () => {
  const f = await creationFixture();
  try {
    assert.equal(await f.form.perform('load'), true);
    f.form.draft(f.rowId, 'Cover5a', '5');
    assert.equal(await f.form.createRow('load'), false);
    assert.equal(await f.form.createRow('start', 'SubVegA-SIVI'), false);
    assert.equal(f.calls.creationReferences, 0);
    assert.equal(f.calls.create, 0);
    assert.equal(f.form.getCloseState().unsaved, true);
  } finally { f.dispose(); }
});

test('actual client owners react across peer drafts, lock, invalid correction, close Save and independent restoration', async () => {
  const f = fixture();
  try {
    assert.equal(await f.parent.loadSource(), true);
    assert.equal(await f.form.perform('load'), false);
    f.parent.unlock();
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.perform('load'), false);
    assert.equal(f.calls.childRead, 0);
    assert.equal(await f.parent.undoSource(), true);
    assert.equal(await f.form.perform('load'), true, f.form.snapshot().error);
    assert.equal(f.form.snapshot().extended, true);
    assert.equal(f.form.snapshot().review[0].Form, 'SubVegA-SIVI');
    f.form.draft(f.rowId, 'Cover5a', '100');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(f.form.getCloseState().canSave, false);
    const refreshes = f.calls.refresh;
    await assert.rejects(f.form.parentSaved(), /Linked combined drafts\/recovery/);
    assert.equal(f.calls.refresh, refreshes, 'dirty peer preflight must precede header refresh');
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'BLOCK' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    assert.match(f.parent.getCloseState().error, /linked vegetation/);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 0);
    f.form.draft(f.rowId, 'Cover5a', '3.5');
    f.form.draft(f.rowId, 'HeightB', 'literal height');
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.form.getCloseState().canSave, true);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.request.edits.length, 2);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().unsaved, false);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.perform('prune'), false);
    assert.equal(f.calls.restore, 0);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.parentSave, 1);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.snapshot().historyId, '201', 'parent refresh must not erase the child receipt');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(f.calls.restore, 1);
    assert.equal(f.parent.snapshot().historyId, '101', 'child refresh must not erase the parent receipt');
    assert.equal(await f.parent.restoreSource(), true);
    assert.equal(f.parent.snapshot().historyId, null);
  } finally { f.dispose(); }
});

test('postcommit refresh failure blocks replay and retains the receipt until explicit recovery', async () => {
  let fail = false;
  const f = fixture({ refresh: async () => { if (fail) throw new Error('held header failure'); } });
  try {
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.perform('load'), true, f.form.snapshot().error);
    f.form.draft(f.rowId, 'Cover5b', '2');
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 1);
    fail = false;
    assert.equal(await f.form.perform('undo'), true);
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(await f.form.perform('prune'), true);
  } finally { f.dispose(); }
});

test('failed parent source refresh recovers in place without losing either history or replaying child Save', async () => {
  let fail = false;
  const f = fixture({ parentRead: (_count, value) => {
    if (fail) throw new Error('transient owned parent read failure');
    return value;
  } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.perform('load'), true);
    f.form.draft(f.rowId, 'Cover5b', '2');
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.parent.snapshot().original, null);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.parent.undoSource(), false, 'ordinary parent Undo must not bypass the blocked peer');
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.perform('undo'), false, 'failed recovery remains explicit and retryable');
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.parent.snapshot().historyId, '101');
    fail = false;
    assert.equal(await f.form.perform('undo'), true);
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.parentSave, 1);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(await f.parent.restoreSource(), true);
  } finally { f.dispose(); }
});

test('held source read blocks the root and cancellation/retirement never populates a replacement owner', async () => {
  const hold = deferred();
  const f = fixture({ read: (count, value) => count === 1 ? hold.promise : Promise.resolve(value),
    cancel: () => hold.reject(new Error('owned read cancelled')) });
  try {
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    const pending = f.form.perform('load');
    assert.equal(f.form.getCloseState().busy, true);
    assert.equal(f.form.getCloseState().canSave, false);
    f.form.cancelSource();
    assert.equal(await pending, false);
    assert.match(f.form.snapshot().error, /cancelled/);
    assert.equal(f.form.getCloseState().busy, false);
    assert.equal(f.calls.childSave, 0);
    assert.equal(await f.form.perform('load'), true);
    assert.equal(f.form.snapshot().error, null);
  } finally { f.dispose(); }
  const late = deferred();
  const old = fixture({ read: () => late.promise });
  assert.equal(await old.parent.loadSource(), true);
  old.parent.unlock();
  const pending = old.form.perform('load');
  old.dispose();
  const replacement = fixture();
  try {
    assert.equal(await replacement.parent.loadSource(), true);
    replacement.parent.unlock();
    late.resolve(review({ extended: true }));
    assert.equal(await pending, false);
    assert.equal(replacement.form.snapshot().review, null);
    assert.equal(replacement.calls.childRead, 0);
  } finally { replacement.dispose(); }
});

test('backend read refusal remains visible and correction/retry uses the same owner without mutation', async () => {
  const f = fixture({ read: (count, value) => {
    if (count === 1) throw new Error('SIVI combined cover/height editing is disabled in this session');
    return value;
  } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), false);
    assert.equal(f.form.snapshot().review, null);
    assert.match(f.form.snapshot().error, /disabled in this session/);
    assert.equal(f.calls.childSave, 0);
    assert.equal(await f.form.perform('load'), true);
    assert.equal(f.form.snapshot().error, null);
    assert.equal(f.calls.childRead, 2);
  } finally { f.dispose(); }
});

test('held mutation cannot be cancelled or replayed and guards parent edits until its actual acknowledgement', async () => {
  const hold = deferred();
  const f = fixture({ save: () => hold.promise });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    f.form.draft(f.rowId, 'Cover5c', '1');
    const saving = f.form.saveForClose();
    assert.equal(f.form.getCloseState().busy, true);
    const cancellations = f.calls.cancelled;
    f.form.cancelSource();
    assert.equal(f.calls.cancelled, cancellations);
    assert.match(f.form.getCloseState().error, /Only a linked source read/);
    assert.equal(await f.form.saveForClose(), false);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'BLOCK' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    assert.equal(f.calls.childSave, 1);
    hold.resolve({ ChangedCells: 1, HistoryID: '201' });
    assert.equal(await saving, true);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().busy, false);
    assert.equal(f.form.getCloseState().error, null);
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(f.calls.childSave, 1);
  } finally { f.dispose(); }
});

test('Collected-only actual client cycles known source values and omits full-cycle/unknown historical noops', async () => {
  for (const [original, expected] of [
    [cell('null'), ['C', 'V', null]], [cell('text', 'C'), ['V', null, 'C']],
    [cell('text', 'V'), [null, 'C', 'V']], [cell('text', '?'), ['?', '?', '?']],
    [cell('text', ''), ['', '', '']],
  ]) {
    const f = fixture({ environment: { VITE_SIVI_COMBINED_EDITING: 'false', VITE_SIVI_COLLECTED_EDITING: 'true' },
      collectedValue: original });
    try {
      assert.equal(await f.parent.loadSource(), true);
      assert.equal(await f.form.collect('load'), false);
      f.parent.unlock();
      assert.equal(await f.form.collect('load'), true);
      assert.equal(f.form.collectedSnapshot().extended, true);
      for (const value of expected) {
        f.form.cycle(f.rowId);
        const draft = f.form.collectedSnapshot().drafts[f.rowId];
        assert.equal(draft.value.storage, value === null ? 'null' : 'text');
        assert.equal(draft.value.text, value);
      }
      assert.equal(f.form.getCloseState().unsaved, false);
      assert.equal(await f.form.collect('save'), true);
      assert.equal(f.calls.collectedSave, 0);
      assert.equal(f.calls.childRead, 0, 'Collected must not require combined');
      assert.equal(f.calls.childSave, 0);
    } finally { f.dispose(); }
  }
});

test('three actual client owners guard drafts and retain independent receipts through each typed restoration', async () => {
  const f = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    f.form.cycle(f.rowId);
    assert.equal(f.form.collectedSnapshot().drafts[f.rowId].value.text, 'C');
    assert.equal(f.form.getCloseState().canSave, true);
    f.form.draft(f.rowId, 'Cover5a', '2');
    assert.equal(f.form.snapshot().drafts.covers[f.rowId], undefined);
    assert.match(f.form.getCloseState().error, /Linked collected drafts/);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    assert.equal(await f.form.perform('load'), false);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.collectedSave, 1);
    assert.equal(f.calls.collectedRequest.edits.length, 1);
    assert.equal(f.calls.collectedRequest.edits[0].form, 'SubVegA-SIVI');
    assert.equal(f.calls.collectedRequest.edits[0].clicks, 1);
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    assert.equal(f.form.snapshot().review[0].Rows[0].cells.at(-1).text, 'C');
    f.form.draft(f.rowId, 'Cover5a', '2');
    f.form.cycle(f.rowId);
    assert.deepEqual(f.form.collectedSnapshot().drafts, {});
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.collect('prune'), false);
    assert.equal(f.calls.collectedRestore, 0);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.collect('prune'), true);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.parent.restoreSource(), true);
    assert.equal(f.calls.collectedSave, 1);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.parentSave, 1);
  } finally { f.dispose(); }
});

for (const target of ['combined', 'collected']) {
  test(`${target} postcommit sibling-read failure retains reachable recovery and exactly one mutation`, async () => {
    let fail = false;
    const f = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' },
      read: (_count, value) => { if (fail && target === 'collected') throw new Error('combined peer read failed'); return value; },
      collectedRead: (_count, value) => { if (fail && target === 'combined') throw new Error('collected peer read failed'); return value; },
    });
    const operation = kind => target === 'combined' ? f.form.perform(kind) : f.form.collect(kind);
    const view = () => target === 'combined' ? f.form.snapshot() : f.form.collectedSnapshot();
    const sibling = () => target === 'combined' ? f.form.collectedSnapshot() : f.form.snapshot();
    const expectedHistory = target === 'combined' ? '201' : '301';
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.perform('load'), true);
      assert.equal(await f.form.collect('load'), true);
      if (target === 'combined') f.form.draft(f.rowId, 'Cover5b', '2');
      else f.form.cycle(f.rowId);
      fail = true;
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(view().historyId, expectedHistory);
      assert.equal(sibling().review, null);
      assert.equal(f.form.getCloseState().blocked, true);
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(await f.parent.undoSource(), false);
      assert.equal(await operation('undo'), false);
      assert.equal(view().historyId, expectedHistory);
      fail = false;
      assert.equal(await operation('undo'), true);
      assert.ok(sibling().review);
      assert.equal(view().historyId, expectedHistory);
      assert.equal(f.form.getCloseState().blocked, false);
      assert.equal(f.calls.childSave + f.calls.collectedSave, 1);
      assert.equal(await operation('prune'), true);
    } finally { f.dispose(); }
  });
}

test('parent postcommit failed child refresh retries unavailable source without dropping either child receipt', async () => {
  let fail = false;
  const f = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' },
    collectedRead: (_count, value) => { if (fail) throw new Error('owned Collected refresh failed'); return value; } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    f.form.cycle(f.rowId); assert.equal(await f.form.saveForClose(), true);
    f.form.draft(f.rowId, 'Cover5a', '3'); assert.equal(await f.form.saveForClose(), true);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.collectedSnapshot().review, null);
    assert.equal(await f.parent.undoSource(), false);
    fail = false;
    assert.equal(await f.parent.undoSource(), true);
    assert.ok(f.form.collectedSnapshot().review);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(await f.form.collect('prune'), true);
    assert.equal(await f.parent.restoreSource(), true);
    assert.deepEqual([f.calls.parentSave, f.calls.childSave, f.calls.collectedSave], [1, 1, 1]);
  } finally { f.dispose(); }
});

test('Collected read Cancel and mutation acknowledgement keep operation-specific ownership and cannot replay', async () => {
  const read = deferred(), save = deferred();
  const f = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' },
    collectedRead: (count, value) => count === 1 ? read.promise : value,
    cancel: () => read.reject(new Error('owned Collected read cancelled')),
    collectedSave: () => save.promise });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    const loading = f.form.collect('load');
    assert.equal(f.form.getCloseState().busy, true);
    f.form.cancelSource();
    assert.equal(f.calls.cancelled, 0, 'another child cannot cancel this read');
    f.form.cancelCollected();
    assert.equal(await loading, false);
    assert.match(f.form.collectedSnapshot().error, /owned Collected read cancelled/);
    assert.equal(await f.form.collect('load'), true);
    f.form.cycle(f.rowId);
    const saving = f.form.saveForClose();
    assert.equal(f.form.getCloseState().busy, true);
    const cancelled = f.calls.cancelled;
    f.form.cancelCollected();
    assert.equal(f.calls.cancelled, cancelled);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.perform('load'), false);
    assert.equal(f.calls.collectedSave, 1);
    save.resolve({ ChangedCells: 1, HistoryID: '301' });
    assert.equal(await saving, true);
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    assert.equal(await f.form.collect('prune'), true);
  } finally { f.dispose(); }
});

test('Collected disabled backend and historical nontext remain explicit without a success-shaped write', async () => {
  const unavailable = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' },
    collectedRead: () => { throw new Error('Collected editing is disabled in this session'); } });
  try {
    assert.equal(await unavailable.parent.loadSource(), true); unavailable.parent.unlock();
    assert.equal(await unavailable.form.collect('load'), false);
    assert.match(unavailable.form.collectedSnapshot().error, /disabled in this session/);
    assert.equal(unavailable.calls.collectedSave, 0);
  } finally { unavailable.dispose(); }
  const historical = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' }, collectedValue: cell('integer', '1') });
  try {
    assert.equal(await historical.parent.loadSource(), true); historical.parent.unlock();
    assert.equal(await historical.form.collect('load'), true);
    historical.form.cycle(historical.rowId);
    assert.match(historical.form.getCloseState().error, /Historical non-text/);
    assert.deepEqual(historical.form.collectedSnapshot().drafts, {});
    assert.equal(historical.calls.collectedSave, 0);
  } finally { historical.dispose(); }
});

test('late Collected source delivery cannot populate a remounted actual client owner', async () => {
  const late = deferred();
  const old = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' }, collectedRead: () => late.promise });
  assert.equal(await old.parent.loadSource(), true); old.parent.unlock();
  const pending = old.form.collect('load');
  old.dispose();
  const replacement = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' } });
  try {
    assert.equal(await replacement.parent.loadSource(), true); replacement.parent.unlock();
    late.resolve(review({ extended: true }));
    assert.equal(await pending, false);
    assert.equal(replacement.form.collectedSnapshot().review, null);
    assert.equal(replacement.calls.collectedRead, 0);
    assert.equal(await replacement.form.collect('load'), true);
  } finally { replacement.dispose(); }
});

test('same-row sibling history preserves the backend full-row restoration guard rather than granting out-of-order Undo', async () => {
  const f = fixture({ environment: { VITE_SIVI_COLLECTED_EDITING: 'true' } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    f.form.cycle(f.rowId); assert.equal(await f.form.saveForClose(), true);
    f.form.draft(f.rowId, 'Cover5a', '3'); assert.equal(await f.form.saveForClose(), true);
    assert.equal(await f.form.collect('prune'), false);
    assert.match(f.form.collectedSnapshot().error, /physical row changed/);
    assert.equal(f.form.collectedSnapshot().blocked, true);
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(f.form.snapshot().review[0].Rows[0].cells.at(-1).text, 'C');
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.collectedSave, 1);
  } finally { f.dispose(); }
});

const identityEnvironment = { VITE_SIVI_IDENTITY_EDITING: 'true', VITE_SIVI_COLLECTED_EDITING: 'true',
  VITE_SIVI_SPECIES_EDITING: 'true' };

for (const [raw, nil, expected] of [
  ['-2147483648', false, cell('integer', '-2147483648')],
  ['2147483647', false, cell('integer', '2147483647')],
  ['', true, cell('null')],
]) {
  test(`identity-only client saves exact ${nil ? 'NULL' : raw} and restores with zero source audits`, async () => {
    const f = fixture({ environment: { VITE_SIVI_IDENTITY_EDITING: 'true', VITE_SIVI_COMBINED_EDITING: 'false' } });
    try {
      assert.equal(await f.parent.loadSource(), true);
      assert.equal(await f.form.identify('load'), false);
      f.parent.unlock();
      assert.equal(await f.form.identify('load'), true);
      f.form.identityDraft(f.rowId, raw, nil);
      assert.equal(f.form.getCloseState().canSave, true);
      assert.equal(await f.form.saveForClose(), true);
      assert.deepEqual(f.calls.identityRequest.edits, [{ rowId: f.rowId, form: 'SubVegA-SIVI',
        column: 'ID', expected: cell('integer', '1'), value: expected }]);
      assert.equal(f.form.identitySnapshot().historyId, '501');
      assert.equal(await f.form.identify('prune'), true);
      assert.deepEqual(f.form.identitySnapshot().review[0].Rows[0].cells[0], cell('integer', '1'));
      assert.deepEqual([f.calls.identitySave, f.calls.identityRestore, f.calls.childRead,
        f.calls.collectedRead, f.calls.speciesRead, f.calls.parentSave], [1, 1, 0, 0, 0, 0]);
    } finally { f.dispose(); }
  });
}

test('identity invalid raw input blocks every actual peer while valid correction and owning Undo remain live', async () => {
  const f = fixture({ environment: identityEnvironment });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    assert.equal(await f.form.select('load'), true);
    assert.equal(await f.form.identify('load'), true);
    for (const raw of ['-2147483649', '2147483648', ' 2', '1.0', '02', '-0', '\ud800']) {
      f.form.identityDraft(f.rowId, raw);
      assert.equal(f.form.identitySnapshot().drafts[f.rowId].raw, raw);
      assert.ok(f.form.identitySnapshot().drafts[f.rowId].error);
      assert.equal(f.form.getCloseState().unsaved, true);
      assert.equal(f.form.getCloseState().canSave, false);
      assert.equal(await f.form.saveForClose(), false);
      f.form.draft(f.rowId, 'Cover5a', '2'); f.form.cycle(f.rowId); f.form.speciesDraft(f.rowId, 'TREE');
      f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
      assert.deepEqual(f.form.snapshot().drafts.covers, {});
      assert.deepEqual(f.form.collectedSnapshot().drafts, {});
      assert.deepEqual(f.form.speciesSnapshot().drafts, {});
      assert.equal(f.parent.getCloseState().unsaved, false);
      assert.equal(await f.parent.undoSource(), false);
      f.form.identityDraft(f.rowId, '2');
      assert.equal(f.form.identitySnapshot().drafts[f.rowId].error, null);
      assert.equal(f.form.getCloseState().canSave, true);
      assert.equal(await f.form.identify('undo'), true);
      assert.equal(f.form.getCloseState().unsaved, false);
    }
    assert.equal(f.calls.identitySave + f.calls.speciesSave + f.calls.collectedSave + f.calls.childSave + f.calls.parentSave, 0);
  } finally { f.dispose(); }
});

test('five owners retain independent histories and restore complete same-row witnesses in reverse edit order', async () => {
  const f = fixture({ environment: identityEnvironment });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    for (const load of [f.form.perform, f.form.collect, f.form.select, f.form.identify]) assert.equal(await load('load'), true);
    f.form.identityDraft(f.rowId, '2'); assert.equal(await f.form.saveForClose(), true);
    f.form.speciesDraft(f.rowId, 'TREE'); assert.equal(await f.form.saveForClose(), true);
    f.form.cycle(f.rowId); assert.equal(await f.form.saveForClose(), true);
    f.form.draft(f.rowId, 'Cover5a', '3'); assert.equal(await f.form.saveForClose(), true);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' }); assert.equal(await f.form.saveForClose(), true);
    assert.deepEqual([f.parent.snapshot().historyId, f.form.snapshot().historyId, f.form.collectedSnapshot().historyId,
      f.form.speciesSnapshot().historyId, f.form.identitySnapshot().historyId], ['101', '201', '301', '401', '501']);
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(await f.form.collect('prune'), true);
    assert.equal(await f.form.select('prune'), true);
    assert.equal(await f.form.identify('prune'), true);
    assert.equal(await f.parent.restoreSource(), true);
    assert.deepEqual(f.form.identitySnapshot().review[0].Rows[0].cells[0], cell('integer', '1'));
    assert.equal(f.form.speciesSnapshot().review[0].Rows[0].cells[2].text, 'RAW');
    assert.deepEqual([f.calls.parentSave, f.calls.childSave, f.calls.collectedSave, f.calls.speciesSave, f.calls.identitySave],
      [1, 1, 1, 1, 1]);
    assert.deepEqual([f.calls.restore, f.calls.collectedRestore, f.calls.speciesRestore, f.calls.identityRestore], [1, 1, 1, 1]);
  } finally { f.dispose(); }
});

for (const owner of ['combined', 'collected', 'species', 'identity']) {
  test(`${owner} retains exactly-once recovery after a different identity/reference owner loses originals`, async () => {
    let fail = false;
    const f = fixture({ environment: identityEnvironment,
      identityRead: (_count, value) => { if (fail && owner !== 'identity') throw new Error('ID source disappeared'); return value; },
      speciesReferences: (_count, value) => { if (fail && owner === 'identity') throw new Error('Species references disappeared'); return value; } });
    const operate = { combined: f.form.perform, collected: f.form.collect, species: f.form.select, identity: f.form.identify }[owner];
    const view = { combined: f.form.snapshot, collected: f.form.collectedSnapshot,
      species: f.form.speciesSnapshot, identity: f.form.identitySnapshot }[owner];
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      for (const load of [f.form.perform, f.form.collect, f.form.select, f.form.identify]) assert.equal(await load('load'), true);
      if (owner === 'combined') f.form.draft(f.rowId, 'Cover5a', '3');
      else if (owner === 'collected') f.form.cycle(f.rowId);
      else if (owner === 'species') f.form.speciesDraft(f.rowId, 'TREE');
      else f.form.identityDraft(f.rowId, '2');
      fail = true;
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(view().historyId, { combined: '201', collected: '301', species: '401', identity: '501' }[owner]);
      assert.equal(f.form.getCloseState().blocked, true);
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(await operate('undo'), false);
      if (owner !== 'identity') assert.equal(f.form.identitySnapshot().review, null);
      fail = false;
      assert.equal(await operate('undo'), true);
      assert.ok(f.form.identitySnapshot().review);
      assert.ok(f.form.speciesSnapshot().references);
      assert.equal(f.form.getCloseState().blocked, false);
      assert.equal(f.calls.identitySave + f.calls.speciesSave + f.calls.collectedSave + f.calls.childSave, 1);
      assert.equal(await operate('prune'), true);
    } finally { f.dispose(); }
  });
}

test('identity postcommit absent-parent recovery and parent failed-ID refresh remain reachable without replay', async () => {
  for (const owner of ['identity', 'parent']) {
    let fail = false;
    const f = fixture({ environment: identityEnvironment,
      parentRead: (_count, value) => { if (fail && owner === 'identity') throw new Error('CHARS parent disappeared'); return value; },
      identityRead: (_count, value) => { if (fail && owner === 'parent') throw new Error('ID refresh disappeared'); return value; } });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.identify('load'), true);
      if (owner === 'identity') f.form.identityDraft(f.rowId, '2');
      else f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
      fail = true;
      assert.equal(await f.form.saveForClose(), false);
      const undo = owner === 'identity' ? () => f.form.identify('undo') : f.parent.undoSource;
      assert.equal(await undo(), false);
      fail = false;
      assert.equal(await undo(), true);
      assert.equal(f.form.getCloseState().blocked, false);
      assert.equal(f.calls.identitySave + f.calls.parentSave, 1);
      assert.equal(owner === 'identity' ? await f.form.identify('prune') : await f.parent.restoreSource(), true);
    } finally { f.dispose(); }
  }
});

test('identity source read cancellation is owned and a held Save cannot cancel or replay', async () => {
  const read = deferred(), save = deferred();
  const f = fixture({ environment: identityEnvironment,
    identityRead: (count, value) => count === 1 ? read.promise : value,
    cancel: () => read.reject(new Error('owned ID read cancelled')), identitySave: () => save.promise });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    const loading = f.form.identify('load');
    assert.equal(f.form.getCloseState().busy, true);
    f.form.cancelSource(); f.form.cancelCollected(); f.form.cancelSpecies();
    assert.equal(f.calls.cancelled, 0);
    f.form.cancelIdentity();
    assert.equal(await loading, false);
    assert.match(f.form.identitySnapshot().error, /owned ID read cancelled/);
    assert.equal(await f.form.identify('load'), true);
    f.form.identityDraft(f.rowId, '2');
    const saving = f.form.saveForClose();
    const cancelled = f.calls.cancelled;
    f.form.cancelIdentity();
    assert.equal(f.calls.cancelled, cancelled);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.identitySave, 1);
    save.resolve({ ChangedCells: 1, HistoryID: '501' });
    assert.equal(await saving, true);
    assert.equal(await f.form.identify('prune'), true);
  } finally { f.dispose(); }
});

test('late identity source cannot populate a remounted client and a clean retry uses current ownership', async () => {
  const read = deferred();
  const old = fixture({ environment: identityEnvironment, identityRead: () => read.promise });
  assert.equal(await old.parent.loadSource(), true); old.parent.unlock();
  const loading = old.form.identify('load');
  old.dispose();
  const next = fixture({ environment: identityEnvironment });
  try {
    assert.equal(await next.parent.loadSource(), true); next.parent.unlock();
    read.resolve(review({ extended: true }));
    assert.equal(await loading, false);
    assert.equal(next.form.identitySnapshot().review, null);
    assert.equal(next.calls.identityRead, 0);
    assert.equal(await next.form.identify('load'), true);
  } finally { next.dispose(); }
});

test('identity backend denial, unsupported source and unchanged NULL never coerce or invent IDs', async () => {
  for (const options of [
    { identityRead: () => { throw new Error('SIVI identity editing is disabled in this session'); } },
    { identityValue: cell('integer', '2147483648') }, { identityValue: cell('text', '1') },
    { identityValue: cell('real', 1) }, { identityValue: cell('blob', 'ff') }, { identityValue: cell('null') },
  ]) {
    const f = fixture({ ...options, environment: identityEnvironment });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      const loaded = await f.form.identify('load');
      if (options.identityRead) {
        assert.equal(loaded, false);
        assert.match(f.form.identitySnapshot().error, /disabled/);
      } else if (options.identityValue.storage === 'null') {
        f.form.identityDraft(f.rowId, '', true);
        assert.equal(f.form.getCloseState().unsaved, false);
      } else {
        assert.equal(loaded, true);
        f.form.identityDraft(f.rowId, '2');
        assert.equal(f.form.identitySnapshot().drafts[f.rowId].raw, '2');
        assert.deepEqual(f.form.identitySnapshot().drafts[f.rowId].expected, options.identityValue);
        assert.match(f.form.getCloseState().error, /Historical unsupported/);
        assert.equal(await f.form.identify('undo'), true);
      }
      assert.equal(f.calls.identitySave, 0);
    } finally { f.dispose(); }
  }
});

test('identity restoration refuses changed complete rows and rejects invented source audit pruning', async () => {
  for (const failure of ['changed-peer', 'audit-ack']) {
    const f = fixture({ environment: identityEnvironment,
      identityRestore: failure === 'audit-ack' ? () => ({ restoredRows: 1, prunedAuditRows: 1,
        cleanedVegRows: 0, cancelled: false }) : undefined });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.identify('load'), true);
      assert.equal(await f.form.perform('load'), true);
      f.form.identityDraft(f.rowId, '2'); assert.equal(await f.form.saveForClose(), true);
      if (failure === 'changed-peer') {
        f.form.draft(f.rowId, 'Cover5a', '3'); assert.equal(await f.form.saveForClose(), true);
      }
      assert.equal(await f.form.identify('prune'), false);
      assert.equal(f.form.getCloseState().blocked, true);
      assert.equal(f.form.identitySnapshot().historyId, null);
      assert.equal(await f.form.identify('prune'), false);
      assert.equal(f.calls.identityRestore, 1);
      assert.equal(f.calls.identitySave, 1);
    } finally { f.dispose(); }
  }
});

test('Species-only actual client preserves strict Go reference wire and explicit eligible extended source context', async () => {
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_COMBINED_EDITING: 'false' } });
  try {
    assert.equal(await f.parent.loadSource(), true);
    assert.equal(await f.form.select('load'), false);
    f.parent.unlock();
    assert.equal(await f.form.select('load'), true);
    assert.equal(f.form.speciesSnapshot().extended, true);
    assert.equal(f.form.speciesSnapshot().references.master.columns[0].declaredType, 'TEXT');
    assert.equal(f.form.speciesSnapshot().references.master.rows[0].rowId, '9223372036854775807');
    f.form.speciesDraft(f.rowId, 'MOSS');
    assert.equal(f.form.getCloseState().canSave, false);
    f.form.speciesContext(f.rowId, 2);
    assert.equal(f.form.getCloseState().canSave, true);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.speciesRequest.edits[0].form, 'SubVegD-SIVI');
    assert.equal(f.calls.speciesRequest.edits[0].value, 'MOSS');
    assert.equal(f.calls.speciesRequest.edits[0].expected.text, 'RAW');
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    assert.equal(await f.form.select('prune'), true);
    assert.equal(f.calls.speciesSave, 1);
    assert.equal(f.calls.childRead, 0);
    assert.equal(f.calls.collectedRead, 0);
  } finally { f.dispose(); }
});

for (const [raw, decision, selected, expected] of [
  ['old', 'keep', undefined, 'OLD'], ['old', 'replace', 'TREE', 'TREE'], ['MINE', 'user', 'MINE', 'MINE'],
]) {
  test(`extended Species ${decision} keeps existing reviewed source decisions, not new definition authority`, async () => {
    const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' } });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.select('load'), true);
      f.form.speciesDraft(f.rowId, raw);
      assert.equal(f.form.getCloseState().canSave, false);
      f.form.speciesChoose(f.rowId, decision, selected);
      assert.equal(await f.form.saveForClose(), true);
      assert.equal(f.calls.speciesRequest.edits[0].decision, decision);
      assert.equal(f.calls.speciesRequest.edits[0].value, expected);
      assert.equal(f.calls.speciesRequest.edits[0].entered, raw);
      assert.equal(await f.form.select('prune'), true);
      assert.equal(f.calls.speciesSave, 1);
    } finally { f.dispose(); }
  });
}

test('Species invalid drafts block all peers while valid correction and owning Undo remain live', async () => {
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_COLLECTED_EDITING: 'true' } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    assert.equal(await f.form.select('load'), true);
    for (const raw of ['TOO-LONG-CODE', '\ud800', 'NEW']) {
      f.form.speciesDraft(f.rowId, raw);
      assert.equal(f.form.speciesSnapshot().drafts[f.rowId].raw, raw);
      assert.ok(f.form.speciesSnapshot().drafts[f.rowId].error);
      assert.equal(f.form.getCloseState().unsaved, true);
      assert.equal(f.form.getCloseState().canSave, false);
      assert.equal(await f.form.saveForClose(), false);
      f.form.draft(f.rowId, 'Cover5a', '2');
      f.form.cycle(f.rowId);
      f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
      assert.deepEqual(f.form.snapshot().drafts.covers, {});
      assert.deepEqual(f.form.collectedSnapshot().drafts, {});
      assert.equal(f.parent.getCloseState().unsaved, false);
      assert.equal(await f.parent.undoSource(), false);
      assert.equal(await f.form.collect('load'), false);
      f.form.speciesDraft(f.rowId, 'TREE');
      assert.equal(f.form.speciesSnapshot().drafts[f.rowId].error, null);
      assert.equal(f.form.getCloseState().canSave, true);
      assert.equal(await f.form.select('undo'), true);
      assert.equal(f.form.getCloseState().unsaved, false);
    }
    assert.equal(f.calls.speciesSave + f.calls.childSave + f.calls.collectedSave + f.calls.parentSave, 0);
  } finally { f.dispose(); }
});

test('four client owners retain receipts and restore same-row history in legitimate reverse mutation order', async () => {
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_COLLECTED_EDITING: 'true' } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    assert.equal(await f.form.select('load'), true);
    f.form.speciesDraft(f.rowId, 'TREE');
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.form.snapshot().review[0].Rows[0].cells[2].text, 'TREE');
    assert.equal(f.form.collectedSnapshot().review[0].Rows[0].cells[2].text, 'TREE');
    f.form.cycle(f.rowId); assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    f.form.draft(f.rowId, 'Cover5a', '3'); assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.collectedSnapshot().historyId, '301');
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(await f.form.collect('prune'), true);
    assert.equal(await f.form.select('prune'), true);
    assert.equal(f.form.snapshot().review[0].Rows[0].cells[2].text, 'RAW');
    assert.equal(await f.parent.restoreSource(), true);
    assert.deepEqual([f.calls.speciesSave, f.calls.collectedSave, f.calls.childSave, f.calls.parentSave], [1, 1, 1, 1]);
    assert.deepEqual([f.calls.speciesRestore, f.calls.collectedRestore, f.calls.restore], [1, 1, 1]);
  } finally { f.dispose(); }
});

for (const owner of ['combined', 'collected', 'species']) {
  test(`${owner} acknowledged mutation retains recovery when a different child's source/reference refresh fails`, async () => {
    let fail = false;
    const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_COLLECTED_EDITING: 'true' },
      read: (_count, value) => { if (fail && owner === 'species') throw new Error('combined source disappeared'); return value; },
      speciesReferences: (_count, value) => { if (fail && owner !== 'species') throw new Error('Species definitions disappeared'); return value; },
    });
    const operate = owner === 'combined' ? kind => f.form.perform(kind)
      : owner === 'collected' ? kind => f.form.collect(kind) : kind => f.form.select(kind);
    const view = () => owner === 'combined' ? f.form.snapshot()
      : owner === 'collected' ? f.form.collectedSnapshot() : f.form.speciesSnapshot();
    const receipt = { combined: '201', collected: '301', species: '401' }[owner];
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.perform('load'), true);
      assert.equal(await f.form.collect('load'), true);
      assert.equal(await f.form.select('load'), true);
      if (owner === 'combined') f.form.draft(f.rowId, 'Cover5a', '3');
      else if (owner === 'collected') f.form.cycle(f.rowId);
      else f.form.speciesDraft(f.rowId, 'TREE');
      fail = true;
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(view().historyId, receipt);
      assert.equal(f.form.getCloseState().blocked, true);
      assert.equal(await f.form.saveForClose(), false);
      assert.equal(await operate('undo'), false);
      if (owner !== 'species') assert.equal(f.form.speciesSnapshot().references, null);
      fail = false;
      assert.equal(await operate('undo'), true);
      assert.equal(view().historyId, receipt);
      assert.ok(f.form.speciesSnapshot().references);
      assert.ok(f.form.snapshot().review);
      assert.equal(f.form.getCloseState().blocked, false);
      assert.equal(f.calls.speciesSave + f.calls.collectedSave + f.calls.childSave, 1);
      assert.equal(await operate('prune'), true);
    } finally { f.dispose(); }
  });
}

test('parent postcommit missing Species definitions retry preserves all previously loaded history owners', async () => {
  let fail = false;
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true', VITE_SIVI_COLLECTED_EDITING: 'true' },
    speciesReferences: (_count, value) => { if (fail) throw new Error('Species definitions reload failed'); return value; } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    assert.equal(await f.form.collect('load'), true);
    assert.equal(await f.form.select('load'), true);
    f.form.speciesDraft(f.rowId, 'TREE'); assert.equal(await f.form.saveForClose(), true);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.speciesSnapshot().references, null);
    assert.equal(await f.parent.undoSource(), false);
    fail = false;
    assert.equal(await f.parent.undoSource(), true);
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.select('prune'), true);
    assert.equal(await f.parent.restoreSource(), true);
    assert.equal(f.calls.speciesSave, 1);
    assert.equal(f.calls.parentSave, 1);
  } finally { f.dispose(); }
});

test('Species partial or foreign reference bundles never enable drafts and explicit retry corrects the error', async () => {
  for (const failure of ['original', 'references', 'foreign']) {
    const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' },
      speciesRead: (count, value) => failure === 'original' && count === 1 ? null : value,
      speciesReferences: (count, value) => count !== 1 ? value
        : failure === 'references' ? null : failure === 'foreign' ? { ...value, contextId: 'foreign owner' } : value });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      assert.equal(await f.form.select('load'), false);
      assert.equal(f.form.speciesSnapshot().review, null);
      assert.equal(f.form.speciesSnapshot().references, null);
      assert.ok(f.form.speciesSnapshot().error);
      f.form.speciesDraft(f.rowId, 'TREE');
      assert.equal(f.calls.speciesSave, 0);
      assert.equal(await f.form.select('load'), true);
      assert.ok(f.form.speciesSnapshot().references);
    } finally { f.dispose(); }
  }
});

test('Species source/reference read cancellation is owner-specific while mutations cannot cancel or replay', async () => {
  const references = deferred(), save = deferred();
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' },
    speciesReferences: (count, value) => count === 1 ? references.promise : value,
    cancel: () => references.reject(new Error('owned Species reference read cancelled')),
    speciesSave: () => save.promise });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    const loading = f.form.select('load');
    assert.equal(f.form.getCloseState().busy, true);
    f.form.cancelSource(); f.form.cancelCollected();
    assert.equal(f.calls.cancelled, 0);
    f.form.cancelSpecies();
    assert.equal(await loading, false);
    assert.match(f.form.speciesSnapshot().error, /owned Species reference read cancelled/);
    assert.equal(await f.form.select('load'), true);
    f.form.speciesDraft(f.rowId, 'TREE');
    const saving = f.form.saveForClose();
    const count = f.calls.cancelled;
    f.form.cancelSpecies();
    assert.equal(f.calls.cancelled, count);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.speciesSave, 1);
    save.resolve({ ChangedCells: 1, HistoryID: '401' });
    assert.equal(await saving, true);
    assert.equal(await f.form.select('prune'), true);
  } finally { f.dispose(); }
});

test('late Species source/reference delivery cannot populate a replacement actual client owner', async () => {
  const references = deferred();
  const old = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' }, speciesReferences: () => references.promise });
  assert.equal(await old.parent.loadSource(), true); old.parent.unlock();
  const loading = old.form.select('load');
  old.dispose();
  const next = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' } });
  try {
    assert.equal(await next.parent.loadSource(), true); next.parent.unlock();
    references.resolve({ ...clone(speciesWire), contextId: wire.Original.ContextID, project: wire.Original.Project, plot: wire.Original.Plot });
    assert.equal(await loading, false);
    assert.equal(next.form.speciesSnapshot().references, null);
    assert.equal(next.calls.speciesRead, 0);
    assert.equal(await next.form.select('load'), true);
  } finally { next.dispose(); }
});

test('Species independent backend denial and historical nontext source refuse without coercion or writes', async () => {
  for (const options of [
    { speciesRead: () => { throw new Error('SIVI Species editing is disabled in this session'); } },
    { speciesValue: cell('integer', '123') },
  ]) {
    const f = fixture({ ...options, environment: { VITE_SIVI_SPECIES_EDITING: 'true' } });
    try {
      assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
      const loaded = await f.form.select('load');
      if (options.speciesRead) {
        assert.equal(loaded, false);
        assert.match(f.form.speciesSnapshot().error, /disabled/);
      } else {
        assert.equal(loaded, true);
        f.form.speciesDraft(f.rowId, 'TREE');
        assert.match(f.form.getCloseState().error, /Historical non-text/);
        assert.equal(f.form.speciesSnapshot().drafts[f.rowId].raw, 'TREE');
        assert.deepEqual(f.form.speciesSnapshot().drafts[f.rowId].expected, cell('integer', '123'));
        assert.equal(f.form.getCloseState().canSave, false);
        assert.equal(await f.form.select('undo'), true);
        assert.deepEqual(f.form.speciesSnapshot().drafts, {});
      }
      assert.equal(f.calls.speciesSave, 0);
    } finally { f.dispose(); }
  }
});

test('Species postcommit parent-read failure retains its reference owner and reachable recovery without originals', async () => {
  let fail = false;
  const f = fixture({ environment: { VITE_SIVI_SPECIES_EDITING: 'true' },
    parentRead: (_count, value) => { if (fail) throw new Error('owned CHARS parent read failed'); return value; } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.select('load'), true);
    f.form.speciesDraft(f.rowId, 'TREE');
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.parent.snapshot().original, null);
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.parent.undoSource(), false);
    assert.equal(await f.form.select('undo'), false);
    fail = false;
    assert.equal(await f.form.select('undo'), true);
    assert.equal(f.form.speciesSnapshot().historyId, '401');
    assert.ok(f.form.speciesSnapshot().references);
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.calls.speciesSave, 1);
    assert.equal(await f.form.select('prune'), true);
  } finally { f.dispose(); }
});
