#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 SHARD..." >&2
  exit 1
fi

module=$(go list -m)
packages=""
for shard in "$@"; do
  case "$shard" in
    kvm) list=$(go list ./kvm/...) ;;
    integration) list=$(go list ./integrationTest/...) ;;
    storage-data) list=$(go list ./storage/... ./data/...) ;;
    rest) list=$(go list ./... | grep -Ev "^${module}/(kvm|integrationTest|storage|data)(/|$)") ;;
    *)
      echo "unknown test shard $shard" >&2
      exit 1
      ;;
  esac
  if [ -z "$list" ]; then
    echo "test shard $shard has no packages" >&2
    exit 1
  fi
  packages+="$list"$'\n'
done

printf '%s' "$packages"
