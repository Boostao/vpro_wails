import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { metadataCellText } from './projectMetadataEditor';

export interface PictureOwner {
  contextId: string;
  project: string;
  plotNumber: string;
}
export interface PictureReview extends PictureOwner {
  records: ReturnType<typeof siviMetadataTableFromWire>;
}
export interface PicturePreview extends PictureOwner {
  rowId: string;
  mime: string;
  width: number;
  height: number;
  sha256: string;
  dataUrl: string;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export function pictureMetadataFromWire(wire: unknown, owner: PictureOwner): PictureReview {
  if (!record(wire) || wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plotNumber !== owner.plotNumber) {
    throw new Error('Picture metadata belongs to another context, project or literal plot.');
  }
  const records = siviMetadataTableFromWire(wire.records);
  const names = ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'];
  if (records.columns.length !== names.length || records.columns.some((column, index) => column.name !== names[index]) ||
      records.rows.some(row => row.cells[3].storage !== 'text' || row.cells[3].text !== owner.plotNumber)) {
    throw new Error('Complete original picture columns and literal physical plot membership were not returned.');
  }
  return { ...owner, records };
}

export function pictureImageFromWire(wire: unknown, owner: PictureOwner, rowId: string): PicturePreview {
  if (!record(wire) || wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plotNumber !== owner.plotNumber ||
      wire.rowId !== rowId || (wire.mime !== 'image/jpeg' && wire.mime !== 'image/png') ||
      typeof wire.width !== 'number' || !Number.isInteger(wire.width) || wire.width <= 0 ||
      typeof wire.height !== 'number' || !Number.isInteger(wire.height) || wire.height <= 0 ||
      wire.width > 16 * 1024 * 1024 / wire.height || typeof wire.sha256 !== 'string' || !/^[0-9a-f]{64}$/.test(wire.sha256) ||
      typeof wire.dataUrl !== 'string') {
    throw new Error('Complete measured picture bytes and owned physical row identity were not returned.');
  }
  const prefix = `data:${wire.mime};base64,`;
  const encoded = wire.dataUrl.slice(prefix.length);
  const bytes = encoded.length / 4 * 3 - (encoded.endsWith('==') ? 2 : encoded.endsWith('=') ? 1 : 0);
  if (!wire.dataUrl.startsWith(prefix) || !encoded || bytes > 32 * 1024 * 1024 ||
      !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(encoded)) {
    throw new Error('Picture preview requires bounded embedded image bytes, not an external URL or repaired payload.');
  }
  return {
    ...owner, rowId, mime: wire.mime, width: wire.width, height: wire.height,
    sha256: wire.sha256, dataUrl: wire.dataUrl,
  };
}

export function pictureCellLabel(cell: ProjectMetadataCell): string {
  return cell.storage === 'null' ? '(NULL)' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell);
}
