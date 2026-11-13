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

import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;
import static java.util.Objects.requireNonNull;

import com.google.javascript.jscomp.deps.ModuleLoader.ModulePath;
import com.google.javascript.jscomp.modules.ModuleMetadataMap.ModuleMetadata;
import com.google.javascript.rhino.Node;
import java.util.Optional;
import org.jspecify.annotations.Nullable;

/**
 * An <code>export</code>ed name in a module.
 *
 * <p>See <a href="https://www.ecma-international.org/ecma-262/9.0/index.html#table-41">the
 * ExportEntry in the ECMAScript spec.</a>
 *
 * @param exportName Returns the name of this export or null if this is an {@code export * from}.
 * @param moduleRequest Returns the module identifier of an export from or null if this is not an
 *     {@code export {} from} or {@code export * from}.
 * @param importName Returns the name imported from another module. * if import all or null if not
 *     an {@code export {} from}.
 * @param localName Returns the local name of this export or null if none. *default* if default.
 * @param modulePath Returns the path of the containing module
 * @param exportNode Node that this export originates from. Used for its source location.
 *     <p>Null only if from non-ES module or from a missing ES module.
 * @param nameNode Node that this export originates from. Used for its source location.
 *     <p>Null only if from non-ES6 module or an export syntax that has no associated name, e.g.
 *     {@code export * from}.
 * @param moduleMetadata The module that contains this export.
 * @param mutated Whether or not this export is potentially mutated after module execution (i.e. in
 *     a function scope).
 */
