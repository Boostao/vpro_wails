import type {
  PMTilesManifest,
  SpatialWorkbenchPayload,
} from '../bindings/github.com/boostao/vpro-wails';

export type SpatialGeoArrowColumns = SpatialWorkbenchPayload['geoArrow'];

export type SpatialComputeBackendKind = 'webgpu' | 'webgl2-cpu';

export interface SpatialComputeBackendInfo {
  backend: SpatialComputeBackendKind;
  available: boolean;
  adapterLabel: string;
  maxComputeWorkgroupSizeX: number;
  fallbackReason: string | null;
}

export interface SpatialFilterKernelResult {
  backend: SpatialComputeBackendKind;
  matchedIndices: number[];
  matchedCount: number;
  densityGrid: Uint32Array;
  gridSize: number;
}

export interface DecodedFlatGeobufPlot {
  plotNumber: string;
  lat: number;
  lon: number;
  utmDerivedLat: number | null;
  utmDerivedLon: number | null;
  divergenceMeters: number | null;
  elevation: number | null;
  slope: number | null;
  aspect: number | null;
  fieldBgc: string;
  gisBgc: string;
  bgcMismatch: boolean;
  siteUnit: string;
}

export interface WorkbenchPlotPoint {
  index: number;
  plotNumber: string;
  lon: number;
  lat: number;
  hasUtm: boolean;
  utmDerivedLon: number | null;
  utmDerivedLat: number | null;
  divergenceMeters: number;
  divergenceFlagged: boolean;
  longitudeFlipped: boolean;
  elevation: number | null;
  slope: number | null;
  aspect: number | null;
  zone: string;
  subZone: string;
  fieldBgc: string;
  gisBgc: string;
  bgcMismatch: boolean;
  siteUnit: string;
  ecosection: string;
  fsRegionDistrict: string;
}

export interface DiscrepancyVectorSegment {
  plotNumber: string;
  sourcePosition: [number, number];
  targetPosition: [number, number];
  divergenceMeters: number;
  flagged: boolean;
}

export interface MapLayerVisibility {
  bgcFill: boolean;
  bgcLabels: boolean;
  ecosections: boolean;
  nrDistricts: boolean;
  hillshade3D: boolean;
  bcOpenMapsWMS: boolean;
  discrepancyVectors: boolean;
}

export const FLATGEOBUF_MAGIC_V3 = new Uint8Array([
  0x66, 0x67, 0x62, 0x03, 0x66, 0x67, 0x62, 0x01,
]);

// Official BC Ministry of Forests BEC v13 Zone thematic palette
export const BEC_ZONE_PALETTE: Record<string, string> = {
  CDF: '#a3cc51',
  CWH: '#20854e',
  MH: '#5c4d99',
  CMA: '#b8c4d0',
  IMA: '#c9d4df',
  BAFA: '#9eb0c2',
  ESSF: '#7a5299',
  ICH: '#2d9a8c',
  SBS: '#3b82b6',
  SBPS: '#6aaed6',
  MS: '#9e7bb5',
  IDF: '#d99b26',
  PP: '#e07b39',
  BG: '#c94a38',
  BWBS: '#4a90a4',
  SWB: '#6d8fa6',
};

export const DEFAULT_BC_VIEW_STATE = {
  longitude: -124.8,
  latitude: 54.2,
  zoom: 4.8,
  pitch: 0,
  bearing: 0,
};

/**
 * Generates the WGSL compute kernel for parallel spatial BBOX + radius filtering
 * and 2D spatial density binning across 71,567+ BECMaster plot coordinates.
 */
export function buildWGSLSpatialFilterShader(workgroupSize = 64): string {
  const wg = Math.max(1, Math.min(256, Math.trunc(workgroupSize)));
  return `struct SpatialFilterUniforms {
  minLon: f32,
  minLat: f32,
  maxLon: f32,
  maxLat: f32,
  gridSize: u32,
  count: u32,
  pad0: u32,
  pad1: u32,
};

@group(0) @binding(0) var<storage, read> coords: array<vec2<f32>>;
@group(0) @binding(1) var<uniform> params: SpatialFilterUniforms;
@group(0) @binding(2) var<storage, read_write> flags: array<u32>;
@group(0) @binding(3) var<storage, read_write> densityBins: array<atomic<u32>>;

@compute @workgroup_size(${wg})
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
  let idx = gid.x;
  if (idx >= params.count) {
    return;
  }
  let pt = coords[idx];
  let inside = pt.x >= params.minLon && pt.x <= params.maxLon &&
               pt.y >= params.minLat && pt.y <= params.maxLat;
  if (!inside) {
    flags[idx] = 0u;
    return;
  }
  flags[idx] = 1u;
  let spanLon = max(params.maxLon - params.minLon, 0.00001);
  let spanLat = max(params.maxLat - params.minLat, 0.00001);
  let nx = clamp((pt.x - params.minLon) / spanLon, 0.0, 0.999999);
  let ny = clamp((pt.y - params.minLat) / spanLat, 0.0, 0.999999);
  let gx = u32(nx * f32(params.gridSize));
  let gy = u32(ny * f32(params.gridSize));
  let binIdx = gy * params.gridSize + gx;
  atomicAdd(&densityBins[binIdx], 1u);
}
`;
}

