<script lang="ts">
  import { untrack } from 'svelte';
  import { CheckCircle2, Newspaper, X } from '@lucide/svelte';
  import { defaultHelpPort, type HelpBackendPort, type WhatsNewStatus } from './helpSystem';

  let {
    open = false,
    initialStatus = null,
    helpPort = defaultHelpPort,
    onstatuschange,
    onclose,
  }: {
    open?: boolean;
    initialStatus?: WhatsNewStatus | null;
    helpPort?: HelpBackendPort;
    onstatuschange?: (status: WhatsNewStatus) => void;
    onclose: () => void;
  } = $props();

  let status = $state.raw<WhatsNewStatus | null>(untrack(() => initialStatus));
  let loading = $state(false);
  let acknowledging = $state(false);
  let error = $state('');
  let loadGen = 0;

  $effect(() => {
    const incoming = initialStatus;
    untrack(() => {
      if (incoming) status = incoming;
    });
  });

  $effect(() => {
    if (!open) return;
    untrack(() => {
      void refreshStatus();
    });
  });

  async function refreshStatus() {
    const gen = ++loadGen;
    loading = true;
    error = '';
    try {
      const fresh = await helpPort.getWhatsNewStatus();
      if (gen !== loadGen) return;
      status = fresh;
      onstatuschange?.(fresh);
    } catch (cause) {
      if (gen === loadGen) {
        error = `Release notes status could not be loaded: ${String(cause)}`;
      }
    } finally {
      if (gen === loadGen) {
        loading = false;
      }
    }
  }

  async function markLatestAsRead() {
    if (!status?.latestVersion || acknowledging) return;
    acknowledging = true;
    error = '';
    try {
      const updated = await helpPort.acknowledgeWhatsNew(status.latestVersion);
      status = updated;
      onstatuschange?.(updated);
    } catch (cause) {
      error = `Could not persist What's New acknowledgement: ${String(cause)}`;
    } finally {
      acknowledging = false;
    }
  }

  function isEntryUnread(index: number): boolean {
    if (!status) return false;
    return index < status.unreadCount;
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
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-whats-new-modal>
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="whats-new-modal-title"
      class="flex max-h-[86vh] w-full max-w-2xl flex-col overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
    >
      <header class="flex items-center justify-between gap-3 border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-3.5 text-white">
        <div class="flex items-center gap-2.5">
          <Newspaper size={18} class="text-[#FCBA19]" />
          <div>
            <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">
              Release &amp; Standards Changelog
            </div>
            <h2 id="whats-new-modal-title" class="text-base font-semibold text-white">
              What's New in VPro
            </h2>
          </div>
        </div>
        <button
          type="button"
          aria-label="Close What's New"
          data-whats-new-close
          class="rounded border border-white/30 bg-white/10 p-1.5 text-white hover:bg-white/20"
          onclick={onclose}
        >
          <X size={16} />
        </button>
      </header>

      <div class="space-y-4 overflow-y-auto p-5 text-xs">
        {#if error}
          <p role="alert" class="rounded border border-[#D8292F] bg-red-50 p-2.5 text-[#D8292F]">{error}</p>
        {/if}

        {#if status}
          <div class="flex flex-wrap items-center justify-between gap-2 rounded border border-slate-300 bg-[#F2F2F2] p-3">
            <div>
              {#if status.hasUnread}
                <span class="font-semibold text-[#003366]" data-whats-new-unread-banner>
                  {status.unreadCount} unread release update{status.unreadCount === 1 ? '' : 's'}
                  {status.lastSeenVersion ? ` since v${status.lastSeenVersion}` : ''}.
                </span>
              {:else}
                <span class="inline-flex items-center gap-1.5 font-medium text-[#2E8540]" data-whats-new-all-read>
                  <CheckCircle2 size={14} /> Up to date — acknowledged through v{status.latestVersion}.
                </span>
              {/if}
            </div>
            {#if status.hasUnread}
              <button
                type="button"
                data-whats-new-acknowledge
                disabled={acknowledging}
                class="inline-flex items-center gap-1.5 rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
                onclick={() => void markLatestAsRead()}
              >
                <CheckCircle2 size={13} />
                {acknowledging ? 'Saving...' : `Mark v${status.latestVersion} as Read`}
              </button>
            {/if}
          </div>

          <div class="space-y-3">
            {#each status.entries as entry, index (entry.version)}
              <article
                data-whats-new-version={entry.version}
                class="space-y-2 rounded border p-3.5 {isEntryUnread(index) ? 'border-[#003366] border-l-[4px] border-l-[#FCBA19] bg-amber-50/20' : 'border-slate-200 bg-white'}"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="flex items-center gap-2">
                    <h3 class="font-mono text-sm font-bold text-[#003366]">Version {entry.version}</h3>
                    <span class="rounded border border-slate-300 bg-[#F2F2F2] px-2 py-0.5 font-mono text-[10px] uppercase text-slate-700">
                      {entry.channel}
                    </span>
                    {#if isEntryUnread(index)}
                      <span class="rounded bg-[#FCBA19] px-2 py-0.5 font-mono text-[10px] font-bold uppercase text-[#003366]">
                        Unread
                      </span>
                    {/if}
                  </div>
                  <span class="font-mono text-[11px] text-slate-600">{entry.date}</span>
                </div>

                <ul class="list-disc space-y-1.5 pl-4 text-xs leading-relaxed text-[#313132]">
                  {#each entry.highlights as item}
                    <li>{item}</li>
                  {/each}
                </ul>
              </article>
            {/each}
          </div>
        {:else if loading}
          <p role="status" class="text-slate-600">Loading release changelog...</p>
        {/if}
      </div>
    </div>
  </div>
{/if}
