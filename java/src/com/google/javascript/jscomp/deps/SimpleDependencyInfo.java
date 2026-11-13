/*
 * Copyright 2009 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.deps;

import static java.util.Objects.requireNonNull;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.errorprone.annotations.Immutable;
import java.util.Collection;
import java.util.Map;
import java.util.Optional;

/** A class to hold JS dependency information for a single .js file. */
@Immutable
public record SimpleDependencyInfo(
    String getName,
    String getPathRelativeToClosureBase,
    ImmutableList<String> getProvides,
    ImmutableList<Require> getRequires,
    ImmutableList<String> getTypeRequires,
    ImmutableMap<String, String> getLoadFlags,
    boolean getHasExternsAnnotation,
    boolean getHasNoCompileAnnotation)
    implements DependencyInfo {
  public SimpleDependencyInfo {
    requireNonNull(getProvides, "getProvides");
    requireNonNull(getRequires, "getRequires");
    requireNonNull(getTypeRequires, "getTypeRequires");
    requireNonNull(getLoadFlags, "getLoadFlags");
  }

  @Override
  public String toString() {
    return "SimpleDependencyInfo{"
        + "name=" + getName + ", "
        + "pathRelativeToClosureBase=" + getPathRelativeToClosureBase + ", "
        + "provides=" + getProvides + ", "
        + "requires=" + getRequires + ", "
        + "typeRequires=" + getTypeRequires + ", "
        + "loadFlags=" + getLoadFlags + ", "
        + "hasExternsAnnotation=" + getHasExternsAnnotation + ", "
        + "hasNoCompileAnnotation=" + getHasNoCompileAnnotation
        + "}";
  }

  public static Builder builder(String srcPathRelativeToClosure, Optional<String> pathOfDefiningFile) {
    return new Builder()
        .setName(pathOfDefiningFile)
        .setPathRelativeToClosureBase(srcPathRelativeToClosure)
        .setProvides(ImmutableList.of())
        .setRequires(ImmutableList.of())
        .setTypeRequires(ImmutableList.of())
        .setLoadFlags(ImmutableMap.of())
        .setHasExternsAnnotation(false)
        .setHasNoCompileAnnotation(false);
  }

  /**
   * Builder for constructing instances of SimpleDependencyInfo.
   * Use the {@link SimpleDependencyInfo#builder(String, String)} method to create an instance.
   */
  public static final class Builder {
    private Optional<String> name = Optional.empty();
    private Optional<String> pathRelativeToClosureBase = Optional.empty();
    private ImmutableList<String> provides;
    private ImmutableList<DependencyInfo.Require> requires;
    private ImmutableList<String> typeRequires;
    private ImmutableMap<String, String> loadFlags;
    private boolean hasExternsAnnotation;
    private boolean hasNoCompileAnnotation;
    private byte set0;

    Builder() {
    }

    public static Builder from(DependencyInfo copy) {
      return new Builder()
          .setName(Optional.of(copy.getName()))
          .setPathRelativeToClosureBase(copy.getPathRelativeToClosureBase())
          .setProvides(copy.getProvides())
          .setRequires(copy.getRequires())
          .setTypeRequires(copy.getTypeRequires())
          .setLoadFlags(copy.getLoadFlags())
          .setHasExternsAnnotation(copy.getHasExternsAnnotation())
          .setHasNoCompileAnnotation(copy.getHasNoCompileAnnotation());
    }

    Builder setName(Optional<String> name) {
      if (name.isEmpty()) {
        throw new NullPointerException("Null name");
      }
      this.name = name;
      return this;
    }
    Builder setPathRelativeToClosureBase(String srcPathRelativeToClosure) {
      this.pathRelativeToClosureBase = Optional.of(srcPathRelativeToClosure);
      return this;
    }

    public Builder setProvides(Collection<String> provides) {
      this.provides = ImmutableList.copyOf(provides);
      return this;
    }
    public final Builder setProvidesStringVarargs(String... provides) {
      return setProvides(ImmutableList.copyOf(provides));
    }

    public Builder setRequires(Collection<Require> requires) {
      this.requires = ImmutableList.copyOf(requires);
      return this;
    }

    public final Builder setRequiresRequireVarargs(Require... requires) {
      return setRequires(ImmutableList.copyOf(requires));
    }

    public Builder setTypeRequires(Collection<String> typeRequires) {
      this.typeRequires = ImmutableList.copyOf(typeRequires);
      return this;
    }

    public final Builder setTypeRequiresStringVarargs(String... typeRequires) {
      return setTypeRequires(ImmutableList.copyOf(typeRequires));
    }

    public Builder setLoadFlags(Map<String, String> loadFlags) {
      this.loadFlags = ImmutableMap.copyOf(loadFlags);
      return this;
    }

    public Builder setHasExternsAnnotation(boolean hasExternsAnnotation) {
      this.hasExternsAnnotation = hasExternsAnnotation;
      set0 |= (byte) 1;
      return this;
    }

    public Builder setHasNoCompileAnnotation(boolean hasNoCompileAnnotation) {
      this.hasNoCompileAnnotation = hasNoCompileAnnotation;
      set0 |= (byte) 2;
      return this;
    }

    private static final ImmutableMap<String, String> GOOG_MODULE_FLAGS =
        ImmutableMap.of("module", "goog");

    public Builder setGoogModule(boolean isModule) {
      return setLoadFlags(isModule ? GOOG_MODULE_FLAGS : ImmutableMap.of());
    }

    public SimpleDependencyInfo build() {
      if (set0 != 3
          || this.name.isEmpty()
          || this.pathRelativeToClosureBase.isEmpty()
          || this.provides == null
          || this.requires == null
          || this.typeRequires == null
          || this.loadFlags == null) {
        StringBuilder missing = new StringBuilder();
        if (this.name.isEmpty()) {
          missing.append(" name");
        }
        if (this.pathRelativeToClosureBase.isEmpty()) {
          missing.append(" pathRelativeToClosureBase");
        }
        if (this.provides == null) {
          missing.append(" provides");
        }
        if (this.requires == null) {
          missing.append(" requires");
        }
        if (this.typeRequires == null) {
          missing.append(" typeRequires");
        }
        if (this.loadFlags == null) {
          missing.append(" loadFlags");
        }
        if ((set0 & 1) == 0) {
          missing.append(" hasExternsAnnotation");
        }
        if ((set0 & 2) == 0) {
          missing.append(" hasNoCompileAnnotation");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new SimpleDependencyInfo(
          this.name.get(),
          this.pathRelativeToClosureBase.get(),
          this.provides,
          this.requires,
          this.typeRequires,
          this.loadFlags,
          this.hasExternsAnnotation,
          this.hasNoCompileAnnotation);
    }
  }

  public static final SimpleDependencyInfo EMPTY = builder("", Optional.of("")).build();
}
