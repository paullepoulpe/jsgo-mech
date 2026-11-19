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
import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Iterables;
import com.google.common.collect.Maps;
import com.google.javascript.jscomp.modules.Module;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticScope;
import com.google.javascript.rhino.jstype.FunctionType;
import com.google.javascript.rhino.jstype.JSType;
import com.google.javascript.rhino.jstype.ObjectType;
import com.google.javascript.rhino.jstype.StaticTypedScope;
import java.util.Collections;
import java.util.EnumMap;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/**
 * TypedScope contains information about variables and their types. Scopes can be nested; a scope
 * points back to its parent scope.
 *
 * <p>TypedScope is also used as a lattice element for flow-sensitive type inference. As a lattice
 * element, a scope is viewed as a map from names to types. A name not in the map is considered to
 * have the bottom type. The join of two maps m1 and m2 is the map of the union of names with {@link
 * JSType#getLeastSupertype} to meet the m1 type and m2 type.
 *
 * <p>TypedScopes also have a concept of a set of 'reserved' names. These are names that will be
 * declared as actual TypedVars later in the compilation. Looking up any of these reserved names
 * will always return null, even if the name is available in a parent scope.
 *
 * @see NodeTraversal
 * @see DataFlowAnalysis
 *     <p>Several methods in this class, such as {@code isBlockScope} throw an exception when
 *     called. The reason for this is that we want to shadow methods from the parent class, to avoid
 *     calling them accidentally.
 */
public class TypedScope extends AbstractScope implements StaticTypedScope {

  private final @Nullable TypedScope parent;
  private final int depth;
  private final @Nullable Module module;

  /** Whether this is a bottom scope for the purposes of type inference. */
  private final boolean isBottomField;

  // Will shrink over time. Set to "ImmutableSet.of()" once empty (so don't try removing
  // elements from an empty set)
  private Set<String> reservedNames;

  // Scope.java contains an arguments field.
  // We haven't added it here because it's unused by the passes that need typed scopes.

  static TypedScope createFromParent(TypedScope parent, Node rootNode) {
    return new TypedScope(parent, rootNode, new LinkedHashSet<>(), null);
  }

  /**
   * Creates a new global or local TypedScope.
   *
   * @param parent The containing scope, unless this is the global scope.
   * @param rootNode Can be any node that creates a scope.
   * @param reservedNames set of simple names that will eventually be declared in this scope.
   */
  TypedScope(TypedScope parent, Node rootNode, Set<String> reservedNames, @Nullable Module module) {
    super(rootNode);
    checkChildScope(parent);
    this.parent = parent;
    this.depth = parent.depth + 1;
    this.isBottomField = false;
    this.reservedNames =
        reservedNames.isEmpty() ? ImmutableSet.of() : new LinkedHashSet<>(reservedNames);
    this.module = module;
  }

  /**
   * Creates a empty Scope (bottom of the lattice).
   *
   * @param rootNode Typically a FUNCTION node or the global ROOT.
   * @param isBottom Whether this is the bottom of a lattice. Otherwise, it must be a global scope.
   */
  private TypedScope(Node rootNode, boolean isBottom) {
    super(rootNode);
    checkRootScope();
    this.parent = null;
    this.depth = 0;
    this.isBottomField = isBottom;
    this.reservedNames = ImmutableSet.of();
    this.module = null;
  }

  static TypedScope createGlobalScope(Node rootNode) {
    return new TypedScope(rootNode, false);
  }

  static TypedScope createLatticeBottom(Node rootNode) {
    return new TypedScope(rootNode, true);
  }

  @Override
  public TypedScope typed() {
    return this;
  }

  /** Asserts that all reserved names in the scope have been actually declared. */
  void validateCompletelyBuilt() {
    checkState(
        reservedNames.isEmpty(),
        "Expected %s to have no reserved names, found: %s. This probably indicates a bug in"
            + " TypedScopeCreator where it is failing to declare a variable.",
        this,
        reservedNames);
    // let a (16-bit) VM garbage collect 64 bytes per TypedScope. (ImmutableSet.of() returns a
    // singleton)
    reservedNames = ImmutableSet.of();
  }

  /** Whether this is the bottom of the lattice. */
  boolean isBottom() {
    return isBottomField;
  }

  @Nullable
  Module getModule() {
    return this.module;
  }

  @Override
  public int getDepth() {
    return depth;
  }

