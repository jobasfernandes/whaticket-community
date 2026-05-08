#!/usr/bin/env bash
set -u

paths=()
[ -d backend ] && paths+=(backend)
[ -d worker ]  && paths+=(worker)

if [ ${#paths[@]} -eq 0 ]; then
  exit 0
fi

if ! command -v gofmt >/dev/null 2>&1; then
  exit 0
fi

out="$(gofmt -l "${paths[@]}" 2>/dev/null || true)"

if [ -n "$out" ]; then
  printf '[gofmt] arquivos sem formatacao:\n'
  printf '%s\n' "$out" | sed 's/^/  - /'
  printf "Rode 'gofmt -w <arquivo>' antes de declarar pronto.\n"
fi

exit 0
