/*
 * Copyright 2017 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.bundle;


import static java.util.Objects.requireNonNull;

import com.google.common.base.Supplier;
import com.google.common.base.Suppliers;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.errorprone.annotations.CheckReturnValue;
import com.google.errorprone.annotations.Immutable;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.deps.DependencyInfo;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Function;
import org.jspecify.annotations.Nullable;

/**
 * An abstract representation of a source file.
 *
 * @param path The path of this source. This may refer to a path on disk or a path on the HTTP
 *     server.
 * @param sourceMap The text of any source map applicable to this file.
 * @param sourceUrl The source URL associated with this file.
 * @param sourceMappingUrl The URL for a source map associated with this file.
 * @param runtimes Any runtime libraries necessary for this source. Any transformation that adds a
 *     runtime library to any sources must be responsible to never add the same library as a
 *     substring to a different source (so that the "no duplicates" invariant of Set will work
 *     correctly).
 * @param loadFlags The load flags, specifying module type and language level.
 * @param estimatedSize A best estimate of the size of this source (in case the source itself is not
 *     yet loaded.
 */
@Immutable
public record Source(
    String path,
    String sourceMap,
    String sourceUrl,
    String sourceMappingUrl,
    ImmutableSet<String> runtimes,
    ImmutableMap<String, String> loadFlags,
    int estimatedSize,
    // Internal-only properties: the code suppliers are necessary for lazy bundling,
    // but we cannot use an ordinary supplier since we need guarantees about equals and
    // hash code.  Thus, we use an internal-only Supplier subtype.
    Lazy<String> codeSupplier,
    @Nullable Lazy<String> originalCodeSupplier) {
  public Source {
    requireNonNull(runtimes, "runtimes");
    requireNonNull(loadFlags, "loadFlags");
    requireNonNull(codeSupplier, "codeSupplier");
  }

  @Override
  public String toString() {
    return "Source{"
        + "path=" + path + ", "
        + "sourceMap=" + sourceMap + ", "
        + "sourceUrl=" + sourceUrl + ", "
        + "sourceMappingUrl=" + sourceMappingUrl + ", "
        + "runtimes=" + runtimes + ", "
        + "loadFlags=" + loadFlags + ", "
        + "estimatedSize=" + estimatedSize + ", "
        + "codeSupplier=" + codeSupplier + ", "
        + "originalCodeSupplier=" + originalCodeSupplier
        + "}";
  }

  /** The actual code in this source file. */
  public final String code() {
    return codeSupplier().get();
  }

  /** The untransformed code from the original source file. */
  public final String originalCode() {
    return originalCodeSupplier().get();
  }

  /** Copies the data from this source to a new builder. */
  public Builder toBuilder() {
    return new Builder(this);
  }

  /** Makes a new empty builder. */
  public static Builder builder() {
    return new Builder()
        .setPath(DEV_NULL)
        .setCode("")
        .setOriginalCodeSupplier(null)
        .setSourceMap("")
        .setSourceUrl("")
        .setSourceMappingUrl("")
        .setRuntimes(ImmutableSet.of())
        .setLoadFlags(ImmutableMap.of())
        .setEstimatedSize(0);
  }

  private static final String DEV_NULL = "/dev/null";

  /** Builder for Source instances. */
  public static final class Builder {
    private Optional<String> path = Optional.empty();
    private Optional<String> sourceMap = Optional.empty();
    private Optional<String> sourceUrl = Optional.empty();
    private Optional<String> sourceMappingUrl = Optional.empty();
    private @Nullable ImmutableSet<String> runtimesField;
    private @Nullable ImmutableMap<String, String> loadFlags;
    private int estimatedSize;
    private Source.@Nullable Lazy<String> codeSupplierField;
    private Source.@Nullable Lazy<String> originalCodeSupplierField;
    private byte set0;

    Builder() {
    }

    Builder(Source source) {
      this.path = Optional.of(source.path());
      this.sourceMap = Optional.of(source.sourceMap());
      this.sourceUrl = Optional.of(source.sourceUrl());
      this.sourceMappingUrl = Optional.of(source.sourceMappingUrl());
      this.runtimesField = source.runtimes();
      this.loadFlags = source.loadFlags();
      this.estimatedSize = source.estimatedSize();
      this.codeSupplierField = source.codeSupplier();
      this.originalCodeSupplierField = source.originalCodeSupplier();
      set0 = (byte) 1;
    }

    public Builder setPath(String path) {
      this.path = Optional.of(Io.pathString(path));
      return this;
    }
    public Builder setSourceMap(String sourceMap) {
      this.sourceMap = Optional.of(sourceMap);
      return this;
    }
    public Builder setSourceUrl(String sourceUrl) {
      this.sourceUrl = Optional.of(sourceUrl);
      return this;
    }
    public Builder setSourceMappingUrl(String sourceMappingUrl) {
      this.sourceMappingUrl = Optional.of(sourceMappingUrl);
      return this;
    }
    public Builder setRuntimes(ImmutableSet<String> runtimes) {
      if (runtimes == null) {
        throw new NullPointerException("Null runtimes");
      }
      this.runtimesField = runtimes;
      return this;
    }
    public Builder setLoadFlags(ImmutableMap<String, String> flags) {
      if (flags == null) {
        throw new NullPointerException("Null loadFlags");
      }
      this.loadFlags = flags;
      return this;
    }
    public Builder setEstimatedSize(int estimatedSize) {
      this.estimatedSize = estimatedSize;
      set0 |= (byte) 1;
      return this;
    }

    public final Builder setCodeFromSupplier(Supplier<String> code) {
      return setCodeSupplier(Lazy.memoize(code));
    }

    public final Builder setCode(String code) {
      return setCodeSupplier(Lazy.ofInstance(code));
    }

    public final Builder setOriginalCode(String code) {
      return setOriginalCodeSupplier(Lazy.ofInstance(code));
    }

    public final Builder addRuntime(String... runtimes) {
      return setRuntimes(ImmutableSet.<String>builder().addAll(runtimes()).add(runtimes).build());
    }

    public final Builder setDependencyInfo(DependencyInfo info) {
      // TODO(sdh): consider whether to set path.
      return setLoadFlags(info.getLoadFlags());
    }

    public final Source build() {
      if (originalCodeSupplier() == null) {
        setOriginalCodeSupplier(codeSupplier());
      }
      return autoBuild();
    }

    // Internal-only getters and setters.
    Builder setCodeSupplier(Lazy<String> code) {
      if (code == null) {
        throw new NullPointerException("Null codeSupplier");
      }
      this.codeSupplierField = code;
      return this;
    }
    Builder setOriginalCodeSupplier(@Nullable Lazy<String> code) {
      this.originalCodeSupplierField = code;
      return this;
    }
    ImmutableSet<String> runtimes() {
      if (this.runtimesField == null) {
        throw new IllegalStateException("Property \"runtimes\" has not been set");
      }
      return runtimesField;
    }
    Lazy<String> codeSupplier() {
      if (this.codeSupplierField == null) {
        throw new IllegalStateException("Property \"codeSupplier\" has not been set");
      }
      return codeSupplierField;
    }
    Source autoBuild() {
      if (set0 != 1
          || this.path.isEmpty()
          || this.sourceMap.isEmpty()
          || this.sourceUrl.isEmpty()
          || this.sourceMappingUrl.isEmpty()
          || this.runtimesField == null
          || this.loadFlags == null
          || this.codeSupplierField == null) {
        StringBuilder missing = new StringBuilder();
        if (this.path.isEmpty()) {
          missing.append(" path");
        }
        if (this.sourceMap.isEmpty()) {
          missing.append(" sourceMap");
        }
        if (this.sourceUrl.isEmpty()) {
          missing.append(" sourceUrl");
        }
        if (this.sourceMappingUrl.isEmpty()) {
          missing.append(" sourceMappingUrl");
        }
        if (this.runtimesField == null) {
          missing.append(" runtimes");
        }
        if (this.loadFlags == null) {
          missing.append(" loadFlags");
        }
        if ((set0 & 1) == 0) {
          missing.append(" estimatedSize");
        }
        if (this.codeSupplierField == null) {
          missing.append(" codeSupplier");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new Source(
          this.path.get(),
          this.sourceMap.get(),
          this.sourceUrl.get(),
          this.sourceMappingUrl.get(),
          this.runtimesField,
          this.loadFlags,
          this.estimatedSize,
          this.codeSupplierField,
          this.originalCodeSupplierField);
    }

    @Nullable Lazy<String> originalCodeSupplier() {
      return originalCodeSupplierField;
    }
  }

  /** An automorphic transformation on sources. */
  @FunctionalInterface
  public interface Transformer {

    /** The main transformation method. */
    Source transform(Source input);

    static Transformer of(Function<Source, Source> function) {
      return function::apply;
    }

    /** Returns an identity transformer. */
    static Transformer identity() {
      return x -> x;
    }

    /** Converts this Transformer to a Function. */
    default Function<Source, Source> asFunction() {
      return this::transform;
    }

    /** Concatenates two Transformers. */
    @CheckReturnValue
    default Transformer andThen(Transformer after) {
      Transformer before = this;
      return x -> after.transform(before.transform(x));
    }

    /** Concatenates two Transformers. */
    @CheckReturnValue
    default Transformer compose(Transformer before) {
      Transformer after = this;
      return x -> after.transform(before.transform(x));
    }
  }

  /** Essentially the same as Supplier, but wraps equals and hashCode. */
  @Immutable
  abstract static class Lazy<T> implements Supplier<T> {

    @Override
    public boolean equals(Object other) {
      return other instanceof Lazy<?> && Objects.equals(get(), ((Lazy<?>) other).get());
    }

    @Override
    public int hashCode() {
      return Objects.hashCode(get());
    }

    /** Returns a Lazy that always returns the same instance. */
    @SuppressWarnings("Immutable") // T is not known to be immutable.
    static <T> Lazy<T> ofInstance(T instance) {
      return new Lazy<T>() {
        @Override
        public T get() {
          return instance;
        }
      };
    }

    /** Returns a Lazy from a memoized supplier. */
    @SuppressWarnings("Immutable") // Supplier is not intrinsically immutable.
    static <T> Lazy<T> memoize(Supplier<T> supplier) {
      Supplier<T> memoized = Suppliers.memoize(supplier);
      return new Lazy<T>() {
        @Override
        public T get() {
          return memoized.get();
        }
      };
    }
  }
}
