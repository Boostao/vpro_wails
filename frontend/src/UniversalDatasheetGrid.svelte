<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte';
  import { ReadRequests } from './readRequests';
  import {
    CANONICAL_DATASHEET_KEYS,
    cellsSemanticallyEqual,
    computeNextGridCell,
    computeVirtualWindow,
    defaultDatasheetBackendPort,
    formatCellForEditor,
    formatTSVRange,
    normalizeGridRange,
    parseAndValidateCellInput,
    parseTSVClipboard,
    toggleMultiColumnSort,
    type DatasheetBackendPort,
    type DatasheetCellEdit,
    type DatasheetColumnMeta,
    type DatasheetFilterSpec,
    type DatasheetFindReplacePreview,
    type DatasheetPage,
    type DatasheetSortSpec,
    type DatasheetTableInfo,
    type DatasheetTableKey,
    type GridSelectionRange,
  } from './universalDatasheetGrid';

  interface Props {
    contextId: string;
    project?: string;
    su?: string;
    hierarchy?: string;
    initialTableKey?: DatasheetTableKey;
    port?: DatasheetBackendPort;
    onBusyChange?: (busy: boolean) => void;
    onDirtyChange?: (dirty: boolean) => void;
    onOpenPlot?: (plotNumber: string) => void;
  }

  let {
    contextId,
    project = '',
    su = 'None',
    hierarchy = 'None',
    initialTableKey = 'Env',
    port = defaultDatasheetBackendPort,
    onBusyChange,
    onDirtyChange,
    onOpenPlot,
  }: Props = $props();

  const reads = new ReadRequests();
  const ROW_HEIGHT = 34;

  let tables = $state.raw<DatasheetTableInfo[]>([]);
  let activeTableKey = $state<DatasheetTableKey>(untrack(() => initialTableKey));
  let page = $state.raw<DatasheetPage | null>(null);
  let pageSize = $state(250);
  let offset = $state(0);
  let sortSpecs = $state<DatasheetSortSpec[]>([]);
  let columnFilters = $state<Record<string, string>>({});
  let globalSearch = $state('');
  let scopeToActiveSU = $state(false);

  // Keyed by `${rowId}::${colName}`
  interface CellDraft {
    rowId: string;
    column: string;
    rawText: string;
    before: DatasheetCellEdit['before'];
    after: DatasheetCellEdit['after'] | null;
    error: string;
  }
  let drafts = $state<Record<string, CellDraft>>({});

  let activeCell = $state<{ row: number; col: number } | null>(null);
  let selectionRange = $state<GridSelectionRange | null>(null);

  let scrollTop = $state(0);
  let viewportHeight = $state(520);
  let viewportEl = $state<HTMLDivElement | null>(null);

  let busy = $state(false);
  let saving = $state(false);
  let errorMessage = $state('');
  let statusBanner = $state('');

  // Find & Replace state
  let findDrawerOpen = $state(false);
  let findColumn = $state('');
  let findText = $state('');
  let replaceText = $state('');
  let findMatchCase = $state(false);
  let findExactCell = $state(false);
  let findPreview = $state.raw<DatasheetFindReplacePreview | null>(null);
  let findBusy = $state(false);

  let generation = 0;

  // Compute display columns with pinned column placed first
  const orderedColumns = $derived.by(() => {
    if (!page) return [] as Array<{ meta: DatasheetColumnMeta; originalIndex: number }>;
    const pinned: Array<{ meta: DatasheetColumnMeta; originalIndex: number }> = [];
    const rest: Array<{ meta: DatasheetColumnMeta; originalIndex: number }> = [];
    page.columns.forEach((meta, idx) => {
      if (meta.pinned) pinned.push({ meta, originalIndex: idx });
      else rest.push({ meta, originalIndex: idx });
    });
    return [...pinned, ...rest];
  });

  const virtualSlice = $derived(
    computeVirtualWindow(scrollTop, viewportHeight, ROW_HEIGHT, page?.rows.length ?? 0, 8)
  );

  const visibleRows = $derived(
    page ? page.rows.slice(virtualSlice.startIndex, virtualSlice.endIndex) : []
  );

  const draftEntries = $derived(Object.values(drafts));
  const invalidDraftCount = $derived(draftEntries.filter(d => d.error !== '').length);
  const validEditCount = $derived(draftEntries.filter(d => d.error === '' && d.after !== null).length);

  function notifyHost() {
    onBusyChange?.(busy || saving || findBusy || draftEntries.length > 0);
    onDirtyChange?.(!busy && !saving && !findBusy && draftEntries.length > 0);
  }

  $effect(() => {
    void busy;
    void saving;
    void findBusy;
    void invalidDraftCount;
    void draftEntries.length;
    notifyHost();
  });

  function draftKey(rowId: string, column: string): string {
    return `${rowId}::${column}`;
  }

  function buildActiveFilterSpecs(): DatasheetFilterSpec[] {
    const out: DatasheetFilterSpec[] = [];
    for (const [col, val] of Object.entries(columnFilters)) {
      const trimmed = val.trim();
      if (!trimmed) continue;
      if (trimmed === '=NULL') {
        out.push({ column: col, op: 'is_null', value: '' });
      } else if (trimmed === '!=NULL') {
        out.push({ column: col, op: 'not_null', value: '' });
      } else if (trimmed.startsWith('=')) {
        out.push({ column: col, op: 'eq', value: trimmed.slice(1) });
      } else {
        out.push({ column: col, op: 'contains', value: trimmed });
      }
    }
    return out;
  }

  async function refreshTablesAndLoad(targetKey: DatasheetTableKey = activeTableKey, nextOffset = 0) {
    if (!contextId) return;
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const listed = await reads.track(port.listTables(contextId));
      if (reqGen !== generation) return;
      tables = listed ?? [];

      const chosen = tables.find(t => t.key === targetKey);
      if (chosen && !chosen.available) {
        page = null;
        statusBanner = chosen.unavailableReason ?? `${chosen.key} is not available in this context.`;
        return;
      }

      const reqPayload = {
        tableKey: targetKey,
        offset: nextOffset,
        limit: pageSize,
        sort: $state.snapshot(sortSpecs),
        filters: buildActiveFilterSpecs(),
        globalSearch,
        scopeToActiveSU,
      };
      const loaded = await reads.track(port.getPage(contextId, reqPayload));
      if (reqGen !== generation) return;
      page = loaded;
      offset = loaded.offset;
      if (!findColumn && loaded.columns.length > 0) {
        const firstEditable = loaded.columns.find(c => c.editable && !c.pinned) ?? loaded.columns[0];
        findColumn = firstEditable.name;
      }
      statusBanner = '';
    } catch (err) {
      if (reqGen === generation) {
        errorMessage = String(err);
      }
    } finally {
      if (reqGen === generation) {
        busy = false;
      }
    }
  }

  async function loadPageOnly(nextOffset = offset) {
    if (!contextId) return;
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const reqPayload = {
        tableKey: activeTableKey,
        offset: Math.max(0, nextOffset),
        limit: pageSize,
        sort: $state.snapshot(sortSpecs),
        filters: buildActiveFilterSpecs(),
        globalSearch,
        scopeToActiveSU,
      };
      const loaded = await reads.track(port.getPage(contextId, reqPayload));
      if (reqGen !== generation) return;
      page = loaded;
      offset = loaded.offset;
    } catch (err) {
      if (reqGen === generation) {
        errorMessage = String(err);
      }
    } finally {
      if (reqGen === generation) {
        busy = false;
      }
    }
  }

  function selectTable(key: DatasheetTableKey) {
    if (draftEntries.length > 0) {
      errorMessage = 'Save or discard unsaved cell edits before switching datasheet tables.';
      return;
    }
    activeTableKey = key;
    offset = 0;
    sortSpecs = [];
    columnFilters = {};
    globalSearch = '';
    activeCell = null;
    selectionRange = null;
    findPreview = null;
    findColumn = '';
    scrollTop = 0;
    void refreshTablesAndLoad(key, 0);
  }

  function setCellDraft(rowIndex: number, displayColIndex: number, rawValue: string) {
    if (!page) return;
    const row = page.rows[rowIndex];
    const colEntry = orderedColumns[displayColIndex];
    if (!row || !colEntry) return;
    const colMeta = colEntry.meta;
    const originalCell = row.cells[colEntry.originalIndex];
    const key = draftKey(row.rowId, colMeta.name);

    const parsed = parseAndValidateCellInput(page.tableKey, colMeta, rawValue, page.canEditMaster);
    if (!parsed.error && parsed.cell && cellsSemanticallyEqual(originalCell, parsed.cell, colMeta)) {
      const next = { ...drafts };
      delete next[key];
      drafts = next;
      return;
    }

    drafts = {
      ...drafts,
      [key]: {
        rowId: row.rowId,
        column: colMeta.name,
        rawText: rawValue,
        before: structuredClone($state.snapshot(originalCell)),
        after: parsed.cell,
        error: parsed.error,
      },
    };
  }

  function discardDrafts() {
    drafts = {};
    errorMessage = '';
    statusBanner = 'Discarded unsaved datasheet edits.';
  }

  async function saveDrafts() {
    if (!page || saving || validEditCount === 0) return;
    if (invalidDraftCount > 0) {
      errorMessage = `Resolve ${invalidDraftCount} invalid cell(s) before saving.`;
      return;
    }
    saving = true;
    errorMessage = '';
    try {
      const edits: DatasheetCellEdit[] = [];
      for (const d of draftEntries) {
        if (d.after) {
          edits.push({
            rowId: d.rowId,
            column: d.column,
            before: $state.snapshot(d.before),
            after: $state.snapshot(d.after),
          });
        }
      }
      const res = await port.applyEdits(contextId, {
        tableKey: page.tableKey,
        edits,
      });
      drafts = {};
      statusBanner = `Saved ${res.updatedCells} cell(s) across ${res.updatedRows} row(s) (${res.auditRows} audit record(s)).`;
      await refreshTablesAndLoad(activeTableKey, offset);
    } catch (err) {
      errorMessage = `Save failed (transaction rolled back): ${String(err)}`;
    } finally {
      saving = false;
    }
  }

  function focusGridInput(rowIdx: number, colIdx: number) {
    activeCell = { row: rowIdx, col: colIdx };
    selectionRange = { startRow: rowIdx, endRow: rowIdx, startCol: colIdx, endCol: colIdx };
    if (viewportEl) {
      const topTarget = rowIdx * ROW_HEIGHT;
      const bottomTarget = topTarget + ROW_HEIGHT;
      if (topTarget < viewportEl.scrollTop) {
        viewportEl.scrollTop = topTarget;
      } else if (bottomTarget > viewportEl.scrollTop + viewportEl.clientHeight - 40) {
        viewportEl.scrollTop = bottomTarget - viewportEl.clientHeight + 48;
      }
    }
    queueMicrotask(() => {
      const el = viewportEl?.querySelector<HTMLInputElement>(
        `input[data-grid-row="${rowIdx}"][data-grid-col="${colIdx}"]`
      );
      el?.focus();
      el?.select();
    });
  }

  function handleCellKeyDown(event: KeyboardEvent, rowIdx: number, colIdx: number) {
    if (!page) return;
    if (event.key === 'Enter' || event.key === 'Tab') {
      const next = computeNextGridCell(
        { row: rowIdx, col: colIdx },
        event.key,
        event.shiftKey,
        page.rows.length,
        orderedColumns.length
      );
      if (next) {
        event.preventDefault();
        focusGridInput(next.row, next.col);
      }
      return;
    }
    if (event.key === 'Escape') {
      const row = page.rows[rowIdx];
      const col = orderedColumns[colIdx];
      if (row && col) {
        const key = draftKey(row.rowId, col.meta.name);
        if (key in drafts) {
          event.preventDefault();
          const copy = { ...drafts };
          delete copy[key];
          drafts = copy;
        }
      }
    }
  }

  function handleCopySelection(event: ClipboardEvent) {
    if (!page || !selectionRange) return;
    const norm = normalizeGridRange(selectionRange);
    // Single-cell native input selection copies normally unless multi-cell range is selected
    if (norm.startRow === norm.endRow && norm.startCol === norm.endCol) return;
    const projectedRows = page.rows.map(r => ({
      rowId: r.rowId,
      cells: orderedColumns.map(c => r.cells[c.originalIndex]),
    }));
    const cols = orderedColumns.map(c => c.meta);
    const tsv = formatTSVRange(projectedRows, cols, norm, (rowId, colName) => {
      const d = drafts[draftKey(rowId, colName)];
      return d?.rawText;
    });
    if (event.clipboardData) {
      event.preventDefault();
      event.clipboardData.setData('text/plain', tsv);
      statusBanner = `Copied ${norm.endRow - norm.startRow + 1}x${norm.endCol - norm.startCol + 1} TSV block to clipboard.`;
    }
  }

  function handlePasteIntoCell(event: ClipboardEvent, anchorRow: number, anchorCol: number) {
    if (!page || !event.clipboardData) return;
    const text = event.clipboardData.getData('text/plain');
    if (!text.includes('\t') && !text.includes('\n')) {
      return; // single value paste uses standard input handling
    }
    const matrix = parseTSVClipboard(text);
    if (matrix.length === 0) return;
    event.preventDefault();

    let applied = 0;
    for (let rOff = 0; rOff < matrix.length; rOff++) {
      const targetRow = anchorRow + rOff;
      if (targetRow >= page.rows.length) break;
      const line = matrix[rOff];
      for (let cOff = 0; cOff < line.length; cOff++) {
        const targetCol = anchorCol + cOff;
        if (targetCol >= orderedColumns.length) break;
        if (!orderedColumns[targetCol].meta.editable) continue;
        setCellDraft(targetRow, targetCol, line[cOff]);
        applied++;
      }
    }
    selectionRange = {
      startRow: anchorRow,
      endRow: Math.min(page.rows.length - 1, anchorRow + matrix.length - 1),
      startCol: anchorCol,
      endCol: Math.min(orderedColumns.length - 1, anchorCol + (matrix[0]?.length ?? 1) - 1),
    };
    statusBanner = `Pasted ${applied} cell(s) from TSV clipboard. Review and click Save Edits to commit.`;
  }

  function handleHeaderSort(colName: string, event: MouseEvent) {
    sortSpecs = toggleMultiColumnSort(sortSpecs, colName, event.shiftKey);
    offset = 0;
    void loadPageOnly(0);
  }

  async function runFindPreview() {
    if (!page || !findColumn) return;
    findBusy = true;
    errorMessage = '';
    try {
      const preview = await reads.track(
        port.previewFindReplace(contextId, {
          tableKey: page.tableKey,
          column: findColumn,
          findText,
          replaceText,
          matchCase: findMatchCase,
          exactCell: findExactCell,
          filters: buildActiveFilterSpecs(),
          globalSearch,
          scopeToActiveSU,
          maxPreview: 500,
        })
      );
      findPreview = preview;
    } catch (err) {
      errorMessage = `Find/Replace preview failed: ${String(err)}`;
    } finally {
      findBusy = false;
    }
  }

  async function applyFindReplaceValid() {
    if (!page || !findPreview || findPreview.validCount === 0) return;
    const validEdits = findPreview.items
      .filter(i => !i.validationError)
      .map(i => $state.snapshot(i.edit));
    if (validEdits.length === 0) return;
    saving = true;
    errorMessage = '';
    try {
      const res = await port.applyEdits(contextId, {
        tableKey: page.tableKey,
        edits: validEdits,
      });
      findPreview = null;
      statusBanner = `Applied ${res.updatedCells} replacement(s) on ${page.tableName} (${res.auditRows} audit record(s)).`;
      await refreshTablesAndLoad(activeTableKey, offset);
    } catch (err) {
      errorMessage = `Find/Replace commit failed (rolled back): ${String(err)}`;
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    if (contextId) {
      void refreshTablesAndLoad(activeTableKey, 0);
    }
  });

  onDestroy(() => {
    generation++;
    reads.cancelAll();
    onDirtyChange?.(false);
    onBusyChange?.(false);
  });
