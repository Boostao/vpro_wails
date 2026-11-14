<script lang="ts">
  import { LifecycleService, type StartupRecoveryRequest, type StartupState } from '../../bindings/github.com/boostao/vpro-wails';

  let {
    startupFailure,
    busy,
    onrecovered,
  }: {
    startupFailure: StartupState;
    busy: boolean;
    onrecovered: (backupNotice: string) => Promise<void> | void;
  } = $props();

  let recoveryWorking = $state(false);
  let recoveryError = $state('');
  let recoveryProjectPath = $state('');
  let recoveryProjectName = $state('');
  let recoverySUPath = $state('');
  let recoveryClearSU = $state(false);
  let recoveryHierarchyPath = $state('');
  let recoveryClearHierarchy = $state(false);

  $effect(() => {
    recoveryProjectPath = startupFailure.projectPath ?? '';
    recoveryProjectName = startupFailure.configuredProject ?? '';
    recoverySUPath = startupFailure.suPath ?? '';
    recoveryClearSU = (startupFailure.configuredSu ?? 'None') !== 'None';
    recoveryHierarchyPath = startupFailure.hierarchyPath ?? '';
    recoveryClearHierarchy = (startupFailure.configuredHierarchy ?? 'None') !== 'None';
  });

  async function fallbackStartupToSample() {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const recovered = await LifecycleService.ResetStartupToSample();
      const notice = recovered.backupPath
        ? `Recovered with built-in Sample project. Previous configuration was backed up to ${recovered.backupPath}.`
        : 'Recovered with built-in Sample project.';
      await onrecovered(notice);
    } catch (cause) {
      recoveryError = `Could not fall back to Sample project: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }

  async function browseStartupDatabase(target: 'project' | 'su' | 'hierarchy') {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const picked = await LifecycleService.ChooseStartupDatabaseFile();
      if (!picked) return;
      if (target === 'project') {
        recoveryProjectPath = picked;
      } else if (target === 'su') {
        recoverySUPath = picked;
        recoveryClearSU = false;
      } else {
        recoveryHierarchyPath = picked;
        recoveryClearHierarchy = false;
      }
    } catch (cause) {
      recoveryError = `File picker unavailable: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }

  async function recoverStartupPaths() {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const payload: StartupRecoveryRequest = $state.snapshot({
        projectPath: recoveryProjectPath.trim(),
        projectName: recoveryProjectName.trim(),
        suPath: recoverySUPath.trim(),
        clearSu: recoveryClearSU,
        hierarchyPath: recoveryHierarchyPath.trim(),
        clearHierarchy: recoveryClearHierarchy,
      });
      const recovered = await LifecycleService.RecoverStartupWithPaths(payload);
      const notice = recovered.backupPath
        ? `Relocated project opened cleanly. Previous configuration was backed up to ${recovered.backupPath}.`
        : 'Relocated project opened cleanly.';
      await onrecovered(notice);
    } catch (cause) {
      recoveryError = `Could not open relocated database: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }
</script>

<section class="startup-recovery space-y-4 p-5 border rounded bg-white max-w-3xl" role="alert" data-startup-recovery>
  <div>
    <div class="eyebrow">Startup Recovery</div>
    <h1 class="text-xl font-semibold">Saved context could not be opened</h1>
  </div>
  <p class="text-sm">{startupFailure.message}</p>
  <p class="text-xs text-slate-600">
    Saved preferences in <code>{startupFailure.configPath}</code> and existing databases have not been modified yet.
    Choose an option below — VPro will automatically back up your current <code>config.yml</code> to
    <code>config.backup-&lt;timestamp&gt;.yml</code> before updating settings.
  </p>
  {#if recoveryError}
    <p role="alert" class="error text-xs" data-startup-recovery-error>{recoveryError}</p>
  {/if}

  <div class="p-3.5 border rounded bg-slate-50 space-y-2">
    <h2 class="text-sm font-semibold">Option 1: Open built-in Sample project</h2>
    <p class="text-xs text-slate-700">
      Backs up your prior <code>config.yml</code> and opens VPro immediately with the built-in <strong>Sample</strong> project.
      None of your existing <code>.db</code> project files are deleted or altered.
    </p>
    <button
      type="button"
      data-startup-fallback-sample
      disabled={recoveryWorking || busy}
      class="px-3.5 py-1.5 rounded bg-[#003366] text-white text-xs font-semibold hover:bg-[#002244] disabled:opacity-50"
      onclick={() => void fallbackStartupToSample()}
    >
      {recoveryWorking ? 'Recovering...' : 'Fall back to Sample project (backup config.yml)'}
    </button>
  </div>

  <form
    class="p-3.5 border rounded space-y-3"
    onsubmit={(event) => { event.preventDefault(); void recoverStartupPaths(); }}
  >
    <h2 class="text-sm font-semibold">Option 2: Relocate missing database path</h2>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
      <label for="recovery-project-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
        Project SQLite database (.db) path
        <input
          id="recovery-project-path"
          class="px-2.5 py-1.5 border rounded text-xs font-mono"
          bind:value={recoveryProjectPath}
          placeholder="/path/to/Project.db"
          disabled={recoveryWorking || busy}
        />
      </label>
      <button
        type="button"
        data-startup-browse-project
        disabled={recoveryWorking || busy}
        class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
        onclick={() => void browseStartupDatabase('project')}
      >
        Browse .db...
      </button>
    </div>
    <label for="recovery-project-name" class="flex flex-col gap-1 text-xs font-medium max-w-xs">
      Project name inside file (optional when single-project .db)
      <input
        id="recovery-project-name"
        class="px-2.5 py-1.5 border rounded text-xs"
        bind:value={recoveryProjectName}
        placeholder={startupFailure.configuredProject || 'Auto-detect'}
        disabled={recoveryWorking || busy}
      />
    </label>

    <div class="flex flex-wrap gap-4 text-xs">
      <label class="inline-flex items-center gap-1.5">
        <input type="checkbox" bind:checked={recoveryClearSU} disabled={recoveryWorking || busy} />
        <span>Reset missing Site Unit (SU) selection to None</span>
      </label>
      <label class="inline-flex items-center gap-1.5">
        <input type="checkbox" bind:checked={recoveryClearHierarchy} disabled={recoveryWorking || busy} />
        <span>Reset missing Hierarchy selection to None</span>
      </label>
    </div>

    {#if !recoveryClearSU && (startupFailure.configuredSu || 'None') !== 'None'}
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
        <label for="recovery-su-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
          Relocated SU database (.db) path ({startupFailure.configuredSu})
          <input
            id="recovery-su-path"
            class="px-2.5 py-1.5 border rounded text-xs font-mono"
            bind:value={recoverySUPath}
            disabled={recoveryWorking || busy}
          />
        </label>
        <button
          type="button"
          disabled={recoveryWorking || busy}
          class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
          onclick={() => void browseStartupDatabase('su')}
        >
          Browse SU .db...
        </button>
      </div>
    {/if}

    {#if !recoveryClearHierarchy && (startupFailure.configuredHierarchy || 'None') !== 'None'}
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
        <label for="recovery-hierarchy-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
          Relocated Hierarchy database (.db) path ({startupFailure.configuredHierarchy})
          <input
            id="recovery-hierarchy-path"
            class="px-2.5 py-1.5 border rounded text-xs font-mono"
            bind:value={recoveryHierarchyPath}
            disabled={recoveryWorking || busy}
          />
        </label>
        <button
          type="button"
          disabled={recoveryWorking || busy}
          class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
          onclick={() => void browseStartupDatabase('hierarchy')}
        >
          Browse Hierarchy .db...
        </button>
      </div>
    {/if}

    <div>
      <button
        type="submit"
        data-startup-relocate-open
        disabled={recoveryWorking || busy || !recoveryProjectPath.trim()}
        class="px-3.5 py-1.5 rounded border border-[#003366] text-[#003366] text-xs font-semibold hover:bg-slate-50 disabled:opacity-50"
      >
        {recoveryWorking ? 'Opening...' : 'Update paths & open project'}
      </button>
    </div>
  </form>
</section>
