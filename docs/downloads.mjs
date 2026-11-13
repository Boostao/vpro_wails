export const repository = 'Boostao/vpro_wails';
export const releasesURL = `https://github.com/${repository}/releases`;
export const manifestPath = './release-manifest.json';

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

const knownAssetNames = new Set(Object.values(assetLinks));
const semverPattern = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;
const isoDatePattern = /^\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)?$/;
const sha256Pattern = /^[0-9a-f]{64}$/;

export function formatBytes(bytes) {
  if (!Number.isSafeInteger(bytes) || bytes <= 0) {
    throw new Error(`Invalid asset sizeBytes: ${String(bytes)}`);
  }
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB'];
  let value = bytes / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value >= 100 ? value.toFixed(0) : value.toFixed(1)} ${units[unitIndex]}`;
}

function normalizePlatformToken(value) {
  const token = String(value || '').trim().toLowerCase();
  return token === 'unknown' ? '' : token;
}

function parseHintArch(hints) {
  if (!hints || typeof hints !== 'object') return null;
  const rawBitness = String(hints.bitness ?? '').trim();
  const rawArch = String(hints.architecture ?? '').trim().toLowerCase();
  if (rawBitness === '32') {
    return {
      arch: 'unknown',
      warning: '32-bit operating systems are not supported; choose a 64-bit package below.',
    };
  }
  if (!rawArch) {
    return null;
  }
  if (/^(?:arm|arm64|aarch64|armv8[a-z]*)$/.test(rawArch) && (!rawBitness || rawBitness === '64')) {
    return { arch: 'arm64', warning: '' };
  }
  if (/^(?:x86|x86[-_]64|amd64|x64)$/.test(rawArch) && (!rawBitness || rawBitness === '64')) {
    return { arch: 'amd64', warning: '' };
  }
  return {
    arch: 'unknown',
    warning: `Unsupported CPU architecture (${rawArch}${rawBitness ? `/${rawBitness}-bit` : ''}); choose a package below.`,
  };
}

export function formatPlatformName(client) {
  if (!client || client.os === 'unknown') return 'No supported desktop platform detected';
  if (client.os === 'darwin') return 'macOS (Universal — Apple Silicon & Intel)';
  const osLabel = client.os === 'windows' ? 'Windows' : 'Linux';
  const archLabel = client.arch === 'arm64' ? 'ARM64' : client.arch === 'amd64' ? 'x86_64' : 'architecture unknown';
  return `${osLabel} (${archLabel})`;
}

export async function detectPlatform(navigator) {
  const ua = String(navigator?.userAgent || '').toLowerCase();
  const navPlatform = normalizePlatformToken(navigator?.platform);
  const oscpu = normalizePlatformToken(navigator?.oscpu);
  const uaDataPlatform = normalizePlatformToken(navigator?.userAgentData?.platform);

  let hints = null;
  let hintsError = null;
  if (navigator?.userAgentData?.getHighEntropyValues) {
    try {
      hints = await navigator.userAgentData.getHighEntropyValues(['platform', 'architecture', 'bitness']);
    } catch (error) {
      hintsError = error;
    }
  }
  const hintPlatform = normalizePlatformToken(hints?.platform);
  const allSignals = [hintPlatform, uaDataPlatform, navPlatform, oscpu, ua].filter(Boolean).join(' ');

  const isIPadDesktopMode = navigator?.userAgentData?.mobile !== true &&
    navigator?.userAgentData?.mobile !== false &&
    !uaDataPlatform &&
    /\bmac/.test(navPlatform) &&
    (navigator?.maxTouchPoints || 0) > 1 &&
    !/\b(?:firefox|electron)\b/.test(ua);

  if (navigator?.userAgentData?.mobile === true ||
      /\b(?:android|iphone|ipad|ipod|windows phone|iemobile|crios|fxios|edgios)\b/.test(allSignals) ||
      isIPadDesktopMode) {
    return { os: 'unknown', arch: 'unknown', warning: 'Desktop packages are not available for this mobile platform.' };
  }

  const structuredPlatform = hintPlatform || uaDataPlatform;
  let os = 'unknown';
  if (/^(?:macos|mac os x|mac|darwin)$/.test(structuredPlatform)) {
    os = 'darwin';
  } else if (/^win(?:dows|32|64)?$/.test(structuredPlatform)) {
    os = 'windows';
  } else if (/^(?:linux|chrome os|chromium os|ubuntu|fedora|debian|arch|opensuse|rhel)$/.test(structuredPlatform)) {
    os = 'linux';
  } else if (/\b(?:macintosh|mac os x|macintel|macppc|mac_powerpc|darwin)\b/.test(allSignals)) {
    os = 'darwin';
  } else if (/\b(?:windows|win32|win64|wow64)\b/.test(allSignals)) {
    os = 'windows';
  } else if (/\b(?:linux|cros)\b/.test(allSignals)) {
    os = 'linux';
  }

  if (os === 'unknown') {
    return { os: 'unknown', arch: 'unknown', warning: '' };
  }
  if (os === 'darwin') {
    return { os: 'darwin', arch: 'universal', warning: '' };
  }

  const archSignals = [navPlatform, oscpu, ua].filter(Boolean).join(' ');
  const has64Token = /\b(?:x86[-_]64|amd64|win64|wow64|x64|arm64|aarch64|armv8[a-z]*)\b/.test(archSignals);
  const has32OnlyToken = !has64Token && /\b(?:i[3456]86|armv[67][a-z]*|win16)\b/.test(archSignals);

  let arch = 'unknown';
  let privacyArchNote = '';
  if (!has32OnlyToken) {
    if (/\b(?:arm64|aarch64|armv8[a-z]*)\b/.test(archSignals)) {
      arch = 'arm64';
    } else if (/\b(?:x86[-_]64|amd64|win64|wow64|x64)\b/.test(archSignals)) {
      arch = 'amd64';
    } else if (os === 'windows' && /\bwindows nt 10\.0\b/.test(archSignals)) {
      arch = 'amd64';
      privacyArchNote = 'Browser privacy settings hid CPU architecture (defaulted to Windows x86_64; choose ARM64 if on Snapdragon).';
    }
  }

  const parsedHint = parseHintArch(hints);
  if (parsedHint) {
    return { os, arch: parsedHint.arch, warning: parsedHint.warning };
  }

  if (arch === 'unknown') {
    const reason = hintsError
      ? `Architecture detection unavailable: ${String(hintsError)}. Choose a package below.`
      : has32OnlyToken
        ? '32-bit operating systems are not supported; choose a 64-bit package below.'
        : 'Architecture not detected; choose a package below.';
    return { os, arch: 'unknown', warning: reason };
  }

  const fallbackWarning = privacyArchNote || (
    os === 'windows' && navigator?.userAgentData?.getHighEntropyValues
      ? 'Verify x86_64 vs. ARM64 above if running Windows on ARM (Snapdragon).'
      : ''
  );
  return { os, arch, warning: fallbackWarning };
}

function downloadURL(value, name, tag) {
  if (typeof value !== 'string' || !value) throw new Error(`Release asset ${name} has no download URL.`);
  let url;
  try {
    url = new URL(value);
  } catch {
    throw new Error(`Release asset ${name} has a malformed download URL.`);
  }
  const segments = url.pathname.split('/');
  if (url.origin !== 'https://github.com' || url.username || url.password || url.search || url.hash ||
      !url.pathname.toLowerCase().startsWith('/boostao/vpro_wails/releases/download/') ||
      segments.length !== 7 || segments[6] !== name ||
      decodeURIComponent(segments[5]) !== tag) {
    throw new Error(`Release asset ${name} has an unexpected download URL.`);
  }
  return url.href;
}

export function reviewChannelRelease(channelRelease, expectedChannel = 'stable') {
  if (!channelRelease || typeof channelRelease !== 'object' || Array.isArray(channelRelease)) {
    throw new Error(`Release metadata for ${expectedChannel} channel is malformed.`);
  }
  const { tag, version, publishedAt, prerelease, unsignedNotice, sha256sumsUrl, standards, assets } = channelRelease;
  if (typeof version !== 'string' || !semverPattern.test(version)) {
    throw new Error(`Channel ${expectedChannel} has an invalid SemVer version.`);
  }
  if (typeof tag !== 'string' || tag !== `v${version}`) {
    throw new Error(`Channel ${expectedChannel} tag must equal v${version}.`);
  }
  if (typeof publishedAt !== 'string' || !isoDatePattern.test(publishedAt) || Number.isNaN(Date.parse(publishedAt))) {
    throw new Error(`Channel ${expectedChannel} has an invalid publishedAt timestamp.`);
  }
  const expectedPrerelease = expectedChannel === 'preview';
  if (typeof prerelease !== 'boolean' || prerelease !== expectedPrerelease) {
    throw new Error(`Channel ${expectedChannel} prerelease flag must be ${String(expectedPrerelease)}.`);
  }
  if (typeof unsignedNotice !== 'boolean') {
    throw new Error(`Channel ${expectedChannel} unsignedNotice flag must be boolean.`);
  }
  const validatedSha256Url = downloadURL(sha256sumsUrl, 'SHA256SUMS.txt', tag);
  if (!standards || typeof standards !== 'object' || Array.isArray(standards)) {
    throw new Error(`Channel ${expectedChannel} standards metadata is missing.`);
  }
  for (const key of ['becVersion', 'taxonomyVersion', 'lmh25Edition']) {
    if (typeof standards[key] !== 'string' || !standards[key].trim()) {
      throw new Error(`Channel ${expectedChannel} standards.${key} is missing.`);
    }
  }
  if (!Array.isArray(assets)) {
    throw new Error(`Channel ${expectedChannel} assets must be an array.`);
  }
  const assetMap = new Map();
  for (const asset of assets) {
    if (!asset || typeof asset !== 'object' || Array.isArray(asset) ||
        typeof asset.name !== 'string' || !asset.name) {
      throw new Error('Release assets contain missing or malformed entries.');
    }
    if (!knownAssetNames.has(asset.name)) {
      throw new Error(`Release assets contain unexpected file ${asset.name}.`);
    }
    if (assetMap.has(asset.name)) {
      throw new Error(`Release assets contain duplicate name ${asset.name}.`);
    }
    const validatedUrl = downloadURL(asset.url, asset.name, tag);
    if (typeof asset.sha256 !== 'string' || !sha256Pattern.test(asset.sha256)) {
      throw new Error(`Release asset ${asset.name} has an invalid SHA-256 digest.`);
    }
    const formattedSize = formatBytes(asset.sizeBytes);
    assetMap.set(asset.name, {
      name: asset.name,
      url: validatedUrl,
      sha256: asset.sha256,
      sizeBytes: asset.sizeBytes,
      formattedSize,
    });
  }
  if (assetMap.has('SHA256SUMS.txt') && assetMap.get('SHA256SUMS.txt').url !== validatedSha256Url) {
    throw new Error('SHA256SUMS.txt asset URL disagrees with sha256sumsUrl.');
  }
  const links = {};
  const metadata = {};
  for (const [id, name] of Object.entries(assetLinks)) {
    if (assetMap.has(name)) {
      const entry = assetMap.get(name);
      links[id] = entry.url;
      metadata[id] = entry;
    } else if (name === 'SHA256SUMS.txt') {
      links[id] = validatedSha256Url;
      metadata[id] = null;
    } else {
      links[id] = null;
      metadata[id] = null;
    }
  }
  return {
    tag,
    version,
    publishedAt,
    prerelease,
    unsignedNotice,
    sha256sumsUrl: validatedSha256Url,
    standards: {
      becVersion: standards.becVersion,
      taxonomyVersion: standards.taxonomyVersion,
      lmh25Edition: standards.lmh25Edition,
    },
    links,
    metadata,
  };
}

export function reviewManifest(manifest) {
  if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) {
    throw new Error('Release manifest is malformed or unavailable.');
  }
  const allowedTopKeys = new Set(['schemaVersion', 'repository', 'defaultChannel', 'channels']);
  for (const key of Object.keys(manifest)) {
    if (!allowedTopKeys.has(key)) {
      throw new Error(`Release manifest contains unexpected key ${key}.`);
    }
  }
  if (manifest.schemaVersion !== 1) {
    throw new Error(`Unsupported release-manifest.json schemaVersion: ${String(manifest.schemaVersion)}`);
  }
  if (manifest.repository !== repository) {
    throw new Error(`Unexpected release-manifest.json repository: ${String(manifest.repository)}`);
  }
  if (manifest.defaultChannel !== 'stable' && manifest.defaultChannel !== 'preview') {
    throw new Error('Release manifest defaultChannel must be "stable" or "preview".');
  }
  if (!manifest.channels || typeof manifest.channels !== 'object' || Array.isArray(manifest.channels) ||
      !('stable' in manifest.channels)) {
    throw new Error('Release manifest channels object must define "stable".');
  }
  for (const key of Object.keys(manifest.channels)) {
    if (key !== 'stable' && key !== 'preview') {
      throw new Error(`Release manifest contains unsupported channel ${key}.`);
    }
  }
  const stable = manifest.channels.stable === null
    ? null
    : reviewChannelRelease(manifest.channels.stable, 'stable');
  const preview = manifest.channels.preview === undefined || manifest.channels.preview === null
    ? null
    : reviewChannelRelease(manifest.channels.preview, 'preview');
  return {
    schemaVersion: 1,
    repository,
    defaultChannel: manifest.defaultChannel,
    channels: { stable, preview },
  };
}

export function reviewRelease(release) {
  if (release && typeof release === 'object' && 'tag' in release && 'sha256sumsUrl' in release) {
    return reviewChannelRelease(release, release.prerelease === true ? 'preview' : 'stable');
  }
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
  return { version: release.tag_name, tag: release.tag_name, links, metadata: {} };
}

export function recommendedAsset(client) {
  if (client.os === 'darwin') return 'dl-mac-dmg';
  if (client.os === 'windows') return client.arch === 'arm64' ? 'dl-win-arm64-installer'
    : client.arch === 'amd64' ? 'dl-win-x64-installer' : null;
  if (client.os === 'linux') return client.arch === 'arm64' ? 'dl-linux-arm64-appimage'
    : client.arch === 'amd64' ? 'dl-linux-appimage' : null;
  return null;
}

function shortSha(sha) {
  return `${sha.slice(0, 8)}...${sha.slice(-6)}`;
}

async function copyDigest(navigator, text, button, statusEl) {
  if (!text) return false;
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    if (button) button.textContent = 'Copied SHA-256';
    if (statusEl) statusEl.textContent = `Copied SHA-256 digest ${shortSha(text)} to clipboard.`;
    return true;
  }
  return false;
}

export async function loadPortal(document, navigator, fetch, options = {}) {
  const required = id => {
    const element = document.getElementById(id);
    if (!element) throw new Error(`Download portal element ${id} is missing.`);
    return element;
  };
  const optional = id => document.getElementById(id) || null;

  const platform = required('detected-platform-name');
  const primary = required('primary-download-btn');
  const primaryText = required('primary-download-text');
  const version = required('release-version-tag');
  const status = required('download-status');
  const cards = {
    windows: required('card-windows'),
    macos: required('card-macos'),
    linux: required('card-linux'),
  };
  const linkElements = Object.fromEntries(Object.keys(assetLinks).map(id => [id, required(id)]));

  const channelButtons = {
    stable: optional('channel-btn-stable'),
    preview: optional('channel-btn-preview'),
  };
  const platformLabelEl = optional('detected-platform-label');
  const platformTargets = [
    ['windows', 'amd64', optional('plat-btn-win-amd64')],
    ['windows', 'arm64', optional('plat-btn-win-arm64')],
    ['darwin', 'universal', optional('plat-btn-mac-universal')],
    ['linux', 'amd64', optional('plat-btn-linux-amd64')],
    ['linux', 'arm64', optional('plat-btn-linux-arm64')],
  ];
  const publishedDateEl = optional('release-published-date');
  const channelBadgeEl = optional('release-channel-badge');
  const primaryMetaEl = optional('primary-download-meta');
  const primaryShaRowEl = optional('primary-sha256-row');
  const primaryShaValueEl = optional('primary-sha256-value');
  const primaryShaCopyEl = optional('primary-sha256-copy');
  const becVersionEl = optional('standard-bec-version');
  const taxonomyVersionEl = optional('standard-taxonomy-version');
  const lmh25VersionEl = optional('standard-lmh25-version');

  const markUnavailable = (id, channelLabel = 'selected') => {
    const element = linkElements[id];
    element.removeAttribute('href');
    element.setAttribute('aria-disabled', 'true');
    element.setAttribute('title', `This package is not available in the ${channelLabel} release channel.`);
    const sizeEl = optional(`${id}-size`);
    if (sizeEl) sizeEl.textContent = 'Unavailable';
    const shaEl = optional(`${id}-sha256`);
    if (shaEl) {
      shaEl.textContent = 'SHA-256 pending';
      shaEl.removeAttribute('data-sha256');
      shaEl.removeAttribute('title');
    }
    const copyEl = optional(`${id}-copy`);
    if (copyEl) {
      copyEl.setAttribute('aria-disabled', 'true');
      copyEl.disabled = true;
      copyEl.removeAttribute('data-sha256');
      copyEl.textContent = 'Copy SHA-256';
    }
  };

  for (const id of Object.keys(assetLinks)) markUnavailable(id, 'latest');
  primary.href = '#downloads';
  primaryText.textContent = 'Browse All Downloads';
  if (primaryMetaEl) primaryMetaEl.textContent = '';
  if (primaryShaValueEl) primaryShaValueEl.textContent = '';
  if (primaryShaRowEl) primaryShaRowEl.hidden = true;

  const detectedClient = await detectPlatform(navigator);
  let activeClient = { ...detectedClient };

  const syncPlatformSelectionUI = isManualOverride => {
    if (platformLabelEl) {
      platformLabelEl.textContent = isManualOverride ? 'Selected workstation:' : 'Detected workstation:';
    }
    platform.textContent = formatPlatformName(activeClient);
    for (const card of Object.values(cards)) {
      card.classList?.remove?.('is-detected-platform');
    }
    if (activeClient.os !== 'unknown') {
      const targetCard = cards[activeClient.os === 'darwin' ? 'macos' : activeClient.os];
      targetCard?.classList?.add?.('is-detected-platform');
    }
    for (const [osName, archName, btn] of platformTargets) {
      if (!btn) continue;
      const selected = activeClient.os === osName && activeClient.arch === archName;
      btn.setAttribute('aria-pressed', selected ? 'true' : 'false');
      btn.classList?.toggle?.('is-active', selected);
    }
  };

  syncPlatformSelectionUI(false);
  status.textContent = activeClient.warning || 'Loading verified release manifest...';

  const endpoint = options.manifestUrl || manifestPath;
  try {
    const response = await fetch(endpoint, {
      cache: 'no-cache',
      headers: { Accept: 'application/json' },
    });
    if (!response || !response.ok) {
      const code = response ? response.status : 'offline';
      throw new Error(code === 404
        ? 'No published release manifest is available yet.'
        : `Release manifest lookup failed (HTTP ${code}).`);
    }
    const reviewed = reviewManifest(await response.json());

    let activeChannel = options.channel || reviewed.defaultChannel;
    if (!options.channel && !reviewed.channels[activeChannel]) {
      const fallback = activeChannel === 'stable' ? 'preview' : 'stable';
      if (reviewed.channels[fallback]) activeChannel = fallback;
    }

    const updateChannelToggleUI = selected => {
      for (const name of ['stable', 'preview']) {
        const btn = channelButtons[name];
        if (!btn) continue;
        const available = Boolean(reviewed.channels[name]);
        const isSelected = name === selected;
        btn.setAttribute('aria-pressed', isSelected ? 'true' : 'false');
        if (btn.classList?.toggle) {
          btn.classList.toggle('is-active', isSelected);
          btn.classList.toggle('is-empty', !available);
        } else if (isSelected && btn.classList?.add) {
          btn.classList.add('is-active');
        }
        if (!available) {
          btn.setAttribute('title', `No ${name} release is currently published.`);
        } else {
          btn.removeAttribute('title');
        }
      }
    };

    const applyChannel = channelName => {
      if (channelName !== 'stable' && channelName !== 'preview') {
        throw new Error(`Unsupported channel: ${String(channelName)}`);
      }
      activeChannel = channelName;
      updateChannelToggleUI(channelName);
      for (const id of Object.keys(assetLinks)) markUnavailable(id, channelName);

      const release = reviewed.channels[channelName];
      if (!release) {
        version.textContent = 'Release unavailable';
        if (publishedDateEl) publishedDateEl.textContent = '';
        if (channelBadgeEl) channelBadgeEl.textContent = channelName === 'preview' ? 'Preview / UAT (None Published)' : 'Stable (None Published)';
        if (primaryMetaEl) primaryMetaEl.textContent = '';
        if (primaryShaRowEl) primaryShaRowEl.hidden = true;
        primary.href = releasesURL;
        primaryText.textContent = 'View Release Status';
        status.textContent = `No published ${channelName} release is available yet. Browse published releases for details.`;
        return;
      }

      version.textContent = channelName === 'preview'
        ? `Preview / UAT Release: ${release.tag}`
        : `Latest Release: ${release.tag}`;
      if (publishedDateEl) {
        publishedDateEl.textContent = `Published ${release.publishedAt.slice(0, 10)}`;
      }
      if (channelBadgeEl) {
        channelBadgeEl.textContent = channelName === 'preview'
          ? 'Preview / UAT Pre-release'
          : 'Official Stable Channel';
      }
      if (becVersionEl) becVersionEl.textContent = release.standards.becVersion;
      if (taxonomyVersionEl) taxonomyVersionEl.textContent = release.standards.taxonomyVersion;
      if (lmh25VersionEl) lmh25VersionEl.textContent = release.standards.lmh25Edition;

      for (const [id, url] of Object.entries(release.links)) {
        if (!url) continue;
        const element = linkElements[id];
        element.href = url;
        element.removeAttribute('aria-disabled');
        element.removeAttribute('title');
        const meta = release.metadata[id];
        if (meta) {
          const sizeEl = optional(`${id}-size`);
          if (sizeEl) sizeEl.textContent = meta.formattedSize;
          const shaEl = optional(`${id}-sha256`);
          if (shaEl) {
            shaEl.textContent = shortSha(meta.sha256);
            shaEl.setAttribute('data-sha256', meta.sha256);
            shaEl.setAttribute('title', `SHA-256: ${meta.sha256}`);
          }
          const copyEl = optional(`${id}-copy`);
          if (copyEl) {
            copyEl.disabled = false;
            copyEl.removeAttribute('aria-disabled');
            copyEl.setAttribute('data-sha256', meta.sha256);
            copyEl.textContent = 'Copy SHA-256';
          }
        }
      }

      const recommended = recommendedAsset(activeClient);
      if (recommended && release.links[recommended]) {
        primary.href = release.links[recommended];
        primaryText.textContent = activeClient.os === 'windows' ? `Download Windows ${activeClient.arch} installer`
          : activeClient.os === 'darwin' ? 'Download macOS Universal (.dmg)' : `Download Linux ${activeClient.arch} AppImage`;
        const recMeta = release.metadata[recommended];
        if (primaryMetaEl) {
          primaryMetaEl.textContent = recMeta ? `${recMeta.name} (${recMeta.formattedSize})` : assetLinks[recommended];
        }
        if (recMeta && primaryShaValueEl && primaryShaCopyEl) {
          primaryShaValueEl.textContent = shortSha(recMeta.sha256);
          primaryShaValueEl.setAttribute('title', recMeta.sha256);
          primaryShaCopyEl.setAttribute('data-sha256', recMeta.sha256);
          primaryShaCopyEl.textContent = 'Copy SHA-256';
          if (primaryShaRowEl) primaryShaRowEl.hidden = false;
        } else if (primaryShaRowEl) {
          primaryShaRowEl.hidden = true;
        }
        const unsignedHint = release.unsignedNotice
          ? 'Phase 1 unsigned build — verify SHA-256 against SHA256SUMS.txt before installation.'
          : 'Verify your download against SHA256SUMS.txt before installation.';
        status.textContent = activeClient.warning ? `${activeClient.warning} ${unsignedHint}` : unsignedHint;
      } else {
        primary.href = '#downloads';
        primaryText.textContent = 'Browse All Downloads';
        if (primaryMetaEl) primaryMetaEl.textContent = '';
        if (primaryShaRowEl) primaryShaRowEl.hidden = true;
        status.textContent = activeClient.warning || 'The matching package is unavailable in this release; choose an available package below.';
      }
      if (Object.values(release.links).some(url => !url)) {
        status.textContent += ' Some release assets are unavailable; their links are disabled.';
      }
    };

    const setPlatform = (osName, archName) => {
      const normalizedArch = osName === 'darwin' ? 'universal' : archName;
      if (!['windows', 'darwin', 'linux'].includes(osName) ||
          (osName === 'darwin' ? normalizedArch !== 'universal' : !['amd64', 'arm64'].includes(normalizedArch))) {
        throw new Error(`Unsupported platform target: ${String(osName)}/${String(archName)}`);
      }
      const matchesDetected = detectedClient.os === osName && detectedClient.arch === normalizedArch;
      activeClient = {
        os: osName,
        arch: normalizedArch,
        warning: matchesDetected ? detectedClient.warning : '',
      };
      syncPlatformSelectionUI(!matchesDetected);
      applyChannel(activeChannel);
    };

    applyChannel(activeChannel);

    for (const [osName, archName, btn] of platformTargets) {
      if (btn?.addEventListener) {
        btn.addEventListener('click', () => setPlatform(osName, archName));
      }
    }
    for (const name of ['stable', 'preview']) {
      const btn = channelButtons[name];
      if (btn?.addEventListener) {
        btn.addEventListener('click', () => applyChannel(name));
      }
    }
    if (primaryShaCopyEl?.addEventListener) {
      primaryShaCopyEl.addEventListener('click', () => {
        const digest = primaryShaCopyEl.getAttribute?.('data-sha256') || primaryShaCopyEl.attributes?.['data-sha256'];
        void copyDigest(navigator, digest, primaryShaCopyEl, status);
      });
    }
    for (const id of Object.keys(assetLinks)) {
      const copyEl = optional(`${id}-copy`);
      if (copyEl?.addEventListener) {
        copyEl.addEventListener('click', () => {
          const digest = copyEl.getAttribute?.('data-sha256') || copyEl.attributes?.['data-sha256'];
          void copyDigest(navigator, digest, copyEl, status);
        });
      }
    }

    return {
      client: detectedClient,
      getPlatform: () => ({ ...activeClient }),
      setPlatform,
      manifest: reviewed,
      getChannel: () => activeChannel,
      setChannel: applyChannel,
    };
  } catch (error) {
    for (const id of Object.keys(assetLinks)) markUnavailable(id, 'latest');
    version.textContent = 'Release unavailable';
    status.textContent = `${String(error)} Browse published releases for details.`;
    primary.href = releasesURL;
    primaryText.textContent = 'View Release Status';
    if (primaryMetaEl) primaryMetaEl.textContent = '';
    if (primaryShaRowEl) primaryShaRowEl.hidden = true;
    return null;
  }
}

if (typeof document !== 'undefined' && typeof globalThis.fetch === 'function') {
  loadPortal(document, navigator, globalThis.fetch.bind(globalThis)).catch(error => {
    console.error('Download portal initialization failed:', error);
    const status = document.getElementById('download-status');
    if (status) status.textContent = `Download portal initialization failed: ${String(error)}`;
  });
}
