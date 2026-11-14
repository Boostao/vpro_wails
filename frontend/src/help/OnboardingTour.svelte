<script lang="ts">
  import { onDestroy, tick, untrack } from 'svelte';
  import { CheckCircle2, ChevronLeft, ChevronRight, Compass, RotateCcw, ShieldCheck, X } from '@lucide/svelte';
  import { defaultHelpPort, type HelpBackendPort, type HelpTour } from './helpSystem';

  let {
    open = false,
    initialTourId = null,
    activeProject = 'Sample',
    helpPort = defaultHelpPort,
    onnavigateview,
    onensurecontextopen,
    onresetsample,
    onclose,
  }: {
    open?: boolean;
    initialTourId?: string | null;
    activeProject?: string;
    helpPort?: HelpBackendPort;
    onnavigateview: (viewName: string) => void;
    onensurecontextopen?: () => void;
    onresetsample?: () => Promise<void> | void;
    onclose: () => void;
  } = $props();

  let tours = $state.raw<HelpTour[]>([]);
  let activeTourId = $state<string | null>(untrack(() => initialTourId));
  let stepIndex = $state(0);
  let targetRect = $state<{ top: number; left: number; width: number; height: number } | null>(null);
  let loading = $state(false);
  let error = $state('');
  let completedTourTitle = $state('');

  let loadGen = 0;

  $effect(() => {
    const requestedTour = initialTourId;
    untrack(() => {
      if (requestedTour) {
        activeTourId = requestedTour;
        stepIndex = 0;
        completedTourTitle = '';
      }
    });
  });

  $effect(() => {
    if (!open) return;
    untrack(() => {
      void loadTours();
    });
  });

  async function loadTours() {
    if (tours.length > 0) {
      if (activeTourId) await syncActiveStepSideEffects();
      return;
    }
    const gen = ++loadGen;
    loading = true;
    error = '';
    try {
      const bundle = await helpPort.getHelpBundle();
      if (gen !== loadGen) return;
      tours = bundle.tours ?? [];
      if (activeTourId) {
        await syncActiveStepSideEffects();
      }
    } catch (cause) {
      if (gen === loadGen) {
        error = `Guided tours could not load: ${String(cause)}`;
      }
    } finally {
      if (gen === loadGen) {
        loading = false;
      }
    }
  }

  const activeTour = $derived(tours.find((t) => t.id === activeTourId) ?? null);
  const activeStep = $derived(activeTour ? activeTour.steps[stepIndex] ?? null : null);

  async function syncActiveStepSideEffects() {
    const step = activeTour?.steps[stepIndex];
    if (!step) {
      targetRect = null;
      return;
    }
    if (step.target === '#project-context') {
      onensurecontextopen?.();
    }
    if (step.view) {
      onnavigateview(step.view);
    }
    await tick();
    measureSpotlightTarget(step.target);
  }

  function measureSpotlightTarget(selector: string) {
    if (typeof document === 'undefined' || !selector) {
      targetRect = null;
      return;
    }
    try {
      const el = document.querySelector(selector);
      if (!el || typeof el.getBoundingClientRect !== 'function') {
        targetRect = null;
        return;
      }
      const rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) {
        targetRect = null;
        return;
      }
      targetRect = {
        top: Math.max(4, rect.top - 4),
        left: Math.max(4, rect.left - 4),
        width: rect.width + 8,
        height: rect.height + 8,
      };
    } catch {
      targetRect = null;
    }
  }

  async function startTour(tourId: string) {
    completedTourTitle = '';
    activeTourId = tourId;
    stepIndex = 0;
    await syncActiveStepSideEffects();
  }

  async function previousStep() {
    if (!activeTour || stepIndex <= 0) return;
    stepIndex -= 1;
    await syncActiveStepSideEffects();
  }

  async function nextStep() {
    if (!activeTour) return;
    if (stepIndex + 1 < activeTour.steps.length) {
      stepIndex += 1;
      await syncActiveStepSideEffects();
    } else {
      completedTourTitle = activeTour.title;
      activeTourId = null;
      stepIndex = 0;
      targetRect = null;
    }
  }

  function returnToCatalog() {
    activeTourId = null;
    stepIndex = 0;
    targetRect = null;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      onclose();
    }
  }

  onDestroy(() => {
    loadGen++;
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  {#if activeTour && activeStep}
    {#if targetRect}
      <div
        aria-hidden="true"
        data-tour-spotlight-ring
        class="pointer-events-none fixed z-40 rounded border-[3px] border-[#FCBA19] shadow-[0_0_0_9999px_rgba(0,25,51,0.38)] transition-all duration-150"
        style={`top:${targetRect.top}px;left:${targetRect.left}px;width:${targetRect.width}px;height:${targetRect.height}px;`}
      ></div>
    {/if}

    <div
      role="dialog"
      aria-modal="false"
      aria-labelledby="onboarding-tour-step-title"
      data-onboarding-tour-active={activeTour.id}
      class="fixed right-5 bottom-5 z-50 w-full max-w-md rounded border border-slate-300 border-t-[4px] border-t-[#FCBA19] bg-white text-[#313132] shadow-2xl"
    >
      <header class="flex items-center justify-between gap-2 bg-[#003366] px-4 py-2.5 text-white">
        <div class="min-w-0">
          <div class="font-mono text-[11px] text-[#FCBA19]">
            {activeTour.title} · Step {stepIndex + 1} of {activeTour.steps.length}
          </div>
        </div>
        <button
          type="button"
          aria-label="Dismiss Guided Tour"
          data-tour-dismiss
          class="rounded p-1 text-white/90 hover:bg-white/15"
          onclick={onclose}
        >
          <X size={15} />
        </button>
      </header>

      <div class="h-1.5 w-full bg-[#F2F2F2]">
        <div
          class="h-full bg-[#FCBA19] transition-all"
          style={`width:${Math.round(((stepIndex + 1) / activeTour.steps.length) * 100)}%`}
        ></div>
      </div>

      <div class="space-y-3 p-4 text-xs">
        <div class="flex items-center justify-between gap-2">
          <h3 id="onboarding-tour-step-title" class="text-sm font-bold text-[#003366]">{activeStep.title}</h3>
          <span class="rounded bg-[#F2F2F2] px-2 py-0.5 font-mono text-[10px] text-slate-700">
            View: {activeStep.view}
          </span>
        </div>

        <p class="leading-relaxed text-[#313132]">{activeStep.body}</p>

        <div class="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 pt-3">
          <button
            type="button"
            class="rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-[#003366] hover:bg-[#F2F2F2]"
            onclick={returnToCatalog}
          >
            All 4 Tours
          </button>

          <div class="flex items-center gap-2">
            <button
              type="button"
              data-tour-back
              disabled={stepIndex === 0}
              class="inline-flex items-center gap-1 rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-semibold text-[#313132] hover:bg-[#F2F2F2] disabled:opacity-45"
              onclick={() => void previousStep()}
            >
              <ChevronLeft size={14} /> Back
            </button>
            <button
              type="button"
              data-tour-next
              class="inline-flex items-center gap-1 rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
              onclick={() => void nextStep()}
            >
              {#if stepIndex + 1 < activeTour.steps.length}
                Next <ChevronRight size={14} />
              {:else}
                Finish Tour <CheckCircle2 size={14} />
              {/if}
            </button>
          </div>
        </div>
      </div>
    </div>

  {:else}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-onboarding-tour-catalog>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="onboarding-catalog-title"
        class="flex max-h-[88vh] w-full max-w-2xl flex-col overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
      >
        <header class="flex items-center justify-between gap-3 border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-3.5 text-white">
          <div class="flex items-center gap-2.5">
            <Compass size={18} class="text-[#FCBA19]" />
            <div>
              <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">
                Coming from VPro 64 · Non-Destructive Walkthroughs
              </div>
              <h2 id="onboarding-catalog-title" class="text-base font-semibold text-white">
                Interactive VPro 2.0 Guided Tours (4 Modules)
              </h2>
            </div>
          </div>
          <button
            type="button"
            aria-label="Close Guided Tours"
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
          {#if completedTourTitle}
            <div role="status" class="flex items-center gap-2 rounded border border-[#2E8540] bg-emerald-50 p-3 text-[#1E5128]">
              <CheckCircle2 size={16} />
              <span>Completed <strong>{completedTourTitle}</strong>. Choose another walkthrough below or close to continue working.</span>
            </div>
          {/if}

          <div class="flex flex-wrap items-center justify-between gap-2 rounded border border-slate-300 bg-[#F2F2F2] p-3">
            <div class="flex items-center gap-2">
              <ShieldCheck size={16} class="shrink-0 text-[#2E8540]" />
              <span>
                Active project: <strong class="font-mono text-[#003366]">{activeProject}</strong>.
                Walkthroughs navigate read-only views safely without mutating user databases.
              </span>
            </div>
            {#if activeProject !== 'Sample' && onresetsample}
              <button
                type="button"
                data-tour-switch-sample
                class="inline-flex items-center gap-1 rounded border border-[#003366] bg-white px-2.5 py-1.5 text-xs font-semibold text-[#003366] hover:bg-slate-50"
                onclick={() => void onresetsample()}
              >
                <RotateCcw size={12} /> Switch to Sample.db
              </button>
            {/if}
          </div>

          {#if loading}
            <p role="status" class="text-slate-600">Loading guided tour definitions...</p>
          {:else}
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {#each tours as tour (tour.id)}
                <article
                  data-tour-card={tour.id}
                  class="flex flex-col justify-between rounded border border-slate-300 border-t-[3px] border-t-[#003366] bg-white p-3.5"
                >
                  <div class="space-y-1.5">
                    <div class="flex items-center justify-between gap-2">
                      <span class="rounded bg-[#F2F2F2] px-2 py-0.5 font-mono text-[11px] font-semibold text-[#003366]">
                        {tour.duration}
                      </span>
                      <span class="font-mono text-[11px] text-slate-500">{tour.steps.length} steps</span>
                    </div>
                    <h3 class="text-sm font-bold text-[#003366]">{tour.title}</h3>
                    <p class="text-xs leading-relaxed text-slate-700">{tour.description}</p>
                  </div>

                  <button
                    type="button"
                    data-start-tour={tour.id}
                    class="mt-3 inline-flex w-full items-center justify-center gap-1.5 rounded bg-[#003366] px-3 py-2 text-xs font-semibold text-white hover:bg-[#002244]"
                    onclick={() => void startTour(tour.id)}
                  >
                    Start Walkthrough <ChevronRight size={14} />
                  </button>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
{/if}
