const assert = require('node:assert/strict');
const { test } = require('node:test');
const { existsSync, readFileSync, statSync } = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const repoRoot = path.join(__dirname, '..', '..');
const portal = import('../../site/src/downloads.mjs');
const siteBuilder = import('../../site/build.mjs');
const portalDocs = import('../../site/src/portal-docs.mjs');

const url = (name, tag = 'v2.0.0') => `https://github.com/Boostao/vpro_wails/releases/download/${tag}/${name}`;
const response = payload => ({ ok: true, status: 200, json: async () => payload });

function shaFor(seed) {
  const hex = Buffer.from(seed, 'utf8').toString('hex');
  return hex.repeat(Math.ceil(64 / hex.length)).slice(0, 64);
}

function channelReleaseFixture(links, {
  tag = 'v2.0.0',
  version = '2.0.0',
  prerelease = false,
  publishedAt = '2026-11-12T15:00:00Z',
} = {}) {
  return {
    tag,
    version,
    publishedAt,
    prerelease,
    unsignedNotice: true,
    sha256sumsUrl: url('SHA256SUMS.txt', tag),
    standards: {
      becVersion: 'BECdb v13 (2026-05-28)',
      taxonomyVersion: 'BCPlantTaxonomy v19 (2026-02-09)',
      lmh25Edition: 'LMH25 (2023)',
    },
    assets: Object.values(links).map((name, idx) => ({
      name,
      url: url(name, tag),
      sha256: shaFor(`${tag}:${name}`),
      sizeBytes: name === 'SHA256SUMS.txt' ? 1420 : (idx + 1) * 1024 * 1024 + 512,
    })),
  };
}

function manifestFixture(links, { includePreview = false, stableNull = false } = {}) {
  return {
    schemaVersion: 1,
    repository: 'Boostao/vpro_wails',
    defaultChannel: 'stable',
    channels: {
      stable: stableNull ? null : channelReleaseFixture(links, { tag: 'v2.0.0', version: '2.0.0', prerelease: false }),
      preview: includePreview
        ? channelReleaseFixture(links, { tag: 'v2.1.0-rc.1', version: '2.1.0-rc.1', prerelease: true })
        : null,
    },
  };
}

function releaseFixture(links) {
  return {
    tag_name: 'v2.0.0',
    draft: false,
    prerelease: false,
    assets: Object.values(links).map(name => ({ name, browser_download_url: url(name, 'v2.0.0') })),
  };
}

function makeElement(id) {
  const listeners = new Map();
  const classes = new Set();
  return {
    id,
    textContent: '',
    href: 'old-incorrect-asset',
    hidden: false,
    disabled: false,
    attributes: {},
    get classes() {
      return Array.from(classes);
    },
    setAttribute(name, value) {
      this.attributes[name] = String(value);
    },
    getAttribute(name) {
      return this.attributes[name];
    },
    removeAttribute(name) {
      delete this.attributes[name];
      if (name === 'href') delete this.href;
    },
    addEventListener(type, fn) {
      if (!listeners.has(type)) listeners.set(type, []);
      listeners.get(type).push(fn);
    },
    click() {
      for (const fn of listeners.get('click') || []) fn({ target: this });
    },
    classList: {
      add(...names) { for (const n of names) classes.add(n); },
      remove(...names) { for (const n of names) classes.delete(n); },
      toggle(name, force) {
        const shouldAdd = force === undefined ? !classes.has(name) : Boolean(force);
        if (shouldAdd) classes.add(name); else classes.delete(name);
        return shouldAdd;
      },
      contains(name) { return classes.has(name); },
    },
  };
}

function documentFixture(links) {
  const elements = new Map();
  const baseIds = [
    'detected-platform-label',
    'detected-platform-name',
    'plat-btn-win-amd64',
    'plat-btn-win-arm64',
    'plat-btn-mac-universal',
    'plat-btn-linux-amd64',
    'plat-btn-linux-arm64',
    'primary-download-btn',
    'primary-download-text',
    'release-version-tag',
    'release-published-date',
    'release-channel-badge',
    'primary-download-meta',
    'primary-sha256-row',
    'primary-sha256-value',
    'primary-sha256-copy',
    'standard-bec-version',
    'standard-taxonomy-version',
    'standard-lmh25-version',
    'channel-btn-stable',
    'channel-btn-preview',
    'download-status',
    'card-windows',
    'card-linux',
    'card-macos',
  ];
  for (const id of baseIds) {
    elements.set(id, makeElement(id));
  }
  for (const id of Object.keys(links)) {
    elements.set(id, makeElement(id));
    elements.set(`${id}-size`, makeElement(`${id}-size`));
    elements.set(`${id}-sha256`, makeElement(`${id}-sha256`));
    elements.set(`${id}-copy`, makeElement(`${id}-copy`));
  }
  return {
    getElementById(id) { return elements.get(id); },
    elements,
  };
}

