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

package com.google.javascript.jscomp;

import static com.google.common.base.Preconditions.checkState;
import static java.util.Objects.requireNonNull;

import com.google.errorprone.annotations.ForOverride;
import java.util.function.Function;

/**
 * A factory for creating JSCompiler passes based on the Options injected.
 *
 * <p>Contains all meta-data about compiler passes (like whether it can be run multiple times, a
 * human-readable name for logging, etc.).
 *
 * @param getName The name of the pass as it will appear in logs.
 * @param getPreconditionCheck Runs before this pass runs to validate preconditions. Throws an
 *     exception if any are violated (unlike {@link #getCondition()}.)
 * @param isRunInFixedPointLoop Whether this factory must or must not appear in a {@link
 *     PhaseOptimizer} loop.
 * @param getInternalFactory A simple factory function for creating actual pass instances.
 *     <p>Users should call {@link #create(AbstractCompiler)} rather than use this object directly.
 */
public record PassFactory(
    String getName,
    Function<CompilerOptions, Boolean> getCondition,
    Function<CompilerOptions, PreconditionResult> getPreconditionCheck,
    boolean isRunInFixedPointLoop,
    Function<AbstractCompiler, ? extends CompilerPass> getInternalFactory) {
  public PassFactory {
    requireNonNull(getName, "getName");
    requireNonNull(getCondition, "getCondition");
    requireNonNull(getPreconditionCheck, "getPreconditionCheck");
    requireNonNull(getInternalFactory, "getInternalFactory");
  }

  @Override
  public String toString() {
    return "PassFactory{"
        + "name=" + getName + ", "
        + "condition=" + getCondition + ", "
        + "preconditionCheck=" + getPreconditionCheck + ", "
        + "runInFixedPointLoop=" + isRunInFixedPointLoop + ", "
        + "internalFactory=" + getInternalFactory
        + "}";
  }

  record PreconditionResult(boolean success, String message) {
    static final PreconditionResult SUCCESS = new PreconditionResult(true, null);
  }

  public Builder toBuilder() {
    return new Builder(this);
  }

  /** A builder for a {@link PassFactory}. */
  public static final class Builder {
    private String name;
    private Function<CompilerOptions, Boolean> condition;
    private Function<CompilerOptions, PassFactory.PreconditionResult> preconditionCheck;
    private boolean runInFixedPointLoop;
    private Function<AbstractCompiler, ? extends CompilerPass> internalFactory;
    private byte set0;

    Builder() {
    }

    Builder(PassFactory source) {
      this.name = source.getName();
      this.condition = source.getCondition();
      this.preconditionCheck = source.getPreconditionCheck();
      this.runInFixedPointLoop = source.isRunInFixedPointLoop();
      this.internalFactory = source.getInternalFactory();
      set0 = (byte) 1;
    }

    public Builder setName(String x) {
      if (x == null) {
        throw new NullPointerException("Null name");
      }
      this.name = x;
      return this;
    }

    public Builder setRunInFixedPointLoop(boolean b) {
      this.runInFixedPointLoop = b;
      set0 |= (byte) 1;
      return this;
    }

    public Builder setCondition(Function<CompilerOptions, Boolean> cond) {
      if (cond == null) {
        throw new NullPointerException("Null condition");
      }
      this.condition = cond;
      return this;
    }

    public Builder setPreconditionCheck(
        Function<CompilerOptions, PreconditionResult> assertion) {
      if (assertion == null) {
        throw new NullPointerException("Null preconditionCheck");
      }
      this.preconditionCheck = assertion;
      return this;
    }

    public Builder setInternalFactory(
        Function<AbstractCompiler, ? extends CompilerPass> x) {
      if (x == null) {
        throw new NullPointerException("Null internalFactory");
      }
      this.internalFactory = x;
      return this;
    }

    @ForOverride
    PassFactory autoBuild() {
      if (set0 != 1
          || this.name == null
          || this.condition == null
          || this.preconditionCheck == null
          || this.internalFactory == null) {
        StringBuilder missing = new StringBuilder();
        if (this.name == null) {
          missing.append(" name");
        }
        if (this.condition == null) {
          missing.append(" condition");
        }
        if (this.preconditionCheck == null) {
          missing.append(" preconditionCheck");
        }
        if ((set0 & 1) == 0) {
          missing.append(" runInFixedPointLoop");
        }
        if (this.internalFactory == null) {
          missing.append(" internalFactory");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new PassFactory(
          this.name,
          this.condition,
          this.preconditionCheck,
          this.runInFixedPointLoop,
          this.internalFactory);
    }

    public final PassFactory build() {
      PassFactory result = autoBuild();
      checkState(!result.getName().isEmpty());
      return result;
    }
  }

  public static Builder builder() {
    return new Builder()
        .setRunInFixedPointLoop(false)
        .setCondition((o) -> true)
        .setPreconditionCheck((o) -> PreconditionResult.SUCCESS);
  }

  void validatePreconditions(CompilerOptions options) {
    PreconditionResult canRun = getPreconditionCheck().apply(options);
    if (!canRun.success) {
      throw new IllegalArgumentException(
          String.format("Precondition for pass %s failed: %s", this.getName(), canRun.message()));
    }
  }

  /** Create a no-op pass that can only run once. Used to break up loops. */
  public static PassFactory createEmptyPass(String name) {
    return builder().setName(name).setInternalFactory((c) -> (externs, root) -> {}).build();
  }

  /** Creates a new compiler pass to be run. */
  public final CompilerPass create(AbstractCompiler compiler) {
    return getInternalFactory().apply(compiler);
  }
}
