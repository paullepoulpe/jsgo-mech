/*
 * Copyright 2020 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.serialization;

import static java.util.Objects.requireNonNull;

import com.google.common.collect.ImmutableList;

/** Configuration options for serialization time. */
public record SerializationOptions(
    boolean includeDebugInfo, boolean runValidation, ImmutableList<String> runtimeLibraries) {
  public SerializationOptions {
    requireNonNull(runtimeLibraries, "runtimeLibraries");
  }

  public static Builder builder() {
    return new Builder()
        .setRunValidation(false)
        .setIncludeDebugInfo(false)
        .setRuntimeLibraries(ImmutableList.of());
  }

  /** Builder for {@link SerializationOptions}. */
  public static final class Builder {
    private boolean includeDebugInfo;
    private boolean runValidation;
    private ImmutableList<String> runtimeLibraries;
    private byte set0;

    Builder() {
    }

    public Builder setIncludeDebugInfo(boolean includeDebugInfo) {
      this.includeDebugInfo = includeDebugInfo;
      set0 |= (byte) 1;
      return this;
    }

    public Builder setRunValidation(boolean runValidation) {
      this.runValidation = runValidation;
      set0 |= (byte) 2;
      return this;
    }

    public Builder setRuntimeLibraries(ImmutableList<String> runtimeLibraries) {
      if (runtimeLibraries == null) {
        throw new NullPointerException("Null runtimeLibraries");
      }
      this.runtimeLibraries = runtimeLibraries;
      return this;
    }

    public SerializationOptions build() {
      if (set0 != 3
          || this.runtimeLibraries == null) {
        StringBuilder missing = new StringBuilder();
        if ((set0 & 1) == 0) {
          missing.append(" includeDebugInfo");
        }
        if ((set0 & 2) == 0) {
          missing.append(" runValidation");
        }
        if (this.runtimeLibraries == null) {
          missing.append(" runtimeLibraries");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new SerializationOptions(
          this.includeDebugInfo,
          this.runValidation,
          this.runtimeLibraries);
    }
  }

}