test('architecture hints override emulated Windows UA; redacted/blocked hints preserve UA fallback; 32-bit and mobile reject', async () => {
  const { detectPlatform, recommendedAsset } = await portal;
  const rows = [
    [{ userAgent: 'Windows NT Win64 x64', platform: 'Win32',
      userAgentData: { platform: 'Windows', getHighEntropyValues: async () => ({ architecture: 'arm', bitness: '64' }) } }, 'windows', 'arm64', 'dl-win-arm64-installer'],
    [{ userAgent: 'Windows NT Win64 x64', platform: 'Win32' }, 'windows', 'amd64', 'dl-win-x64-installer'],
    [{ userAgent: 'Mozilla/5.0 (Windows NT 10.0; rv:159.0) Gecko/20100101 Firefox/159.0', platform: 'Win32' }, 'windows', 'amd64', 'dl-win-x64-installer'],
    [{ userAgent: 'Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36', platform: 'Win32' }, 'windows', 'amd64', 'dl-win-x64-installer'],
    [{ userAgent: 'Linux aarch64', platform: 'Linux' }, 'linux', 'arm64', 'dl-linux-arm64-appimage'],
    [{ userAgent: 'Linux x86_64', platform: 'Linux' }, 'linux', 'amd64', 'dl-linux-appimage'],
    [{ userAgent: 'Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:159.0) Gecko/20100101 Firefox/159.0', platform: 'Linux x86_64', oscpu: 'Linux x86_64' }, 'linux', 'amd64', 'dl-linux-appimage'],
    [{ userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/156.0.0.0', platform: 'Linux x86_64',
      userAgentData: { platform: 'Unknown', getHighEntropyValues: async () => ({ architecture: '', bitness: '' }) } }, 'linux', 'amd64', 'dl-linux-appimage'],
    [{ userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/156.0.0.0', platform: 'Linux x86_64',
      userAgentData: { platform: 'Linux', getHighEntropyValues: async () => ({ architecture: 'x86_64', bitness: 64 }) } }, 'linux', 'amd64', 'dl-linux-appimage'],
    [{ userAgent: 'Macintosh', platform: 'MacIntel' }, 'darwin', 'universal', 'dl-mac-dmg'],
    [{ userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/156.0', platform: 'MacIntel', maxTouchPoints: 5,
      userAgentData: { platform: 'macOS', mobile: false } }, 'darwin', 'universal', 'dl-mac-dmg'],
    [{ userAgent: '', platform: 'darwin' }, 'darwin', 'universal', 'dl-mac-dmg'],
    [{ userAgent: 'Windows NT', platform: 'Win32' }, 'windows', 'unknown', null],
    [{ userAgent: 'Mozilla/5.0 (X11; Linux i686)', platform: 'Linux i686' }, 'linux', 'unknown', null],
    [{ userAgent: 'Android Linux aarch64', platform: 'Linux' }, 'unknown', 'unknown', null],
    [{ userAgent: 'iPhone', platform: 'iPhone' }, 'unknown', 'unknown', null],
    [{ userAgent: 'Macintosh', platform: 'MacIntel', maxTouchPoints: 5 }, 'unknown', 'unknown', null],
    [{ userAgent: '', platform: '' }, 'unknown', 'unknown', null],
  ];
  for (const [navigator, os, arch, asset] of rows) {
    const client = await detectPlatform(navigator);
    assert.equal(client.os, os, JSON.stringify(navigator));
    assert.equal(client.arch, arch, JSON.stringify(navigator));
    assert.equal(recommendedAsset(client), asset, JSON.stringify(navigator));
  }

  // Redacted/empty or rejected high-entropy hints fall back to UA/platform arch when present, and warn when UA lacks arch.
  const redactedLinux = await detectPlatform({
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64)',
    platform: 'Linux x86_64',
    userAgentData: { platform: 'Linux', getHighEntropyValues: async () => ({}) },
  });
  assert.equal(redactedLinux.os, 'linux');
  assert.equal(redactedLinux.arch, 'amd64');

  const rejectedWithNoUAArch = await detectPlatform({
    userAgent: 'Windows NT',
    platform: 'Win32',
    userAgentData: {
      platform: 'Windows',
      getHighEntropyValues: async () => { throw Error('hints denied'); },
    },
  });
  assert.equal(rejectedWithNoUAArch.arch, 'unknown');
  assert.match(rejectedWithNoUAArch.warning, /hints denied/);

  for (const hints of [{ architecture: 'x86', bitness: '32' }, { architecture: 'arm', bitness: '32' }, { architecture: 'riscv64', bitness: '64' }]) {
    const client = await detectPlatform({
      userAgent: 'Windows Win64 x64',
      userAgentData: {
        platform: 'Windows',
        getHighEntropyValues: async () => hints,
      },
    });
    assert.equal(client.arch, 'unknown');
    assert.equal(recommendedAsset(client), null);
  }
});

