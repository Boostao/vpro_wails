<script lang="ts">
  import { onDestroy } from 'svelte';
  import { GoogleEarthReviewService, GoogleEarthKMLService, GoogleEarthKMLExportService, GoogleEarthPreferencesService, type GoogleEarthKMLReview, type GoogleEarthKMLExportReview } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { reportCellText } from './longEnvironmentReport';
  import { validateGoogleEarthFields, validateGoogleEarthReview, type ValidatedGoogleEarthFields, type ValidatedGoogleEarthReview } from './googleEarthReview';
  import { googleEarthXMLText, validateGoogleEarthKMLReview } from './googleEarthKMLReview';
  import { validateGoogleEarthKMLExportReview, googleEarthKMLPublicationSession } from './googleEarthKMLExport';
  import { wellFormedUTF16 } from './qualityEditor';
  import { googleEarthPreferencesSession } from './googleEarthPreferences';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let fields = $state<ValidatedGoogleEarthFields | null>(null);
  let selectedField = $state('');
  let review = $state<ValidatedGoogleEarthReview | null>(null);
  let busy = $state(false);
  let error = $state('');
  const kmlEnabled = import.meta.env.VITE_GOOGLE_EARTH_KML_PREVIEW === 'true';
  const kmlExportEnabled = import.meta.env.VITE_GOOGLE_EARTH_KML_EXPORT === 'true';
  const preferencesEnabled = import.meta.env.VITE_GOOGLE_EARTH_PREFERENCES === 'true';
  let title = $state('VPro Plot Locations');
  let kml = $state<GoogleEarthKMLReview | null>(null);
  let approval = $state<GoogleEarthKMLExportReview | null>(null);
  let destination = $state('');
  const publicationSession = googleEarthKMLPublicationSession(scope());
  let publication = $state(publicationSession.view());
  const preferenceSession = preferencesEnabled ? googleEarthPreferencesSession(scope()) : null;
  const initialPreferences = preferenceSession?.view() ?? null;
  let preferences = $state(initialPreferences);
  if (initialPreferences) { title = initialPreferences.draft.title; selectedField = initialPreferences.draft.descriptionField; }
  const preferenceLocked = $derived(!!preferences && (preferences.busy || preferences.blocked));
  const locked = $derived(busy || publication.busy || publication.blocked || preferenceLocked);
  function updateBusy() { onBusyChange(busy || publication.busy || publication.blocked || !!preferences && (preferences.busy || preferences.blocked || !!preferences.draftError)); }
  const unsubscribe = publicationSession.subscribe(() => {
    publication = publicationSession.view();
    updateBusy();
  });
  const unsubscribePreferences = preferenceSession?.subscribe(() => { preferences = preferenceSession.view(); updateBusy(); });
  const pageSize = 50;
  const reads = new ReadRequests();
  let generation = 0;
  const previousOffset = $derived(review ? Math.max(0, review.offset - pageSize) : 0);
  const nextOffset = $derived(review ? review.offset + pageSize : 0);
  function scope() { return { contextId, project, projectPath, su, suPath }; }
  function cancel() { generation++; preferenceSession?.cancelLoad(); reads.cancelAll(); busy = false; updateBusy(); }
  function rememberDraft() { preferenceSession?.edit({ title, descriptionField: selectedField }); }
  function changeField(event: Event) {
    selectedField = (event.currentTarget as HTMLSelectElement).value;
    cancel(); rememberDraft(); review = null; kml = null; approval = null; error = '';
  }
  function changeTitle(event: Event) {
    title = (event.currentTarget as HTMLTextAreaElement).value;
    cancel(); rememberDraft(); kml = null; approval = null;
    error = preferencesEnabled || googleEarthXMLText(title) ? '' : 'Place Name requires valid Unicode and XML text; no text repaired.';
  }
  function undoPreferences() {
    if (locked || !preferenceSession) return;
    cancel(); preferenceSession.undo();
    const restored = preferenceSession.view().draft;
    title = restored.title; selectedField = restored.descriptionField;
    review = null; kml = null; approval = null; error = '';
  }
  async function loadPreferences() {
    if (locked || !preferenceSession) return;
    cancel(); const request = generation, preferenceRequest = preferenceSession.beginLoad();
    error = ''; busy = true; updateBusy();
    try {
      const value = await reads.track(GoogleEarthPreferencesService.GetGoogleEarthPreferences(contextId));
      if (request !== generation) return;
      if (!preferenceSession.applyLoaded(value, preferenceRequest)) return;
      const applied = preferenceSession.view().draft;
      title = applied.title; selectedField = applied.descriptionField;
      review = null; kml = null; approval = null;
      if (!googleEarthXMLText(title)) error = 'Saved Place Name retained literally but is not valid KML XML text; correct it explicitly before preparing KML.';
    } catch (cause) {
      if (request === generation) error = `Saved preferences unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  async function savePreferences() {
    if (locked || !preferenceSession) return;
    error = '';
    try {
      await preferenceSession.save(fields?.fields.map(field => field.name) ?? [], request =>
        GoogleEarthPreferencesService.SaveGoogleEarthPreferences(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  async function loadFields() {
    if (locked) { error = 'Finish or acknowledge the current operation before reloading fields.'; return; }
    cancel(); const request = generation, expected = scope();
    fields = null; if (!preferencesEnabled) selectedField = ''; review = null; kml = null; approval = null; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(GoogleEarthReviewService.GetGoogleEarthDescriptionFields(contextId));
      if (request !== generation) return;
      fields = validateGoogleEarthFields(value, expected);
      preferenceSession?.setPhysicalFields(fields.fields.map(field => field.name));
    } catch (cause) {
      if (request === generation) error = `Description fields unavailable; no data, audits, preferences or files written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  async function prepareKML() {
    if (locked) { error = 'Finish or acknowledge the current operation before preparing KML.'; return; }
    if (!fields || !fields.fields.some(field => field.name === selectedField) || !googleEarthXMLText(title)) {
      error = 'Load and choose a literal description field and correct Place Name before preparing KML.'; return;
    }
    cancel(); const request = generation, expected = scope(), descriptionField = selectedField, placeName = title;
    kml = null; approval = null; error = ''; busy = true; onBusyChange(true);
    try {
      const payload = JSON.stringify({ descriptionField, title: placeName });
      if (kmlExportEnabled) {
        const value = await reads.track(GoogleEarthKMLExportService.GetGoogleEarthKMLExportReview(contextId, payload));
        if (request !== generation) return;
        const validated = await validateGoogleEarthKMLExportReview(value, expected, descriptionField, placeName);
        if (request !== generation) return;
        approval = validated; kml = validated.review;
      } else {
        const value = await reads.track(GoogleEarthKMLService.GetGoogleEarthKMLReview(contextId, payload));
        if (request !== generation) return;
        const validated = await validateGoogleEarthKMLReview(value, expected, descriptionField, placeName);
        if (request !== generation) return;
        kml = validated;
      }
    } catch (cause) {
      if (request === generation) error = `KML preview unavailable; no data, audits, preferences or files written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  async function show(offset = 0) {
    if (locked) { error = 'Finish or acknowledge the current operation before another review.'; return; }
    if (!fields || !fields.fields.some(field => field.name === selectedField)) {
      error = 'Load the owned physical Env fields and choose a literal description field before reviewing.'; return;
    }
    cancel(); const request = generation, expected = scope(), descriptionField = selectedField;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(GoogleEarthReviewService.GetGoogleEarthReview(contextId,
        JSON.stringify({ descriptionField, offset, limit: pageSize })));
      if (request !== generation) return;
      const validated = await validateGoogleEarthReview(value, expected, descriptionField, offset, pageSize);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Google Earth preparation unavailable; no data, audits, preferences or files written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  async function publishKML() {
    if (locked || !approval) { error = 'Prepare an owned KML file review and acknowledge any prior outcome first.'; return; }
    error = '';
    try {
      await publicationSession.publish(approval, destination, request =>
        GoogleEarthKMLExportService.ExportReviewedGoogleEarthKML(contextId, JSON.stringify(request)));
    } catch (cause) {
      error = `KML publication request unavailable: ${String(cause)}`;
    }
  }
  function acknowledgePublication() {
    publicationSession.acknowledge(); approval = null; kml = null;
  }
  onDestroy(cancel);
  onDestroy(unsubscribe);
  onDestroy(() => unsubscribePreferences?.());
</script>

<section class="earth-review" data-google-earth-review aria-label="Google Earth location preparation">
  <h1>Google Earth location preparation</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if preferences?.draftError}<p class="error" role="alert" data-google-earth-preferences-draft-error>{preferences.draftError}</p>{/if}
  {#if preferencesEnabled && preferences}
    <section data-google-earth-preferences aria-label="Reviewed Google Earth preferences">
      <button type="button" data-google-earth-preferences-load disabled={locked} onclick={loadPreferences}>Load saved preferences (apply to live fields)</button>
      <button type="button" data-google-earth-preferences-save disabled={locked || !preferences.saved || !!preferences.draftError} onclick={savePreferences}>Save reviewed preferences</button>
      <button type="button" data-google-earth-preferences-undo disabled={locked || !preferences.saved} onclick={undoPreferences}>Undo preference draft</button>
      {#if preferences.attempted}
        <div data-google-earth-preferences-outcome>
        {#if preferences.busy}<p role="status">Saving reviewed preferences. This operation cannot be cancelled.</p>
        {:else if preferences.outcome}
          <p role={preferences.error ? 'alert' : 'status'}>Preference outcome:
            {preferences.outcome.committed ? 'committed; do not replay' : preferences.error ? 'not committed' : 'unchanged'}.
            {preferences.error}</p>
        {:else}<p role="alert">{preferences.error}</p>{/if}
        <button type="button" data-google-earth-preferences-acknowledge disabled={preferences.busy || !preferences.blocked}
          onclick={() => preferenceSession?.acknowledge()}>Acknowledge preference outcome</button>
        </div>
      {/if}
    </section>
  {/if}
  {#if publication.requestedDestination}
    <section data-google-earth-kml-outcome aria-label="KML file publication outcome">
      {#if publication.busy}
        <p role="status">Publishing the reviewed KML file. Wait for its outcome; publication cannot be cancelled or replayed.</p>
      {:else if publication.outcome}
        <p role={publication.outcome.status === 'published' ? 'status' : 'alert'}>
          KML file outcome: {publication.outcome.status}. {publication.outcome.errorMessage}
          {#if publication.outcome.status !== 'not-published'}Do not repeat this publication.{/if}
        </p>
        {#if publication.outcome.path}<p>Observed destination: {publication.outcome.path}</p>{/if}
        {#if publication.outcome.sha256}<p>SHA256: {publication.outcome.sha256}</p>{/if}
      {:else if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
      <p>Requested destination: {publication.requestedDestination}</p>
      {#if publication.blocked && !publication.busy}
        <button type="button" data-google-earth-kml-acknowledge onclick={acknowledgePublication}>Acknowledge outcome before a new review</button>
      {/if}
    </section>
  {/if}
  <p>Project {project} / SU {su}. Direct Env scope: no Admin, current-plot or profile filter.</p>
  <dl><div><dt>Owned project database</dt><dd>{projectPath}</dd></div>
    {#if su !== 'None'}<div><dt>Owned SU database</dt><dd>{suPath}</dd></div>{/if}</dl>
  <div class="actions">
    <button type="button" data-google-earth-fields disabled={locked} onclick={loadFields}>Load description fields</button>
    {#if busy}<button type="button" data-google-earth-cancel onclick={cancel}>Cancel preparation read</button>
      <p role="status">Reading and checking the owned snapshot...</p>{/if}
  </div>
  <div class="options">
  {#if kmlExportEnabled}
    <div class="field">
      <label for="google-earth-kml-destination">New output file:</label>
      <textarea id="google-earth-kml-destination" rows="2" disabled={locked} bind:value={destination}></textarea>
    </div>
  {/if}
  {#if kmlEnabled || kmlExportEnabled || preferencesEnabled}
    <div class="field" data-google-earth-kml-controls>
      <label for="google-earth-place-name">Place Name:</label>
      <textarea id="google-earth-place-name" rows="2" disabled={locked} bind:value={title} oninput={changeTitle}></textarea>
    </div>
  {/if}
  <div class="field">
    <label for="google-earth-description">Description field:</label>
    <select id="google-earth-description" disabled={locked || !fields} bind:value={selectedField} onchange={changeField}>
      <option value="">Choose a literal Env field</option>
      {#if preferencesEnabled && selectedField && !fields?.fields.some(field => field.name === selectedField)}
        <option value={selectedField}>{selectedField} (saved literal; physical availability not established)</option>
      {/if}
      {#each fields?.fields ?? [] as field (field.name)}<option value={field.name}>{field.name}</option>{/each}
    </select>
  </div>
  </div>
  {#if preferencesEnabled}
    <p class="notice">Only explicit Load applies saved literals. Save changes shared ReportOptions preferences only, not source approvals or files.
      Saved fields may be unavailable or unsupported for KML; they are retained, never substituted.
      Live description literal: “{selectedField}”. Physical membership alone does not establish KML support; KML requires TEXT/NULL description cells.
      {#if selectedField && !fields?.fields.some(field => field.name === selectedField)}
        Current literal “{selectedField}” is not an established current physical field; export remains unavailable until physical membership is verified.
      {/if}
    </p>
  {/if}
  <div class="actions">
    <button type="button" data-google-earth-read disabled={locked || !selectedField || !fields} onclick={() => show()}>Review Google Earth locations</button>
  </div>
  {#if kmlEnabled || kmlExportEnabled}
    <div class="actions">
      <button type="button" data-google-earth-kml-prepare disabled={locked || !selectedField || !fields || !googleEarthXMLText(title)}
        onclick={prepareKML}>{kmlExportEnabled ? 'Review KML file export (no file yet)' : 'Prepare KML text (no file)'}</button>
      {#if kmlExportEnabled}
        <button type="button" data-google-earth-kml-publish
          disabled={locked || !approval || !destination || !wellFormedUTF16(destination) || destination.includes('\0')}
          onclick={publishKML}>Build reviewed KML file (never replace)</button>
      {/if}
    </div>
    {#if kml}
      <p role="status" data-google-earth-kml-summary>{kml.placemarkCount} placemarks; {kml.byteCount} UTF-8 bytes from a new complete owned snapshot.
        {#if preferencesEnabled}Text preparation itself creates no file, saves no preference and launches no viewer; explicit preference saves and file publications have separate retained outcomes.
        {:else if !publication.requestedDestination}No file created, preference saved or viewer launched.
        {:else}Text preparation itself creates no file; the separate publication outcome is retained above.{/if}</p>
      <label for="google-earth-kml-text">Prepared KML text:</label>
      <textarea id="google-earth-kml-text" readonly rows="10" value={kml.kml}></textarea>
    {/if}
    <p class="notice">{preferencesEnabled ? 'Place Name and description are live drafts; only explicit Load applies saved preferences and explicit Save persists them.' : 'Place Name is temporary; the visible initial text is the source default, not a saved preference.'}
      TEXT/NULL descriptions only; NULL contributes empty text plus the source period.
      Finite map coordinate bounds apply only to KML preparation. HTML/XML text is escaped and remote icons are omitted.
      XML shape is not schema, viewer or native Access acceptance; viewers may automatically link URLs.</p>
    {#if kmlExportEnabled}<p class="notice">Choose an explicit absolute new file path. Existing files are never replaced and no extension is added.
      Review binds original raw cells and physical rows, not only XML. Publication is not cancellable; acknowledge its retained outcome before another review.
      File publication leaves data, audits and preferences unchanged. Remote icons and viewer launch remain unavailable.</p>{/if}
  {/if}
  {#if review}
    <p role="status" data-google-earth-summary>{review.totalRows} physical Env/SU pairs with both coordinates present.
      Duplicate memberships remain separate.
      {#if preferencesEnabled}This location review writes no data, audits, preferences or files; explicit preference saves have separate retained outcomes.
      {:else}No data, audits, preferences or files written.{/if}</p>
    <div class="table-scroll">
      <table>
        <caption>Direct Env preparation; numeric-negated longitude then original latitude, altitude not yet serialized</caption>
        <thead><tr>{#each review.fields as field, i}<th id={`earth-field-${i}`} scope="col">{field.label}</th>{/each}</tr></thead>
        <tbody>{#each review.rows as row (`${row.envRowId}:${row.membershipRowId}`)}
          <tr data-google-earth-row={`${row.envRowId}:${row.membershipRowId}`}>
            {#each [row.plotNumber, row.longitude, row.latitude, row.description] as cell, i}
              <td headers={`earth-field-${i}`} aria-label={`${review.fields[i].label} for Env row ${row.envRowId}, SU row ${row.membershipRowId || 'not selected'}`}>{reportCellText(cell)}</td>
            {/each}
          </tr>
        {:else}<tr><td colspan="4">No coordinate pairs in this page. No coordinates inferred.</td></tr>{/each}</tbody>
      </table>
    </div>
    <div class="actions" aria-label="Google Earth preparation pages">
      <button type="button" disabled={locked || review.offset === 0} onclick={() => show(previousOffset)}>Previous pairs</button>
      <span>{review.rows.length ? review.offset + 1 : 0}-{review.rows.length ? review.offset + review.rows.length : 0} of {review.totalRows}</span>
      <button type="button" disabled={locked || review.offset + review.rows.length >= review.totalRows} onclick={() => show(nextOffset)}>Next pairs</button>
    </div>
    <details><summary>Physical provenance for this page</summary>
      {#each review.rows as row (`${row.envRowId}:${row.membershipRowId}`)}
        <p>Env row {row.envRowId}; SU row {row.membershipRowId || 'not selected'};
          stored longitude {reportCellText(row.storedLongitude)}.</p>
      {/each}
    </details>
  {/if}
  <p class="notice">Read-only migration preview. Description selection does not save a preference.
    NULL, empty and non-text values remain distinguishable. Historical coordinate ranges are retained, not declared map-ready.
    Saved Place Name, remote icons and Google Earth launch remain unavailable.
    {#if !kmlExportEnabled}Output file and KML/XML publication remain unavailable.{/if}</p>
</section>

<style>
  .earth-review { padding: 1rem; min-width: 0; max-width: 100%; }
  h1 { font-size: 1.5rem; margin-bottom: .75rem; }
  .actions { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; margin: 1rem 0; }
  .field { display: grid; grid-template-columns: minmax(10rem, auto) minmax(0, 30rem); align-items: center; gap: .5rem; }
  .options { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 30rem), 1fr)); gap: 1rem; }
  label, dt { font-weight: 600; }
  select, textarea, button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; min-width: 0; }
  select, textarea { width: 100%; }
  button:disabled { opacity: .5; }
  dl > div { margin: .6rem 0; }
  dd, p { overflow-wrap: anywhere; }
  .table-scroll { overflow-x: auto; max-width: 100%; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #cbd5e1; padding: .4rem .6rem; text-align: left; vertical-align: top; white-space: pre-wrap; }
  th { background: #f1f5f9; }
  caption { text-align: left; margin-bottom: .5rem; }
  .notice { color: #475569; }
  .error { color: #991b1b; }
  @media (max-width: 40rem) { .field { grid-template-columns: minmax(0, 1fr); } }
</style>
