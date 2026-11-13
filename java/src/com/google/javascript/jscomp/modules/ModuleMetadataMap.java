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

package com.google.javascript.jscomp.modules;

import static java.util.Objects.requireNonNull;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.javascript.jscomp.base.SimpleMultiset;
import com.google.javascript.jscomp.deps.ModuleLoader.ModulePath;
import com.google.javascript.rhino.Node;
import java.util.Map;
import org.jspecify.annotations.Nullable;

/**
 * Contains metadata around modules (or scripts) that is useful for checking imports / requires.
 *
 * <p>TODO(johnplaisted): There's an opportunity for reuse here in ClosureRewriteModules, which
 * would involve putting this in some common location. Currently this is only used as a helper class
 * for Es6RewriteModules. CompilerInput already has some (not all) of this information but it is not
 * always populated. It may also be ideal to include CommonJS here too as ES6 modules can import
 * them. That would allow decoupling of how these modules are written; right now Es6RewriteModule
 * only checks this for goog.requires and goog: imports, not for ES6 path imports.
 */
public final class ModuleMetadataMap {
  /** Various types of Javascript modules and scripts that can be found in the JS Compiler. */
  public enum ModuleType {
    ES6_MODULE("an ES6 module"),
    /** A goog.module that does not declare a legacy namespace. */
    GOOG_MODULE("a goog.module"),
    /** A goog.module that declares a legacy namespace with goog.module.declareLegacyNamespace. */
    LEGACY_GOOG_MODULE("a goog.module"),
    COMMON_JS("a CommonJS module"),
    // The following two cases are not actually modules, but are useful to include in the map.
    GOOG_PROVIDE("a script file that contains at least one goog.provide"),
    SCRIPT("a script file that does not contain a goog.provide");

    public final String description;

    ModuleType(String description) {
      this.description = description;
    }
  }

  /**
   * Map from module path to module. These modules represent files and thus will contain all goog
   * namespaces that are in the file. These are not the same modules in modulesByGoogNamespace.
   */
  private final ImmutableMap<String, ModuleMetadata> modulesByPath;

  /**
   * Map from Closure namespace to module. These modules represent just the single namespace and
   * thus each module has only one goog namespace in its {@link ModuleMetadata#googNamespaces()}.
   * These are not the same modules in modulesByPath.
   */
  private final ImmutableMap<String, ModuleMetadata> modulesByGoogNamespace;

  private final ImmutableSet<ModuleMetadata> moduleMetadata;

  public ModuleMetadataMap(
      Map<String, ModuleMetadata> modulesByPath,
      Map<String, ModuleMetadata> modulesByGoogNamespace) {
    this.modulesByPath = ImmutableMap.copyOf(modulesByPath);
    this.modulesByGoogNamespace = ImmutableMap.copyOf(modulesByGoogNamespace);
    this.moduleMetadata =
        ImmutableSet.<ModuleMetadata>builder()
            .addAll(modulesByPath.values())
            .addAll(modulesByGoogNamespace.values())
            .build();
  }

