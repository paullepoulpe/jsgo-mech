/*
 * Copyright 2018 The Closure Compiler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.google.javascript.jscomp;

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Sets;
import com.google.javascript.rhino.JSDocInfo;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticRef;
import com.google.javascript.rhino.StaticSlot;
import com.google.javascript.rhino.StaticSourceFile;
import com.google.javascript.rhino.StaticSourceFile.SourceKind;
import com.google.javascript.rhino.Token;
import org.jspecify.annotations.Nullable;

/**
 * Used by {@code Scope} to store information about variables.
 */
public abstract class AbstractVar
    extends ScopedName implements StaticSlot, StaticRef {

  final String name;

  private final @Nullable Node nameNode;

  // null if not an implicit goog namespace; otherwise starts out as an ArrayList then is frozen
  // into an ImmutableList.
  private @Nullable SourceKind implicitGoogNamespaceStrength;

  /** Input source */
  private final CompilerInput input;

  /**
   * The index at which the var is declared. e.g. if it's 0, it's the first declared variable in
   * that scope
   */
  private final int index;

  /**
   * @param name The name of this var. Does not have to be semantically valid JS identifier.
   * @param nameNode The node representing this variables declaration or null
   * @param scope The scope containing this var
   * @param index The index at which the var is declared in the scope. Must be either positive, or
   *     -1 for implicit variables.
   * @param input The compiler input containing the given nameNode, if any. May be null to allow for
   *     declaring the native types in the type system.
   */
  AbstractVar(
      String name,
      @Nullable Node nameNode,
      int index,
      @Nullable CompilerInput input,
      @Nullable Node implicitGoogNamespaceDefinition) {
    checkArgument(index >= -1, index);
    this.name = name;
    if (implicitGoogNamespaceDefinition != null) {
      this.nameNode = null;
      this.implicitGoogNamespaceStrength = strengthOf(implicitGoogNamespaceDefinition);
    } else {
      this.nameNode = nameNode;
      this.implicitGoogNamespaceStrength = null;
    }
    this.index = index;
    this.input = input;
  }

  @Override
  public final String getName() {
    return name;
  }

  public abstract Node getScopeRoot();

  @Override
  public final Node getNode() {
    return nameNode;
  }

  final CompilerInput getInput() {
    return input;
  }

  public abstract StaticSourceFile getSourceFile();

  public abstract AbstractVar getSymbol();

  public abstract AbstractVar getDeclaration();

  public final @Nullable Node getParentNode() {
    return this.getNode() == null ? null : this.getNode().getParent();
  }

  /**
   * Whether this is a bleeding function (an anonymous named function
   * that bleeds into the inner scope).
   */
  public boolean isBleedingFunction() {
    Node parent = getParentNode();
    return parent != null && NodeUtil.isFunctionExpression(parent);
  }

  public abstract AbstractScope getScope();

  /**
   * The index at which the var is declared. e.g. if it's 0, it's the first declared variable in
   * that scope
   */
  int getIndex() {
    return index;
  }

  public abstract boolean isGlobal();

  public abstract boolean isLocal();

  final boolean isExtern() {
    return input == null || input.isExtern();
  }

  /** Returns {@code true} if the variable is declared or inferred to be a constant. */
  public final boolean isDeclaredOrInferredConst() {
    Node declarationNode = this.getNode();
    if (declarationNode == null) {
      return false;
    }

    return declarationNode.isDeclaredConstantVar()
        || declarationNode.isInferredConstantVar()
        || declarationNode.getBooleanProp(Node.IS_CONSTANT_NAME);
  }

  /**
   * Returns {@code true} if the variable is declared as a define.
   * A variable is a define if it is annotated by {@code @define}.
   */
  public final boolean isDefine() {
    JSDocInfo info = getJSDocInfo();
    return info != null && info.isDefine();
  }

  public final @Nullable Node getInitialValue() {
    return this.getNode() == null ? null : NodeUtil.getRValueOfLValue(this.getNode());
  }

  public final Node getNameNode() {
    return this.getNode();
  }

  @Override
  public final @Nullable JSDocInfo getJSDocInfo() {
    return this.getNode() == null ? null : NodeUtil.getBestJSDocInfo(this.getNode());
  }

  final boolean isVar() {
    return declarationType() == Token.VAR;
  }

  final boolean isCatch() {
    return declarationType() == Token.CATCH;
  }

  final boolean isLet() {
    return declarationType() == Token.LET;
  }

  final boolean isConst() {
    return declarationType() == Token.CONST;
  }

  final boolean isClass() {
    return declarationType() == Token.CLASS;
  }

  final boolean isParam() {
    return declarationType() == Token.PARAM_LIST;
  }

  public final boolean isDefaultParam() {
    Node parent = this.getNode().getParent();
    return parent.getParent().isParamList() && parent.isDefaultValue()
        && parent.getFirstChild() == nameNode;
  }

  final boolean isImport() {
    return declarationType() == Token.IMPORT;
  }

  public abstract boolean isArguments();

  abstract boolean isGoogModuleExports();

  public abstract boolean isThis();

  static final ImmutableSet<Token> DECLARATION_TYPES = Sets.immutableEnumSet(
      Token.VAR,
      Token.LET,
      Token.CONST,
      Token.FUNCTION,
      Token.CLASS,
      Token.CATCH,
      Token.IMPORT,
      Token.PARAM_LIST);

  abstract @Nullable Token declarationType();

  /**
   * Whether this name comes from a goog.provide or legacy goog.module and does not have a normal
   * syntactical definition
   */
  final boolean isImplicitGoogNamespace() {
    return this.implicitGoogNamespaceStrength != null;
  }

  /**
   * Indicates this namespace was provided at the given node
   *
   * @param definition the STRINGLIT in a goog.provide or goog.module call
   */
  final void addImplicitGoogNamespaceDefinition(Node definition) {
    checkState(this.isImplicitGoogNamespace(), this);
    this.implicitGoogNamespaceStrength =
        strongerOf(this.implicitGoogNamespaceStrength, strengthOf(definition));
  }

  /** Returns the strongest kind of all definitions of this namespace */
  final SourceKind getImplicitGoogNamespaceStrength() {
    checkState(this.isImplicitGoogNamespace(), this);
    return this.implicitGoogNamespaceStrength;
  }

  private static final SourceKind strengthOf(Node n) {
    StaticSourceFile source = n.getStaticSourceFile();
    if (source == null) {
      return SourceKind.EXTERN;
    }

    return source.getKind();
  }

  private static SourceKind strongerOf(SourceKind left, SourceKind right) {
    if (left.equals(SourceKind.STRONG) || right.equals(SourceKind.STRONG)) {
      return SourceKind.STRONG;
    } else if (left.equals(SourceKind.EXTERN) || right.equals(SourceKind.EXTERN)) {
      // Externs are stronger because they aren't deleted.
      return SourceKind.EXTERN;
    }
    return SourceKind.WEAK;
  }
}