interface MinimalGPUDevice {
  limits?: { maxComputeWorkgroupSizeX?: number };
  createShaderModule(descriptor: { code: string }): unknown;
  createBuffer?(descriptor: { size: number; usage: number }): any;
  createBindGroupLayout?(descriptor: unknown): unknown;
  createPipelineLayout?(descriptor: unknown): unknown;
  createComputePipeline?(descriptor: unknown): any;
  createBindGroup?(descriptor: unknown): unknown;
  createCommandEncoder?(): any;
  queue?: {
    writeBuffer(buffer: any, offset: number, data: BufferSource): void;
    submit(commands: unknown[]): void;
  };
  destroy?(): void;
}

interface MinimalGPUAdapter {
  info?: { vendor?: string; architecture?: string; description?: string };
  limits?: { maxComputeWorkgroupSizeX?: number };
  requestDevice(): Promise<MinimalGPUDevice | null>;
}

interface MinimalNavigatorWithGPU {
  gpu?: {
    requestAdapter(): Promise<MinimalGPUAdapter | null>;
  };
}

/**
 * Probes navigator.gpu (requestAdapter + requestDevice + WGSL shader module compilation)
 * and falls back cleanly to WebGL2 + TypedArray CPU execution when WebGPU is unavailable.
 */
export async function detectSpatialComputeBackend(
  nav?: unknown
): Promise<SpatialComputeBackendInfo> {
  const targetNav = (nav !== undefined
    ? nav
    : typeof navigator !== 'undefined'
      ? navigator
      : undefined) as MinimalNavigatorWithGPU | undefined;

  if (!targetNav || !targetNav.gpu || typeof targetNav.gpu.requestAdapter !== 'function') {
    return {
      backend: 'webgl2-cpu',
      available: false,
      adapterLabel: 'WebGL2 + TypedArray CPU Fallback',
      maxComputeWorkgroupSizeX: 0,
      fallbackReason: 'navigator.gpu is unavailable in this WebView runtime',
    };
  }

  try {
    const adapter = await targetNav.gpu.requestAdapter();
    if (!adapter || typeof adapter.requestDevice !== 'function') {
      return {
        backend: 'webgl2-cpu',
        available: false,
        adapterLabel: 'WebGL2 + TypedArray CPU Fallback',
        maxComputeWorkgroupSizeX: 0,
        fallbackReason: 'WebGPU adapter unavailable on active graphics driver',
      };
    }
    const device = await adapter.requestDevice();
    if (!device || typeof device.createShaderModule !== 'function') {
      return {
        backend: 'webgl2-cpu',
        available: false,
        adapterLabel: 'WebGL2 + TypedArray CPU Fallback',
        maxComputeWorkgroupSizeX: 0,
        fallbackReason: 'WebGPU logical device creation returned null',
      };
    }
    device.createShaderModule({ code: buildWGSLSpatialFilterShader(64) });
    const labelParts = [
      adapter.info?.vendor,
      adapter.info?.description || adapter.info?.architecture,
    ].filter(Boolean);
    const maxWg =
      device.limits?.maxComputeWorkgroupSizeX ??
      adapter.limits?.maxComputeWorkgroupSizeX ??
      256;
    device.destroy?.();
    return {
      backend: 'webgpu',
      available: true,
      adapterLabel: labelParts.length > 0 ? labelParts.join(' - ') : 'WebGPU Hardware Compute',
      maxComputeWorkgroupSizeX: maxWg,
      fallbackReason: null,
    };
  } catch (cause) {
    return {
      backend: 'webgl2-cpu',
      available: false,
      adapterLabel: 'WebGL2 + TypedArray CPU Fallback',
      maxComputeWorkgroupSizeX: 0,
      fallbackReason: cause instanceof Error ? cause.message : String(cause),
    };
  }
}