test('exact asset names prevent AMD64/ARM64 and executable/installer confusion in either response order', async () => {
  const { assetLinks, reviewManifest, reviewRelease } = await portal;
  assert.equal(Object.keys(assetLinks).length, 15);
  for (const reversed of [false, true]) {
    const legacy = releaseFixture(assetLinks);
    if (reversed) legacy.assets.reverse();
    const legacyReview = reviewRelease(legacy);
    for (const [id, name] of Object.entries(assetLinks)) assert.equal(legacyReview.links[id], url(name));

    const manifest = manifestFixture(assetLinks, { includePreview: true });
    if (reversed) manifest.channels.stable.assets.reverse();
    const reviewed = reviewManifest(manifest);
    for (const [id, name] of Object.entries(assetLinks)) {
      assert.equal(reviewed.channels.stable.links[id], url(name, 'v2.0.0'));
      assert.equal(reviewed.channels.preview.links[id], url(name, 'v2.1.0-rc.1'));
    }
    assert.notEqual(reviewed.channels.stable.links['dl-win-x64-installer'], reviewed.channels.stable.links['dl-win-arm64-installer']);
  }
});

test('portal names match the production release validator, including its checksum filename', async () => {
  const { assetLinks } = await portal;
  const pythonCmd = spawnSync('python3', ['--version']).status === 0 ? 'python3' : 'python';
  const result = spawnSync(pythonCmd, ['-c',
    "import json, runpy; release=runpy.run_path('scripts/release.py'); print(json.dumps(sorted(release['REQUIRED'] | {release['MANIFEST']})))"],
  { cwd: repoRoot, encoding: 'utf8' });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(Object.values(assetLinks).sort(), JSON.parse(result.stdout));
});

