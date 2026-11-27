#!/bin/sh
set -e

if [ "${1:-}" = init ]; then
    shift
    exec /usr/local/bin/panel init --profile docker "$@"
fi

if [ "$#" -gt 0 ]; then
    exec /usr/local/bin/panel "$@"
fi

config_path=/etc/kvzd-panel/config.json
legacy_config=/var/lib/kvzd-panel/config.json

migrate_config() (
    temp_config=$(mktemp "${config_path%/*}/.panel-config-XXXXXX") || exit 1
    trap 'unlink "$temp_config"' EXIT
    chmod 600 "$temp_config" && cat "$legacy_config" > "$temp_config" || exit 1
    # Publish only if the target is still absent, including during a concurrent init.
    ln -T "$temp_config" "$config_path" || [ -f "$config_path" ]
)

if [ -z "${PANEL_CONFIG:-}" ] && [ ! -e "$config_path" ] && [ ! -L "$config_path" ]; then
    if [ -f "$legacy_config" ]; then
        if ! migrate_config; then
            echo "Could not migrate the Panel configuration. Using $legacy_config." >&2
            export PANEL_CONFIG="$legacy_config"
        fi
    else
        export PANEL_CONFIG=/usr/local/share/kvzd-panel/docker-defaults.json
    fi
fi

# Start nginx so the demo site is reachable immediately. The panel can
# stop/reload/restart it afterwards through its binary control mode.
nginx

exec /usr/local/bin/panel
