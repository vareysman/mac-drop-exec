#!/bin/sh
set -euo pipefail
# Manage admin-daemon LaunchDaemon (install/status/uninstall).

ROOT="${ROOT:-/usr/local/sbin}"
PLIST_SRC="$(dirname "$0")/com.admin.daemon.plist"
PLIST_DST="/Library/LaunchDaemons/com.admin.daemon.plist"
LABEL="com.admin.daemon"

usage() {
  echo "Usage: $0 [install|status|uninstall]"
}

detect_binary() {
  ARCH="$(uname -m)"
  case "$ARCH" in
    arm64)  BINARY="mac-drop-exec-darwin-arm64" ;;
    x86_64) BINARY="mac-drop-exec-darwin-amd64" ;;
    *)
      echo "Unsupported architecture: $ARCH" >&2
      exit 1
      ;;
  esac

  BINARY_PATH="$(dirname "$0")/$BINARY"
  if [ ! -f "$BINARY_PATH" ]; then
    echo "Binary not found: $BINARY_PATH" >&2
    echo "Download it from https://github.com/vareysman/mac-drop-exec/releases" >&2
    echo "or build from source: go build -o $BINARY ." >&2
    exit 1
  fi

  echo "$BINARY_PATH"
}

do_install() {
  BINARY_PATH="$(detect_binary)"

  install -d "$ROOT"
  install -m 755 "$BINARY_PATH" "$ROOT/admin-daemon"

  install -m 644 "$PLIST_SRC" "$PLIST_DST"
  launchctl bootout system "$PLIST_DST" 2>/dev/null || true
  launchctl bootstrap system "$PLIST_DST"
  launchctl enable "system/$LABEL"
  launchctl kickstart -k "system/$LABEL"

  echo "Installed. Create /tmp/xipt with the shell command to run; on failure read /tmp/xopt.0 (or /tmp/xopt.<worker> if ADMIN_DAEMON_PROC_NUM>1)."
}

show_daemons() {
  RESULT="/tmp/result.txt"
  rm -f "$RESULT" 2>/dev/null || true
  echo "whoami > $RESULT" > /tmp/xipt
  echo "Waiting 10 seconds for daemon response..."
  sleep 10
  if [ -f "$RESULT" ]; then
    echo "$LABEL: running (user: $(cat "$RESULT"))"
    rm -f "$RESULT" 2>/dev/null || true
  else
    echo "$LABEL: not responding"
  fi
}

do_uninstall() {
  launchctl bootout system "$PLIST_DST" 2>/dev/null || true
  launchctl disable "system/$LABEL" 2>/dev/null || true
  rm -f "$PLIST_DST" "$ROOT/admin-daemon"
  echo "Uninstalled $LABEL."
}

ACTION="${1:-install}"
case "$ACTION" in
  install)
    do_install
    ;;
  status)
    show_daemons
    ;;
  uninstall)
    do_uninstall
    ;;
  *)
    usage
    exit 1
    ;;
esac
