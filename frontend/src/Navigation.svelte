<script lang="ts">
  import { ChartColumn, ChevronDown, ClipboardList, FileText, Globe, HelpCircle, History, House, Leaf, Link, List, Network, Newspaper, Palette, Table, TableProperties, Tag, Tent, UserCog } from '@lucide/svelte';

  type View = 'home' | 'plots' | 'hierarchy' | 'fs882' | 'long-environment' | 'summary-environment' | 'long-vegetation' | 'summary-vegetation' | 'lifeform-summary' | 'table-csv' | 'plot-locations' | 'google-earth-review' | 'picture-manager' | 'plot-label-preview';
  type Item = { name: string; icon: typeof House; view?: View; action?: 'sivi' | 'metadata' };
  type Menu = { label: string; icon: typeof House; groups: { label: string; items: Item[] }[] };
  let { view, onnavigate, siviAvailable = false, onopensivi, metadataAvailable = false, onopenmetadata }: {
    view: View; onnavigate: (view: View) => void;
    siviAvailable?: boolean; onopensivi?: () => void;
    metadataAvailable?: boolean; onopenmetadata?: () => void;
  } = $props();
  let navigation: HTMLElement;
  let openMenu = $state<string | null>(null);

  const menus: Menu[] = [
    { label: 'Forms', icon: ClipboardList, groups: [
      { label: 'Data Entry', items: [
        { name: 'FS882 Data Forms (6x4) — Experimental', icon: ClipboardList, view: 'fs882' },
        { name: 'Enter/Edit SIVI Data', icon: Tent, action: 'sivi' }
      ] },
      { label: 'Others', items: [
        { name: 'Metadata', icon: Table, action: 'metadata' },
        { name: 'Combine Species', icon: Link },
        { name: 'Herbarium', icon: Leaf },
        { name: 'Plot Pictures — Read-only', icon: FileText, view: import.meta.env.VITE_PICTURE_MANAGER === 'true' ? 'picture-manager' : undefined },
        { name: 'Colour-theme', icon: Palette },
        { name: 'User setup', icon: UserCog },
        { name: 'User log', icon: History }
      ] }
    ] },
    { label: 'Reports', icon: ChartColumn, groups: [
      { label: 'Vegetation', items: [
        { name: 'Long Vegetation', icon: TableProperties, view: import.meta.env.VITE_LONG_VEGETATION_REPORT === 'true' ? 'long-vegetation' : undefined },
        { name: 'Lifeform Summary', icon: Leaf, view: import.meta.env.VITE_LIFEFORM_SUMMARY === 'true' ? 'lifeform-summary' : undefined },
        { name: 'Summary Vegetation', icon: ChartColumn, view: import.meta.env.VITE_SUMMARY_VEGETATION === 'true' ? 'summary-vegetation' : undefined }
      ] },
      { label: 'Environment', items: [
        { name: 'Long Environment', icon: TableProperties, view: import.meta.env.VITE_LONG_ENVIRONMENT_REPORT === 'true' ? 'long-environment' : undefined },
        { name: 'Summary Environment', icon: ChartColumn, view: import.meta.env.VITE_SITE_UNIT_SUMMARY === 'true' ? 'summary-environment' : undefined }
      ] },
      { label: 'Others', items: [
        { name: 'Subzone Matrix of Units', icon: Table },
        { name: 'Hierarchy table', icon: Network, view: 'hierarchy' },
        { name: 'Hierarchy Diagram', icon: Network },
        { name: 'Print a Plot Label', icon: Tag },
        { name: import.meta.env.VITE_PLOT_LABEL_PDF_EXPORT === 'true' ? 'Saved plot label PDF — Portable' : 'Saved plot label preview — Read-only', icon: Tag,
          view: (import.meta.env.VITE_PLOT_LABEL_PREVIEW === 'true' || import.meta.env.VITE_PLOT_LABEL_PDF_EXPORT === 'true') ? 'plot-label-preview' : undefined },
        { name: 'Plot location review', icon: FileText, view: import.meta.env.VITE_PLOT_LOCATION_REVIEW === 'true' ? 'plot-locations' : undefined },
        { name: 'Google Earth location preparation', icon: Globe, view: import.meta.env.VITE_GOOGLE_EARTH_REVIEW === 'true' ? 'google-earth-review' : undefined },
        { name: 'Create Plot Locations File', icon: FileText },
        { name: 'Show Plot Locations in Google Earth', icon: Globe }
      ] }
    ] },
    { label: 'Interchange', icon: Table, groups: [
      { label: 'Read-only preparation', items: [
        { name: 'Project table CSV review', icon: Table, view: import.meta.env.VITE_TABLE_CSV_REVIEW === 'true' ? 'table-csv' : undefined }
      ] }
    ] },
    { label: 'Help', icon: HelpCircle, groups: [
      { label: 'Help', items: [{ name: "What's New", icon: Newspaper }] }
    ] }
  ];

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
    if (item.action === 'metadata') return metadataAvailable && !!onopenmetadata;
    return !!item.view;
  }

  function activate(item: Item) {
    if (!itemEnabled(item)) return;
    if (item.action === 'sivi' && onopensivi) {
      closeMenu(true);
      onopensivi();
    } else if (item.action === 'metadata' && onopenmetadata) {
      closeMenu(true);
      onopenmetadata();
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
  {#each menus as menu (menu.label)}
    <details class="nav-menu" open={openMenu === menu.label}>
      <summary
        class:current={menu.groups.some(group => group.items.some(item => item.view === view))}
        onclick={(event) => {
          event.preventDefault();
          openMenu = openMenu === menu.label ? null : menu.label;
        }}
      >
        <menu.icon size={16} aria-hidden="true" />
        {menu.label}<ChevronDown size={14} aria-hidden="true" />
      </summary>
      <div class="nav-menu-content">
        <p class="nav-unavailable">Unavailable workflows are disabled.</p>
        {#each menu.groups as group (group.label)}
          <section aria-labelledby={`nav-${menu.label}-${group.label.replaceAll(' ', '-')}`}>
            <h2 class="nav-group-label" id={`nav-${menu.label}-${group.label.replaceAll(' ', '-')}`}>{group.label}</h2>
            {#each group.items as item (item.name)}
              <button
                disabled={!itemEnabled(item)}
                class:current={item.view === view}
                aria-current={item.view === view ? 'page' : undefined}
                title={item.action === 'sivi'
                  ? itemEnabled(item) ? 'Experimental normal SIVI / FS1333 for the selected plot. Existing draft and close guards apply. Use disposable projects only.'
                    : 'Select a plot and enable the independent SIVI standalone and parent-review gates.'
                  : item.action === 'metadata'
                    ? 'Project metadata for the open plot. Requires the existing metadata gate and a clean, unlocked editor; finish or Undo drafts first.'
                  : item.view === 'fs882' ? 'Experimental: only verified workflows are writable. Use disposable projects only.' : item.view ? item.name : 'Not yet available in the desktop app'}
                onclick={() => activate(item)}
              >
                <item.icon size={16} aria-hidden="true" />
                <span>{item.name}</span>
              </button>
            {/each}
          </section>
        {/each}
      </div>
    </details>
  {/each}
</nav>
