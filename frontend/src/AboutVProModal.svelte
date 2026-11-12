<script lang="ts">
  import { untrack } from 'svelte';
  import { CheckCircle2, Copy, FileCheck2, ShieldCheck, X } from '@lucide/svelte';
  import { defaultHelpPort, type AboutInfo, type HelpBackendPort, type HelpDataUseAgreement } from './helpSystem';

  let {
    open = false,
    helpPort = defaultHelpPort,
    onclose,
  }: {
    open?: boolean;
    helpPort?: HelpBackendPort;
    onclose: () => void;
  } = $props();

  let about = $state.raw<AboutInfo | null>(null);
  let agreement = $state.raw<HelpDataUseAgreement | null>(null);
  let loading = $state(false);
  let error = $state('');
  let copiedDigest = $state('');
  let loadGen = 0;

  $effect(() => {
    if (!open) return;
    untrack(() => {
      void loadAboutAndAgreement();
    });
  });

  async function loadAboutAndAgreement() {
    const gen = ++loadGen;
    loading = true;
    error = '';
    try {
      const [aboutRes, bundleRes] = await Promise.all([
        helpPort.getAboutInfo(),
        helpPort.getHelpBundle(),
      ]);
      if (gen !== loadGen) return;
      about = aboutRes;
      agreement = bundleRes.dataUseAgreement ?? null;
    } catch (cause) {
      if (gen === loadGen) {
        error = `About & provenance metadata failed to load: ${String(cause)}`;
      }
    } finally {
      if (gen === loadGen) {
        loading = false;
      }
    }
  }

  async function copyDigest(sha: string) {
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(sha);
      }
      copiedDigest = sha;
    } catch {
      copiedDigest = '';
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
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-about-vpro-modal>
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="about-vpro-modal-title"
      class="flex max-h-[88vh] w-full max-w-2xl flex-col overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
    >
      <header class="flex items-center justify-between gap-3 border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-3.5 text-white">
        <div class="flex items-center gap-2.5">
          <ShieldCheck size={18} class="text-[#FCBA19]" />
          <div>
            <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">
              Institutional Identity &amp; Standards Provenance
            </div>
            <h2 id="about-vpro-modal-title" class="text-base font-semibold text-white">
              About VPro &amp; 2026 Master Standards Libraries
            </h2>
          </div>
        </div>
        <button
          type="button"
          aria-label="Close About VPro"
          data-about-vpro-close
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

        {#if loading && !about}
          <p role="status" class="text-slate-600">Loading standards provenance stamps...</p>
        {:else if about}
          <section class="rounded border border-slate-300 border-l-[4px] border-l-[#003366] bg-[#F2F2F2] p-3.5">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h3 class="text-base font-bold text-[#003366]">{about.productName}</h3>
              <div class="flex items-center gap-2 font-mono text-[11px]">
                <span class="rounded bg-[#003366] px-2 py-0.5 font-semibold text-white">v{about.appVersion}</span>
                <span class="rounded border border-slate-300 bg-white px-2 py-0.5 font-semibold text-[#003366]">
                  Schema {about.schemaVersion}
                </span>
                <span class="rounded border border-slate-300 bg-white px-2 py-0.5 uppercase text-slate-700">
                  Channel: {about.releaseChannel}
                </span>
              </div>
            </div>
            <div class="mt-1 text-xs font-semibold text-[#313132]">{about.fullName}</div>
            <div class="mt-0.5 text-xs text-slate-700">{about.organization}</div>
          </section>

          <section class="space-y-2.5" aria-label="2026 Provincial Standards SHA-256 Stamps">
            <h4 class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-[#003366]">
              <FileCheck2 size={14} /> Embedded 2026 Master Standards (SHA-256 Verified)
            </h4>

            <div class="grid grid-cols-1 gap-2.5">
              <div class="rounded border border-slate-200 bg-white p-3">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-semibold text-[#003366]">BECdb Master Classification</span>
                  <span class="rounded bg-emerald-50 px-2 py-0.5 font-mono text-[11px] font-semibold text-[#2E8540]">
                    {about.libraryStamps.becVersion}
                  </span>
                </div>
                <div class="mt-1.5 space-y-1 font-mono text-[11px] text-slate-700">
                  <div class="break-all">Archive SHA-256: {about.libraryStamps.becArchiveSha256}</div>
                  <div class="break-all">ACCDB SHA-256: {about.libraryStamps.becExtractedSha256}</div>
                </div>
              </div>

              <div class="rounded border border-slate-200 bg-white p-3">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-semibold text-[#003366]">B.C. Master Plant Taxonomy (USysAllSpecs)</span>
                  <span class="rounded bg-emerald-50 px-2 py-0.5 font-mono text-[11px] font-semibold text-[#2E8540]">
                    {about.libraryStamps.taxonomyVersion}
                  </span>
                </div>
                <div class="mt-1.5 space-y-1 font-mono text-[11px] text-slate-700">
                  <div class="break-all">Archive SHA-256: {about.libraryStamps.taxonomyArchiveSha256}</div>
                  <div class="break-all">ACCDB SHA-256: {about.libraryStamps.taxonomyExtractedSha256}</div>
                </div>
              </div>

              <div class="rounded border border-slate-200 bg-white p-3">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-semibold text-[#003366]">Field Manual Specification</span>
                  <span class="rounded bg-emerald-50 px-2 py-0.5 font-mono text-[11px] font-semibold text-[#2E8540]">
                    {about.libraryStamps.lmh25Edition}
                  </span>
                </div>
                <div class="mt-1.5 break-all font-mono text-[11px] text-slate-700">
                  PDF SHA-256: {about.libraryStamps.lmh25Pdf2023Sha256}
                </div>
              </div>
            </div>
          </section>

          <details class="rounded border border-slate-200 bg-white p-3">
            <summary class="cursor-pointer text-xs font-bold text-[#003366]">
              Embedded SQLite Read-Model Artifacts ({about.libraryStamps.artifacts.length})
            </summary>
            <div class="mt-2 overflow-x-auto">
              <table class="w-full min-w-0 border-collapse text-[11px]">
                <thead>
                  <tr class="bg-[#F2F2F2] text-left font-semibold text-[#003366]">
                    <th class="border-b border-slate-200 px-2 py-1.5">Artifact</th>
                    <th class="border-b border-slate-200 px-2 py-1.5">Row Counts</th>
                    <th class="border-b border-slate-200 px-2 py-1.5">SHA-256</th>
                  </tr>
                </thead>
                <tbody>
                  {#each about.libraryStamps.artifacts as art (art.path)}
                    <tr class="border-b border-slate-100 last:border-b-0">
                      <td class="whitespace-nowrap px-2 py-1.5 font-mono text-[#003366]">{art.path}</td>
                      <td class="whitespace-normal px-2 py-1.5 font-mono text-slate-700">
                        {Object.entries(art.counts).map(([k, v]) => `${k}: ${v}`).join(', ')}
                      </td>
                      <td class="whitespace-nowrap px-2 py-1.5 font-mono text-slate-600">
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 rounded border border-slate-200 px-1.5 py-0.5 hover:bg-[#F2F2F2]"
                          title={art.sha256}
                          onclick={() => void copyDigest(art.sha256)}
                        >
                          {#if copiedDigest === art.sha256}
                            <CheckCircle2 size={11} class="text-[#2E8540]" /> Copied
                          {:else}
                            <Copy size={11} /> {art.sha256.slice(0, 12)}…
                          {/if}
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </details>

          {#if agreement}
            <details class="rounded border border-slate-300 bg-[#F2F2F2] p-3" data-bec-data-use-agreement>
              <summary class="cursor-pointer text-xs font-bold text-[#003366]">
                {agreement.title} ({agreement.clauses.length} Clauses &amp; Official Citation)
              </summary>
              <div class="mt-2.5 space-y-2.5 border-t border-slate-300 pt-2.5 text-xs">
                <div class="rounded border border-slate-300 bg-white p-2.5">
                  <div class="text-[10px] font-bold uppercase text-slate-500">Recommended Citation</div>
                  <p class="mt-1 font-mono text-[11px] text-[#313132]">{agreement.citation}</p>
                </div>
                <ol class="space-y-1.5 text-[11px] leading-relaxed text-[#313132]">
                  {#each agreement.clauses as clause}
                    <li class="rounded bg-white p-2">{clause}</li>
                  {/each}
                </ol>
              </div>
            </details>
          {/if}
        {/if}
      </div>
    </div>
  </div>
{/if}
