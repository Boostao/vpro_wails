<script lang="ts">
  import type { WhatsNewStatus } from '../help/helpSystem';

  let {
    region = 'banners',
    buildMode = '',
    appVersion = '',
    shortBuildCommit = '',
    releaseChannel = 'stable',
    startupFailed = false,
    whatsNewStatus = null,
    whatsNewPromptDismissed = false,
    resetSampleNotice = '',
    diagnostics = [],
    onopenabout = () => {},
    onopenwhatsnew = () => {},
    onacknowledgewhatsnew = () => {},
    ondismisswhatsnew = () => {},
    ondismissresetnotice = () => {},
  }: {
    region?: 'badge' | 'banners';
    buildMode?: string;
    appVersion?: string;
    shortBuildCommit?: string;
    releaseChannel?: string;
    startupFailed?: boolean;
    whatsNewStatus?: WhatsNewStatus | null;
    whatsNewPromptDismissed?: boolean;
    resetSampleNotice?: string;
    diagnostics?: Array<{ file: string; message: string }>;
    onopenabout?: () => void;
    onopenwhatsnew?: () => void;
    onacknowledgewhatsnew?: () => void;
    ondismisswhatsnew?: () => void;
    ondismissresetnotice?: () => void;
  } = $props();
</script>

{#if region === 'badge'}
  {#if buildMode === 'dev'}
    <button type="button" data-build-tier-badge="dev" title="Local developer build — open About VPro & diagnostics"
      class="ml-1.5 rounded border border-[#FCBA19] bg-[#003366] px-2 py-0.5 font-mono text-[11px] font-semibold tracking-tight text-[#FCBA19] hover:bg-[#002244]"
      onclick={() => { if (!startupFailed) onopenabout(); }}>
      DEV BUILD · v{appVersion || '2.0.1-dev'}{shortBuildCommit ? ` · ${shortBuildCommit}` : ''}
    </button>
  {:else if buildMode === 'alpha'}
    <button type="button" data-build-tier-badge="alpha" title="Alpha preview build — open About VPro & verification details"
      class="ml-1.5 rounded border border-[#003366] bg-[#FCBA19] px-2 py-0.5 font-mono text-[11px] font-bold tracking-tight text-[#003366] hover:brightness-95"
      onclick={() => { if (!startupFailed) onopenabout(); }}>
      ALPHA PREVIEW (v{appVersion})
    </button>
  {:else if buildMode === 'release' && releaseChannel === 'experimental'}
    <button type="button" data-build-tier-badge="experimental" title="Experimental release channel enabled in config.yml"
      class="ml-1.5 rounded border border-white/40 bg-white/10 px-2 py-0.5 font-mono text-[11px] font-semibold uppercase tracking-tight text-white hover:bg-white/20"
      onclick={() => { if (!startupFailed) onopenabout(); }}>
      EXPERIMENTAL CHANNEL
    </button>
  {/if}
{:else}
  {#if !startupFailed && whatsNewStatus?.hasUnread && !whatsNewPromptDismissed}
    <section aria-label="Unread release notes" data-whats-new-banner
      class="mb-4 flex flex-wrap items-center justify-between gap-2 rounded border border-[#003366] border-l-[4px] border-l-[#FCBA19] bg-white px-3.5 py-2.5 text-xs text-[#313132]">
      <div>
        <strong class="font-semibold text-[#003366]">What's New in VPro v{whatsNewStatus.latestVersion}:</strong>
        <span> {whatsNewStatus.unreadCount} unread release update{whatsNewStatus.unreadCount === 1 ? '' : 's'} available.</span>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="rounded bg-[#003366] px-2.5 py-1 text-xs font-semibold text-white hover:bg-[#002244]" onclick={onopenwhatsnew}>Read Release Notes</button>
        <button type="button" class="rounded border border-[#003366] bg-white px-2.5 py-1 text-xs font-semibold text-[#003366] hover:bg-slate-50" onclick={onacknowledgewhatsnew}>Mark as Read</button>
        <button type="button" class="rounded border border-slate-300 bg-white px-2 py-1 text-xs text-slate-600 hover:bg-slate-50" onclick={ondismisswhatsnew}>Dismiss</button>
      </div>
    </section>
  {/if}
  {#if resetSampleNotice}
    <div role="status" data-reset-sample-notice class="mb-4 flex items-center justify-between gap-2 rounded border border-[#2E8540] bg-emerald-50 px-3.5 py-2 text-xs text-[#1E5128]">
      <span>{resetSampleNotice}</span>
      <button type="button" class="rounded border border-emerald-700/30 bg-white px-2 py-0.5 text-[11px]" onclick={ondismissresetnotice}>Dismiss</button>
    </div>
  {/if}
  {#if diagnostics.length > 0}
    <div class="project-warning" role="status">
      <strong>Some project files could not be opened.</strong>
      {#each diagnostics as diagnostic (diagnostic.file)}
        <div>{diagnostic.file}: {diagnostic.message}</div>
      {/each}
    </div>
  {/if}
{/if}
