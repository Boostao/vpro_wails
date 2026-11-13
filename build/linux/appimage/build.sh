#!/usr/bin/env bash
# Copyright (c) 2018-Present Lea Anthony
# SPDX-License-Identifier: MIT

# Fail script on any error
set -euxo pipefail

APP_NAME="${APP_NAME:-VPro}"
APP_BINARY="${APP_BINARY:?APP_BINARY is required}"
ICON_PATH="${ICON_PATH:?ICON_PATH is required}"
DESKTOP_FILE="${DESKTOP_FILE:?DESKTOP_FILE is required}"
OUTPUT_DIR="${OUTPUT_DIR:-}"
BUILD_DIR="${BUILD_DIR:-$(pwd)}"

APP_BINARY="$(realpath "${APP_BINARY}")"
ICON_PATH="$(realpath "${ICON_PATH}")"
DESKTOP_FILE="$(realpath "${DESKTOP_FILE}")"
if [[ -n "${OUTPUT_DIR}" ]]; then
    mkdir -p "${OUTPUT_DIR}"
    OUTPUT_DIR="$(realpath "${OUTPUT_DIR}")"
fi
mkdir -p "${BUILD_DIR}"
BUILD_DIR="$(realpath "${BUILD_DIR}")"

if [[ "$(uname -m)" == *x86_64* ]]; then
    LINUXDEPLOY_ARCH="x86_64"
else
    LINUXDEPLOY_ARCH="aarch64"
fi

APP_DIR="${BUILD_DIR}/${APP_NAME}.AppDir"
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/usr/bin"

cp -f "${APP_BINARY}" "${APP_DIR}/usr/bin/${APP_NAME}"
chmod 0755 "${APP_DIR}/usr/bin/${APP_NAME}"
cp -f "${ICON_PATH}" "${APP_DIR}/.DirIcon"
cp -f "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp -f "${DESKTOP_FILE}" "${APP_DIR}/${APP_NAME}.desktop"

# Bundle WebKitGTK helper processes and injected bundle so WEBKIT_EXEC_PATH resolves inside the AppImage.
LD_OUT="$(ldd "${APP_BINARY}" || true)"
INJECTED_BUNDLE_NAME="libwebkitgtkinjectedbundle.so"
if [[ "${LD_OUT}" != *"libgtk-4.so"* ]] && [[ "${LD_OUT}" == *"libgtk-3.so"* ]]; then
    INJECTED_BUNDLE_NAME="libwebkit2gtkinjectedbundle.so"
fi

WEBKIT_EXEC_REL=""
WEBKIT_BUNDLE_REL=""
for helper in WebKitWebProcess WebKitNetworkProcess "${INJECTED_BUNDLE_NAME}"; do
    found_path="$(find /usr/lib /usr/lib64 /usr/libexec -type f -name "${helper}" 2>/dev/null | head -n 1 || true)"
    if [[ -n "${found_path}" ]]; then
        rel_dir="${found_path#/}"
        rel_dir="$(dirname "${rel_dir}")"
        mkdir -p "${APP_DIR}/${rel_dir}"
        cp -a "${found_path}" "${APP_DIR}/${rel_dir}/"
        if [[ "${helper}" == "WebKitWebProcess" ]]; then
            WEBKIT_EXEC_REL="${rel_dir}"
        elif [[ "${helper}" == "${INJECTED_BUNDLE_NAME}" ]]; then
            WEBKIT_BUNDLE_REL="${rel_dir}"
        fi
    fi
done

if [[ -d /usr/share/glib-2.0/schemas ]] && command -v glib-compile-schemas >/dev/null 2>&1; then
    mkdir -p "${APP_DIR}/usr/share/glib-2.0/schemas"
    cp -a /usr/share/glib-2.0/schemas/. "${APP_DIR}/usr/share/glib-2.0/schemas/"
    glib-compile-schemas "${APP_DIR}/usr/share/glib-2.0/schemas"
fi

# Custom AppRun: preserve native Wayland/X11 selection (never force GDK_BACKEND=x11)
# and honor or populate WEBKIT_EXEC_PATH for bundled WebKitGTK subprocesses.
# Stage outside APP_DIR so linuxdeploy --custom-apprun never unlinks its own source file.
CUSTOM_APPRUN="${BUILD_DIR}/AppRun.${APP_NAME}"
cat > "${CUSTOM_APPRUN}" <<EOF
#!/usr/bin/env bash
set -e
APPDIR="\${APPDIR:-\$(dirname "\$(readlink -f "\$0")")}"
export APPDIR
export PATH="\${APPDIR}/usr/bin:\${PATH:-/usr/bin:/bin}"
export LD_LIBRARY_PATH="\${APPDIR}/usr/lib:\${APPDIR}/usr/lib64:\${LD_LIBRARY_PATH:-}"
export XDG_DATA_DIRS="\${APPDIR}/usr/share:/usr/local/share:/usr/share:\${XDG_DATA_DIRS:-}"
if [[ -d "\${APPDIR}/usr/share/glib-2.0/schemas" ]]; then
    export GSETTINGS_SCHEMA_DIR="\${APPDIR}/usr/share/glib-2.0/schemas"