  /**
   * Struct containing basic information about a module/script including its type and goog
   * namespaces.
   *
   * @param rootNode AST node that represents the root of this module.
   *     <p>May be null if this is a synthetic piece of metadata - e.g. in a test, or something used
   *     as a fallback.
   * @param usesClosure Whether this file uses Closure Library at all. Note that a file could use
   *     Closure Library even without calling goog.provide/module/require - there are some
   *     primitives in base.js that can be used without being required like goog.isArray.
   *     <p>If this is true this indicates the base.js is needed and is not part of this script - it
   *     is an EXTERNAL dependencym otherwise false. If this is also false if Closure Library is
   *     part of this script - e.g. a bundle with base.js. So something could be a "goog.provide'd
   *     file", but not use Closure if the bundle already contains Closure.
   * @param isTestOnly Whether goog.setTestOnly was called.
   * @param googNamespaces Closure namespaces that this file is associated with. Created by
   *     goog.provide, goog.module, and goog.declareModuleId.
   *     <p>This is a multiset as it does not warn on duplicate namespaces, but will still
   *     encapsulate that information with this multiset.
   * @param stronglyRequiredGoogNamespaces Closure namespaces this file strongly requires, i.e.,
   *     arguments to goog.require calls.
   *     <p>This is a multiset as it does not warn on duplicate namespaces, but will still
   *     encapsulate that information with this multiset.
   * @param dynamicallyRequiredGoogNamespaces Closure namespaces this file dynamically require,
   *     i.e., arguments to goog.requireDynamic() calls.
   *     <p>This is a multiset as it does not warn on duplicate namespaces, but will still
   *     encapsulate that information with this multiset.
   * @param maybeRequiredGoogNamespaces Closure namespaces this file "maybe" require, i.e.,
   *     arguments to goog.maybeRequireFrameworkInternalOnlyDoNotCallOrElse() calls.
   *     <p>This is a multiset as it does not warn on duplicate namespaces, but will still
   *     encapsulate that information with this multiset.
   * @param weaklyRequiredGoogNamespaces Closure namespaces this file weakly requires, i.e.,
   *     arguments to goog.requireType calls.
   *     <p>This is a multiset as it does not warn on duplicate namespaces, but will still
   *     encapsulate that information with this multiset.
   * @param es6ImportSpecifiers Raw text of all ES6 import specifiers (includes "export from" as
   *     well).
   * @param readToggles Arguments to goog.readToggleInternalDoNotCallDirectly() calls.
   *     <p>This is a multiset as it does not warn on duplicate toggles, but will still encapsulate
   *     that information with this multiset.
   */
  public record ModuleMetadata(
      ModuleType moduleType,
      @Nullable Node rootNode,
      boolean usesClosure,
      boolean isTestOnly,
      SimpleMultiset<String> googNamespaces,
      SimpleMultiset<String> stronglyRequiredGoogNamespaces,
      SimpleMultiset<String> dynamicallyRequiredGoogNamespaces,
      SimpleMultiset<String> maybeRequiredGoogNamespaces,
      SimpleMultiset<String> weaklyRequiredGoogNamespaces,
      SimpleMultiset<String> es6ImportSpecifiers,
      ImmutableList<ModuleMetadata> nestedModules,
      SimpleMultiset<String> readToggles,
      @Nullable ModulePath path) {
    public ModuleMetadata {
      requireNonNull(moduleType, "moduleType");
      requireNonNull(googNamespaces, "googNamespaces");
      requireNonNull(stronglyRequiredGoogNamespaces, "stronglyRequiredGoogNamespaces");
      requireNonNull(dynamicallyRequiredGoogNamespaces, "dynamicallyRequiredGoogNamespaces");
      requireNonNull(maybeRequiredGoogNamespaces, "maybeRequiredGoogNamespaces");
      requireNonNull(weaklyRequiredGoogNamespaces, "weaklyRequiredGoogNamespaces");
      requireNonNull(es6ImportSpecifiers, "es6ImportSpecifiers");
      requireNonNull(nestedModules, "nestedModules");
      requireNonNull(readToggles, "readToggles");
    }

    @Override
    public String toString() {
      return "ModuleMetadata{"
          + "moduleType=" + moduleType + ", "
          + "rootNode=" + rootNode + ", "
          + "usesClosure=" + usesClosure + ", "
          + "isTestOnly=" + isTestOnly + ", "
          + "googNamespaces=" + googNamespaces + ", "
          + "stronglyRequiredGoogNamespaces=" + stronglyRequiredGoogNamespaces + ", "
          + "dynamicallyRequiredGoogNamespaces=" + dynamicallyRequiredGoogNamespaces + ", "
          + "maybeRequiredGoogNamespaces=" + maybeRequiredGoogNamespaces + ", "
          + "weaklyRequiredGoogNamespaces=" + weaklyRequiredGoogNamespaces + ", "
          + "es6ImportSpecifiers=" + es6ImportSpecifiers + ", "
          + "nestedModules=" + nestedModules + ", "
          + "readToggles=" + readToggles + ", "
          + "path=" + path
          + "}";
    }

    public boolean isEs6Module() {
      return moduleType() == ModuleType.ES6_MODULE;
    }

    public boolean isGoogModule() {
      return isNonLegacyGoogModule() || isLegacyGoogModule();
    }

    public boolean isNonLegacyGoogModule() {
      return moduleType() == ModuleType.GOOG_MODULE;
    }

    public boolean isLegacyGoogModule() {
      return moduleType() == ModuleType.LEGACY_GOOG_MODULE;
    }

    public boolean isGoogProvide() {
      return moduleType() == ModuleType.GOOG_PROVIDE;
    }

    public boolean hasLegacyGoogNamespaces() {
      return isGoogProvide() || isLegacyGoogModule();
    }

    public boolean isCommonJs() {
      return moduleType() == ModuleType.COMMON_JS;
    }

    public boolean isNonProvideScript() {
      return moduleType() == ModuleType.SCRIPT;
    }

    /** Whether this is a module (with it's own local scope). */
    public boolean isModule() {
      return switch (moduleType()) {
        case GOOG_PROVIDE, SCRIPT -> false;
        case COMMON_JS, ES6_MODULE, GOOG_MODULE, LEGACY_GOOG_MODULE -> true;
      };
    }

    public static Builder builder() {
      return new Builder();
    }

    // Use reference equality to prevent bad LinkedHashSet<ModuleMetadata> performance on GWT.
    // GatherModuleMetadata is guaranteed to create exactly one ModuleMetadata instance for each
    // input module.
    // NOTE(user): consider removing this override now that GWT & J2CL builds are gone.
    @Override
    public final boolean equals(Object other) {
      return this == other;
    }

    @Override
    public final int hashCode() {
      return System.identityHashCode(this);
    }

    /** Builder for {@link ModuleMetadata}. */
    public static final class Builder {
      private ModuleMetadataMap.@Nullable ModuleType moduleTypeField;
      private @Nullable Node rootNodeField;
      private boolean usesClosureField;
      private boolean isTestOnlyField;
      private @Nullable SimpleMultiset<String> googNamespacesBuilderField;
      private @Nullable SimpleMultiset<String> googNamespaces;
      private @Nullable SimpleMultiset<String> stronglyRequiredGoogNamespacesBuilderField;
      private @Nullable SimpleMultiset<String> stronglyRequiredGoogNamespaces;
      private @Nullable SimpleMultiset<String> dynamicallyRequiredGoogNamespacesBuilderField;
      private @Nullable SimpleMultiset<String> dynamicallyRequiredGoogNamespaces;
      private @Nullable SimpleMultiset<String> maybeRequiredGoogNamespacesBuilderField;
      private @Nullable SimpleMultiset<String> maybeRequiredGoogNamespaces;
      private @Nullable SimpleMultiset<String> weaklyRequiredGoogNamespacesBuilderField;
      private @Nullable SimpleMultiset<String> weaklyRequiredGoogNamespaces;
      private @Nullable SimpleMultiset<String> es6ImportSpecifiersBuilderField;
      private @Nullable SimpleMultiset<String> es6ImportSpecifiers;
      private ImmutableList.@Nullable Builder<ModuleMetadataMap.ModuleMetadata> nestedModulesBuilderField;
      private @Nullable ImmutableList<ModuleMetadataMap.ModuleMetadata> nestedModules;
      private @Nullable SimpleMultiset<String> readTogglesBuilderField;
      private @Nullable SimpleMultiset<String> readToggles;
      private @Nullable ModulePath pathField;
      private byte set0;

      Builder() {
      }

      public ModuleMetadata build() {
        if (googNamespacesBuilderField != null) {
          this.googNamespaces = googNamespacesBuilderField.immutableCopy();
        } else if (this.googNamespaces == null) {
          this.googNamespaces = SimpleMultiset.of();
        }
        if (stronglyRequiredGoogNamespacesBuilderField != null) {
          this.stronglyRequiredGoogNamespaces = stronglyRequiredGoogNamespacesBuilderField.immutableCopy();
        } else if (this.stronglyRequiredGoogNamespaces == null) {
          this.stronglyRequiredGoogNamespaces = SimpleMultiset.of();
        }
        if (dynamicallyRequiredGoogNamespacesBuilderField != null) {
          this.dynamicallyRequiredGoogNamespaces = dynamicallyRequiredGoogNamespacesBuilderField.immutableCopy();
        } else if (this.dynamicallyRequiredGoogNamespaces == null) {
          this.dynamicallyRequiredGoogNamespaces = SimpleMultiset.of();
        }
        if (maybeRequiredGoogNamespacesBuilderField != null) {
          this.maybeRequiredGoogNamespaces = maybeRequiredGoogNamespacesBuilderField.immutableCopy();
        } else if (this.maybeRequiredGoogNamespaces == null) {
          this.maybeRequiredGoogNamespaces = SimpleMultiset.of();
        }
        if (weaklyRequiredGoogNamespacesBuilderField != null) {
          this.weaklyRequiredGoogNamespaces = weaklyRequiredGoogNamespacesBuilderField.immutableCopy();
        } else if (this.weaklyRequiredGoogNamespaces == null) {
          this.weaklyRequiredGoogNamespaces = SimpleMultiset.of();
        }
        if (es6ImportSpecifiersBuilderField != null) {
          this.es6ImportSpecifiers = es6ImportSpecifiersBuilderField.immutableCopy();
        } else if (this.es6ImportSpecifiers == null) {
          this.es6ImportSpecifiers = SimpleMultiset.of();
        }
        if (nestedModulesBuilderField != null) {
          this.nestedModules = nestedModulesBuilderField.build();
        } else if (this.nestedModules == null) {
          this.nestedModules = ImmutableList.of();
        }
        if (readTogglesBuilderField != null) {
          this.readToggles = readTogglesBuilderField.immutableCopy();
        } else if (this.readToggles == null) {
          this.readToggles = SimpleMultiset.of();
        }
        if (set0 != 3
            || this.moduleTypeField == null) {
          StringBuilder missing = new StringBuilder();
          if (this.moduleTypeField == null) {
            missing.append(" moduleType");
          }
          if ((set0 & 1) == 0) {
            missing.append(" usesClosure");
          }
          if ((set0 & 2) == 0) {
            missing.append(" isTestOnly");
          }
          throw new IllegalStateException("Missing required properties:" + missing);
        }
        return new ModuleMetadata(
            this.moduleTypeField,
            this.rootNodeField,
            this.usesClosureField,
            this.isTestOnlyField,
            this.googNamespaces,
            this.stronglyRequiredGoogNamespaces,
            this.dynamicallyRequiredGoogNamespaces,
            this.maybeRequiredGoogNamespaces,
            this.weaklyRequiredGoogNamespaces,
            this.es6ImportSpecifiers,
            this.nestedModules,
            this.readToggles,
            this.pathField);
      }

      public SimpleMultiset<String> googNamespacesBuilder() {
        if (googNamespacesBuilderField == null) {
          googNamespacesBuilderField = SimpleMultiset.create();
        }
        return googNamespacesBuilderField;
      }

      @CanIgnoreReturnValue
      public Builder addGoogNamespace(String namespace) {
        googNamespacesBuilder().add(namespace);
        return this;
      }

      public SimpleMultiset<String> stronglyRequiredGoogNamespacesBuilder() {
        if (stronglyRequiredGoogNamespacesBuilderField == null) {
          stronglyRequiredGoogNamespacesBuilderField = SimpleMultiset.create();
        }
        return stronglyRequiredGoogNamespacesBuilderField;
      }

      public SimpleMultiset<String> dynamicallyRequiredGoogNamespacesBuilder() {
        if (dynamicallyRequiredGoogNamespacesBuilderField == null) {
          dynamicallyRequiredGoogNamespacesBuilderField = SimpleMultiset.create();
        }
        return dynamicallyRequiredGoogNamespacesBuilderField;
      }

      public SimpleMultiset<String> maybeRequiredGoogNamespacesBuilder() {
        if (maybeRequiredGoogNamespacesBuilderField == null) {
          maybeRequiredGoogNamespacesBuilderField = SimpleMultiset.create();
        }
        return maybeRequiredGoogNamespacesBuilderField;
      }

      public SimpleMultiset<String> weaklyRequiredGoogNamespacesBuilder() {
        if (weaklyRequiredGoogNamespacesBuilderField == null) {
          weaklyRequiredGoogNamespacesBuilderField = SimpleMultiset.create();
        }
        return weaklyRequiredGoogNamespacesBuilderField;
      }

      public SimpleMultiset<String> es6ImportSpecifiersBuilder() {
        if (es6ImportSpecifiersBuilderField == null) {
          es6ImportSpecifiersBuilderField = SimpleMultiset.create();
        }
        return es6ImportSpecifiersBuilderField;
      }

      public ImmutableList.Builder<ModuleMetadata> nestedModulesBuilder() {
        if (nestedModulesBuilderField == null) {
          nestedModulesBuilderField = ImmutableList.builder();
        }
        return nestedModulesBuilderField;
      }

      public SimpleMultiset<String> readTogglesBuilder() {
        if (readTogglesBuilderField == null) {
          readTogglesBuilderField = SimpleMultiset.create();
        }
        return readTogglesBuilderField;
      }

      public Builder path(@Nullable ModulePath value) {
        this.pathField = value;
        return this;
      }

      public Builder usesClosure(boolean value) {
        this.usesClosureField = value;
        set0 |= (byte) 1;
        return this;
      }

      public Builder isTestOnly(boolean value) {
        this.isTestOnlyField = value;
        set0 |= (byte) 2;
        return this;
      }

      public ModuleType moduleType() {
        if (this.moduleTypeField == null) {
          throw new IllegalStateException("Property \"moduleType\" has not been set");
        }
        return moduleTypeField;
      }

      public Builder moduleType(ModuleType value) {
        if (value == null) {
          throw new NullPointerException("Null moduleType");
        }
        this.moduleTypeField = value;
        return this;
      }

      public Builder rootNode(@Nullable Node root) {
        this.rootNodeField = root;
        return this;
      }
    }
  }

