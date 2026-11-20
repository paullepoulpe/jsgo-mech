#!/usr/bin/env bash
# baseline.sh <dir with g_*.go>: the spellings of erasure in translator output, the counts a stage of the rule is measured by
cd "$1" || exit 1
for p in '&[A-Za-z_0-9]*_tfn[_\[{]' '&[A-Za-z_0-9]*_fn(\[[^]]*\])?\{' '\bcast_\[' '\bicast_[A-Za-z_0-9]*\(' '\.as(List|Set|Map|Deque|Imm|Sorted|Entry)\(\)' '\bstr_\(' '\bNode_of_\(' '\bobj_\(' '\.box_\(\)' '\bstrobj_\(' '\bunbox_\[' '\bboxT_\(' '\bunboxT_\[' '\bforeach_\(' '\btce?_[A-Za-z_0-9]+' '\b[A-Za-z_]+Of\['; do
  printf '%6d  %s\n' "$(cat g_[A-Z]*.go | grep -oP -- "$p" | wc -l)" "$p"
done
printf '%6d  adapter types in g_zstubs.go\n' "$(grep -cE '^type [A-Za-z_0-9]*_t?fn(_|\b|\[)' g_zstubs.go)"
printf '%6d  conversion descriptors in g_0casts.go\n' "$(grep -cE '^(var|func) tce?_' g_0casts.go)"
printf '%6d  aliases in g_0aliases.go\n' "$(grep -c '^type ' g_0aliases.go)"
