export const repository = 'Boostao/vpro_wails';
export const releasesURL = `https://github.com/${repository}/releases`;
export const assetLinks = Object.freeze({
  'dl-win-x64-installer': 'vpro-windows-amd64-installer.exe',
  'dl-win-x64-exe': 'vpro-windows-amd64.exe',
  'dl-win-x64-zip': 'vpro-windows-amd64.zip',
  'dl-win-arm64-installer': 'vpro-windows-arm64-installer.exe',
  'dl-win-arm64-exe': 'vpro-windows-arm64.exe',
  'dl-win-arm64-zip': 'vpro-windows-arm64.zip',
  'dl-mac-dmg': 'vpro-macos-universal.dmg',
  'dl-mac-zip': 'vpro-macos-universal.zip',
  'dl-linux-appimage': 'vpro-linux-amd64.AppImage',
  'dl-linux-deb': 'vpro-linux-amd64.deb',
  'dl-linux-tar': 'vpro-linux-amd64.tar.gz',
  'dl-linux-arm64-appimage': 'vpro-linux-arm64.AppImage',
  'dl-linux-arm64-deb': 'vpro-linux-arm64.deb',
  'dl-linux-arm64-tar': 'vpro-linux-arm64.tar.gz',
  'dl-checksums': 'SHA256SUMS.txt',
});

export async function detectPlatform(navigator) {
  const ua = (navigator.userAgent || '').toLowerCase();
  const platform = (navigator.userAgentData?.platform || navigator.platform || '').toLowerCase();
  if (/android|iphone|ipad|ipod/.test(ua + ' ' + platform) ||
      /mac/.test(platform) && navigator.maxTouchPoints > 1) {
    return { os: 'unknown', arch: 'unknown', warning: 'Desktop packages are not available for this mobile platform.' };
  }
  const os = /win/.test(platform) || /windows/.test(ua) ? 'windows'
    : /mac/.test(platform) || /macintosh|mac os x/.test(ua) ? 'darwin'
    : /linux/.test(platform + ' ' + ua) ? 'linux' : 'unknown';
  if (os === 'darwin') return { os, arch: 'universal', warning: '' };
  let arch = /arm64|aarch64/.test(ua + ' ' + platform) ? 'arm64'
    : /x86_64|amd64|win64|x64/.test(ua + ' ' + platform) ? 'amd64' : 'unknown';
  if (navigator.userAgentData?.getHighEntropyValues) {
    try {
      const hints = await navigator.userAgentData.getHighEntropyValues(['architecture', 'bitness']);
      arch = hints.bitness === '64' && hints.architecture === 'arm' ? 'arm64'
        : hints.bitness === '64' && hints.architecture === 'x86' ? 'amd64' : 'unknown';
    } catch (error) {
      return { os, arch: 'unknown', warning: `Architecture detection unavailable: ${String(error)}. Choose a package below.` };
    }
  }
  return { os, arch, warning: arch === 'unknown' ? 'Architecture not detected; choose a package below.' : '' };
}

function downloadURL(value, name, tag) {
  if (typeof value !== 'string') throw new Error(`Release asset ${name} has no download URL.`);
  const url = new URL(value);
  if (url.origin !== 'https://github.com' || url.username || url.password || url.search || url.hash ||
      !url.pathname.toLowerCase().startsWith('/boostao/vpro_wails/releases/download/') ||
      !url.pathname.endsWith('/' + name) || url.pathname.split('/').length !== 7 ||
      decodeURIComponent(url.pathname.split('/')[5]) !== tag) {
    throw new Error(`Release asset ${name} has an unexpected download URL.`);
  }
  return url.href;
}

export function reviewRelease(release) {
  if (!release || typeof release.tag_name !== 'string' || !release.tag_name ||
      release.draft !== false || release.prerelease !== false || !Array.isArray(release.assets)) {
    throw new Error('Latest stable release metadata is malformed or unavailable.');
  }
  const assets = new Map();
  for (const asset of release.assets) {
    if (!asset || typeof asset.name !== 'string' || assets.has(asset.name)) {
      throw new Error('Release assets contain missing or duplicate names.');
    }
    assets.set(asset.name, asset.browser_download_url);
  }
  const links = {};
  for (const [id, name] of Object.entries(assetLinks)) {
    links[id] = assets.has(name) ? downloadURL(assets.get(name), name, release.tag_name) : null;
  }
  return { version: release.tag_name, links };
}

