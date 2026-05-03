#!/bin/sh
# Install the `ai` CLI from a GitHub release.
#
# Usage:
#   curl -fsSL https://ai.hra42.com/install | sh
#   curl -fsSL https://ai.hra42.com/install | VERSION=v0.1.0 sh
#   curl -fsSL https://ai.hra42.com/install | INSTALL_DIR=$HOME/.local/bin sh

set -eu

REPO="hra42/ai"
BINARY="ai"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

err() {
	printf '\033[1;31merror:\033[0m %s\n' "$1" >&2
	exit 1
}

info() {
	printf '\033[1;36m==>\033[0m %s\n' "$1"
}

have() {
	command -v "$1" >/dev/null 2>&1
}

detect_os() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$os" in
		darwin) echo darwin ;;
		linux)  echo linux ;;
		*) err "unsupported OS: $os (only darwin and linux are published)" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
		x86_64|amd64)  echo amd64 ;;
		arm64|aarch64) echo arm64 ;;
		*) err "unsupported architecture: $arch (only amd64 and arm64 are published)" ;;
	esac
}

# Download $1 to $2. Quiet on success, fails loudly.
download() {
	url=$1; out=$2
	if have curl; then
		# -f makes curl exit non-zero on 4xx/5xx, -L follows redirects.
		curl -fsSL --retry 3 --retry-delay 1 -o "$out" "$url" || return 1
	elif have wget; then
		wget -qO "$out" "$url" || return 1
	else
		err "need curl or wget"
	fi
}

# HEAD request: returns 0 if URL exists (HTTP 200), non-zero otherwise.
url_exists() {
	url=$1
	if have curl; then
		curl -fsSI -o /dev/null "$url"
	elif have wget; then
		wget -q --spider "$url"
	else
		err "need curl or wget"
	fi
}

resolve_version() {
	if [ "$VERSION" != "latest" ]; then
		echo "$VERSION"
		return
	fi
	# /releases/latest 302-redirects to /releases/tag/<version>; we follow and read the URL.
	api_url="https://api.github.com/repos/$REPO/releases/latest"
	tmp=$(mktemp)
	if ! download "$api_url" "$tmp"; then
		rm -f "$tmp"
		err "no published release found at https://github.com/$REPO/releases — has the project been released yet?"
	fi
	# Extract "tag_name": "vX.Y.Z" without depending on jq.
	tag=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp" | head -n1)
	rm -f "$tmp"
	if [ -z "$tag" ]; then
		err "could not parse latest release tag from GitHub API"
	fi
	echo "$tag"
}

verify_checksum() {
	archive=$1; checksums=$2
	expected=$(grep " $(basename "$archive")\$" "$checksums" | awk '{print $1}')
	if [ -z "$expected" ]; then
		err "no checksum entry for $(basename "$archive") in checksums.txt"
	fi
	if have sha256sum; then
		actual=$(sha256sum "$archive" | awk '{print $1}')
	elif have shasum; then
		actual=$(shasum -a 256 "$archive" | awk '{print $1}')
	else
		err "need sha256sum or shasum to verify the download"
	fi
	if [ "$expected" != "$actual" ]; then
		err "checksum mismatch for $(basename "$archive"): expected $expected, got $actual"
	fi
}

install_binary() {
	src=$1; dest_dir=$2
	dest="$dest_dir/$BINARY"
	if [ -w "$dest_dir" ] || { [ ! -e "$dest_dir" ] && mkdir -p "$dest_dir" 2>/dev/null; }; then
		mv "$src" "$dest"
		chmod +x "$dest"
	elif have sudo; then
		info "writing to $dest_dir requires sudo"
		sudo mv "$src" "$dest"
		sudo chmod +x "$dest"
	else
		err "cannot write to $dest_dir and sudo is not available — set INSTALL_DIR to a writable location"
	fi
	echo "$dest"
}

main() {
	OS=$(detect_os)
	ARCH=$(detect_arch)
	TAG=$(resolve_version)

	# Strip leading "v" from tag for the archive filename if needed; goreleaser uses Os_Arch.
	ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
	BASE="https://github.com/$REPO/releases/download/$TAG"
	ARCHIVE_URL="$BASE/$ARCHIVE"
	CHECKSUM_URL="$BASE/checksums.txt"

	info "installing $BINARY $TAG ($OS/$ARCH)"

	if ! url_exists "$ARCHIVE_URL"; then
		err "no $OS/$ARCH binary in release $TAG ($ARCHIVE_URL) — check https://github.com/$REPO/releases for available platforms"
	fi

	tmpdir=$(mktemp -d)
	trap 'rm -rf "$tmpdir"' EXIT

	info "downloading $ARCHIVE"
	download "$ARCHIVE_URL" "$tmpdir/$ARCHIVE" || err "failed to download $ARCHIVE_URL"

	info "verifying checksum"
	download "$CHECKSUM_URL" "$tmpdir/checksums.txt" || err "failed to download checksums.txt"
	verify_checksum "$tmpdir/$ARCHIVE" "$tmpdir/checksums.txt"

	info "extracting"
	tar -xzf "$tmpdir/$ARCHIVE" -C "$tmpdir"
	if [ ! -f "$tmpdir/$BINARY" ]; then
		err "archive did not contain a '$BINARY' binary at the top level"
	fi

	dest=$(install_binary "$tmpdir/$BINARY" "$INSTALL_DIR")
	info "installed $dest"

	# Sanity check.
	if "$dest" --version >/dev/null 2>&1; then
		"$dest" --version
	else
		info "(could not run $dest --version — check your shell PATH)"
	fi

	# PATH hint.
	case ":$PATH:" in
		*":$INSTALL_DIR:"*) ;;
		*) info "note: $INSTALL_DIR is not in your \$PATH — add it to your shell rc to use '$BINARY' directly" ;;
	esac
}

main "$@"
