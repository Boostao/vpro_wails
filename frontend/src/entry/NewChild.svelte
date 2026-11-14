<script lang="ts">
  import { onMount } from 'svelte';
  let { kind, busy, error, oncreate, oncancel }: {
    kind: 'humus' | 'mineral' | 'other'; busy: boolean;
    error: string | null;
    oncreate: (first: string, second: string) => Promise<void>;
    oncancel: () => void;
  } = $props();
  let first = $state('');
  let second = $state('');
  let dialog: HTMLDialogElement;
  onMount(() => { dialog.showModal(); });
</script>

  <dialog bind:this={dialog} aria-label={`New ${kind} record`}
    oncancel={(event) => { event.preventDefault(); if (!busy) oncancel(); }}>
    <h3>New {kind} record</h3>
    <p>Only currently mapped fields are saved. No values are guessed.</p>
    {#if error}<p role="alert">{error}</p>{/if}
    <label>{kind === 'other' ? 'Data name' : 'Horizon'}
      <input bind:value={first} disabled={busy} aria-label={kind === 'other' ? 'Data name' : 'Horizon'} />
    </label>
    <label>{kind === 'other' ? 'Data item' : 'Comment'}
      <input bind:value={second} disabled={busy} aria-label={kind === 'other' ? 'Data item' : 'Comment'} />
    </label>
    <div class="actions">
      <button disabled={busy} onclick={oncancel}>Cancel</button>
      <button disabled={busy} onclick={() => void oncreate(first, second)}>Create record</button>
    </div>
  </dialog>

<style>
  dialog { margin: auto; background: white; border-radius: .5rem; padding: 1rem; width: min(25rem, calc(100vw - 2rem)); box-sizing: border-box; box-shadow: 0 1rem 3rem #0003; }
  dialog::backdrop { background: #0006; }
  h3 { font-weight: 600; }
  p { margin: .5rem 0; font-size: .75rem; color: #78716c; }
  label { display: block; margin: .5rem 0; font-size: .8rem; }
  input { display: block; box-sizing: border-box; min-height: 40px; width: 100%; padding: .5rem; border: 1px solid #a8a29e; font: inherit; }
  .actions { display: flex; justify-content: flex-end; gap: .5rem; margin-top: .75rem; }
  button { padding: .25rem .5rem; border: 1px solid #a8a29e; border-radius: .25rem; }
</style>
