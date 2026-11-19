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
import static com.google.common.base.Preconditions.checkNotNull;

import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticScope;
import java.io.Serializable;
import org.jspecify.annotations.Nullable;

/**
 * Scope contains information about a variable scope in JavaScript. Scopes can be nested, a scope
 * points back to its parent scope. A Scope contains information about variables defined in that
 * scope.
 *
 * <p>ES 2015 introduces new scoping rules, which adds some complexity to this class. In particular,
 * scopes fall into two mutually exclusive categories: <i>block</i> and <i>container</i>. Block
 * scopes are all scopes whose roots are blocks, as well as any control structures whose optional
 * blocks are omitted. These scopes did not exist at all prior to ES 2015. Container scopes comprise
 * function scopes, global scopes, and module scopes, and (aside from modules, which didn't exist in
 * ES5) corresponds to the ES5 scope rules. This corresponds roughly to one container scope per CFG
 * root (but not exactly, due to SCRIPT-level CFGs).
 *
 * <p>All container scopes, plus the outermost block scope within a function (i.e. the <i>function
 * block scope</i>) are considered <i>hoist scopes</i>. All functions thus have two hoist scopes:
 * the function scope and the function block scope. Hoist scopes are relevant because "var"
 * declarations are hoisted to the closest hoist scope, as opposed to "let" and "const" which always
 * apply to the specific scope in which they occur.
 *
 * <p>Note that every function actually has two distinct hoist scopes: a container scope on the
 * FUNCTION node, and a block-scope on the top-level BLOCK in the function (the "function block").
 * Local variables are declared on the function block, while parameters and optionally the function
 * name (if it bleeds, i.e. from a named function expression) are declared on the container scope.
 * This is required so that default parameter initializers can refer to names from outside the
 * function that could possibly be shadowed in the function block. But these scopes are not fully
 * independent of one another, since the language does not allow a top-level local variable to
 * shadow a parameter name - so in some situations these scopes must be treated as a single scope.
 *
 * @see NodeTraversal
 */