// TODO(johnplaisted): Add validation tests. Current ModulePath makes this difficult as it is non
public record Export(
    Optional<String> exportName,
    Optional<String> moduleRequest,
    Optional<String> importName,
    Optional<String> localName,
    @Nullable ModulePath modulePath,
    @Nullable Node exportNode,
    @Nullable Node nameNode,
    ModuleMetadata moduleMetadata,
    Optional<String> closureNamespace,
    boolean mutated) {
  public Export {
    requireNonNull(moduleMetadata, "moduleMetadata");
  }

  /**
   * The {@link Export#localName()} of anonymous ES module default exports, e.g. {@code export
   * default 0}.
   */
  public static final String DEFAULT_EXPORT_NAME = "*default*";

  /**
   * The {@link Export#exportName()} of anonymous ES module default exports, e.g. {@code export
   * default 0}.
   */
  static final String DEFAULT = "default";

  /**
   * The {@link Export#exportName()} of goog.module default exports, e.g. {@code exports = class
   * {};}, and the 'namespace' of an ES module consisting of all exported names.
   */
  public static final String NAMESPACE = "*exports*";

  static final class Builder {
    private Optional<String> exportNameField = Optional.empty();
    private Optional<String> moduleRequestField = Optional.empty();
    private Optional<String> importNameField = Optional.empty();
    private Optional<String> localNameField = Optional.empty();
    private ModulePath modulePathField;
    private Node exportNodeField;
    private Node nameNodeField;
    private ModuleMetadataMap.@Nullable ModuleMetadata moduleMetadataField;
    private Optional<String> closureNamespaceField = Optional.empty();
    private boolean mutatedField;
    private byte set0;

    Builder() {
    }

    Builder(Export source) {
      this.exportNameField = source.exportName();
      this.moduleRequestField = source.moduleRequest();
      this.importNameField = source.importName();
      this.localNameField = source.localName();
      this.modulePathField = source.modulePath();
      this.exportNodeField = source.exportNode();
      this.nameNodeField = source.nameNode();
      this.moduleMetadataField = source.moduleMetadata();
      this.closureNamespaceField = source.closureNamespace();
      this.mutatedField = source.mutated();
      set0 = (byte) 1;
    }

    Builder exportName(Optional<String> value) {
      this.exportNameField = value;
      return this;
    }

    Builder moduleRequest(Optional<String> value) {
      this.moduleRequestField = value;
      return this;
    }

    Builder importName(Optional<String> value) {
      this.importNameField = value;
      return this;
    }

    Builder localName(Optional<String> value) {
      this.localNameField = value;
      return this;
    }

    Builder modulePath(@Nullable ModulePath value) {
      this.modulePathField = value;
      return this;
    }

    Builder exportNode(@Nullable Node value) {
      this.exportNodeField = value;
      return this;
    }

    Builder nameNode(@Nullable Node value) {
      this.nameNodeField = value;
      return this;
    }

    Builder moduleMetadata(ModuleMetadata value) {
      if (value == null) {
        throw new NullPointerException("Null moduleMetadata");
      }
      this.moduleMetadataField = value;
      return this;
    }

    Builder closureNamespace(Optional<String> value) {
      this.closureNamespaceField = value;
      return this;
    }

    Builder mutated(boolean value) {
      this.mutatedField = value;
      set0 |= (byte) 1;
      return this;
    }

    Export autoBuild() {
      if (set0 != 1
          || this.moduleMetadataField == null) {
        StringBuilder missing = new StringBuilder();
        if (this.moduleMetadataField == null) {
          missing.append(" moduleMetadata");
        }
        if ((set0 & 1) == 0) {
          missing.append(" mutated");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new Export(
          this.exportNameField,
          this.moduleRequestField,
          this.importNameField,
          this.localNameField,
          this.modulePathField,
          this.exportNodeField,
          this.nameNodeField,
          this.moduleMetadataField,
          this.closureNamespaceField,
          this.mutatedField);
    }

    final Export build() {
      Export e = autoBuild();
      if (e.moduleMetadata().isEs6Module()) {
        validateEsModule(e);
      } else if (e.moduleMetadata().isGoogModule()) {
        validateGoogModule(e);
      } else {
        validateOtherModule(e);
      }
      return e;
    }

    /** Export from an ES module. */
    private void validateEsModule(Export e) {
      checkState(e.closureNamespace().isEmpty());

      checkState(
          !"*".equals(e.importName().orElse(""))
              || (e.moduleRequest().isPresent() && e.exportName().isEmpty() && e.localName().isEmpty()),
          "Star exports should not have exported / local names.");

      checkState(
          e.localName().isEmpty() || e.moduleRequest().isEmpty(),
          "Local exports should not have module requests.");
      checkState(
          e.moduleRequest().isEmpty() || e.localName().isEmpty(),
          "Reexports should not have local names.");

      checkState(
          e.moduleRequest().isEmpty() || e.importName().isPresent(),
          "Reexports should have import names.");
      checkState(
          e.importName().isEmpty() || e.moduleRequest().isPresent(),
          "Exports with an import name should be a reexport.");
    }

    /** Some export from a goog module. */
    private static void validateGoogModule(Export e) {
      checkState(e.closureNamespace().isPresent(), "Exports should be associated with a namespace");
      checkState(e.exportName().isPresent(), "Exports should be named");
      checkState(e.exportNode() != null, "Exports should have a node");
      checkState(e.localName().isEmpty(), "goog.module Exports don't set a localName");
      checkState(e.moduleRequest().isEmpty(), "goog modules cannot export from other modules");
    }

    /** Some faux export from a non-ES module. */
    private static void validateOtherModule(Export e) {
      checkNotNull(e.exportName().orElseThrow());

      // Fields ignored for these fake exports. Should not set these.
      checkState(e.exportNode() == null);
      checkState(e.localName().isEmpty());
      checkState(e.moduleRequest().isEmpty());
      checkState(e.importName().isEmpty());
      checkState(e.nameNode() == null);
    }
  }

  static Builder builder() {
    return new Builder().mutated(false);
  }

  Builder toBuilder() {
    return new Builder(this);
  }

  /** Returns a copy of this export that has the {@link #mutated()} bit set. */
  final Export mutatedCopy() {
    return toBuilder().mutated(true).autoBuild();
  }

}
