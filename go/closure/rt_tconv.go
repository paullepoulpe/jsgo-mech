package closure

// tc_string is the element conversion of the typed collections of strings (post-flip step 3
// wrote it into g_zconv_lists.go with the first typed list of strings; on a translator whose
// own typed family holds every such list until step 402 unifies the two families, no step
// before that declares it, and the tests of the post-flip family name it). The text is the
// one tools/postflip/colls writes; the collection steps use a conversion that is declared.
var tc_string = &tconv_[string]{box: func(v string) any { return strobj_(v) }, unbox: func(v any) string { return str_(v) }, null: nullstr_}
