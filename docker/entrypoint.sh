#!/bin/sh
# Prepares the data volume, keeps yt-dlp fresh and starts the server as an
# unprivileged user. Failures of the yt-dlp update never block start-up.
set -u

APP_USER=melotrack
DATA="${DATA_DIR:-/data}"

if [ "$(id -u)" = "0" ]; then
  # A bind-mounted ./data is created by Docker as root on Linux hosts, which the
  # app user cannot write to. Hand it over, then drop privileges for good.
  mkdir -p "$DATA"
  if [ "$(stat -c %u "$DATA")" != "$(id -u "$APP_USER")" ]; then
    echo "entrypoint: giving $DATA to $APP_USER"
    chown -R "$APP_USER:$APP_USER" "$DATA"
  fi
  export HOME="/home/$APP_USER"
  exec setpriv --reuid="$APP_USER" --regid="$APP_USER" --init-groups "$0" "$@"
fi

PIP=/opt/yt-dlp/bin/pip
PACKAGES="yt-dlp[default,curl-cffi]"

if [ "${YTDLP_POT_PROVIDER:-false}" = "true" ]; then
  # Plugin for the optional PO-token provider container (see README).
  PACKAGES="$PACKAGES bgutil-ytdlp-pot-provider"
fi

if [ "${YTDLP_AUTO_UPDATE:-true}" = "true" ]; then
  PRE=""
  [ "${YTDLP_NIGHTLY:-false}" = "true" ] && PRE="--pre"
  echo "entrypoint: updating yt-dlp…"
  if ! timeout 180 $PIP install --quiet --no-cache-dir --disable-pip-version-check --upgrade $PRE $PACKAGES; then
    echo "entrypoint: yt-dlp update failed, using the installed version"
  fi
elif [ "${YTDLP_POT_PROVIDER:-false}" = "true" ]; then
  timeout 180 $PIP install --quiet --no-cache-dir --disable-pip-version-check bgutil-ytdlp-pot-provider || true
fi

echo "entrypoint: yt-dlp $(/opt/yt-dlp/bin/yt-dlp --version 2>/dev/null || echo '?'), deno $(deno --version 2>/dev/null | head -n1 | cut -d' ' -f2)"
exec /app/melotrack
