<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Map as MapLibreMap,
    NavigationControl,
    ScaleControl,
    addProtocol,
    setWorkerUrl,
    type GeoJSONSource,
    type IControl,
  } from 'maplibre-gl';
  import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
  import 'maplibre-gl/dist/maplibre-gl.css';
  import { Protocol } from 'pmtiles';
  import { MapboxOverlay } from '@deck.gl/mapbox';
  import { LineLayer, ScatterplotLayer } from '@deck.gl/layers';
  import {
    SpatialService,
    type ApplySpatialAttributesRequest,
    type AssignSpatialAOIToSURequest,
    type SpatialAOISelectionResult,
    type SpatialAttributeSuggestion,
    type SpatialBatchAuditReport,
    type SpatialWorkbenchPayload,
  } from '../bindings/github.com/boostao/vpro-wails';

  type SpatialPlotQARow = NonNullable<SpatialBatchAuditReport['rows']>[number];
  import {
    BEC_ZONE_PALETTE,
    DEFAULT_BC_VIEW_STATE,
    buildMapLibreGeospatialStyle,
    decodeFlatGeobufPoints,
    detectSpatialComputeBackend,
    executeSpatialFilterKernel,
    materializeWorkbenchPlots,
    pointInPolygonRing,
    summarizeArchiveResolution,
    type DiscrepancyVectorSegment,
    type MapLayerVisibility,
    type SpatialComputeBackendInfo,
    type WorkbenchPlotPoint,
  } from './geospatialWorkbench';

  let {
    contextId,
    project,
    su,
    onBusyChange,
    onContextChanged,
    onOpenPlot,
  }: {
    contextId: string;
    project: string;
    su: string;
    onBusyChange?: (busy: boolean) => void;
    onContextChanged?: () => Promise<void> | void;
    onOpenPlot?: (plotNumber: string) => void;
  } = $props();

  let mapContainer: HTMLDivElement | undefined = $state();
  let mapInstance: MapLibreMap | null = null;
  let deckOverlay: MapboxOverlay | null = null;
  let alive = true;
  let loadGeneration = 0;
  let qaGeneration = 0;
  let aoiGeneration = 0;
  let inspectorGeneration = 0;

  let loading = $state(false);
  let mutating = $state(false);
  let errorMsg = $state('');
  let statusMsg = $state('');
  let mapRenderWarning = $state('');

  let computeInfo = $state<SpatialComputeBackendInfo>({
    backend: 'webgl2-cpu',
    available: false,
    adapterLabel: 'Probing GPU compute...',
    maxComputeWorkgroupSizeX: 0,
    fallbackReason: null,
  });

  let payload = $state<SpatialWorkbenchPayload | null>(null);
  let plotPoints = $state.raw<WorkbenchPlotPoint[]>([]);
  let plotCoordsF32 = $state.raw<Float32Array>(new Float32Array(0));
  let discrepancySegments = $state.raw<DiscrepancyVectorSegment[]>([]);
  let flatGeobufCount = $state(0);
  let viewportMatchedCount = $state(0);

  let visibility = $state<MapLayerVisibility>({
    bgcFill: true,
    bgcLabels: true,
    ecosections: false,
    nrDistricts: false,
    hillshade3D: false,
    bcOpenMapsWMS: false,
    discrepancyVectors: true,
  });

  let zoneFilter = $state('');
  let plotSearch = $state('');
  let activePanel = $state<'inspector' | 'aoi' | 'qa'>('inspector');

  // Selected Plot Pin Inspector state
  let selectedPlot = $state<WorkbenchPlotPoint | null>(null);
  let selectedSuggestion = $state<SpatialAttributeSuggestion | null>(null);
  let suggestionBusy = $state(false);

  // Interactive Polygon / Lasso AOI -> *_SU state
  let aoiDrawMode = $state(false);
  let aoiVertices = $state<Array<[number, number]>>([]);
  let aoiPreview = $state<SpatialAOISelectionResult | null>(null);
  let aoiTargetMode = $state<'update_active' | 'create_project_su' | 'create_su_file'>('create_project_su');
  let aoiSUName = $state('');
  let aoiFilePath = $state('');
  let aoiSiteUnit = $state('');
  let aoiIncludeUnassigned = $state(false);
  let aoiActivate = $state(true);

  // Spatial QA & Auto-1 Batch Stamp Drawer state
  let qaReport = $state<SpatialBatchAuditReport | null>(null);
  let qaFilterOnlyIssues = $state(true);
  let qaSelectedPlots = $state<Record<string, boolean>>({});
  let stampGISBGC = $state(true);
  let stampAdminFields = $state(true);
  let stampOnlyWhenBlank = $state(true);
  let normalizeLonSign = $state(true);
  let syncUTMFromDD = $state(true);

  const aoiMatchedSet = $derived(new Set(aoiPreview?.plotNumbers ?? []));
  const filteredPoints = $derived.by(() => {
    const zf = zoneFilter.trim().toUpperCase();
    const sq = plotSearch.trim().toLowerCase();
    return plotPoints.filter((pt) => {
      if (zf && pt.zone.toUpperCase() !== zf && !pt.gisBgc.toUpperCase().startsWith(zf)) {
        return false;
      }
      if (sq && !pt.plotNumber.toLowerCase().includes(sq) && !pt.siteUnit.toLowerCase().includes(sq)) {
        return false;
      }
      return true;
    });
  });

  const filteredQARows = $derived.by((): SpatialPlotQARow[] => {
    const rows = qaReport?.rows ?? [];
    if (!qaFilterOnlyIssues) return rows;
    return rows.filter(
      (r) =>
        r.bgcMismatch ||
        r.divergenceFlagged ||
        r.longitudeSignFlipped ||
        r.elevationFlagged ||
        (!r.storedGisBgc && !!r.suggestedGisBgc)
    );
  });

  const distinctZones = $derived.by(() => {
    const set = new Set<string>();
    for (const p of plotPoints) {
      if (p.zone) set.add(p.zone);
    }
    return [...set].sort();
  });

  function setBusyState(nextBusy: boolean) {
    onBusyChange?.(nextBusy);
  }

  let maplibreWorkerConfigured = false;
  function ensureMapLibreWorker() {
    if (maplibreWorkerConfigured) return;
    if (typeof setWorkerUrl === 'function' && maplibreWorkerUrl) {
      setWorkerUrl(maplibreWorkerUrl);
    }
    maplibreWorkerConfigured = true;
  }

  let pmtilesProtocolAdded = false;
  function ensurePMTilesProtocol() {
    ensureMapLibreWorker();
    if (pmtilesProtocolAdded) return;
    try {
      const protocol = new Protocol();
      addProtocol('pmtiles', protocol.tile);
      pmtilesProtocolAdded = true;
    } catch {
      pmtilesProtocolAdded = true;
    }
  }

  function syncAOIGeoJSONOnMap() {
    if (!mapInstance) return;
    const src = mapInstance.getSource('vpro-aoi-overlay') as GeoJSONSource | undefined;
    const features: any[] = [];
    if (aoiVertices.length >= 3) {
      const closed = [...aoiVertices, aoiVertices[0]];
      features.push({
        type: 'Feature',
        properties: {},
        geometry: { type: 'Polygon', coordinates: [closed] },
      });
    } else if (aoiVertices.length === 2) {
      features.push({
        type: 'Feature',
        properties: {},
        geometry: { type: 'LineString', coordinates: aoiVertices },
      });
    }
    for (const v of aoiVertices) {
      features.push({
        type: 'Feature',
        properties: {},
        geometry: { type: 'Point', coordinates: v },
      });
    }
    const fc = { type: 'FeatureCollection' as const, features };
    if (src) {
      src.setData(fc);
      return;
    }
    if (!mapInstance.isStyleLoaded()) return;
    mapInstance.addSource('vpro-aoi-overlay', { type: 'geojson', data: fc });
    mapInstance.addLayer({
      id: 'vpro-aoi-fill',
      type: 'fill',
      source: 'vpro-aoi-overlay',
      filter: ['==', '$type', 'Polygon'],
      paint: {
        'fill-color': '#FCBA19',
        'fill-opacity': 0.22,
      },
    });
    mapInstance.addLayer({
      id: 'vpro-aoi-stroke',
      type: 'line',
      source: 'vpro-aoi-overlay',
      paint: {
        'line-color': '#FCBA19',
        'line-width': 2.5,
      },
    });
    mapInstance.addLayer({
      id: 'vpro-aoi-vertices',
      type: 'circle',
      source: 'vpro-aoi-overlay',
      filter: ['==', '$type', 'Point'],
      paint: {
        'circle-radius': 5,
        'circle-color': '#003366',
        'circle-stroke-color': '#FCBA19',
        'circle-stroke-width': 2,
      },
    });
  }

  function refreshDeckOverlay() {
    if (!deckOverlay) return;
    const pts = filteredPoints;
    const matched = aoiMatchedSet;
    const selPlotNum = selectedPlot?.plotNumber ?? '';

    const layers: any[] = [];
    if (visibility.discrepancyVectors && discrepancySegments.length > 0) {
      layers.push(
        new LineLayer<DiscrepancyVectorSegment>({
          id: 'vpro-utm-dd-vectors',
          data: discrepancySegments,
          getSourcePosition: (d) => d.sourcePosition,
          getTargetPosition: (d) => d.targetPosition,
          getColor: (d) => (d.flagged ? [220, 38, 38, 220] : [245, 158, 11, 180]),
          getWidth: 2.5,
          widthUnits: 'pixels',
        }),
        new ScatterplotLayer<DiscrepancyVectorSegment>({
          id: 'vpro-utm-derived-pins',
          data: discrepancySegments,
          getPosition: (d) => d.targetPosition,
          getFillColor: [249, 115, 22, 200],
          getLineColor: [0, 51, 102, 255],
          stroked: true,
          lineWidthMinPixels: 1.5,
          getRadius: 4.5,
          radiusUnits: 'pixels',
        })
      );
    }

    layers.push(
      new ScatterplotLayer<WorkbenchPlotPoint>({
        id: 'vpro-plot-pins',
        data: pts,
        pickable: true,
        getPosition: (d) => [d.lon, d.lat],
        getRadius: (d) =>
          d.plotNumber === selPlotNum ? 9 : matched.has(d.plotNumber) ? 7.5 : 5.5,
        radiusUnits: 'pixels',
        stroked: true,
        lineWidthMinPixels: 1.5,
        getLineColor: (d) =>
          d.plotNumber === selPlotNum ? [252, 186, 25, 255] : [255, 255, 255, 235],
        getFillColor: (d) => {
          if (matched.has(d.plotNumber)) return [252, 186, 25, 245];
          if (d.divergenceFlagged || d.bgcMismatch) return [220, 38, 38, 235];
          return [5, 150, 105, 230];
        },
        updateTriggers: {
          getRadius: [selPlotNum, aoiPreview],
          getLineColor: [selPlotNum],
          getFillColor: [aoiPreview],
        },
        onClick: (info: { object?: WorkbenchPlotPoint }) => {
          if (info.object && !aoiDrawMode) {
            void inspectPlotPin(info.object);
          }
        },
      })
    );

    deckOverlay.setProps({ layers });
  }

  function applyLayerVisibilities() {
    if (!mapInstance) return;
    const style = buildMapLibreGeospatialStyle(payload?.manifest, $state.snapshot(visibility));
    mapInstance.setStyle(style as any);
    mapInstance.easeTo({
      pitch: visibility.hillshade3D ? 52 : 0,
      bearing: visibility.hillshade3D ? -14 : 0,
      duration: 450,
    });
    mapInstance.once('styledata', () => {
      syncAOIGeoJSONOnMap();
      refreshDeckOverlay();
    });
  }

  async function runViewportKernel() {
    if (plotCoordsF32.length === 0) {
      viewportMatchedCount = 0;
      return;
    }
    let bounds: [number, number, number, number] = [-140, 47, -113, 61];
    if (mapInstance) {
      const b = mapInstance.getBounds();
      bounds = [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()];
    } else if (payload?.boundingBox) {
      bounds = [
        payload.boundingBox[0],
        payload.boundingBox[1],
        payload.boundingBox[2],
        payload.boundingBox[3],
      ];
    }
    const res = await executeSpatialFilterKernel(plotCoordsF32, bounds, { gridSize: 16 });
    if (!alive) return;
    viewportMatchedCount = res.matchedCount;
  }

  function initializeMapCanvas() {
    if (!mapContainer || mapInstance) return;
    ensurePMTilesProtocol();
    try {
      setWorkerUrl(maplibreWorkerUrl);
      const style = buildMapLibreGeospatialStyle(payload?.manifest, $state.snapshot(visibility));
      const map = new MapLibreMap({
        container: mapContainer,
        style: style as any,
        center: [DEFAULT_BC_VIEW_STATE.longitude, DEFAULT_BC_VIEW_STATE.latitude],
        zoom: DEFAULT_BC_VIEW_STATE.zoom,
        pitch: 0,
        bearing: 0,
      });
      map.addControl(new NavigationControl({ visualizePitch: true }), 'top-right');
      map.addControl(new ScaleControl({ maxWidth: 140, unit: 'metric' }), 'bottom-left');

      deckOverlay = new MapboxOverlay({ interleaved: false, layers: [] });
      map.addControl(deckOverlay as unknown as IControl);

      map.on('load', () => {
        if (!alive) return;
        syncAOIGeoJSONOnMap();
        refreshDeckOverlay();
        fitMapToPlots();
        void runViewportKernel();
      });

      map.on('moveend', () => {
        if (!alive) return;
        void runViewportKernel();
      });

      map.on('click', (evt) => {
        if (!alive || !aoiDrawMode) return;
        const nextPt: [number, number] = [
          Math.round(evt.lngLat.lng * 1e6) / 1e6,
          Math.round(evt.lngLat.lat * 1e6) / 1e6,
        ];
        aoiVertices = [...aoiVertices, nextPt];
        syncAOIGeoJSONOnMap();
        if (aoiVertices.length >= 3) {
          void previewAOIPolygon();
        }
      });

      mapInstance = map;
    } catch (cause) {
      mapRenderWarning = `Hardware WebGL2 map viewport is unavailable in this environment (${cause instanceof Error ? cause.message : String(cause)}). Tabular Spatial QA, Auto-1 Attribution, FlatGeobuf stream, and Polygon AOI -> SU workflows remain active.`;
    }
  }

  function fitMapToPlots() {
    if (!mapInstance || !payload || payload.plottableCount === 0) return;
    const [minLon, minLat, maxLon, maxLat] = payload.boundingBox;
    if (!Number.isFinite(minLon) || !Number.isFinite(maxLon)) return;
    if (Math.abs(maxLon - minLon) < 0.005 && Math.abs(maxLat - minLat) < 0.005) {
      mapInstance.jumpTo({ center: [minLon, minLat], zoom: 9.5 });
      return;
    }
    mapInstance.fitBounds(
      [
        [minLon, minLat],
        [maxLon, maxLat],
      ],
      { padding: 48, maxZoom: 10, duration: 500 }
    );
  }

  async function loadWorkbenchData() {
    if (!contextId) return;
    const gen = ++loadGeneration;
    loading = true;
    errorMsg = '';
    setBusyState(true);
    try {
      const [wb, fgbRaw] = await Promise.all([
        SpatialService.GetSpatialWorkbenchPayload(contextId),
        SpatialService.GetSpatialPlotsFlatGeobuf(contextId),
      ]);
      if (!alive || gen !== loadGeneration) return;
      payload = wb;
      const mat = materializeWorkbenchPlots(wb?.geoArrow);
      plotPoints = mat.points;
      plotCoordsF32 = mat.coordsFloat32;
      discrepancySegments = mat.discrepancyVectors;

      const decodedFgb = decodeFlatGeobufPoints(fgbRaw);
      flatGeobufCount = decodedFgb.length;

      if (su && su !== 'None') {
        aoiTargetMode = 'update_active';
      } else {
        aoiTargetMode = 'create_project_su';
        if (!aoiSUName) aoiSUName = `${project}_AOI`;
      }

      if (mapInstance) {
        applyLayerVisibilities();
        fitMapToPlots();
      } else {
        initializeMapCanvas();
      }
      refreshDeckOverlay();
      await runViewportKernel();

      if (plotPoints.length > 0 && !selectedPlot) {
        void inspectPlotPin(plotPoints[0], false);
      }
    } catch (cause) {
      if (alive && gen === loadGeneration) {
        errorMsg = `Failed to load spatial workbench: ${cause instanceof Error ? cause.message : String(cause)}`;
      }
    } finally {
      if (alive && gen === loadGeneration) {
        loading = false;
        setBusyState(mutating);
      }
    }
  }

  async function inspectPlotPin(pt: WorkbenchPlotPoint, fly = true) {
    selectedPlot = pt;
    selectedSuggestion = null;
    activePanel = 'inspector';
    refreshDeckOverlay();
    if (fly && mapInstance) {
      mapInstance.easeTo({ center: [pt.lon, pt.lat], zoom: Math.max(mapInstance.getZoom(), 8.5), duration: 400 });
    }
    const gen = ++inspectorGeneration;
    suggestionBusy = true;
    try {
      const sug = await SpatialService.SuggestSpatialAttributes(pt.lat, pt.lon);
      if (!alive || gen !== inspectorGeneration) return;
      selectedSuggestion = sug;
    } catch {
      if (alive && gen === inspectorGeneration) selectedSuggestion = null;
    } finally {
      if (alive && gen === inspectorGeneration) suggestionBusy = false;
    }
  }

  let aoiPreviewBusy = $state(false);

  async function previewAOIPolygon() {
    aoiPreview = null;
    if (!contextId || aoiVertices.length < 3) {
      aoiPreviewBusy = false;
      refreshDeckOverlay();
      return;
    }
    const gen = ++aoiGeneration;
    aoiPreviewBusy = true;
    const ringSnapshot = $state.snapshot(aoiVertices);
    // Immediate client-side feedback using pointInPolygonRing while Go verifies against authoritative SQLite
    const localMatches = plotPoints.filter((p) => pointInPolygonRing(p.lon, p.lat, ringSnapshot));
    try {
      const req = {
        polygon: ringSnapshot,
        rings: [] as number[][][],
        zone: zoneFilter.trim(),
      };
      const res = await SpatialService.SelectPlotsInPolygonAOI(contextId, req);
      if (!alive || gen !== aoiGeneration) return;
      aoiPreview = res;
      if (!aoiSiteUnit && localMatches[0]?.gisBgc) {
        aoiSiteUnit = `${localMatches[0].gisBgc}/01`;
      }
      refreshDeckOverlay();
    } catch (cause) {
      if (alive && gen !== aoiGeneration) return;
      if (alive) {
        errorMsg = `AOI polygon preview failed: ${cause instanceof Error ? cause.message : String(cause)}`;
      }
    } finally {
      if (alive && gen === aoiGeneration) aoiPreviewBusy = false;
    }
  }

  function seedAOIFromCurrentViewport() {
    let minLon = -124.5;
    let minLat = 53.2;
    let maxLon = -121.5;
    let maxLat = 54.6;
    if (mapInstance) {
      const b = mapInstance.getBounds();
      const padLon = (b.getEast() - b.getWest()) * 0.15;
      const padLat = (b.getNorth() - b.getSouth()) * 0.15;
      minLon = Math.round((b.getWest() + padLon) * 1e5) / 1e5;
      maxLon = Math.round((b.getEast() - padLon) * 1e5) / 1e5;
      minLat = Math.round((b.getSouth() + padLat) * 1e5) / 1e5;
      maxLat = Math.round((b.getNorth() - padLat) * 1e5) / 1e5;
    } else if (payload?.boundingBox) {
      [minLon, minLat, maxLon, maxLat] = payload.boundingBox;
    }
    aoiVertices = [
      [minLon, minLat],
      [maxLon, minLat],
      [maxLon, maxLat],
      [minLon, maxLat],
    ];
    syncAOIGeoJSONOnMap();
    void previewAOIPolygon();
  }

  function clearAOIPolygon() {
    aoiGeneration++;
    aoiVertices = [];
    aoiPreview = null;
    syncAOIGeoJSONOnMap();
    refreshDeckOverlay();
  }

  async function commitAOIToSU() {
    if (!contextId || mutating || loading || aoiPreviewBusy || !aoiPreview || aoiPreview.matchedCount === 0) return;
    if (!aoiSiteUnit.trim()) {
      errorMsg = 'Enter a target SiteUnit code (for example SBSmk1/01) before saving the AOI selection.';
      return;
    }
    mutating = true;
    errorMsg = '';
    statusMsg = '';
    setBusyState(true);
    try {
      const req: AssignSpatialAOIToSURequest = $state.snapshot({
        mode: aoiTargetMode,
        suName: aoiSUName.trim(),
        filePath: aoiFilePath.trim(),
        siteUnit: aoiSiteUnit.trim(),
        plotNumbers: aoiPreview.plotNumbers ?? [],
        polygon: aoiVertices,
        rings: [],
        includeNonAoiPlotsAsUnassigned: aoiIncludeUnassigned,
        activate: aoiActivate,
      });
      const res = await SpatialService.AssignSpatialAOIToSU(contextId, req);
      if (!alive || !res) return;
      statusMsg = `Committed ${res.matchedPlots} AOI plots to ${res.tableName} as ${res.siteUnit} (${res.changedRows} row writes).`;
      if (res.activated) {
        await onContextChanged?.();
      } else {
        await loadWorkbenchData();
      }
    } catch (cause) {
      if (alive) {
        errorMsg = `AOI SU assignment failed: ${cause instanceof Error ? cause.message : String(cause)}`;
      }
    } finally {
      if (alive) {
        mutating = false;
        setBusyState(loading);
      }
    }
  }

  async function loadBatchAuditQA() {
    if (!contextId) return;
    const gen = ++qaGeneration;
    loading = true;
    errorMsg = '';
    setBusyState(true);
    try {
      const report = await SpatialService.BatchAuditSpatialQA(contextId);
      if (!alive || gen !== qaGeneration) return;
      qaReport = report;
      const nextSelected: Record<string, boolean> = {};
      for (const r of report?.rows ?? []) {
        if (
          r.bgcMismatch ||
          r.divergenceFlagged ||
          r.longitudeSignFlipped ||
          (!r.storedGisBgc && !!r.suggestedGisBgc)
        ) {
          nextSelected[r.plotNumber] = true;
        }
      }
      qaSelectedPlots = nextSelected;
    } catch (cause) {
      if (alive && gen === qaGeneration) {
        errorMsg = `Spatial QA batch audit failed: ${cause instanceof Error ? cause.message : String(cause)}`;
      }
    } finally {
      if (alive && gen === qaGeneration) {
        loading = false;
        setBusyState(mutating);
      }
    }
  }

  async function applyAuto1SpatialStamp(allScoped: boolean) {
    if (!contextId || mutating || loading) return;
    const chosen = Object.entries(qaSelectedPlots)
      .filter(([, checked]) => checked)
      .map(([plotNum]) => plotNum);
    if (!allScoped && chosen.length === 0) {
      errorMsg = 'Select at least one plot in the Spatial QA table or run across all scoped plots.';
      return;
    }
    mutating = true;
    errorMsg = '';
    statusMsg = '';
    setBusyState(true);
    try {
      const req: ApplySpatialAttributesRequest = $state.snapshot({
        plotNumbers: allScoped ? [] : chosen,
        allScopedPlots: allScoped,
        stampGisBgc: stampGISBGC,
        stampAdminFields: stampAdminFields,
        onlyWhenBlank: stampOnlyWhenBlank,
        normalizeLongitudeSign: normalizeLonSign,
        syncUtmFromLatLon: syncUTMFromDD,
        syncLatLonFromUtm: false,
      });
      const res = await SpatialService.ApplySpatialAttributes(contextId, req);
      if (!alive || !res) return;
      statusMsg = `Stamped ${res.updatedPlots} plot(s) (${res.updatedCells} cell updates, ${res.auditRows} <Project>_Audit entries).`;
      await loadWorkbenchData();
      await loadBatchAuditQA();
    } catch (cause) {
      if (alive) {
        errorMsg = `Auto-1 spatial stamp failed: ${cause instanceof Error ? cause.message : String(cause)}`;
      }
    } finally {
      if (alive) {
        mutating = false;
        setBusyState(loading);
      }
    }
  }

  $effect(() => {
    void zoneFilter;
    void plotSearch;
    refreshDeckOverlay();
  });

  onMount(() => {
    void detectSpatialComputeBackend().then((info) => {
      if (alive) computeInfo = info;
    });
    initializeMapCanvas();
    void loadWorkbenchData();
  });

  onDestroy(() => {
    alive = false;
    loadGeneration++;
    qaGeneration++;
    aoiGeneration++;
    inspectorGeneration++;
    setBusyState(false);
    if (deckOverlay && mapInstance) {
      try {
        mapInstance.removeControl(deckOverlay as unknown as IControl);
      } catch {
        // Ignore teardown control removal errors
      }
    }
    deckOverlay = null;
    if (mapInstance) {
      mapInstance.remove();
      mapInstance = null;
    }
  });
