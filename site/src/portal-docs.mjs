export function initPortalDocs(document, windowObj = globalThis, navigatorObj = globalThis.navigator) {
  const searchInput = document.getElementById('docs-search-input');
  const clearBtn = document.getElementById('docs-search-clear');
  const countEl = document.getElementById('docs-results-count');
  const emptyEl = document.getElementById('docs-empty-state');
  const items = Array.from(document.querySelectorAll?.('[data-doc-item]') || []);
  const filterBtns = Array.from(document.querySelectorAll?.('[data-doc-filter]') || []);

  let activeFilter = 'all';
  let activeQuery = '';

  const applyFilters = () => {
    const tokens = activeQuery.trim().toLowerCase().split(/\s+/).filter(Boolean);
    let visibleCount = 0;
    for (const item of items) {
      const kind = item.getAttribute('data-doc-kind') || '';
      const haystack = (item.getAttribute('data-search-text') || '').toLowerCase();
      const matchesKind = activeFilter === 'all' || kind === activeFilter;
      const matchesTokens = tokens.length === 0 || tokens.every(tok => haystack.includes(tok));
      const show = matchesKind && matchesTokens;
      item.hidden = !show;
      if (show) visibleCount += 1;
    }
    if (countEl) {
      countEl.textContent = `Showing ${visibleCount} of ${items.length} reference entries`;
    }
    if (emptyEl) {
      emptyEl.hidden = visibleCount > 0;
    }
    if (clearBtn) {
      clearBtn.hidden = activeQuery.length === 0 && activeFilter === 'all';
    }
    for (const btn of filterBtns) {
      const filterValue = btn.getAttribute('data-doc-filter');
      const selected = filterValue === activeFilter;
      btn.setAttribute('aria-pressed', selected ? 'true' : 'false');
      if (btn.classList?.toggle) btn.classList.toggle('is-active', selected);
    }
  };

  const revealHashTarget = hash => {
    if (!hash || (!hash.startsWith('#doc-') && !hash.startsWith('#field-'))) return false;
    const id = decodeURIComponent(hash.slice(1));
    const target = document.getElementById(id);
    if (!target) return false;
    if (target.hidden) {
      activeFilter = 'all';
      activeQuery = '';
      if (searchInput) searchInput.value = '';
      applyFilters();
    }
    for (const prev of Array.from(document.querySelectorAll?.('.is-anchor-target') || [])) {
      prev.classList?.remove?.('is-anchor-target');
    }
    target.classList?.add?.('is-anchor-target');
    if (target.tagName === 'DETAILS') target.open = true;
    target.scrollIntoView?.({ behavior: 'smooth', block: 'start' });
    return true;
  };

  if (searchInput?.addEventListener) {
    searchInput.addEventListener('input', event => {
      activeQuery = event?.target?.value ?? searchInput.value ?? '';
      applyFilters();
    });
  }

  if (clearBtn?.addEventListener) {
    clearBtn.addEventListener('click', () => {
      activeQuery = '';
      activeFilter = 'all';
      if (searchInput) searchInput.value = '';
      applyFilters();
    });
  }

  for (const btn of filterBtns) {
    if (btn?.addEventListener) {
      btn.addEventListener('click', () => {
        activeFilter = btn.getAttribute('data-doc-filter') || 'all';
        applyFilters();
      });
    }
  }

  const citationBtn = document.getElementById('copy-citation-btn');
  const citationText = document.getElementById('mandatory-citation-text');
  const citationStatus = document.getElementById('copy-citation-status');
  if (citationBtn?.addEventListener && citationText) {
    citationBtn.addEventListener('click', async () => {
      const text = citationText.textContent?.trim() || '';
      if (text && navigatorObj?.clipboard?.writeText) {
        await navigatorObj.clipboard.writeText(text);
        citationBtn.textContent = 'Citation Copied';
        if (citationStatus) citationStatus.textContent = 'Mandatory BEC citation copied to clipboard.';
      }
    });
  }

  for (const anchorBtn of Array.from(document.querySelectorAll?.('[data-copy-anchor]') || [])) {
    if (anchorBtn?.addEventListener) {
      anchorBtn.addEventListener('click', async () => {
        const anchor = anchorBtn.getAttribute('data-copy-anchor');
        if (!anchor || !navigatorObj?.clipboard?.writeText) return;
        const base = windowObj?.location?.href ? windowObj.location.href.split('#')[0] : '';
        await navigatorObj.clipboard.writeText(`${base}#${anchor}`);
        anchorBtn.textContent = 'Link Copied';
      });
    }
  }

  applyFilters();
  if (windowObj?.location?.hash) {
    revealHashTarget(windowObj.location.hash);
  }
  windowObj?.addEventListener?.('hashchange', () => {
    revealHashTarget(windowObj.location?.hash || '');
  });

  return {
    setQuery(query) {
      activeQuery = String(query || '');
      if (searchInput) searchInput.value = activeQuery;
      applyFilters();
    },
    setFilter(filter) {
      activeFilter = filter || 'all';
      applyFilters();
    },
    revealHashTarget,
  };
}

if (typeof document !== 'undefined') {
  initPortalDocs(document, globalThis, globalThis.navigator);
}