export function executeCPUSpatialFilterKernel(
  coordsLonLat: Float32Array,
  bounds: readonly [number, number, number, number],
  gridSize = 16
): SpatialFilterKernelResult {
  const safeGrid = Math.max(1, Math.min(64, Math.trunc(gridSize)));
  const count = Math.floor(coordsLonLat.length / 2);
  const [minLon, minLat, maxLon, maxLat] = bounds;
  const spanLon = Math.max(maxLon - minLon, 1e-5);
  const spanLat = Math.max(maxLat - minLat, 1e-5);
  const densityGrid = new Uint32Array(safeGrid * safeGrid);
  const matchedIndices: number[] = [];

  for (let i = 0; i < count; i++) {
    const lon = coordsLonLat[i * 2];
    const lat = coordsLonLat[i * 2 + 1];
    if (lon >= minLon && lon <= maxLon && lat >= minLat && lat <= maxLat) {
      matchedIndices.push(i);
      const nx = Math.min(0.999999, Math.max(0, (lon - minLon) / spanLon));
      const ny = Math.min(0.999999, Math.max(0, (lat - minLat) / spanLat));
      const gx = Math.floor(nx * safeGrid);
      const gy = Math.floor(ny * safeGrid);
      densityGrid[gy * safeGrid + gx]++;
    }
  }

  return {
    backend: 'webgl2-cpu',
    matchedIndices,
    matchedCount: matchedIndices.length,
    densityGrid,
    gridSize: safeGrid,
  };
}

/**
 * Dispatches the WGSL spatial filter & density-bin compute shader on WebGPU when available,
 * falling back deterministically to executeCPUSpatialFilterKernel otherwise.
 */
export async function executeSpatialFilterKernel(
  coordsLonLat: Float32Array,
  bounds: readonly [number, number, number, number],
  options?: { nav?: unknown; gridSize?: number }
): Promise<SpatialFilterKernelResult> {
  const safeGrid = Math.max(1, Math.min(64, Math.trunc(options?.gridSize ?? 16)));
  const count = Math.floor(coordsLonLat.length / 2);
  if (count === 0) {
    return {
      backend: 'webgl2-cpu',
      matchedIndices: [],
      matchedCount: 0,
      densityGrid: new Uint32Array(safeGrid * safeGrid),
      gridSize: safeGrid,
    };
  }

  const targetNav = (options?.nav !== undefined
    ? options.nav
    : typeof navigator !== 'undefined'
      ? navigator
      : undefined) as MinimalNavigatorWithGPU | undefined;

  const gpuBufUsage = (globalThis as any).GPUBufferUsage;
  const gpuMapMode = (globalThis as any).GPUMapMode;

  if (targetNav?.gpu && gpuBufUsage && gpuMapMode) {
    let device: MinimalGPUDevice | null = null;
    try {
      const adapter = await targetNav.gpu.requestAdapter();
      device = adapter ? await adapter.requestDevice() : null;
      if (
        device &&
        device.createBuffer &&
        device.createComputePipeline &&
        device.createBindGroup &&
        device.createCommandEncoder &&
        device.queue
      ) {
        const shaderModule = device.createShaderModule({
          code: buildWGSLSpatialFilterShader(64),
        });
        const pipeline = device.createComputePipeline({
          layout: 'auto',
          compute: { module: shaderModule, entryPoint: 'main' },
        });

        const coordsBytes = Math.max(8, coordsLonLat.byteLength);
        const flagsBytes = Math.max(4, count * 4);
        const binsBytes = safeGrid * safeGrid * 4;

        const coordsBuffer = device.createBuffer({
          size: coordsBytes,
          usage: gpuBufUsage.STORAGE | gpuBufUsage.COPY_DST,
        });
        const uniformBuffer = device.createBuffer({
          size: 32,
          usage: gpuBufUsage.UNIFORM | gpuBufUsage.COPY_DST,
        });
        const flagsBuffer = device.createBuffer({
          size: flagsBytes,
          usage: gpuBufUsage.STORAGE | gpuBufUsage.COPY_SRC,
        });
        const binsBuffer = device.createBuffer({
          size: binsBytes,
          usage: gpuBufUsage.STORAGE | gpuBufUsage.COPY_SRC | gpuBufUsage.COPY_DST,
        });
        const flagsReadBuffer = device.createBuffer({
          size: flagsBytes,
          usage: gpuBufUsage.MAP_READ | gpuBufUsage.COPY_DST,
        });
        const binsReadBuffer = device.createBuffer({
          size: binsBytes,
          usage: gpuBufUsage.MAP_READ | gpuBufUsage.COPY_DST,
        });

        const uniformData = new ArrayBuffer(32);
        const uv = new DataView(uniformData);
        uv.setFloat32(0, bounds[0], true);
        uv.setFloat32(4, bounds[1], true);
        uv.setFloat32(8, bounds[2], true);
        uv.setFloat32(12, bounds[3], true);
        uv.setUint32(16, safeGrid, true);
        uv.setUint32(20, count, true);
        uv.setUint32(24, 0, true);
        uv.setUint32(28, 0, true);

        device.queue.writeBuffer(coordsBuffer, 0, coordsLonLat as unknown as BufferSource);
        device.queue.writeBuffer(uniformBuffer, 0, uniformData);
        device.queue.writeBuffer(binsBuffer, 0, new Uint32Array(safeGrid * safeGrid));

        const bindGroup = device.createBindGroup({
          layout: pipeline.getBindGroupLayout(0),
          entries: [
            { binding: 0, resource: { buffer: coordsBuffer } },
            { binding: 1, resource: { buffer: uniformBuffer } },
            { binding: 2, resource: { buffer: flagsBuffer } },
            { binding: 3, resource: { buffer: binsBuffer } },
          ],
        });

        const encoder = device.createCommandEncoder();
        const pass = encoder.beginComputePass();
        pass.setPipeline(pipeline);
        pass.setBindGroup(0, bindGroup);
        pass.dispatchWorkgroups(Math.ceil(count / 64));
        pass.end();

        encoder.copyBufferToBuffer(flagsBuffer, 0, flagsReadBuffer, 0, flagsBytes);
        encoder.copyBufferToBuffer(binsBuffer, 0, binsReadBuffer, 0, binsBytes);
        device.queue.submit([encoder.finish()]);

        await Promise.all([
          flagsReadBuffer.mapAsync(gpuMapMode.READ),
          binsReadBuffer.mapAsync(gpuMapMode.READ),
        ]);

        const flagsArr = new Uint32Array(flagsReadBuffer.getMappedRange().slice(0));
        const densityGrid = new Uint32Array(binsReadBuffer.getMappedRange().slice(0));
        flagsReadBuffer.unmap();
        binsReadBuffer.unmap();

        const matchedIndices: number[] = [];
        for (let i = 0; i < count; i++) {
          if (flagsArr[i] === 1) matchedIndices.push(i);
        }

        coordsBuffer.destroy?.();
        uniformBuffer.destroy?.();
        flagsBuffer.destroy?.();
        binsBuffer.destroy?.();
        flagsReadBuffer.destroy?.();
        binsReadBuffer.destroy?.();
        device.destroy?.();

        return {
          backend: 'webgpu',
          matchedIndices,
          matchedCount: matchedIndices.length,
          densityGrid,
          gridSize: safeGrid,
        };
      }
    } catch {
      device?.destroy?.();
    }
  }

  return executeCPUSpatialFilterKernel(coordsLonLat, bounds, safeGrid);
}

