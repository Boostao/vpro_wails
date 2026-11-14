<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte';
  import { ReadRequests } from '../shared/readRequests';
  import {
    createDefaultBreakpointOptions,
    createDefaultSiteUnitExclusionRules,
    defaultSiteUnitExclusionPort,
    type ExclusionMatchMode,
    type HierarchyBreakpointOptions,
    type LongVegetationExclusionPreview,
    type SiteUnitExclusionBackendPort,
    type SiteUnitExclusionPreview,
    type SiteUnitExclusionRule,
    type SummaryVegetationExclusionPreview,
  } from './siteUnitExclusionFilter';

  interface Props {
    contextId: string;
    su?: string;
    hierarchy?: string;
    reportMode?: 'summary' | 'long';
    average?: number;
    display?: number;
    showClass?: number;
    groupAndOrder?: number;
    initialRules?: SiteUnitExclusionRule[];
    initialUseBreakpoints?: boolean;
    port?: SiteUnitExclusionBackendPort;
    onBusyChange?: (busy: boolean) => void;
    onRulesChange?: (
      rules: SiteUnitExclusionRule[],
      useHierarchyBreakpoints: boolean,
      breakpointOptions: HierarchyBreakpointOptions
    ) => void;
    onSummaryPreview?: (result: SummaryVegetationExclusionPreview) => void;
    onLongPreview?: (result: LongVegetationExclusionPreview) => void;
  }

  let {
    contextId,
    su = 'None',
    hierarchy = 'None',
    reportMode = 'summary',
    average = 1,
    display = 6,
    showClass = 0,
    groupAndOrder = 11,
    initialRules,
    initialUseBreakpoints = false,
    port = defaultSiteUnitExclusionPort,
    onBusyChange,
    onRulesChange,
    onSummaryPreview,
    onLongPreview,
  }: Props = $props();

  const reads = new ReadRequests();

  let rules = $state<SiteUnitExclusionRule[]>(
    untrack(() =>
      initialRules && initialRules.length > 0
        ? initialRules.map(r => ({ ...r }))
        : createDefaultSiteUnitExclusionRules()
    )
  );
  let useHierarchyBreakpoints = $state<boolean>(untrack(() => initialUseBreakpoints));
  let breakpointOptions = $state<HierarchyBreakpointOptions>(createDefaultBreakpointOptions());

  let newPattern = $state('');
  let newMatchMode = $state<ExclusionMatchMode>('auto');

  let preview = $state.raw<SiteUnitExclusionPreview | null>(null);
  let busy = $state(false);
  let errorMessage = $state('');
  let generation = 0;

  $effect(() => {
    onBusyChange?.(busy);
  });

  function emitChange() {
    onRulesChange?.(
      $state.snapshot(rules),
      useHierarchyBreakpoints,
      $state.snapshot(breakpointOptions)
    );
  }

  async function refreshExclusionPreview() {
    if (!contextId || !su || su === 'None') {
      preview = null;
      return;
    }
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const effectiveBreak = useHierarchyBreakpoints && hierarchy !== 'None' && hierarchy !== '';
      const res = await reads.track(
        port.previewExclusions(contextId, {
          rules: $state.snapshot(rules),
          useDefaultPresets: false,
          useHierarchyBreakpoints: effectiveBreak,
          breakpointOptions: $state.snapshot(breakpointOptions),
        })
      );
      if (reqGen !== generation) return;
      preview = res;
    } catch (err) {
      if (reqGen === generation) {
        errorMessage = String(err);
      }
    } finally {
      if (reqGen === generation) {
        busy = false;
      }
    }
  }

  async function runFilteredReportPreview() {
    if (!contextId || !su || su === 'None' || busy) return;
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const effectiveBreak = useHierarchyBreakpoints && hierarchy !== 'None' && hierarchy !== '';
      if (reportMode === 'summary') {
        const res = await reads.track(
          port.previewSummaryWithExclusions(contextId, {
            average,
            display,
            showClass,
            groupAndOrder,
            rules: $state.snapshot(rules),
            useDefaultPresets: false,
            useHierarchyBreakpoints: effectiveBreak,
            breakpointOptions: $state.snapshot(breakpointOptions),
          })
        );
        if (reqGen !== generation) return;
        preview = {
          contextId,
          project: '',
          su,
          hierarchy,
          exclusions: res.exclusions,
          breakpoints: res.breakpoints,
        };
        onSummaryPreview?.(res);
      } else {
        const res = await reads.track(
          port.previewLongWithExclusions(contextId, {
            rules: $state.snapshot(rules),
            useDefaultPresets: false,
            useHierarchyBreakpoints: effectiveBreak,
            breakpointOptions: $state.snapshot(breakpointOptions),
          })
        );
        if (reqGen !== generation) return;
        preview = {
          contextId,
          project: '',
          su,
          hierarchy,
          exclusions: res.exclusions,
          breakpoints: res.breakpoints,
        };
        onLongPreview?.(res);
      }
    } catch (err) {
      if (reqGen === generation) {
        errorMessage = String(err);
      }
    } finally {
      if (reqGen === generation) {
        busy = false;
      }
    }
  }

  function addRule() {
    const trimmed = newPattern.trim();
    if (!trimmed) return;
    if (rules.some(r => r.pattern.toLowerCase() === trimmed.toLowerCase() && r.matchMode === newMatchMode)) {
      newPattern = '';
      return;
    }
    rules = [...rules, { pattern: trimmed, matchMode: newMatchMode, enabled: true }];
    newPattern = '';
    emitChange();
    void refreshExclusionPreview();
  }

  function removeRule(idx: number) {
    rules = rules.filter((_, i) => i !== idx);
    emitChange();
    void refreshExclusionPreview();
  }

  function toggleRule(idx: number) {
    rules = rules.map((r, i) => (i === idx ? { ...r, enabled: !r.enabled } : r));
    emitChange();
    void refreshExclusionPreview();
  }

  function resetPresets() {
    rules = createDefaultSiteUnitExclusionRules();
    emitChange();
    void refreshExclusionPreview();
  }

  onMount(() => {
    if (contextId && su && su !== 'None') {
      void refreshExclusionPreview();
    }
  });

  onDestroy(() => {
    generation++;
    reads.cancelAll();
    onBusyChange?.(false);
  });
