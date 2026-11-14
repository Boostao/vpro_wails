<script lang="ts">
  import { untrack } from 'svelte';
  import { BookOpen, CheckCircle2, Compass, FileText, RotateCcw, Scale, Search, SlidersHorizontal, X } from '@lucide/svelte';
  import {
    COVER_COMPARISON_BLOCKS,
    answerDecisionKeyStep,
    createDecisionKeyState,
    defaultHelpPort,
    isDichotomousDecisionKey,
    type DecisionKeyEvalState,
    type HelpBackendPort,
    type HelpBundle,
    type HelpDecisionKey,
    type HelpDrawerTab,
    type HelpGuide,
    type HelpLookupResult,
    type HelpSearchHit,
  } from './helpSystem';

  let {
    open = false,
    topicId = 'site.plot_number',
    initialTab = 'inspector',
    helpPort = defaultHelpPort,
    onclose,
    onstarttour,
  }: {
    open?: boolean;
    topicId?: string;
    initialTab?: HelpDrawerTab;
    helpPort?: HelpBackendPort;
    onclose: () => void;
    onstarttour?: (tourId?: string) => void;
  } = $props();

  let activeTab = $state<HelpDrawerTab>(untrack(() => initialTab));
  let bundle = $state.raw<HelpBundle | null>(null);
  let lookup = $state.raw<HelpLookupResult | null>(null);
  let selectedFieldId = $state(untrack(() => topicId));
  let selectedKeyId = $state('key.rsmr_9_4');
  let selectedGuideId = $state('migrating-from-vpro64');
  let keyEval = $state.raw<DecisionKeyEvalState | null>(null);

  let searchQuery = $state('');
  let searchHits = $state.raw<HelpSearchHit[]>([]);
  let searchBusy = $state(false);
  let loading = $state(false);
  let error = $state('');

  let loadGeneration = 0;
  let lookupGeneration = 0;
  let searchGeneration = 0;

  $effect(() => {
    const incomingTab = initialTab;
    untrack(() => {
      activeTab = incomingTab;
    });
  });

  $effect(() => {
    if (!open) return;
    const requestedTopic = topicId;
    untrack(() => {
      void ensureBundleAndLookup(requestedTopic);
    });
  });

  async function ensureBundleAndLookup(targetTopic: string) {
    const gen = ++loadGeneration;
    loading = true;
    error = '';
    try {
      let currentBundle = bundle;
      if (!currentBundle) {
        currentBundle = await helpPort.getHelpBundle();
        if (gen !== loadGeneration) return;
        bundle = currentBundle;
        if (currentBundle.guides.length > 0 && !currentBundle.guides.some((g) => g.id === selectedGuideId)) {
          selectedGuideId = currentBundle.guides[0].id;
        }
      }
      await loadTopic(targetTopic || 'site.plot_number');
    } catch (cause) {
      if (gen === loadGeneration) {
        error = `Offline LMH25 help failed to load: ${String(cause)}`;
      }
    } finally {
      if (gen === loadGeneration) {
        loading = false;
      }
    }
  }

  async function loadTopic(helpId: string) {
    const gen = ++lookupGeneration;
    error = '';
    try {
      const resolved = await helpPort.lookupHelpTopic(helpId);
      if (gen !== lookupGeneration) return;
      lookup = resolved;
      if (resolved.field) {
        selectedFieldId = resolved.field.id;
      }
      if (resolved.decisionKey) {
        selectedKeyId = resolved.decisionKey.id;
        initKeyEvaluator(resolved.decisionKey);
      } else if (bundle && !keyEval) {
        const defaultKey = bundle.decisionKeys.find((k) => k.id === selectedKeyId) ?? bundle.decisionKeys[0];
        if (defaultKey) initKeyEvaluator(defaultKey);
      }
    } catch (cause) {
      if (gen === lookupGeneration) {
        error = `Help topic lookup failed: ${String(cause)}`;
      }
    }
  }

  function initKeyEvaluator(key: HelpDecisionKey) {
    if (isDichotomousDecisionKey(key)) {
      keyEval = createDecisionKeyState(key);
    } else {
      keyEval = null;
    }
  }

  function selectDecisionKey(id: string) {
    selectedKeyId = id;
    const found = bundle?.decisionKeys.find((k) => k.id === id);
    if (found) {
      initKeyEvaluator(found);
    }
  }

  function answerKey(choice: 'yes' | 'no') {
    const activeKey = bundle?.decisionKeys.find((k) => k.id === selectedKeyId);
    if (!activeKey || !keyEval) return;
    keyEval = answerDecisionKeyStep(activeKey, keyEval, choice);
  }

  function resetKey() {
    const activeKey = bundle?.decisionKeys.find((k) => k.id === selectedKeyId);
    if (!activeKey) return;
    initKeyEvaluator(activeKey);
  }

  async function runSearch(query: string) {
    searchQuery = query;
    const trimmed = query.trim();
    const gen = ++searchGeneration;
    if (!trimmed) {
      searchHits = [];
      searchBusy = false;
      return;
    }
    searchBusy = true;
    try {
      const hits = await helpPort.searchHelp(trimmed);
      if (gen !== searchGeneration) return;
      searchHits = hits ?? [];
    } catch (cause) {
      if (gen === searchGeneration) {
        error = `Offline search failed: ${String(cause)}`;
      }
    } finally {
      if (gen === searchGeneration) {
        searchBusy = false;
      }
    }
  }

  function openSearchHit(hit: HelpSearchHit) {
    searchQuery = '';
    searchHits = [];
    if (hit.kind === 'field') {
      activeTab = 'inspector';
      void loadTopic(hit.id);
    } else if (hit.kind === 'key') {
      activeTab = 'keys';
      selectDecisionKey(hit.id);
    } else if (hit.kind === 'guide') {
      activeTab = 'guides';
      selectedGuideId = hit.id;
    } else if (hit.kind === 'tour') {
      onstarttour?.(hit.id);
    }
  }

  function openGuide(guide: HelpGuide) {
    selectedGuideId = guide.id;
    activeTab = 'guides';
  }

  const activeDecisionKey = $derived(
    bundle?.decisionKeys.find((k) => k.id === selectedKeyId) ?? lookup?.decisionKey ?? null
  );
  const currentKeyStepObj = $derived.by(() => {
    const activeKey = activeDecisionKey;
    const evalState = keyEval;
    if (!activeKey || !evalState) return null;
    return activeKey.steps.find((s) => s.step === evalState.currentStep) ?? null;
  });
  const activeGuide = $derived(
    bundle?.guides.find((g) => g.id === selectedGuideId) ?? bundle?.guides[0] ?? null
  );
  const groupedFields = $derived.by(() => {
    const groups = new Map<string, HelpBundle['fields']>();
    for (const field of bundle?.fields ?? []) {
      const list = groups.get(field.section) ?? [];
      list.push(field);
      groups.set(field.section, list);
    }
    return [...groups.entries()];
  });
