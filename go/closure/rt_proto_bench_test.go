package closure

import "testing"

// pbBenchTree_ builds an AST message the way TypedAstSerializer.visit does: a builder per
// node, its children built first.
func pbBenchTree_(depth, fan int, n *int32) *AstNode {
	*n++
	b := AstNode_newBuilder().setRelativeLine(*n & 1).setRelativeColumn(*n%7 - 3)
	if *n%3 != 0 {
		b.setType(*n % 5000)
	}
	b.setKindValue(1 + *n%40)
	switch *n % 4 {
	case 0, 1:
		b.setStringValuePointer(*n % 3000)
	case 2:
		b.setDoubleValue(float64(*n))
	}
	b.setBooleanProperties(int64(*n & 4))
	b.setSourceFile(0)
	if depth > 0 {
		for i := 0; i < fan; i++ {
			b.addChild(pbBenchTree_(depth-1, fan, n))
		}
	}
	b.setOriginalNamePointer(0)
	return b.buildLastT_()
}

// BenchmarkProtoAstBuildBytes: building a script's AST message (3,280 nodes) and writing it.
func BenchmarkProtoAstBuildBytes(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var n int32
		if len(pbBenchTree_(7, 3, &n).bytes_()) == 0 {
			b.Fatal("empty")
		}
	}
}

// BenchmarkProtoAstBuild: the building alone.
func BenchmarkProtoAstBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var n int32
		pbBenchTree_(7, 3, &n)
	}
}
