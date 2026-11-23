//go:build !j2go_nofntypes

package closure

import "testing"

// Kind `guava`: the wrapper of the runtime's erased LoadingCache (a Function_base_e is a func
// value only with rule "function types": with J2GO_NOFNTYPES=1 this file is not built).
func TestLgT2GuavaCache(t *testing.T) {
	lc := CacheBuilder_newBuilder().build(CacheLoader_from(func(k any) any { return k.(string) + "!" }))
	c := loadingCacheAs_(tc_string, tc_string, lc)
	if c.getUnchecked_t("x") != "x!" || c.get_t("y") != "y!" || LoadingCache_un_(c) != lc || lgBox_(c) != any(lc) {
		t.Fatal("the wrapper of a LoadingCache")
	}
}
