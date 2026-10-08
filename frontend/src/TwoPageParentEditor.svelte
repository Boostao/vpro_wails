<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { TwoPageParentExtraService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import type { EditorCloseState } from './closeLifecycle';
  import TwoPageParentFields from './TwoPageParentFields.svelte';
  import TwoPageEntryLayout from './TwoPageEntryLayout.svelte';
  import { twoPageEntryValues, type TwoPageEntryChild } from './twoPageEntryProjection';
  import type { PaperControl } from './paperLayout';
  import { TwoPageParentSession, type TwoPageParentForm, type TwoPageParentColumn,
    type TwoPageParentInput, type TwoPageParentSessionCache } from './twoPageParentSession';

  let { contextId, project, plot, cache, revision, onchange, onbusy, onclosed, oncommitted, embedded: childContent }: {
    contextId: string; project: string; plot: string;
    cache: TwoPageParentSessionCache; revision: number; onchange: () => void;
    onbusy: (value: boolean) => void; onclosed: () => void;
    oncommitted: () => Promise<void>;
    embedded?: Snippet<[TwoPageEntryChild, PaperControl]>;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot }));
  const reviewEnabled = import.meta.env.VITE_TWO_PAGE_PARENT_REVIEW === 'true';
  const editingEnabled = reviewEnabled && import.meta.env.VITE_TWO_PAGE_PARENT_EXTRA_EDITING === 'true';
  const sourceLayoutEnabled = reviewEnabled && import.meta.env.VITE_TWO_PAGE_PARENT_SOURCE_LAYOUT === 'true';
  const reads = new ReadRequests();
  const key = JSON.stringify(owner);
  const sessions = untrack(() => {
    const owned = cache.get(key) ?? new Map<TwoPageParentForm, TwoPageParentSession>();
    cache.set(key, owned);
    return owned;
  });
  let form = $state<TwoPageParentForm>('FS882-8x6XL');
  let error = $state<string | null>(null);
  let session = $state(makeSession('FS882-8x6XL'));
  const view = $derived.by(() => { revision; return session.view(); });
  const close = $derived.by(() => { revision; return session.closeState(); });

  function makeSession(sourceForm: TwoPageParentForm) {
    const existing = sessions.get(sourceForm);
    if (existing) return existing;
    const next = new TwoPageParentSession(owner, sourceForm, {
      read: () => reads.track(TwoPageParentExtraService.GetOriginal(owner.contextId, owner.plot, sourceForm)),
      cancelRead: () => reads.cancelAll(),
      save: request => TwoPageParentExtraService.Save(owner.contextId, owner.plot, sourceForm, request),
      restore: (history, action) => TwoPageParentExtraService.Restore(owner.contextId, owner.plot, sourceForm, history, action),
      refreshParent: async () => {
        await oncommitted();
        for (const [variant, peer] of sessions) {
          if (variant === sourceForm || !peer.view().original) continue;
          if (peer.closeState().unsaved || peer.closeState().busy || !await peer.load()) {
            throw new Error(peer.view().error ?? 'Another two-page source owner could not be refreshed; no write was replayed.');
          }
        }
      },
    }, onchange);
    sessions.set(sourceForm, next);
    return next;
  }
  $effect(() => { onbusy(view.busy); });
  onDestroy(() => { reads.cancelAll(); });

  export function getCloseState(): EditorCloseState {
    return { ...close, unsaved: true, canSave: false, error: error ?? view.error,
      saveReason: 'Save or Undo additional fields explicitly, then close their review before saving or closing the plot.' };
  }
  export function undo() { void operation('undo'); }

  async function chooseForm(next: TwoPageParentForm) {
    if (close.unsaved || close.busy || close.blocked) {
      error = 'Finish or Undo the current source variant before changing its owner.';
      return;
    }
    form = next;
    session = makeSession(next);
    onchange(); error = null;
    await operation('load');
  }
  function stage(column: TwoPageParentColumn, input: TwoPageParentInput) {
    try {
      if (!editingEnabled) throw new Error('Additional-field editing is independently disabled.');
      session.stage(column, input);
      error = session.view().error;
    } catch (cause) { error = `Additional-field draft failed; owned state retained: ${String(cause)}`; }
  }
  async function operation(action: 'load' | 'save' | 'undo' | 'retain' | 'prune') {
    try {
      if (!reviewEnabled || action !== 'load' && action !== 'undo' && !editingEnabled) {
        throw new Error('Two-page review/editing is independently disabled.');
      }
      error = null;
      const ok = action === 'load' ? await session.load() : action === 'save' ? await session.save()
        : action === 'undo' ? await session.undo() : await session.restore(
          action === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) error = session.view().error;
    } catch (cause) { error = `Additional-field operation failed; owned state retained: ${String(cause)}`; }
  }
  function cancel() {
    try { session.cancel(); error = session.view().error; }
    catch (cause) { error = `Additional-field cancellation refused: ${String(cause)}`; }
  }
  function closeReview() {
    if (close.unsaved || close.busy || close.blocked) {
      error = close.saveReason || 'Save or Undo additional fields before closing their review.';
      return;
    }
    onclosed();
  }
</script>

<section class="space-y-3" aria-label="Two-page source review">
  {#if error}<p role="alert" class="text-sm text-red-700">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={close.unsaved || close.busy || close.blocked}
      aria-pressed={form === 'FS882-8x6XL'} onclick={() => chooseForm('FS882-8x6XL')}>Normal two-page source</button>
    <button type="button" disabled={close.unsaved || close.busy || close.blocked}
      aria-pressed={form === 'FS882-8x6XL-CHARS'} onclick={() => chooseForm('FS882-8x6XL-CHARS')}>CHARS two-page source</button>
    <button type="button" disabled={close.unsaved || close.busy || close.blocked} onclick={closeReview}>Close additional-field review</button>
  </div>
  <TwoPageParentFields {form} {view} disabled={!editingEnabled} readDisabled={!reviewEnabled}
    canSave={editingEnabled && close.canSave} onstage={stage}
    onoperation={action => void operation(action)} oncancel={cancel}
    children={sourceLayoutEnabled ? completeLayout : undefined} />
</section>

{#snippet completeLayout(editor: { columns: readonly string[]; input: Snippet<[PaperControl, string]>; labelled: true })}
  {#if view.original}
    <p class="text-sm">Additional fields are editable in this source layout. Other parent fields and linked records
      remain read-only during this review; finish and close this scope before editing another.</p>
    <TwoPageEntryLayout {form} values={twoPageEntryValues(view.original, form)} editors={[{ ...editor, scope: 'extra' }]}>
      {#snippet identity(field)}
        <label class="form-field"><span class="field-label">Plot number</span>
          <input data-column={field.column} value={owner.plot} readonly aria-label="Plot number"
            title="Existing plot identities cannot be renamed here" />
        </label>
      {/snippet}
      {#snippet embedded(child, control)}
        {#if childContent}{@render childContent(child, control)}
        {:else}<p>{child.form}: linked records are unavailable in this review.</p>{/if}
      {/snippet}
    </TwoPageEntryLayout>
  {/if}
{/snippet}
