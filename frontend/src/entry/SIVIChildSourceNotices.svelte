<script lang="ts">
  import type { SIVIChildSourceNotice } from './siviChildSourceNotices';

  let { notices, saved }: { notices: SIVIChildSourceNotice[]; saved: boolean } = $props();
  let focusError = $state<string | null>(null);
  function focusSpecies(id: string) {
    const target = document.getElementById(id);
    if (!target) {
      focusError = 'The read-only Species focus target is unavailable; reopen this panel.';
      return;
    }
    focusError = null;
    target.focus();
  }
</script>

{#if notices.length}
  <aside class="space-y-3 rounded border border-amber-300 bg-amber-50 p-3" aria-label="SIVI source cover warnings" data-sivi-source-notices>
    <h3 class="font-medium">{saved ? 'Source warnings from the acknowledged Save' : 'Source warnings for the planned Save'}</h3>
    {#if focusError}<p role="alert" class="text-sm text-red-700">{focusError}</p>{/if}
    {#each notices as notice (JSON.stringify([notice.form, notice.rowId]))}
      {@const id = `sivi-source-species-${notice.form}-${notice.rowId}`}
      <div class="space-y-1" data-sivi-source-notice={notice.form} data-sivi-source-notice-row={notice.rowId}>
        <p role="status">{notice.message}</p>
        <p id={id} tabindex="-1" class="break-words text-sm focus:outline focus:outline-2" aria-label={`Read-only Species, physical row ${notice.rowId}`}>
          Species: {notice.species}; physical row {notice.rowId}
        </p>
        <button type="button" class="rounded border px-3 py-1 text-sm" data-sivi-source-species-focus
          aria-controls={id} onclick={() => focusSpecies(id)}>Focus read-only Species</button>
      </div>
    {/each}
    <p class="text-xs text-stone-600">Informational, not a Save rejection. Species remains read-only.
      Access event popups and automatic focus are adapted to persistent feedback and an explicit focus action.</p>
  </aside>
{/if}
