package closure

// buildT_ is a protobuf builder's build() with the type Java's generated builder declares for
// it, the message class (javac's covariant override of MessageLite.Builder.build()); the
// translation's build() is the erased one, returning the message as a MessageLite that the
// caller casts back (post-flip step pbbuild calls buildT_ instead). The same message, built
// by the same pbBuild_. One per message of rt_proto_gen.go.

func (this *Mapping_OriginalMapping_Builder) buildT_() *Mapping_OriginalMapping {
	return this.pbBuild_().(*Mapping_OriginalMapping)
}
func (this *AdditionalConformanceReport_Builder) buildT_() *AdditionalConformanceReport {
	return this.pbBuild_().(*AdditionalConformanceReport)
}
func (this *AdditionalConformanceReport_ReportByRuleId_Builder) buildT_() *AdditionalConformanceReport_ReportByRuleId {
	return this.pbBuild_().(*AdditionalConformanceReport_ReportByRuleId)
}
func (this *AdditionalConformanceReport_ViolationLocation_Builder) buildT_() *AdditionalConformanceReport_ViolationLocation {
	return this.pbBuild_().(*AdditionalConformanceReport_ViolationLocation)
}
func (this *JSCompilerStateProto_Builder) buildT_() *JSCompilerStateProto {
	return this.pbBuild_().(*JSCompilerStateProto)
}
func (this *ChunkProto_Builder) buildT_() *ChunkProto       { return this.pbBuild_().(*ChunkProto) }
func (this *UniqueIdProto_Builder) buildT_() *UniqueIdProto { return this.pbBuild_().(*UniqueIdProto) }
func (this *AccessorSummaryEntryProto_Builder) buildT_() *AccessorSummaryEntryProto {
	return this.pbBuild_().(*AccessorSummaryEntryProto)
}
func (this *AccessorSummaryProto_Builder) buildT_() *AccessorSummaryProto {
	return this.pbBuild_().(*AccessorSummaryProto)
}
func (this *VariableMapEntryProto_Builder) buildT_() *VariableMapEntryProto {
	return this.pbBuild_().(*VariableMapEntryProto)
}
func (this *ConformanceViolationsProto_Builder) buildT_() *ConformanceViolationsProto {
	return this.pbBuild_().(*ConformanceViolationsProto)
}
func (this *ViolationProto_Builder) buildT_() *ViolationProto {
	return this.pbBuild_().(*ViolationProto)
}
func (this *ConformanceConfig_Builder) buildT_() *ConformanceConfig {
	return this.pbBuild_().(*ConformanceConfig)
}
func (this *Requirement_Builder) buildT_() *Requirement { return this.pbBuild_().(*Requirement) }
func (this *RequirementScopeEntry_Builder) buildT_() *RequirementScopeEntry {
	return this.pbBuild_().(*RequirementScopeEntry)
}
func (this *RequirementScope_Builder) buildT_() *RequirementScope {
	return this.pbBuild_().(*RequirementScope)
}
func (this *InstrumentationPoint_Builder) buildT_() *InstrumentationPoint {
	return this.pbBuild_().(*InstrumentationPoint)
}
func (this *InstrumentationPointStats_Builder) buildT_() *InstrumentationPointStats {
	return this.pbBuild_().(*InstrumentationPointStats)
}
func (this *FileProfile_Builder) buildT_() *FileProfile     { return this.pbBuild_().(*FileProfile) }
func (this *ReportProfile_Builder) buildT_() *ReportProfile { return this.pbBuild_().(*ReportProfile) }
func (this *JsCompilerConformanceReportWrapper_Builder) buildT_() *JsCompilerConformanceReportWrapper {
	return this.pbBuild_().(*JsCompilerConformanceReportWrapper)
}
func (this *JsCompilerConformanceReport_Builder) buildT_() *JsCompilerConformanceReport {
	return this.pbBuild_().(*JsCompilerConformanceReport)
}
func (this *JsCompilerConformanceReport_ConformanceViolation_Builder) buildT_() *JsCompilerConformanceReport_ConformanceViolation {
	return this.pbBuild_().(*JsCompilerConformanceReport_ConformanceViolation)
}
func (this *OptimizationJsdoc_Builder) buildT_() *OptimizationJsdoc {
	return this.pbBuild_().(*OptimizationJsdoc)
}
func (this *SourceFilePool_Builder) buildT_() *SourceFilePool {
	return this.pbBuild_().(*SourceFilePool)
}
func (this *SourceFileProto_Builder) buildT_() *SourceFileProto {
	return this.pbBuild_().(*SourceFileProto)
}
func (this *SourceFileProto_FileOnDisk_Builder) buildT_() *SourceFileProto_FileOnDisk {
	return this.pbBuild_().(*SourceFileProto_FileOnDisk)
}
func (this *SourceFileProto_ZipEntryOnDisk_Builder) buildT_() *SourceFileProto_ZipEntryOnDisk {
	return this.pbBuild_().(*SourceFileProto_ZipEntryOnDisk)
}
func (this *TypedAst_Builder) buildT_() *TypedAst           { return this.pbBuild_().(*TypedAst) }
func (this *TypedAst_List_Builder) buildT_() *TypedAst_List { return this.pbBuild_().(*TypedAst_List) }
func (this *LazyAst_Builder) buildT_() *LazyAst             { return this.pbBuild_().(*LazyAst) }
func (this *NonLazyTypedAst_Builder) buildT_() *NonLazyTypedAst {
	return this.pbBuild_().(*NonLazyTypedAst)
}
func (this *NonLazyTypedAst_List_Builder) buildT_() *NonLazyTypedAst_List {
	return this.pbBuild_().(*NonLazyTypedAst_List)
}
func (this *NonLazyAst_Builder) buildT_() *NonLazyAst { return this.pbBuild_().(*NonLazyAst) }
func (this *StringPoolProto_Builder) buildT_() *StringPoolProto {
	return this.pbBuild_().(*StringPoolProto)
}
func (this *ExternsSummary_Builder) buildT_() *ExternsSummary {
	return this.pbBuild_().(*ExternsSummary)
}
func (this *AstNode_Builder) buildT_() *AstNode { return this.pbBuild_().(*AstNode) }
func (this *TemplateStringValue_Builder) buildT_() *TemplateStringValue {
	return this.pbBuild_().(*TemplateStringValue)
}
func (this *TypePool_Builder) buildT_() *TypePool { return this.pbBuild_().(*TypePool) }
func (this *TypePool_DebugInfo_Builder) buildT_() *TypePool_DebugInfo {
	return this.pbBuild_().(*TypePool_DebugInfo)
}
func (this *TypePool_DebugInfo_Mismatch_Builder) buildT_() *TypePool_DebugInfo_Mismatch {
	return this.pbBuild_().(*TypePool_DebugInfo_Mismatch)
}
func (this *SubtypingEdge_Builder) buildT_() *SubtypingEdge { return this.pbBuild_().(*SubtypingEdge) }
func (this *UnionTypeProto_Builder) buildT_() *UnionTypeProto {
	return this.pbBuild_().(*UnionTypeProto)
}
func (this *ObjectTypeProto_Builder) buildT_() *ObjectTypeProto {
	return this.pbBuild_().(*ObjectTypeProto)
}
func (this *ObjectTypeProto_DebugInfo_Builder) buildT_() *ObjectTypeProto_DebugInfo {
	return this.pbBuild_().(*ObjectTypeProto_DebugInfo)
}
func (this *TypeProto_Builder) buildT_() *TypeProto       { return this.pbBuild_().(*TypeProto) }
func (this *TypePoolList_Builder) buildT_() *TypePoolList { return this.pbBuild_().(*TypePoolList) }