// --- OGC FlatGeobuf v3 Point Stream Decoder ---

const BASE64_LOOKUP = (() => {
  const table = new Int16Array(128).fill(-1);
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  for (let i = 0; i < alphabet.length; i++) {
    table[alphabet.charCodeAt(i)] = i;
  }
  table['-'.charCodeAt(0)] = 62;
  table['_'.charCodeAt(0)] = 63;
  return table;
})();

function decodeBase64Pure(raw: string): Uint8Array {
  const cleaned = raw.replace(/[\r\n\s=]+/g, '');
  const outLen = Math.floor((cleaned.length * 3) / 4);
  const out = new Uint8Array(outLen);
  let outIdx = 0;
  let acc = 0;
  let bits = 0;
  for (let i = 0; i < cleaned.length; i++) {
    const code = cleaned.charCodeAt(i);
    const val = code < 128 ? BASE64_LOOKUP[code] : -1;
    if (val < 0) continue;
    acc = (acc << 6) | val;
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      out[outIdx++] = (acc >> bits) & 0xff;
    }
  }
  return outIdx === outLen ? out : out.subarray(0, outIdx);
}

function decodeUTF8Slice(bytes: Uint8Array): string {
  if (typeof TextDecoder !== 'undefined') {
    return new TextDecoder('utf-8').decode(bytes);
  }
  let out = '';
  let i = 0;
  while (i < bytes.length) {
    const b0 = bytes[i++];
    if (b0 < 0x80) {
      out += String.fromCharCode(b0);
    } else if ((b0 & 0xe0) === 0xc0 && i < bytes.length) {
      const b1 = bytes[i++] & 0x3f;
      out += String.fromCharCode(((b0 & 0x1f) << 6) | b1);
    } else if ((b0 & 0xf0) === 0xe0 && i + 1 < bytes.length) {
      const b1 = bytes[i++] & 0x3f;
      const b2 = bytes[i++] & 0x3f;
      out += String.fromCharCode(((b0 & 0x0f) << 12) | (b1 << 6) | b2);
    } else if ((b0 & 0xf8) === 0xf0 && i + 2 < bytes.length) {
      const b1 = bytes[i++] & 0x3f;
      const b2 = bytes[i++] & 0x3f;
      const b3 = bytes[i++] & 0x3f;
      const cp = ((b0 & 0x07) << 18) | (b1 << 12) | (b2 << 6) | b3;
      out += String.fromCodePoint(cp);
    }
  }
  return out;
}

function toUint8Array(input: Uint8Array | ArrayBuffer | string | null | undefined): Uint8Array {
  if (!input) return new Uint8Array(0);
  if (input instanceof Uint8Array) return input;
  if (ArrayBuffer.isView(input)) {
    return new Uint8Array(input.buffer, input.byteOffset, input.byteLength);
  }
  if (input instanceof ArrayBuffer || Object.prototype.toString.call(input) === '[object ArrayBuffer]') {
    return new Uint8Array(input as ArrayBuffer);
  }
  if (typeof input === 'string') {
    return decodeBase64Pure(input);
  }
  return new Uint8Array(0);
}

