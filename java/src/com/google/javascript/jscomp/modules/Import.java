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

import com.google.javascript.jscomp.deps.ModuleLoader.ModulePath;
import com.google.javascript.rhino.Node;
import org.jspecify.annotations.Nullable;

/**
 * An <code>import</code>ed name in a module.
 *
 * <p>See <a href="https://www.ecma-international.org/ecma-262/9.0/index.html#table-39">the
 * ImportEntry in the ECMAScript spec.</a>
 *
 * @param moduleRequest Returns the module identifier of this import.
 * @param importName Returns the name that was imported from the requested module.
 *     <p>For {@code import *} this will return "*".
 * @param localName Returns the local name the imported value is bound to.
 * @param modulePath Returns the path of the containing module, if from an ES module.
 * @param importNode Returns the import node for source information.
 * @param nameNode Returns the name node for source information.
 */
public record Import(
    String moduleRequest,
    String importName,
    String localName,
    @Nullable ModulePath modulePath,
    Node importNode,
    Node nameNode) {
  public Import {
    requireNonNull(moduleRequest, "moduleRequest");
    requireNonNull(importName, "importName");
    requireNonNull(localName, "localName");
    requireNonNull(importNode, "importNode");
    requireNonNull(nameNode, "nameNode");
  }

  static final class Builder {
    private String moduleRequestField;
    private String importNameField;
    private String localNameField;
    private ModulePath modulePathField;
    private Node importNodeField;
    private Node nameNodeField;

    Builder() {
    }

    Builder moduleRequest(String value) {
      if (value == null) {
        throw new NullPointerException("Null moduleRequest");
      }
      this.moduleRequestField = value;
      return this;
    }

    Builder importName(String value) {
      if (value == null) {
        throw new NullPointerException("Null importName");
      }
      this.importNameField = value;
      return this;
    }

    Builder localName(String value) {
      if (value == null) {
        throw new NullPointerException("Null localName");
      }
      this.localNameField = value;
      return this;
    }

    Builder modulePath(ModulePath value) {
      this.modulePathField = value;
      return this;
    }

    Builder importNode(Node value) {
      if (value == null) {
        throw new NullPointerException("Null importNode");
      }
      this.importNodeField = value;
      return this;
    }

    Builder nameNode(Node value) {
      if (value == null) {
        throw new NullPointerException("Null nameNode");
      }
      this.nameNodeField = value;
      return this;
    }

    Import build() {
      if (this.moduleRequestField == null
          || this.importNameField == null
          || this.localNameField == null
          || this.importNodeField == null
          || this.nameNodeField == null) {
        StringBuilder missing = new StringBuilder();
        if (this.moduleRequestField == null) {
          missing.append(" moduleRequest");
        }
        if (this.importNameField == null) {
          missing.append(" importName");
        }
        if (this.localNameField == null) {
          missing.append(" localName");
        }
        if (this.importNodeField == null) {
          missing.append(" importNode");
        }
        if (this.nameNodeField == null) {
          missing.append(" nameNode");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new Import(
          this.moduleRequestField,
          this.importNameField,
          this.localNameField,
          this.modulePathField,
          this.importNodeField,
          this.nameNodeField);
    }
  }

  static Builder builder() {
    return new Builder();
  }
}
