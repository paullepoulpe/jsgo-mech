/*
 * Copyright 2004 The Closure Compiler Authors.
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
import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Maps;
import com.google.javascript.rhino.Node;
import java.util.Collections;
import java.util.EnumMap;
import java.util.LinkedHashMap;
import java.util.Map;
import org.jspecify.annotations.Nullable;

/**
 * Scope contains information about a variable scope in JavaScript. Scopes can be nested, a scope
 * points back to its parent scope. A Scope contains information about variables defined in that
 * scope.
 *
 * @see NodeTraversal
 */
public final class Scope extends AbstractScope {

  private final @Nullable Scope parent;
  private final int depth;

  static Scope createGlobalScope(Node rootNode) {
    return new Scope(rootNode);
  }

  static Scope createChildScope(Scope parent, Node rootNode) {
    return new Scope(parent, rootNode);
  }

  private Scope(Scope parent, Node rootNode) {
    super(rootNode);
    checkChildScope(parent);
    this.parent = parent;
    this.depth = parent.getDepth() + 1;
  }

  private Scope(Node rootNode) {
    super(rootNode);
    checkRootScope();
    this.parent = null;
    this.depth = 0;
  }

  @Override
  public Scope untyped() {
    return this;
  }

  @Override
  public int getDepth() {
    return depth;
  }

  @Override
  public Scope getParent() {
    return parent;
  }

  /**
   * Declares a variable.
   *
   * @param name name of the variable
   * @param nameNode the NAME node declaring the variable
   * @param input the input in which this variable is defined.
   */
  Var declare(String name, Node nameNode, CompilerInput input) {
    checkArgument(!name.isEmpty());
    // Make sure that it's declared only once
    checkState(getOwnSlot(name) == null);
    Var var =
        new Var(
            name,
            nameNode,
            this,
            getVarCount(),
            input,
            /* implicitGoogNamespaceDefinition= */ null);
    declareInternal(name, var);
    return var;
  }

  /**
   * Declares an implicit goog.provide or goog.module namespace in this scope
   *
   * @param name name of the variable
   * @param definition the STRINGLIT node holding the full namespace
   */
  Var declareImplicitGoogNamespaceIfAbsent(String name, Node definition) {
    checkArgument(!name.isEmpty());
    checkState(this.isGlobal(), "Cannot declare implicit goog namespace in local scope %s", this);
    // Allow redeclarations of provides, since they are implicit and don't have a single
    // declaration site.
    Var var = getOwnSlot(name);
    if (var == null) {
      var = Var.createImplicitGoogNamespace(name, this, definition);
      declareInternal(name, var);
    } else if (var.isImplicitGoogNamespace()) {
      var.addImplicitGoogNamespaceDefinition(definition);
    }
    return var;
  }

  @Override
  Var makeImplicitVar(ImplicitVar var) {
    return new Var(
        var.nameField,
        /* nameNode= */ null,
        this,
        /* index= */ -1,
        /* input= */ null,
        /* implicitGoogNamespaceDefinition= */ null);
  }

  // Members pushed down from AbstractScope (prep 17), with its type parameters replaced.

  private Map<String, Var> vars = ImmutableMap.of();

  private Map<ImplicitVar, Var> implicitVars = ImmutableMap.of();

  /**
   * @return True if this scope contains {@code other}, or is the same scope as {@code other}.
   */
  final boolean contains(Scope other) {
    Scope s = checkNotNull(other);
    while (s != null) {
      if (s == this) {
        return true;
      }
      s = s.getParent();
    }
    return false;
  }

  /** Walks up the tree to find the global scope. */
  @Override
  public final Scope getGlobalScope() {
    Scope result = this;
    while (result.getParent() != null) {
      result = result.getParent();
    }
    return result;
  }

  @Override
  public final Scope getParentScope() {
    return getParent();
  }

  /**
   * Undeclares a variable, to be used when the compiler optimizes out a variable and removes it
   * from the scope.
   */
  final void undeclare(Var var) {
    checkState(var.getScope() == this);
    checkState(vars.get(var.getName()).equals(var));
    undeclareInteral(var);
  }

