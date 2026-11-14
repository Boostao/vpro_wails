<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    countDiagnosticsBySeverity,
    defaultLMH25DiagnosticsPort,
    formatHumusComparison,
    normalizeDiagnosticsWire,
    substrateProgressPercent,
    substrateStatusLabel,
    type LMH25DiagnosticsPort,
    type PlotDiagnosticsResult,
  } from './lmh25Diagnostics';

  let {
    contextId = '',
    plotNumber = '',
    header = null,
    humus = [],
    mineral = [],
    port = defaultLMH25DiagnosticsPort,
    onApplyHumusThickness,
    onApplyKeyedSMR,
    onApplyKeyedSNR,
  }: {
    contextId?: string;
    plotNumber?: string;
    header?: Record<string, unknown> | null;
    humus?: Record<string, unknown>[];
    mineral?: Record<string, unknown>[];
    port?: LMH25DiagnosticsPort;
    onApplyHumusThickness?: (value: number) => void;
    onApplyKeyedSMR?: (smrCode: string) => void;
    onApplyKeyedSNR?: (snrCode: string) => void;
  } = $props();

  let busy = $state(false);
  let error = $state('');
  let diagnostics = $state.raw<PlotDiagnosticsResult | null>(null);
  let detailsOpen = $state(false);
  let generation = 0;

  function cancel() {
    generation++;
    busy = false;
  }

  const counts = $derived.by(() => countDiagnosticsBySeverity(diagnostics?.issues));
  const substratePct = $derived.by(() => substrateProgressPercent(diagnostics?.substrateSum ?? null));
  const substrateLabel = $derived.by(() => substrateStatusLabel(diagnostics));
  const humusSummary = $derived.by(() => formatHumusComparison(diagnostics));

  async function evaluateCurrent() {
    const reqGen = ++generation;
    const owner = contextId;
    error = '';
    busy = true;
    try {
      let raw: PlotDiagnosticsResult;
      if (header) {
        const headerSnap = $state.snapshot(header);
        const humusSnap = $state.snapshot(humus);
        const mineralSnap = $state.snapshot(mineral);
        raw = await port.evaluatePlotDiagnostics(headerSnap, humusSnap, mineralSnap);
      } else if (owner && plotNumber) {
        raw = await port.evaluateStoredPlotDiagnostics(owner, plotNumber);
      } else {
        busy = false;
        return;
      }
      if (reqGen !== generation || owner !== contextId) return;
      const validated = normalizeDiagnosticsWire(raw);
      if (reqGen !== generation || owner !== contextId) return;
      diagnostics = validated;
    } catch (cause) {
      if (reqGen === generation) error = String(cause);
    } finally {
      if (reqGen === generation) busy = false;
    }
  }

  onDestroy(cancel);
</script>