  /**
   * @return map from module path to module. These modules represent files and thus {@link
   *     ModuleMetadata#googNamespaces()} contains all Closure namespaces in the file. These are not
   *     the same modules from ModuleMetadataMap#getModulesByGoogNamespace(). It is not valid to
   *     call ModuleRenaming#getGlobalName(ModuleMetadata, String) on {@link
   *     ModuleType#GOOG_PROVIDE} modules from this map that have more than one Closure namespace as
   *     it is ambiguous.
   */
  public ImmutableMap<String, ModuleMetadata> getModulesByPath() {
    return modulesByPath;
  }

  /**
   * @return map from Closure namespace to module. These modules represent the Closure namespace and
   *     thus {@link ModuleMetadata#googNamespaces()} will have size 1. As a result, it is valid to
   *     call ModuleRenaming#getGlobalName(ModuleMetadata, String) on these modules. These are not
   *     the same modules from {@link ModuleMetadataMap#getModulesByPath()}.
   */
  public ImmutableMap<String, ModuleMetadata> getModulesByGoogNamespace() {
    return modulesByGoogNamespace;
  }

  /**
   * The set of all modules across both maps.
   *
   * <p>{@code goog.loadModule} calls have no associated path, and non-Closure modules have no
   * namespaces.
   */
  public ImmutableSet<ModuleMetadata> getAllModuleMetadata() {
    return moduleMetadata;
  }

  public static ModuleMetadataMap emptyForTesting() {
    return new ModuleMetadataMap(ImmutableMap.of(), ImmutableMap.of());
  }
}
