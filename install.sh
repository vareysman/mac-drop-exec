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

do_install() {
  cd "$(dirname "$0")"
  go build -o admin-daemon .

  install -d "$ROOT"
  install -m 755 admin-daemon "$ROOT/admin-daemon"

  install -m 644 "$PLIST_SRC" "$PLIST_DST"
  launchctl bootout system "$PLIST_DST" 2>/dev/null || true
  launchctl bootstrap system "$PLIST_DST"
  launchctl enable "system/$LABEL"
  launchctl kickstart -k "system/$LABEL"

  echo "Installed. Create /tmp/xipt with the shell command to run; on failure read /tmp/xopt."
}

show_daemons() {
  launchctl list | awk -v label="$LABEL" '
    BEGIN { IGNORECASE = 1 }
    /daemon/ || index($0, label) > 0 { print }
  '
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