function fbFieldPos(dv: DataView, tablePos: number, fieldIdx: number): number | null {
  if (tablePos < 0 || tablePos + 4 > dv.byteLength) return null;
  const soffset = dv.getInt32(tablePos, true);
  const vtPos = tablePos - soffset;
  if (vtPos < 0 || vtPos + 4 > dv.byteLength) return null;
  const vtLen = dv.getUint16(vtPos, true);
  const slotOff = 4 + 2 * fieldIdx;
  if (slotOff + 2 > vtLen || vtPos + slotOff + 2 > dv.byteLength) return null;
  const rel = dv.getUint16(vtPos + slotOff, true);
  if (rel === 0 || tablePos + rel > dv.byteLength) return null;
  return tablePos + rel;
}

function fbDeref(dv: DataView, slotPos: number): number | null {
  if (slotPos < 0 || slotPos + 4 > dv.byteLength) return null;
  const target = slotPos + dv.getUint32(slotPos, true);
  return target >= 0 && target <= dv.byteLength ? target : null;
}

function fbReadString(
  dv: DataView,
  bytes: Uint8Array,
  slotPos: number
): string {
  const strPos = fbDeref(dv, slotPos);
  if (strPos === null || strPos + 4 > dv.byteLength) return '';
  const len = dv.getUint32(strPos, true);
  if (strPos + 4 + len > dv.byteLength) return '';
  return decodeUTF8Slice(bytes.subarray(strPos + 4, strPos + 4 + len));
}

/**
 * Decodes an OGC FlatGeobuf v3 binary buffer (or Wails base64 string) into plot point records.
 */
