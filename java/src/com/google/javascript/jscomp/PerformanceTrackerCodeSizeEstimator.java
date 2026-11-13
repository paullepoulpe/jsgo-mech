/*
 * Copyright 2016 The Closure Compiler Authors.
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


import com.google.javascript.jscomp.base.Io;
import com.google.javascript.rhino.Node;
import java.util.ArrayList;
import java.util.List;
import org.jspecify.annotations.Nullable;

/** A class to get a raw and gzip'ed size estimate; it doesn't generate code. */
final class PerformanceTrackerCodeSizeEstimator extends CodeConsumer {
  private int size = 0;
  private char lastChar = '\0';
  // The UTF-8 bytes of each appended string, when tracking the gzipped size.
  private final @Nullable List<byte[]> output;
  private final boolean trackGzSize;

  static PerformanceTrackerCodeSizeEstimator estimate(Node jsRoot, boolean trackGzSize) {
    PerformanceTrackerCodeSizeEstimator estimator = new PerformanceTrackerCodeSizeEstimator(
        trackGzSize);
    CodeGenerator.forCostEstimation(estimator).addNode(jsRoot);
    return estimator;
  }

  private PerformanceTrackerCodeSizeEstimator(boolean trackGzSize) {
    this.trackGzSize = trackGzSize;
    if (trackGzSize) {
      output = new ArrayList<>();
    } else {
      output = null;
    }
  }

  @Override
  void append(String str) {
    int len = str.length();
    if (len > 0) {
      size += len;
      lastChar = str.charAt(len - 1);
      if (trackGzSize) {
        output.add(Io.utf8Bytes(str));
      }
    }
  }

  @Override
  char getLastChar() {
    return lastChar;
  }

  int getCodeSize() {
    return size;
  }

  int getZippedCodeSize() {
    if (trackGzSize) {
      int length = 0;
      for (byte[] chunk : output) {
        length += chunk.length;
      }
      byte[] bytes = new byte[length];
      int offset = 0;
      for (byte[] chunk : output) {
        System.arraycopy(chunk, 0, bytes, offset, chunk.length);
        offset += chunk.length;
      }
      return Io.gzip(bytes).length;
    } else {
      return 0;
    }
  }
}