  /** Without any safety checks */
  final void undeclareInteral(Var var) {
    // Assume that vars must contain var, and thus not be empty
    vars.remove(var.getName());
  }

  final void declareInternal(String name, Var var) {
    checkState(hasOwnSlot(name) || canDeclare(name), "Illegal shadow: %s", var.getNode());

    // For memory savings, only initialize the map once it needs to add its first element
    ImmutableMap<String, Var> emptySentinel = ImmutableMap.of();
    if (vars == emptySentinel) {
      vars = Maps.newLinkedHashMapWithExpectedSize(1);
    }
    vars.put(name, var);
  }

  @Override
  final void clearVarsInternal() {
    if (!vars.isEmpty()) {
      vars.clear();
    }
  }

  /** Returns true if a variable is declared in this scope, with no recursion. */
  @Override
  public final boolean hasOwnSlot(String name) {
    return vars.containsKey(name) || hasOwnImplicitSlot(ImplicitVar.of(name));
  }

  /** Returns true if a variable is declared in this or any parent scope. */
  @Override
  public final boolean hasSlot(String name) {
    for (Scope scope = this; scope != null; scope = scope.getParent()) {
      if (scope.hasOwnSlot(name)) {
        return true;
      }
    }
    return false;
  }

  private final @Nullable Var getOwnImplicitSlot(@Nullable ImplicitVar name) {
    if (!hasOwnImplicitSlot(name)) {
      return null;
    }

    // For memory savings, only initialize the map once it needs to add its first element
    ImmutableMap<ImplicitVar, Var> emptySentinel = ImmutableMap.of();
    if (implicitVars == emptySentinel) {
      implicitVars = new EnumMap<>(ImplicitVar.class);
    }

    return implicitVars.computeIfAbsent(name, this::makeImplicitVar);
  }

  @Override
  public final Var getOwnSlot(String name) {
    Var var = vars.get(name);
    if (var != null) {
      return var;
    }

    return getOwnImplicitSlot(ImplicitVar.of(name));
  }

  @Override
  public final Var getSlot(String name) {
    return getVar(name);
  }

  /**
   * Returns the variable, may be null
   *
   * <p>Non-final for {@link TypedScope} which needs to handle qualified names.
   */
  @Override
  public Var getVar(String name) {
    for (Scope scope = this; scope != null; scope = scope.getParent()) {
      @Nullable Var var = scope.getOwnSlot(name);
      if (var != null) {
        return var;
      }
    }
    return null;
  }

  /**
   * Get a unique Var object to represent "arguments" within this scope.
   *
   * <p>This explicitly excludes user declared variables that are names "arguments". It only returns
   * special "arguments" variable that is inherent to a function.
   */
  @Override
  public final Var getArgumentsVar() {
    for (Scope scope = this; scope != null; scope = scope.getParent()) {
      @Nullable Var arguments = scope.getOwnImplicitSlot(ImplicitVar.ARGUMENTS);
      if (arguments != null) {
        return arguments;
      }
    }
    return null;
  }

  /**
   * Returns true if the name can be declared on this scope without causing illegal shadowing.
   * Specifically, this is aware of the connection between function container scopes and function
   * block scopes and returns false for redeclaring parameters on the block scope.
   */
  @Override
  final boolean canDeclare(String name) {
    return !hasOwnSlot(name)
        && (!isFunctionBlockScope()
            || !getParent().hasOwnSlot(name)
            || isBleedingFunctionName(name));
  }

  /**
   * Returns true if the given name is a bleeding function name in this scope. Local variables in
   * the function block are not allowed to shadow parameters, but they are allowed to shadow a
   * bleeding function name.
   */
  private boolean isBleedingFunctionName(String name) {
    Var var = getVar(name);
    return var != null //
        && var.getNode() != null //
        && var.getNode().getParent().isFunction();
  }

  /**
   * Return an iterable over all of the variables declared in this scope (except the special
   * 'arguments' variable).
   */
  @Override
  public final Iterable<Var> getVarIterable() {
    return vars.values();
  }

