#!/usr/bin/env bash
# Install a manually-issued TLS certificate for one host, and reload nginx.
#
# This exists for hosts certbot does NOT manage. Today that is
# mind-lite.uni-robot.cn, which carries an Aliyun-issued DV certificate
# because Let's Encrypt's production validation queries the name from several
# vantage points worldwide and the non-China ones time out against
# uni-robot.cn's hichina nameservers.
#
# THIS IS ALSO THE RENEWAL PROCEDURE. An Aliyun DV certificate lasts ~90 days
# and nothing renews it automatically. When it nears expiry:
#
#   1. Aliyun console → SSL 证书 → issue/renew for the host → download,
#      choosing "Nginx" as the server type. You get <host>.pem and <host>.key.
#   2. scp both to the server's ~ (they are secrets; do not commit them).
#   3. cd ~/mind-imprint && sudo bash deploy/install-tls-cert.sh <host>
#
# Usage (on the server, as root):
#   sudo bash deploy/install-tls-cert.sh <host> [pem] [key]
# pem/key default to /home/deploy/<host>.pem and /home/deploy/<host>.key.
set -euo pipefail

HOST="${1:-}"
[ -n "$HOST" ] || { echo "usage: sudo bash deploy/install-tls-cert.sh <host> [pem] [key]" >&2; exit 2; }
PEM="${2:-/home/deploy/$HOST.pem}"
KEY="${3:-/home/deploy/$HOST.key}"

[ -f "$PEM" ] || { echo "no certificate at $PEM" >&2; exit 1; }
[ -f "$KEY" ] || { echo "no private key at $KEY" >&2; exit 1; }

# Refuse a mismatched pair rather than reloading nginx into a broken state.
# Compares the PUBLIC half of each, so nothing secret is printed.
cert_pub=$(openssl x509 -in "$PEM" -noout -pubkey | openssl sha256)
key_pub=$(openssl pkey -in "$KEY" -pubout | openssl sha256)
if [ "$cert_pub" != "$key_pub" ]; then
  echo "certificate and private key do not match — refusing to install" >&2
  exit 1
fi

# A leaf-only file breaks clients that have not cached the intermediate, and
# the symptom (works in my browser, fails elsewhere) is miserable to chase.
chain_count=$(grep -c 'BEGIN CERTIFICATE' "$PEM" || true)
if [ "$chain_count" -lt 2 ]; then
  echo "warning: $PEM holds $chain_count certificate(s) — expected leaf + intermediate" >&2
fi

# Verify the certificate actually covers the host it is being installed for.
if ! openssl x509 -in "$PEM" -noout -checkhost "$HOST" >/dev/null; then
  echo "certificate does not cover $HOST — refusing to install" >&2
  exit 1
fi

mkdir -p /etc/nginx/ssl
install -o root -g root -m 644 "$PEM" "/etc/nginx/ssl/$HOST.pem"
# 600: the private key must not be readable by the deploy user or anyone else.
install -o root -g root -m 600 "$KEY" "/etc/nginx/ssl/$HOST.key"

nginx -t
systemctl reload nginx

echo "installed TLS for $HOST"
openssl x509 -in "/etc/nginx/ssl/$HOST.pem" -noout -subject -issuer -dates

# The staging copies were scp'd into a home directory to get here. Now that
# the real ones are in place, root-owned and 600, leave no second copy of a
# private key lying around readable by the deploy user.
if [ "$KEY" != "/etc/nginx/ssl/$HOST.key" ]; then
  rm -f "$KEY" "$PEM"
  echo "removed staging copies ($PEM, $KEY)"
fi