  @Override
  public TypedScope getParent() {
    return parent;
  }

  /** Gets the type of {@code this} in the current scope. */
  @Override
  public @Nullable JSType getTypeOfThis() {
    Node root = getRootNode();
    if (isGlobal()) {
      return ObjectType.cast(root.getJSType());
    } else if (NodeUtil.isNonArrowFunction(root)) {
      JSType nodeType = root.getJSType();
      if (nodeType != null && nodeType.isFunctionType()) {
        return nodeType.toMaybeFunctionType().getTypeOfThis();
      } else {
        // Executed when the current scope has not been typechecked.
        return null;
      }
    } else if (this.isStaticBlockScope()) {
      return getParent().getRootNode().getJSType();
    } else if (this.isMemberFieldDefScope() || this.isComputedFieldDefRhsScope()) {
      JSType classType = getParent().getRootNode().getJSType();
      if (root.isStaticMember()) {
        return classType;
      } else {
        return classType.assertFunctionType().getInstanceType();
      }
    } else {
      return getParent().getTypeOfThis();
    }
  }

  TypedVar declare(String name, Node nameNode,
      JSType type, CompilerInput input, boolean inferred) {
    checkState(!name.isEmpty());
    if (!reservedNames.isEmpty()) {
      // (reservedNames only contains simple, non-qualified names; so it's completely normal to
      // declare qualified names that were not 'reserved')
      reservedNames.remove(name);
    }
    TypedVar var = new TypedVar(inferred, name, nameNode, type, this, getVarCount(), input);
    declareInternal(name, var);
    return var;
  }

  @Override
  @Nullable TypedVar makeImplicitVar(ImplicitVar var) {
    if (this.isGlobal()) {
      // TODO(sdh): This is incorrect for 'global this', but since that's currently not handled
      // by this code, it's okay to bail out now until we find the root cause.  See b/74980936.
      return null;
    } else if (var.equals(ImplicitVar.EXPORTS)) {
      // Instead of using the implicit 'exports' var, we want to pretend that the var is actually
      // declared.
      return null;
    }
    return new TypedVar(false, var.nameField, null, getImplicitVarType(var), this, -1, null);
  }

  @Override
  protected boolean hasOwnImplicitSlot(@Nullable ImplicitVar name) {
    return name != null && !name.equals(ImplicitVar.EXPORTS) && name.isMadeByScope(this);
  }

  private @Nullable JSType getImplicitVarType(ImplicitVar var) {
    switch (var) {
      case ARGUMENTS -> {
        // Look for an extern named "arguments" and use its type if available.
        // TODO(sdh): consider looking for "Arguments" ctor rather than "arguments" var: this could
        // allow deleting the variable, which doesn't really belong in externs in the first place.
        TypedVar globalArgs = getGlobalScope().getVar(Var.ARGUMENTS);
        return globalArgs != null && globalArgs.isExtern() ? globalArgs.getType() : null;
      }
      case THIS -> {
        return getTypeOfThis();
      }
      case SUPER -> {
        // Inside a constructor, `super` may have two different types. Calls to `super()` use the
        // super-ctor type, while property accesses use the super-instance type. This logic always
        // returns the latter case.
        ObjectType receiverType = ObjectType.cast(getTypeOfThis());
        if (receiverType == null) {
          return null;
        } else if (receiverType.isInstanceType()) {
          FunctionType superclassCtor = receiverType.getSuperClassConstructor();
          return superclassCtor == null ? null : superclassCtor.getInstanceType();
        } else {
          return receiverType.getImplicitPrototype();
        }
      }
      case EXPORTS ->
          throw new AssertionError("TypedScopes should not contain an implicit 'exports'");
    }

    throw new AssertionError();
  }

  /**
   * Returns the variables in this scope that have been declared with 'var' and not yet declared
   * with a known type.
   *
   * <p>These variables can safely be set to undefined (rather than unknown) at the start of type
   * inference, and will be reset to the correct type when analyzing the first assignment to them.
   * Parameters and externs are excluded because they are not initialized in the function body, and
   * lexically-bound variables (let and const) are excluded because they are initialized when
   * inferring the LET/CONST node, which is guaranteed to occur before any use, since they are not
   * hoisted.
   */
  public Iterable<TypedVar> getDeclarativelyUnboundVarsWithoutTypes() {
    return Iterables.filter(getVarIterable(), this::isDeclarativelyUnboundVarWithoutType);
  }