test('malformed duplicate and unsafe static manifest or asset metadata refuses closed', async () => {
  const { assetLinks, reviewManifest, reviewRelease } = await portal;
  for (const bad of [
    null,
    {},
    { schemaVersion: 2, repository: 'Boostao/vpro_wails', defaultChannel: 'stable', channels: { stable: null } },
    { schemaVersion: 1, repository: 'Someone/other', defaultChannel: 'stable', channels: { stable: null } },
    { schemaVersion: 1, repository: 'Boostao/vpro_wails', defaultChannel: 'nightly', channels: { stable: null } },
    { schemaVersion: 1, repository: 'Boostao/vpro_wails', defaultChannel: 'stable', channels: {} },
    { schemaVersion: 1, repository: 'Boostao/vpro_wails', defaultChannel: 'stable', channels: { stable: null, nightly: null } },
    { ...manifestFixture(assetLinks), extraKey: true },
  ]) {
    assert.throws(() => reviewManifest(bad));
  }

  // Channel prerelease mismatch & tag/version mismatch
  const wrongStable = manifestFixture(assetLinks);
  wrongStable.channels.stable.prerelease = true;
  assert.throws(() => reviewManifest(wrongStable), /prerelease/);

  const wrongPreview = manifestFixture(assetLinks, { includePreview: true });
  wrongPreview.channels.preview.prerelease = false;
  assert.throws(() => reviewManifest(wrongPreview), /prerelease/);

  const mismatchedTag = manifestFixture(assetLinks);
  mismatchedTag.channels.stable.tag = 'v9.9.9';
  assert.throws(() => reviewManifest(mismatchedTag), /tag/);

  // Duplicate, unknown, bad SHA-256, bad sizeBytes, and unsafe URLs
  const duplicate = manifestFixture(assetLinks);
  duplicate.channels.stable.assets.push(duplicate.channels.stable.assets[0]);
  assert.throws(() => reviewManifest(duplicate), /duplicate/);

  const unexpectedFile = manifestFixture(assetLinks);
  unexpectedFile.channels.stable.assets[0].name = 'unexpected-trojan.exe';
  unexpectedFile.channels.stable.assets[0].url = url('unexpected-trojan.exe');
  assert.throws(() => reviewManifest(unexpectedFile), /unexpected file/);

  for (const badSha of ['', 'ABC'.repeat(21) + 'A', 'g'.repeat(64), 'a'.repeat(63)]) {
    const m = manifestFixture(assetLinks);
    m.channels.stable.assets[0].sha256 = badSha;
    assert.throws(() => reviewManifest(m), /SHA-256/);
  }

  for (const badSize of [0, -100, 12.5, NaN, '1024']) {
    const m = manifestFixture(assetLinks);
    m.channels.stable.assets[0].sizeBytes = badSize;
    assert.throws(() => reviewManifest(m), /sizeBytes/);
  }

  for (const badURL of [
    undefined,
    'javascript:alert(1)',
    'https://example.com/package.exe',
    'https://user:pass@github.com/Boostao/vpro_wails/releases/download/v2.0.0/' + assetLinks['dl-win-x64-installer'],
    url(assetLinks['dl-win-x64-installer']) + '?redirect=evil',
    url(assetLinks['dl-win-x64-installer']) + '#fragment',
    url(assetLinks['dl-win-x64-installer']).replace('/v2.0.0/', '/v0.9.0/'),
    'https://github.com/Someone/another/releases/download/v2.0.0/' + assetLinks['dl-win-x64-installer'],
  ]) {
    const m = manifestFixture(assetLinks);
    m.channels.stable.assets[0].url = badURL;
    assert.throws(() => reviewManifest(m));

    const legacy = releaseFixture(assetLinks);
    legacy.assets[0].browser_download_url = badURL;
    assert.throws(() => reviewRelease(legacy));
  }
});

test('loadPortal fetches local ./release-manifest.json with zero api.github.com calls and supports Stable <-> Preview channel switching + SHA-256 copy', async () => {
  const { assetLinks, loadPortal } = await portal;
  for (const [userAgent, platform, primary] of [
    ['Windows Win64 x64', 'Win32', 'dl-win-x64-installer'],
    ['Windows arm64', 'Win32', 'dl-win-arm64-installer'],
    ['Linux x86_64', 'Linux', 'dl-linux-appimage'],
    ['Linux aarch64', 'Linux', 'dl-linux-arm64-appimage'],
    ['Macintosh', 'MacIntel', 'dl-mac-dmg'],
  ]) {
    let copiedText = '';
    const document = documentFixture(assetLinks);
    const manifest = manifestFixture(assetLinks, { includePreview: true });
    const controller = await loadPortal(
      document,
      {
        userAgent,
        platform,
        clipboard: { writeText: async text => { copiedText = text; } },
      },
      async (endpoint, options) => {
        assert.equal(endpoint, './release-manifest.json');
        assert.equal(options.headers.Accept, 'application/json');
        for (const id of Object.keys(assetLinks)) assert.equal(document.getElementById(id).href, undefined);
        return response(manifest);
      },
    );

    assert.equal(controller.getChannel(), 'stable');
    for (const [id, name] of Object.entries(assetLinks)) {
      assert.equal(document.getElementById(id).href, url(name, 'v2.0.0'));
      assert.equal(document.getElementById(id).attributes['aria-disabled'], undefined);
      assert.match(document.getElementById(`${id}-size`).textContent, /KB|MB/);
      assert.equal(document.getElementById(`${id}-sha256`).attributes['data-sha256'], shaFor(`v2.0.0:${name}`));
    }
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks[primary], 'v2.0.0'));
    assert.match(document.getElementById('release-version-tag').textContent, /v2\.0\.0/);
    assert.match(document.getElementById('download-status').textContent, /SHA256SUMS/);

    // Copy SHA-256 button copies exact 64-hex digest
    document.getElementById(`${primary}-copy`).click();
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(copiedText, shaFor(`v2.0.0:${assetLinks[primary]}`));

    // Switch to Preview / UAT channel via UI click
    document.getElementById('channel-btn-preview').click();
    assert.equal(controller.getChannel(), 'preview');
    assert.match(document.getElementById('release-version-tag').textContent, /v2\.1\.0-rc\.1/);
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks[primary], 'v2.1.0-rc.1'));
    assert.equal(document.getElementById(`${primary}-sha256`).attributes['data-sha256'], shaFor(`v2.1.0-rc.1:${assetLinks[primary]}`));

    // Switch back to Stable channel
    document.getElementById('channel-btn-stable').click();
    assert.equal(controller.getChannel(), 'stable');
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks[primary], 'v2.0.0'));

    // Interactive platform switcher overrides spoofed UA in 1 click
    document.getElementById('plat-btn-mac-universal').click();
    assert.deepEqual(controller.getPlatform(), { os: 'darwin', arch: 'universal', warning: '' });
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks['dl-mac-dmg'], 'v2.0.0'));
    assert.equal(document.getElementById('card-macos').classList.contains('is-detected-platform'), true);
    assert.equal(document.getElementById('card-windows').classList.contains('is-detected-platform'), false);
    document.getElementById('plat-btn-linux-amd64').click();
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks['dl-linux-appimage'], 'v2.0.0'));
  }
});

