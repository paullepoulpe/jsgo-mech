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


import static com.google.common.base.Preconditions.checkState;

import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticRef;
import com.google.javascript.rhino.StaticSlot;
import com.google.javascript.rhino.StaticSourceFile;
import com.google.javascript.rhino.Token;
import org.jspecify.annotations.Nullable;

/**
 * Used by {@code Scope} to store information about variables.
 */
public class Var extends AbstractVar implements StaticSlot, StaticRef {

  static final String ARGUMENTS = "arguments";
  static final String EXPORTS = "exports";

  Var(
      String name,
      @Nullable Node nameNode,
      Scope scope,
      int index,
      @Nullable CompilerInput input,
      @Nullable Node implicitGoogNamespaceDefinition) {
    super(name, nameNode, index, input, implicitGoogNamespaceDefinition);
    this.scope = scope;
    if (nameNode != null) {
      switch (nameNode.getToken()) {
        case MODULE_BODY, NAME, IMPORT_STAR -> {}
        default -> throw new IllegalArgumentException("Invalid name node " + nameNode);
      }
    }
  }

  static Var createImplicitGoogNamespace(String name, Scope scope, Node definition) {
    return new Var(name, /* nameNode= */ null, scope, -1, /* input= */ null, definition);
  }

  @Override
  public String toString() {
    return "Var " + getName() + " @ " + getNameNode();
  }

  // Members pushed down from AbstractVar (prep 18), with its type parameters replaced.

  private final Scope scope;

  @Override
  public final Node getScopeRoot() {
    return scope.getRootNode();
  }

  @Override
  public final StaticSourceFile getSourceFile() {
    return (this.getNode() != null ? this.getNode() : scope.getRootNode()).getStaticSourceFile();
  }

  @Override
  public final Var getSymbol() {
    return this;
  }

  @Override
  public final Var getDeclaration() {
    return this.getNode() == null ? null : this;
  }

  @Override
  public final Scope getScope() {
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