  /**
   * Return an iterable over all of the variables accessible to this scope (i.e. the variables in
   * this scope and its parent scopes). Any variables declared in the local scope with the same name
   * as a variable declared in a parent scope gain precedence - if let x exists in the block scope,
   * a declaration let x from the parent scope would not be included because the parent scope's
   * variable gets shadowed.
   *
   * <p>The iterable contains variables from inner scopes before adding variables from outer parent
   * scopes.
   *
   * <p>We do not include the special 'arguments' variable.
   */
  @Override
  public final Iterable<Var> getAllAccessibleVariables() {
    Map<String, Var> accessibleVars = new LinkedHashMap<>();
    Scope s = this;

    while (s != null) {
      for (Var v : s.getVarIterable()) {
        accessibleVars.putIfAbsent(v.getName(), v);
      }
      s = s.getParent();
    }
    return accessibleVars.values();
  }

  @Override
  public final Iterable<Var> getAllSymbols() {
    return Collections.unmodifiableCollection(vars.values());
  }

  /** Returns number of variables in this scope (excluding the special 'arguments' variable) */
  @Override
  public final int getVarCount() {
    return vars.size();
  }

  /**
   * If a var were declared in this scope, return the scope it would be hoisted to.
   *
   * <p>For function scopes, we return back the scope itself, since even though there is no way to
   * declare a var inside function parameters, it would make even less sense to say that such
   * declarations would be "hoisted" somewhere else.
   */
  @Override
  public final Scope getClosestHoistScope() {
    Scope current = this;
    while (current != null) {
      if (current.isHoistScope()) {
        return current;
      }
      current = current.getParent();
    }
    return null;
  }

  @Override
  public final Scope getClosestCfgRootScope() {
    Scope current = this;
    while (!current.isCfgRootScope()) {
      current = current.getParent();
    }
    return current;
  }

  /**
   * Returns the closest container scope. This is equivalent to what the current scope would have
   * been for non-ES6 scope creators, and is thus useful for migrating code to use block scopes.
   */
  @Override
  public final Scope getClosestContainerScope() {
    Scope scope = getClosestHoistScope();
    if (scope.isFunctionBlockScope()) {
      scope = scope.getParent();
      checkState(!scope.isBlockScope(), scope);
    }
    return scope;
  }

  /** Performs simple validity checks on when constructing a child scope. */
  final void checkChildScope(Scope parent) {
    checkNotNull(parent);
    checkArgument(NodeUtil.createsScope(rootNode), rootNode);
    checkArgument(
        rootNode != parent.getRootNode(),
        "rootNode should not be the parent's root node: %s",
        rootNode);
  }

  /** Returns the nearest common parent between two scopes. */
  Scope getCommonParent(Scope other) {
    Scope left = this;
    Scope right = other;
    while (left != null && right != null && left != right) {
      int leftDepth = left.getDepth();
      int rightDepth = right.getDepth();
      if (leftDepth >= rightDepth) {
        left = left.getParent();
      }
      if (leftDepth <= rightDepth) {
        right = right.getParent();
      }
    }

    checkState(left != null && left == right);
    return left;
  }

  /**
   * Whether {@code this} and the {@code other} scope have the same container scope (i.e. are in the
   * same function, or else both in the global hoist scope). This is equivalent to asking whether
   * the two scopes would be equivalent in a pre-ES2015-block-scopes view of the world.
   */
  boolean hasSameContainerScope(Scope other) {
    // Do identity check first as a shortcut.
    return this == other || getClosestContainerScope() == other.getClosestContainerScope();
  }

  /**
   * Returns the closest scope upon which `this` is defined, which is either the global scope or a
   * non-arrow-function scope.
   */
  @Override
  final Scope getScopeOfThis() {
    Scope scope = getClosestContainerScope();
    while (!scope.isGlobal() && !NodeUtil.isNonArrowFunction(scope.getRootNode())) {
      scope = scope.getParent().getClosestContainerScope();
    }
    return scope;
  }
}
