<script lang="ts">
  import { BookOpen, ChartColumn, ChevronDown, ClipboardList, Compass, FileText, Globe, HelpCircle, History, House, Info, Keyboard, Leaf, Link, List, Network, Newspaper, Palette, RotateCcw, Table, TableProperties, Tag, Tent, UserCog } from '@lucide/svelte';

  type View = 'home' | 'plots' | 'geospatial-map' | 'diagnostic-synthesis' | 'r-sidecar' | 'hierarchy' | 'fs882' | 'datasheet' | 'classification-explorer' | 'project-admin' | 'lump-editor' | 'interchange-hub' | 'long-environment' | 'summary-environment' | 'long-vegetation' | 'summary-vegetation' | 'lifeform-summary' | 'subzone-matrix' | 'table-csv' | 'user-log' | 'plot-locations' | 'google-earth-review' | 'picture-manager' | 'plot-label-preview';
  type HelpMenuAction = 'guide' | 'tours' | 'shortcuts' | 'whats-new' | 'reset-sample' | 'about';
  type Item = { name: string; icon: typeof House; view?: View; action?: 'sivi' | 'new-sivi' | 'metadata' | 'cloud-sync'; helpAction?: HelpMenuAction; deferredPhase2?: boolean };
  type Menu = { label: string; icon: typeof House; groups: { label: string; items: Item[] }[] };
  let { view, onnavigate, siviAvailable = false, onopensivi, newSiviAvailable = false, onopennewsivi,
    metadataAvailable = false, onopenmetadata, onopencloudsync, onhelpaction, whatsNewUnreadCount = 0, resetSampleAvailable = true,
    releaseChannel = 'stable' }: {
    view: View; onnavigate: (view: View) => void;
    siviAvailable?: boolean; onopensivi?: () => void;
    newSiviAvailable?: boolean; onopennewsivi?: () => void;
    metadataAvailable?: boolean; onopenmetadata?: () => void;
    onopencloudsync?: () => void;
    onhelpaction?: (action: HelpMenuAction) => void;
    whatsNewUnreadCount?: number;
    resetSampleAvailable?: boolean;
    releaseChannel?: string;
  } = $props();
  let navigation: HTMLElement;
  let openMenu = $state<string | null>(null);

  const showDeferredPhase2 = $derived(releaseChannel === 'experimental');

  const menus: Menu[] = $derived([
    { label: 'Forms', icon: ClipboardList, groups: [
      { label: 'Data Entry', items: [
        { name: 'FS882 Data Forms (6x4)', icon: ClipboardList, view: 'fs882' },
        { name: 'Enter/Edit SIVI Data', icon: Tent, action: 'sivi' },
        { name: 'New SIVI / FS1333 Plot', icon: Tent, action: 'new-sivi' },
        { name: 'Universal Datasheet Grid', icon: TableProperties, view: 'datasheet' }
      ] },
      { label: 'Project & Classification', items: [
        { name: 'Project Admin, QA & Merge', icon: ClipboardList, view: 'project-admin' },
        { name: 'Cloud BECMaster Sync', icon: Globe, action: 'cloud-sync' },
        { name: 'SU & Hierarchy Explorer', icon: Network, view: 'classification-explorer' }
      ] },
      { label: 'Others', items: [
        { name: 'Metadata', icon: Table, action: 'metadata' },
        { name: 'Combine Species', icon: Link, view: 'lump-editor' },
        { name: 'Plot Pictures', icon: FileText, view: import.meta.env.VITE_PICTURE_MANAGER !== 'false' ? 'picture-manager' : undefined },
        { name: 'User log', icon: History, view: import.meta.env.VITE_USER_LOG_REVIEW !== 'false' ? 'user-log' : undefined },
        ...(showDeferredPhase2 ? [
          { name: 'Herbarium (Phase 2 Deferred)', icon: Leaf, deferredPhase2: true },
          { name: 'Colour-theme (Phase 2 Deferred)', icon: Palette, deferredPhase2: true },
          { name: 'User setup (Phase 2 Deferred)', icon: UserCog, deferredPhase2: true },
        ] : []),
      ] }
    ] },
    { label: 'Reports', icon: ChartColumn, groups: [
      { label: 'Vegetation', items: [
        { name: 'Long Vegetation', icon: TableProperties, view: import.meta.env.VITE_LONG_VEGETATION_REPORT !== 'false' ? 'long-vegetation' : undefined },
        { name: 'Lifeform Summary', icon: Leaf, view: import.meta.env.VITE_LIFEFORM_SUMMARY !== 'false' ? 'lifeform-summary' : undefined },
        { name: 'Summary Vegetation', icon: ChartColumn, view: import.meta.env.VITE_SUMMARY_VEGETATION !== 'false' ? 'summary-vegetation' : undefined }
      ] },
      { label: 'Environment', items: [
        { name: 'Long Environment', icon: TableProperties, view: import.meta.env.VITE_LONG_ENVIRONMENT_REPORT !== 'false' ? 'long-environment' : undefined },
        { name: 'Summary Environment', icon: ChartColumn, view: import.meta.env.VITE_SITE_UNIT_SUMMARY !== 'false' ? 'summary-environment' : undefined }
      ] },
      { label: 'Spatial & Synthesis', items: [
        { name: '3D Geospatial Workbench (BGC, QA & AOI)', icon: Globe, view: 'geospatial-map' },
        { name: 'BEC Diagnostic & Pairwise Synthesis', icon: ChartColumn, view: 'diagnostic-synthesis' },
        { name: 'R Analytics, climr aSMR & Quarto Guides', icon: Network, view: 'r-sidecar' }
      ] },
      { label: 'Others', items: [
        { name: 'Subzone Matrix of Units', icon: Table, view: import.meta.env.VITE_SUBZONE_MATRIX_WORKBOOK !== 'false' ? 'subzone-matrix' : undefined },
        { name: 'Hierarchy table', icon: Network, view: 'hierarchy' },
        { name: 'Plot Label PDF', icon: Tag,
          view: (import.meta.env.VITE_PLOT_LABEL_PREVIEW !== 'false' || import.meta.env.VITE_PLOT_LABEL_PDF_EXPORT !== 'false') ? 'plot-label-preview' : undefined },
        { name: 'Plot Locations', icon: FileText, view: import.meta.env.VITE_PLOT_LOCATION_REVIEW !== 'false' ? 'plot-locations' : undefined },
        { name: 'Google Earth (KML)', icon: Globe, view: import.meta.env.VITE_GOOGLE_EARTH_REVIEW !== 'false' ? 'google-earth-review' : undefined },
        ...(showDeferredPhase2 ? [
          { name: 'Hierarchy Diagram (Phase 2 Deferred)', icon: Network, deferredPhase2: true },
        ] : []),
      ] }
    ] },
    { label: 'Interchange', icon: Table, groups: [
      { label: 'Import & Export', items: [
        { name: 'Interchange Hub (VENUS 5, TurboVeg, RDS, GeoPackage)', icon: Globe, view: 'interchange-hub' },
        { name: 'Cloud BECMaster Sync', icon: Globe, action: 'cloud-sync' },
        { name: 'Project Table CSV Archive', icon: Table, view: import.meta.env.VITE_TABLE_CSV_REVIEW !== 'false' ? 'table-csv' : undefined }
      ] }
    ] },
    { label: 'Help', icon: HelpCircle, groups: [
      { label: 'Documentation & Onboarding', items: [
        { name: 'VPro User Guide & LMH25 Reference (F1)', icon: BookOpen, helpAction: 'guide' },
        { name: 'Interactive Walkthroughs (4 Tours)', icon: Compass, helpAction: 'tours' },
        { name: 'Keyboard Shortcuts (?)', icon: Keyboard, helpAction: 'shortcuts' }
      ] },
      { label: 'Release & Workspace', items: [
        { name: "What's New / Release Notes", icon: Newspaper, helpAction: 'whats-new' },
        { name: 'Reset Workspace to Sample', icon: RotateCcw, helpAction: 'reset-sample' },
        { name: 'About VPro & Standards Libraries', icon: Info, helpAction: 'about' }
      ] }
    ] }
  ]);

  function closeMenu(restoreFocus = false) {
    if (restoreFocus && openMenu) {
      navigation.querySelector<HTMLDetailsElement>('details[open]')?.querySelector('summary')?.focus();
    }
    openMenu = null;
  }

  function dismissOutside(event: Event) {
    if (event.target instanceof Node && !navigation?.contains(event.target)) closeMenu();
  }

  function dismissOnFocus(event: FocusEvent) {
    const disclosure = navigation?.querySelector('details[open]');
    if (disclosure && event.target instanceof Node && !disclosure.contains(event.target)) closeMenu();
  }

  function escape(event: KeyboardEvent) {
    if (event.key === 'Escape' && openMenu) {
      event.preventDefault();
      closeMenu(true);
    }
  }

  function navigate(next: View) {
    closeMenu(true);
    onnavigate(next);
  }

  function itemEnabled(item: Item) {
    if (item.action === 'sivi') return siviAvailable && !!onopensivi;
    if (item.action === 'new-sivi') return newSiviAvailable && !!onopennewsivi;
    if (item.action === 'metadata') return metadataAvailable && !!onopenmetadata;
    if (item.action === 'cloud-sync') return true;
    if (item.helpAction === 'reset-sample') return resetSampleAvailable;
    if (item.helpAction) return true;
    return !!item.view;
  }

  function activate(item: Item) {
    if (!itemEnabled(item)) return;
    if (item.action === 'sivi' && onopensivi) {
      closeMenu(true);
      onopensivi();
    } else if (item.action === 'new-sivi' && onopennewsivi) {
      closeMenu(true);
      onopennewsivi();
    } else if (item.action === 'metadata' && onopenmetadata) {
      closeMenu(true);
      onopenmetadata();
    } else if (item.action === 'cloud-sync') {
      if (typeof onopencloudsync === 'function') {
        closeMenu(true);
        onopencloudsync();
      } else {
        navigate('project-admin');
      }
    } else if (item.helpAction) {
      closeMenu(true);
      if (typeof onhelpaction === 'function') onhelpaction(item.helpAction);
    } else if (item.view) {
      navigate(item.view);
    }
  }
