<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    defaultLumpEditorPort,
    groupLumpRowsByCode,
    parseGenusRollupTokens,
    validateLumpBaseName,
    validateLumpRowDraft,
    type LumpEditorPort,
    type LumpImpactPreview,
    type LumpMaterializeResult,
    type LumpPresetInfo,
    type LumpRow,
    type LumpSpec,
    type LumpSpecPreview,
    type LumpTableSummary,
  } from './lumpEditor';

  let {
    contextId,
    operationHeld = false,
    port = defaultLumpEditorPort,
    onBusyChange = () => {},
    onChanged = () => {},
  }: {
    contextId: string;
    operationHeld?: boolean;
    port?: LumpEditorPort;
    onBusyChange?: (busy: boolean) => void;
    onChanged?: (tableName: string) => void;
  } = $props();

  let activeTab = $state<'tables' | 'recipes'>('tables');
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');

  // Discovered tables & active selection
  let externalDbInput = $state('');
  let tables = $state.raw<LumpTableSummary[]>([]);
  let selectedTable = $state('');
  let selectedDbPath = $state('');
  let rows = $state.raw<LumpRow[]>([]);
  let impact = $state.raw<LumpImpactPreview | null>(null);
  let filterQuery = $state('');

  // Create / SaveAs inputs
  let newTableName = $state('');
  let saveAsTargetName = $state('');

  // Row editor inputs
  let draftLumpCode = $state('');
  let draftSppCode = $state('');
  let draftUse = $state(true);
  let draftAllowUnlisted = $state(false);
  let editingSppCode = $state('');

  // Declarative LumpSpec (Auto-5) state
  let presets = $state.raw<LumpPresetInfo[]>([]);
  let specPreset = $state('BEC_Provincial_Correlation_2026');
  let collapseInfraspecific = $state(true);
  let collapseSeedlingAndDead = $state(true);
  let genusRollupsText = $state('CARE*, SALI*, SPHA*, CLAD*');
  let rollupMosses = $state(false);
  let rollupLiverworts = $state(false);
  let rollupLichens = $state(false);
  let scopeToProject = $state(true);
  let materializeName = $state('BEC2026');
  let replaceExisting = $state(true);
  let specPreview = $state.raw<LumpSpecPreview | null>(null);
  let lastMaterialized = $state.raw<LumpMaterializeResult | null>(null);

  let generation = 0;

  function setBusyState(next: boolean) {
    busy = next;
    onBusyChange(next);
  }

  function cancelPending() {
    generation++;
    setBusyState(false);
  }

  const filteredRows = $derived.by(() => {
    const q = filterQuery.trim().toUpperCase();
    if (!q) return rows;
    return rows.filter(
      (r) =>
        r.lumpCode.toUpperCase().includes(q) ||
        r.sppCode.toUpperCase().includes(q) ||
        r.scientificName.toUpperCase().includes(q) ||
        r.englishName.toUpperCase().includes(q)
    );
  });

  const groupedRows = $derived.by(() => groupLumpRowsByCode(filteredRows));

  const selectedSummary = $derived.by(() =>
    tables.find((t) => t.tableName === selectedTable && t.databasePath === selectedDbPath) ?? null
  );

  function buildCurrentSpec(): LumpSpec {
    const lifeforms: number[] = [];
    if (rollupMosses) lifeforms.push(9);
    if (rollupLiverworts) lifeforms.push(10);
    if (rollupLichens) lifeforms.push(11);
    return {
      preset: specPreset,
      collapseInfraspecific,
      collapseSeedlingAndDeadSuffixes: collapseSeedlingAndDead,
      genusRollups: parseGenusRollupTokens(genusRollupsText),
      nonVascularLifeforms: lifeforms,
      baseLumpTable: selectedSummary?.isActiveProject ? selectedSummary.tableName : '',
      scopeToProjectSpecies: scopeToProject,
    };
  }

  async function refreshTables() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    setBusyState(true);
    try {
      const extPaths = externalDbInput
        .split(/[,;\n]+/)
        .map((s) => s.trim())
        .filter(Boolean);
      const extSnapshot = $state.snapshot(extPaths);
      const [discovered, loadedPresets] = await Promise.all([
        port.discoverLumpTables(owner, extSnapshot),
        port.listLumpPresets(),
      ]);
      if (reqGen !== generation || owner !== contextId) return;
      tables = discovered ?? [];
      presets = loadedPresets ?? [];
      if (tables.length > 0 && !tables.some((t) => t.tableName === selectedTable && t.databasePath === selectedDbPath)) {
        selectedTable = tables[0].tableName;
        selectedDbPath = tables[0].databasePath;
      }
      if (selectedTable) {
        const [loadedRows, loadedImpact] = await Promise.all([
          port.getLumpTableRows(owner, selectedTable, selectedDbPath),
          port.previewLumpImpact(owner, selectedTable, selectedDbPath),
        ]);
        if (reqGen !== generation || owner !== contextId) return;
        rows = loadedRows ?? [];
        impact = loadedImpact;
      } else {
        rows = [];
        impact = null;
      }
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusyState(false);
    }
  }

  async function selectLumpTable(tableName: string, dbPath: string) {
    if (busy || operationHeld || !contextId) return;
    selectedTable = tableName;
    selectedDbPath = dbPath;
    editingSppCode = '';
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    setBusyState(true);
    try {
      const [loadedRows, loadedImpact] = await Promise.all([
        port.getLumpTableRows(owner, tableName, dbPath),
        port.previewLumpImpact(owner, tableName, dbPath),
      ]);
      if (reqGen !== generation || owner !== contextId) return;
      rows = loadedRows ?? [];
      impact = loadedImpact;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusyState(false);
    }
  }

  async function createNewLump() {
    const check = validateLumpBaseName(newTableName);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    notice = '';
    setBusyState(true);
    try {
      const created = await port.createLumpTable(owner, check.normalized);
      if (reqGen !== generation || owner !== contextId) return;
      selectedTable = created.tableName;
      selectedDbPath = created.databasePath;
      newTableName = '';
      notice = `Created ${created.tableName} (VP05).`;
      onChanged(created.tableName);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await refreshTables();
  }

  async function saveAsLump() {
    if (!selectedTable) {
      error = 'Select a source lump table first.';
      return;
    }
    const check = validateLumpBaseName(saveAsTargetName);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    notice = '';
    setBusyState(true);
    try {
      const copied = await port.saveAsLumpTable(owner, selectedTable, selectedDbPath, check.normalized);
      if (reqGen !== generation || owner !== contextId) return;
      selectedTable = copied.tableName;
      selectedDbPath = copied.databasePath;
      saveAsTargetName = '';
      notice = `Copied to ${copied.tableName} (${copied.rowCount} rows).`;
      onChanged(copied.tableName);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await refreshTables();
  }

  async function submitRow() {
    if (!selectedSummary || !selectedSummary.isActiveProject) {
      error = 'Select a local active-project *_Lump table (or Save As into the active project) before editing rows.';
      return;
    }
    const check = validateLumpRowDraft(draftLumpCode, draftSppCode);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      lumpCode: check.lumpCode,
      sppCode: check.sppCode,
      use: draftUse,
      allowUnlisted: draftAllowUnlisted,
    });
    error = '';
    setBusyState(true);
    try {
      if (editingSppCode) {
        await port.updateLumpRow(owner, selectedTable, editingSppCode, payload);
      } else {
        await port.upsertLumpRow(owner, selectedTable, payload);
      }
      if (reqGen !== generation || owner !== contextId) return;
      draftSppCode = '';
      editingSppCode = '';
      onChanged(selectedTable);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await selectLumpTable(selectedTable, selectedDbPath);
  }

  async function toggleRowUse(row: LumpRow) {
    if (!selectedSummary?.isActiveProject || busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    setBusyState(true);
    try {
      await port.toggleLumpRow(owner, selectedTable, row.sppCode, !row.use);
      if (reqGen !== generation || owner !== contextId) return;
      onChanged(selectedTable);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await selectLumpTable(selectedTable, selectedDbPath);
  }

  async function deleteRow(sppCode: string) {
    if (!selectedSummary?.isActiveProject || busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    setBusyState(true);
    try {
      await port.deleteLumpRow(owner, selectedTable, sppCode);
      if (reqGen !== generation || owner !== contextId) return;
      onChanged(selectedTable);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await selectLumpTable(selectedTable, selectedDbPath);
  }

  async function setAllUse(nextUse: boolean) {
    if (!selectedSummary?.isActiveProject || busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    setBusyState(true);
    try {
      await port.setAllLumpRowsUse(owner, selectedTable, nextUse);
      if (reqGen !== generation || owner !== contextId) return;
      onChanged(selectedTable);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await selectLumpTable(selectedTable, selectedDbPath);
  }

  function applyPresetSelection(presetID: string) {
    specPreset = presetID;
    const found = presets.find((p) => p.id === presetID);
    if (found) {
      collapseInfraspecific = found.spec.collapseInfraspecific;
      collapseSeedlingAndDead = found.spec.collapseSeedlingAndDeadSuffixes;
      genusRollupsText = (found.spec.genusRollups ?? []).join(', ');
    }
  }

  async function runSpecPreview() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const specSnapshot = $state.snapshot(buildCurrentSpec());
    error = '';
    setBusyState(true);
    try {
      const result = await port.previewLumpSpec(owner, specSnapshot);
      if (reqGen !== generation || owner !== contextId) return;
      specPreview = result;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusyState(false);
    }
  }

  async function runMaterializeSpec() {
    const check = validateLumpBaseName(materializeName);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const specSnapshot = $state.snapshot(buildCurrentSpec());
    error = '';
    notice = '';
    setBusyState(true);
    try {
      const res = await port.materializeLumpSpec(owner, check.normalized, specSnapshot, replaceExisting);
      if (reqGen !== generation || owner !== contextId) return;
      lastMaterialized = res;
      selectedTable = res.table.tableName;
      selectedDbPath = res.table.databasePath;
      notice = `Materialized ${res.rowsWritten} folding rules into ${res.table.tableName}.`;
      onChanged(res.table.tableName);
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
      setBusyState(false);
      return;
    }
    setBusyState(false);
    await refreshTables();
  }

  function startEditRow(row: LumpRow) {
    editingSppCode = row.sppCode;
    draftLumpCode = row.lumpCode;
    draftSppCode = row.sppCode;
    draftUse = row.use;
    draftAllowUnlisted = row.sourceCatalog === 'Unlisted';
  }

  onDestroy(cancelPending);
</script>

<section class="bcgov-lump-editor" data-lump-editor aria-label="Combine Species and Declarative LumpSpec Workbench">
  <header class="bcgov-header">
    <div>
      <p class="bcgov-eyebrow">BEC TAXONOMIC CORRELATION</p>
      <h2>Combine Species (*_Lump) &amp; Declarative LumpSpec Recipes</h2>
    </div>
    <div class="bcgov-tab-strip" role="tablist">
      <button
        type="button"
        role="tab"
        aria-selected={activeTab === 'tables'}
        class:active={activeTab === 'tables'}
        onclick={() => (activeTab = 'tables')}
      >
        Lump Tables (*_Lump)
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={activeTab === 'recipes'}
        class:active={activeTab === 'recipes'}
        onclick={() => (activeTab = 'recipes')}
      >
        Declarative LumpSpec (Auto-5)
      </button>
      <button type="button" class="bcgov-btn-secondary" disabled={busy || operationHeld || !contextId} onclick={refreshTables}>
        Refresh Catalog
      </button>
    </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  {#if notice}
    <p class="bcgov-notice" role="status">{notice}</p>
  {/if}

  {#if activeTab === 'tables'}
    <div class="lump-grid">
      <aside class="bcgov-card">
        <h3>Discovered *_Lump Tables</h3>
        <label class="field-block">
          <span>External .db paths (optional, comma-separated)</span>
          <input type="text" bind:value={externalDbInput} placeholder="/path/to/ExternalProject.db" disabled={busy || operationHeld} />
        </label>

        <ul class="lump-table-list">
          {#if tables.length === 0}
            <li class="empty-note">No *_Lump tables loaded. Click Refresh Catalog or create a new table below.</li>
          {/if}
          {#each tables as item (item.databasePath + '::' + item.tableName)}
            <li>
              <button
                type="button"
                class:selected={item.tableName === selectedTable && item.databasePath === selectedDbPath}
                onclick={() => selectLumpTable(item.tableName, item.databasePath)}
              >
                <strong>{item.tableName}</strong>
                <span>{item.activeCount}/{item.rowCount} active | {item.isActiveProject ? 'Project' : 'External'}</span>
              </button>
            </li>
          {/each}
        </ul>

        <div class="stack-actions">
          <label class="field-block">
            <span>Create New Lump Table</span>
            <div class="inline-row">
              <input type="text" bind:value={newTableName} placeholder="e.g. Regional2026" disabled={busy || operationHeld} />
              <button type="button" class="bcgov-btn-primary" disabled={busy || operationHeld || !contextId} onclick={createNewLump}>
                Create
              </button>
            </div>
          </label>

          <label class="field-block">
            <span>Save Selected As</span>
            <div class="inline-row">
              <input type="text" bind:value={saveAsTargetName} placeholder="CopyName" disabled={busy || operationHeld || !selectedTable} />
              <button type="button" class="bcgov-btn-secondary" disabled={busy || operationHeld || !selectedTable} onclick={saveAsLump}>
                Save As
              </button>
            </div>
          </label>
        </div>
      </aside>

      <div class="bcgov-card">
        <h3>
          {selectedTable || 'No Lump Table Selected'}
          {#if selectedSummary}
            <span class="badge">{selectedSummary.metadataVersion || 'VP05'}</span>
          {/if}
        </h3>

        {#if selectedSummary?.isActiveProject}
          <div class="row-editor" aria-label="Add or update species lump mapping">
            <label>
              <span>Target LumpCode (&lt;=50)</span>
              <input type="text" bind:value={draftLumpCode} placeholder="CAREX" disabled={busy || operationHeld} />
            </label>
            <label>
              <span>Source SppCode (&lt;=8)</span>
              <input type="text" bind:value={draftSppCode} placeholder="CARESIT" disabled={busy || operationHeld} />
            </label>
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={draftUse} disabled={busy || operationHeld} />
              <span>Use (-1)</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={draftAllowUnlisted} disabled={busy || operationHeld} />
              <span>Allow unlisted code</span>
            </label>
            <button type="button" class="bcgov-btn-primary" disabled={busy || operationHeld} onclick={submitRow}>
              {editingSppCode ? `Update ${editingSppCode}` : 'Add / Upsert'}
            </button>
            {#if editingSppCode}
              <button type="button" class="bcgov-btn-secondary" onclick={() => { editingSppCode = ''; draftSppCode = ''; }}>
                Cancel Edit
              </button>
            {/if}
          </div>

          <div class="toolbar-row">
            <input type="search" bind:value={filterQuery} placeholder="Filter by code or scientific name..." />
            <button type="button" class="bcgov-btn-secondary" disabled={busy || operationHeld || rows.length === 0} onclick={() => setAllUse(true)}>
              Enable All (-1)
            </button>
            <button type="button" class="bcgov-btn-secondary" disabled={busy || operationHeld || rows.length === 0} onclick={() => setAllUse(false)}>
              Disable All (0)
            </button>
          </div>
        {/if}

        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>LumpCode</th>
                <th>SppCode</th>
                <th>Scientific Name</th>
                <th>Catalog</th>
                <th>Active (Use)</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {#if groupedRows.length === 0}
                <tr><td colspan="6" class="empty-row">No species mappings in current selection.</td></tr>
              {/if}
              {#each groupedRows as grp (grp.lumpCode)}
                {#each grp.rows as row (row.sppCode)}
                  <tr>
                    <td class="mono">{row.lumpCode}</td>
                    <td class="mono">{row.sppCode}</td>
                    <td>{row.scientificName || 'Unlisted taxon'}</td>
                    <td>{row.sourceCatalog}</td>
                    <td>
                      <input
                        type="checkbox"
                        checked={row.use}
                        disabled={!selectedSummary?.isActiveProject || busy || operationHeld}
                        onchange={() => toggleRowUse(row)}
                        aria-label={`Toggle ${row.sppCode}`}
                      />
                    </td>
                    <td>
                      {#if selectedSummary?.isActiveProject}
                        <button type="button" class="link-btn" onclick={() => startEditRow(row)}>Edit</button>
                        <button type="button" class="link-btn danger" onclick={() => deleteRow(row.sppCode)}>Delete</button>
                      {:else}
                        <span>Read-only external</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </div>

    {#if impact}
      <section class="bcgov-card impact-panel" aria-label="Project species folding impact preview">
        <h3>Live Project Folding Impact (Summed Cover Capped at {impact.coverCap}%)</h3>
        <div class="stat-strip">
          <div><span>Scoped Plots</span><strong>{impact.totalPlots}</strong></div>
          <div><span>Raw Species</span><strong>{impact.distinctRawSpecies}</strong></div>
          <div><span>Folded Species</span><strong>{impact.distinctFoldedSpecies}</strong></div>
          <div><span>Merged Plot-Layer Cells</span><strong>{impact.mergedCells}</strong></div>
          <div><span>Cells Capped at 99%</span><strong>{impact.cappedCells}</strong></div>
        </div>
      </section>
    {/if}
  {:else}
    <div class="bcgov-card">
      <h3>Declarative LumpSpec Compiler (Auto-5)</h3>
      <div class="recipe-grid">
        <label class="field-block">
          <span>Provincial Preset</span>
          <select value={specPreset} onchange={(e) => applyPresetSelection(e.currentTarget.value)} disabled={busy || operationHeld}>
            <option value="BEC_Provincial_Correlation_2026">BEC_Provincial_Correlation_2026</option>
            <option value="">Custom Recipe Only</option>
          </select>
        </label>

        <label class="field-block">
          <span>Wildcard Genus Rollups</span>
          <input type="text" bind:value={genusRollupsText} placeholder="CARE*, SALI*, SPHA*, CLAD*" disabled={busy || operationHeld} />
        </label>

        <div class="checkbox-group">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={collapseInfraspecific} disabled={busy || operationHeld} />
            <span>Collapse 8-char infraspecifics (1..8 -&gt; 7-char parent)</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={collapseSeedlingAndDead} disabled={busy || operationHeld} />
            <span>Collapse seedling &amp; dead codes (_D / _X)</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={rollupMosses} disabled={busy || operationHeld} />
            <span>Rollup Lifeform 9 (Mosses) to genus</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={rollupLiverworts} disabled={busy || operationHeld} />
            <span>Rollup Lifeform 10 (Liverworts) to genus</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={rollupLichens} disabled={busy || operationHeld} />
            <span>Rollup Lifeform 11 (Lichens) to genus</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={scopeToProject} disabled={busy || operationHeld} />
            <span>Scope materialization to species present in active project</span>
          </label>
        </div>

        <div class="inline-row">
          <button type="button" class="bcgov-btn-secondary" disabled={busy || operationHeld || !contextId} onclick={runSpecPreview}>
            Preview LumpSpec Impact
          </button>
          <input type="text" bind:value={materializeName} placeholder="Target name (e.g. BEC2026)" disabled={busy || operationHeld} />
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={replaceExisting} disabled={busy || operationHeld} />
            <span>Overwrite if exists</span>
          </label>
          <button type="button" class="bcgov-btn-primary" disabled={busy || operationHeld || !contextId} onclick={runMaterializeSpec}>
            Materialize into *_Lump
          </button>
        </div>
      </div>

      {#if specPreview}
        <div class="spec-results">
          <p>
            <strong>Compiled Rules:</strong> {specPreview.rows.length} mappings |
            <strong>Raw -&gt; Folded Species:</strong> {specPreview.impact.distinctRawSpecies} -&gt; {specPreview.impact.distinctFoldedSpecies} |
            <strong>Merged Cells:</strong> {specPreview.impact.mergedCells} |
            <strong>99%-Capped Cells:</strong> {specPreview.impact.cappedCells}
          </p>
          <div class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>LumpCode</th>
                  <th>SppCode</th>
                  <th>Scientific Name</th>
                  <th>Rule Provenance</th>
                </tr>
              </thead>
              <tbody>
                {#each specPreview.rows.slice(0, 100) as row (row.sppCode)}
                  <tr>
                    <td class="mono">{row.lumpCode}</td>
                    <td class="mono">{row.sppCode}</td>
                    <td>{row.scientificName}</td>
                    <td>{row.ruleOrigin || 'recipe'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
      {#if lastMaterialized}
        <p class="bcgov-notice">
          Materialized {lastMaterialized.rowsWritten} rows into {lastMaterialized.table.tableName}.
        </p>
      {/if}
    </div>
  {/if}
</section>

<style>
  .bcgov-lump-editor { display: grid; gap: 1rem; min-width: 0; }
  .bcgov-header { background: #003366; color: #ffffff; border-bottom: 4px solid #fcba19; padding: 1rem 1.25rem; border-radius: 4px; display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 1rem; }
  .bcgov-header h2 { margin: 0.2rem 0 0; font-size: 1.2rem; color: #ffffff; }
  .bcgov-eyebrow { margin: 0; font-size: 0.72rem; letter-spacing: 0.08em; color: #fcba19; font-weight: 700; }
  .bcgov-tab-strip { display: flex; flex-wrap: wrap; gap: 0.5rem; }
  .bcgov-tab-strip button[role="tab"] { background: rgba(255, 255, 255, 0.12); color: #ffffff; border: 1px solid rgba(255, 255, 255, 0.28); padding: 0.45rem 0.85rem; border-radius: 4px; font-size: 0.85rem; }
  .bcgov-tab-strip button[role="tab"].active { background: #fcba19; color: #003366; border-color: #fcba19; font-weight: 700; }
  .bcgov-card { background: #ffffff; border: 1px solid #d1d9e0; border-top: 3px solid #003366; border-radius: 4px; padding: 1rem; min-width: 0; }
  .bcgov-card h3 { margin: 0 0 0.75rem; font-size: 1rem; color: #003366; display: flex; align-items: center; gap: 0.5rem; }
  .badge { background: #fcba19; color: #003366; padding: 0.1rem 0.45rem; border-radius: 3px; font-size: 0.75rem; font-weight: 700; }
  .lump-grid { display: grid; grid-template-columns: minmax(260px, 320px) minmax(0, 1fr); gap: 1rem; }
  .lump-table-list { list-style: none; padding: 0; margin: 0.75rem 0; display: grid; gap: 0.35rem; max-height: 240px; overflow-y: auto; }
  .lump-table-list button { width: 100%; display: flex; justify-content: space-between; align-items: center; padding: 0.5rem 0.65rem; border: 1px solid #cbd5e1; border-radius: 4px; background: #f8fafc; text-align: left; font-size: 0.82rem; }
  .lump-table-list button.selected { background: #e6eef8; border-color: #003366; color: #003366; }
  .field-block { display: grid; gap: 0.3rem; font-size: 0.82rem; margin-bottom: 0.65rem; }
  .inline-row, .toolbar-row, .row-editor { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: end; margin-bottom: 0.75rem; }
  .row-editor label { display: grid; gap: 0.25rem; font-size: 0.8rem; }
  .checkbox-group { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 0.45rem; margin: 0.65rem 0; }
  .checkbox-label { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.83rem; }
  input:not([type="checkbox"]), select { padding: 0.45rem 0.6rem; border: 1px solid #94a3b8; border-radius: 4px; font-size: 0.85rem; }
  .bcgov-btn-primary { background: #003366; color: #ffffff; border: 1px solid #003366; padding: 0.48rem 0.85rem; border-radius: 4px; font-weight: 600; font-size: 0.84rem; }
  .bcgov-btn-secondary { background: #ffffff; color: #003366; border: 1px solid #003366; padding: 0.48rem 0.85rem; border-radius: 4px; font-weight: 600; font-size: 0.84rem; }
  .bcgov-notice { padding: 0.65rem 0.9rem; background: #eef5fc; border-left: 4px solid #003366; color: #003366; margin: 0; font-size: 0.85rem; }
  .stat-strip { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 0.75rem; }
  .stat-strip div { padding: 0.55rem 0.75rem; background: #f8fafc; border-left: 3px solid #fcba19; display: grid; }
  .stat-strip span { font-size: 0.75rem; color: #475569; }
  .stat-strip strong { font-size: 1.1rem; color: #003366; }
  .mono { font-family: var(--font-mono, monospace); }
  .link-btn { background: transparent; border: 0; color: #003366; text-decoration: underline; padding: 0.15rem 0.35rem; font-size: 0.8rem; }
  .link-btn.danger { color: #991b1b; }
  .empty-note { font-size: 0.82rem; color: #64748b; padding: 0.5rem 0; }
  @media (max-width: 860px) { .lump-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
