#!/usr/bin/env bash
# Install host-nginx site configs for 思维印记 front doors.
#
# Runs ON the ECS as root, from the repo, naming the hosts to install:
#
#     cd ~/mind-imprint && sudo bash deploy/install-nginx-sites.sh mind-lite.uni-robot.cn
#
# ⚠️  IT OVERWRITES THE INSTALLED CONFIG OF EVERY HOST YOU NAME, AND THE FILES
# IN deploy/nginx/ ARE HTTP-ONLY. certbot adds each host's :443 server block
# and http→https redirect by editing the INSTALLED file in place, so naming a
# host that is already serving TLS drops it back to plain HTTP until certbot
# runs again. That is why this script has no "install everything" default and
# refuses to run with no arguments: reinstalling a live host must be a thing
# you asked for by name, not something you get by pressing enter.
#
# After adding a NEW host, give it a certificate:
#
#     sudo certbot --nginx -d mind-lite.uni-robot.cn
#
# Committed rather than kept in the operator's local .deploy-local/ (where it
# used to live): a copy that exists on one laptop drifts the moment anyone
# else — or a rebuilt server — needs it.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "$#" -eq 0 ]; then
  echo "usage: sudo bash deploy/install-nginx-sites.sh <host> [host...]" >&2
  echo "available:" >&2
  ls -1 deploy/nginx/ | sed 's/\.conf$//' | sed 's/^/  /' >&2
  exit 2
fi

for site in "$@"; do
  src="deploy/nginx/$site.conf"
  [ -f "$src" ] || { echo "no config for '$site' (looked for $src)" >&2; exit 1; }
  if [ -e "/etc/nginx/sites-available/$site.conf" ]; then
    echo "note: $site is already installed — its certbot TLS block will be overwritten; re-run certbot after this"
  fi
  # `install` rather than cp+chmod: one call, explicit mode, no umask surprise.
  install -m 644 "$src" "/etc/nginx/sites-available/$site.conf"
  ln -sf "/etc/nginx/sites-available/$site.conf" "/etc/nginx/sites-enabled/$site.conf"
  echo "installed $site"
done

nginx -t
systemctl reload nginx
echo "NGINX_SITES_INSTALLED"
ls -l /etc/nginx/sites-enabled/ | grep mind
