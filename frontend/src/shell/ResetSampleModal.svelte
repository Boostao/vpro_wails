<script lang="ts">
  let {
    open,
    busy,
    editorBusy,
    companionPanelBusy,
    transitionWorking,
    contextAvailable,
    onconfirm,
    onclose,
  }: {
    open: boolean;
    busy: boolean;
    editorBusy: boolean;
    companionPanelBusy: boolean;
    transitionWorking: boolean;
    contextAvailable: boolean;
    onconfirm: () => void;
    onclose: () => void;
  } = $props();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-reset-sample-modal>
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="reset-sample-modal-title"
      class="w-full max-w-md overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
    >
      <header class="border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-4 py-3 text-white">
        <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">Safe Non-Destructive Recovery</div>
        <h2 id="reset-sample-modal-title" class="text-sm font-semibold text-white">Reset Workspace to Sample</h2>
      </header>
      <div class="space-y-3 p-4 text-xs leading-relaxed">
        <p>
          Materialize a fresh, pristine copy of embedded <code class="font-mono text-[#003366]">Sample.db</code> and switch the active session cleanly to
          <strong>Project = Sample</strong>, <strong>SU = Sample</strong>, and <strong>Hierarchy = Sample</strong>?
        </p>
        <p class="rounded border border-slate-200 bg-[#F2F2F2] p-2.5 text-slate-700">
          Existing user project <code class="font-mono">.db</code> files in your projects directory are <strong>never deleted or modified</strong>.
        </p>
        <div class="flex justify-end gap-2 pt-2">
          <button
            type="button"
            disabled={busy || transitionWorking}
            class="rounded border border-slate-300 bg-white px-3 py-1.5 text-xs font-medium text-[#313132] hover:bg-slate-50"
            onclick={onclose}
          >
            Cancel
          </button>
          <button
            type="button"
            data-confirm-reset-sample
            disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !contextAvailable}
            class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={onconfirm}
          >
            {busy ? 'Resetting...' : 'Reset Workspace to Sample'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