  private boolean isDeclarativelyUnboundVarWithoutType(TypedVar var) {
    return var.getParentNode() != null
        && var.getType() == null
        // TODO(bradfordcsmith): update this for destructuring
        && var.getParentNode().isVar()  // NOTE: explicitly excludes let/const
        && !var.isExtern();
  }

  /**
   * Returns the slot for {@code name}, considering shadowing of qualified names.
   *
   * <p>The superclass method does not handle shadowing.
   *
   * <p>Lookup of qualified names (i.e. names with dots) executes against scopes in the following
   * precedence:
   *
   * <ol>
   *   <li>This {@link Scope}.
   *   <li>The first ancestor {@link Scope}, if any, that declares the root of the qualified name.
   *   <li>The global {@link Scope}.
   * </ol>
   *
   * <p>An example of where this is necessary: say the global scope contains "a" and "a.b" and a
   * function scope contains "a". When looking up "a.b" in the function scope, AbstractScope::getVar
   * returns "a.b". This method returns null because the global "a" is shadowed.
   */
  @Override
  public final @Nullable TypedVar getVar(String name) {
    TypedVar ownSlot = getOwnSlot(name);
    if (ownSlot != null) {
      // Micro-optimization: variables declared directly in this scope cannot have been shadowed.
      return ownSlot;
    } else if (this.getParent() == null) {
      return null;
    }
    // Find the root name and its slot.
    int dot = name.indexOf('.');
    String rootName = dot < 0 ? name : name.substring(0, dot);
    TypedVar rootVar = getVarSuper(rootName); // Use the superclass method to skip string checks.

    if (dot < 0) {
      return rootVar;
    } else if (rootVar == null) {
      // Default to the global scope because externs may have qualified names with undeclared roots.
      return this.getParent().getGlobalScope().getOwnSlot(name);
    } else {
      // Qualified names 'a.b.c' are declared in the same scope as 'a', never a child scope, which
      // is why calling `getOwnSlot` is sufficient.
      return rootVar.getScope().getOwnSlot(name);
    }
  }

  final @Nullable JSType getTypeThroughNamespace(String moduleId) {
    int split = moduleId.lastIndexOf('.');
    if (split >= 0) {
      String parentName = moduleId.substring(0, split);
      String prop = moduleId.substring(split + 1);
      JSType parentType = getTypeThroughNamespace(parentName);
      if (parentType == null || parentType.toMaybeObjectType() == null) {
        return null;
      }
      return parentType.assertObjectType().getPropertyType(prop);
    } else {
      TypedVar var = this.getSlot(moduleId);
      return var != null ? var.getType() : null;
    }
  }

  @Override
  public @Nullable StaticScope getTopmostScopeOfEventualDeclaration(String name) {
    if (getOwnSlot(name) != null || reservedNames.contains(name)) {
      return this;
    } else if (this.getParent() == null) {
      return null;
    } else {
      // Recurse on the parent because it, too, may be incomplete.
      return this.getParent().getTopmostScopeOfEventualDeclaration(name);
    }
  }

  // Members pushed down from AbstractScope (prep 17), with its type parameters replaced.

  private Map<String, TypedVar> vars = ImmutableMap.of();

  private Map<ImplicitVar, TypedVar> implicitVars = ImmutableMap.of();

