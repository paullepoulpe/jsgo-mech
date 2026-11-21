package closure

import "unsafe"

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

// buildLastT_ is buildT_ for a builder that nothing uses after the call (post-flip step pblast
// rewrites the buildT_ calls of which it shows that): the builder's allocation becomes the
// message (pbData_.buildLast_) instead of being copied into a new one. One per message.

func (this *Mapping_OriginalMapping_Builder) buildLastT_() *Mapping_OriginalMapping {
	return (*Mapping_OriginalMapping)(unsafe.Pointer(this.buildLast_()))
}
func (this *AdditionalConformanceReport_Builder) buildLastT_() *AdditionalConformanceReport {
	return (*AdditionalConformanceReport)(unsafe.Pointer(this.buildLast_()))
}
func (this *AdditionalConformanceReport_ReportByRuleId_Builder) buildLastT_() *AdditionalConformanceReport_ReportByRuleId {
	return (*AdditionalConformanceReport_ReportByRuleId)(unsafe.Pointer(this.buildLast_()))
}
func (this *AdditionalConformanceReport_ViolationLocation_Builder) buildLastT_() *AdditionalConformanceReport_ViolationLocation {
	return (*AdditionalConformanceReport_ViolationLocation)(unsafe.Pointer(this.buildLast_()))
}
func (this *JSCompilerStateProto_Builder) buildLastT_() *JSCompilerStateProto {
	return (*JSCompilerStateProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ChunkProto_Builder) buildLastT_() *ChunkProto {
	return (*ChunkProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *UniqueIdProto_Builder) buildLastT_() *UniqueIdProto {
	return (*UniqueIdProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *AccessorSummaryEntryProto_Builder) buildLastT_() *AccessorSummaryEntryProto {
	return (*AccessorSummaryEntryProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *AccessorSummaryProto_Builder) buildLastT_() *AccessorSummaryProto {
	return (*AccessorSummaryProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *VariableMapEntryProto_Builder) buildLastT_() *VariableMapEntryProto {
	return (*VariableMapEntryProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ConformanceViolationsProto_Builder) buildLastT_() *ConformanceViolationsProto {
	return (*ConformanceViolationsProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ViolationProto_Builder) buildLastT_() *ViolationProto {
	return (*ViolationProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ConformanceConfig_Builder) buildLastT_() *ConformanceConfig {
	return (*ConformanceConfig)(unsafe.Pointer(this.buildLast_()))
}
func (this *Requirement_Builder) buildLastT_() *Requirement {
	return (*Requirement)(unsafe.Pointer(this.buildLast_()))
}
func (this *RequirementScopeEntry_Builder) buildLastT_() *RequirementScopeEntry {
	return (*RequirementScopeEntry)(unsafe.Pointer(this.buildLast_()))
}
func (this *RequirementScope_Builder) buildLastT_() *RequirementScope {
	return (*RequirementScope)(unsafe.Pointer(this.buildLast_()))
}
func (this *InstrumentationPoint_Builder) buildLastT_() *InstrumentationPoint {
	return (*InstrumentationPoint)(unsafe.Pointer(this.buildLast_()))
}
func (this *InstrumentationPointStats_Builder) buildLastT_() *InstrumentationPointStats {
	return (*InstrumentationPointStats)(unsafe.Pointer(this.buildLast_()))
}
func (this *FileProfile_Builder) buildLastT_() *FileProfile {
	return (*FileProfile)(unsafe.Pointer(this.buildLast_()))
}
func (this *ReportProfile_Builder) buildLastT_() *ReportProfile {
	return (*ReportProfile)(unsafe.Pointer(this.buildLast_()))
}
func (this *JsCompilerConformanceReportWrapper_Builder) buildLastT_() *JsCompilerConformanceReportWrapper {
	return (*JsCompilerConformanceReportWrapper)(unsafe.Pointer(this.buildLast_()))
}
func (this *JsCompilerConformanceReport_Builder) buildLastT_() *JsCompilerConformanceReport {
	return (*JsCompilerConformanceReport)(unsafe.Pointer(this.buildLast_()))
}
func (this *JsCompilerConformanceReport_ConformanceViolation_Builder) buildLastT_() *JsCompilerConformanceReport_ConformanceViolation {
	return (*JsCompilerConformanceReport_ConformanceViolation)(unsafe.Pointer(this.buildLast_()))
}
func (this *OptimizationJsdoc_Builder) buildLastT_() *OptimizationJsdoc {
	return (*OptimizationJsdoc)(unsafe.Pointer(this.buildLast_()))
}
func (this *SourceFilePool_Builder) buildLastT_() *SourceFilePool {
	return (*SourceFilePool)(unsafe.Pointer(this.buildLast_()))
}
func (this *SourceFileProto_Builder) buildLastT_() *SourceFileProto {
	return (*SourceFileProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *SourceFileProto_FileOnDisk_Builder) buildLastT_() *SourceFileProto_FileOnDisk {
	return (*SourceFileProto_FileOnDisk)(unsafe.Pointer(this.buildLast_()))
}
func (this *SourceFileProto_ZipEntryOnDisk_Builder) buildLastT_() *SourceFileProto_ZipEntryOnDisk {
	return (*SourceFileProto_ZipEntryOnDisk)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypedAst_Builder) buildLastT_() *TypedAst {
	return (*TypedAst)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypedAst_List_Builder) buildLastT_() *TypedAst_List {
	return (*TypedAst_List)(unsafe.Pointer(this.buildLast_()))
}
func (this *LazyAst_Builder) buildLastT_() *LazyAst {
	return (*LazyAst)(unsafe.Pointer(this.buildLast_()))
}
func (this *NonLazyTypedAst_Builder) buildLastT_() *NonLazyTypedAst {
	return (*NonLazyTypedAst)(unsafe.Pointer(this.buildLast_()))
}
func (this *NonLazyTypedAst_List_Builder) buildLastT_() *NonLazyTypedAst_List {
	return (*NonLazyTypedAst_List)(unsafe.Pointer(this.buildLast_()))
}
func (this *NonLazyAst_Builder) buildLastT_() *NonLazyAst {
	return (*NonLazyAst)(unsafe.Pointer(this.buildLast_()))
}
func (this *StringPoolProto_Builder) buildLastT_() *StringPoolProto {
	return (*StringPoolProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ExternsSummary_Builder) buildLastT_() *ExternsSummary {
	return (*ExternsSummary)(unsafe.Pointer(this.buildLast_()))
}
func (this *AstNode_Builder) buildLastT_() *AstNode {
	return (*AstNode)(unsafe.Pointer(this.buildLast_()))
}
func (this *TemplateStringValue_Builder) buildLastT_() *TemplateStringValue {
	return (*TemplateStringValue)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypePool_Builder) buildLastT_() *TypePool {
	return (*TypePool)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypePool_DebugInfo_Builder) buildLastT_() *TypePool_DebugInfo {
	return (*TypePool_DebugInfo)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypePool_DebugInfo_Mismatch_Builder) buildLastT_() *TypePool_DebugInfo_Mismatch {
	return (*TypePool_DebugInfo_Mismatch)(unsafe.Pointer(this.buildLast_()))
}
func (this *SubtypingEdge_Builder) buildLastT_() *SubtypingEdge {
	return (*SubtypingEdge)(unsafe.Pointer(this.buildLast_()))
}
func (this *UnionTypeProto_Builder) buildLastT_() *UnionTypeProto {
	return (*UnionTypeProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ObjectTypeProto_Builder) buildLastT_() *ObjectTypeProto {
	return (*ObjectTypeProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *ObjectTypeProto_DebugInfo_Builder) buildLastT_() *ObjectTypeProto_DebugInfo {
	return (*ObjectTypeProto_DebugInfo)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypeProto_Builder) buildLastT_() *TypeProto {
	return (*TypeProto)(unsafe.Pointer(this.buildLast_()))
}
func (this *TypePoolList_Builder) buildLastT_() *TypePoolList {
	return (*TypePoolList)(unsafe.Pointer(this.buildLast_()))
}