export function decodeFlatGeobufPoints(
  input: Uint8Array | ArrayBuffer | string | null | undefined
): DecodedFlatGeobufPlot[] {
  const bytes = toUint8Array(input);
  if (bytes.byteLength === 0) return [];
  if (bytes.byteLength < 12) {
    throw new Error('FlatGeobuf buffer shorter than 12-byte magic + header prefix');
  }
  for (let i = 0; i < 8; i++) {
    if (bytes[i] !== FLATGEOBUF_MAGIC_V3[i]) {
      throw new Error('Invalid OGC FlatGeobuf v3 magic signature');
    }
  }

  const fullDV = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let pos = 8;
  const hdrLen = fullDV.getUint32(pos, true);
  pos += 4;
  if (hdrLen < 8 || pos + hdrLen > bytes.byteLength) {
    throw new Error('FlatGeobuf header size exceeds buffer bounds');
  }

  const hdrBytes = bytes.subarray(pos, pos + hdrLen);
  const hdrDV = new DataView(hdrBytes.buffer, hdrBytes.byteOffset, hdrBytes.byteLength);
  pos += hdrLen;

  const hdrRoot = fbDeref(hdrDV, 0);
  if (hdrRoot === null) {
    throw new Error('Invalid FlatGeobuf header root offset');
  }

  const colDefs: Array<{ name: string; colType: number }> = [];
  const colsSlot = fbFieldPos(hdrDV, hdrRoot, 7);
  if (colsSlot !== null) {
    const colsVec = fbDeref(hdrDV, colsSlot);
    if (colsVec !== null && colsVec + 4 <= hdrDV.byteLength) {
      const nCols = hdrDV.getUint32(colsVec, true);
      for (let i = 0; i < nCols; i++) {
        const elemSlot = colsVec + 4 + i * 4;
        const colTbl = fbDeref(hdrDV, elemSlot);
        if (colTbl === null) throw new Error('Invalid FlatGeobuf column table offset');
        const nameSlot = fbFieldPos(hdrDV, colTbl, 0);
        const typeSlot = fbFieldPos(hdrDV, colTbl, 1);
        colDefs.push({
          name: nameSlot !== null ? fbReadString(hdrDV, hdrBytes, nameSlot) : '',
          colType: typeSlot !== null ? hdrDV.getUint8(typeSlot) : 0,
        });
      }
    }
  }

  const out: DecodedFlatGeobufPlot[] = [];
  while (pos < bytes.byteLength) {
    if (pos + 4 > bytes.byteLength) {
      throw new Error('Truncated FlatGeobuf feature size prefix');
    }
    const fLen = fullDV.getUint32(pos, true);
    pos += 4;
    if (fLen < 4 || pos + fLen > bytes.byteLength) {
      throw new Error('FlatGeobuf feature size exceeds buffer bounds');
    }
    const fBytes = bytes.subarray(pos, pos + fLen);
    const fDV = new DataView(fBytes.buffer, fBytes.byteOffset, fBytes.byteLength);
    pos += fLen;

    const fRoot = fbDeref(fDV, 0);
    if (fRoot === null) throw new Error('Invalid FlatGeobuf feature root offset');

    const rec: DecodedFlatGeobufPlot = {
      plotNumber: '',
      lat: 0,
      lon: 0,
      utmDerivedLat: null,
      utmDerivedLon: null,
      divergenceMeters: null,
      elevation: null,
      slope: null,
      aspect: null,
      fieldBgc: '',
      gisBgc: '',
      bgcMismatch: false,
      siteUnit: '',
    };

    const geomSlot = fbFieldPos(fDV, fRoot, 0);
    if (geomSlot !== null) {
      const geomTbl = fbDeref(fDV, geomSlot);
      if (geomTbl !== null) {
        const xySlot = fbFieldPos(fDV, geomTbl, 1);
        if (xySlot !== null) {
          const xyVec = fbDeref(fDV, xySlot);
          if (xyVec !== null && xyVec + 20 <= fDV.byteLength) {
            const nCoords = fDV.getUint32(xyVec, true);
            if (nCoords >= 2) {
              rec.lon = fDV.getFloat64(xyVec + 4, true);
              rec.lat = fDV.getFloat64(xyVec + 12, true);
            }
          }
        }
      }
    }

    const propSlot = fbFieldPos(fDV, fRoot, 1);
    if (propSlot !== null) {
      const propVec = fbDeref(fDV, propSlot);
      if (propVec !== null && propVec + 4 <= fDV.byteLength) {
        const pLen = fDV.getUint32(propVec, true);
        if (propVec + 4 + pLen <= fDV.byteLength) {
          const pSub = fBytes.subarray(propVec + 4, propVec + 4 + pLen);
          const pDV = new DataView(pSub.buffer, pSub.byteOffset, pSub.byteLength);
          let pIdx = 0;
          while (pIdx + 2 <= pSub.byteLength) {
            const colIdx = pDV.getUint16(pIdx, true);
            pIdx += 2;
            const col = colDefs[colIdx];
            if (!col) break;
            if (col.colType === 11) {
              if (pIdx + 4 > pSub.byteLength) break;
              const sLen = pDV.getUint32(pIdx, true);
              pIdx += 4;
              if (pIdx + sLen > pSub.byteLength) break;
              const sv = decodeUTF8Slice(pSub.subarray(pIdx, pIdx + sLen));
              pIdx += sLen;
              if (col.name === 'PlotNumber') rec.plotNumber = sv;
              else if (col.name === 'FieldBGC') rec.fieldBgc = sv;
              else if (col.name === 'GIS_BGC') rec.gisBgc = sv;
              else if (col.name === 'SiteUnit') rec.siteUnit = sv;
            } else if (col.colType === 8) {
              if (pIdx + 8 > pSub.byteLength) break;
              const dv = pDV.getFloat64(pIdx, true);
              pIdx += 8;
              if (col.name === 'Lat') rec.lat = dv;
              else if (col.name === 'Lon') rec.lon = dv;
              else if (col.name === 'UTMDerivedLat') rec.utmDerivedLat = dv;
              else if (col.name === 'UTMDerivedLon') rec.utmDerivedLon = dv;
              else if (col.name === 'DivergenceMeters') rec.divergenceMeters = dv;
              else if (col.name === 'Elevation') rec.elevation = dv;
              else if (col.name === 'Slope') rec.slope = dv;
              else if (col.name === 'Aspect') rec.aspect = dv;
            } else if (col.colType === 5) {
              if (pIdx + 1 > pSub.byteLength) break;
              const bv = pDV.getUint8(pIdx) !== 0;
              pIdx += 1;
              if (col.name === 'BGCMismatch') rec.bgcMismatch = bv;
            } else {
              break;
            }
          }
        }
      }
    }

    out.push(rec);
  }

  return out;
}

/**
 * Even-odd Jordan curve ray-casting test for an EPSG:4326 [lon, lat] polygon ring.
 */
export function pointInPolygonRing(
  lon: number,
  lat: number,
  ring: ReadonlyArray<readonly [number, number]>
): boolean {
  if (!Number.isFinite(lon) || !Number.isFinite(lat) || ring.length < 3) {
    return false;
  }
  const effectiveLen =
    ring.length >= 4 &&
    ring[ring.length - 1][0] === ring[0][0] &&
    ring[ring.length - 1][1] === ring[0][1]
      ? ring.length - 1
      : ring.length;
  if (effectiveLen < 3) return false;

  let inside = false;
  let j = effectiveLen - 1;
  for (let i = 0; i < effectiveLen; i++) {
    const xi = ring[i][0];
    const yi = ring[i][1];
    const xj = ring[j][0];
    const yj = ring[j][1];
    if (yi > lat !== yj > lat && lon < ((xj - xi) * (lat - yi)) / (yj - yi) + xi) {
      inside = !inside;
    }
    j = i;
  }
  return inside;
}

