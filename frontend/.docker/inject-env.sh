#!/bin/sh
set -eu

INDEX_HTML="${INDEX_HTML:-/usr/share/nginx/html/index.html}"
[ -f "$INDEX_HTML" ] || exit 0

ENV_JSON="$(jq --compact-output --null-input 'env | with_entries(select(.key | startswith("VITE_")))')"
ESCAPED="$(printf '%s' "$ENV_JSON" | sed -e 's/[\&/]/\\&/g')"
sed -i "s|<noscript id=\"env-insertion-point\"></noscript>|<script>var ENV=${ESCAPED}</script>|g" "$INDEX_HTML"
