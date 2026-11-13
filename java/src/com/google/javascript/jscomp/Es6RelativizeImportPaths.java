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

package com.google.javascript.jscomp;

import com.google.javascript.jscomp.NodeTraversal.AbstractPreOrderCallback;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.deps.ModuleLoader;
import com.google.javascript.rhino.Node;

/**
 * Rewrites ES6 import paths to be relative after resolving according to the compiler's module
 * resolver.
 *
 * <p>Useful for servers that wish to preserve ES6 modules, meaning their paths need to be valid in
 * the browser.
 */
public class Es6RelativizeImportPaths implements CompilerPass {

  private final AbstractCompiler compiler;

  public Es6RelativizeImportPaths(AbstractCompiler compiler) {
    this.compiler = compiler;
  }

  @Override
  public void process(Node externs, Node root) {
    for (Node script = root.getFirstChild(); script != null; script = script.getNext()) {
      if (Es6RewriteModules.isEs6ModuleRoot(script)) {
        NodeTraversal.traverse(compiler, script, new Rewriter());
      }
    }
  }

  private static class Rewriter extends AbstractPreOrderCallback {
    @Override
    public boolean shouldTraverse(NodeTraversal nodeTraversal, Node n, Node parent) {
      return switch (n.getToken()) {
        case ROOT, MODULE_BODY, SCRIPT -> true;
        case IMPORT -> {
          visitImport(nodeTraversal, n);
          yield false;
        }
        case EXPORT -> {
          visitExport(nodeTraversal, n);
          yield false;
        }
        default -> false;
      };
    }

    private void visitImport(NodeTraversal t, Node importDecl) {
      Node specifierNode = importDecl.getLastChild();
      String specifier = specifierNode.getString();

      // Leave relative and truly absolute paths (those with a scheme) alone. Only transform
      // absolute paths without a scheme (starting with "/") and ambiguous paths.
      if (ModuleLoader.isRelativeIdentifier(specifier)) {
        return;
      } else {
        if (Io.uriParse(specifier).isErr()) {
          return;
        }
        if (Io.uriScheme(specifier) != null) {
          return;
        }
      }

      String scriptPath = t.getInput().getPath().toString();

      if (Io.uriParse(scriptPath).isErr()) {
        return;
      }
      // If the script path has a scheme / host / port then just use the path part.
      scriptPath = Io.uriPath(scriptPath);

      String newSpecifier = t.getInput().getPath().resolveModuleAsPath(specifier).toString();

      // If a module root is stripped then this won't start with "/" when it probably should.
      if (!newSpecifier.startsWith("/")) {
        newSpecifier = "/" + newSpecifier;
      }

      newSpecifier = Io.pathRelativize(Io.pathParent(scriptPath), newSpecifier);

      // Relativizing two paths with the same directory yields an ambiguous path rather than one
      // starting with "./".
      if (ModuleLoader.isAmbiguousIdentifier(newSpecifier)) {
        newSpecifier = "./" + newSpecifier;
      }

      if (!newSpecifier.equals(specifier)) {
        specifierNode.setString(newSpecifier);
        t.reportCodeChange(specifierNode);
      }
    }

    private void visitExport(NodeTraversal t, Node export) {
      if (export.hasTwoChildren()) {
        // export from
        visitImport(t, export);
      }
    }
  }
}
