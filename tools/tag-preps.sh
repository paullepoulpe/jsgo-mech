#!/usr/bin/env bash
# Creates one annotated tag per prep step from the commit subjects, which all
# read "prep(NN-name): ...". Tag: prep-NN-name. Also tags the pristine import.
#   tools/tag-preps.sh          create missing tags locally
#   tools/tag-preps.sh --push   and push them
set -euo pipefail
cd "$(dirname "$0")/.."
git log --reverse --format='%H %s' | while read -r sha subject; do
  case "$subject" in
  "java: import Closure Compiler "*) tag=closure-${subject##* Compiler }; tag=${tag%% *} ;;
  "prep("*"): "*) tag=${subject#prep(}; tag=prep-${tag%%)*} ;;
  *) continue ;;
  esac
  git rev-parse -q --verify "refs/tags/$tag" >/dev/null || git tag -a "$tag" -m "$subject" "$sha"
  echo "$tag $sha"
done
[[ ${1:-} == --push ]] && git push origin --tags
