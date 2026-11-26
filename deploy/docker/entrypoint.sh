#!/bin/sh
set -e

# Start nginx so the demo site is reachable immediately. The panel can
# stop/reload/restart it afterwards through its binary control mode.
nginx

exec /usr/local/bin/panel
