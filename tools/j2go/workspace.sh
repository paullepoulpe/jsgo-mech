#!/usr/bin/env bash
# Makes a private copy of the tree for one parallel worker: /home/user/work/<name>.
# It shares .cache (read-only use) and a frozen build of the translator, so that
# `tools/j2go/regen.sh go/closure` there is unaffected by work going on elsewhere.
set -euo pipefail
cd "$(dirname "$0")/../.."
name=$1
dest=/home/user/work/$name
rm -rf "$dest" && mkdir -p "$dest"
[[ -d .cache/j2go-frozen ]] || cp -r .cache/j2go .cache/j2go-frozen
cp -r tools java go pin.env "$dest/"
ln -s "$PWD/.cache" "$dest/.cache"
echo "export J2GO_CLASSES=$PWD/.cache/j2go-frozen" >"$dest/env.sh"
echo "$dest"
