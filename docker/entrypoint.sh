#!/bin/sh
# Keeps yt-dlp fresh before starting the server. Failures never block start-up.
set -u

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
