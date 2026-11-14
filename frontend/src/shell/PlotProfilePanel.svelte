<script lang="ts">
  import { tick } from 'svelte';
  import {
    ContextService,
    type PlotPage,
    type PlotProfileSource,
    type PlotProfileSourceInfo,
    type PlotSummary,
    type ProjectPlotProfileFilterRequest,
  } from '../../bindings/github.com/boostao/vpro-wails';
  import { projectState } from '../shared/state';
  import type { ReadRequests } from '../shared/readRequests';
  import {
    validatePlotProfileSource,
    validateProjectPlotProfileReview,
    type ProfileNavigation,
    type ValidatedProjectPlotProfileReview,
  } from '../classification/projectPlotProfileReview';
  import { validateProfileSUReview, validateProfileSUCreated, type ReviewedProfileSU } from '../classification/profileSU';
  import { validateProfileFileReview, validateProfileFileCreated, type ReviewedProfileFile } from '../classification/profileFile';
  import { validateProfileTableReview, validateProfileTableCreated, type ReviewedProfileTable } from '../classification/profileTable';
  import { validateProjectSUReview, validateProjectSUCreated, type ReviewedProjectSU } from '../classification/profileSUProject';

  let {
    view = $bindable(),
    busy = $bindable(false),
    error = $bindable(''),
    editorPlotNumber = $bindable(undefined),
    profileNavigation = $bindable(null),
    selected = $bindable(null),
    page = $bindable(null),
    editorBusy,
    vegetationPublicationBusy,
    lifeformPublicationBusy,
    summaryPublicationBusy,
    transitionWorking,
    pendingTransition,
    closeRequest,
    profileIndex,
    stateReads,
    requestTransition,
    loadPlots,
    navigateProfilePlot,
    clearProfileNavigation,
    cancelAllReads,
  }: {
    view: string;
    busy: boolean;
    error: string;
    editorPlotNumber: string | undefined;
    profileNavigation: ProfileNavigation | null;
    selected: PlotSummary | null;
    page: PlotPage | null;
    editorBusy: boolean;
    vegetationPublicationBusy: boolean;
    lifeformPublicationBusy: boolean;
    summaryPublicationBusy: boolean;
    transitionWorking: boolean;
    pendingTransition: unknown;
    closeRequest: string;
    profileIndex: number;
    stateReads: ReadRequests;
    requestTransition: (run: (contextId: string) => Promise<void>) => Promise<void>;
    loadPlots: (nextOffset: number) => Promise<void>;
    navigateProfilePlot: (delta: number) => Promise<void>;
    clearProfileNavigation: () => Promise<void>;
    cancelAllReads: () => void;
  } = $props();

  const profileSelectionEnabled = true;
  const profileWriteOwnershipEnabled = true;
  const profileFileCreationEnabled = true;
  const profileTableCreationEnabled = true;
  const suProjectEnabled = true;

  let profileTableReview = $state<ReviewedProfileTable | null>(null);
  let profileTableContext = $state('');
  let profileTableName = $state('');
  let profileTableError = $state('');
  let profileTableSuccess = $state('');
  let profileTableCommitted = $state(false);
  let profileFileReview = $state<ReviewedProfileFile | null>(null);
  let profileFileContext = $state('');
  let profileFileName = $state('');
  let profileFilePath = $state('');
  let profileFileError = $state('');
  let profileFileSuccess = $state('');
  let profileFileCommitted = $state(false);
  let profileWriteReview = $state<ValidatedProjectPlotProfileReview | null>(null);
  let profileWriteContext = $state('');
  let profileWriteError = $state('');
  let profilePath = $state('');
  let profileSources = $state<PlotProfileSourceInfo[]>([]);
  let profileSourceContext = $state('');
  let profileSourceError = $state('');
  let suReview = $state<ReviewedProfileSU | null>(null);
  let suReviewContext = $state('');
  let suName = $state('');
  let suPath = $state('');
  let suError = $state('');
  let suSuccess = $state('');
  let suCommitted = $state(false);
  let suProjectReview = $state<ReviewedProjectSU | null>(null);
  let suProjectContext = $state('');
  let suProjectName = $state('');
  let suProjectError = $state('');
  let suProjectSuccess = $state('');
  let suProjectCommitted = $state(false);

  export function clearCachedSources() {
    profileSources = [];
    profileSourceContext = '';
  }

  export function resetAllReviews() {
    profileSources = [];
    profileSourceContext = '';
    profileWriteReview = null;
    profileWriteContext = '';
    profileWriteError = '';
    suReview = null;
    suReviewContext = '';
    suError = '';
    suSuccess = '';
    suCommitted = false;
    suProjectReview = null;
    suProjectContext = '';
    suProjectError = '';
    suProjectSuccess = '';
    suProjectCommitted = false;
  }

  async function inspectProfileSources() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileSourceError = 'Wait for the current operation or decision before inspecting profile sources.'; return;
    }
    busy = true; profileSourceError = '';
    try {
      await tick();
      const sources = await stateReads.track(ContextService.ListPlotProfileSources(contextId, profilePath));
      if ($projectState?.contextId !== contextId) throw new Error('Profile inspection belongs to an old context; inspect again.');
      if (!Array.isArray(sources)) throw new Error('Profile source inspection returned an incomplete collection.');
      const next = sources.map(validatePlotProfileSource);
      profileSources = next; profileSourceContext = contextId;
      if (next.length === 0) profileSourceError = 'No original physical *_Profile tables were found; selection remains unchanged.';
    } catch (cause) { profileSourceError = `Profile sources were not replaced: ${String(cause)}`; }
    finally { busy = false; }
  }

  export async function reviewProfileSU(proposal: ProjectPlotProfileFilterRequest, origin: string) {
    if (busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest ||
        $projectState?.contextId !== origin) throw new Error('Wait for the current owned operation before reviewing Save as SU.');
    busy = true; suError = '';
    try {
      await tick();
      const source = await stateReads.track(ContextService.ReviewProjectPlotProfileSU(origin, proposal));
      if ($projectState?.contextId !== origin) throw new Error('Save as SU review belongs to an old context.');
      suReview = validateProfileSUReview(source, proposal); suReviewContext = origin;
      suCommitted = false; suSuccess = '';
    } finally { busy = false; }
  }

  async function saveProfileSU() {
    if (!suReview || suCommitted || suReviewContext !== $projectState?.contextId) {
      suError = 'Review the current stored profile before creating one new SU file.'; return;
    }
    const proposal = $state.snapshot({ review: suReview, name: suName, path: suPath, confirmed: true });
    const origin = suReviewContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Save as SU proposal belongs to an old context.');
      busy = true; suError = '';
      try {
        await tick();
        const response = await ContextService.SaveProjectPlotProfileSU(contextId, proposal);
        suCommitted = true;
        const created = validateProfileSUCreated(response, proposal.name, proposal.review.plots.length);
        suSuccess = `Created ${created.name}_SU with ${created.plotCount} stored plots in ${created.path}. Current project/SU selection is unchanged.`;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `SU file published, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (suCommitted || String(cause).includes('Profile SU file published, but')) {
          suCommitted = true; suError = `SU file published; do not replay creation: ${String(cause)}`;
        } else suError = `No SU file published; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function reviewProfileSUProject() {
    const state = $projectState;
    if (!state?.contextId || !suReview || suReviewContext !== state.contextId ||
        busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      suProjectError = 'Wait for the current operation and review the current profile before reviewing a project-file SU.'; return;
    }
    if (!state.projectPath) {
      suProjectError = 'Current project has no owned physical destination; no SU table can be created.'; return;
    }
    const proposal = $state.snapshot(suReview.filter);
    busy = true; suProjectError = '';
    try {
      await tick();
      const source = await stateReads.track(ContextService.ReviewProjectPlotProfileSUInProject(state.contextId, proposal));
      if ($projectState?.contextId !== state.contextId) throw new Error('Project-file SU review belongs to an old context.');
      suProjectReview = validateProjectSUReview(source, proposal, state.activeProject, state.projectPath);
      suProjectContext = state.contextId; suProjectCommitted = false; suProjectSuccess = '';
    } catch (cause) { suProjectError = `No SU table created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function saveProfileSUProject() {
    if (!suProjectReview || suProjectCommitted || suProjectContext !== $projectState?.contextId) {
      suProjectError = 'Review the current project destination before creating one SU table.'; return;
    }
    const proposal = $state.snapshot({ review: suProjectReview, name: suProjectName, confirmed: true });
    const origin = suProjectContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Project-file SU proposal belongs to an old context.');
      busy = true; suProjectError = '';
      try {
        await tick();
        const response = await ContextService.SaveProjectPlotProfileSUInProject(contextId, proposal);
        suProjectCommitted = true;
        const created = validateProjectSUCreated(response, proposal.name, proposal.review.su.plots.length, proposal.review.path);
        suProjectSuccess = `Created ${created.name}_SU with ${created.plotCount} stored plots in ${created.path}. Existing data/descriptions and project/SU selection are unchanged; select it separately.`;
        view = 'plots'; editorPlotNumber = undefined;
        const state = await stateReads.track(ContextService.GetState());
        if (state.contextId !== contextId || $projectState?.contextId !== contextId) {
          throw new Error('SU table committed, but context changed before destination discovery refresh; do not replay creation.');
        }
        projectState.set(state);
        await loadPlots(0);
        if (error) error = `SU table committed, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (suProjectCommitted || String(cause).includes('Profile SU table committed, but')) {
          suProjectCommitted = true; suProjectError = `SU table committed; do not replay creation: ${String(cause)}`;
        } else suProjectError = `No SU table created; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function choosePlotProfile(source: PlotProfileSource, origin: string) {
    const proposal = { name: source.name, path: source.path };
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Profile source belongs to an old context; inspect again.');
      busy = true;
      try {
        await tick();
        const state = await ContextService.SelectPlotProfile(contextId, proposal);
        cancelAllReads();
        projectState.set(state); profileNavigation = null; profileSources = []; profileSourceContext = '';
        selected = null; page = null; editorPlotNumber = undefined; view = 'plots'; error = '';
        await loadPlots(0);
        if (error) error = `Profile selection committed, but refresh failed; do not replay selection: ${error}`;
      } finally { busy = false; }
    });
  }

  async function reviewProfileWriteOwnership() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) return;
    busy = true; profileWriteError = '';
    try {
      await tick();
      const review = await stateReads.track(ContextService.ReviewProjectPlotProfile(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Profile ownership review belongs to an old context.');
      profileWriteReview = validateProjectPlotProfileReview(review); profileWriteContext = contextId;
    } catch (cause) { profileWriteError = `Profile editing authorization unchanged: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function publishProfileWriteOwnership() {
    if (!profileWriteReview || profileWriteContext !== $projectState?.contextId) {
      profileWriteError = 'Review the current selected profile before changing write authorization.'; return;
    }
    const proposal = $state.snapshot({ review: profileWriteReview, enabled: !profileWriteReview.source.writable, confirmed: true });
    const origin = profileWriteContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Profile ownership review belongs to an old context.');
      busy = true; profileWriteError = '';
      let published = false;
      try {
        await tick();
        const state = await ContextService.SetPlotProfileEditing(contextId, proposal);
        published = true;
        cancelAllReads();
        projectState.set(state); profileNavigation = null; profileSources = []; profileSourceContext = '';
        profileWriteReview = null; profileWriteContext = '';
        selected = null; page = null; editorPlotNumber = undefined; view = 'plots'; error = '';
        await loadPlots(0);
        if (error) error = `Profile authorization committed, but refresh failed; do not replay authorization: ${error}`;
      } catch (cause) {
        profileWriteError = published ? `Profile authorization committed; do not replay: ${String(cause)}`
          : `Profile authorization unchanged; review retained for retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function reviewBlankProfileFile() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileFileError = 'Wait for the current operation or decision before reviewing a new profile file.'; return;
    }
    busy = true; profileFileError = '';
    try {
      await tick();
      const response = await stateReads.track(ContextService.ReviewPlotProfileFileCreation(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Blank profile review belongs to an old context.');
      profileFileReview = validateProfileFileReview(response); profileFileContext = contextId;
      profileFileCommitted = false; profileFileSuccess = '';
    } catch (cause) { profileFileError = `No profile file created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function createBlankProfileFile() {
    if (!profileFileReview || profileFileCommitted || profileFileContext !== $projectState?.contextId) {
      profileFileError = 'Review the current original empty template before creating one new profile file.'; return;
    }
    const proposal = $state.snapshot({ review: profileFileReview, name: profileFileName, path: profileFilePath, confirmed: true });
    const origin = profileFileContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Blank profile proposal belongs to an old context.');
      busy = true; profileFileError = '';
      try {
        await tick();
        const response = await ContextService.CreatePlotProfileFile(contextId, proposal);
        profileFileCommitted = true;
        const created = validateProfileFileCreated(response, proposal.name);
        profileFileSuccess = `Created ${created.table} with zero rules in ${created.source.path}. Inspect, select and authorize it explicitly to begin editing; current context selection is unchanged.`;
        profilePath = created.source.path;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `Profile file published, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (profileFileCommitted || String(cause).includes('Profile file published, but')) {
          profileFileCommitted = true; profileFileError = `Profile file published; do not replay creation: ${String(cause)}`;
        } else profileFileError = `No profile file published; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function reviewBlankProfileTable() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileTableError = 'Wait for the current operation or decision before reviewing an existing-file profile.'; return;
    }
    busy = true; profileTableError = '';
    try {
      await tick();
      const response = await stateReads.track(ContextService.ReviewPlotProfileTableCreation(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Existing-file profile review belongs to an old context.');
      profileTableReview = validateProfileTableReview(response); profileTableContext = contextId;
      profileTableCommitted = false; profileTableSuccess = '';
    } catch (cause) { profileTableError = `No profile table created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function createBlankProfileTable() {
    if (!profileTableReview || profileTableCommitted || profileTableContext !== $projectState?.contextId) {
      profileTableError = 'Review current selected-profile ownership and the original template before creating one table.'; return;
    }
    const proposal = $state.snapshot({ review: profileTableReview, name: profileTableName, confirmed: true });
    const origin = profileTableContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Existing-file profile proposal belongs to an old context.');
      busy = true; profileTableError = '';
      try {
        await tick();
        const response = await ContextService.CreatePlotProfileTable(contextId, proposal);
        profileTableCommitted = true;
        const created = validateProfileTableCreated(response, proposal.name, proposal.review.profile.source.source.path);
        profileTableSuccess = `Created ${created.table} with zero rules in ${created.source.path}. Existing data/descriptions and current selection are unchanged. Inspect and select it explicitly; selection clears external session grants.`;
        profilePath = created.source.path;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `Profile table committed, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (profileTableCommitted || String(cause).includes('Profile table committed, but')) {
          profileTableCommitted = true; profileTableError = `Profile table committed; do not replay creation: ${String(cause)}`;
        } else profileTableError = `No profile table created; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }
</script>

{#if suReview}
  <section class="m-2 p-3 border rounded" aria-label="Reviewed Save as SU">
    <h2 class="font-semibold">Save reviewed profile as a new SU file</h2>
    <p class="mt-2">{suReview.plots.length} stored plots; SiteUnit values {suReview.sourceSU ? 'copied from the independently reviewed selected SU' : 'remain NULL because no SU is selected'}.</p>
    <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
      <label for="profile-su-name" class="flex flex-col gap-1">New SU name
        <input id="profile-su-name" class="px-2 py-1 border rounded min-w-0" bind:value={suName} disabled={busy || transitionWorking || pendingTransition !== null || suCommitted} />
      </label>
      <label for="profile-su-path" class="flex flex-col gap-1">New SQLite file (absolute .db path)
        <input id="profile-su-path" class="px-2 py-1 border rounded min-w-0" bind:value={suPath} disabled={busy || transitionWorking || pendingTransition !== null || suCommitted} />
      </label>
    </div>
    {#if suError}<p role="alert" class="mt-2 text-red-800">{suError}</p>{/if}
    {#if suSuccess}<p role="status" class="mt-2 text-emerald-800">{suSuccess}</p>{/if}
    <div class="mt-3 overflow-x-auto">
      <table class="w-full text-sm"><thead><tr><th class="text-left">Plot number</th><th class="text-left">SiteUnit</th></tr></thead>
        <tbody>{#each suReview.plots.slice(0, 25) as plot}<tr><td>{plot.plotNumber}</td><td>{plot.siteUnit === null ? 'NULL (no value)' : plot.siteUnit === '' ? 'Empty text' : plot.siteUnit}</td></tr>{/each}</tbody>
      </table>
      {#if suReview.plots.length > 25}<p>Showing the first 25 of {suReview.plots.length} explicitly reviewed rows.</p>{/if}
    </div>
    <div class="mt-3 flex flex-wrap gap-2">
      <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
        disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suCommitted || !suName || !suPath || suReviewContext !== $projectState?.contextId}
        onclick={() => void saveProfileSU()}>Create reviewed SU file</button>
      <button type="button" class="px-3 py-2 border rounded disabled:opacity-50" disabled={busy || transitionWorking || pendingTransition !== null}
        onclick={() => { suReview = null; suError = ''; suSuccess = ''; }}>Dismiss SU review</button>
    </div>
    <p class="mt-3 text-sm">Creates a new SQLite SU database without overwriting existing files or switching the active project/SU context. Table names follow the 31-character VPro identifier rule (None, Sample, and master names are reserved).</p>
    {#if suProjectEnabled}
      <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
        disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suReviewContext !== $projectState?.contextId}
        onclick={() => void reviewProfileSUProject()}>Review Save as SU in current project file</button>
      {#if suProjectReview}
        <section class="mt-3 p-3 border rounded" aria-label="Reviewed project-file SU creation">
          <h3 class="font-semibold">Add one SU table to the current owned project</h3>
          <p class="break-words">Destination: {suProjectReview.path}</p>
          <p>{suProjectReview.su.plots.length} stored plots and {suProjectReview.metadata.rows.length} original table descriptions reviewed. No descriptions are synthesized or removed.</p>
          <label for="profile-su-project-name" class="mt-3 flex flex-col gap-1">New project-file SU name
            <input id="profile-su-project-name" class="px-2 py-1 border rounded min-w-0" bind:value={suProjectName}
              disabled={busy || transitionWorking || pendingTransition !== null || suProjectCommitted} />
          </label>
          {#if suProjectSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{suProjectSuccess}</p>{/if}
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suProjectCommitted || !suProjectName || suProjectContext !== $projectState?.contextId}
              onclick={() => void saveProfileSUProject()}>Create reviewed SU in project file</button>
            <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
              onclick={() => { suProjectReview = null; suProjectError = ''; suProjectSuccess = ''; }}>Dismiss project-file SU review</button>
          </div>
          <p class="mt-3 text-sm">Adds only the reviewed SU, its original indexes and transactional creation provenance. No implicit selection/attachment, Master authorization or YAML write. Arbitrary destinations and present template-description copying are unavailable; use new-file creation for present metadata. The existing desktop name policy applies.</p>
        </section>
      {/if}
      {#if suProjectError}<p role="alert" class="mt-2 text-red-800">{suProjectError}</p>{/if}
    {/if}
  </section>
{/if}
{#if profileSelectionEnabled && $projectState?.contextId}
  <details class="m-2 p-3 border rounded" aria-label="Stored plot profile selection">
    <summary class="font-semibold cursor-pointer">Plot profile: {$projectState.plotProfile.source.name}</summary>
    <p class="mt-2 text-sm">Select an optional SQLite database containing saved plot profile tables.</p>
    <div class="mt-2 flex flex-wrap items-end gap-2">
      <label for="plot-profile-path" class="flex flex-col gap-1 flex-1 min-w-0">SQLite profile database path (blank inspects project and core)
        <input id="plot-profile-path" class="px-2 py-1 border rounded min-w-0" bind:value={profilePath} disabled={busy || transitionWorking || pendingTransition !== null} />
      </label>
      <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
        onclick={() => void inspectProfileSources()}>Inspect stored profiles</button>
      <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
        onclick={() => void choosePlotProfile({ name: 'None', path: '' }, $projectState?.contextId ?? '')}>Select no profile</button>
    </div>
    {#if profileSourceError}<p role="alert" class="mt-2 text-red-800">{profileSourceError}</p>{/if}
    <ul class="mt-2 space-y-2">
      {#each profileSources as info}
        <li class="flex flex-wrap items-center gap-2">
          <span>{info.table} - {info.source.path} ({info.writable ? 'project-owned editing' : 'read-only'})</span>
          <button type="button" class="px-3 py-1 border rounded disabled:opacity-50"
            disabled={!info.available || profileSourceContext !== $projectState.contextId || busy || editorBusy || transitionWorking || pendingTransition !== null}
            onclick={() => void choosePlotProfile(info.source, profileSourceContext)}>Select {info.table}</button>
          {#if !info.available}<span role="alert">{info.reason}</span>{/if}
        </li>
      {/each}
    </ul>
    {#if profileWriteOwnershipEnabled && $projectState.plotProfile.available &&
      ($projectState.plotProfile.source.name !== $projectState.activeProject || $projectState.plotProfile.source.path !== $projectState.projectPath)}
      <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
        disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
        onclick={() => void reviewProfileWriteOwnership()}>Review profile write ownership</button>
      {#if profileWriteReview}
        <section class="mt-3 p-3 border rounded" aria-label="Reviewed profile write ownership">
          <p>{profileWriteReview.table} in {profileWriteReview.source.source.path}</p>
          <p>{profileWriteReview.rules.rows.length} physical rules and {profileWriteReview.descriptions.rows.length} table descriptions reviewed.</p>
          {#if profileWriteReview.descriptions.columns.length === 0}<p>Description metadata is absent; no descriptions are inferred or created.</p>{/if}
          <p>Authorization applies only to this selected profile file/table for the current session. Context changes or restart clear it. Parent/child writes remain project-owned; support database writes are unavailable.</p>
          <button type="button" class="mt-2 px-3 py-2 border rounded disabled:opacity-50"
            disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileWriteContext !== $projectState.contextId}
            onclick={() => void publishProfileWriteOwnership()}>{profileWriteReview.source.writable ? 'Stop selected profile editing' : 'Authorize selected profile editing'}</button>
          <button type="button" class="mt-2 ml-2 px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
            onclick={() => { profileWriteReview = null; profileWriteError = ''; }}>Dismiss ownership review</button>
        </section>
      {/if}
      {#if profileWriteError}<p role="alert" class="mt-2 text-red-800">{profileWriteError}</p>{/if}
    {/if}
    <p class="mt-2 text-sm">Profile rules are independent of project/SU selection. External and other-table profiles are read-only unless explicitly authorized for this session.</p>
    {#if profileFileCreationEnabled}
      <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
        disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
        onclick={() => void reviewBlankProfileFile()}>Review new blank profile file</button>
      {#if profileFileReview}
        <section class="mt-3 p-3 border rounded" aria-label="Reviewed blank profile file creation">
          <h2 class="font-semibold">Create one new empty profile file</h2>
          <p>Original nine nullable fields, zero rules, no template indexes/triggers and absent description metadata. No descriptions or legacy population values are invented.</p>
          <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label for="profile-file-name" class="flex flex-col gap-1">New profile name
              <input id="profile-file-name" class="px-2 py-1 border rounded min-w-0" bind:value={profileFileName} disabled={busy || transitionWorking || pendingTransition !== null || profileFileCommitted} />
            </label>
            <label for="profile-file-path" class="flex flex-col gap-1">New SQLite file (absolute .db path)
              <input id="profile-file-path" class="px-2 py-1 border rounded min-w-0" bind:value={profileFilePath} disabled={busy || transitionWorking || pendingTransition !== null || profileFileCommitted} />
            </label>
          </div>
          {#if profileFileSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{profileFileSuccess}</p>{/if}
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileFileCommitted || !profileFileName || !profileFilePath || profileFileContext !== $projectState.contextId}
              onclick={() => void createBlankProfileFile()}>Create reviewed blank profile file</button>
            <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
              onclick={() => { profileFileReview = null; profileFileError = ''; profileFileSuccess = ''; }}>Dismiss blank profile review</button>
          </div>
          <p class="mt-3 text-sm">Creates a new SQLite profile database without overwriting existing files or altering active session selections. Profile names follow the 31-character ASCII VPro identifier rule (None, Sample, and master names are reserved).</p>
        </section>
      {/if}
      {#if profileFileError}<p role="alert" class="mt-2 text-red-800">{profileFileError}</p>{/if}
    {/if}
    {#if profileTableCreationEnabled && $projectState.plotProfile.writable}
      <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
        disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
        onclick={() => void reviewBlankProfileTable()}>Review blank profile in selected file</button>
      {#if profileTableReview}
        <section class="mt-3 p-3 border rounded" aria-label="Reviewed existing-file profile creation">
          <h2 class="font-semibold">Create one empty profile in the owned selected file</h2>
          <p class="break-words">Destination: {profileTableReview.profile.source.source.path}</p>
          <p>Current ownership, selected rules/descriptions and the original empty nine-field template are reviewed independently.</p>
          <label for="profile-table-name" class="mt-3 flex flex-col gap-1">New profile table name
            <input id="profile-table-name" class="px-2 py-1 border rounded min-w-0" bind:value={profileTableName}
              disabled={busy || transitionWorking || pendingTransition !== null || profileTableCommitted} />
          </label>
          {#if profileTableSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{profileTableSuccess}</p>{/if}
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileTableCommitted || !profileTableName || profileTableContext !== $projectState.contextId}
              onclick={() => void createBlankProfileTable()}>Create reviewed profile in selected file</button>
            <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
              onclick={() => { profileTableReview = null; profileTableError = ''; profileTableSuccess = ''; }}>Dismiss existing-file profile review</button>
          </div>
          <p class="mt-3 text-sm">Adds one table and transactional creation provenance; never replaces existing objects or creates descriptions. No automatic selection/attachment or new write grant. Arbitrary unselected destinations and legacy population remain unavailable. The existing desktop name policy applies.</p>
        </section>
      {/if}
      {#if profileTableError}<p role="alert" class="mt-2 text-red-800">{profileTableError}</p>{/if}
    {/if}
  </details>
{/if}
{#if profileNavigation}
  <section class="m-2 p-2 border rounded flex flex-wrap items-center gap-3" aria-label="Active profile navigation">
    <span>{profileNavigation.plots.length} of {profileNavigation.result.totalPlots} stored plots in reviewed profile navigation.</span>
    {#if view === 'fs882'}
      <span>Record {profileIndex + 1} of {profileNavigation.plots.length}</span>
      <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileIndex <= 0}
        onclick={() => void navigateProfilePlot(-1)}>Previous profile plot</button>
      <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileIndex < 0 || profileIndex >= profileNavigation.plots.length - 1}
        onclick={() => void navigateProfilePlot(1)}>Next profile plot</button>
    {/if}
    <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
      onclick={() => void clearProfileNavigation()}>Clear profile navigation</button>
  </section>
{/if}
