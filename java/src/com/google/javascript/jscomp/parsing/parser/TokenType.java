/*
 * Copyright 2011 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.parsing.parser;

import java.util.Optional;

// 7.5 Tokens
public enum TokenType {
  END_OF_FILE(Optional.of("End of File")),
  ERROR(Optional.of("error")),

  // 7.6 Identifier Names and Identifiers
  IDENTIFIER(Optional.of("identifier")),

  // 7.6.1.1 keywords
  BREAK,
  CASE,
  CATCH,
  CONTINUE,
  DEBUGGER,
  DEFAULT,
  DELETE,
  DO,
  ELSE,
  FINALLY,
  FOR,
  FUNCTION,
  IF,
  IN,
  INSTANCEOF,
  NEW,
  RETURN,
  SWITCH,
  THIS,
  THROW,
  TRY,
  TYPEOF,
  VAR,
  VOID,
  WHILE,
  WITH,

  // 7.6.1.2 Future reserved words
  CLASS,
  CONST,
  ENUM,
  EXPORT,
  EXTENDS,
  IMPORT,
  SUPER,

  // Future reserved words in strict mode
  IMPLEMENTS,
  INTERFACE,
  LET,
  PACKAGE,
  PRIVATE,
  PROTECTED,
  PUBLIC,
  STATIC,
  YIELD,

  // 7.7 Punctuators
  OPEN_CURLY(Optional.of("{")),
  CLOSE_CURLY(Optional.of("}")),
  OPEN_PAREN(Optional.of("(")),
  CLOSE_PAREN(Optional.of(")")),
  OPEN_SQUARE(Optional.of("[")),
  CLOSE_SQUARE(Optional.of("]")),
  PERIOD(Optional.of(".")),
  SEMI_COLON(Optional.of(";")),
  COMMA(Optional.of(",")),
  OPEN_ANGLE(Optional.of("<")),
  CLOSE_ANGLE(Optional.of(">")),
  LESS_EQUAL(Optional.of("<=")),
  GREATER_EQUAL(Optional.of(">=")),
  ARROW(Optional.of("=>")),
  EQUAL_EQUAL(Optional.of("==")),
  NOT_EQUAL(Optional.of("!=")),
  EQUAL_EQUAL_EQUAL(Optional.of("===")),
  NOT_EQUAL_EQUAL(Optional.of("!==")),
  PLUS(Optional.of("+")),
  MINUS(Optional.of("-")),
  STAR(Optional.of("*")),
  STAR_STAR(Optional.of("**")),
  PERCENT(Optional.of("%")),
  PLUS_PLUS(Optional.of("++")),
  MINUS_MINUS(Optional.of("--")),
  LEFT_SHIFT(Optional.of("<<")),
  RIGHT_SHIFT(Optional.of(">>")),
  UNSIGNED_RIGHT_SHIFT(Optional.of(">>>")),
  AMPERSAND(Optional.of("&")),
  BAR(Optional.of("|")),
  CARET(Optional.of("^")),
  BANG(Optional.of("!")),
  TILDE(Optional.of("~")),
  AND(Optional.of("&&")),
  OR(Optional.of("||")),
  QUESTION(Optional.of("?")),
  QUESTION_QUESTION(Optional.of("??")),
  QUESTION_DOT(Optional.of("?.")),
  COLON(Optional.of(":")),
  EQUAL(Optional.of("=")),
  PLUS_EQUAL(Optional.of("+=")),
  MINUS_EQUAL(Optional.of("-=")),
  STAR_EQUAL(Optional.of("*=")),
  STAR_STAR_EQUAL(Optional.of("**=")),
  PERCENT_EQUAL(Optional.of("%=")),
  LEFT_SHIFT_EQUAL(Optional.of("<<=")),
  RIGHT_SHIFT_EQUAL(Optional.of(">>=")),
  UNSIGNED_RIGHT_SHIFT_EQUAL(Optional.of(">>>=")),
  AMPERSAND_EQUAL(Optional.of("&=")),
  BAR_EQUAL(Optional.of("|=")),
  CARET_EQUAL(Optional.of("^=")),
  SLASH(Optional.of("/")),
  SLASH_EQUAL(Optional.of("/=")),

  // Logical Assignment Punctuators
  AND_EQUAL(Optional.of("&&=")),
  OR_EQUAL(Optional.of("||=")),
  QUESTION_QUESTION_EQUAL(Optional.of("??=")),

  // 7.8 Literals
  NULL,
  TRUE,
  FALSE,
  NUMBER(Optional.of("number literal")),
  STRING(Optional.of("string literal")),
  BIGINT(Optional.of("bigint literal")),
  REGULAR_EXPRESSION(Optional.of("regular expression literal")),

  // Harmony extensions
  ELLIPSIS(Optional.of("...")),
  // 12.2.9 Template Literals
  // Template literal tokens corresponding to different parts of the literal
  // Eg: `hello` is scanned as a single NO_SUBSTITUTION_TEMPLATE: hello
  // `hello${world}!` is scanned as TEMPLATE_HEAD: hello, TEMPLATE_MIDDLE: world,
  // and TEMPLATE_TAIL: !
  TEMPLATE_HEAD(Optional.of("template head")),
  TEMPLATE_MIDDLE(Optional.of("template middle")),
  TEMPLATE_TAIL(Optional.of("template tail")),
  NO_SUBSTITUTION_TEMPLATE(Optional.of("no substitution template")),

  // TypeScript
  TYPE,
  DECLARE,
  MODULE,
  NAMESPACE;

  public final Optional<String> value;

  TokenType() {
    this(Optional.empty());
  }

  TokenType(Optional<String> value) {
    this.value = value;
  }

  @Override
  public String toString() {
    // TODO: straighten this out
    return value.isEmpty() ? Keywords.get(this).toString() : value.get();
  }
}