</script>

{#if open}
  <div
    class="fixed top-0 right-0 bottom-0 z-40 flex w-full max-w-[32rem] flex-col border-l border-slate-300 bg-white text-[#313132] shadow-2xl"
    role="complementary"
    aria-label="VPro Help and LMH25 Reference"
    data-help-drawer
  >
    <header class="border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-4 py-3 text-white">
      <div class="flex items-center justify-between gap-2">
        <div class="min-w-0">
          <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">
            100% Offline Reference · LMH25 (2023)
          </div>
          <h2 class="truncate text-base font-semibold text-white">VPro Field Inspector &amp; BEC User Guide</h2>
        </div>
        <button
          type="button"
          class="rounded border border-white/30 bg-white/10 p-1.5 text-white hover:bg-white/20"
          aria-label="Close Help Drawer"
          data-help-drawer-close
          onclick={onclose}
        >
          <X size={16} />
        </button>
      </div>

      <div class="mt-3">
        <label for="help-drawer-search" class="sr-only">Search LMH25 fields, codes, keys and guides</label>
        <div class="relative flex items-center">
          <Search size={15} class="pointer-events-none absolute left-2.5 text-slate-500" />
          <input
            id="help-drawer-search"
            type="search"
            data-help-search-input
            value={searchQuery}
            placeholder="Search codes (SITE 20, SOIL 5, Of, Mor, rSMR), keys or guides..."
            class="w-full rounded border border-slate-300 bg-white py-1.5 pr-3 pl-8 text-xs text-[#313132] placeholder:text-slate-500 focus:outline-2 focus:outline-[#FCBA19]"
            oninput={(e) => void runSearch(e.currentTarget.value)}
          />
        </div>
      </div>
    </header>

    {#if searchQuery.trim()}
      <div class="max-h-64 overflow-y-auto border-b border-slate-300 bg-[#F2F2F2] p-3" data-help-search-results>
        <div class="mb-2 flex items-center justify-between text-xs text-slate-600">
          <span>{searchBusy ? 'Searching offline index...' : `${searchHits.length} matching reference topic(s)`}</span>
          <button
            type="button"
            class="text-xs font-medium text-[#003366] underline"
            onclick={() => { searchQuery = ''; searchHits = []; }}
          >
            Clear search
          </button>
        </div>
        {#if searchHits.length === 0 && !searchBusy}
          <p class="rounded border border-slate-200 bg-white p-2.5 text-xs text-slate-600">
            No offline topics matched "{searchQuery}". Try an LMH25 code (SITE 20, SOIL 5, VEG 2) or symbol (Of, Mor, rSMR).
          </p>
        {:else}
          <div class="space-y-1.5">
            {#each searchHits as hit (`${hit.kind}:${hit.id}`)}
              <button
                type="button"
                data-help-search-hit={hit.id}
                class="w-full rounded border border-slate-200 bg-white p-2 text-left hover:border-[#003366]"
                onclick={() => openSearchHit(hit)}
              >
                <div class="flex items-center justify-between gap-2">
                  <span class="font-mono text-[11px] font-semibold uppercase text-[#003366]">
                    {hit.code ? `${hit.code} · ` : ''}{hit.kind}
                  </span>
                  {#if hit.lmh25Page}
                    <span class="font-mono text-[11px] text-slate-500">LMH25 p. {hit.lmh25Page}</span>
                  {/if}
                </div>
                <div class="mt-0.5 text-xs font-semibold text-[#313132]">{hit.title}</div>
                <p class="mt-0.5 line-clamp-2 text-[11px] text-slate-600">{hit.snippet}</p>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <nav class="flex border-b border-slate-200 bg-[#F2F2F2] px-3 pt-2 text-xs font-semibold" aria-label="Help Drawer tabs">
      <button
        type="button"
        data-help-tab="inspector"
        aria-current={activeTab === 'inspector' ? 'page' : undefined}
        class="flex items-center gap-1.5 rounded-t border border-b-0 px-3 py-2 {activeTab === 'inspector' ? 'border-slate-300 bg-white text-[#003366]' : 'border-transparent text-slate-600 hover:text-[#003366]'}"
        onclick={() => { activeTab = 'inspector'; }}
      >
        <SlidersHorizontal size={14} /> Field Inspector
      </button>
      <button
        type="button"
        data-help-tab="keys"
        aria-current={activeTab === 'keys' ? 'page' : undefined}
        class="flex items-center gap-1.5 rounded-t border border-b-0 px-3 py-2 {activeTab === 'keys' ? 'border-slate-300 bg-white text-[#003366]' : 'border-transparent text-slate-600 hover:text-[#003366]'}"
        onclick={() => { activeTab = 'keys'; }}
      >
        <Scale size={14} /> Decision Keys
      </button>
      <button
        type="button"
        data-help-tab="guides"
        aria-current={activeTab === 'guides' ? 'page' : undefined}
        class="flex items-center gap-1.5 rounded-t border border-b-0 px-3 py-2 {activeTab === 'guides' ? 'border-slate-300 bg-white text-[#003366]' : 'border-transparent text-slate-600 hover:text-[#003366]'}"
        onclick={() => { activeTab = 'guides'; }}
      >
        <BookOpen size={14} /> User Guides
      </button>
    </nav>

    <div class="flex-1 space-y-4 overflow-y-auto p-4 text-xs leading-relaxed">
      {#if error}
        <p role="alert" class="rounded border border-[#D8292F] bg-red-50 p-2.5 text-[#D8292F]">{error}</p>
      {/if}
      {#if loading && !bundle}
        <p role="status" class="text-slate-600">Loading embedded LMH25 (2023) reference bundle...</p>
      {/if}

      {#if activeTab === 'inspector'}
        <div class="space-y-3">
          <div>
            <label for="help-field-selector" class="block text-[11px] font-semibold uppercase tracking-wide text-slate-600">
              Jump to LMH25 (2023) Field Specification
            </label>
            <select
              id="help-field-selector"
              class="mt-1 w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs text-[#313132]"
              value={selectedFieldId}
              onchange={(e) => void loadTopic(e.currentTarget.value)}
            >
              {#each groupedFields as [sectionName, fields] (sectionName)}
                <optgroup label={sectionName}>
                  {#each fields as f (f.id)}
                    <option value={f.id}>{f.code} — {f.title}</option>
                  {/each}
                </optgroup>
              {/each}
            </select>
          </div>

          {#if lookup?.field}
            {@const field = lookup.field}
            <article class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#003366] bg-white p-3.5" data-help-active-field={field.id}>
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="rounded bg-[#003366] px-2 py-0.5 font-mono text-[11px] font-semibold text-white">
                  {field.code}
                </span>
                <span class="rounded border border-slate-300 bg-[#F2F2F2] px-2 py-0.5 font-mono text-[11px] font-medium text-[#313132]">
                  LMH25 (2023) p. {field.lmh25Page}
                </span>
              </div>

              <div>
                <div class="text-[11px] font-medium text-slate-500">{field.section}</div>
                <h3 class="mt-0.5 text-sm font-bold text-[#003366]">{field.title}</h3>
              </div>

              <dl class="grid grid-cols-1 gap-2 rounded bg-[#F2F2F2] p-2.5 sm:grid-cols-2">
                <div>
                  <dt class="text-[10px] font-semibold uppercase text-slate-500">VPro Column(s)</dt>
                  <dd class="mt-0.5 font-mono text-[11px] font-medium text-[#313132]">{field.column}</dd>
                </div>
                <div>
                  <dt class="text-[10px] font-semibold uppercase text-slate-500">Domain &amp; Units</dt>
                  <dd class="mt-0.5 text-[11px] font-medium text-[#313132]">{field.unit}</dd>
                </div>
              </dl>

              <p class="text-xs text-[#313132]">{field.summary}</p>

              {#if field.rules.length > 0}
                <section aria-label="Calibration and directional rules" class="space-y-1.5">
                  <h4 class="text-[11px] font-bold uppercase tracking-wide text-[#003366]">
                    LMH25 &amp; VPro Calibration Rules
                  </h4>
                  <ul class="list-disc space-y-1 pl-4 text-xs text-[#313132]">
                    {#each field.rules as rule}
                      <li>{rule}</li>
                    {/each}
                  </ul>
                </section>
              {/if}

              {#if lookup.decisionKey}
                {@const dk = lookup.decisionKey}
                <div class="rounded border border-[#FCBA19] bg-amber-50/50 p-2.5">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <div class="font-mono text-[11px] font-semibold text-[#003366]">{dk.code}</div>
                      <div class="text-xs font-medium text-[#313132]">{dk.title}</div>
                    </div>
                    <button
                      type="button"
                      data-help-open-linked-key={dk.id}
                      class="rounded bg-[#003366] px-2.5 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
                      onclick={() => { selectDecisionKey(dk.id); activeTab = 'keys'; }}
                    >
                      Open Interactive Key
                    </button>
                  </div>
                </div>
              {/if}

              {#if field.codes.length > 0}
                <section aria-label="Valid picklist codes" class="space-y-1.5">
                  <h4 class="text-[11px] font-bold uppercase tracking-wide text-[#003366]">
                    Standard Picklist Codes (USysTableOfLists)
                  </h4>
                  <div class="overflow-x-auto rounded border border-slate-200">
                    <table class="w-full min-w-0 border-collapse text-xs">
                      <thead>
                        <tr class="bg-[#F2F2F2] text-left text-[11px] font-semibold text-[#003366]">
                          <th class="border-b border-slate-200 px-2.5 py-1.5">Code</th>
                          <th class="border-b border-slate-200 px-2.5 py-1.5">LMH25 Definition</th>
                        </tr>
                      </thead>
                      <tbody>
                        {#each field.codes as opt (opt.code)}
                          <tr class="border-b border-slate-100 last:border-b-0">
                            <td class="whitespace-nowrap px-2.5 py-1.5 font-mono font-semibold text-[#003366]">{opt.code}</td>
                            <td class="whitespace-normal px-2.5 py-1.5 text-[#313132]">{opt.label}</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                </section>
              {/if}

              {#if lookup.relatedGuides.length > 0}
                <section aria-label="Related guides" class="space-y-1.5 border-t border-slate-200 pt-2.5">
                  <h4 class="text-[11px] font-bold uppercase tracking-wide text-slate-600">Related Workflow Guides</h4>
                  <div class="flex flex-wrap gap-1.5">
                    {#each lookup.relatedGuides as guide (guide.id)}
                      <button
                        type="button"
                        class="inline-flex items-center gap-1 rounded border border-slate-300 bg-[#F2F2F2] px-2.5 py-1 text-[11px] font-medium text-[#003366] hover:bg-slate-200"
                        onclick={() => openGuide(guide)}
                      >
                        <FileText size={12} /> {guide.title}
                      </button>
                    {/each}
                  </div>
                </section>
              {/if}
            </article>
          {/if}
        </div>

      {:else if activeTab === 'keys'}
        <div class="space-y-4" data-help-keys-panel>
          <div class="flex flex-wrap gap-1.5" role="group" aria-label="Select LMH25 Decision Key">
            {#each bundle?.decisionKeys ?? [] as dk (dk.id)}
              <button
                type="button"
                data-help-key-pill={dk.id}
                aria-pressed={selectedKeyId === dk.id}
                class="rounded border px-2.5 py-1.5 text-xs font-semibold {selectedKeyId === dk.id ? 'border-[#003366] bg-[#003366] text-white' : 'border-slate-300 bg-white text-[#003366] hover:bg-[#F2F2F2]'}"
                onclick={() => selectDecisionKey(dk.id)}
              >
                {dk.code}
              </button>
            {/each}
          </div>

          {#if activeDecisionKey}
            <section class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] bg-white p-3.5">
              <div class="flex items-start justify-between gap-2">
                <div>
                  <div class="font-mono text-[11px] font-semibold text-[#003366]">{activeDecisionKey.code}</div>
                  <h3 class="text-sm font-bold text-[#313132]">{activeDecisionKey.title}</h3>
                </div>
                <span class="shrink-0 rounded bg-[#F2F2F2] px-2 py-0.5 font-mono text-[11px] text-slate-700">
                  LMH25 p. {activeDecisionKey.lmh25Page}
                </span>
              </div>

              {#if isDichotomousDecisionKey(activeDecisionKey) && keyEval}
                <div class="space-y-3 rounded border border-slate-200 bg-[#F2F2F2] p-3" data-decision-key-evaluator={activeDecisionKey.id}>
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-[11px] font-bold uppercase text-[#003366]">
                      Interactive Yes / No Key Evaluator
                    </span>
                    <button
                      type="button"
                      data-decision-key-reset
                      class="inline-flex items-center gap-1 rounded border border-slate-300 bg-white px-2 py-1 text-[11px] font-medium text-[#313132] hover:bg-slate-100"
                      onclick={resetKey}
                    >
                      <RotateCcw size={12} /> Reset Key
                    </button>
                  </div>

                  {#if keyEval.completed && keyEval.conclusion}
                    <div role="status" data-decision-key-conclusion class="rounded border border-[#2E8540] bg-emerald-50 p-3 text-[#1E5128]">
                      <div class="flex items-center gap-1.5 text-[11px] font-bold uppercase">
                        <CheckCircle2 size={14} /> Key Determination Reached
                      </div>
                      <div class="mt-1 font-mono text-sm font-bold text-[#003366]">{keyEval.conclusion}</div>
                    </div>
                  {:else if currentKeyStepObj}
                    <div class="rounded border border-slate-300 bg-white p-3">
                      <div class="font-mono text-[11px] font-semibold text-[#003366]">
                        Step {currentKeyStepObj.step} of {activeDecisionKey.steps.length}
                      </div>
                      <p class="mt-1 text-xs font-medium text-[#313132]">{currentKeyStepObj.prompt}</p>
                      <div class="mt-3 flex flex-wrap gap-2">
                        <button
                          type="button"
                          data-decision-key-yes
                          class="rounded bg-[#003366] px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
                          onclick={() => answerKey('yes')}
                        >
                          Yes ({currentKeyStepObj.yes})
                        </button>
                        <button
                          type="button"
                          data-decision-key-no
                          class="rounded border border-[#003366] bg-white px-3.5 py-1.5 text-xs font-semibold text-[#003366] hover:bg-slate-50"
                          onclick={() => answerKey('no')}
                        >
                          No ({currentKeyStepObj.no})
                        </button>
                      </div>
                    </div>
                  {/if}

                  {#if keyEval.history.length > 0}
                    <ol class="space-y-1 text-[11px] text-slate-700" aria-label="Key evaluation path">
                      {#each keyEval.history as item (item.step)}
                        <li class="flex items-baseline justify-between gap-2 rounded bg-white px-2 py-1">
                          <span>Step {item.step}: <strong>{item.choice.toUpperCase()}</strong></span>
                          <span class="font-mono text-[#003366]">{item.outcome}</span>
                        </li>
                      {/each}
                    </ol>
                  {/if}
                </div>
              {/if}

              {#if activeDecisionKey.id === 'key.cover_chart'}
                <div class="space-y-2.5" data-cover-comparison-chart>
                  <p class="text-xs text-slate-700">
                    Standard <strong>10 m × 10 m (100 m²)</strong> macroplot foliar crown-projection comparison plates calibrated to <em>LMH25 (2023)</em> Appendix p. 274:
                  </p>
                  <div class="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
                    {#each COVER_COMPARISON_BLOCKS as block (block.percent)}
                      <div class="rounded border border-slate-300 bg-[#F2F2F2] p-2" data-cover-plate={block.label}>
                        <div class="mb-1 flex items-baseline justify-between">
                          <span class="font-mono text-xs font-bold text-[#003366]">{block.label}</span>
                          <span class="font-mono text-[10px] text-slate-600">{block.areaM2}</span>
                        </div>
                        <svg
                          viewBox="0 0 100 100"
                          class="aspect-square w-full rounded border border-slate-400 bg-white"
                          role="img"
                          aria-label={`${block.label} foliar cover reference square (${block.equivalentSquare})`}
                        >
                          <line x1="50" y1="0" x2="50" y2="100" stroke="#cbd5e1" stroke-width="0.7" stroke-dasharray="2,2" />
                          <line x1="0" y1="50" x2="100" y2="50" stroke="#cbd5e1" stroke-width="0.7" stroke-dasharray="2,2" />
                          {#each block.patches as p}
                            <rect x={p.x} y={p.y} width={p.w} height={p.h} rx="1" fill="#003366" />
                          {/each}
                        </svg>
                        <div class="mt-1 font-mono text-[10px] text-slate-600">{block.equivalentSquare}</div>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}

              <details class="rounded border border-slate-200 bg-white p-2.5">
                <summary class="cursor-pointer text-xs font-semibold text-[#003366]">
                  Full {activeDecisionKey.code} Step Table ({activeDecisionKey.steps.length} steps)
                </summary>
                <ol class="mt-2 space-y-2 border-t border-slate-200 pt-2">
                  {#each activeDecisionKey.steps as step (step.step)}
                    <li class="rounded bg-[#F2F2F2] p-2">
                      <div class="font-mono text-[11px] font-bold text-[#003366]">Step {step.step}</div>
                      <div class="mt-0.5 text-[#313132]">{step.prompt}</div>
                      {#if step.yes || step.no}
                        <div class="mt-1 flex flex-wrap gap-3 font-mono text-[11px]">
                          {#if step.yes}<span class="text-[#2E8540]">Yes → {step.yes}</span>{/if}
                          {#if step.no}<span class="text-slate-700">No → {step.no}</span>{/if}
                        </div>
                      {/if}
                    </li>
                  {/each}
                </ol>
              </details>
            </section>
          {/if}
        </div>

      {:else if activeTab === 'guides'}
        <div class="space-y-3" data-help-guides-panel>
          {#if onstarttour}
            <div class="flex items-center justify-between gap-2 rounded border border-[#FCBA19] bg-amber-50/50 p-3">
              <div>
                <div class="text-xs font-bold text-[#003366]">Coming from Microsoft Access VPro 64?</div>
                <div class="text-[11px] text-slate-700">Launch the 4 interactive spotlight walkthroughs on Sample.db.</div>
              </div>
              <button
                type="button"
                class="shrink-0 inline-flex items-center gap-1 rounded bg-[#003366] px-2.5 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
                onclick={() => onstarttour()}
              >
                <Compass size={13} /> 4 Tours
              </button>
            </div>
          {/if}

          <div>
            <label for="help-guide-selector" class="block text-[11px] font-semibold uppercase tracking-wide text-slate-600">
              Workflow &amp; Architecture Guides
            </label>
            <select
              id="help-guide-selector"
              class="mt-1 w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs text-[#313132]"
              value={selectedGuideId}
              onchange={(e) => { selectedGuideId = e.currentTarget.value; }}
            >
              {#each bundle?.guides ?? [] as g (g.id)}
                <option value={g.id}>{g.category} — {g.title}</option>
              {/each}
            </select>
          </div>

          {#if activeGuide}
            <article class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#003366] bg-white p-3.5" data-help-active-guide={activeGuide.id}>
              <div class="font-mono text-[11px] font-semibold uppercase text-[#003366]">{activeGuide.category}</div>
              <h3 class="text-sm font-bold text-[#313132]">{activeGuide.title}</h3>
              <p class="rounded bg-[#F2F2F2] p-2.5 text-xs text-slate-700">{activeGuide.summary}</p>
              <div class="space-y-3">
                {#each activeGuide.sections as sec (sec.heading)}
                  <section class="space-y-1 border-l-2 border-[#FCBA19] pl-3">
                    <h4 class="text-xs font-bold text-[#003366]">{sec.heading}</h4>
                    <p class="text-xs text-[#313132]">{sec.body}</p>
                  </section>
                {/each}
              </div>
            </article>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}