</script>

<section class="geospatial-workbench" aria-label="3D Geospatial Workbench">
  <header class="wb-banner">
    <div class="wb-title-block">
      <span class="wb-eyebrow">BC MINISTRY OF FORESTS - BEC GEOSPATIAL WORKBENCH</span>
      <h1>3D BGC v13 Map, Geodetic QA &amp; Spatial AOI Classification</h1>
    </div>
    <div class="wb-badges">
      <span class="wb-badge wb-badge-gold" data-compute-backend={computeInfo.backend} title={computeInfo.fallbackReason ?? computeInfo.adapterLabel}>
        {computeInfo.backend === 'webgpu' ? 'WebGPU WGSL Compute' : 'WebGL2 + TypedArray Fallback'}
      </span>
      <span class="wb-badge">
        {summarizeArchiveResolution(payload)}
      </span>
      <span class="wb-badge">
        Plots: {payload?.plottableCount ?? 0} / {payload?.totalPlots ?? 0} (FGB: {flatGeobufCount})
      </span>
      <span class="wb-badge" class:wb-badge-alert={(payload?.bgcMismatchCount ?? 0) > 0 || (payload?.utmDivergenceCount ?? 0) > 0}>
        BGC Mismatch: {payload?.bgcMismatchCount ?? 0} | UTM Div: {payload?.utmDivergenceCount ?? 0}
      </span>
    </div>
  </header>

  <div class="wb-toolbar" role="toolbar" aria-label="Map layers and spatial tools">
    <div class="wb-layer-toggles">
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.bgcFill} onchange={applyLayerVisibilities} />
        <span>BGC v13 Polygons</span>
      </label>
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.ecosections} onchange={applyLayerVisibilities} />
        <span>Ecosections</span>
      </label>
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.nrDistricts} onchange={applyLayerVisibilities} />
        <span>NR Districts</span>
      </label>
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.discrepancyVectors} onchange={refreshDeckOverlay} />
        <span>Dual-Pin UTM Vectors ({discrepancySegments.length})</span>
      </label>
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.hillshade3D} onchange={applyLayerVisibilities} />
        <span>3D Reconnaissance Tilt</span>
      </label>
      <label class="wb-toggle">
        <input type="checkbox" bind:checked={visibility.bcOpenMapsWMS} onchange={applyLayerVisibilities} />
        <span>Online BC OpenMaps WMS</span>
      </label>
    </div>

    <div class="wb-filter-controls">
      <label for="wb-zone-filter" class="sr-only">Filter by BEC Zone</label>
      <select id="wb-zone-filter" bind:value={zoneFilter} class="wb-input">
        <option value="">All BEC Zones</option>
        {#each distinctZones as z (z)}
          <option value={z}>{z}</option>
        {/each}
      </select>

      <label for="wb-plot-search" class="sr-only">Filter plots by PlotNumber or SiteUnit</label>
      <input
        id="wb-plot-search"
        type="search"
        placeholder="Find plot or unit..."
        bind:value={plotSearch}
        class="wb-input"
      />

      <button type="button" class="wb-btn" onclick={fitMapToPlots} disabled={loading || plotPoints.length === 0}>
        Fit Project Extent
      </button>
      <button type="button" class="wb-btn" onclick={() => void loadWorkbenchData()} disabled={loading || mutating}>
        Refresh Stream
      </button>
    </div>
  </div>

  {#if errorMsg}
    <div role="alert" class="wb-alert wb-alert-error">{errorMsg}</div>
  {/if}
  {#if statusMsg}
    <div role="status" class="wb-alert wb-alert-ok">{statusMsg}</div>
  {/if}
  {#if mapRenderWarning}
    <div role="status" class="wb-alert wb-alert-info">{mapRenderWarning}</div>
  {/if}

  <div class="wb-main-split">
    <div class="wb-map-stage">
      <div
        bind:this={mapContainer}
        class="wb-map-canvas"
        class:wb-crosshair={aoiDrawMode}
        data-testid="maplibre-geospatial-canvas"
      ></div>
      <div class="wb-map-legend" aria-label="BGC Zone and Pin Legend">
        <div class="wb-legend-row">
          <span class="wb-dot wb-dot-ok"></span><span>Verified Plot</span>
          <span class="wb-dot wb-dot-err"></span><span>BGC / UTM Discrepancy</span>
          <span class="wb-dot wb-dot-aoi"></span><span>AOI Selected ({aoiPreview?.matchedCount ?? 0})</span>
          <span class="wb-viewport-stat">In Viewport: {viewportMatchedCount}</span>
        </div>
        <div class="wb-zone-swatches">
          {#each Object.entries(BEC_ZONE_PALETTE).slice(0, 10) as [zCode, zHex] (zCode)}
            <span class="wb-swatch"><i style={`background:${zHex}`}></i>{zCode}</span>
          {/each}
        </div>
      </div>
    </div>

    <aside class="wb-drawer" aria-label="Spatial Inspector, AOI and QA Tools">
      <div class="wb-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={activePanel === 'inspector'}
          class:active={activePanel === 'inspector'}
          onclick={() => { aoiDrawMode = false; activePanel = 'inspector'; }}
        >
          Plot Inspector
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activePanel === 'aoi'}
          class:active={activePanel === 'aoi'}
          onclick={() => { activePanel = 'aoi'; }}
        >
          Lasso / AOI -&gt; SU
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activePanel === 'qa'}
          class:active={activePanel === 'qa'}
          onclick={() => {
            aoiDrawMode = false;
            activePanel = 'qa';
            if (!qaReport) void loadBatchAuditQA();
          }}
        >
          Auto-1 QA Stamp
        </button>
      </div>

      {#if activePanel === 'inspector'}
        <div class="wb-panel-body">
          {#if selectedPlot}
            <div class="wb-card">
              <div class="wb-card-head">
                <div>
                  <span class="wb-label">PLOT PIN</span>
                  <h2>{selectedPlot.plotNumber}</h2>
                </div>
                {#if onOpenPlot}
                  <button
                    type="button"
                    class="wb-btn wb-btn-primary"
                    onclick={() => onOpenPlot?.(selectedPlot!.plotNumber)}
                  >
                    Open in FS882 Editor
                  </button>
                {/if}
              </div>

              <dl class="wb-kv-grid">
                <div><dt>NAD83 Lat, Lon</dt><dd>{selectedPlot.lat.toFixed(5)}, {selectedPlot.lon.toFixed(5)}</dd></div>
                <div><dt>Active SiteUnit</dt><dd>{selectedPlot.siteUnit || 'Unassigned'}</dd></div>
                <div><dt>Field BGC</dt><dd>{selectedPlot.fieldBgc || 'Blank'}</dd></div>
                <div>
                  <dt>GIS_BGC (v13)</dt>
                  <dd class:wb-flag-text={selectedPlot.bgcMismatch}>
                    {selectedPlot.gisBgc || selectedSuggestion?.gisBgc || 'Pending'}
                    {#if selectedPlot.bgcMismatch} (Mismatch){/if}
                  </dd>
                </div>
                <div>
                  <dt>UTM vs DD Delta</dt>
                  <dd class:wb-flag-text={selectedPlot.divergenceFlagged}>
                    {selectedPlot.hasUtm ? `${selectedPlot.divergenceMeters.toFixed(1)} m` : 'No UTM entered'}
                  </dd>
                </div>
                <div><dt>Elevation / Slope</dt><dd>{selectedPlot.elevation ?? '-'} m / {selectedPlot.slope ?? '-'}%</dd></div>
                <div><dt>Ecosection</dt><dd>{selectedPlot.ecosection || selectedSuggestion?.ecosection || '-'}</dd></div>
                <div><dt>NR District</dt><dd>{selectedPlot.fsRegionDistrict || selectedSuggestion?.fsRegionDistrict || '-'}</dd></div>
              </dl>

              {#if suggestionBusy}
                <p class="wb-muted" role="status">Querying local PMTiles v3 Hilbert MVT index...</p>
              {:else if selectedSuggestion}
                <div class="wb-suggestion-box">
                  <strong>Auto-1 PMTiles Attribution ({selectedSuggestion.bgcSource || 'overview'} z{selectedSuggestion.bgcZoom}):</strong>
                  <div>BGC: {selectedSuggestion.gisBgc} ({selectedSuggestion.zoneName || 'BC'})</div>
                  <div>NAD83 UTM Zone {selectedSuggestion.utmZone}: {selectedSuggestion.utmEasting.toFixed(1)} E, {selectedSuggestion.utmNorthing.toFixed(1)} N</div>
                  <div>BCGS 1:20k Sheet: {selectedSuggestion.ntsMapSheet} | District: {selectedSuggestion.fsRegionDistrict}</div>
                  {#if selectedSuggestion.terrainSanityNote}
                    <div class="wb-muted">{selectedSuggestion.terrainSanityNote}</div>
                  {/if}
                </div>
              {/if}
            </div>
          {:else}
            <p class="wb-muted">Click any plot pin on the map or choose from the list below to inspect coordinates and BGC v13 overlay.</p>
          {/if}

          <div class="wb-list-header">Scoped Plots ({filteredPoints.length})</div>
          <div class="wb-plot-list">
            {#each filteredPoints.slice(0, 120) as pt (pt.plotNumber)}
              <button
                type="button"
                class="wb-plot-row"
                class:selected={selectedPlot?.plotNumber === pt.plotNumber}
                onclick={() => void inspectPlotPin(pt)}
              >
                <span class="mono font-semibold">{pt.plotNumber}</span>
                <span>{pt.fieldBgc || '-'} -&gt; {pt.gisBgc || '-'}</span>
                {#if pt.bgcMismatch || pt.divergenceFlagged}
                  <span class="wb-pill-err">QA</span>
                {/if}
              </button>
            {/each}
          </div>
        </div>

      {:else if activePanel === 'aoi'}
        <div class="wb-panel-body">
          <div class="wb-card">
            <h2>Polygon / Lasso AOI -&gt; Site Unit (*_SU)</h2>
            <p class="wb-muted">Click vertices directly on the MapLibre canvas or capture the current viewport extent to select plots spatially.</p>
            <div class="wb-btn-row">
              <button
                type="button"
                class="wb-btn"
                class:wb-btn-gold={aoiDrawMode}
                onclick={() => { aoiDrawMode = !aoiDrawMode; }}
              >
                {aoiDrawMode ? 'Finish Drawing AOI' : 'Draw Polygon on Map'}
              </button>
              <button type="button" class="wb-btn" onclick={seedAOIFromCurrentViewport}>
                Use Viewport BBOX
              </button>
              <button type="button" class="wb-btn" onclick={clearAOIPolygon} disabled={aoiVertices.length === 0}>
                Clear ({aoiVertices.length} pts)
              </button>
            </div>

            {#if aoiPreview}
              {@const previewNums = aoiPreview.plotNumbers ?? []}
              <div class="wb-suggestion-box" role="status">
                <strong>AOI Selection: {aoiPreview.matchedCount} of {aoiPreview.totalEvaluated} plots</strong>
                <div>Approx. Area: {aoiPreview.approxAreaKm2.toFixed(1)} km^2</div>
                <div class="wb-chip-wrap">
                  {#each previewNums.slice(0, 20) as pNum (pNum)}
                    <span class="wb-chip">{pNum}</span>
                  {/each}
                  {#if previewNums.length > 20}
                    <span class="wb-muted">+{previewNums.length - 20} more</span>
                  {/if}
                </div>
              </div>
            {/if}

            <div class="wb-form-stack">
              <label for="aoi-target-mode">Target *_SU Destination</label>
              <select id="aoi-target-mode" bind:value={aoiTargetMode} class="wb-input">
                <option value="update_active" disabled={!su || su === 'None'}>
                  Update Active SU ({su && su !== 'None' ? `${su}_SU` : 'None selected'})
                </option>
                <option value="create_project_su">Create New *_SU Table in Project ({project}.db)</option>
                <option value="create_su_file">Create New Standalone SQLite SU File (.db)</option>
              </select>

              {#if aoiTargetMode !== 'update_active'}
                <label for="aoi-su-name">New SU Table Prefix</label>
                <input id="aoi-su-name" class="wb-input" bind:value={aoiSUName} placeholder="e.g. SBS_NorthAOI" />
              {/if}

              {#if aoiTargetMode === 'create_su_file'}
                <label for="aoi-su-filepath">New SQLite File Path (.db)</label>
                <input id="aoi-su-filepath" class="wb-input" bind:value={aoiFilePath} placeholder="/path/to/NewAOI_SU.db" />
              {/if}

              <label for="aoi-site-unit">Assigned SiteUnit Code</label>
              <input id="aoi-site-unit" class="wb-input" bind:value={aoiSiteUnit} placeholder="e.g. SBSmk1/01" />

              {#if aoiTargetMode !== 'update_active'}
                <label class="wb-check-row">
                  <input type="checkbox" bind:checked={aoiIncludeUnassigned} />
                  <span>Include non-AOI project plots as unassigned (NULL)</span>
                </label>
                <label class="wb-check-row">
                  <input type="checkbox" bind:checked={aoiActivate} />
                  <span>Switch active context to new *_SU immediately</span>
                </label>
              {/if}

              <button
                type="button"
                class="wb-btn wb-btn-primary"
                disabled={mutating || loading || !aoiPreview || aoiPreview.matchedCount === 0 || !aoiSiteUnit.trim()}
                onclick={() => void commitAOIToSU()}
              >
                {mutating ? 'Saving *_SU...' : `Assign ${aoiPreview?.matchedCount ?? 0} AOI Plots to *_SU`}
              </button>
            </div>
          </div>
        </div>

      {:else if activePanel === 'qa'}
        <div class="wb-panel-body">
          <div class="wb-card">
            <div class="wb-card-head">
              <h2>Auto-1 Batch Geodetic &amp; BGC Stamp</h2>
              <button type="button" class="wb-btn" onclick={() => void loadBatchAuditQA()} disabled={loading || mutating}>
                Re-Audit
              </button>
            </div>

            {#if qaReport}
              <div class="wb-qa-summary">
                <span>Unsigned West Lon: <strong>{qaReport.unsignedLonCount}</strong></span>
                <span>UTM &gt;50m Div: <strong>{qaReport.utmDivergenceCount}</strong></span>
                <span>BGC Mismatch: <strong>{qaReport.bgcMismatchCount}</strong></span>
                <span>Missing GIS_BGC: <strong>{qaReport.missingGisBgcCount}</strong></span>
              </div>
            {/if}

            <div class="wb-form-stack">
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={stampGISBGC} />
                <span>Stamp &lt;Project&gt;_Admin.GIS_BGC &amp; GIS_BGC_VER = 13</span>
              </label>
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={stampAdminFields} />
                <span>Stamp Ecosection, FSRegionDistrict, NtsMapSheet &amp; Province</span>
              </label>
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={stampOnlyWhenBlank} />
                <span>Preserve existing non-blank field entries (only fill blanks)</span>
              </label>
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={normalizeLonSign} />
                <span>Normalize unsigned positive West longitudes (125.4 -&gt; -125.4)</span>
              </label>
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={syncUTMFromDD} />
                <span>Project NAD83 Lat/Lon -&gt; UTMZone / Easting / Northing</span>
              </label>
              <label class="wb-check-row">
                <input type="checkbox" bind:checked={qaFilterOnlyIssues} />
                <span>Show only plots with QA discrepancies or missing GIS_BGC</span>
              </label>
            </div>

            <div class="wb-btn-row">
              <button
                type="button"
                class="wb-btn wb-btn-primary"
                disabled={mutating || loading}
                onclick={() => void applyAuto1SpatialStamp(false)}
              >
                Stamp Checked Plots + Audit
              </button>
              <button
                type="button"
                class="wb-btn"
                disabled={mutating || loading}
                onclick={() => void applyAuto1SpatialStamp(true)}
              >
                Stamp All Scoped Plots
              </button>
            </div>
          </div>

          <div class="wb-qa-table-wrap">
            <table class="wb-qa-table">
              <thead>
                <tr>
                  <th scope="col">Sel</th>
                  <th scope="col">Plot</th>
                  <th scope="col">Field-&gt;GIS</th>
                  <th scope="col">UTM Delta</th>
                </tr>
              </thead>
              <tbody>
                {#each filteredQARows.slice(0, 150) as row (row.plotNumber)}
                  <tr>
                    <td>
                      <input
                        type="checkbox"
                        aria-label={`Select plot ${row.plotNumber} for spatial stamp`}
                        checked={!!qaSelectedPlots[row.plotNumber]}
                        onchange={(e) => {
                          qaSelectedPlots = {
                            ...qaSelectedPlots,
                            [row.plotNumber]: (e.currentTarget as HTMLInputElement).checked,
                          };
                        }}
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        class="wb-link-btn mono"
                        onclick={() => onOpenPlot?.(row.plotNumber)}
                      >
                        {row.plotNumber}
                      </button>
                    </td>
                    <td class:wb-flag-text={row.bgcMismatch}>
                      {row.fieldBgc || '-'} -&gt; {row.suggestedGisBgc || row.storedGisBgc || '-'}
                    </td>
                    <td class:wb-flag-text={row.divergenceFlagged || row.longitudeSignFlipped}>
                      {#if row.longitudeSignFlipped}+W {/if}
                      {row.divergenceMeters != null ? `${row.divergenceMeters.toFixed(0)}m` : '-'}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
    </aside>
  </div>
</section>

<style>
  .geospatial-workbench {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 640px;
    background: #f4f6f9;
    color: #1f2937;
  }
  .wb-banner {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    background: #003366;
    border-bottom: 3px solid #fcba19;
    color: #ffffff;
  }
  .wb-eyebrow {
    display: block;
    font-size: 0.68rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: #fcba19;
  }
  .wb-title-block h1 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
  }
  .wb-badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
  }
  .wb-badge {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.2rem 0.55rem;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.14);
    border: 1px solid rgba(255, 255, 255, 0.25);
  }
  .wb-badge-gold {
    background: #fcba19;
    color: #003366;
    border-color: #fcba19;
    font-weight: 700;
  }
  .wb-badge-alert {
    background: #991b1b;
    border-color: #fca5a5;
  }
  .wb-toolbar {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.65rem;
    padding: 0.5rem 0.85rem;
    background: #ffffff;
    border-bottom: 1px solid #cbd5e1;
    font-size: 0.78rem;
  }
  .wb-layer-toggles,
  .wb-filter-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.65rem;
  }
  .wb-toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    cursor: pointer;
    font-weight: 500;
  }
  .wb-input {
    border: 1px solid #94a3b8;
    border-radius: 4px;
    padding: 0.3rem 0.5rem;
    font-size: 0.78rem;
    background: #ffffff;
  }
  .wb-btn {
    border: 1px solid #003366;
    border-radius: 4px;
    padding: 0.32rem 0.65rem;
    font-size: 0.78rem;
    font-weight: 600;
    background: #ffffff;
    color: #003366;
    cursor: pointer;
  }
  .wb-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .wb-btn-primary {
    background: #003366;
    color: #ffffff;
  }
  .wb-btn-gold {
    background: #fcba19;
    border-color: #b45309;
    color: #003366;
  }
  .wb-alert {
    padding: 0.45rem 0.9rem;
    font-size: 0.8rem;
    border-bottom: 1px solid #cbd5e1;
  }
  .wb-alert-error {
    background: #fef2f2;
    color: #991b1b;
  }
  .wb-alert-ok {
    background: #ecfdf5;
    color: #065f46;
  }
  .wb-alert-info {
    background: #eff6ff;
    color: #1e3a8a;
  }
  .wb-main-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 370px;
    flex: 1;
    min-height: 520px;
  }
  @media (max-width: 960px) {
    .wb-main-split {
      grid-template-columns: 1fr;
    }
  }
  .wb-map-stage {
    display: flex;
    flex-direction: column;
    min-height: 460px;
    background: #cbd5e1;
  }
  .wb-map-canvas {
    flex: 1;
    width: 100%;
    min-height: 430px;
  }
  .wb-crosshair {
    cursor: crosshair;
  }
  .wb-map-legend {
    padding: 0.4rem 0.75rem;
    background: #0f172a;
    color: #f8fafc;
    font-size: 0.72rem;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .wb-legend-row,
  .wb-zone-swatches {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.55rem;
  }
  .wb-viewport-stat {
    margin-left: auto;
    color: #fcba19;
    font-weight: 600;
  }
  .wb-dot {
    display: inline-block;
    width: 9px;
    height: 9px;
    border-radius: 999px;
  }
  .wb-dot-ok {
    background: #059669;
  }
  .wb-dot-err {
    background: #dc2626;
  }
  .wb-dot-aoi {
    background: #fcba19;
  }
  .wb-swatch {
    display: inline-flex;
    align-items: center;
    gap: 0.22rem;
  }
  .wb-swatch i {
    display: inline-block;
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
  .wb-drawer {
    display: flex;
    flex-direction: column;
    background: #ffffff;
    border-left: 1px solid #cbd5e1;
    overflow: hidden;
  }
  .wb-tabs {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    background: #e2e8f0;
    border-bottom: 2px solid #003366;
  }
  .wb-tabs button {
    padding: 0.5rem 0.35rem;
    font-size: 0.75rem;
    font-weight: 700;
    color: #334155;
    background: transparent;
    border: none;
    cursor: pointer;
  }
  .wb-tabs button.active {
    background: #003366;
    color: #fcba19;
  }
  .wb-panel-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    font-size: 0.8rem;
  }
  .wb-card {
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 0.7rem;
    background: #f8fafc;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .wb-card h2 {
    margin: 0;
    font-size: 0.92rem;
    font-weight: 700;
    color: #003366;
  }
  .wb-card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .wb-label {
    font-size: 0.65rem;
    font-weight: 700;
    color: #64748b;
  }
  .wb-kv-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.4rem;
    margin: 0;
  }
  .wb-kv-grid dt {
    font-size: 0.68rem;
    color: #64748b;
  }
  .wb-kv-grid dd {
    margin: 0;
    font-weight: 600;
    word-break: break-word;
  }
  .wb-flag-text {
    color: #b91c1c;
    font-weight: 700;
  }
  .wb-suggestion-box {
    background: #eff6ff;
    border-left: 3px solid #003366;
    padding: 0.45rem 0.55rem;
    border-radius: 3px;
    font-size: 0.74rem;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .wb-muted {
    color: #64748b;
    font-size: 0.74rem;
    margin: 0;
  }
  .wb-list-header {
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    color: #475569;
  }
  .wb-plot-list {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    max-height: 250px;
    overflow-y: auto;
  }
  .wb-plot-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.3rem 0.5rem;
    border: 1px solid #e2e8f0;
    border-radius: 4px;
    background: #ffffff;
    text-align: left;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .wb-plot-row.selected {
    border-color: #003366;
    background: #eff6ff;
  }
  .wb-pill-err {
    background: #fee2e2;
    color: #991b1b;
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
  }
  .wb-btn-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .wb-form-stack {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .wb-check-row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.75rem;
  }
  .wb-chip-wrap {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-top: 0.2rem;
  }
  .wb-chip {
    background: #003366;
    color: #fcba19;
    padding: 0.08rem 0.35rem;
    border-radius: 3px;
    font-size: 0.68rem;
    font-family: monospace;
  }
  .wb-qa-summary {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.3rem;
    font-size: 0.73rem;
    background: #ffffff;
    padding: 0.4rem;
    border: 1px solid #e2e8f0;
    border-radius: 4px;
  }
  .wb-qa-table-wrap {
    max-height: 260px;
    overflow-y: auto;
    border: 1px solid #e2e8f0;
    border-radius: 4px;
  }
  .wb-qa-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.73rem;
  }
  .wb-qa-table th,
  .wb-qa-table td {
    padding: 0.3rem 0.4rem;
    border-bottom: 1px solid #e2e8f0;
    text-align: left;
  }
  .wb-qa-table th {
    background: #f1f5f9;
    color: #003366;
    position: sticky;
    top: 0;
  }
  .wb-link-btn {
    background: none;
    border: none;
    padding: 0;
    color: #003366;
    text-decoration: underline;
    cursor: pointer;
    font-weight: 700;
  }
  .mono {
    font-family: "IBM Plex Mono", monospace;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
