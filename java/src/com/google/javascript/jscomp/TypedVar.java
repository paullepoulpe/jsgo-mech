/*
 * Copyright 2015 The Closure Compiler Authors.
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
import com.google.javascript.rhino.ErrorReporter;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticSourceFile;
import com.google.javascript.rhino.Token;
import com.google.javascript.rhino.jstype.JSType;
import com.google.javascript.rhino.jstype.StaticTypedRef;
import com.google.javascript.rhino.jstype.StaticTypedSlot;
import org.jspecify.annotations.Nullable;

/**
 * {@link AbstractVar} subclass for use with {@link TypedScope}.
 *
 * <p>Note that this class inherits its {@link #equals} and {@link #hashCode} implementations from
 * {@link ScopedName}, which does not include any type information. This is necessary because {@code
 * Var}-keyed maps are used across multiple top scopes, but it comes with the caveat that if {@code
 * TypedVar} instances are stored in a set, the type information is at risk of disappearing if an
 * untyped (or differently typed) var is added for the same symbol.
 */
public class TypedVar extends AbstractVar
    implements StaticTypedSlot, StaticTypedRef {

  private JSType type;
  // The next two fields and the associated methods are only used by
  // TypeInference.java. Maybe there is a way to avoid having them in all typed variable instances.
  private boolean markedEscaped = false;
  private boolean markedAssignedExactlyOnce = false;

  /**
   * Whether the variable's type has been inferred or is declared. An inferred
   * type may change over time (as more code is discovered), whereas a
   * declared type is a static contract that must be matched.
   */
  private final boolean typeInferred;

  // includes nodes that in plain JS semantics are not 'declarations', but that the type system
  // & compiler treat as declarations.
  private static final ImmutableSet<Token> NAME_NODE_TYPES =
      Sets.immutableEnumSet(
          Token.NAME,
          Token.THIS, // `if ('prop' in this) {` creates a TypedVar `this.prop` requiring a node
          Token.IMPORT_STAR,
          Token.EXPR_RESULT, // implicit variables from goog.provide
          Token.EXPORT, // tracks the *default* export
          Token.GETPROP,
          Token.FUNCTION,
          Token.STRING_KEY,
          Token.SETTER_DEF,
          Token.GETTER_DEF,
          Token.MEMBER_FUNCTION_DEF,
          Token.MODULE_BODY); // for the implicit exports object in a module

  TypedVar(boolean inferred, String name, Node nameNode, JSType type,
      TypedScope scope, int index, CompilerInput input) {
    super(name, nameNode, index, input, /* implicitGoogNamespaceDefinition= */ null);
    this.scope = scope;
    if (nameNode != null) {
      checkArgument(
          NAME_NODE_TYPES.contains(nameNode.getToken()),
          "Invalid name node token %s",
          nameNode.getToken());
    }
    this.type = type;
    this.typeInferred = inferred;
  }

  /**
   * Gets this variable's type. To know whether this type has been inferred,
   * see {@code #isTypeInferred()}.
   */
  @Override
  public JSType getType() {
    return type;
  }

  void setType(JSType type) {
    this.type = type;
  }

  void resolveType(ErrorReporter errorReporter) {
    if (type != null) {
      this.type = type.resolve(errorReporter);
    }
  }

  /**
   * Returns whether this variable's type is inferred. To get the variable's
   * type, see {@link #getType()}.
   */
  @Override
  public boolean isTypeInferred() {
    return typeInferred;
  }

  public String getInputName() {
    if (getInput() == null) {
      return "<non-file>";
    }
    return getInput().getName();
  }

  @Override
  public String toString() {
    return "Var " + getName() + "{" + type + "}";
  }

  void markEscaped() {
    markedEscaped = true;
  }

  boolean isMarkedEscaped() {
    return markedEscaped;
  }

  void markAssignedExactlyOnce() {
    markedAssignedExactlyOnce = true;
  }

  boolean isMarkedAssignedExactlyOnce() {
    return markedAssignedExactlyOnce;
  }

  // Members pushed down from AbstractVar (prep 18), with its type parameters replaced.

  private final TypedScope scope;

  @Override
  public final Node getScopeRoot() {
    return scope.getRootNode();
  }

  @Override
  public final StaticSourceFile getSourceFile() {
    return (this.getNode() != null ? this.getNode() : scope.getRootNode()).getStaticSourceFile();
  }

  @Override
  public final TypedVar getSymbol() {
    return this;
  }

  @Override
  public final TypedVar getDeclaration() {
    return this.getNode() == null ? null : this;
  }

  @Override
  public final TypedScope getScope() {
    return scope;
  }

  @Override
  public final boolean isGlobal() {
    return scope.isGlobal();
  }

  @Override
  public final boolean isLocal() {
    return scope.isLocal();
  }

  @Override
  public final boolean isArguments() {
    return Var.ARGUMENTS.equals(name) && scope.isFunctionScope();
  }

  @Override
  final boolean isGoogModuleExports() {
    return scope.isModuleScope() && "exports".equals(name) && isImplicit();
  }

  @Override
  public final boolean isThis() {
    return "this".equals(name) && scope.isFunctionScope();
  }

  private boolean isImplicit() {
    if (isImplicitGoogNamespace()) {
      return true;
    }
    AbstractScope.ImplicitVar var = AbstractScope.ImplicitVar.of(name);
    return var != null && var.isMadeByScope(scope);
  }

  @Override
  final @Nullable Token declarationType() {
    if (isImplicitGoogNamespace()) {
      return null;
    }
    for (Node current = this.getNode(); current != null; current = current.getParent()) {
      if (DECLARATION_TYPES.contains(current.getToken())) {
        return current.getToken();
      }
    }
    checkState(
        isImplicit(),
        "The nameNode for %s must be a descendant of one of: %s", this, DECLARATION_TYPES);
    return null;
  }
}
