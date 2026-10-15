const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const portal = import('../../docs/downloads.mjs');
const url = name => `https://github.com/Boostao/vpro_wails/releases/download/v1.2.3/${name}`;
const response = release => ({ ok: true, status: 200, json: async () => release });
function documentFixture(links) {
  const elements = new Map();
  for (const id of ['detected-platform-name', 'primary-download-btn', 'primary-download-text',
    'release-version-tag', 'download-status', 'card-windows', 'card-linux', 'card-macos', ...Object.keys(links)]) {
    elements.set(id, { textContent: '', href: 'old-incorrect-asset', attributes: {}, classes: [],
      setAttribute(name, value) { this.attributes[name] = value; },
      removeAttribute(name) { delete this.attributes[name]; if (name === 'href') delete this.href; },
      classList: { add(...names) { elements.get(id).classes.push(...names); } },
    });
  }
  return { getElementById(id) { return elements.get(id); }, elements };
}
function releaseFixture(links) {
  return { tag_name: 'v1.2.3', draft: false, prerelease: false,
    assets: Object.values(links).map(name => ({ name, browser_download_url: url(name) })) };
}
test('architecture hints override emulated Windows UA; every supported and mobile platform is explicit', async () => {
  const { detectPlatform, recommendedAsset } = await portal;
  const rows = [
    [{ userAgent: 'Windows NT Win64 x64', platform: 'Win32',
      userAgentData: { platform: 'Windows', getHighEntropyValues: async () => ({ architecture: 'arm', bitness: '64' }) } }, 'windows', 'arm64', 'dl-win-arm64-installer'],
    [{ userAgent: 'Windows NT Win64 x64', platform: 'Win32' }, 'windows', 'amd64', 'dl-win-x64-installer'],
    [{ userAgent: 'Linux aarch64', platform: 'Linux' }, 'linux', 'arm64', 'dl-linux-arm64-appimage'],
    [{ userAgent: 'Linux x86_64', platform: 'Linux' }, 'linux', 'amd64', 'dl-linux-appimage'],
    [{ userAgent: 'Macintosh', platform: 'MacIntel' }, 'darwin', 'universal', 'dl-mac-dmg'],
    [{ userAgent: 'Windows NT', platform: 'Win32' }, 'windows', 'unknown', null],
    [{ userAgent: 'Android Linux aarch64', platform: 'Linux' }, 'unknown', 'unknown', null],
    [{ userAgent: 'iPhone', platform: 'iPhone' }, 'unknown', 'unknown', null],
    [{ userAgent: 'Macintosh', platform: 'MacIntel', maxTouchPoints: 5 }, 'unknown', 'unknown', null],
    [{ userAgent: '', platform: '' }, 'unknown', 'unknown', null],
  ];
  for (const [navigator, os, arch, asset] of rows) {
    const client = await detectPlatform(navigator);
    assert.equal(client.os, os); assert.equal(client.arch, arch); assert.equal(recommendedAsset(client), asset);
  }
  const rejected = await detectPlatform({ userAgent: 'Windows Win64 x64', userAgentData: {
    platform: 'Windows', getHighEntropyValues: async () => { throw Error('hints denied'); },
  } });
  assert.equal(rejected.arch, 'unknown'); assert.match(rejected.warning, /hints denied/);
  for (const hints of [{ architecture: 'x86', bitness: '32' }, { architecture: 'arm', bitness: '32' }, {}]) {
    const client = await detectPlatform({ userAgent: 'Windows Win64 x64', userAgentData: {
      platform: 'Windows', getHighEntropyValues: async () => hints,
    } });
    assert.equal(client.arch, 'unknown'); assert.equal(recommendedAsset(client), null);
  }
});
test('exact asset names prevent AMD64/ARM64 and executable/installer confusion in either response order', async () => {
  const { assetLinks, reviewRelease } = await portal;
  assert.equal(Object.keys(assetLinks).length, 15);
  for (const reversed of [false, true]) {
    const fixture = releaseFixture(assetLinks);
    if (reversed) fixture.assets.reverse();
    const review = reviewRelease(fixture);
    for (const [id, name] of Object.entries(assetLinks)) assert.equal(review.links[id], url(name));
    assert.notEqual(review.links['dl-win-x64-installer'], review.links['dl-win-arm64-installer']);
  }
});
test('portal names match the production release validator, including its checksum filename', async () => {
  const { assetLinks } = await portal;
  const result = spawnSync('python', ['-c',
    "import json, runpy; release=runpy.run_path('scripts/release.py'); print(json.dumps(sorted(release['REQUIRED'] | {release['MANIFEST']})))"],
  { cwd: path.join(__dirname, '..', '..'), encoding: 'utf8' });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(Object.values(assetLinks).sort(), JSON.parse(result.stdout));
});
test('malformed duplicate and unsafe asset metadata refuses rather than selecting first matches', async () => {
  const { assetLinks, reviewRelease } = await portal;
  for (const bad of [null, {}, { tag_name: '', assets: [] }, { tag_name: 'v1', assets: {}, draft: false },
    { ...releaseFixture(assetLinks), draft: true }, { ...releaseFixture(assetLinks), prerelease: true }]) {
    assert.throws(() => reviewRelease(bad));
  }
  const duplicate = releaseFixture(assetLinks); duplicate.assets.push(duplicate.assets[0]);
  assert.throws(() => reviewRelease(duplicate), /duplicate/);
  const malformed = releaseFixture(assetLinks); malformed.assets.push(null);
  assert.throws(() => reviewRelease(malformed), /missing/);
  for (const badURL of [undefined, 'javascript:alert(1)', 'https://example.com/package.exe',
    url(assetLinks['dl-win-x64-installer']) + '?redirect=evil',
    url(assetLinks['dl-win-x64-installer']) + '#fragment',
    url(assetLinks['dl-win-x64-installer']).replace('/v1.2.3/', '/v0.9.0/'),
    'https://github.com/Someone/another/releases/download/v1/' + assetLinks['dl-win-x64-installer']]) {
    const fixture = releaseFixture(assetLinks); fixture.assets[0].browser_download_url = badURL;
    assert.throws(() => reviewRelease(fixture));
  }
  for (const key of ['draft', 'prerelease']) {
    for (const value of [undefined, 'false', 0, null]) {
      assert.throws(() => reviewRelease({ ...releaseFixture(assetLinks), [key]: value }));
    }
  }
});
test('actual portal logic updates every link and primary button independently for every desktop architecture', async () => {
  const { assetLinks, loadPortal } = await portal;
  for (const [userAgent, platform, primary] of [
    ['Windows Win64 x64', 'Win32', 'dl-win-x64-installer'],
    ['Windows arm64', 'Win32', 'dl-win-arm64-installer'],
    ['Linux x86_64', 'Linux', 'dl-linux-appimage'],
    ['Linux aarch64', 'Linux', 'dl-linux-arm64-appimage'],
    ['Macintosh', 'MacIntel', 'dl-mac-dmg'],
  ]) {
    const document = documentFixture(assetLinks);
    await loadPortal(document, { userAgent, platform }, async (endpoint, options) => {
      assert.equal(endpoint, 'https://api.github.com/repos/Boostao/vpro_wails/releases/latest');
      assert.equal(options.headers.Accept, 'application/vnd.github+json');
      for (const id of Object.keys(assetLinks)) assert.equal(document.getElementById(id).href, undefined);
      return response(releaseFixture(assetLinks));
    });
    for (const [id, name] of Object.entries(assetLinks)) {
      assert.equal(document.getElementById(id).href, url(name));
      assert.equal(document.getElementById(id).attributes['aria-disabled'], undefined);
    }
    assert.equal(document.getElementById('primary-download-btn').href, url(assetLinks[primary]));
    assert.match(document.getElementById('release-version-tag').textContent, /v1.2.3/);
    assert.match(document.getElementById('download-status').textContent, /SHA256SUMS/);
  }
});
test('missing matching assets and unknown architecture keep browsing live but never recommend a different CPU', async () => {
  const { assetLinks, loadPortal } = await portal;
  const fixture = releaseFixture(assetLinks);
  fixture.assets = fixture.assets.filter(asset => !asset.name.includes('arm64'));
  const document = documentFixture(assetLinks);
  await loadPortal(document, { userAgent: 'Windows arm64', platform: 'Win32' }, async () => response(fixture));
  assert.equal(document.getElementById('primary-download-btn').href, '#downloads');
  assert.equal(document.getElementById('dl-win-arm64-installer').href, undefined);
  assert.equal(document.getElementById('dl-win-arm64-installer').attributes['aria-disabled'], 'true');
  assert.equal(document.getElementById('dl-win-x64-installer').href, url(assetLinks['dl-win-x64-installer']));
  assert.match(document.getElementById('download-status').textContent, /unavailable/);
  const unknown = documentFixture(assetLinks);
  await loadPortal(unknown, { userAgent: 'Windows NT', platform: 'Win32' }, async () => response(releaseFixture(assetLinks)));
  assert.equal(unknown.getElementById('primary-download-btn').href, '#downloads');
  assert.match(unknown.getElementById('download-status').textContent, /Architecture not detected/);
});
test('unpublished HTTP and transport failures are visible without stale download targets', async () => {
  const { assetLinks, loadPortal, releasesURL } = await portal;
  for (const fetch of [
    async () => ({ ok: false, status: 404 }),
    async () => ({ ok: false, status: 403 }),
    async () => { throw Error('network offline'); },
    async () => response(null),
  ]) {
    const document = documentFixture(assetLinks);
    await loadPortal(document, { userAgent: 'Windows Win64', platform: 'Win32' }, fetch);
    assert.equal(document.getElementById('primary-download-btn').href, releasesURL);
    assert.equal(document.getElementById('release-version-tag').textContent, 'Release unavailable');
    assert.match(document.getElementById('download-status').textContent, /unavailable|failed|offline|malformed|no published/i);
    for (const id of Object.keys(assetLinks)) assert.equal(document.getElementById(id).href, undefined);
  }
});
test('every planned HTML selector exists exactly once and missing controls cannot pass vacuously', async () => {
  const { assetLinks, loadPortal } = await portal;
  const html = readFileSync(path.join(__dirname, '..', '..', 'docs', 'index.html'), 'utf8');
  for (const id of [...Object.keys(assetLinks), 'download-status']) {
    assert.equal((html.match(new RegExp(`id="${id}"`, 'g')) || []).length, 1, id);
  }
  assert.match(html, /<script type="module" src="\.\/downloads\.mjs"><\/script>/);
  assert.match(html, /sha256sum/); assert.match(html, /Get-FileHash/); assert.match(html, /shasum -a 256/);
  const document = documentFixture(assetLinks); document.elements.delete('dl-win-arm64-installer');
  await assert.rejects(loadPortal(document, {}, async () => response(releaseFixture(assetLinks))), /element .* missing/);
});