<aside class="lmh25-bar" data-lmh25-diagnostics aria-label="LMH25 (2023) Edatopic and Pedon Rule Diagnostics">
  <div class="lmh25-summary-row">
    <div class="lmh25-brand">
      <span class="lmh25-tag">LMH25 (2023)</span>
      <strong>Pedon &amp; Edatopic Co-Pilot</strong>
    </div>

    {#if diagnostics}
      <div class="lmh25-chips">
        <span class="chip" class:divergent={diagnostics.smrDivergent} data-smr-badge>
          Keyed rSMR: <strong>{diagnostics.keyedSmr || 'N/A'}</strong>
          {#if diagnostics.fieldSmr}
            (Field: {diagnostics.fieldSmr})
          {/if}
          {#if diagnostics.smrDivergent}
            <strong class="alert-flag">Diverges &gt;=2</strong>
          {/if}
          {#if onApplyKeyedSMR && diagnostics.keyedSmrClass !== null}
            <button type="button" class="chip-action" onclick={() => onApplyKeyedSMR!(String(diagnostics!.keyedSmrClass))}>
              Apply {diagnostics.keyedSmrClass}
            </button>
          {/if}
        </span>

        <span class="chip" class:divergent={diagnostics.snrDivergent} data-snr-badge>
          Keyed rSNR: <strong>{diagnostics.keyedSnr || 'N/A'}</strong>
          {#if diagnostics.fieldSnr}
            (Field: {diagnostics.fieldSnr})
          {/if}
          {#if diagnostics.snrDivergent}
            <strong class="alert-flag">Diverges &gt;=2</strong>
          {/if}
          {#if onApplyKeyedSNR && diagnostics.keyedSnr}
            <button type="button" class="chip-action" onclick={() => onApplyKeyedSNR!(diagnostics!.keyedSnr.slice(0, 1))}>
              Apply {diagnostics.keyedSnr.slice(0, 1)}
            </button>
          {/if}
        </span>

        <span class="chip" data-humus-badge>
          Humus: <strong>{humusSummary}</strong>
          {#if onApplyHumusThickness && diagnostics.derivedHumusThickness !== null && (diagnostics.headerHumusThickness === null || (diagnostics.humusThicknessDiff !== null && Math.abs(diagnostics.humusThicknessDiff) > 0.05))}
            <button type="button" class="chip-action" onclick={() => onApplyHumusThickness!(diagnostics!.derivedHumusThickness!)}>
              Use {diagnostics.derivedHumusThickness} cm
            </button>
          {/if}
        </span>

        <span class="chip" class:divergent={!diagnostics.substrateValid && diagnostics.substratePopulated > 0} data-substrate-badge>
          Substrates: <strong>{substrateLabel}</strong>
          <span class="substrate-meter" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={substratePct}>
            <span class="substrate-fill" class:invalid={!diagnostics.substrateValid} style={`width: ${substratePct}%`}></span>
          </span>
        </span>
      </div>
    {/if}

    <div class="lmh25-controls">
      <button type="button" class="bcgov-btn" disabled={busy || (!header && (!contextId || !plotNumber))} onclick={evaluateCurrent}>
        {busy ? 'Checking...' : 'Evaluate LMH25 Rules'}
      </button>
      {#if diagnostics}
        <button type="button" class="bcgov-btn-outline" onclick={() => (detailsOpen = !detailsOpen)}>
          {detailsOpen ? 'Hide Trace' : `Issues (${counts.errors + counts.warnings}) & Key Trace`}
        </button>
      {/if}
    </div>
  </div>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if diagnostics && detailsOpen}
    <div class="lmh25-drawer">
      <div class="drawer-col">
        <h4>Pedon, Horizon &amp; Substrate Findings ({diagnostics.issues.length})</h4>
        {#if diagnostics.issues.length === 0}
          <p class="ok-note">All upland/wetland horizon depth signs, contiguity, von Post Table 2.23 bounds, and substrates pass LMH25 checks.</p>
        {:else}
          <ul class="issue-list">
            {#each diagnostics.issues as iss, idx (iss.code + ':' + idx)}
              <li class={`sev-${iss.severity}`}>
                <strong>[{iss.severity.toUpperCase()}] {iss.code}</strong>: {iss.message}
                <span class="ref-cite">({iss.lmh25Ref})</span>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <div class="drawer-col">
        <h4>Keys 9.3, 9.4 (rSMR) &amp; 9.6 (rSNR) Decision Trace</h4>
        <ul class="trace-list">
          {#each [...diagnostics.smrTrace, ...diagnostics.snrTrace] as step, idx (step.key + ':' + idx)}
            <li>
              <span class="trace-key">Key {step.key}</span>
              <strong>{step.step}:</strong> {step.condition} -&gt; <strong>{step.outcome}</strong>
            </li>
          {/each}
        </ul>
      </div>
    </div>
  {/if}
</aside>

<style>
  .lmh25-bar { background: #f8fafc; border: 1px solid #cbd5e1; border-left: 5px solid #003366; border-radius: 4px; padding: 0.65rem 0.9rem; margin-bottom: 0.85rem; }
  .lmh25-summary-row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.65rem; }
  .lmh25-brand { display: inline-flex; align-items: center; gap: 0.5rem; color: #003366; font-size: 0.88rem; }
  .lmh25-tag { background: #003366; color: #fcba19; padding: 0.15rem 0.45rem; border-radius: 3px; font-size: 0.72rem; font-weight: 700; letter-spacing: 0.04em; }
  .lmh25-chips { display: flex; flex-wrap: wrap; gap: 0.45rem; align-items: center; }
  .chip { display: inline-flex; align-items: center; gap: 0.35rem; background: #ffffff; border: 1px solid #cbd5e1; border-radius: 4px; padding: 0.25rem 0.55rem; font-size: 0.8rem; color: #1e293b; }
  .chip.divergent { background: #fffbeb; border-color: #d97706; color: #78350f; }
  .alert-flag { background: #fcba19; color: #003366; padding: 0.05rem 0.35rem; border-radius: 3px; font-size: 0.72rem; }
  .chip-action { background: #003366; color: #ffffff; border: 0; border-radius: 3px; padding: 0.12rem 0.4rem; font-size: 0.73rem; }
  .substrate-meter { width: 56px; height: 8px; background: #e2e8f0; border-radius: 4px; overflow: hidden; display: inline-block; }
  .substrate-fill { display: block; height: 100%; background: #003366; }
  .substrate-fill.invalid { background: #d97706; }
  .lmh25-controls { display: flex; gap: 0.4rem; }
  .bcgov-btn { background: #003366; color: #ffffff; border: 1px solid #003366; border-radius: 4px; padding: 0.35rem 0.7rem; font-size: 0.8rem; font-weight: 600; }
  .bcgov-btn-outline { background: #ffffff; color: #003366; border: 1px solid #003366; border-radius: 4px; padding: 0.35rem 0.7rem; font-size: 0.8rem; font-weight: 600; }
  .lmh25-drawer { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid #cbd5e1; display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; font-size: 0.82rem; }
  .lmh25-drawer h4 { margin: 0 0 0.45rem; color: #003366; font-size: 0.86rem; }
  .issue-list, .trace-list { margin: 0; padding-left: 1.1rem; display: grid; gap: 0.3rem; }
  .sev-error { color: #991b1b; }
  .sev-warning { color: #92400e; }
  .sev-info { color: #003366; }
  .ref-cite { color: #64748b; font-size: 0.76rem; }
  .trace-key { background: #e6eef8; color: #003366; padding: 0.05rem 0.35rem; border-radius: 3px; font-weight: 600; margin-right: 0.25rem; }
  .ok-note { color: #166534; margin: 0; }
</style>
