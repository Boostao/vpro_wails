import { ContextService, type FS882Header, type VegRecord, type HumusRecord, type MineralRecord,
  type OtherRecord, type HeightRecordUpdate, type AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';

export function bindContextPlots(contextId: string, port = ContextService) {
  if (!contextId) throw new Error('An editor requires a loaded project context identity.');
  return {
    GetPlot: (plot: string) => port.GetPlot(contextId, plot),
    GetHeaderCapabilities: () => port.GetHeaderCapabilities(contextId),
    GetChildCapabilities: (kind: string) => port.GetChildCapabilities(contextId, kind),
    ListVegRecords: (plot: string) => port.ListVegRecords(contextId, plot),
    ListHumusRecords: (plot: string) => port.ListHumusRecords(contextId, plot),
    ListMineralRecords: (plot: string) => port.ListMineralRecords(contextId, plot),
    ListOtherRecords: (plot: string) => port.ListOtherRecords(contextId, plot),
    ListAuditEntries: (plot: string) => port.ListAuditEntries(contextId, plot),
    CreatePlot: (header: FS882Header) => port.CreatePlot(contextId, header),
    UpdatePlot: (header: FS882Header) => port.UpdatePlot(contextId, header),
    SaveVegRecord: (record: VegRecord) => port.SaveVegRecord(contextId, record),
    UpdateVegRecord: (record: VegRecord) => port.UpdateVegRecord(contextId, record),
    DeleteVegRecord: (plot: string, id: number) => port.DeleteVegRecord(contextId, plot, id),
    SaveHumusRecord: (record: HumusRecord) => port.SaveHumusRecord(contextId, record),
    UpdateHumusRecord: (record: HumusRecord) => port.UpdateHumusRecord(contextId, record),
    DeleteHumusRecord: (plot: string, id: number) => port.DeleteHumusRecord(contextId, plot, id),
    SaveMineralRecord: (record: MineralRecord) => port.SaveMineralRecord(contextId, record),
    UpdateMineralRecord: (record: MineralRecord) => port.UpdateMineralRecord(contextId, record),
    DeleteMineralRecord: (plot: string, id: number) => port.DeleteMineralRecord(contextId, plot, id),
    SaveOtherRecord: (record: OtherRecord) => port.SaveOtherRecord(contextId, record),
    UpdateOtherRecord: (record: OtherRecord) => port.UpdateOtherRecord(contextId, record),
    DeleteOtherRecord: (plot: string, id: number) => port.DeleteOtherRecord(contextId, plot, id),
    UpdateHeightRecords: (plot: string, updates: HeightRecordUpdate[] | null) => port.UpdateHeightRecords(contextId, plot, updates),
    SetAuditRestoreSelection: (plot: string, rowIds: string[] | null) => port.SetAuditRestoreSelection(contextId, plot, rowIds),
    RestoreSelectedAuditRecords: (plot: string, rowIds: string[] | null, action: AuditRestoreAction) =>
      port.RestoreSelectedAuditRecords(contextId, plot, rowIds, action),
  };
}