export function materializeWorkbenchPlots(cols: SpatialGeoArrowColumns | null | undefined): {
  points: WorkbenchPlotPoint[];
  coordsFloat32: Float32Array;
  discrepancyVectors: DiscrepancyVectorSegment[];
} {
  if (!cols || !cols.count) {
    return { points: [], coordsFloat32: new Float32Array(0), discrepancyVectors: [] };
  }
  const n = cols.count;
  const points: WorkbenchPlotPoint[] = new Array(n);
  const coordsFloat32 = new Float32Array(n * 2);
  const discrepancyVectors: DiscrepancyVectorSegment[] = [];

  for (let i = 0; i < n; i++) {
    const lon = cols.lon?.[i] ?? 0;
    const lat = cols.lat?.[i] ?? 0;
    const hasUtm = cols.hasUtm?.[i] ?? false;
    const utmLon = hasUtm ? (cols.utmDerivedLon?.[i] ?? null) : null;
    const utmLat = hasUtm ? (cols.utmDerivedLat?.[i] ?? null) : null;
    const divM = cols.divergenceMeters?.[i] ?? 0;
    const divFlag = cols.divergenceFlagged?.[i] ?? false;

    coordsFloat32[i * 2] = lon;
    coordsFloat32[i * 2 + 1] = lat;

    const pt: WorkbenchPlotPoint = {
      index: i,
      plotNumber: cols.plotNumber?.[i] ?? '',
      lon,
      lat,
      hasUtm,
      utmDerivedLon: utmLon,
      utmDerivedLat: utmLat,
      divergenceMeters: divM,
      divergenceFlagged: divFlag,
      longitudeFlipped: cols.longitudeFlipped?.[i] ?? false,
      elevation: cols.hasElevation?.[i] ? (cols.elevation?.[i] ?? null) : null,
      slope: cols.hasSlope?.[i] ? (cols.slope?.[i] ?? null) : null,
      aspect: cols.hasAspect?.[i] ? (cols.aspect?.[i] ?? null) : null,
      zone: cols.zone?.[i] ?? '',
      subZone: cols.subZone?.[i] ?? '',
      fieldBgc: cols.fieldBgc?.[i] ?? '',
      gisBgc: cols.gisBgc?.[i] ?? '',
      bgcMismatch: cols.bgcMismatch?.[i] ?? false,
      siteUnit: cols.siteUnit?.[i] ?? '',
      ecosection: cols.ecosection?.[i] ?? '',
      fsRegionDistrict: cols.fsRegionDistrict?.[i] ?? '',
    };
    points[i] = pt;

    if (hasUtm && utmLon !== null && utmLat !== null && divM >= 5.0) {
      discrepancyVectors.push({
        plotNumber: pt.plotNumber,
        sourcePosition: [lon, lat],
        targetPosition: [utmLon, utmLat],
        divergenceMeters: divM,
        flagged: divFlag,
      });
    }
  }

  return { points, coordsFloat32, discrepancyVectors };
}

export function buildBECZoneMatchExpression(): any[] {
  const expr: any[] = ['match', ['get', 'ZONE']];
  for (const [zone, hex] of Object.entries(BEC_ZONE_PALETTE)) {
    expr.push(zone, hex);
  }
  expr.push('#4b6b88');
  return expr;
}

/**
 * Builds the offline-first MapLibre GL JS v6 style specification backed by local PMTiles
 * byte-range sources and optional BC OpenMaps WMS orthoimagery.
 */