</script>

<section class="vpro-su-exclusions" aria-label="SiteUnit Exclusion Filters and Hierarchy Breakpoints">
  <header class="vpro-sux-header">
    <div>
      <p class="vpro-sux-eyebrow">SITE UNIT FILTERING &amp; DYNAMIC BREAKPOINTS</p>
      <h3 class="vpro-sux-title">
        SiteUnit Exclusions (.support / .poor / .nudum)
        {#if reportMode === 'summary'}&amp; '~' Hierarchy Rollup{/if}
      </h3>
    </div>

    <div class="vpro-sux-actions">
      <button type="button" class="vpro-btn-secondary" disabled={busy} onclick={resetPresets}>
        Reset BEC Defaults
      </button>
      <button
        type="button"
        class="vpro-btn-secondary"
        disabled={busy || !su || su === 'None'}
        onclick={refreshExclusionPreview}
      >
        Check Units
      </button>
      <button
        type="button"
        class="vpro-btn-primary"
        disabled={busy || !su || su === 'None'}
        onclick={runFilteredReportPreview}
      >
        {busy
          ? 'Computing...'
          : reportMode === 'summary'
            ? 'Preview Summary Vegetation'
            : 'Preview Long Vegetation'}
      </button>
    </div>
  </header>

  {#if errorMessage}
    <p role="alert" class="vpro-sux-alert">{errorMessage}</p>
  {/if}

  <div class="vpro-sux-rules" role="list" aria-label="Active SiteUnit Exclusion Rules">
    {#each rules as rule, idx (`${rule.pattern}:${rule.matchMode}:${idx}`)}
      <div role="listitem" class="vpro-sux-chip" class:disabled={!rule.enabled}>
        <input
          type="checkbox"
          checked={rule.enabled}
          aria-label={`Enable exclusion rule ${rule.pattern}`}
          onchange={() => toggleRule(idx)}
        />
        <code class="mono">{rule.pattern}</code>
        <span class="vpro-sux-mode-tag">{rule.matchMode}</span>
        <button
          type="button"
          class="vpro-sux-remove"
          aria-label={`Remove exclusion rule ${rule.pattern}`}
          onclick={() => removeRule(idx)}
        >
          x
        </button>
      </div>
    {/each}
  </div>

  <div class="vpro-sux-add-row">
    <label>
      <span>Exclude Suffix / Phrase / Wildcard</span>
      <input
        type="text"
        bind:value={newPattern}
        placeholder="e.g. .transitional or *unassigned*"
        onkeydown={e => {
          if (e.key === 'Enter') {
            e.preventDefault();
            addRule();
          }
        }}
      />
    </label>
    <label>
      <span>Match Mode</span>
      <select bind:value={newMatchMode}>
        <option value="auto">Auto (Dot-Suffix / Phrase / *?)</option>
        <option value="suffix">Case-Insensitive Suffix</option>
        <option value="contains">Case-Insensitive Substring</option>
        <option value="wildcard">Wildcard (* / ?)</option>
      </select>
    </label>
    <button type="button" class="vpro-btn-secondary" onclick={addRule}>
      + Add Exclusion
    </button>
  </div>

  <div class="vpro-sux-breakpoints">
    <label class="vpro-sux-check">
      <input
        type="checkbox"
        bind:checked={useHierarchyBreakpoints}
        disabled={!hierarchy || hierarchy === 'None'}
        onchange={() => {
          emitChange();
          void refreshExclusionPreview();
        }}
      />
      <span>
        Roll up descendant Site Units under <code>~Branch</code> Hierarchy Breakpoints
        {#if !hierarchy || hierarchy === 'None'}
          <em>(Select an active Hierarchy in the sidebar to enable)</em>
        {:else}
          (Active Hierarchy: <strong>{hierarchy}</strong>)
        {/if}
      </span>
    </label>

    {#if useHierarchyBreakpoints && hierarchy && hierarchy !== 'None'}
      <div class="vpro-sux-bp-opts">
        <label>
          <span>Nested '~' Breakpoint Policy</span>
          <select
            bind:value={breakpointOptions.breakpointPolicy}
            onchange={() => {
              emitChange();
              void refreshExclusionPreview();
            }}
          >
            <option value="lowest">Lowest '~' Ancestor (Nearest Branch)</option>
            <option value="highest">Highest '~' Ancestor (Top Branch)</option>
          </select>
        </label>
        <label>
          <span>Units Without '~' Ancestor</span>
          <select
            bind:value={breakpointOptions.unbrokenUnitPolicy}
            onchange={() => {
              emitChange();
              void refreshExclusionPreview();
            }}
          >
            <option value="keep_original">Keep Original SiteUnit</option>
            <option value="exclude_unbroken">Exclude Unbroken Units</option>
          </select>
        </label>
        <label class="vpro-sux-check">
          <input
            type="checkbox"
            bind:checked={breakpointOptions.stripTildePrefix}
            onchange={() => {
              emitChange();
              void refreshExclusionPreview();
            }}
          />
          <span>Strip '~' prefix in column header</span>
        </label>
      </div>
    {/if}
  </div>

  {#if preview}
    <div class="vpro-sux-summary" role="status">
      <p>
        SU <code>{preview.su}</code>:
        <strong>{preview.exclusions.retainedPlots}</strong> of
        <strong>{preview.exclusions.totalPlotsBefore}</strong> plots retained ·
        <strong>{preview.exclusions.excludedPlots}</strong> plots excluded across
        <strong>{preview.exclusions.excludedUnits.length}</strong> unit(s).
        {#if preview.breakpoints}
          Dynamic <code>~</code> rollup:
          <strong>{preview.breakpoints.totalUnitsBefore}</strong> unit(s) -&gt;
          <strong>{preview.breakpoints.totalUnitsAfter}</strong> report unit(s)
          ({preview.breakpoints.rolledUpPlotCount} plot(s) aggregated).
        {/if}
      </p>

      {#if preview.exclusions.excludedUnits.length > 0}
        <details class="vpro-sux-details">
          <summary>Excluded Site Units ({preview.exclusions.excludedUnits.length})</summary>
          <ul>
            {#each preview.exclusions.excludedUnits as ex (ex.siteUnit)}
              <li>
                <code class="mono">{ex.siteUnit}</code> — {ex.plotCount} plot(s) (matched
                <code>{ex.matchedPattern}</code>)
              </li>
            {/each}
          </ul>
        </details>
      {/if}

      {#if preview.breakpoints && preview.breakpoints.buckets.length > 0}
        <details class="vpro-sux-details" open>
          <summary>Dynamic '~' Breakpoint Rollup Buckets ({preview.breakpoints.buckets.length})</summary>
          <ul>
            {#each preview.breakpoints.buckets as b (b.breakpointUnit)}
              <li>
                <strong class="mono">{b.breakpointUnit}</strong> ({b.plotCount} plot(s)) &lt;=
                {b.sourceUnits.join(', ')}
              </li>
            {/each}
          </ul>
        </details>
      {/if}
    </div>
  {/if}
</section>

<style>
  .vpro-su-exclusions {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    background: #f8fafc;
    border: 1px solid #cbd5e1;
    border-left: 4px solid #003366;
    border-radius: 4px;
    padding: 0.85rem 1rem;
    min-width: 0;
  }
  .vpro-sux-header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .vpro-sux-eyebrow {
    margin: 0;
    font-size: 0.68rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #003366;
  }
  .vpro-sux-title {
    margin: 0.1rem 0 0;
    font-size: 0.96rem;
    font-weight: 700;
    color: #003366;
  }
  .vpro-sux-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .vpro-sux-rules {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .vpro-sux-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.25rem 0.5rem;
    background: #ffffff;
    border: 1px solid #003366;
    border-radius: 999px;
    font-size: 0.78rem;
  }
  .vpro-sux-chip.disabled {
    opacity: 0.55;
    border-color: #94a3b8;
  }
  .vpro-sux-mode-tag {
    font-size: 0.66rem;
    background: #e2e8f0;
    color: #334155;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
  }
  .vpro-sux-remove {
    border: 0;
    background: transparent;
    color: #991b1b;
    font-weight: 700;
    padding: 0 2px;
  }
  .vpro-sux-add-row,
  .vpro-sux-bp-opts {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.5rem;
  }
  .vpro-sux-add-row label,
  .vpro-sux-bp-opts label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.76rem;
    font-weight: 600;
    color: #003366;
  }
  .vpro-sux-add-row input,
  .vpro-sux-add-row select,
  .vpro-sux-bp-opts select {
    min-height: 30px;
    padding: 0.2rem 0.45rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    font-size: 0.8rem;
    background: #ffffff;
  }
  .vpro-sux-breakpoints {
    border-top: 1px solid #e2e8f0;
    padding-top: 0.5rem;
    display: grid;
    gap: 0.45rem;
  }
  .vpro-sux-check {
    display: inline-flex !important;
    flex-direction: row !important;
    align-items: center;
    gap: 0.4rem !important;
    font-size: 0.8rem;
    color: #1e293b;
  }
  .vpro-sux-summary {
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-top: 2px solid #fcba19;
    padding: 0.55rem 0.75rem;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .vpro-sux-summary p {
    margin: 0;
  }
  .vpro-sux-details {
    margin-top: 0.4rem;
    font-size: 0.76rem;
  }
  .vpro-sux-details ul {
    margin: 0.25rem 0 0;
    padding-left: 1.2rem;
  }
  .vpro-sux-alert {
    margin: 0;
    padding: 0.45rem 0.7rem;
    background: #fef2f2;
    border-left: 4px solid #b91c1c;
    color: #991b1b;
    font-size: 0.8rem;
  }
  .vpro-btn-primary {
    min-height: 32px;
    padding: 0.3rem 0.75rem;
    background: #003366;
    color: #ffffff;
    border: 1px solid #003366;
    border-bottom: 2px solid #fcba19;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
  }
  .vpro-btn-secondary {
    min-height: 32px;
    padding: 0.3rem 0.65rem;
    background: #ffffff;
    color: #003366;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .mono {
    font-family: var(--font-mono, monospace);
  }
</style>
