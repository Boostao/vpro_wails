<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageGoogleEarthService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { googleEarthXMLText } from './googleEarthKMLReview';
  import { metadataCellText } from './projectMetadataEditor';
  import { twoPageGoogleEarthOptionsFromWire, twoPageGoogleEarthReviewFromWire, twoPageGoogleEarthCell,
    type TwoPageGoogleEarthOwner, type TwoPageGoogleEarthOptionsView, type TwoPageGoogleEarthReviewView } from './twoPageGoogleEarth';

  let { contextId, project, projectPath, plot, form, onBusyChange }: TwoPageGoogleEarthOwner & {
    onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, projectPath, plot, form }));
  let options = $state.raw<TwoPageGoogleEarthOptionsView | null>(null);
  let review = $state.raw<TwoPageGoogleEarthReviewView | null>(null);
  let title = $state('');
  let descriptionField = $state('');
  let busy = $state(false);
  let error = $state('');
  let generation = 0;
  const reads = new ReadRequests();
  const validOptions = $derived(!!options && !optionError());

  function optionError() {
    if (!googleEarthXMLText(title)) return 'Place Name requires valid Unicode and XML text; no value repaired.';
    if (!options?.fields.some(field => field.name === descriptionField)) return 'Description Field requires a current literal physical Env field; no binding repaired.';
    return '';
  }
  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    onBusyChange(false);
  }
  function editTitle(event: Event) {
    title = (event.currentTarget as HTMLTextAreaElement).value;
    cancel(); review = null;
    error = optionError();
  }
  function editField(event: Event) {
    descriptionField = (event.currentTarget as HTMLSelectElement).value;
    cancel(); review = null; error = optionError();
  }
  async function loadOptions() {
    if (busy) return;
    cancel(); const request = generation;
    options = null; review = null; title = ''; descriptionField = ''; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(TwoPageGoogleEarthService.GetOptions(owner.contextId, owner.plot, owner.form));
      if (request !== generation) return;
      options = twoPageGoogleEarthOptionsFromWire(value, owner);
      title = options.title; descriptionField = options.descriptionField;
      if (!googleEarthXMLText(title) || !options.fields.some(field => field.name === descriptionField)) {
        error = 'Saved options retained literally but unavailable for KML. Correct preview options explicitly; saved preferences remain unchanged.';
      }
    } catch (cause) {
      if (request === generation) error = `Saved-plot options unavailable; no defaults or writes applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  async function prepare() {
    if (busy) return;
    if (!validOptions) { error = 'Load saved options and correct the literal description field and Place Name before preparing KML.'; return; }
    cancel(); const request = generation, field = descriptionField, name = title;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(TwoPageGoogleEarthService.GetReview(owner.contextId, owner.plot, owner.form,
        JSON.stringify({ descriptionField: field, title: name })));
      if (request !== generation) return;
      const validated = await twoPageGoogleEarthReviewFromWire(value, owner, field, name);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Saved-plot KML unavailable; data, history, preferences and files unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  function label(column: string) {
    if (!review) throw new Error('Saved-plot KML original is unavailable.');
    const cell = twoPageGoogleEarthCell(review, column);
    return cell.storage === 'null' ? '(NULL)' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell);
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="Saved single-plot Google Earth KML"
  data-two-page-google-earth-panel data-source-parent-form={owner.form}>
  <h3 class="font-semibold">Plot on Google Earth: saved-plot KML review</h3>
  <p class="break-words">Owned project {owner.project}; literal plot {JSON.stringify(owner.plot)}.</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={loadOptions}>Load saved Google Earth options</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel saved-plot read</button><p role="status">Reading owned saved plot...</p>{/if}
  </div>
  {#if options}
    <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
      <label class="min-w-0">Place Name (preview only)
        <textarea class="box-border w-full min-w-0" rows="2" value={title} disabled={busy} oninput={editTitle}></textarea>
      </label>
      <label class="min-w-0">Description Field (preview only)
        <select class="box-border w-full min-w-0" value={descriptionField} disabled={busy} onchange={editField}>
          {#if !options.fields.some(field => field.name === descriptionField)}
            <option value={descriptionField} disabled>Unavailable saved field: {JSON.stringify(descriptionField)}</option>
          {/if}
          {#each options.fields as field (field.name)}<option value={field.name}>{field.name}</option>{/each}
        </select>
      </label>
    </div>
    <button type="button" disabled={busy || !validOptions} onclick={prepare}>Prepare saved-plot KML</button>
  {/if}
  {#if review}
    <p role="status">One saved placemark; {review.byteCount} UTF-8 bytes. Original physical Env row {review.original.rows[0].rowId}.</p>
    <dl class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
      {#each ['Longitude', 'Latitude', review.descriptionField] as column, index}
        <div class="min-w-0" data-saved-plot-column={`${index}:${column}`}>
          <dt class="font-medium">{index === 0 ? 'Longitude (stored west-positive)' : index === 1 ? 'Latitude' : `Description (${column})`}</dt>
          <dd class="whitespace-pre-wrap break-words">{label(column)}</dd>
        </div>
      {/each}
    </dl>
    <label class="block min-w-0">Saved-plot KML (read only)
      <textarea class="box-border w-full min-w-0" rows="10" readonly value={review.kml} data-saved-plot-kml></textarea>
    </label>
  {/if}
  <p class="text-xs text-stone-600">Explicit preview of saved data, not an implicit Save.
    Temporary preview options never change saved preferences. Longitude retains its original storage and is negated only in KML.
    Configured-file overwrite, publication, remote icons and external Google Earth launch are unavailable.</p>
</section>
