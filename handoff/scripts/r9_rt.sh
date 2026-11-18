#!/bin/bash
# applies the runtime additions for nested typed collections to the closure dir $1
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cp $S/r9_rt_tnested.go $1/rt_tnested.go
sed -i 's|^type tsetView_\[K comparable\] struct{ m \*TMap\[K, struct{}\] }|type tsetView_[K comparable] struct {\n\tm *TMap[K, struct{}]\n\ts *TSet[K] // the set itself (m is its map)\n}|; s|x.sview = &tsetView_\[K\]{m: &s.TMap}|x.sview = \&tsetView_[K]{m: \&s.TMap, s: s}|' $1/rt_tmap.go
