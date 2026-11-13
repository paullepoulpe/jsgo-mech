/*
 * Copyright 2026 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.base;

import org.jspecify.annotations.Nullable;

/**
 * The outcome of an operation that has an expected way to fail: either a value or an error.
 *
 * <p>Expected failures (bad input, unreadable file, ...) are returned as an error {@code Outcome}
 * and checked by the caller with {@link #isErr()}; exceptions are left for bugs.
 */
public final class Outcome<T> {
  private final @Nullable T value;
  private final @Nullable Throwable errorField;

  private Outcome(@Nullable T value, @Nullable Throwable error) {
    this.value = value;
    this.errorField = error;
  }

  /** A successful outcome; the value may be null when the operation's value may be. */
  public static <T> Outcome<T> ok(@Nullable T value) {
    return new Outcome<>(value, null);
  }

  /** A successful outcome of an operation that has no value (a {@code void} method). */
  public static Outcome<Void> ok() {
    return new Outcome<>(null, null);
  }

  /** A failed outcome carrying the exception that describes the failure. */
  public static <T> Outcome<T> err(Throwable error) {
    if (error == null) {
      throw new NullPointerException();
    }
    return new Outcome<>(null, error);
  }

  public boolean isOk() {
    return errorField == null;
  }

  public boolean isErr() {
    return errorField != null;
  }

  /** The value of a successful outcome; calling this on a failed outcome is a bug. */
  public T get() {
    if (errorField != null) {
      throw new IllegalStateException("Outcome.get() called on an error outcome", errorField);
    }
    return value;
  }

  /** The error of a failed outcome, or null for a successful one. */
  public @Nullable Throwable error() {
    return errorField;
  }

  /** The message of the error of a failed outcome, like {@link Throwable#getMessage()}. */
  public @Nullable String errorMessage() {
    return errorField == null ? null : errorField.getMessage();
  }
}