export function buildMapLibreGeospatialStyle(
  manifest: PMTilesManifest | null | undefined,
  visibility: MapLayerVisibility
): Record<string, any> {
  const bgcArchive = manifest?.archives?.['bc-bgc-v13.pmtiles'];
  const adminArchive = manifest?.archives?.['bc-admin-ecosections-districts.pmtiles'];
  const bgcMaxZoom = bgcArchive?.maxZoom && bgcArchive.maxZoom > 0 ? bgcArchive.maxZoom : 6;
  const adminMaxZoom = adminArchive?.maxZoom && adminArchive.maxZoom > 0 ? adminArchive.maxZoom : 6;

  const sources: Record<string, any> = {
    bc_bgc_pmtiles: {
      type: 'vector',
      url: 'pmtiles:///maps/bc-bgc-v13.pmtiles',
      minzoom: 3,
      maxzoom: bgcMaxZoom,
      attribution: 'BC Ministry of Forests BEC v13',
    },
    bc_admin_pmtiles: {
      type: 'vector',
      url: 'pmtiles:///maps/bc-admin-ecosections-districts.pmtiles',
      minzoom: 3,
      maxzoom: adminMaxZoom,
      attribution: 'GeoBC Ecosections & NR Districts',
    },
  };

  if (visibility.bcOpenMapsWMS) {
    sources.bc_openmaps_wms = {
      type: 'raster',
      tiles: [
        'https://openmaps.gov.bc.ca/geo/pub/ows?service=WMS&version=1.3.0&request=GetMap&layers=pub:WHSE_BASEMAPPING.DRA_DGTL_ROAD_ATLAS_MPAR_SP&styles=&crs=EPSG:3857&bbox={bbox-epsg-3857}&width=256&height=256&format=image/png&transparent=true',
      ],
      tileSize: 256,
      attribution: 'BC Data Catalogue OpenMaps WMS',
    };
  }

  const layers: Record<string, any>[] = [
    {
      id: 'vpro-ocean-background',
      type: 'background',
      paint: {
        'background-color': visibility.hillshade3D ? '#0c2238' : '#dde7f0',
      },
    },
    {
      id: 'bgc-zones-fill',
      type: 'fill',
      source: 'bc_bgc_pmtiles',
      'source-layer': 'bgc',
      layout: {
        visibility: visibility.bgcFill ? 'visible' : 'none',
      },
      paint: {
        'fill-color': buildBECZoneMatchExpression(),
        'fill-opacity': visibility.hillshade3D ? 0.62 : 0.48,
      },
    },
    {
      id: 'bgc-subzone-outline',
      type: 'line',
      source: 'bc_bgc_pmtiles',
      'source-layer': 'bgc',
      layout: {
        visibility: visibility.bgcFill ? 'visible' : 'none',
      },
      paint: {
        'line-color': '#1e3a5f',
        'line-width': ['interpolate', ['linear'], ['zoom'], 4, 0.35, 9, 1.1],
        'line-opacity': 0.55,
      },
    },
    {
      id: 'ecosections-outline',
      type: 'line',
      source: 'bc_admin_pmtiles',
      'source-layer': 'ecosections',
      layout: {
        visibility: visibility.ecosections ? 'visible' : 'none',
      },
      paint: {
        'line-color': '#003366',
        'line-width': 1.4,
        'line-dasharray': [3, 2],
        'line-opacity': 0.85,
      },
    },
    {
      id: 'nr-districts-outline',
      type: 'line',
      source: 'bc_admin_pmtiles',
      'source-layer': 'nr_districts',
      layout: {
        visibility: visibility.nrDistricts ? 'visible' : 'none',
      },
      paint: {
        'line-color': '#b45309',
        'line-width': 1.8,
        'line-opacity': 0.9,
      },
    },
  ];

  if (visibility.bcOpenMapsWMS) {
    layers.push({
      id: 'bc-openmaps-wms-layer',
      type: 'raster',
      source: 'bc_openmaps_wms',
      paint: { 'raster-opacity': 0.72 },
    });
  }

  return {
    version: 8,
    name: 'VPro Offline BGC v13 Geospatial Style',
    sources,
    layers,
  };
}

export function summarizeArchiveResolution(payload: SpatialWorkbenchPayload | null): string {
  if (!payload?.manifest?.archives) return 'Resolving local PMTiles archives...';
  const bgc = payload.manifest.archives['bc-bgc-v13.pmtiles'];
  if (!bgc) return 'Embedded Overview PMTiles (z3..6)';
  if (bgc.fullResolution) {
    return `Full-Res Provincial PMTiles (z${bgc.minZoom}..${bgc.maxZoom})`;
  }
  return `Embedded Overview Fallback (z${bgc.minZoom}..${bgc.maxZoom})`;
}

export function classifyWMSMapError(
  event: unknown,
  wmsEnabled: boolean,
  online?: boolean
): string | null {
  if (!wmsEnabled) return null;
  if (online === false) {
    return 'BC OpenMaps WMS imagery is unreachable while offline. Offline BGC v13, Ecosections, NR Districts, and plot layers remain active.';
  }
  if (!event || typeof event !== 'object') return null;
  const rec = event as Record<string, any>;
  const sourceId = typeof rec.sourceId === 'string' ? rec.sourceId : '';
  const errObj = rec.error;
  const errMsg =
    errObj instanceof Error
      ? errObj.message
      : typeof errObj?.message === 'string'
        ? errObj.message
        : typeof rec.message === 'string'
          ? rec.message
          : '';
  const errUrl =
    typeof errObj?.url === 'string'
      ? errObj.url
      : typeof rec.tile?.tileID === 'object' || typeof rec.url === 'string'
        ? String(rec.url ?? '')
        : '';
  if (
    sourceId === 'bc_openmaps_wms' ||
    errUrl.includes('openmaps.gov.bc.ca') ||
    errMsg.toLowerCase().includes('openmaps.gov.bc.ca') ||
    errMsg.toLowerCase().includes('failed to fetch') ||
    errMsg.toLowerCase().includes('networkerror') ||
    errMsg.toLowerCase().includes('load tile')
  ) {
    return 'BC OpenMaps WMS tile request failed (offline or remote service unreachable). Offline PMTiles vector layers remain active.';
  }
  return null;
}