export function recommendedAsset(client) {
  if (client.os === 'darwin') return 'dl-mac-dmg';
  if (client.os === 'windows') return client.arch === 'arm64' ? 'dl-win-arm64-installer'
    : client.arch === 'amd64' ? 'dl-win-x64-installer' : null;
  if (client.os === 'linux') return client.arch === 'arm64' ? 'dl-linux-arm64-appimage'
    : client.arch === 'amd64' ? 'dl-linux-appimage' : null;
  return null;
}

export async function loadPortal(document, navigator, fetch) {
  const required = id => {
    const element = document.getElementById(id);
    if (!element) throw new Error(`Download portal element ${id} is missing.`);
    return element;
  };
  const platform = required('detected-platform-name');
  const primary = required('primary-download-btn');
  const primaryText = required('primary-download-text');
  const version = required('release-version-tag');
  const status = required('download-status');
  const linkElements = Object.fromEntries(Object.keys(assetLinks).map(id => [id, required(id)]));
  const unavailable = element => {
    element.removeAttribute('href');
    element.setAttribute('aria-disabled', 'true');
    element.setAttribute('title', 'This asset is not available in the latest stable release.');
  };
  for (const element of Object.values(linkElements)) unavailable(element);
  primary.href = '#downloads';
  primaryText.textContent = 'Browse All Downloads';
  const client = await detectPlatform(navigator);
  platform.textContent = client.os === 'darwin' ? 'macOS (Universal - Apple Silicon & Intel)'
    : client.os === 'unknown' ? 'No supported desktop platform detected'
    : `${client.os === 'windows' ? 'Windows' : 'Linux'} (${client.arch === 'arm64' ? 'ARM64' : client.arch === 'amd64' ? 'x86_64' : 'architecture unknown'})`;
  if (client.os !== 'unknown') {
    required(`card-${client.os === 'darwin' ? 'macos' : client.os}`).classList.add('ring-2', 'ring-forest-600', 'bg-forest-50/20');
  }
  status.textContent = client.warning || 'Loading latest stable release assets...';
  try {
    const response = await fetch(`https://api.github.com/repos/${repository}/releases/latest`, {
      headers: { Accept: 'application/vnd.github+json' },
    });
    if (!response.ok) throw new Error(response.status === 404 ? 'No published stable release is available yet.' : `Release lookup failed (HTTP ${response.status}).`);
    const release = reviewRelease(await response.json());
    version.textContent = `Latest Release: ${release.version}`;
    for (const [id, url] of Object.entries(release.links)) {
      if (!url) continue;
      linkElements[id].href = url;
      linkElements[id].removeAttribute('aria-disabled');
      linkElements[id].removeAttribute('title');
    }
    const recommended = recommendedAsset(client);
    if (recommended && release.links[recommended]) {
      primary.href = release.links[recommended];
      primaryText.textContent = client.os === 'windows' ? `Download Windows ${client.arch} installer`
        : client.os === 'darwin' ? 'Download macOS Universal (.dmg)' : `Download Linux ${client.arch} AppImage`;
      status.textContent = client.warning || 'Verify your download against SHA256SUMS.txt before installation.';
    } else {
      status.textContent = client.warning || 'The matching package is unavailable in this release; choose an available package below.';
    }
    if (Object.values(release.links).some(url => !url)) {
      status.textContent += ' Some release assets are unavailable; their links are disabled.';
    }
  } catch (error) {
    version.textContent = 'Release unavailable';
    status.textContent = `${String(error)} Browse published releases for details.`;
    primary.href = releasesURL;
    primaryText.textContent = 'View Release Status';
  }
}

if (typeof document !== 'undefined') {
  loadPortal(document, navigator, globalThis.fetch.bind(globalThis)).catch(error => {
    console.error('Download portal initialization failed:', error);
    const status = document.getElementById('download-status');
    if (status) status.textContent = `Download portal initialization failed: ${String(error)}`;
  });
}