test('when only Preview/RC is published in release-manifest.json, portal activates preview automatically instead of failing', async () => {
  const { assetLinks, loadPortal } = await portal;
  const document = documentFixture(assetLinks);
  const previewOnly = manifestFixture(assetLinks, { includePreview: true, stableNull: true });
  const controller = await loadPortal(document, { userAgent: 'Windows Win64 x64', platform: 'Win32' }, async () => response(previewOnly));
  assert.equal(controller.getChannel(), 'preview');
  assert.match(document.getElementById('release-version-tag').textContent, /v2\.1\.0-rc\.1/);
  assert.equal(document.getElementById('primary-download-btn').href, url('vpro-windows-amd64-installer.exe', 'v2.1.0-rc.1'));
});

test('missing matching assets and unknown architecture keep browsing live but never recommend a different CPU', async () => {
  const { assetLinks, loadPortal } = await portal;
  const partial = manifestFixture(assetLinks);
  partial.channels.stable.assets = partial.channels.stable.assets.filter(asset => !asset.name.includes('arm64'));
  const document = documentFixture(assetLinks);
  await loadPortal(document, { userAgent: 'Windows arm64', platform: 'Win32' }, async () => response(partial));
  assert.equal(document.getElementById('primary-download-btn').href, '#downloads');
  assert.equal(document.getElementById('dl-win-arm64-installer').href, undefined);
  assert.equal(document.getElementById('dl-win-arm64-installer').attributes['aria-disabled'], 'true');
  assert.equal(document.getElementById('dl-win-x64-installer').href, url(assetLinks['dl-win-x64-installer'], 'v2.0.0'));
  assert.match(document.getElementById('download-status').textContent, /unavailable/);

  const unknown = documentFixture(assetLinks);
  await loadPortal(unknown, { userAgent: 'Windows NT', platform: 'Win32' }, async () => response(manifestFixture(assetLinks)));
  assert.equal(unknown.getElementById('primary-download-btn').href, '#downloads');
  assert.match(unknown.getElementById('download-status').textContent, /Architecture not detected/);
});

test('unpublished null channels, HTTP errors, and offline failures leave zero stale download targets', async () => {
  const { assetLinks, loadPortal, releasesURL } = await portal;
  const emptyChannelsManifest = manifestFixture(assetLinks, { includePreview: false, stableNull: true });
  for (const fetchFn of [
    async () => response(emptyChannelsManifest),
    async () => ({ ok: false, status: 404 }),
    async () => ({ ok: false, status: 500 }),
    async () => { throw Error('network offline'); },
    async () => response(null),
  ]) {
    const document = documentFixture(assetLinks);
    await loadPortal(document, { userAgent: 'Windows Win64', platform: 'Win32' }, fetchFn);
    assert.equal(document.getElementById('primary-download-btn').href, releasesURL);
    assert.equal(document.getElementById('release-version-tag').textContent, 'Release unavailable');
    assert.match(document.getElementById('download-status').textContent, /unavailable|failed|offline|malformed|no published/i);
    for (const id of Object.keys(assetLinks)) assert.equal(document.getElementById(id).href, undefined);
  }
});

