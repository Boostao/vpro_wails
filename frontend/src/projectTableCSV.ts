import type { ProjectTableCSVReview, TableCSVManifest } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';

export const coreTableSuffixes = ['Admin', 'Audit', 'Env', 'Humus', 'Metadata', 'Mineral', 'Other', 'Veg'] as const;

export type ValidatedProjectTableCSVReview = Omit<ProjectTableCSVReview, 'manifest'> & {
  manifest: Omit<TableCSVManifest, 'columns' | 'rowIds' | 'storage' | 'descriptions'> & {
    columns: NonNullable<TableCSVManifest['columns']>;
    rowIds: string[];
    storage: string[][];
    descriptions: NonNullable<TableCSVManifest['descriptions']>;
  };
};

function csvRecords(data: string): string[][] {
  const records: string[][] = [];
  let row: string[] = [], field = '', quoted = false, closed = false, started = false;
  for (let index = 0; index < data.length; index++) {
    const character = data[index];
    if (quoted) {
      if (character === '"' && data[index + 1] === '"') { field += '"'; index++; }
      else if (character === '"') { quoted = false; closed = true; }
      else field += character;
    } else if (character === '"') {
      if (started || closed) throw new Error('CSV has an unexpected field quote.');
      quoted = true; started = true;
    } else if (character === ',' || character === '\n') {
      if (character === '\n' && !started && !closed && row.length === 0) throw new Error('CSV has a bare blank record.');
      row.push(field); field = ''; started = false; closed = false;
      if (character === '\n') { records.push(row); row = []; }
    } else {
      if (closed) throw new Error('CSV contains data after a closing quote.');
      field += character; started = true;
    }
  }
  if (quoted || closed || started || row.length) throw new Error('CSV has an incomplete trailing record.');
  return records;
}

export async function tableCSVSHA256(data: string): Promise<string> {
  const bytes = new TextEncoder().encode(data);
  const hash = await crypto.subtle.digest('SHA-256', bytes);
  return [...new Uint8Array(hash)].map(value => value.toString(16).padStart(2, '0')).join('');
}

export async function validateProjectTableCSVReview(value: ProjectTableCSVReview | null, contextId: string,
  project: string, projectPath: string, table: string,
  checksum: (data: string) => Promise<string> = tableCSVSHA256): Promise<ValidatedProjectTableCSVReview> {
  value = value ? structuredClone(value) : null;
  const manifest = value?.manifest;
  if (!value || !contextId || value.contextId !== contextId || value.project !== project ||
      value.projectPath !== projectPath || typeof value.descriptionMetadataPresent !== 'boolean' ||
      !coreTableSuffixes.some(suffix => table === `${project}_${suffix}`) ||
      !manifest || manifest.version !== 1 || manifest.table !== table ||
      !Array.isArray(manifest.columns) || manifest.columns.length === 0 ||
      !Array.isArray(manifest.rowIds) || !Array.isArray(manifest.storage) ||
      manifest.storage.length !== manifest.rowIds.length || !Array.isArray(manifest.descriptions) ||
      !value.descriptionMetadataPresent && manifest.descriptions.length > 0 ||
      typeof manifest.sha256 !== 'string' || !/^[0-9a-f]{64}$/.test(manifest.sha256) ||
      typeof value.csv !== 'string' || !wellFormedUTF16(value.csv) || value.csv.includes('\r')) {
    throw new Error('Table CSV review has a foreign or incomplete owned context/manifest.');
  }
  const { columns, rowIds, descriptions: candidates } = manifest;
  const storage: string[][] = [];
  const names = new Set<string>(), identities = new Set<string>(), descriptions = new Set<string>();
  for (const column of manifest.columns) {
    if (typeof column.name !== 'string' || !column.name || !wellFormedUTF16(column.name) ||
        /[\0\r]/.test(column.name) || names.has(column.name) ||
        typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) {
      throw new Error('Table CSV schema is incomplete or ambiguous.');
    }
    names.add(column.name);
  }
  for (const candidate of manifest.descriptions) {
    if (!exactSigned64(candidate.rowId) || descriptions.has(candidate.rowId) || !completeCell(candidate.value)) {
      throw new Error('Table CSV Description candidates lost physical identity or tagged values.');
    }
    descriptions.add(candidate.rowId);
  }
  const records = csvRecords(value.csv);
  if (records.length !== manifest.rowIds.length + 1 || records[0]?.length !== manifest.columns.length ||
      records[0].some((name, index) => name !== columns[index].name)) {
    throw new Error('Table CSV records/header differ from the complete physical manifest.');
  }
  for (let row = 0; row < manifest.rowIds.length; row++) {
    const identity = manifest.rowIds[row], tags = manifest.storage[row], record = records[row + 1];
    if (!exactSigned64(identity) || identities.has(identity) || !Array.isArray(tags) ||
        tags.length !== names.size || record.length !== names.size) throw new Error('Table CSV row identity/storage is incomplete.');
    identities.add(identity);
    storage.push(tags);
    for (let column = 0; column < record.length; column++) {
      const raw = record[column], tag = tags[column];
      if (tag === 'null') {
        if (raw !== '') throw new Error('Table CSV NULL became a value.');
      } else if (tag === 'text') {
        const text: unknown = JSON.parse(raw);
        if (typeof text !== 'string' || !wellFormedUTF16(text)) throw new Error('Table CSV text is malformed; no repair was applied.');
      } else if (tag === 'integer') {
        if (!exactSigned64(raw)) throw new Error('Table CSV integer lost its exact signed64 representation.');
      } else if (tag === 'real') {
        if (!/^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:e[+-]?\d+)?$/.test(raw) || !Number.isFinite(Number(raw))) {
          throw new Error('Table CSV real is not a finite numeric value.');
        }
      } else if (tag === 'blob') {
        if (!/^(?:[0-9a-f]{2})*$/.test(raw)) throw new Error('Table CSV BLOB lost its exact hex bytes.');
      } else throw new Error('Table CSV has an unsupported storage tag.');
    }
  }
  if (await checksum(value.csv) !== manifest.sha256) throw new Error('Table CSV checksum differs; no partial review was accepted.');
  return { ...value, manifest: { ...manifest, columns, rowIds, storage, descriptions: candidates } };
}
