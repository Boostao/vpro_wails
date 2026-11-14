<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    TURBOVEG_LAYER_CROSSWALK,
    defaultInterchangeHubPort,
    defaultYieldScheduler,
    validateTargetProjectName,
    validateXMLSnippetChunked,
    type CSVExportOutcome,
    type GeoPackageExportOutcome,
    type InterchangeHubPort,
    type RDSExportOutcome,
    type TurboVegExportOutcome,
    type TurboVegImportOutcome,
    type UserSpeciesCollisionPolicy,
    type UserSpeciesExportResult,
    type UserSpeciesImportPreview,
    type UserSpeciesImportResult,
    type VenusImportResult,
    type VenusSourceInspection,
  } from './interchangeHub';

  type HubSection = 'venus' | 'turboveg' | 'userspecies' | 'analytics' | 'geopackage';

  let {
    contextId,
    operationHeld = false,
    port = defaultInterchangeHubPort,
    yieldScheduler = defaultYieldScheduler,
    onBusyChange = () => {},
    onProjectActivated = () => {},
  }: {
    contextId: string;
    operationHeld?: boolean;
    port?: InterchangeHubPort;
    yieldScheduler?: () => Promise<void>;
    onBusyChange?: (busy: boolean) => void;
    onProjectActivated?: (state: Record<string, unknown>) => void;
  } = $props();

  let section = $state<HubSection>('venus');
  let busy = $state(false);
  let error = $state('');
  let generation = 0;

  // 1. VENUS 5.0 state
  let venusSourcePath = $state('');
  let venusInspection = $state.raw<VenusSourceInspection | null>(null);
  let venusSelectedProject = $state('');
  let venusTargetProject = $state('');
  let venusDestinationPath = $state('');
  let venusActivate = $state(false);
  let venusResult = $state.raw<VenusImportResult | null>(null);

  // 2. TurboVeg 3 / XML state
  let tvExportFormat = $state<'xml' | 'sqlite'>('sqlite');
  let tvExportPath = $state('');
  let tvExportLump = $state('');
  let tvExportOutcome = $state.raw<TurboVegExportOutcome | null>(null);

  let tvImportPath = $state('');
  let tvImportFormat = $state<'xml' | 'sqlite' | ''>('');
  let tvImportTarget = $state('');
  let tvImportDestPath = $state('');
  let tvImportActivate = $state(false);
  let tvXmlSnippet = $state('');
  let tvXmlSnippetCount = $state<number | null>(null);
  let tvImportOutcome = $state.raw<TurboVegImportOutcome | null>(null);

  // 3. User Species List state
  let userSppExportFormat = $state<'json' | 'csv'>('json');
  let userSppExportPath = $state('');
  let userSppExportResult = $state.raw<UserSpeciesExportResult | null>(null);

  let userSppImportPath = $state('');
  let userSppImportContent = $state('');
  let userSppPolicy = $state<UserSpeciesCollisionPolicy>('skip');
  let userSppPreview = $state.raw<UserSpeciesImportPreview | null>(null);
  let userSppImportResult = $state.raw<UserSpeciesImportResult | null>(null);

  // 4. Long/Wide CSV & R .rds state
  let csvLumpTable = $state('');
  let longCsvDest = $state('');
  let wideCsvDest = $state('');
  let wideLayerMode = $state<'rollup_all' | 'strata_4' | 'species_layer'>('rollup_all');
  let wideZeroFill = $state(true);
  let wideIncludeSiteUnit = $state(true);
  let csvOutcome = $state.raw<CSVExportOutcome | null>(null);

  let rdsDestPath = $state('');
  let rdsOutcome = $state.raw<RDSExportOutcome | null>(null);

  // 5. GeoPackage (.gpkg) state
  let gpkgDestPath = $state('');
  let gpkgCrs = $state<'EPSG:3005' | 'EPSG:4326'>('EPSG:3005');
  let gpkgIncludeAttributes = $state(true);
  let gpkgIncludeUnlocated = $state(false);
  let gpkgLumpTable = $state('');
  let gpkgOutcome = $state.raw<GeoPackageExportOutcome | null>(null);

  function setBusy(next: boolean) {
    busy = next;
    onBusyChange(next);
  }

  function cancelOperation() {
    generation++;
    setBusy(false);
  }

  async function inspectVenus() {
    if (!venusSourcePath.trim()) {
      error = 'Enter a VENUS 5.0 .mdb or .xml source file path.';
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    venusInspection = null;
    setBusy(true);
    try {
      const res = await port.inspectVenusSource(owner, venusSourcePath.trim());
      if (reqGen !== generation || owner !== contextId) return;
      venusInspection = res;
      if (res.projects.length > 0) {
        venusSelectedProject = res.projects[0].projectName;
        venusTargetProject = res.projects[0].suggestedTarget;
        venusDestinationPath = res.projects[0].suggestedDestPath;
      }
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runVenusImport() {
    const check = validateTargetProjectName(venusTargetProject);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      sourcePath: venusSourcePath.trim(),
      sourceProject: venusSelectedProject.trim(),
      targetProject: check.normalized,
      destinationPath: venusDestinationPath.trim(),
      activateOnSuccess: venusActivate,
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.importVenusProject(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      venusResult = res;
      if (res.activated && res.state) {
        onProjectActivated(res.state);
      }
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runTurboVegExport() {
    if (!tvExportPath.trim()) {
      error = 'Enter a destination path for TurboVeg export.';
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      format: tvExportFormat,
      destinationPath: tvExportPath.trim(),
      lumpTable: tvExportLump.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportTurboVeg(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      tvExportOutcome = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function checkTurboVegXmlSnippet() {
    if (busy || operationHeld) return;
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    tvXmlSnippetCount = null;
    setBusy(true);
    try {
      const check = await validateXMLSnippetChunked(
        tvXmlSnippet,
        8192,
        yieldScheduler,
        () => reqGen !== generation || owner !== contextId
      );
      if (reqGen !== generation || owner !== contextId) return;
      if (!check.valid) {
        error = check.error;
        return;
      }
      tvXmlSnippetCount = check.plotTagCount;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runTurboVegImport() {
    const check = validateTargetProjectName(tvImportTarget);
    if (!check.valid) {
      error = check.error;
      return;
    }
    if (!tvImportPath.trim()) {
      error = 'Enter a TurboVeg .xml or .sqlite source path.';
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      sourcePath: tvImportPath.trim(),
      format: tvImportFormat,
      targetProject: check.normalized,
      destinationPath: tvImportDestPath.trim(),
      activateOnSuccess: tvImportActivate,
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.importTurboVeg(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      tvImportOutcome = res;
      if (res.activated && res.state) {
        onProjectActivated(res.state);
      }
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runUserSpeciesExport() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      format: userSppExportFormat,
      destinationPath: userSppExportPath.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportUserSpeciesList(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      userSppExportResult = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runUserSpeciesPreview() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      sourcePath: userSppImportPath.trim(),
      content: userSppImportContent,
      collisionPolicy: userSppPolicy,
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.previewUserSpeciesImport(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      userSppPreview = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runUserSpeciesImport() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      sourcePath: userSppImportPath.trim(),
      content: userSppImportContent,
      collisionPolicy: userSppPolicy,
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.importUserSpeciesList(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      userSppImportResult = res;
      userSppPreview = res.preview;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runLongCsvExport() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      destinationPath: longCsvDest.trim(),
      lumpTable: csvLumpTable.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportLongTidyCSV(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      csvOutcome = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runWideCsvExport() {
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      destinationPath: wideCsvDest.trim(),
      layerMode: wideLayerMode,
      zeroFill: wideZeroFill,
      includeSiteUnit: wideIncludeSiteUnit,
      lumpTable: csvLumpTable.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportWideMatrixCSV(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      csvOutcome = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runRdsExport() {
    if (!rdsDestPath.trim()) {
      error = 'Enter a destination .rds file path.';
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      destinationPath: rdsDestPath.trim(),
      lumpTable: csvLumpTable.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportProjectRDS(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      rdsOutcome = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  async function runGeoPackageExport() {
    if (!gpkgDestPath.trim()) {
      error = 'Enter a destination .gpkg file path.';
      return;
    }
    if (busy || operationHeld || !contextId) return;
    const reqGen = ++generation;
    const owner = contextId;
    const payload = $state.snapshot({
      destinationPath: gpkgDestPath.trim(),
      crs: gpkgCrs,
      includeAttributeTables: gpkgIncludeAttributes,
      includeUnlocatedPlots: gpkgIncludeUnlocated,
      lumpTable: gpkgLumpTable.trim(),
    });
    error = '';
    setBusy(true);
    try {
      const res = await port.exportGeoPackage(owner, payload);
      if (reqGen !== generation || owner !== contextId) return;
      gpkgOutcome = res;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) setBusy(false);
    }
  }

  onDestroy(cancelOperation);
</script>

<section class="bcgov-interchange" data-interchange-hub aria-label="VPro Standards and Analytical Interchange Hub">
  <header class="bcgov-banner">
    <div>
      <p class="eyebrow-gold">STANDARDS, ANALYTICS &amp; GIS INTERCHANGE</p>
      <h2>VENUS 5.0, TurboVeg 3 / XML, User Species, CSV / R (.rds) &amp; OGC GeoPackage (.gpkg)</h2>
    </div>
    <nav class="hub-tabs" aria-label="Interchange formats">
      <button type="button" class:active={section === 'venus'} onclick={() => (section = 'venus')}>VENUS 5.0</button>
      <button type="button" class:active={section === 'turboveg'} onclick={() => (section = 'turboveg')}>TurboVeg 3 / XML</button>
      <button type="button" class:active={section === 'userspecies'} onclick={() => (section = 'userspecies')}>User Species List</button>
      <button type="button" class:active={section === 'analytics'} onclick={() => (section = 'analytics')}>Long/Wide CSV &amp; R .rds</button>
      <button type="button" class:active={section === 'geopackage'} onclick={() => (section = 'geopackage')}>GeoPackage (.gpkg)</button>
      {#if busy}
        <button type="button" class="cancel-btn" onclick={cancelOperation}>Cancel</button>
      {/if}
    </nav>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if section === 'venus'}
    <div class="hub-card">
      <h3>VENUS 5.0 (.mdb &amp; .xml) Project Importer</h3>
      <p class="desc">Pivots normalized VENUS 5.0 vegetation layers, site/soil attributes, and organic/mineral horizons into a verified VP08 SQLite project.</p>
      <div class="form-row">
        <label class="grow">
          <span>VENUS Source File (.mdb, .accdb, or .xml)</span>
          <input type="text" bind:value={venusSourcePath} placeholder="/path/to/VenusProject.mdb or .xml" disabled={busy || operationHeld} />
        </label>
        <button type="button" class="btn-secondary" disabled={busy || operationHeld || !contextId} onclick={inspectVenus}>
          Inspect Source
        </button>
      </div>

      {#if venusInspection}
        <div class="receipt-box">
          <p>
            <strong>Detected:</strong> VENUS {venusInspection.venusVersion} ({venusInspection.sourceFormat.toUpperCase()}) |
            <strong>SHA-256:</strong> <code>{venusInspection.sourceSha256.slice(0, 16)}...</code>
          </p>
          <div class="form-row">
            <label>
              <span>Source VENUS Project</span>
              <select bind:value={venusSelectedProject} disabled={busy || operationHeld}>
                {#each venusInspection.projects as cand (cand.projectName)}
                  <option value={cand.projectName}>{cand.projectName} ({cand.plotCount} plots)</option>
                {/each}
              </select>
            </label>
            <label>
              <span>Target VP08 Project Name</span>
              <input type="text" bind:value={venusTargetProject} disabled={busy || operationHeld} />
            </label>
            <label class="grow">
              <span>Destination .db Path</span>
              <input type="text" bind:value={venusDestinationPath} disabled={busy || operationHeld} />
            </label>
            <label class="check-inline">
              <input type="checkbox" bind:checked={venusActivate} disabled={busy || operationHeld} />
              <span>Open after import</span>
            </label>
            <button type="button" class="btn-primary" disabled={busy || operationHeld} onclick={runVenusImport}>
              Import into VP08
            </button>
          </div>
        </div>
      {/if}

      {#if venusResult}
        <div class="receipt-box success">
          <strong>Imported {venusResult.plotCount} plots into {venusResult.targetProject}</strong>
          <span>Veg rows: {venusResult.vegRowCount} | Humus: {venusResult.humusRowCount} | Mineral: {venusResult.mineralRowCount}</span>
          <span>Saved to: <code>{venusResult.destinationPath}</code></span>
        </div>
      {/if}
    </div>
  {:else if section === 'turboveg'}
    <div class="two-col">
      <div class="hub-card">
        <h3>Export TurboVeg 3 (31-Table .sqlite) or Standard XML (&lt;Plot_package&gt;)</h3>
        <div class="form-stack">
          <label>
            <span>Target Format</span>
            <select bind:value={tvExportFormat} disabled={busy || operationHeld}>
              <option value="sqlite">TurboVeg 3 Official 31-Table SQLite (.sqlite)</option>
              <option value="xml">TurboVeg Standard XML (&lt;Plot_package&gt; for TV2 / TV3 / JUICE / vegdata)</option>
            </select>
          </label>
          <label>
            <span>Destination File Path</span>
            <input type="text" bind:value={tvExportPath} placeholder="/path/to/export.sqlite or .xml" disabled={busy || operationHeld} />
          </label>
          <label>
            <span>Optional *_Lump Table</span>
            <input type="text" bind:value={tvExportLump} placeholder="None (raw species)" disabled={busy || operationHeld} />
          </label>
          <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runTurboVegExport}>
            Export TurboVeg
          </button>
        </div>

        {#if tvExportOutcome}
          <div class="receipt-box success">
            <strong>Exported {tvExportOutcome.plotCount} plots ({tvExportOutcome.observationCount} abundances, {tvExportOutcome.speciesCount} taxa)</strong>
            <span>File: <code>{tvExportOutcome.path}</code></span>
          </div>
        {/if}

        <h4>Canonical VPro &lt;-&gt; TurboVeg Layer Crosswalk (0..10 + UDF Sublayer Tags)</h4>
        <table class="compact-table">
          <thead><tr><th>VPro Columns</th><th>TurboVeg Layer</th><th>Crosswalk &amp; UDF Provenance</th></tr></thead>
          <tbody>
            {#each TURBOVEG_LAYER_CROSSWALK as cw (cw.tvGroup)}
              <tr><td class="mono">{cw.vproLayers}</td><td><strong>{cw.tvGroup}</strong></td><td>{cw.description}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="hub-card">
        <h3>Import TurboVeg (.sqlite or &lt;Plot_package&gt; .xml) into VP08</h3>
        <div class="form-stack">
          <label>
            <span>Source TurboVeg File (.sqlite or .xml)</span>
            <input type="text" bind:value={tvImportPath} placeholder="/path/to/turboveg.sqlite or .xml" disabled={busy || operationHeld} />
          </label>
          <label>
            <span>Format Override</span>
            <select bind:value={tvImportFormat} disabled={busy || operationHeld}>
              <option value="">Auto-detect from extension</option>
              <option value="sqlite">TurboVeg 3 31-Table SQLite (.sqlite)</option>
              <option value="xml">TurboVeg Standard XML (&lt;Plot_package&gt; .xml)</option>
            </select>
          </label>
          <label>
            <span>Target VP08 Project Name</span>
            <input type="text" bind:value={tvImportTarget} placeholder="e.g. Phytosoc2026" disabled={busy || operationHeld} />
          </label>
          <label>
            <span>Destination .db Path (optional)</span>
            <input type="text" bind:value={tvImportDestPath} placeholder="Defaults to projects/<Target>.db" disabled={busy || operationHeld} />
          </label>
          <label class="check-inline">
            <input type="checkbox" bind:checked={tvImportActivate} disabled={busy || operationHeld} />
            <span>Activate imported project</span>
          </label>
          <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runTurboVegImport}>
            Import TurboVeg Project
          </button>
        </div>

        <details class="xml-check-box">
          <summary>Pre-validate pasted XML snippet (chunked non-blocking)</summary>
          <textarea bind:value={tvXmlSnippet} rows={3} placeholder="<Plot_package><Plots><Plot releve_nr=&quot;1&quot;>..." disabled={busy || operationHeld}></textarea>
          <button type="button" class="btn-secondary" disabled={busy || operationHeld} onclick={checkTurboVegXmlSnippet}>
            Validate XML Snippet
          </button>
          {#if tvXmlSnippetCount !== null}
            <p class="ok-text">Valid UTF-8 XML ({tvXmlSnippetCount} plot element(s) detected).</p>
          {/if}
        </details>

        {#if tvImportOutcome}
          <div class="receipt-box success">
            <strong>Imported {tvImportOutcome.plotCount} plots ({tvImportOutcome.observationCount} observations) into {tvImportOutcome.targetProject}</strong>
            <span>Destination: <code>{tvImportOutcome.destinationPath}</code></span>
          </div>
        {/if}
      </div>
    </div>
  {:else if section === 'userspecies'}
    <div class="two-col">
      <div class="hub-card">
        <h3>Export User Species List (VUser.USysUserSpp)</h3>
        <div class="form-stack">
          <label>
            <span>Manifest Format</span>
            <select bind:value={userSppExportFormat} disabled={busy || operationHeld}>
              <option value="json">Signed JSON Manifest (with SHA-256)</option>
              <option value="csv">Portable CSV</option>
            </select>
          </label>
          <label>
            <span>Destination File Path (optional; leave blank for inline copy)</span>
            <input type="text" bind:value={userSppExportPath} placeholder="/path/to/user_species.json" disabled={busy || operationHeld} />
          </label>
          <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runUserSpeciesExport}>
            Export User Species
          </button>
        </div>
        {#if userSppExportResult}
          <div class="receipt-box">
            <strong>{userSppExportResult.rowCount} species | Payload SHA-256: <code>{userSppExportResult.payloadSha256.slice(0, 16)}...</code></strong>
            <textarea readonly rows={6} value={userSppExportResult.content}></textarea>
          </div>
        {/if}
      </div>

      <div class="hub-card">
        <h3>Import Crew Provisional Species with Collision Review</h3>
        <div class="form-stack">
          <label>
            <span>Source File Path (.json or .csv, optional if pasting below)</span>
            <input type="text" bind:value={userSppImportPath} placeholder="/path/to/crew_species.json" disabled={busy || operationHeld} />
          </label>
          <label>
            <span>Or Paste JSON / CSV Payload</span>
            <textarea bind:value={userSppImportContent} rows={4} placeholder="Code,ScientificName,LifeForm,EnglishName" disabled={busy || operationHeld}></textarea>
          </label>
          <label>
            <span>Collision Policy for Existing VUser.USysUserSpp Codes</span>
            <select bind:value={userSppPolicy} disabled={busy || operationHeld}>
              <option value="skip">Skip existing user codes (preserve local definitions)</option>
              <option value="overwrite">Overwrite existing user codes (provincial VLists always protected)</option>
            </select>
          </label>
          <div class="form-row">
            <button type="button" class="btn-secondary" disabled={busy || operationHeld || !contextId} onclick={runUserSpeciesPreview}>
              Review Collisions
            </button>
            <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runUserSpeciesImport}>
              Import User Species
            </button>
          </div>
        </div>

        {#if userSppPreview}
          <div class="receipt-box">
            <p>
              <strong>Total:</strong> {userSppPreview.totalRows} |
              <strong>New:</strong> {userSppPreview.newCount} |
              <strong>User Collisions:</strong> {userSppPreview.userCollisions} |
              <strong>Protected Master Collisions:</strong> {userSppPreview.masterCollisions}
            </p>
          </div>
        {/if}
        {#if userSppImportResult}
          <div class="receipt-box success">
            <strong>Inserted: {userSppImportResult.insertedCount} | Overwritten: {userSppImportResult.overwrittenCount} | Skipped: {userSppImportResult.skippedCount}</strong>
          </div>
        {/if}
      </div>
    </div>
  {:else if section === 'analytics'}
    <div class="two-col">
      <div class="hub-card">
        <h3>Long Tidy CSV &amp; Wide Plot-by-Species Matrix CSV</h3>
        <label class="field-label-block">
          <span>Optional *_Lump Table Applied at Export Time</span>
          <input type="text" bind:value={csvLumpTable} placeholder="None" disabled={busy || operationHeld} />
        </label>

        <div class="sub-box">
          <h4>Long Tidy CSV (PlotNumber, SiteUnit, Layer, Species, ScientificName, Lifeform, Cover, Height, Collected)</h4>
          <div class="form-row">
            <input type="text" class="grow" bind:value={longCsvDest} placeholder="/path/to/veg_long.csv (optional for preview)" disabled={busy || operationHeld} />
            <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runLongCsvExport}>
              Export Long CSV
            </button>
          </div>
        </div>

        <div class="sub-box">
          <h4>Wide Plot x Species Matrix CSV</h4>
          <div class="form-row">
            <label>
              <span>Column Rollup</span>
              <select bind:value={wideLayerMode} disabled={busy || operationHeld}>
                <option value="rollup_all">Species Rollup (summed across layers, 99% cap)</option>
                <option value="strata_4">4-Strata Columns (Species_A, _B, _C, _D)</option>
                <option value="species_layer">Full Sub-Layer Columns (Species_A1..Ep)</option>
              </select>
            </label>
            <label class="check-inline">
              <input type="checkbox" bind:checked={wideZeroFill} disabled={busy || operationHeld} />
              <span>Zero-fill absences</span>
            </label>
            <label class="check-inline">
              <input type="checkbox" bind:checked={wideIncludeSiteUnit} disabled={busy || operationHeld} />
              <span>Include SiteUnit column</span>
            </label>
          </div>
          <div class="form-row">
            <input type="text" class="grow" bind:value={wideCsvDest} placeholder="/path/to/veg_wide_matrix.csv" disabled={busy || operationHeld} />
            <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runWideCsvExport}>
              Export Wide Matrix CSV
            </button>
          </div>
        </div>

        {#if csvOutcome}
          <div class="receipt-box">
            <strong>{csvOutcome.mode}: {csvOutcome.rowCount} rows x {csvOutcome.columnCount} cols ({csvOutcome.speciesCount} taxa)</strong>
            <textarea readonly rows={6} value={csvOutcome.previewCsv}></textarea>
          </div>
        {/if}
      </div>

      <div class="hub-card">
        <h3>Native R Serialization Format v2 (.rds) Export</h3>
        <p class="desc">Writes a pure-Go gzipped RDX2 list of typed data.frames (<code>env</code>, <code>admin</code>, <code>veg_long</code>, <code>humus</code>, <code>mineral</code>, <code>su</code>, <code>metadata</code>) readable directly via <code>readRDS()</code> with zero R runtime dependency.</p>
        <div class="form-stack">
          <label>
            <span>Destination .rds Path</span>
            <input type="text" bind:value={rdsDestPath} placeholder="/path/to/project_snapshot.rds" disabled={busy || operationHeld} />
          </label>
          <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runRdsExport}>
            Export Native R .rds
          </button>
        </div>

        {#if rdsOutcome}
          <div class="receipt-box success">
            <strong>Exported {rdsOutcome.plotCount} plots ({rdsOutcome.byteSize} bytes) to <code>{rdsOutcome.path}</code></strong>
            <table class="compact-table">
              <thead><tr><th>R data.frame</th><th>Rows</th><th>Columns</th></tr></thead>
              <tbody>
                {#each rdsOutcome.tables as tbl (tbl.name)}
                  <tr><td class="mono">{tbl.name}</td><td>{tbl.rowCount}</td><td>{tbl.columnCount}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="hub-card">
      <h3>OGC GeoPackage 1.3 (.gpkg) Spatial &amp; Relational Exporter</h3>
      <p class="desc">Writes native SQLite GeoPackage Point features with full untruncated Env + Admin + SiteUnit columns, automatic Western Hemisphere longitude sign normalization, and optional non-spatial relational tables for QGIS and ArcGIS Pro.</p>
      <div class="form-row">
        <label class="grow">
          <span>Destination .gpkg File Path</span>
          <input type="text" bind:value={gpkgDestPath} placeholder="/path/to/bec_plots.gpkg" disabled={busy || operationHeld} />
        </label>
        <label>
          <span>Coordinate Reference System</span>
          <select bind:value={gpkgCrs} disabled={busy || operationHeld}>
            <option value="EPSG:3005">EPSG:3005 — NAD83 / BC Albers (Ministry Standard)</option>
            <option value="EPSG:4326">EPSG:4326 — NAD83 Geographic (Lon/Lat)</option>
          </select>
        </label>
      </div>
      <div class="form-row">
        <label class="check-inline">
          <input type="checkbox" bind:checked={gpkgIncludeAttributes} disabled={busy || operationHeld} />
          <span>Include veg, humus, mineral attribute tables</span>
        </label>
        <label class="check-inline">
          <input type="checkbox" bind:checked={gpkgIncludeUnlocated} disabled={busy || operationHeld} />
          <span>Include unlocated plots (NULL geometry)</span>
        </label>
        <label>
          <span>Optional *_Lump Table</span>
          <input type="text" bind:value={gpkgLumpTable} placeholder="None" disabled={busy || operationHeld} />
        </label>
        <button type="button" class="btn-primary" disabled={busy || operationHeld || !contextId} onclick={runGeoPackageExport}>
          Export GeoPackage (.gpkg)
        </button>
      </div>

      {#if gpkgOutcome}
        <div class="receipt-box success">
          <strong>Exported {gpkgOutcome.featureCount} plot features ({gpkgOutcome.crs}, SRS {gpkgOutcome.srsId})</strong>
          <span>Located: {gpkgOutcome.locatedPlotCount} | Skipped unlocated: {gpkgOutcome.skippedUnlocatedCount} | Flipped positive West longitudes: {gpkgOutcome.normalizedWestLongitudes}</span>
          <span>Attribute tables: {gpkgOutcome.attributeTables.join(', ') || 'none'} | Path: <code>{gpkgOutcome.path}</code></span>
        </div>
      {/if}
    </div>
  {/if}
</section>

<style>
  .bcgov-interchange { display: grid; gap: 1rem; min-width: 0; }
  .bcgov-banner { background: #003366; color: #ffffff; border-bottom: 4px solid #fcba19; border-radius: 4px; padding: 1rem 1.25rem; display: grid; gap: 0.75rem; }
  .bcgov-banner h2 { margin: 0.15rem 0 0; font-size: 1.15rem; color: #ffffff; }
  .eyebrow-gold { margin: 0; font-size: 0.72rem; letter-spacing: 0.08em; color: #fcba19; font-weight: 700; }
  .hub-tabs { display: flex; flex-wrap: wrap; gap: 0.45rem; }
  .hub-tabs button { background: rgba(255, 255, 255, 0.12); color: #ffffff; border: 1px solid rgba(255, 255, 255, 0.3); border-radius: 4px; padding: 0.42rem 0.8rem; font-size: 0.82rem; }
  .hub-tabs button.active { background: #fcba19; color: #003366; border-color: #fcba19; font-weight: 700; }
  .hub-tabs button.cancel-btn { background: #991b1b; border-color: #fecaca; margin-left: auto; }
  .two-col { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 1rem; }
  .hub-card { background: #ffffff; border: 1px solid #cbd5e1; border-top: 3px solid #003366; border-radius: 4px; padding: 1rem; display: grid; gap: 0.75rem; align-content: start; }
  .hub-card h3 { margin: 0; color: #003366; font-size: 1rem; }
  .hub-card h4 { margin: 0.35rem 0 0.25rem; color: #003366; font-size: 0.86rem; }
  .desc { margin: 0; color: #475569; font-size: 0.83rem; line-height: 1.45; }
  .form-row { display: flex; flex-wrap: wrap; gap: 0.6rem; align-items: end; }
  .form-stack { display: grid; gap: 0.6rem; }
  .form-row label, .form-stack label, .field-label-block { display: grid; gap: 0.25rem; font-size: 0.8rem; color: #1e293b; }
  .grow { flex: 1 1 220px; }
  .check-inline { display: inline-flex !important; flex-direction: row; align-items: center; gap: 0.4rem; padding-bottom: 0.35rem; }
  input:not([type="checkbox"]), select, textarea { padding: 0.45rem 0.6rem; border: 1px solid #94a3b8; border-radius: 4px; font-size: 0.84rem; font-family: inherit; }
  textarea { font-family: var(--font-mono, monospace); font-size: 0.78rem; width: 100%; box-sizing: border-box; }
  .btn-primary { background: #003366; color: #ffffff; border: 1px solid #003366; border-radius: 4px; padding: 0.48rem 0.9rem; font-weight: 600; font-size: 0.83rem; }
  .btn-secondary { background: #ffffff; color: #003366; border: 1px solid #003366; border-radius: 4px; padding: 0.48rem 0.9rem; font-weight: 600; font-size: 0.83rem; }
  .sub-box { border: 1px solid #e2e8f0; border-left: 3px solid #fcba19; padding: 0.65rem; border-radius: 4px; display: grid; gap: 0.5rem; }
  .receipt-box { background: #f8fafc; border: 1px solid #cbd5e1; border-left: 4px solid #003366; padding: 0.65rem 0.8rem; border-radius: 4px; display: grid; gap: 0.35rem; font-size: 0.82rem; }
  .receipt-box.success { background: #f0fdf4; border-left-color: #166534; }
  .compact-table { width: 100%; min-width: 0; border-collapse: collapse; font-size: 0.78rem; }
  .compact-table th, .compact-table td { padding: 0.35rem 0.5rem; border-bottom: 1px solid #e2e8f0; white-space: normal; }
  .mono { font-family: var(--font-mono, monospace); }
  .xml-check-box { font-size: 0.8rem; border-top: 1px solid #e2e8f0; padding-top: 0.5rem; }
  .ok-text { color: #166534; margin: 0.3rem 0 0; font-weight: 600; }
</style>
