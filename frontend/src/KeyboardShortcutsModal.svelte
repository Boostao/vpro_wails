<script lang="ts">
  import { untrack } from 'svelte';
  import { Keyboard, X } from '@lucide/svelte';
  import { defaultHelpPort, type HelpBackendPort, type HelpShortcut } from './helpSystem';

  let {
    open = false,
    helpPort = defaultHelpPort,
    onclose,
  }: {
    open?: boolean;
    helpPort?: HelpBackendPort;
    onclose: () => void;
  } = $props();

  let shortcuts = $state.raw<HelpShortcut[]>([]);
  let loading = $state(false);
  let error = $state('');
  let loadGen = 0;

  $effect(() => {
    if (!open) return;
    untrack(() => {
      void ensureShortcuts();
    });
  });

  async function ensureShortcuts() {
    if (shortcuts.length > 0) return;
    const gen = ++loadGen;
    loading = true;
    error = '';
    try {
      const bundle = await helpPort.getHelpBundle();
      if (gen !== loadGen) return;
      shortcuts = bundle.shortcuts ?? [];
    } catch (cause) {
      if (gen === loadGen) {
        error = `Keyboard shortcuts failed to load: ${String(cause)}`;
      }
    } finally {
      if (gen === loadGen) {
        loading = false;
      }
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      onclose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-shortcuts-modal>
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="shortcuts-modal-title"
      class="flex max-h-[85vh] w-full max-w-xl flex-col overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
    >
      <header class="flex items-center justify-between gap-3 border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-3.5 text-white">
        <div class="flex items-center gap-2.5">
          <Keyboard size={18} class="text-[#FCBA19]" />
          <div>
            <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">
              High-Speed Plot &amp; Datasheet Entry
            </div>
            <h2 id="shortcuts-modal-title" class="text-base font-semibold text-white">
              VPro Keyboard Shortcuts Cheat-Sheet
            </h2>
          </div>
        </div>
        <button
          type="button"
          aria-label="Close Keyboard Shortcuts"
          data-shortcuts-close
          class="rounded border border-white/30 bg-white/10 p-1.5 text-white hover:bg-white/20"
          onclick={onclose}
        >
          <X size={16} />
        </button>
      </header>

      <div class="space-y-3 overflow-y-auto p-5 text-xs">
        {#if error}
          <p role="alert" class="rounded border border-[#D8292F] bg-red-50 p-2.5 text-[#D8292F]">{error}</p>
        {/if}
        {#if loading}
          <p role="status" class="text-slate-600">Loading keyboard shortcuts...</p>
        {:else}
          <div class="overflow-x-auto rounded border border-slate-200">
            <table class="w-full min-w-0 border-collapse text-xs">
              <thead>
                <tr class="bg-[#F2F2F2] text-left text-[11px] font-bold uppercase text-[#003366]">
                  <th class="border-b border-slate-300 px-3 py-2">Accelerator</th>
                  <th class="border-b border-slate-300 px-3 py-2">Scope</th>
                  <th class="border-b border-slate-300 px-3 py-2">Action</th>
                </tr>
              </thead>
              <tbody>
                {#each shortcuts as item (item.keys)}
                  <tr class="border-b border-slate-100 last:border-b-0">
                    <td class="whitespace-nowrap px-3 py-2">
                      <kbd class="rounded border border-slate-300 bg-[#F2F2F2] px-2 py-0.5 font-mono text-[11px] font-bold text-[#003366]">
                        {item.keys}
                      </kbd>
                    </td>
                    <td class="whitespace-normal px-3 py-2 font-medium text-slate-700">{item.scope}</td>
                    <td class="whitespace-normal px-3 py-2 text-[#313132]">{item.action}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}

        <div class="flex items-center justify-between border-t border-slate-200 pt-3 text-[11px] text-slate-600">
          <span>Tip: Press <kbd class="rounded border border-slate-300 bg-[#F2F2F2] px-1.5 py-0.5 font-mono">F1</kbd> inside any FS882, Two-Page, Soil, or SIVI field to inspect its LMH25 rule.</span>
          <button
            type="button"
            class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
            onclick={onclose}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
