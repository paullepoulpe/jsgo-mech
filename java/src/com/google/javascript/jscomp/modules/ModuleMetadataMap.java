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
import com.google.common.collect.ImmutableMultiset;
import com.google.common.collect.ImmutableSet;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
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
      ImmutableMultiset<String> googNamespaces,
      ImmutableMultiset<String> stronglyRequiredGoogNamespaces,
      ImmutableMultiset<String> dynamicallyRequiredGoogNamespaces,
      ImmutableMultiset<String> maybeRequiredGoogNamespaces,
      ImmutableMultiset<String> weaklyRequiredGoogNamespaces,
      ImmutableMultiset<String> es6ImportSpecifiers,
      ImmutableList<ModuleMetadata> nestedModules,
      ImmutableMultiset<String> readToggles,
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
      private ModuleMetadataMap.@Nullable ModuleType moduleType;
      private @Nullable Node rootNode;
      private boolean usesClosure;
      private boolean isTestOnly;
      private ImmutableMultiset.@Nullable Builder<String> googNamespacesBuilder;
      private @Nullable ImmutableMultiset<String> googNamespaces;
      private ImmutableMultiset.@Nullable Builder<String> stronglyRequiredGoogNamespacesBuilder;
      private @Nullable ImmutableMultiset<String> stronglyRequiredGoogNamespaces;
      private ImmutableMultiset.@Nullable Builder<String> dynamicallyRequiredGoogNamespacesBuilder;
      private @Nullable ImmutableMultiset<String> dynamicallyRequiredGoogNamespaces;
      private ImmutableMultiset.@Nullable Builder<String> maybeRequiredGoogNamespacesBuilder;
      private @Nullable ImmutableMultiset<String> maybeRequiredGoogNamespaces;
      private ImmutableMultiset.@Nullable Builder<String> weaklyRequiredGoogNamespacesBuilder;
      private @Nullable ImmutableMultiset<String> weaklyRequiredGoogNamespaces;
      private ImmutableMultiset.@Nullable Builder<String> es6ImportSpecifiersBuilder;
      private @Nullable ImmutableMultiset<String> es6ImportSpecifiers;
      private ImmutableList.@Nullable Builder<ModuleMetadataMap.ModuleMetadata> nestedModulesBuilder;
      private @Nullable ImmutableList<ModuleMetadataMap.ModuleMetadata> nestedModules;
      private ImmutableMultiset.@Nullable Builder<String> readTogglesBuilder;
      private @Nullable ImmutableMultiset<String> readToggles;
      private @Nullable ModulePath path;
      private byte set0;

      Builder() {
      }

      public ModuleMetadata build() {
        if (googNamespacesBuilder != null) {
          this.googNamespaces = googNamespacesBuilder.build();
        } else if (this.googNamespaces == null) {
          this.googNamespaces = ImmutableMultiset.of();
        }
        if (stronglyRequiredGoogNamespacesBuilder != null) {
          this.stronglyRequiredGoogNamespaces = stronglyRequiredGoogNamespacesBuilder.build();
        } else if (this.stronglyRequiredGoogNamespaces == null) {
          this.stronglyRequiredGoogNamespaces = ImmutableMultiset.of();
        }
        if (dynamicallyRequiredGoogNamespacesBuilder != null) {
          this.dynamicallyRequiredGoogNamespaces = dynamicallyRequiredGoogNamespacesBuilder.build();
        } else if (this.dynamicallyRequiredGoogNamespaces == null) {
          this.dynamicallyRequiredGoogNamespaces = ImmutableMultiset.of();
        }
        if (maybeRequiredGoogNamespacesBuilder != null) {
          this.maybeRequiredGoogNamespaces = maybeRequiredGoogNamespacesBuilder.build();
        } else if (this.maybeRequiredGoogNamespaces == null) {
          this.maybeRequiredGoogNamespaces = ImmutableMultiset.of();
        }
        if (weaklyRequiredGoogNamespacesBuilder != null) {
          this.weaklyRequiredGoogNamespaces = weaklyRequiredGoogNamespacesBuilder.build();
        } else if (this.weaklyRequiredGoogNamespaces == null) {
          this.weaklyRequiredGoogNamespaces = ImmutableMultiset.of();
        }
        if (es6ImportSpecifiersBuilder != null) {
          this.es6ImportSpecifiers = es6ImportSpecifiersBuilder.build();
        } else if (this.es6ImportSpecifiers == null) {
          this.es6ImportSpecifiers = ImmutableMultiset.of();
        }
        if (nestedModulesBuilder != null) {
          this.nestedModules = nestedModulesBuilder.build();
        } else if (this.nestedModules == null) {
          this.nestedModules = ImmutableList.of();
        }
        if (readTogglesBuilder != null) {
          this.readToggles = readTogglesBuilder.build();
        } else if (this.readToggles == null) {
          this.readToggles = ImmutableMultiset.of();
        }
        if (set0 != 3
            || this.moduleType == null) {
          StringBuilder missing = new StringBuilder();
          if (this.moduleType == null) {
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
            this.moduleType,
            this.rootNode,
            this.usesClosure,
            this.isTestOnly,
            this.googNamespaces,
            this.stronglyRequiredGoogNamespaces,
            this.dynamicallyRequiredGoogNamespaces,
            this.maybeRequiredGoogNamespaces,
            this.weaklyRequiredGoogNamespaces,
            this.es6ImportSpecifiers,
            this.nestedModules,
            this.readToggles,
            this.path);
      }

      public ImmutableMultiset.Builder<String> googNamespacesBuilder() {
        if (googNamespacesBuilder == null) {
          googNamespacesBuilder = ImmutableMultiset.builder();
        }
        return googNamespacesBuilder;
      }

      @CanIgnoreReturnValue
      public Builder addGoogNamespace(String namespace) {
        googNamespacesBuilder().add(namespace);
        return this;
      }

      public ImmutableMultiset.Builder<String> stronglyRequiredGoogNamespacesBuilder() {
        if (stronglyRequiredGoogNamespacesBuilder == null) {
          stronglyRequiredGoogNamespacesBuilder = ImmutableMultiset.builder();
        }
        return stronglyRequiredGoogNamespacesBuilder;
      }

      public ImmutableMultiset.Builder<String> dynamicallyRequiredGoogNamespacesBuilder() {
        if (dynamicallyRequiredGoogNamespacesBuilder == null) {
          dynamicallyRequiredGoogNamespacesBuilder = ImmutableMultiset.builder();
        }
        return dynamicallyRequiredGoogNamespacesBuilder;
      }

      public ImmutableMultiset.Builder<String> maybeRequiredGoogNamespacesBuilder() {
        if (maybeRequiredGoogNamespacesBuilder == null) {
          maybeRequiredGoogNamespacesBuilder = ImmutableMultiset.builder();
        }
        return maybeRequiredGoogNamespacesBuilder;
      }

      public ImmutableMultiset.Builder<String> weaklyRequiredGoogNamespacesBuilder() {
        if (weaklyRequiredGoogNamespacesBuilder == null) {
          weaklyRequiredGoogNamespacesBuilder = ImmutableMultiset.builder();
        }
        return weaklyRequiredGoogNamespacesBuilder;
      }

      public ImmutableMultiset.Builder<String> es6ImportSpecifiersBuilder() {
        if (es6ImportSpecifiersBuilder == null) {
          es6ImportSpecifiersBuilder = ImmutableMultiset.builder();
        }
        return es6ImportSpecifiersBuilder;
      }

      public ImmutableList.Builder<ModuleMetadata> nestedModulesBuilder() {
        if (nestedModulesBuilder == null) {
          nestedModulesBuilder = ImmutableList.builder();
        }
        return nestedModulesBuilder;
      }

      public ImmutableMultiset.Builder<String> readTogglesBuilder() {
        if (readTogglesBuilder == null) {
          readTogglesBuilder = ImmutableMultiset.builder();
        }
        return readTogglesBuilder;
      }

      public Builder path(@Nullable ModulePath value) {
        this.path = value;
        return this;
      }

      public Builder usesClosure(boolean value) {
        this.usesClosure = value;
        set0 |= (byte) 1;
        return this;
      }

      public Builder isTestOnly(boolean value) {
        this.isTestOnly = value;
        set0 |= (byte) 2;
        return this;
      }

      public ModuleType moduleType() {
        if (this.moduleType == null) {
          throw new IllegalStateException("Property \"moduleType\" has not been set");
        }
        return moduleType;
      }

      public Builder moduleType(ModuleType value) {
        if (value == null) {
          throw new NullPointerException("Null moduleType");
        }
        this.moduleType = value;
        return this;
      }

      public Builder rootNode(@Nullable Node root) {
        this.rootNode = root;
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