</script>

<svelte:window onpointerdown={dismissOutside} onfocusin={dismissOnFocus} onkeydowncapture={escape} />

<nav bind:this={navigation} class="main-nav" aria-label="Main navigation">
  <button class:current={view === 'home'} aria-current={view === 'home' ? 'page' : undefined} onclick={() => navigate('home')}>
    <House size={16} aria-hidden="true" />Home
  </button>
  <button class:current={view === 'plots'} aria-current={view === 'plots' ? 'page' : undefined} onclick={() => navigate('plots')}>
    <List size={16} aria-hidden="true" />Plots
  </button>
  <button class:current={view === 'geospatial-map'} aria-current={view === 'geospatial-map' ? 'page' : undefined} onclick={() => navigate('geospatial-map')}>
    <Globe size={16} aria-hidden="true" />3D Map
  </button>
  {#each menus as menu (menu.label)}
    <details class="nav-menu" data-nav-group={menu.label} open={openMenu === menu.label}>
      <summary
        class:current={menu.groups.some(group => group.items.some(item => item.view === view))}
        onclick={(event) => {
          event.preventDefault();
          openMenu = openMenu === menu.label ? null : menu.label;
        }}
      >
        <menu.icon size={16} aria-hidden="true" />
        {menu.label}
        {#if menu.label === 'Help' && whatsNewUnreadCount > 0}
          <span class="rounded bg-[#FCBA19] px-1.5 py-0.5 font-mono text-[10px] font-bold text-[#003366]" data-nav-whats-new-badge>{whatsNewUnreadCount}</span>
        {/if}
        <ChevronDown size={14} aria-hidden="true" />
      </summary>
      <div class="nav-menu-content">
        {#each menu.groups as group (group.label)}
          <section data-nav-group={group.label === 'Project & Classification' ? 'Project' : group.label} aria-labelledby={`nav-${menu.label}-${group.label.replaceAll(' ', '-')}`}>
            <h2 class="nav-group-label" id={`nav-${menu.label}-${group.label.replaceAll(' ', '-')}`}>{group.label}</h2>
            {#each group.items as item (item.name)}
              <button
                disabled={!itemEnabled(item)}
                class:current={item.view === view}
                data-help-menu-item={item.helpAction}
                aria-current={item.view === view ? 'page' : undefined}
                title={item.action === 'sivi'
                  ? itemEnabled(item) ? 'SIVI / FS1333 editor for the selected plot. Existing draft and close guards apply.'
                    : 'Select a plot to open SIVI / FS1333 data entry.'
                  : item.action === 'metadata'
                    ? 'Project metadata for the open plot. Requires a clean, unlocked editor; finish or Undo drafts first.'
                  : item.action === 'cloud-sync'
                    ? item.name
                  : item.helpAction === 'reset-sample'
                    ? 'Non-destructive recovery: materializes a fresh Sample.db and switches session to Sample without deleting user .db files.'
                  : item.helpAction ? item.name
                  : item.deferredPhase2 ? 'Deferred to Phase 2 per BEC VPro 2.0 scope triage'
                  : item.view === 'fs882' ? 'FS882 6x4 & 2-Page plot data entry' : item.view ? item.name : item.name}
                onclick={() => activate(item)}
              >
                <item.icon size={16} aria-hidden="true" />
                <span>{item.name}{item.helpAction === 'whats-new' && whatsNewUnreadCount > 0 ? ` (${whatsNewUnreadCount} new)` : ''}</span>
              </button>
            {/each}
          </section>
        {/each}
      </div>
    </details>
  {/each}
</nav>
