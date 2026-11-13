#!/usr/bin/env node
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { assetLinks, releasesURL, reviewManifest } from './src/downloads.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const defaultRepoRoot = path.resolve(__dirname, '..');

const REQUIRED_FONTS = Object.freeze([
  'BCSans-Regular.woff2',
  'BCSans-Italic.woff2',
  'BCSans-Bold.woff2',
  'BCSans-BoldItalic.woff2',
  'ibm-plex-mono-latin-400-normal.woff2',
  'ibm-plex-mono-latin-500-normal.woff2',
  'ibm-plex-mono-latin-600-normal.woff2',
]);

export function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

const ICONS = Object.freeze({
  crest: '<svg class="icon-lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M12 2L4 5.5v6c0 5.25 3.4 9.85 8 11 4.6-1.15 8-5.75 8-11v-6L12 2z"/><path d="M12 7v10"/><path d="M8.5 11.5L12 8l3.5 3.5"/><path d="M7.5 15L12 11l4.5 4"/></svg>',
  download: '<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
  shield: '<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>',
  windows: '<svg class="icon-lg" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><path d="M3 5.3l7.4-1v7.1H3V5.3zm8.4-1.1L21 3v8.4h-9.6V4.2zM3 12.6h7.4v7.1l-7.4-1v-6.1zm8.4 0H21V21l-9.6-1.3v-7.1z"/></svg>',
  apple: '<svg class="icon-lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><rect x="3" y="4" width="18" height="12" rx="2"/><line x1="8" y1="20" x2="16" y2="20"/><line x1="12" y1="16" x2="12" y2="20"/></svg>',
  linux: '<svg class="icon-lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>',
  search: '<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>',
  book: '<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>',
  github: '<svg class="icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0 0 22 12.017C22 6.484 17.522 2 12 2z"/></svg>',
});

function renderAssetRow(id, label, recommended = false) {
  return `
              <div class="asset-row">
                <div class="asset-row-top">
                  <a id="${escapeHtml(id)}" class="download-link" aria-disabled="true">${escapeHtml(label)}${recommended ? ' (Recommended)' : ''}</a>
                  <span id="${escapeHtml(id)}-size" class="asset-size">Unavailable</span>
                </div>
                <div class="asset-row-bottom">
                  <code id="${escapeHtml(id)}-sha256" class="asset-sha">SHA-256 pending</code>
                  <button type="button" id="${escapeHtml(id)}-copy" class="copy-btn" disabled aria-disabled="true">Copy SHA-256</button>
                </div>
              </div>`;
}

function renderNoscriptFallback(reviewedManifest) {
  const active = reviewedManifest.channels[reviewedManifest.defaultChannel]
    || reviewedManifest.channels.stable
    || reviewedManifest.channels.preview;
  if (!active) {
    return `
      <noscript>
        <div class="noscript-box">
          <strong>JavaScript is disabled:</strong> Static release manifest check indicates no published desktop release is currently active in <code>release-manifest.json</code>.
          Browse <a href="${escapeHtml(releasesURL)}">published GitHub releases</a> directly and verify downloaded archives against <code>SHA256SUMS.txt</code>.
        </div>
      </noscript>`;
  }
  const items = Object.entries(assetLinks)
    .filter(([id]) => Boolean(active.links[id]))
    .map(([id, name]) => {
      const meta = active.metadata[id];
      const details = meta ? ` (${escapeHtml(meta.formattedSize)}, SHA-256: <code>${escapeHtml(meta.sha256)}</code>)` : '';
      return `<li><a href="${escapeHtml(active.links[id])}">${escapeHtml(name)}</a>${details}</li>`;
    })
    .join('\n            ');
  return `
      <noscript>
        <div class="noscript-box">
          <strong>Static Direct Downloads (${escapeHtml(active.tag)}${active.prerelease ? ' — Preview/UAT' : ''}):</strong>
          <p>Verify every downloaded package against <a href="${escapeHtml(active.sha256sumsUrl)}">SHA256SUMS.txt</a> prior to installation.</p>
          <ul>
            ${items}
          </ul>
        </div>
      </noscript>`;
}

function deriveStandardsSummary(provenance) {
  const summary = {
    becVersion: 'BECdb v13 (2026)',
    taxonomyVersion: 'BCPlantTaxonomy v19 (2026)',
    lmh25Edition: 'LMH25 (2023)',
    becSha256: '',
    taxonomySha256: '',
    vlistsSha256: '',
  };
  for (const src of provenance?.sources || []) {
    if (String(src.version || '').startsWith('BECdb_v13')) {
      summary.becSha256 = src.extractedSha256 || '';
    } else if (String(src.version || '').startsWith('BCPlantTaxa_v19')) {
      summary.taxonomySha256 = src.extractedSha256 || '';
    }
  }
  for (const artifact of provenance?.artifacts || []) {
    if (artifact.path === 'resources/database-family/VLists.db') {
      summary.vlistsSha256 = artifact.sha256 || '';
    }
  }
  return summary;
}