  /**
   * @return True if this scope contains {@code other}, or is the same scope as {@code other}.
   */
  final boolean contains(TypedScope other) {
    TypedScope s = checkNotNull(other);
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
  public final TypedScope getGlobalScope() {
    TypedScope result = this;
    while (result.getParent() != null) {
      result = result.getParent();
    }
    return result;
  }

  @Override
  public final TypedScope getParentScope() {
    return getParent();
  }

  /**
   * Undeclares a variable, to be used when the compiler optimizes out a variable and removes it
   * from the scope.
   */
  final void undeclare(TypedVar var) {
    checkState(var.getScope() == this);
    checkState(vars.get(var.getName()).equals(var));
    undeclareInteral(var);
  }

  /** Without any safety checks */
  final void undeclareInteral(TypedVar var) {
    // Assume that vars must contain var, and thus not be empty
    vars.remove(var.getName());
  }

  final void declareInternal(String name, TypedVar var) {
    checkState(hasOwnSlot(name) || canDeclare(name), "Illegal shadow: %s", var.getNode());

    // For memory savings, only initialize the map once it needs to add its first element
    ImmutableMap<String, TypedVar> emptySentinel = ImmutableMap.of();
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
    for (TypedScope scope = this; scope != null; scope = scope.getParent()) {
      if (scope.hasOwnSlot(name)) {
        return true;
      }
    }
    return false;
  }

  private final @Nullable TypedVar getOwnImplicitSlot(@Nullable ImplicitVar name) {
    if (!hasOwnImplicitSlot(name)) {
      return null;
    }

    // For memory savings, only initialize the map once it needs to add its first element
    ImmutableMap<ImplicitVar, TypedVar> emptySentinel = ImmutableMap.of();
    if (implicitVars == emptySentinel) {
      implicitVars = new EnumMap<>(ImplicitVar.class);
    }

    return implicitVars.computeIfAbsent(name, this::makeImplicitVar);
  }

  @Override
  public final TypedVar getOwnSlot(String name) {
    TypedVar var = vars.get(name);
    if (var != null) {
      return var;
    }

    return getOwnImplicitSlot(ImplicitVar.of(name));
  }

  @Override
  public final TypedVar getSlot(String name) {
    return getVar(name);
  }

  /**
   * Returns the variable, may be null
   *
   * <p>Non-final for {@link TypedScope} which needs to handle qualified names.
   */
  private TypedVar getVarSuper(String name) {
    for (TypedScope scope = this; scope != null; scope = scope.getParent()) {
      @Nullable TypedVar var = scope.getOwnSlot(name);
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
  public final TypedVar getArgumentsVar() {
    for (TypedScope scope = this; scope != null; scope = scope.getParent()) {
      @Nullable TypedVar arguments = scope.getOwnImplicitSlot(ImplicitVar.ARGUMENTS);
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
    TypedVar var = getVar(name);
    return var != null //
        && var.getNode() != null //
        && var.getNode().getParent().isFunction();
  }

  /**
   * Return an iterable over all of the variables declared in this scope (except the special
   * 'arguments' variable).
   */
  @Override
  public final Iterable<TypedVar> getVarIterable() {
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
  public final Iterable<TypedVar> getAllAccessibleVariables() {
    Map<String, TypedVar> accessibleVars = new LinkedHashMap<>();
    TypedScope s = this;

    while (s != null) {
      for (TypedVar v : s.getVarIterable()) {
        accessibleVars.putIfAbsent(v.getName(), v);
      }
      s = s.getParent();
    }
    return accessibleVars.values();
  }

  @Override
  public final Iterable<TypedVar> getAllSymbols() {
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
  public final TypedScope getClosestHoistScope() {
    TypedScope current = this;
    while (current != null) {
      if (current.isHoistScope()) {
        return current;
      }
      current = current.getParent();
    }
    return null;
  }

  @Override
  public final TypedScope getClosestCfgRootScope() {
    TypedScope current = this;
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
  public final TypedScope getClosestContainerScope() {
    TypedScope scope = getClosestHoistScope();
    if (scope.isFunctionBlockScope()) {
      scope = scope.getParent();
      checkState(!scope.isBlockScope(), scope);
    }
    return scope;
  }

  /** Performs simple validity checks on when constructing a child scope. */
  final void checkChildScope(TypedScope parent) {
    checkNotNull(parent);
    checkArgument(NodeUtil.createsScope(rootNode), rootNode);
    checkArgument(
        rootNode != parent.getRootNode(),
        "rootNode should not be the parent's root node: %s",
        rootNode);
  }

  /** Returns the nearest common parent between two scopes. */
  TypedScope getCommonParent(TypedScope other) {
    TypedScope left = this;
    TypedScope right = other;
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
  boolean hasSameContainerScope(TypedScope other) {
    // Do identity check first as a shortcut.
    return this == other || getClosestContainerScope() == other.getClosestContainerScope();
  }

  /**
   * Returns the closest scope upon which `this` is defined, which is either the global scope or a
   * non-arrow-function scope.
   */
  @Override
  final TypedScope getScopeOfThis() {
    TypedScope scope = getClosestContainerScope();
    while (!scope.isGlobal() && !NodeUtil.isNonArrowFunction(scope.getRootNode())) {
      scope = scope.getParent().getClosestContainerScope();
    }
    return scope;
  }
}