</script>

<section
  class="vpro-datasheet-workbench"
  aria-label="Universal Project Datasheet Grid"
  oncopy={handleCopySelection}
>
  <header class="vpro-ds-header">
    <div>
      <p class="vpro-ds-eyebrow">DIRECT DATASHEET WORKBENCH</p>
      <h2 class="vpro-ds-title">
        Universal Project Datasheet
        {#if project}<span class="vpro-ds-context-pill">Project: {project}</span>{/if}
        {#if su && su !== 'None'}<span class="vpro-ds-context-pill">SU: {su}</span>{/if}
        {#if hierarchy && hierarchy !== 'None'}<span class="vpro-ds-context-pill">Hierarchy: {hierarchy}</span>{/if}
      </h2>
    </div>

    <div class="vpro-ds-toolbar-actions">
      <label class="vpro-ds-inline-check">
        <input
          type="checkbox"
          bind:checked={scopeToActiveSU}
          disabled={!su || su === 'None' || busy || saving}
          onchange={() => loadPageOnly(0)}
        />
        <span>Scope to Active SU</span>
      </label>

      <input
        type="search"
        class="vpro-ds-search"
        placeholder="Search all columns..."
        bind:value={globalSearch}
        onkeydown={e => {
          if (e.key === 'Enter') void loadPageOnly(0);
        }}
      />
      <button type="button" class="vpro-btn-secondary" disabled={busy} onclick={() => loadPageOnly(0)}>
        Apply Filter
      </button>
      <button
        type="button"
        class="vpro-btn-secondary"
        disabled={!page}
        onclick={() => {
          findDrawerOpen = !findDrawerOpen;
        }}
      >
        {findDrawerOpen ? 'Hide Find & Replace' : 'Find & Replace'}
      </button>
      <button
        type="button"
        class="vpro-btn-secondary"
        disabled={draftEntries.length === 0 || saving}
        onclick={discardDrafts}
      >
        Discard ({draftEntries.length})
      </button>
      <button
        type="button"
        class="vpro-btn-primary"
        disabled={validEditCount === 0 || invalidDraftCount > 0 || saving}
        onclick={saveDrafts}
      >
        {saving ? 'Saving...' : `Save Edits (${validEditCount})`}
      </button>
    </div>
  </header>

  <nav class="vpro-ds-tabs" aria-label="Canonical Project Tables">
    {#each CANONICAL_DATASHEET_KEYS as key (key)}
      {@const info = tables.find(t => t.key === key)}
      <button
        type="button"
        class="vpro-ds-tab"
        class:active={activeTableKey === key}
        disabled={saving}
        onclick={() => selectTable(key)}
      >
        <span>_{key}</span>
        {#if info}
          <span class="vpro-ds-badge" class:unavailable={!info.available}>
            {info.available ? info.rowCount.toLocaleString() : 'N/A'}
          </span>
        {/if}
      </button>
    {/each}
  </nav>

  {#if errorMessage}
    <p role="alert" class="vpro-ds-alert">{errorMessage}</p>
  {/if}
  {#if statusBanner}
    <p role="status" class="vpro-ds-status">{statusBanner}</p>
  {/if}

  {#if findDrawerOpen && page}
    <section class="vpro-ds-find-drawer" aria-label="Column Find and Replace">
      <div class="vpro-ds-find-controls">
        <label>
          <span>Column</span>
          <select bind:value={findColumn}>
            {#each page.columns.filter(c => c.editable) as col (col.name)}
              <option value={col.name}>{col.name} ({col.kind})</option>
            {/each}
          </select>
        </label>
        <label>
          <span>Find</span>
          <input type="text" bind:value={findText} placeholder="Text to find..." />
        </label>
        <label>
          <span>Replace With</span>
          <input type="text" bind:value={replaceText} placeholder="Replacement (blank = NULL)" />
        </label>
        <label class="vpro-ds-inline-check">
          <input type="checkbox" bind:checked={findMatchCase} />
          <span>Match case</span>
        </label>
        <label class="vpro-ds-inline-check">
          <input type="checkbox" bind:checked={findExactCell} />
          <span>Whole cell</span>
        </label>
        <button type="button" class="vpro-btn-secondary" disabled={findBusy} onclick={runFindPreview}>
          {findBusy ? 'Previewing...' : 'Dry-Run Preview'}
        </button>
        {#if findPreview && findPreview.validCount > 0}
          <button
            type="button"
            class="vpro-btn-primary"
            disabled={saving}
            onclick={applyFindReplaceValid}
          >
            Commit {findPreview.validCount} Valid Replacement(s)
          </button>
        {/if}
      </div>

      {#if findPreview}
        <div class="vpro-ds-find-summary">
          <p>
            Matched <strong>{findPreview.matchedRows}</strong> row(s) in
            <code>{findPreview.tableName}.{findPreview.column}</code> —
            <strong>{findPreview.validCount}</strong> valid,
            <strong>{findPreview.invalidCount}</strong> blocked by validation.
          </p>
          {#if findPreview.items.length > 0}
            <div class="vpro-ds-find-preview-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{page.pinnedColumn}</th>
                    <th>Before</th>
                    <th>After</th>
                    <th>Validation</th>
                  </tr>
                </thead>
                <tbody>
                  {#each findPreview.items.slice(0, 50) as item (item.rowId)}
                    <tr>
                      <td class="mono">{item.pinnedIdentity}</td>
                      <td class="mono">{item.beforeText || '(NULL)'}</td>
                      <td class="mono">{item.afterText || '(NULL)'}</td>
                      <td class:text-err={!!item.validationError}>
                        {item.validationError || 'Ready'}
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      {/if}
    </section>
  {/if}

  {#if page}
    <div
      class="vpro-ds-viewport"
      bind:this={viewportEl}
      bind:clientHeight={viewportHeight}
      onscroll={e => {
        scrollTop = e.currentTarget.scrollTop;
      }}
    >
      <table class="vpro-ds-grid" role="grid" aria-rowcount={page.totalRows}>
        <thead>
          <tr class="vpro-ds-header-row">
            {#each orderedColumns as colEntry (colEntry.meta.name)}
              {@const sortIdx = sortSpecs.findIndex(
                s => s.column.toLowerCase() === colEntry.meta.name.toLowerCase()
              )}
              <th
                class:pinned={colEntry.meta.pinned}
                class:protected={colEntry.meta.protected}
                scope="col"
              >
                <button
                  type="button"
                  class="vpro-ds-sort-btn"
                  title="Click to sort; Shift+Click for multi-column sort"
                  onclick={e => handleHeaderSort(colEntry.meta.name, e)}
                >
                  <span>{colEntry.meta.name}</span>
                  {#if sortIdx >= 0}
                    <span class="vpro-ds-sort-indicator">
                      {sortSpecs[sortIdx].direction === 'asc' ? 'ASC' : 'DESC'}{sortSpecs.length > 1
                        ? ` #${sortIdx + 1}`
                        : ''}
                    </span>
                  {/if}
                </button>
              </th>
            {/each}
          </tr>
          <tr class="vpro-ds-filter-row">
            {#each orderedColumns as colEntry (colEntry.meta.name)}
              <th class:pinned={colEntry.meta.pinned}>
                <input
                  type="text"
                  class="vpro-ds-col-filter"
                  aria-label={`Filter ${colEntry.meta.name}`}
                  placeholder={colEntry.meta.pinned ? '=00337 or text' : 'Filter...'}
                  bind:value={columnFilters[colEntry.meta.name]}
                  onkeydown={e => {
                    if (e.key === 'Enter') void loadPageOnly(0);
                  }}
                />
              </th>
            {/each}
          </tr>
        </thead>

        <tbody>
          {#if virtualSlice.topSpacerPx > 0}
            <tr aria-hidden="true" style={`height:${virtualSlice.topSpacerPx}px;`}>
              <td colspan={orderedColumns.length} style="padding:0;border:0;"></td>
            </tr>
          {/if}

          {#each visibleRows as row, localIdx (row.rowId)}
            {@const rowIdx = virtualSlice.startIndex + localIdx}
            <tr class="vpro-ds-row" style={`height:${ROW_HEIGHT}px;`}>
              {#each orderedColumns as colEntry, colIdx (colEntry.meta.name)}
                {@const colMeta = colEntry.meta}
                {@const cell = row.cells[colEntry.originalIndex]}
                {@const key = draftKey(row.rowId, colMeta.name)}
                {@const draft = drafts[key]}
                {@const displayVal = draft ? draft.rawText : formatCellForEditor(cell, colMeta)}
                <td
                  class:pinned={colMeta.pinned}
                  class:dirty={!!draft && !draft.error}
                  class:invalid={!!draft?.error}
                  class:readonly={!colMeta.editable}
                >
                  <div class="vpro-ds-cell-wrap">
                    <input
                      type="text"
                      class="vpro-ds-cell-input"
                      class:mono={colMeta.pinned || colMeta.kind !== 'text'}
                      data-grid-row={rowIdx}
                      data-grid-col={colIdx}
                      value={displayVal}
                      readonly={!colMeta.editable}
                      aria-invalid={draft?.error ? 'true' : undefined}
                      title={draft?.error || `${colMeta.name} (${colMeta.declaredType || colMeta.kind})`}
                      onfocus={() => {
                        activeCell = { row: rowIdx, col: colIdx };
                        if (!selectionRange) {
                          selectionRange = {
                            startRow: rowIdx,
                            endRow: rowIdx,
                            startCol: colIdx,
                            endCol: colIdx,
                          };
                        }
                      }}
                      onmousedown={e => {
                        if (e.shiftKey && activeCell) {
                          selectionRange = {
                            startRow: activeCell.row,
                            endRow: rowIdx,
                            startCol: activeCell.col,
                            endCol: colIdx,
                          };
                        } else {
                          selectionRange = {
                            startRow: rowIdx,
                            endRow: rowIdx,
                            startCol: colIdx,
                            endCol: colIdx,
                          };
                        }
                      }}
                      oninput={e => setCellDraft(rowIdx, colIdx, e.currentTarget.value)}
                      onkeydown={e => handleCellKeyDown(e, rowIdx, colIdx)}
                      onpaste={e => handlePasteIntoCell(e, rowIdx, colIdx)}
                    />
                    {#if colMeta.pinned && onOpenPlot && (page.tableKey === 'Env' || page.tableKey === 'Admin') && displayVal}
                      <button
                        type="button"
                        class="vpro-ds-open-plot"
                        title={`Open Plot ${displayVal} in Form`}
                        onclick={() => onOpenPlot(displayVal)}
                      >
                        Form
                      </button>
                    {/if}
                  </div>
                </td>
              {/each}
            </tr>
          {/each}

          {#if virtualSlice.bottomSpacerPx > 0}
            <tr aria-hidden="true" style={`height:${virtualSlice.bottomSpacerPx}px;`}>
              <td colspan={orderedColumns.length} style="padding:0;border:0;"></td>
            </tr>
          {/if}

          {#if page.rows.length === 0}
            <tr>
              <td colspan={orderedColumns.length} class="vpro-ds-empty">
                No rows matched the current filter criteria.
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <footer class="vpro-ds-footer">
      <div>
        Showing
        <strong>{page.rows.length ? page.offset + 1 : 0}</strong>–<strong
          >{page.offset + page.rows.length}</strong
        >
        of <strong>{page.totalRows.toLocaleString()}</strong> rows in
        <code>{page.tableName}</code>
        {#if invalidDraftCount > 0}
          <span class="vpro-ds-err-pill">{invalidDraftCount} invalid cell(s)</span>
        {/if}
      </div>

      <div class="vpro-ds-pager">
        <label>
          <span>Window</span>
          <select
            bind:value={pageSize}
            disabled={busy || draftEntries.length > 0}
            onchange={() => loadPageOnly(0)}
          >
            <option value={100}>100 rows</option>
            <option value={250}>250 rows</option>
            <option value={500}>500 rows</option>
            <option value={1000}>1,000 rows</option>
          </select>
        </label>
        <button
          type="button"
          class="vpro-btn-secondary"
          disabled={busy || offset === 0 || draftEntries.length > 0}
          onclick={() => loadPageOnly(Math.max(0, offset - pageSize))}
        >
          Previous
        </button>
        <button
          type="button"
          class="vpro-btn-secondary"
          disabled={busy || offset + page.rows.length >= page.totalRows || draftEntries.length > 0}
          onclick={() => loadPageOnly(offset + pageSize)}
        >
          Next
        </button>
      </div>
    </footer>
  {/if}
</section>

<style>
  .vpro-datasheet-workbench {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-top: 4px solid #003366;
    border-radius: 4px;
    padding: 1rem;
    min-width: 0;
  }
  .vpro-ds-header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: flex-end;
    gap: 0.75rem;
  }
  .vpro-ds-eyebrow {
    margin: 0;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #003366;
  }
  .vpro-ds-title {
    margin: 0.15rem 0 0;
    font-size: 1.2rem;
    font-weight: 600;
    color: #003366;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }
  .vpro-ds-context-pill {
    font-size: 0.75rem;
    font-weight: 500;
    background: #f0f4f8;
    border: 1px solid #cbd5e1;
    color: #003366;
    padding: 0.15rem 0.5rem;
    border-radius: 3px;
  }
  .vpro-ds-toolbar-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }
  .vpro-ds-inline-check {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.82rem;
    color: #1e293b;
  }
  .vpro-ds-search {
    min-height: 34px;
    padding: 0.3rem 0.6rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.85rem;
  }
  .vpro-btn-primary {
    min-height: 34px;
    padding: 0.35rem 0.8rem;
    background: #003366;
    color: #ffffff;
    border: 1px solid #003366;
    border-bottom: 2px solid #fcba19;
    border-radius: 4px;
    font-size: 0.82rem;
    font-weight: 600;
  }
  .vpro-btn-primary:disabled {
    opacity: 0.5;
  }
  .vpro-btn-secondary {
    min-height: 34px;
    padding: 0.35rem 0.7rem;
    background: #f8fafc;
    color: #003366;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.82rem;
    font-weight: 500;
  }
  .vpro-ds-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    border-bottom: 2px solid #e2e8f0;
    padding-bottom: 0.35rem;
  }
  .vpro-ds-tab {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.4rem 0.7rem;
    border: 1px solid #cbd5e1;
    border-radius: 4px 4px 0 0;
    background: #f8fafc;
    color: #334155;
    font-size: 0.82rem;
    font-weight: 600;
  }
  .vpro-ds-tab.active {
    background: #003366;
    color: #ffffff;
    border-color: #003366;
    box-shadow: inset 0 -3px 0 #fcba19;
  }
  .vpro-ds-badge {
    font-size: 0.7rem;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.2);
  }
  .vpro-ds-tab:not(.active) .vpro-ds-badge {
    background: #e2e8f0;
    color: #334155;
  }
  .vpro-ds-badge.unavailable {
    opacity: 0.65;
  }
  .vpro-ds-alert {
    margin: 0;
    padding: 0.55rem 0.75rem;
    background: #fef2f2;
    border-left: 4px solid #b91c1c;
    color: #991b1b;
    font-size: 0.85rem;
  }
  .vpro-ds-status {
    margin: 0;
    padding: 0.5rem 0.75rem;
    background: #f0f4f8;
    border-left: 4px solid #fcba19;
    color: #003366;
    font-size: 0.83rem;
  }
  .vpro-ds-find-drawer {
    background: #f8fafc;
    border: 1px solid #cbd5e1;
    border-left: 4px solid #003366;
    padding: 0.75rem;
    border-radius: 4px;
    display: grid;
    gap: 0.6rem;
  }
  .vpro-ds-find-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.6rem;
  }
  .vpro-ds-find-controls label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.78rem;
    font-weight: 600;
    color: #003366;
  }
  .vpro-ds-find-controls input[type='text'],
  .vpro-ds-find-controls select {
    min-height: 32px;
    padding: 0.25rem 0.5rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    font-size: 0.82rem;
  }
  .vpro-ds-find-preview-scroll {
    max-height: 180px;
    overflow: auto;
    border: 1px solid #cbd5e1;
    background: #fff;
  }
  .vpro-ds-viewport {
    height: 520px;
    overflow: auto;
    border: 1px solid #cbd5e1;
    background: #ffffff;
    position: relative;
  }
  .vpro-ds-grid {
    border-collapse: separate;
    border-spacing: 0;
    width: max-content;
    min-width: 100%;
    font-size: 0.82rem;
  }
  .vpro-ds-grid thead {
    position: sticky;
    top: 0;
    z-index: 3;
  }
  .vpro-ds-header-row th {
    background: #003366;
    color: #ffffff;
    padding: 0;
    border-right: 1px solid #1e4976;
    border-bottom: 2px solid #fcba19;
    min-width: 118px;
  }
  .vpro-ds-filter-row th {
    background: #f1f5f9;
    padding: 3px 4px;
    border-right: 1px solid #cbd5e1;
    border-bottom: 1px solid #94a3b8;
  }
  .vpro-ds-grid th.pinned,
  .vpro-ds-grid td.pinned {
    position: sticky;
    left: 0;
    z-index: 2;
    box-shadow: 2px 0 0 #003366;
  }
  .vpro-ds-grid thead th.pinned {
    z-index: 4;
  }
  .vpro-ds-sort-btn {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.4rem;
    padding: 0.45rem 0.55rem;
    background: transparent;
    color: inherit;
    border: 0;
    font-size: 0.78rem;
    font-weight: 600;
    text-align: left;
  }
  .vpro-ds-sort-indicator {
    font-size: 0.68rem;
    background: #fcba19;
    color: #003366;
    padding: 0.05rem 0.3rem;
    border-radius: 2px;
    font-weight: 700;
  }
  .vpro-ds-col-filter {
    width: 100%;
    box-sizing: border-box;
    padding: 0.2rem 0.35rem;
    border: 1px solid #cbd5e1;
    border-radius: 2px;
    font-size: 0.76rem;
    background: #ffffff;
  }
  .vpro-ds-row td {
    padding: 0;
    border-right: 1px solid #e2e8f0;
    border-bottom: 1px solid #e2e8f0;
    background: #ffffff;
  }
  .vpro-ds-row td.pinned {
    background: #f8fafc;
  }
  .vpro-ds-row td.dirty {
    background: #fef9c3;
  }
  .vpro-ds-row td.invalid {
    background: #fee2e2;
  }
  .vpro-ds-row td.readonly {
    background: #f1f5f9;
  }
  .vpro-ds-cell-wrap {
    display: flex;
    align-items: center;
    height: 33px;
  }
  .vpro-ds-cell-input {
    width: 100%;
    height: 100%;
    box-sizing: border-box;
    border: 0;
    padding: 0 0.45rem;
    background: transparent;
    color: #0f172a;
    font-size: 0.82rem;
  }
  .vpro-ds-cell-input:focus {
    outline: 2px solid #003366;
    outline-offset: -2px;
    background: #ffffff;
  }
  .vpro-ds-open-plot {
    flex: none;
    margin-right: 3px;
    padding: 1px 5px;
    font-size: 0.68rem;
    background: #003366;
    color: #ffffff;
    border: 0;
    border-radius: 2px;
  }
  .mono {
    font-family: var(--font-mono, monospace);
  }
  .text-err {
    color: #b91c1c;
    font-weight: 600;
  }
  .vpro-ds-empty {
    padding: 2rem;
    text-align: center;
    color: #64748b;
  }
  .vpro-ds-footer {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.82rem;
    color: #334155;
  }
  .vpro-ds-pager {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .vpro-ds-pager label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .vpro-ds-err-pill {
    margin-left: 0.5rem;
    padding: 0.1rem 0.45rem;
    background: #fee2e2;
    color: #991b1b;
    border-radius: 3px;
    font-weight: 600;
  }
</style>
