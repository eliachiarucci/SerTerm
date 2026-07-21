#!/bin/sh
# Install script for serterm.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/eliachiarucci/serterm/main/install.sh | sh
#
# Environment variables:
#   SERTERM_VERSION  - version to install (default: latest release)
#   SERTERM_INSTALL  - install directory (default: the directory of the
#                      existing install, if any; otherwise /usr/local/bin,
#                      or ~/.local/bin if not writable)

set -eu

REPO="eliachiarucci/serterm"

main() {
    os=$(detect_os)
    arch=$(detect_arch)
    version="${SERTERM_VERSION:-$(latest_version)}"
    version="${version#v}"

    if [ "$os" = "windows" ]; then
        echo "On Windows, download the zip from:" >&2
        echo "  https://github.com/${REPO}/releases/latest" >&2
        exit 1
    fi

    install_dir="${SERTERM_INSTALL:-}"
    if [ -z "$install_dir" ]; then
        # Updates go to the directory of the existing install.
        existing=$(command -v serterm 2>/dev/null || true)
        if [ -n "$existing" ]; then
            install_dir=$(dirname "$existing")
        elif [ -w /usr/local/bin ]; then
            install_dir="/usr/local/bin"
        else
            install_dir="${HOME}/.local/bin"
        fi
    fi
    mkdir -p "$install_dir"
    if [ ! -w "$install_dir" ]; then
        echo "Cannot write to ${install_dir}." >&2
        echo "Rerun with sudo, or choose a directory with SERTERM_INSTALL=<dir>." >&2
        exit 1
    fi

    archive="serterm_${version}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/v${version}/${archive}"

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    echo "Downloading serterm v${version} (${os}/${arch})..."
    curl -fsSL "$url" -o "${tmp}/${archive}"
    tar -xzf "${tmp}/${archive}" -C "$tmp" serterm
    install -m 755 "${tmp}/serterm" "${install_dir}/serterm"

    echo "Installed serterm v${version} to ${install_dir}/serterm"
    case ":$PATH:" in
        *":${install_dir}:"*) ;;
        *) echo "Note: ${install_dir} is not in your PATH." ;;
    esac
}

detect_os() {
    case "$(uname -s)" in
        Linux) echo linux ;;
        Darwin) echo darwin ;;
        MINGW* | MSYS* | CYGWIN*) echo windows ;;
        *)
            echo "Unsupported OS: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64 | amd64) echo amd64 ;;
        aarch64 | arm64) echo arm64 ;;
        *)
            echo "Unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name":' | head -1 | cut -d '"' -f 4
}

main "$@"
