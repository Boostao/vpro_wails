<script lang="ts">
  import {
    AuditRestoreAction,
    type OrdinaryAuditReadEntry,
  } from '../../bindings/github.com/boostao/vpro-wails';
  import AuditRestore, { auditRestoreReason } from './AuditRestore.svelte';
  import type { FS882ChildCapabilities } from './fs882FormHelpers';

  let {
    entries,
    project,
    plot,
    expectedPlotNumber,
    capabilities,
    childCapabilities,
    enabled,
    disabled,
    gateReason,
    revision,
    getLoadRequest,
    PlotService,
    onBusyChange,
    onFeedback,
    onRefreshAfterRestore,
    onRefreshFailure,
  }: {
    entries: OrdinaryAuditReadEntry[];
    project: string;
    plot: string;
    expectedPlotNumber: string | undefined;
    capabilities: Record<string, boolean | undefined>;
    childCapabilities: FS882ChildCapabilities;
    enabled: boolean;
    disabled: boolean;
    gateReason: string;
    revision: number;
    getLoadRequest: () => number;
    PlotService: {
      SetAuditRestoreSelection: (plot: string, rowIds: string[] | null) => Promise<unknown>;
      RestoreSelectedAuditRecords: (
        plot: string,
        rowIds: string[] | null,
        action: AuditRestoreAction,
      ) => Promise<{ cancelled?: boolean; restoredRows?: number; prunedAuditRows?: number; cleanedVegRows?: number } | null>;
    };
    onBusyChange: (busy: boolean) => void;
    onFeedback: (error: string | null, successMsg: string | null) => void;
    onRefreshAfterRestore: (plot: string, request: number) => Promise<void>;
    onRefreshFailure: (message: string) => void;
  } = $props();

  async function restoreAudit(rowIds: string[], action: AuditRestoreAction) {
    if (!enabled || disabled || !plot || plot !== expectedPlotNumber) {
      onFeedback(gateReason, null);
      return;
    }
    if (action !== AuditRestoreAction.AuditRestoreCancel) {
      if (rowIds.length === 0 || new Set(rowIds).size !== rowIds.length) {
        onFeedback('Select distinct audit rows before restoring.', null);
        return;
      }
      for (const rowId of rowIds) {
        const entry = entries.find(row => row.rowId === rowId);
        const reason = entry
          ? auditRestoreReason(entry, project, plot, { header: capabilities, children: childCapabilities })
          : 'Audit row disappeared; refresh before restoring.';
        if (reason) {
          onFeedback(reason, null);
          return;
        }
      }
    }
    const request = getLoadRequest();
    onBusyChange(true);
    onFeedback(null, null);
    let currentError: string | null = null;
    let marksWritten = false;
    try {
      if (action !== AuditRestoreAction.AuditRestoreCancel) {
        await PlotService.SetAuditRestoreSelection(plot, rowIds);
        marksWritten = true;
      }
      const result = await PlotService.RestoreSelectedAuditRecords(plot, rowIds, action);
      if (!result) throw new Error('Restoration result was not returned.');
      if (result.cleanedVegRows !== 0) throw new Error('Unexpected vegetation deletion reported; stop using this application binary.');
      if (request === getLoadRequest()) {
        onFeedback(
          null,
          result.cancelled
            ? 'Restoration cancelled. Data and history were unchanged.'
            : `Restored ${result.restoredRows} fields; ${result.prunedAuditRows} selected audit rows pruned. No vegetation records deleted.`,
        );
      }
    } catch (cause) {
      if (request === getLoadRequest()) {
        currentError = `Restoration failed: ${String(cause)}`;
        onFeedback(currentError, null);
      }
    } finally {
      if (marksWritten) {
        try { await PlotService.SetAuditRestoreSelection(plot, []); }
        catch (cause) {
          if (request === getLoadRequest()) {
            currentError = `${currentError ?? ''} Selection marks could not be cleared: ${String(cause)}`;
            onFeedback(currentError, null);
          }
        }
      }
      try { await onRefreshAfterRestore(plot, request); }
      catch (cause) {
        if (request === getLoadRequest()) {
          onRefreshFailure(`${currentError ?? ''} Restoration refresh failed: ${String(cause)}`);
        }
      }
      if (request === getLoadRequest()) onBusyChange(false);
    }
  }
</script>

<AuditRestore
  {entries}
  {project}
  {plot}
  capabilities={{ header: capabilities, children: childCapabilities }}
  {enabled}
  {disabled}
  {gateReason}
  {revision}
  onrestore={restoreAudit}
/>