public abstract class AbstractScope
    implements StaticScope, Serializable {
  final Node rootNode;

  AbstractScope(Node rootNode) {
    this.rootNode = checkNotNull(rootNode);
  }

  /** The depth of the scope. The global scope has depth 0. */
  public abstract int getDepth();

  /** Returns the parent scope, or null if this is the global scope. */
  public abstract AbstractScope getParent();

  @Override
  public final String toString() {
    return "Scope@" + rootNode;
  }

  public Scope untyped() {
    throw new IllegalStateException("untyped() called, but not an untyped scope.");
  }

  public TypedScope typed() {
    throw new IllegalStateException("typed() called, but not a typed scope.");
  }

  /**
   * Gets the container node of the scope. This is typically the FUNCTION node or the global
   * BLOCK/SCRIPT node.
   */
  @Override
  public final Node getRootNode() {
    return rootNode;
  }

  /** Walks up the tree to find the global scope. */
  public abstract AbstractScope getGlobalScope();

  public abstract AbstractScope getParentScope();

  abstract AbstractVar makeImplicitVar(ImplicitVar type);

  abstract void clearVarsInternal();

  /** Returns true iff this scope implies a slot with the given name. */
  protected boolean hasOwnImplicitSlot(@Nullable ImplicitVar name) {
    return name != null && name.isMadeByScope(this);
  }

  /** Returns true if a variable is declared in this scope, with no recursion. */
  public abstract boolean hasOwnSlot(String name);

  /** Returns true if a variable is declared in this or any parent scope. */
  public abstract boolean hasSlot(String name);

  public abstract AbstractVar getOwnSlot(String name);

  public abstract AbstractVar getSlot(String name);

  /**
   * Returns the variable, may be null
   *
   * <p>Non-final for {@link TypedScope} which needs to handle qualified names.
   */
  public abstract AbstractVar getVar(String name);

  /**
   * Get a unique Var object to represent "arguments" within this scope.
   *
   * <p>This explicitly excludes user declared variables that are names "arguments". It only returns
   * special "arguments" variable that is inherent to a function.
   */
  public abstract AbstractVar getArgumentsVar();

  /**
   * Returns true if the name can be declared on this scope without causing illegal shadowing.
   * Specifically, this is aware of the connection between function container scopes and function
   * block scopes and returns false for redeclaring parameters on the block scope.
   */
  abstract boolean canDeclare(String name);

  /**
   * Return an iterable over all of the variables declared in this scope (except the special
   * 'arguments' variable).
   */
  public abstract Iterable<? extends AbstractVar> getVarIterable();

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
  public abstract Iterable<? extends AbstractVar> getAllAccessibleVariables();

  public abstract Iterable<? extends AbstractVar> getAllSymbols();

  /** Returns number of variables in this scope (excluding the special 'arguments' variable) */
  public abstract int getVarCount();

  /** Returns whether this is the global scope. */
  public final boolean isGlobal() {
    return getParent() == null;
  }

  /** Returns whether this is a local scope (i.e. not the global scope). */
  public final boolean isLocal() {
    return getParent() != null;
  }

  public final boolean isBlockScope() {
    return NodeUtil.createsBlockScope(rootNode);
  }

  public final boolean isStaticBlockScope() {
    return NodeUtil.isClassStaticBlock(getRootNode());
  }

  public final boolean isFunctionBlockScope() {
    return NodeUtil.isFunctionBlock(getRootNode());
  }

  public final boolean isFunctionScope() {
    return getRootNode().isFunction();
  }

  public final boolean isModuleScope() {
    return getRootNode().isModuleBody();
  }

  public final boolean isMemberFieldDefScope() {
    return getRootNode().isMemberFieldDef();
  }

  public final boolean isComputedFieldDefRhsScope() {
    return getRootNode().isComputedFieldDef();
  }

  public final boolean isCatchScope() {
    return getRootNode().isBlock()
        && getRootNode().hasOneChild()
        && getRootNode().getFirstChild().isCatch();
  }

  public final boolean isCfgRootScope() {
    return NodeUtil.isValidCfgRoot(rootNode);
  }

  /**
   * If a var were declared in this scope, would it belong to this scope (as opposed to some
   * enclosing scope)?
   *
   * <p>We consider function scopes to be hoist scopes. Even though it's impossible to declare a var
   * inside function parameters, it would make less sense to say that if you did declare one in the
   * function parameters, it would be hoisted somewhere else.
   */
  final boolean isHoistScope() {
    return isFunctionScope()
        || isFunctionBlockScope()
        || isGlobal()
        || isModuleScope()
        || isStaticBlockScope();
  }

  /**
   * If a var were declared in this scope, return the scope it would be hoisted to.
   *
   * <p>For function scopes, we return back the scope itself, since even though there is no way to
   * declare a var inside function parameters, it would make even less sense to say that such
   * declarations would be "hoisted" somewhere else.
   */
  public abstract AbstractScope getClosestHoistScope();

  public abstract AbstractScope getClosestCfgRootScope();

  /**
   * Returns the closest container scope. This is equivalent to what the current scope would have
   * been for non-ES6 scope creators, and is thus useful for migrating code to use block scopes.
   */
  public abstract AbstractScope getClosestContainerScope();

  /** Performs simple validity checks on when constructing a root scope. */
  final void checkRootScope() {
    // TODO(tbreisacher): Can we tighten this to just NodeUtil.createsScope?
    checkArgument(
        NodeUtil.createsScope(rootNode) || rootNode.isScript() || rootNode.isRoot(), rootNode);
  }

  /**
   * Returns the closest scope upon which `this` is defined, which is either the global scope or a
   * non-arrow-function scope.
   */
  abstract AbstractScope getScopeOfThis();

  /**
   * The three implicit var types, which are defined implicitly (at least) in every vanilla function
   * scope without actually being declared.
   */
  enum ImplicitVar {
    ARGUMENTS("arguments"),
    EXPORTS("exports"),
    SUPER("super"),
    // TODO(sdh): Expand THIS.isMadeByScope to check super.isMadeByScope(scope) || scope.isGlobal()
    // Currently this causes a number of problems (see b/74980936), but could eventually lead to
    // better type information.  We might also want to restrict this so that module-root scopes
    // explicitly *don't* have access to the global this, though I think this is more than just
    // returning false in isMadeByScope - rather, getVar() needs to stop checking and immediately
    // return null.
    THIS("this");

    final String nameField;

    ImplicitVar(String name) {
      this.nameField = name;
    }

    /** Whether this kind of implicit variable is created/owned by the given scope. */
    boolean isMadeByScope(AbstractScope scope) {
      return switch (this) {
        case EXPORTS ->
            scope.isModuleScope()
                && scope.getRootNode().getParent().getBooleanProp(Node.GOOG_MODULE);
        case SUPER, THIS ->
            scope.isStaticBlockScope()
                || NodeUtil.isNonArrowFunction(scope.getRootNode())
                || scope.isMemberFieldDefScope()
                || scope.isComputedFieldDefRhsScope();
        case ARGUMENTS -> NodeUtil.isNonArrowFunction(scope.getRootNode());
      };
    }

    static @Nullable ImplicitVar of(String name) {
      return switch (name) {
        case "arguments" -> ARGUMENTS;
        case "super" -> SUPER;
        case "this" -> THIS;
        case "exports" -> EXPORTS;
        default -> null;
      };
    }
  }
}
