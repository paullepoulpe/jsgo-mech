/*
 * Copyright 2019 The Closure Compiler Authors.
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

import com.google.javascript.jscomp.AbstractCompiler;
import com.google.javascript.jscomp.CompilerPass;
import com.google.javascript.jscomp.RemoveCastNodes;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.rhino.Node;
import java.util.function.Consumer;

/**
 * A compiler pass intended to serialize the types in the AST.
 *
 * <p>Serialization means that the AST is converted to a proto representation and output to a file
 * or stream. Deserialization means the reverse process.
 */
public final class SerializeTypedAstPass implements CompilerPass {

  private final AbstractCompiler compiler;
  private final Consumer<TypedAst> consumer;
  private final SerializationOptions serializationOptions;

  SerializeTypedAstPass(
      AbstractCompiler compiler,
      Consumer<TypedAst> astConsumer,
      SerializationOptions serializationOptions) {
    this.compiler = compiler;
    this.consumer = astConsumer;
    this.serializationOptions = serializationOptions;
  }

  /**
   * Serializes a TypedAst to bytes (a TypedAst.List proto) and hands them to the consumer.
   *
   * <p>Unlike {@link #createFromPath(AbstractCompiler, String, SerializationOptions)}, this method
   * does not gzip the TypedAST.
   */
  public static SerializeTypedAstPass createFromConsumer(
      AbstractCompiler c, Consumer<byte[]> out, SerializationOptions serializationOptions) {
    Consumer<TypedAst> toBytes =
        ast -> out.accept(TypedAst.List.newBuilder().addTypedAsts(ast).build().toByteArray());
    return new SerializeTypedAstPass(c, toBytes, serializationOptions);
  }

  /** Serializes a gzipped TypedAst to the specified outputPath */
  public static SerializeTypedAstPass createFromPath(
      AbstractCompiler compiler, String outputPath, SerializationOptions serializationOptions) {
    Consumer<TypedAst> toPath =
        ast -> {
          Outcome<Boolean> written =
              Io.writeBytesNio(
                  outputPath,
                  Io.gzipFast(TypedAst.List.newBuilder().addTypedAsts(ast).build().toByteArray()));
          if (written.isErr()) {
            throw new IllegalArgumentException(
                "Cannot create TypedAst output file", written.error());
          }
        };
    return new SerializeTypedAstPass(compiler, toPath, serializationOptions);
  }

  @Override
  public void process(Node externs, Node root) {
    new RemoveCastNodes(compiler).process(externs, root);
    TypedAstSerializer serializer = new TypedAstSerializer(this.compiler, serializationOptions);
    TypedAst ast = serializer.serializeRoots(externs, root);
    consumer.accept(ast);
  }
}
