<script lang="ts">
  import { FolderOpen } from '@lucide/svelte';
  import {
    ContextService,
    ImportExportService,
    type AccessDatabaseInspection,
    type AccessProjectCandidate,
    type AccessProjectImportResult,
    type ContextSelection,
    type ProjectInfo,
  } from '../../bindings/github.com/boostao/vpro-wails';
  import { projectState } from '../shared/state';

  let {
    busy,
    editorBusy,
    companionPanelBusy,
    transitionWorking,
    requestTransition,
    switchContext,
    onAccessActivated,
  }: {
    busy: boolean;
    editorBusy: boolean;
    companionPanelBusy: boolean;
    transitionWorking: boolean;
    requestTransition: (run: (contextId: string) => Promise<void>) => Promise<void>;
    switchContext: (contextId: string, selection: ContextSelection, next: 'plots' | 'hierarchy') => Promise<void>;
    onAccessActivated: () => Promise<void>;
  } = $props();

  const externalProjectsEnabled = true;

  let externalProject = $state('');
  let externalPath = $state('');
  let externalDiscoveredProjects = $state<ProjectInfo[]>([]);
  let externalPickMessage = $state('');
  let externalPickError = $state('');
  let revealMessage = $state('');
  let revealError = $state('');
  let accessSourcePath = $state('');
  let accessInspection = $state<AccessDatabaseInspection | null>(null);
  let accessSelectedSourceProject = $state('');
  let accessTargetProject = $state('');
  let accessDestinationPath = $state('');
  let accessActivateOnSuccess = $state(true);
  let accessWorking = $state(false);
  let accessError = $state('');
  let accessResult = $state<AccessProjectImportResult | null>(null);

  const accessSelectedCandidate = $derived(
    accessInspection?.projects?.find(p => p.sourceProject === accessSelectedSourceProject) ?? null
  );
  const activeHierarchyIndex = $derived(
    $projectState?.hierarchies?.findIndex(
      hierarchy => hierarchy.name === $projectState?.activeHierarchy && hierarchy.path === $projectState?.hierarchyPath
    ) ?? -1
  );
  const activeSUIndex = $derived(
    $projectState?.sus?.findIndex(
      su => su.name === $projectState?.activeSU && su.path === $projectState?.suPath
    ) ?? -1
  );

  function selectedContext(): ContextSelection {
    const state = $projectState;
    if (!state?.projectPath) throw new Error('The active project file identity is unavailable.');
    return {
      project: state.activeProject,
      projectPath: state.projectPath,
      su: state.activeSU,
      suPath: state.suPath ?? '',
      hierarchy: state.activeHierarchy,
      hierarchyPath: state.hierarchyPath ?? '',
    };
  }

  async function chooseProject(project: ProjectInfo) {
    await requestTransition(async contextId => {
      if (!project.path) throw new Error('Choose a project with an explicit file identity.');
      const selection = selectedContext();
      selection.project = project.name;
      selection.projectPath = project.path;
      selection.su = 'None';
      selection.suPath = '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function chooseSU(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    const chosen = $projectState?.sus?.[Number(select.value)];
    select.value = String(activeSUIndex);
    await requestTransition(async contextId => {
      const selection = selectedContext();
      selection.su = chosen?.name ?? 'None';
      selection.suPath = chosen?.path ?? '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function chooseHierarchy(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    const chosen = $projectState?.hierarchies?.[Number(select.value)];
    select.value = String(activeHierarchyIndex);
    await requestTransition(async contextId => {
      const selection = selectedContext();
      selection.hierarchy = chosen?.name ?? 'None';
      selection.hierarchyPath = chosen?.path ?? '';
      await switchContext(contextId, selection, 'hierarchy');
    });
  }

  async function attachExternalProject(event: SubmitEvent) {
    event.preventDefault();
    const name = externalProject;
    const filePath = externalPath;
    await requestTransition(async contextId => {
      const selection = selectedContext();
      selection.project = name;
      selection.projectPath = filePath;
      selection.su = 'None';
      selection.suPath = '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function browseExternalSQLiteProject() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    externalPickError = '';
    externalPickMessage = '';
    try {
      const inspection = await ContextService.ChooseExternalSQLiteFile(contextId);
      if (!inspection.selectedPath) return;
      externalPath = inspection.selectedPath;
      externalDiscoveredProjects = inspection.projects ?? [];
      const firstCompatible = externalDiscoveredProjects.find(p => p.compatible) ?? externalDiscoveredProjects[0];
      if (firstCompatible) {
        externalProject = firstCompatible.name;
        externalPickMessage = `Found ${externalDiscoveredProjects.length} project(s) in ${firstCompatible.file}.`;
      } else {
        externalPickMessage = 'Selected SQLite file contains no *_Env project table.';
      }
    } catch (cause) {
      externalPickError = `Could not inspect SQLite file: ${String(cause)}`;
    }
  }

  async function revealInExplorer(target: 'root' | 'projects' | 'project' | 'su' | 'hierarchy') {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    revealError = '';
    revealMessage = '';
    try {
      const opened = await ContextService.RevealInFileExplorer(contextId, target);
      revealMessage = `Opened in File Explorer: ${opened}`;
    } catch (cause) {
      revealError = `Could not open File Explorer: ${String(cause)}`;
    }
  }

  function applyAccessCandidate(candidate: AccessProjectCandidate | null) {
    if (!candidate) return;
    accessSelectedSourceProject = candidate.sourceProject;
    accessTargetProject = candidate.suggestedTarget;
    accessDestinationPath = candidate.suggestedDestPath;
  }

  async function browseAccessDatabase() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    accessError = '';
    accessResult = null;
    try {
      const chosen = await ImportExportService.ChooseAccessDatabaseFile(contextId);
      if (!chosen) return;
      accessSourcePath = chosen;
      await inspectAccessSource(chosen);
    } catch (cause) {
      accessError = `Could not select Access file: ${String(cause)}`;
    }
  }

  async function inspectAccessSource(pathToInspect = accessSourcePath) {
    const contextId = $projectState?.contextId;
    if (!contextId || !pathToInspect) return;
    accessWorking = true;
    accessError = '';
    accessResult = null;
    try {
      const inspection = await ImportExportService.InspectAccessDatabase(contextId, pathToInspect);
      accessInspection = inspection;
      accessSourcePath = inspection.sourcePath;
      const projects = inspection.projects ?? [];
      const first = projects.find(p => p.compatible) ?? projects[0] ?? null;
      applyAccessCandidate(first);
      if (!first) {
        accessError = 'No *_Env project tables were found in the selected Access database.';
      }
    } catch (cause) {
      accessInspection = null;
      accessError = `Access inspection failed: ${String(cause)}`;
    } finally {
      accessWorking = false;
    }
  }

  async function browseAccessDestinationPath() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    accessError = '';
    try {
      const chosen = await ContextService.ChooseNewSQLiteProjectFile(contextId, accessTargetProject);
      if (chosen) accessDestinationPath = chosen;
    } catch (cause) {
      accessError = `Could not choose destination .db file: ${String(cause)}`;
    }
  }

  async function runAccessProjectImport(event: SubmitEvent) {
    event.preventDefault();
    if (!accessInspection || !accessSelectedSourceProject || !accessTargetProject || !accessDestinationPath) return;
    const sourcePath = accessInspection.sourcePath;
    const sourceProject = accessSelectedSourceProject;
    const targetProject = accessTargetProject;
    const destinationPath = accessDestinationPath;
    const activateOnSuccess = accessActivateOnSuccess;
    await requestTransition(async contextId => {
      accessWorking = true;
      accessError = '';
      accessResult = null;
      try {
        const result = await ImportExportService.ImportAccessProject(contextId, {
          sourcePath,
          sourceProject,
          targetProject,
          destinationPath,
          activateOnSuccess,
        });
        accessResult = result;
        if (result.state) {
          projectState.set(result.state);
        }
        if (result.activated) {
          await onAccessActivated();
        }
      } catch (cause) {
        accessError = `Import failed: ${String(cause)}`;
        throw cause;
      } finally {
        accessWorking = false;
      }
    });
  }
</script>

<aside id="project-context" class="sidebar" aria-label="Projects">
  <div class="sidebar-heading">
    <span>PROJECTS</span>
    <button type="button" class="icon-button" title="Reveal VPRO database folder in File Explorer" aria-label="Reveal in File Explorer" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('projects')}>
      <FolderOpen size={16} strokeWidth={1.8} />
    </button>
  </div>
  {#if revealMessage}<p role="status" class="px-3 py-1 text-xs text-emerald-800 break-words">{revealMessage}</p>{/if}
  {#if revealError}<p role="alert" class="px-3 py-1 text-xs text-red-800 break-words">{revealError}</p>{/if}
  <nav aria-label="Project list">
    {#each $projectState?.projects ?? [] as project ((project.path ?? project.file) + project.name)}
      <button class:active={project.name === $projectState?.activeProject && project.path === $projectState?.projectPath} disabled={!project.compatible || busy || editorBusy || companionPanelBusy || transitionWorking} title={project.compatible ? project.path : `${project.version} is not a complete VP08 project`} onclick={() => chooseProject(project)}>
        <span class="project-name">{project.name}
          <span class="project-location">{$projectState?.projects?.filter(item => item.name === project.name).length === 1 ? project.file : project.path}</span>
        </span>
        <span class="project-version">{project.version}</span>
      </button>
    {/each}
  </nav>
  <div class="su-picker">
    <label for="active-su">SITE UNIT</label>
    <select id="active-su" value={String(activeSUIndex)} onchange={chooseSU} disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState}>
      <option value="-1">None</option>
      {#each $projectState?.sus ?? [] as su, index ((su.path ?? '') + su.name)}
        <option value={String(index)} disabled={!su.compatible} title={su.path}>{su.name}{su.kind === 'master' ? ' (master)' : ''}</option>
      {/each}
    </select>
  </div>
  <div class="su-picker">
    <label for="active-hierarchy">HIERARCHY</label>
    <select id="active-hierarchy" value={String(activeHierarchyIndex)} onchange={chooseHierarchy} disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState}>
      <option value="-1">None</option>
      {#each $projectState?.hierarchies ?? [] as hierarchy, index ((hierarchy.path ?? hierarchy.file) + hierarchy.name)}
        <option value={String(index)} disabled={!hierarchy.compatible} title={hierarchy.path}>{hierarchy.name} ({hierarchy.file})</option>
      {/each}
    </select>
  </div>
  {#if externalProjectsEnabled}
    <details class="su-picker">
      <summary>Attach external SQLite project</summary>
      <form onsubmit={attachExternalProject}>
        <button type="button" id="browse-external-sqlite" disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState?.contextId} onclick={() => void browseExternalSQLiteProject()}>Browse SQLite .db file…</button>
        {#if externalPickMessage}<p role="status" class="text-emerald-800 break-words">{externalPickMessage}</p>{/if}
        {#if externalPickError}<p role="alert" class="text-red-800 break-words">{externalPickError}</p>{/if}
        {#if externalDiscoveredProjects.length > 1}
          <label for="external-project-select">DISCOVERED PROJECTS IN FILE</label>
          <select id="external-project-select" value={externalProject} onchange={(e) => { externalProject = (e.currentTarget as HTMLSelectElement).value; }} disabled={busy || editorBusy || companionPanelBusy || transitionWorking}>
            {#each externalDiscoveredProjects as p (p.name)}
              <option value={p.name} disabled={!p.compatible}>{p.name} ({p.version})</option>
            {/each}
          </select>
        {/if}
        <label for="external-project">PROJECT PREFIX</label>
        <input id="external-project" bind:value={externalProject} required disabled={busy || editorBusy || companionPanelBusy || transitionWorking} />
        <label for="external-path">FULL SQLITE FILE PATH</label>
        <input id="external-path" bind:value={externalPath} required disabled={busy || editorBusy || companionPanelBusy || transitionWorking} />
        <p>Attaching opens the SQLite database in place without copying it; edits write directly to the attached file.</p>
        <button type="submit" disabled={busy || editorBusy || companionPanelBusy || transitionWorking}>Attach project</button>
      </form>
    </details>
    <details class="su-picker" id="access-import-wizard">
      <summary>Import Access (.mdb / .accdb) wizard</summary>
      <form onsubmit={runAccessProjectImport}>
        <button type="button" id="browse-access-db" disabled={busy || transitionWorking || accessWorking || !$projectState?.contextId} onclick={() => void browseAccessDatabase()}>Browse Access (.mdb / .accdb)…</button>
        <label for="access-source-path">ACCESS SOURCE FILE (.mdb / .accdb)</label>
        <input id="access-source-path" bind:value={accessSourcePath} placeholder="C:\path\to\project.accdb" disabled={busy || transitionWorking || accessWorking} />
        <button type="button" id="inspect-access-db" disabled={busy || transitionWorking || accessWorking || !accessSourcePath || !$projectState?.contextId} onclick={() => void inspectAccessSource()}>Inspect Access file</button>
        {#if accessInspection && (accessInspection.projects?.length ?? 0) > 0}
          <label for="access-source-project">SOURCE PROJECT IN ACCESS DB</label>
          <select id="access-source-project" value={accessSelectedSourceProject} onchange={(e) => {
            const chosen = (accessInspection?.projects ?? []).find(p => p.sourceProject === (e.currentTarget as HTMLSelectElement).value) ?? null;
            applyAccessCandidate(chosen);
          }} disabled={busy || transitionWorking || accessWorking}>
            {#each accessInspection.projects ?? [] as cand (cand.sourceProject)}
              <option value={cand.sourceProject}>{cand.sourceProject} ({cand.sourceVersion} · {cand.plotCount} plots)</option>
            {/each}
          </select>
          {#if accessSelectedCandidate}
            <p class="text-xs break-words"><strong>Autoconversion:</strong> {accessSelectedCandidate.conversionMode}</p>
            <p class="text-xs break-words">{accessSelectedCandidate.statusMessage}</p>
            {#if (accessSelectedCandidate.optionalSUs?.length ?? 0) > 0}
              <p class="text-xs break-words">SU tables included: {(accessSelectedCandidate.optionalSUs ?? []).join(', ')}</p>
            {/if}
          {/if}
          <label for="access-target-project">NEW VPRO PROJECT PREFIX</label>
          <input id="access-target-project" bind:value={accessTargetProject} required disabled={busy || transitionWorking || accessWorking} />
          <label for="access-dest-path">NEW SQLITE DESTINATION (.db)</label>
          <input id="access-dest-path" bind:value={accessDestinationPath} required disabled={busy || transitionWorking || accessWorking} />
          <button type="button" id="browse-access-dest" disabled={busy || transitionWorking || accessWorking || !$projectState?.contextId} onclick={() => void browseAccessDestinationPath()}>Browse destination .db…</button>
          <label class="flex items-center gap-2 text-xs">
            <input type="checkbox" style="width:auto;min-height:auto;" bind:checked={accessActivateOnSuccess} disabled={busy || transitionWorking || accessWorking} />
            Switch to imported project on completion
          </label>
          <button type="submit" id="convert-import-access-btn" disabled={busy || transitionWorking || accessWorking || !accessSelectedCandidate?.compatible || !accessTargetProject || !accessDestinationPath}>
            {accessWorking ? 'Converting & importing…' : 'Convert & import to VPRO DB'}
          </button>
        {/if}
        {#if accessError}<p role="alert" class="text-red-800 break-words">{accessError}</p>{/if}
        {#if accessResult}
          <p role="status" class="text-emerald-800 break-words">
            Imported {accessResult.targetProject} ({accessResult.plotCount} plots, {accessResult.conversionMode}) into {accessResult.destinationPath}.
          </p>
        {/if}
      </form>
    </details>
  {/if}
  <div class="su-picker">
    <label for="reveal-explorer-btn">DATA ON DISK</label>
    <div class="grid gap-1.5">
      <button type="button" id="reveal-explorer-btn" class="px-2 py-1.5 border rounded bg-white text-xs text-left" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('project')}>
        Reveal active project .db in File Explorer
      </button>
      <button type="button" id="reveal-root-explorer-btn" class="px-2 py-1.5 border rounded bg-white text-xs text-left" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('root')}>
        Open VPRO DBs root folder in File Explorer
      </button>
    </div>
  </div>
  <div class="sidebar-footer">{$projectState?.projects?.length ?? 0} project{($projectState?.projects?.length ?? 0) === 1 ? '' : 's'}</div>
</aside>