test('BC Gov Design System build outputs self-hosted fonts, searchable LMH25 docs, 9-clause BECWeb Data Use Agreement, noscript links, and zero CDN/API/emoji defects', async () => {
  const { assetLinks, loadPortal } = await portal;
  const { buildSite, renderPortalHtml } = await siteBuilder;
  const { initPortalDocs } = await portalDocs;

  buildSite({ repoRoot });

  const distDir = path.join(repoRoot, 'site', 'dist');
  const distHtml = readFileSync(path.join(distDir, 'index.html'), 'utf8');
  const distCss = readFileSync(path.join(distDir, 'styles.css'), 'utf8');
  const distDownloads = readFileSync(path.join(distDir, 'downloads.mjs'), 'utf8');
  const distPortalDocs = readFileSync(path.join(distDir, 'portal-docs.mjs'), 'utf8');
  assert.ok(existsSync(path.join(distDir, 'favicon.ico')), 'Missing site/dist/favicon.ico');
  assert.equal(existsSync(path.join(repoRoot, 'docs')), false, 'Redundant root docs/ copy must remain absent');
  assert.match(distHtml, /<link rel="icon" href="\.\/favicon\.ico"/);

  // Required self-hosted WOFF2 fonts in site/dist/fonts/
  for (const fontName of [
    'BCSans-Regular.woff2',
    'BCSans-Italic.woff2',
    'BCSans-Bold.woff2',
    'BCSans-BoldItalic.woff2',
    'ibm-plex-mono-latin-400-normal.woff2',
    'ibm-plex-mono-latin-500-normal.woff2',
    'ibm-plex-mono-latin-600-normal.woff2',
  ]) {
    const fontPath = path.join(distDir, 'fonts', fontName);
    assert.ok(existsSync(fontPath), `Missing font ${fontName}`);
    assert.ok(statSync(fontPath).size > 1000, `Empty font ${fontName}`);
    assert.ok(distCss.includes(`./fonts/${fontName}`), `CSS missing @font-face for ${fontName}`);
  }

  // BC Gov Design System color tokens
  for (const color of ['#003366', '#FCBA19', '#FFFFFF', '#F2F2F2', '#313132', '#2E8540', '#D8292F', '#1E5128']) {
    assert.ok(distCss.includes(color), `Missing BC Gov token ${color}`);
  }

  // Zero emojis, zero CDN scripts, zero runtime GitHub API references, zero legacy branding bugs
  for (const content of [distHtml, distCss, distDownloads, distPortalDocs]) {
    assert.equal(/\p{Extended_Pictographic}/u.test(content), false, 'Portal must contain zero emojis');
    assert.equal(/cdn\.tailwindcss\.com/i.test(content), false, 'Portal must not load Tailwind Play CDN');
    assert.equal(/api\.github\.com/i.test(content), false, 'Portal must never call api.github.com');
    assert.equal(/Wails Edition|vprouser\.ca|VPRO 6\.x|vpro-wails\.app/i.test(content), false, 'Portal contains forbidden legacy copy');
  }

  // Accurate Ministry branding, VPro 64 + vuser.ca history, VPro.app bundle, and checksum CLI guidance
  assert.match(distHtml, /VPro 64/);
  assert.match(distHtml, /vuser\.ca/);
  assert.match(distHtml, /VPro\.app/);
  assert.match(distHtml, /sha256sum/);
  assert.match(distHtml, /Get-FileHash/);
  assert.match(distHtml, /shasum -a 256/);
  assert.match(distHtml, /<script type="module" src="\.\/downloads\.mjs\?v=[0-9a-f]{8}"><\/script>/);

  // Every planned HTML selector exists once
  for (const id of [...Object.keys(assetLinks), 'download-status', 'channel-btn-stable', 'channel-btn-preview']) {
    assert.equal((distHtml.match(new RegExp(`id="${id}"`, 'g')) || []).length, 1, id);
  }

  // Single-source User Guides, LMH25 (2023) fields/keys, and all 9 BECWeb Data Use Agreement clauses
  const guides = JSON.parse(readFileSync(path.join(repoRoot, 'docs-src', 'guides.json'), 'utf8'));
  const lmh25 = JSON.parse(readFileSync(path.join(repoRoot, 'docs-src', 'lmh25_fields.json'), 'utf8'));
  const dua = JSON.parse(readFileSync(path.join(repoRoot, 'docs-src', 'data_use_agreement.json'), 'utf8'));
  const whatsNew = JSON.parse(readFileSync(path.join(repoRoot, 'docs-src', 'whats_new.json'), 'utf8'));
  const shortcuts = JSON.parse(readFileSync(path.join(repoRoot, 'docs-src', 'shortcuts.json'), 'utf8'));
  const provenance = JSON.parse(readFileSync(path.join(repoRoot, 'resources', 'standards-2026-provenance.json'), 'utf8'));

  for (const guide of guides) assert.ok(distHtml.includes(`id="doc-${guide.id}"`), `Missing guide #doc-${guide.id}`);
  for (const field of lmh25.fields) assert.ok(distHtml.includes(`id="field-${field.id}"`), `Missing field #field-${field.id}`);
  for (const key of lmh25.decisionKeys) assert.ok(distHtml.includes(`id="doc-${key.id}"`), `Missing key #doc-${key.id}`);
  assert.equal(dua.clauses.length, 9);
  for (let i = 1; i <= 9; i += 1) {
    assert.ok(distHtml.includes(`${i}. `), `Missing Data Use Agreement clause ${i}`);
  }

  // Populated manifest populates static <noscript> fallback links at build time while preserving unique IDs
  const populatedHtml = renderPortalHtml({
    guides,
    lmh25,
    whatsNew,
    shortcuts,
    dataUseAgreement: dua,
    provenance,
    manifest: manifestFixture(assetLinks, { includePreview: true }),
  });
  for (const name of Object.values(assetLinks)) {
    assert.ok(populatedHtml.includes(url(name, 'v2.0.0')), `Missing static noscript URL for ${name}`);
  }
  for (const id of Object.keys(assetLinks)) {
    assert.equal((populatedHtml.match(new RegExp(`id="${id}"`, 'g')) || []).length, 1, `Duplicate id=${id} when noscript populated`);
  }

  // Client-side documentation search & deep-link anchor reveal
  const cardA = makeElement('doc-migrating-from-vpro64');
  cardA.setAttribute('data-doc-kind', 'guide');
  cardA.setAttribute('data-search-text', 'Migrating from VPro 64 SQLite Access');
  const cardB = makeElement('field-soil.humus_depth');
  cardB.setAttribute('data-doc-kind', 'field');
  cardB.setAttribute('data-search-text', 'SOIL 4 Humus UpperDepth LowerDepth LMH25');
  const searchInput = makeElement('docs-search-input');
  const countEl = makeElement('docs-results-count');
  const emptyEl = makeElement('docs-empty-state');
  const clearBtn = makeElement('docs-search-clear');
  const docsDoc = {
    getElementById(id) {
      return {
        'docs-search-input': searchInput,
        'docs-results-count': countEl,
        'docs-empty-state': emptyEl,
        'docs-search-clear': clearBtn,
        'doc-migrating-from-vpro64': cardA,
        'field-soil.humus_depth': cardB,
      }[id] || null;
    },
    querySelectorAll(sel) {
      if (sel === '[data-doc-item]') return [cardA, cardB];
      return [];
    },
  };
  const searchCtrl = initPortalDocs(docsDoc, { location: { hash: '' } }, {});
  searchCtrl.setQuery('UpperDepth');
  assert.equal(cardA.hidden, true);
  assert.equal(cardB.hidden, false);
  assert.equal(searchCtrl.revealHashTarget('#doc-migrating-from-vpro64'), true);
  assert.equal(cardA.hidden, false);
  assert.equal(cardA.classList.contains('is-anchor-target'), true);

  // Missing required DOM element still rejects instead of passing vacuously
  const brokenDoc = documentFixture(assetLinks);
  brokenDoc.elements.delete('dl-win-arm64-installer');
  await assert.rejects(loadPortal(brokenDoc, {}, async () => response(manifestFixture(assetLinks))), /element .* missing/);
});