fi
if [[ -n "${WEBKIT_EXEC_REL}" ]] && [[ -d "\${APPDIR}/${WEBKIT_EXEC_REL}" ]]; then
    export WEBKIT_EXEC_PATH="\${WEBKIT_EXEC_PATH:-\${APPDIR}/${WEBKIT_EXEC_REL}}"
fi
if [[ -n "${WEBKIT_BUNDLE_REL}" ]] && [[ -d "\${APPDIR}/${WEBKIT_BUNDLE_REL}" ]]; then
    export WEBKIT_INJECTED_BUNDLE_PATH="\${WEBKIT_INJECTED_BUNDLE_PATH:-\${APPDIR}/${WEBKIT_BUNDLE_REL}}"
fi
if [[ -d "\${APPDIR}/apprun-hooks" ]]; then
    _VPRO_HAD_GDK_BACKEND="\${GDK_BACKEND+1}"
    _VPRO_SAVED_GDK_BACKEND="\${GDK_BACKEND:-}"
    for hook in "\${APPDIR}/apprun-hooks/"*.sh; do
        if [[ -f "\${hook}" ]]; then
            # shellcheck disable=SC1090
            source "\${hook}"
        fi
    done
    if [[ -z "\${_VPRO_HAD_GDK_BACKEND}" ]]; then
        unset GDK_BACKEND
    else
        export GDK_BACKEND="\${_VPRO_SAVED_GDK_BACKEND}"
    fi
fi
exec "\${APPDIR}/usr/bin/${APP_NAME}" "\$@"
EOF
chmod 0755 "${CUSTOM_APPRUN}"

cd "${BUILD_DIR}"
LINUXDEPLOY_BIN="${BUILD_DIR}/linuxdeploy-${LINUXDEPLOY_ARCH}.AppImage"
if [[ ! -x "${LINUXDEPLOY_BIN}" ]]; then
    LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${LINUXDEPLOY_ARCH}.AppImage"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${LINUXDEPLOY_URL}" -o "${LINUXDEPLOY_BIN}"
    else
        wget -q -4 -O "${LINUXDEPLOY_BIN}" "${LINUXDEPLOY_URL}"
    fi
    chmod +x "${LINUXDEPLOY_BIN}"
fi

# Disable linuxdeploy strip on modern toolchains with .relr.dyn sections.
for probe_lib in /usr/lib/libgtk-4.so.1 /usr/lib64/libgtk-4.so.1 /usr/lib/x86_64-linux-gnu/libgtk-4.so.1 /usr/lib/aarch64-linux-gnu/libgtk-4.so.1; do
    if [[ -f "${probe_lib}" ]] && command -v readelf >/dev/null 2>&1; then
        if readelf -S "${probe_lib}" 2>/dev/null | grep -q '\.relr\.dyn'; then
            export NO_STRIP=1
            break
        fi
    fi
done

rm -f "${BUILD_DIR}/${APP_NAME}"*.AppImage
"${LINUXDEPLOY_BIN}" --appimage-extract-and-run \
    --appdir "${APP_DIR}" \
    --custom-apprun "${CUSTOM_APPRUN}"

if [[ -d "${APP_DIR}/apprun-hooks" ]]; then
    for hook in "${APP_DIR}/apprun-hooks/"*.sh; do
        if [[ -f "${hook}" ]]; then
            sed -i '/^[[:space:]]*export[[:space:]]\+GDK_BACKEND=x11/d' "${hook}"
        fi
    done
fi

install -m 0755 "${CUSTOM_APPRUN}" "${APP_DIR}/AppRun"
cp -f "${ICON_PATH}" "${APP_DIR}/.DirIcon"
cp -f "${ICON_PATH}" "${APP_DIR}/${APP_NAME}.png"
cp -f "${DESKTOP_FILE}" "${APP_DIR}/${APP_NAME}.desktop"
chmod -R u+rwX,go+rX "${APP_DIR}"
chmod 0755 "${APP_DIR}/AppRun" "${APP_DIR}/usr/bin/${APP_NAME}"

"${LINUXDEPLOY_BIN}" --appimage-extract-and-run \
    --appdir "${APP_DIR}" \
    --custom-apprun "${CUSTOM_APPRUN}" \
    --output appimage
rm -f "${CUSTOM_APPRUN}"

# Rename the generated AppImage using an unquoted wildcard expansion.
shopt -s nullglob
generated_appimages=( "${APP_NAME}"*.AppImage )
shopt -u nullglob
if [[ "${#generated_appimages[@]}" -eq 0 ]]; then
    echo "Error: linuxdeploy produced no ${APP_NAME}*.AppImage output" >&2
    exit 1
fi
if [[ "${generated_appimages[0]}" != "${APP_NAME}.AppImage" ]]; then
    mv -f "${generated_appimages[0]}" "${APP_NAME}.AppImage"
fi

if [[ -n "${OUTPUT_DIR}" ]]; then
    rm -f "${OUTPUT_DIR}/${APP_NAME}"*.AppImage "${OUTPUT_DIR}/vpro"*.AppImage
    mv -f "${APP_NAME}.AppImage" "${OUTPUT_DIR}/${APP_NAME}-${LINUXDEPLOY_ARCH}.AppImage"
fi