export function renderPortalHtml({
  guides,
  lmh25,
  whatsNew,
  shortcuts,
  dataUseAgreement,
  provenance,
  manifest,
  assetVersions = {},
}) {
  const reviewedManifest = reviewManifest(manifest);
  if (!Array.isArray(dataUseAgreement?.clauses) || dataUseAgreement.clauses.length !== 9) {
    throw new Error(`Expected 9 BECWeb Data Use Agreement clauses, got ${dataUseAgreement?.clauses?.length}`);
  }
  const standards = deriveStandardsSummary(provenance);
  const fields = Array.isArray(lmh25?.fields) ? lmh25.fields : [];
  const decisionKeys = Array.isArray(lmh25?.decisionKeys) ? lmh25.decisionKeys : [];
  const totalDocEntries = guides.length + fields.length + decisionKeys.length + 1;

  const guidesHtml = guides.map(guide => {
    const anchorId = `doc-${guide.id}`;
    const searchBlob = [
      guide.category,
      guide.title,
      guide.summary,
      ...(guide.sections || []).flatMap(sec => [sec.heading, sec.body]),
    ].join(' ');
    const sectionsHtml = (guide.sections || []).map(sec => `
              <div class="guide-subcard">
                <h4>${escapeHtml(sec.heading)}</h4>
                <p>${escapeHtml(sec.body)}</p>
              </div>`).join('');
    return `
          <article id="${escapeHtml(anchorId)}" class="doc-card" data-doc-item data-doc-kind="guide" data-search-text="${escapeHtml(searchBlob)}">
            <div class="doc-card-header">
              <div class="doc-meta-badges">
                <span class="code-chip">User Guide</span>
                <span class="page-chip">${escapeHtml(guide.category)}</span>
              </div>
              <div class="doc-meta-badges">
                <a class="anchor-link" href="#${escapeHtml(anchorId)}">#${escapeHtml(anchorId)}</a>
                <button type="button" class="copy-btn" data-copy-anchor="${escapeHtml(anchorId)}">Copy Link</button>
              </div>
            </div>
            <h3>${escapeHtml(guide.title)}</h3>
            <p class="doc-summary">${escapeHtml(guide.summary)}</p>
            <div class="guide-sections">${sectionsHtml}
            </div>
          </article>`;
  }).join('\n');

  const keysHtml = decisionKeys.map(key => {
    const anchorId = `doc-${key.id}`;
    const searchBlob = [
      key.code,
      key.title,
      `LMH25 page ${key.lmh25Page}`,
      ...(key.steps || []).flatMap(st => [st.prompt, st.yes || '', st.no || '']),
    ].join(' ');
    const hasYesNo = (key.steps || []).some(st => st.yes || st.no);
    const rowsHtml = (key.steps || []).map(st => hasYesNo
      ? `<tr>
                  <td class="mono">${escapeHtml(st.step)}</td>
                  <td>${escapeHtml(st.prompt)}</td>
                  <td><strong>${escapeHtml(st.yes || '—')}</strong></td>
                  <td>${escapeHtml(st.no || '—')}</td>
                </tr>`
      : `<tr>
                  <td class="mono">${escapeHtml(st.step)}</td>
                  <td colspan="3">${escapeHtml(st.prompt)}</td>
                </tr>`).join('\n                ');
    return `
          <article id="${escapeHtml(anchorId)}" class="doc-card" data-doc-item data-doc-kind="key" data-search-text="${escapeHtml(searchBlob)}">
            <div class="doc-card-header">
              <div class="doc-meta-badges">
                <span class="code-chip">${escapeHtml(key.code)}</span>
                <span class="page-chip">LMH25 (2023) p. ${escapeHtml(key.lmh25Page)}</span>
              </div>
              <div class="doc-meta-badges">
                <a class="anchor-link" href="#${escapeHtml(anchorId)}">#${escapeHtml(anchorId)}</a>
                <button type="button" class="copy-btn" data-copy-anchor="${escapeHtml(anchorId)}">Copy Link</button>
              </div>
            </div>
            <h3>${escapeHtml(key.title)}</h3>
            <table class="bcgov-table">
              <thead>
                <tr>
                  <th>Step</th>
                  <th>Diagnostic Criterion / Reference Scale</th>
                  ${hasYesNo ? '<th>If Yes</th><th>If No</th>' : '<th colspan="2">Reference</th>'}
                </tr>
              </thead>
              <tbody>
                ${rowsHtml}
              </tbody>
            </table>
          </article>`;
  }).join('\n');

  const fieldsHtml = fields.map(field => {
    const anchorId = `field-${field.id}`;
    const searchBlob = [
      field.id,
      field.code,
      field.section,
      field.title,
      field.column,
      field.unit,
      `LMH25 page ${field.lmh25Page}`,
      field.summary,
      ...(field.rules || []),
      ...(field.codes || []).flatMap(c => [c.code, c.label]),
    ].join(' ');
    const rulesHtml = (field.rules || []).length > 0
      ? `<ul class="rules-list">${field.rules.map(r => `<li>${escapeHtml(r)}</li>`).join('')}</ul>`
      : '';
    const codesHtml = (field.codes || []).length > 0
      ? `<div class="codes-grid">${field.codes.map(c => `<div><code class="mono"><strong>${escapeHtml(c.code)}</strong></code> — ${escapeHtml(c.label)}</div>`).join('')}</div>`
      : '';
    const keyLinkHtml = field.keyRef
      ? `<a class="anchor-link" href="#doc-${escapeHtml(field.keyRef)}">See Decision Key (${escapeHtml(field.keyRef)})</a>`
      : '';
    return `
          <article id="${escapeHtml(anchorId)}" class="doc-card" data-doc-item data-doc-kind="field" data-search-text="${escapeHtml(searchBlob)}">
            <div class="doc-card-header">
              <div class="doc-meta-badges">
                <span class="code-chip">${escapeHtml(field.code)}</span>
                <span class="page-chip">LMH25 (2023) p. ${escapeHtml(field.lmh25Page)}</span>
                <span class="page-chip">${escapeHtml(field.section)}</span>
              </div>
              <div class="doc-meta-badges">
                ${keyLinkHtml}
                <a class="anchor-link" href="#${escapeHtml(anchorId)}">#${escapeHtml(anchorId)}</a>
                <button type="button" class="copy-btn" data-copy-anchor="${escapeHtml(anchorId)}">Copy Link</button>
              </div>
            </div>
            <h3>${escapeHtml(field.title)}</h3>
            <p class="doc-summary">${escapeHtml(field.summary)}</p>
            <p class="doc-summary"><strong>SQLite / VP08 Column:</strong> <code>${escapeHtml(field.column)}</code> · <strong>Unit / Type:</strong> <code>${escapeHtml(field.unit)}</code></p>
            ${rulesHtml}
            ${codesHtml}
          </article>`;
  }).join('\n');

  const shortcutsSearchBlob = [
    'Keyboard Shortcuts Accelerators F1 Enter Ctrl+S Ctrl+F Esc',
    ...shortcuts.flatMap(s => [s.keys, s.scope, s.action]),
  ].join(' ');
  const shortcutsRows = shortcuts.map(s => `
                <tr>
                  <td><kbd>${escapeHtml(s.keys)}</kbd></td>
                  <td>${escapeHtml(s.scope)}</td>
                  <td>${escapeHtml(s.action)}</td>
                </tr>`).join('');
  const shortcutsHtml = `
          <article id="doc-keyboard-shortcuts" class="doc-card" data-doc-item data-doc-kind="shortcut" data-search-text="${escapeHtml(shortcutsSearchBlob)}">
            <div class="doc-card-header">
              <div class="doc-meta-badges">
                <span class="code-chip">Keyboard Reference</span>
                <span class="page-chip">Desktop Entry &amp; Navigation</span>
              </div>
              <div class="doc-meta-badges">
                <a class="anchor-link" href="#doc-keyboard-shortcuts">#doc-keyboard-shortcuts</a>
                <button type="button" class="copy-btn" data-copy-anchor="doc-keyboard-shortcuts">Copy Link</button>
              </div>
            </div>
            <h3>VPro 2.0 Keyboard Shortcuts &amp; High-Speed Card Entry</h3>
            <table class="bcgov-table">
              <thead>
                <tr>
                  <th>Shortcut</th>
                  <th>Context / Form Scope</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>${shortcutsRows}
              </tbody>
            </table>
          </article>`;

  const clausesHtml = dataUseAgreement.clauses.map(clause => `
          <li class="clause-item">${escapeHtml(clause)}</li>`).join('');

  const changelogRowsHtml = whatsNew.map(entry => {
    const badgeClass = entry.channel === 'stable' ? 'status-verified' : 'status-preview';
    const items = (entry.highlights || []).map(h => `<li>${escapeHtml(h)}</li>`).join('');
    return `
              <tr>
                <td class="mono"><strong>v${escapeHtml(entry.version)}</strong></td>
                <td class="mono">${escapeHtml(entry.date)}</td>
                <td><span class="${badgeClass}">${escapeHtml(entry.channel)}</span></td>
                <td><ul class="rules-list">${items}</ul></td>
              </tr>`;
  }).join('');

  const cssHref = assetVersions.css ? `./styles.css?v=${encodeURIComponent(assetVersions.css)}` : './styles.css';
  const downloadsSrc = assetVersions.downloads ? `./downloads.mjs?v=${encodeURIComponent(assetVersions.downloads)}` : './downloads.mjs';
  const portalDocsSrc = assetVersions.portalDocs ? `./portal-docs.mjs?v=${encodeURIComponent(assetVersions.portalDocs)}` : './portal-docs.mjs';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>VPro — VENUS (Vegetation and Environment NexUS) Professional | B.C. Ministry of Forests BEC Program</title>
  <meta name="description" content="Official release portal, LMH25 (2023) field reference, and BECWeb Data Use Agreement for VPro (VENUS — Vegetation and Environment NexUS — Professional), B.C. Ministry of Forests, Research Branch — Biogeoclimatic Ecosystem Classification (BEC) Program.">
  <link rel="icon" href="./favicon.ico" sizes="any">
  <link rel="stylesheet" href="${cssHref}">
</head>
<body>
  <a class="skip-link" href="#main-content">Skip to main content</a>

  <header class="bcgov-header">
    <div class="container bcgov-header-inner">
      <a class="bcgov-brand" href="#main-content">
        <span class="bcgov-crest">${ICONS.crest}</span>
        <span>
          <span class="bcgov-brand-title">
            <span>VPro</span>
            <span class="bec-badge">BEC Program</span>
          </span>
          <span class="bcgov-brand-subtitle">B.C. Ministry of Forests · Research Branch · Biogeoclimatic Ecosystem Classification</span>
        </span>
      </a>
      <nav class="bcgov-nav" aria-label="Primary navigation">
        <a href="#downloads">Downloads</a>
        <a href="#migration">Migrating from VPro 64</a>
        <a href="#installation">Checksum &amp; Setup</a>
        <a href="#documentation">User Guide &amp; LMH25</a>
        <a href="#data-use-agreement">Data Use Agreement</a>
        <a href="#changelog">Release History</a>
        <a href="https://www.for.gov.bc.ca/hre/becweb/" target="_blank" rel="noopener noreferrer">BECWeb</a>
        <a href="https://github.com/Boostao/vpro_wails" target="_blank" rel="noopener noreferrer">${ICONS.github} GitHub</a>
      </nav>
    </div>
  </header>

  <main id="main-content">
    <section class="hero-section" aria-labelledby="hero-heading">
      <div class="container hero-grid">
        <div>
          <div class="hero-eyebrow">
            ${ICONS.shield}
            <span>Successor to VPro 64 (Table Version VP08) at vuser.ca</span>
          </div>
          <h1 id="hero-heading">VPro — VENUS (Vegetation and Environment NexUS) Professional</h1>
          <p class="hero-lead">
            Official cross-platform ecological plot management, validation, classification, and synthesis desktop software for the
            <strong>B.C. Ministry of Forests, Research Branch — Biogeoclimatic Ecosystem Classification (BEC) Program</strong>.
            Runs natively on transactional SQLite WAL storage with zero Microsoft Access runtime dependency.
          </p>
          <div class="standards-strip" aria-label="Bundled 2026 Provincial Reference Standards">
            <span class="standard-pill" title="SHA-256: ${escapeHtml(standards.becSha256)}">
              <span>Master BEC:</span>
              <strong id="standard-bec-version">${escapeHtml(standards.becVersion)}</strong>
            </span>
            <span class="standard-pill" title="SHA-256: ${escapeHtml(standards.taxonomySha256)}">
              <span>Taxonomy:</span>
              <strong id="standard-taxonomy-version">${escapeHtml(standards.taxonomyVersion)}</strong>
            </span>
            <span class="standard-pill">
              <span>Field Manual:</span>
              <strong id="standard-lmh25-version">${escapeHtml(standards.lmh25Edition)}</strong>
            </span>
          </div>
        </div>

        <div class="download-panel" aria-label="Recommended Desktop Package">
          <div class="channel-bar">
            <span id="release-channel-badge" class="channel-bar-label">Official Stable Channel</span>
            <div id="channel-switcher" class="channel-switcher" role="group" aria-label="Select release channel">
              <button type="button" id="channel-btn-stable" class="channel-btn is-active" data-channel="stable" aria-pressed="true">Stable</button>
              <button type="button" id="channel-btn-preview" class="channel-btn" data-channel="preview" aria-pressed="false">Preview / UAT</button>
            </div>
          </div>

          <div id="detected-platform-banner" class="detected-row">
            <span id="detected-platform-label">Detected workstation:</span> <strong id="detected-platform-name">Detecting...</strong>
          </div>

          <div id="platform-switcher" class="platform-switcher" role="group" aria-label="Select workstation platform and architecture">
            <button type="button" id="plat-btn-win-amd64" class="platform-pill" aria-pressed="false">Windows x64</button>
            <button type="button" id="plat-btn-win-arm64" class="platform-pill" aria-pressed="false">Windows ARM64</button>
            <button type="button" id="plat-btn-mac-universal" class="platform-pill" aria-pressed="false">macOS</button>
            <button type="button" id="plat-btn-linux-amd64" class="platform-pill" aria-pressed="false">Linux x64</button>
            <button type="button" id="plat-btn-linux-arm64" class="platform-pill" aria-pressed="false">Linux ARM64</button>
          </div>

          <a id="primary-download-btn" href="#downloads" class="primary-btn">
            ${ICONS.download}
            <span id="primary-download-text">Browse All Downloads</span>
          </a>

          <div class="release-meta-row">
            <span id="release-version-tag" class="mono">Version: Loading...</span>
            <span id="release-published-date"></span>
            <span id="primary-download-meta" class="mono"></span>
          </div>

          <div id="primary-sha256-row" class="primary-sha-row" hidden>
            <span>SHA-256: <code id="primary-sha256-value" class="mono"></code></span>
            <button type="button" id="primary-sha256-copy" class="copy-btn">Copy SHA-256</button>
          </div>

          <p id="download-status" class="status-banner" role="status" aria-live="polite">
            Loading verified static release manifest...
          </p>
        </div>
      </div>
    </section>

    <section id="migration" class="section section-white" aria-labelledby="migration-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="migration-heading">Migrating from VPro 64</h2>
          <p>Move existing Microsoft Access (<code>.accdb</code> / <code>.mdb</code>) ecological plot archives from <code>vuser.ca</code> VPro 64 workflows into VPro 2.0 in three steps.</p>
        </div>
        <div class="three-col-grid">
          <div class="card">
            <div>
              <span class="card-step-num">1</span>
              <h3>1. Install VPro 2.0 — Zero Access Runtime</h3>
              <p>Deploy a single self-contained ~38 MB native desktop executable backed by SQLite Write-Ahead Logging (<code>PRAGMA journal_mode = WAL</code>). Eliminates 32-bit vs. 64-bit Microsoft Access Database Engine conflicts on Ministry Windows 11 workstations.</p>
            </div>
            <a href="#downloads">Choose workstation package</a>
          </div>
          <div class="card">
            <div>
              <span class="card-step-num">2</span>
              <h3>2. Import Existing .accdb/.mdb Projects</h3>
              <p>Read legacy VPro 64 databases (Table Versions <code>VP05</code>–<code>VP08</code>, including 71,000+ plot <code>BECMaster</code> archives) directly into the canonical 7-database SQLite family with atomic audit logging and orphan-row quarantine receipts.</p>
            </div>
            <a href="#doc-migrating-from-vpro64">Read VPro 64 migration guide</a>
          </div>
          <div class="card">
            <div>
              <span class="card-step-num">3</span>
              <h3>3. Audit Standards &amp; Publish Reports</h3>
              <p>Validate plots against embedded <code>BECdb v13 (2026)</code>, <code>BCPlantTaxonomy v19 (2026)</code>, and <code>LMH25 (2023)</code> rules, run sub-10 ms BEC differential synthesis, inspect 3D offline <code>PMTiles</code> maps, and export <code>.xlsx</code>, <code>.gpkg</code>, <code>VENUS 5.0</code>, <code>TurboVeg 3</code>, or R <code>.rds</code>.</p>
            </div>
            <a href="#doc-standards-and-interchange">Explore standards &amp; exports</a>
          </div>
        </div>
      </div>
    </section>

    <section id="downloads" class="section" aria-labelledby="downloads-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="downloads-heading">Desktop Packages &amp; Static Checksum Manifest</h2>
          <p>All download metadata is served from a build-verified static <code>release-manifest.json</code> with zero browser API calls, ensuring reliable access behind shared Government of B.C. network gateways.</p>
        </div>

        <div class="advisory-box">
          <span class="alert-note">Phase 1 Unsigned Package Notice:</span>
          Phase 1 desktop installers and portable archives are distributed as unsigned builds (macOS <code>VPro.app</code> is ad-hoc signed, not notarized). Ministry of Forests and contractor workstations should verify package SHA-256 digests against <code>SHA256SUMS.txt</code> before first launch. Note that standalone <code>VTrees</code> mensuration, CWD, and WHA modules remain out of scope for Phase 1.
        </div>

        <div class="three-col-grid">
          <div id="card-windows" class="card">
            <div>
              <h3>
                ${ICONS.windows}
                <span>Windows</span>
                <span class="os-tag">Windows 10 / 11</span>
              </h3>
              <p>Native WebView2 desktop client with embedded SQLite WAL engine. Portable <code>.zip</code> runs without administrator elevation.</p>

              <div class="arch-heading">x86_64 (Standard 64-bit Intel / AMD)</div>
              <div class="asset-list">
                ${renderAssetRow('dl-win-x64-installer', 'NSIS Installer (.exe)', true)}
                ${renderAssetRow('dl-win-x64-zip', 'Portable Archive (.zip)')}
                ${renderAssetRow('dl-win-x64-exe', 'Standalone Executable (.exe)')}
              </div>

              <div class="arch-heading">ARM64 (Surface Pro / Snapdragon)</div>
              <div class="asset-list">
                ${renderAssetRow('dl-win-arm64-installer', 'NSIS Installer (.exe)')}
                ${renderAssetRow('dl-win-arm64-zip', 'Portable Archive (.zip)')}
                ${renderAssetRow('dl-win-arm64-exe', 'Standalone Executable (.exe)')}
              </div>
            </div>
          </div>

          <div id="card-macos" class="card">
            <div>
              <h3>
                ${ICONS.apple}
                <span>macOS</span>
                <span class="os-tag">macOS 12+ Universal</span>
              </h3>
              <p>Fat Mach-O universal application bundle (<code>VPro.app</code>) supporting both Apple Silicon (M1–M4) and Intel Macs natively.</p>

              <div class="arch-heading">Universal (Apple Silicon &amp; Intel)</div>
              <div class="asset-list">
                ${renderAssetRow('dl-mac-dmg', 'Disk Image (.dmg)', true)}
                ${renderAssetRow('dl-mac-zip', 'VPro.app Bundle Archive (.zip)')}
              </div>
            </div>
          </div>

          <div id="card-linux" class="card">
            <div>
              <h3>
                ${ICONS.linux}
                <span>Linux</span>
                <span class="os-tag">GTK4 / WebKitGTK 6.0</span>
              </h3>
              <p>Standalone AppImage runs directly on Fedora, RHEL, Ubuntu 24.04+, and Debian 13+; native <code>.deb</code> provided for Debian and Ubuntu.</p>

              <div class="arch-heading">x86_64 (amd64)</div>
              <div class="asset-list">
                ${renderAssetRow('dl-linux-appimage', 'Standalone AppImage', true)}
                ${renderAssetRow('dl-linux-deb', 'Debian / Ubuntu (.deb)')}
                ${renderAssetRow('dl-linux-tar', 'Binary Tarball (.tar.gz)')}
              </div>

              <div class="arch-heading">ARM64 (aarch64)</div>
              <div class="asset-list">
                ${renderAssetRow('dl-linux-arm64-appimage', 'Standalone AppImage')}
                ${renderAssetRow('dl-linux-arm64-deb', 'Debian / Ubuntu (.deb)')}
                ${renderAssetRow('dl-linux-arm64-tar', 'Binary Tarball (.tar.gz)')}
              </div>
            </div>
          </div>
        </div>

        <div class="checksum-bar">
          <div>
            ${ICONS.shield}
            <strong>Cryptographic Checksum Manifest:</strong>
            Verify downloaded packages against <a id="dl-checksums" class="download-link" aria-disabled="true">SHA256SUMS.txt</a>
            (<span id="dl-checksums-size" class="asset-size">Unavailable</span>, <code id="dl-checksums-sha256" class="asset-sha">SHA-256 pending</code>).
          </div>
          <div class="doc-meta-badges">
            <button type="button" id="dl-checksums-copy" class="copy-btn" disabled aria-disabled="true">Copy SHA-256</button>
            <a href="${escapeHtml(releasesURL)}">View all GitHub releases</a>
          </div>
        </div>
        ${renderNoscriptFallback(reviewedManifest)}
      </div>
    </section>

    <section id="installation" class="section section-white" aria-labelledby="installation-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="installation-heading">Workstation Installation &amp; SHA-256 Verification</h2>
          <p>Verify every Phase 1 package hash locally before running on Ministry of Forests or research workstations.</p>
        </div>
        <div class="install-grid">
          <div class="card">
            <div>
              <h3>${ICONS.windows} Windows 10 / 11</h3>
              <ol class="rules-list">
                <li>Download <code>vpro-windows-amd64-installer.exe</code> (or the portable <code>.zip</code> when installing without local admin rights).</li>
                <li>Verify the SHA-256 hash in PowerShell against <code>SHA256SUMS.txt</code>:
                  <pre class="code-block">Get-FileHash .\\vpro-windows-amd64-installer.exe -Algorithm SHA256</pre>
                </li>
                <li>If Windows Defender SmartScreen prompts on an unsigned Phase 1 build, confirm the SHA-256 matches <code>SHA256SUMS.txt</code> before selecting <strong>More info</strong> &rarr; <strong>Run anyway</strong>.</li>
              </ol>
            </div>
          </div>

          <div class="card">
            <div>
              <h3>${ICONS.apple} macOS Universal (VPro.app)</h3>
              <ol class="rules-list">
                <li>Download <code>vpro-macos-universal.dmg</code> and verify its SHA-256 digest in Terminal:
                  <pre class="code-block">shasum -a 256 vpro-macos-universal.dmg</pre>
                </li>
                <li>Mount the disk image and drag <strong>VPro.app</strong> to your <strong>Applications</strong> folder.</li>
                <li>Because Phase 1 builds use ad-hoc signing without Apple notarization, Control-click <strong>VPro.app</strong> in Applications and select <strong>Open</strong> on first launch after verifying the checksum.</li>
              </ol>
            </div>
          </div>

          <div class="card">
            <div>
              <h3>${ICONS.linux} Linux (amd64 &amp; arm64)</h3>
              <ol class="rules-list">
                <li>Verify the downloaded package against <code>SHA256SUMS.txt</code>:
                  <pre class="code-block">sha256sum -c --ignore-missing SHA256SUMS.txt</pre>
                </li>
                <li>Run the AppImage directly or install the Debian package:
                  <pre class="code-block">chmod +x vpro-linux-amd64.AppImage &amp;&amp; ./vpro-linux-amd64.AppImage
sudo apt install ./vpro-linux-amd64.deb</pre>
                </li>
                <li>Requires GTK4 and WebKitGTK 6.0 runtime libraries; pass <code>--appimage-extract-and-run</code> when FUSE is unavailable.</li>
              </ol>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section id="documentation" class="section" aria-labelledby="documentation-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="documentation-heading">${ICONS.book} Online User Guide &amp; LMH25 (2023) Field Reference</h2>
          <p>Compiled from the same single-source <code>docs-src/</code> dictionary embedded offline inside VPro's in-app <code>F1</code> Field &amp; Key Inspector. Share deep-link anchors (<code>#doc-*</code> and <code>#field-*</code>) directly in email or Teams support threads.</p>
        </div>

        <div class="docs-toolbar" role="search" aria-label="Filter User Guide and LMH25 Field Reference">
          <div class="search-input-wrap">
            ${ICONS.search}
            <input
              id="docs-search-input"
              class="docs-search-input"
              type="search"
              placeholder="Search LMH25 fields (rSMR, Humus, Cover6, PlotNumber), keys (9.4, 9.6), or VPro guides..."
              aria-label="Search documentation and LMH25 fields"
            >
          </div>
          <div class="filter-pills" role="group" aria-label="Filter by category">
            <button type="button" id="docs-filter-all" class="filter-pill is-active" data-doc-filter="all" aria-pressed="true">All (${totalDocEntries})</button>
            <button type="button" id="docs-filter-guides" class="filter-pill" data-doc-filter="guide" aria-pressed="false">User Guides (${guides.length})</button>
            <button type="button" id="docs-filter-keys" class="filter-pill" data-doc-filter="key" aria-pressed="false">LMH25 Keys (${decisionKeys.length})</button>
            <button type="button" id="docs-filter-fields" class="filter-pill" data-doc-filter="field" aria-pressed="false">LMH25 Fields (${fields.length})</button>
            <button type="button" id="docs-filter-shortcuts" class="filter-pill" data-doc-filter="shortcut" aria-pressed="false">Shortcuts (1)</button>
            <button type="button" id="docs-search-clear" class="copy-btn" hidden>Reset Filter</button>
          </div>
        </div>

        <div id="docs-results-count" class="docs-count" aria-live="polite">
          Showing ${totalDocEntries} of ${totalDocEntries} reference entries
        </div>

        <p id="docs-empty-state" class="advisory-box" hidden>
          No User Guide or LMH25 (2023) entries matched your search query. Try searching by field code (<code>SITE 7</code>, <code>SOIL 4</code>, <code>VEG 2</code>), database column (<code>MoistureRegime</code>), or key number (<code>9.4</code>).
        </p>

        <div class="docs-stack">
          ${guidesHtml}
          ${keysHtml}
          ${fieldsHtml}
          ${shortcutsHtml}
        </div>
      </div>
    </section>

    <section id="data-use-agreement" class="section section-white" aria-labelledby="dua-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="dua-heading">${escapeHtml(dataUseAgreement.title)}</h2>
          <p><strong>Custodial Authority:</strong> ${escapeHtml(dataUseAgreement.authority)} · <a href="${escapeHtml(dataUseAgreement.sourceUrl)}" target="_blank" rel="noopener noreferrer">Official BECWeb Policy Source</a></p>
        </div>

        <div class="citation-callout">
          <div class="doc-card-header">
            <strong>Mandatory Attribution &amp; Database Version Citation</strong>
            <div class="doc-meta-badges">
              <span id="copy-citation-status" class="asset-size" aria-live="polite"></span>
              <button type="button" id="copy-citation-btn" class="copy-btn">Copy Citation</button>
            </div>
          </div>
          <p id="mandatory-citation-text" class="mono" style="margin: 0.35rem 0 0; font-size: 0.85rem;">${escapeHtml(dataUseAgreement.citation)}</p>
        </div>

        <ol class="clauses-list">
          ${clausesHtml}
        </ol>
      </div>
    </section>

    <section id="changelog" class="section" aria-labelledby="changelog-heading">
      <div class="container">
        <div class="section-header">
          <h2 id="changelog-heading">Release History &amp; Standards Provenance Ledger</h2>
          <p>Structured release history replacing legacy <code>tblWhatsNew</code> and <code>vuser.ca</code> version tables, paired with cryptographic digests for embedded 2026 BEC reference databases.</p>
        </div>

        <table class="bcgov-table" style="margin-bottom: 1.5rem;">
          <thead>
            <tr>
              <th>Version</th>
              <th>Date</th>
              <th>Channel</th>
              <th>Key Capabilities &amp; Calibration Notes</th>
            </tr>
          </thead>
          <tbody>${changelogRowsHtml}
          </tbody>
        </table>

        <table class="bcgov-table">
          <thead>
            <tr>
              <th>Embedded 2026 Master Standard</th>
              <th>Release Stamp</th>
              <th>Verified SHA-256 Digest</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><strong>BECdb Master Classification</strong></td>
              <td class="mono">BECdb_v13_2026 (2026-05-28)</td>
              <td class="mono">${escapeHtml(standards.becSha256)}</td>
            </tr>
            <tr>
              <td><strong>B.C. Provincial Plant Taxonomy</strong></td>
              <td class="mono">BCPlantTaxonomy v19 (2026-02-09)</td>
              <td class="mono">${escapeHtml(standards.taxonomySha256)}</td>
            </tr>
            <tr>
              <td><strong>Canonical SQLite VLists.db</strong></td>
              <td class="mono">resources/database-family/VLists.db</td>
              <td class="mono">${escapeHtml(standards.vlistsSha256)}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>

  <footer class="bcgov-footer">
    <div class="container footer-inner">
      <div>
        <strong>Province of British Columbia — Ministry of Forests, Research Branch</strong>
        <div>Biogeoclimatic Ecosystem Classification (BEC) Program · VPro (VENUS — Vegetation and Environment NexUS — Professional)</div>
        <div style="margin-top: 0.35rem; color: #C9D8E8;">Successor to VPro 64 (Table Version VP08) formerly distributed via <code>vuser.ca</code>.</div>
      </div>
      <div class="footer-links">
        <a href="https://www.for.gov.bc.ca/hre/becweb/" target="_blank" rel="noopener noreferrer">BECWeb Home</a>
        <a href="#data-use-agreement">Data Use Agreement</a>
        <a href="#documentation">LMH25 (2023) Reference</a>
        <a href="https://github.com/Boostao/vpro_wails" target="_blank" rel="noopener noreferrer">Source Repository</a>
        <a href="https://github.com/Boostao/vpro_wails/issues" target="_blank" rel="noopener noreferrer">Issue Tracker</a>
      </div>
    </div>
  </footer>

  <script type="module" src="${downloadsSrc}"></script>
  <script type="module" src="${portalDocsSrc}"></script>
</body>
</html>
`;
}

export function validateBuiltAssets({ html, css, downloadsJs, portalDocsJs }) {
  const combined = [html, css, downloadsJs, portalDocsJs].join('\n');
  const emojiMatch = combined.match(/\p{Extended_Pictographic}/u);
  if (emojiMatch) {
    throw new Error(`Forbidden emoji character detected in portal output: ${emojiMatch[0]}`);
  }
  const forbiddenPatterns = [
    [/cdn\.tailwindcss\.com/i, 'Tailwind Play CDN'],
    [/api\.github\.com/i, 'runtime GitHub REST API URL'],
    [/fonts\.googleapis\.com|fonts\.gstatic\.com/i, 'external Google Fonts CDN'],
    [/Wails Edition/i, 'internal "Wails Edition" label'],
    [/vprouser\.ca/i, 'inaccurate "vprouser.ca" domain'],
    [/VPRO 6\.x/i, 'inaccurate "VPRO 6.x" version name'],
    [/vpro-wails\.app/i, 'legacy "vpro-wails.app" bundle name'],
  ];
  for (const [pattern, description] of forbiddenPatterns) {
    if (pattern.test(combined)) {
      throw new Error(`Forbidden ${description} found in compiled portal.`);
    }
  }
  const uniqueIds = [
    ...Object.keys(assetLinks),
    'detected-platform-label',
    'detected-platform-name',
    'platform-switcher',
    'plat-btn-win-amd64',
    'plat-btn-win-arm64',
    'plat-btn-mac-universal',
    'plat-btn-linux-amd64',
    'plat-btn-linux-arm64',
    'primary-download-btn',
    'primary-download-text',
    'release-version-tag',
    'download-status',
    'card-windows',
    'card-macos',
    'card-linux',
    'channel-btn-stable',
    'channel-btn-preview',
  ];
  for (const id of uniqueIds) {
    const matches = html.match(new RegExp(`id="${id}"`, 'g')) || [];
    if (matches.length !== 1) {
      throw new Error(`Expected id="${id}" exactly once in index.html, found ${matches.length}.`);
    }
  }
}

export function buildSite({
  repoRoot = defaultRepoRoot,
  outDir = path.join(repoRoot, 'site', 'dist'),
  syncDocs = true,
  manifestOverride = null,
} = {}) {
  const docsSrcDir = path.join(repoRoot, 'docs-src');
  const siteDir = path.join(repoRoot, 'site');
  const fontsSrcDir = path.join(siteDir, 'fonts');

  const lmh25 = JSON.parse(fs.readFileSync(path.join(docsSrcDir, 'lmh25_fields.json'), 'utf8'));
  const guides = JSON.parse(fs.readFileSync(path.join(docsSrcDir, 'guides.json'), 'utf8'));
  const whatsNew = JSON.parse(fs.readFileSync(path.join(docsSrcDir, 'whats_new.json'), 'utf8'));
  const shortcuts = JSON.parse(fs.readFileSync(path.join(docsSrcDir, 'shortcuts.json'), 'utf8'));
  const dataUseAgreement = JSON.parse(fs.readFileSync(path.join(docsSrcDir, 'data_use_agreement.json'), 'utf8'));
  const provenance = JSON.parse(fs.readFileSync(path.join(repoRoot, 'resources', 'standards-2026-provenance.json'), 'utf8'));
  const manifest = manifestOverride ?? JSON.parse(fs.readFileSync(path.join(siteDir, 'release-manifest.json'), 'utf8'));

  const css = fs.readFileSync(path.join(siteDir, 'src', 'styles.css'), 'utf8');
  const downloadsJs = fs.readFileSync(path.join(siteDir, 'src', 'downloads.mjs'), 'utf8');
  const portalDocsJs = fs.readFileSync(path.join(siteDir, 'src', 'portal-docs.mjs'), 'utf8');
  const shortHash = content => crypto.createHash('sha256').update(content, 'utf8').digest('hex').slice(0, 8);
  const html = renderPortalHtml({
    guides,
    lmh25,
    whatsNew,
    shortcuts,
    dataUseAgreement,
    provenance,
    manifest,
    assetVersions: {
      css: shortHash(css),
      downloads: shortHash(downloadsJs),
      portalDocs: shortHash(portalDocsJs),
    },
  });
  const manifestJson = JSON.stringify(manifest, null, 2) + '\n';

  validateBuiltAssets({ html, css, downloadsJs, portalDocsJs });

  fs.rmSync(outDir, { recursive: true, force: true });
  fs.mkdirSync(path.join(outDir, 'fonts'), { recursive: true });

  for (const fontFile of REQUIRED_FONTS) {
    const srcFont = path.join(fontsSrcDir, fontFile);
    if (!fs.existsSync(srcFont)) {
      throw new Error(`Missing required self-hosted WOFF2 font: ${srcFont}`);
    }
    fs.copyFileSync(srcFont, path.join(outDir, 'fonts', fontFile));
  }

  const faviconSrc = fs.existsSync(path.join(siteDir, 'favicon.ico'))
    ? path.join(siteDir, 'favicon.ico')
    : path.join(repoRoot, 'frontend', 'public', 'favicon.ico');
  if (!fs.existsSync(faviconSrc)) {
    throw new Error(`Missing required favicon.ico: ${faviconSrc}`);
  }
  fs.copyFileSync(faviconSrc, path.join(outDir, 'favicon.ico'));

  fs.writeFileSync(path.join(outDir, 'index.html'), html, 'utf8');
  fs.writeFileSync(path.join(outDir, 'styles.css'), css, 'utf8');
  fs.writeFileSync(path.join(outDir, 'downloads.mjs'), downloadsJs, 'utf8');
  fs.writeFileSync(path.join(outDir, 'portal-docs.mjs'), portalDocsJs, 'utf8');
  fs.writeFileSync(path.join(outDir, 'release-manifest.json'), manifestJson, 'utf8');
  fs.writeFileSync(path.join(outDir, '.nojekyll'), '', 'utf8');

  if (syncDocs) {
    const docsDir = path.join(repoRoot, 'docs');
    fs.mkdirSync(docsDir, { recursive: true });
    fs.copyFileSync(faviconSrc, path.join(docsDir, 'favicon.ico'));
    fs.writeFileSync(path.join(docsDir, 'index.html'), html, 'utf8');
    fs.writeFileSync(path.join(docsDir, 'styles.css'), css, 'utf8');
    fs.writeFileSync(path.join(docsDir, 'downloads.mjs'), downloadsJs, 'utf8');
    fs.writeFileSync(path.join(docsDir, 'portal-docs.mjs'), portalDocsJs, 'utf8');
    fs.writeFileSync(path.join(docsDir, 'release-manifest.json'), manifestJson, 'utf8');
  }

  return { outDir, fonts: REQUIRED_FONTS.length };
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const started = performance.now();
  const result = buildSite();
  const elapsedMs = (performance.now() - started).toFixed(1);
  console.log(`Built BC Gov VPro portal -> ${path.relative(defaultRepoRoot, result.outDir)} (${result.fonts} WOFF2 fonts) in ${elapsedMs}ms`);
}
