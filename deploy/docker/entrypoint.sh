#!/bin/sh
set -e

if [ "${1:-}" = init ]; then
    shift
    exec /usr/local/bin/panel init --profile docker "$@"
fi

if [ "$#" -gt 0 ]; then
    exec /usr/local/bin/panel "$@"
fi

if [ -z "${PANEL_CONFIG:-}" ] && [ -f /var/lib/kvzd-panel/config.json ]; then
    export PANEL_CONFIG=/var/lib/kvzd-panel/config.json
fi

# Start nginx so the demo site is reachable immediately. The panel can
# stop/reload/restart it afterwards through its binary control mode.
nginx

exec /usr/local/bin/panel
