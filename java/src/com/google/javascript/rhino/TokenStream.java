/*
 *
 * ***** BEGIN LICENSE BLOCK *****
 * Version: MPL 1.1/GPL 2.0
 *
 * The contents of this file are subject to the Mozilla Public License Version
 * 1.1 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * http://www.mozilla.org/MPL/
 *
 * Software distributed under the License is distributed on an "AS IS" basis,
 * WITHOUT WARRANTY OF ANY KIND, either express or implied. See the License
 * for the specific language governing rights and limitations under the
 * License.
 *
 * The Original Code is Rhino code, released
 * May 6, 1999.
 *
 * The Initial Developer of the Original Code is
 * Netscape Communications Corporation.
 * Portions created by the Initial Developer are Copyright (C) 1997-1999
 * the Initial Developer. All Rights Reserved.
 *
 * Contributor(s):
 *   Roger Lawrence
 *   Mike McCabe
 *   Igor Bukanov
 *   Ethan Hugg
 *   Bob Jervis
 *   Terry Lucas
 *   Milen Nankov
 *   Pascal-Louis Perez
 *
 * Alternatively, the contents of this file may be used under the terms of
 * the GNU General Public License Version 2 or later (the "GPL"), in which
 * case the provisions of the GPL are applicable instead of those above. If
 * you wish to allow use of your version of this file only under the terms of
 * the GPL and not to allow others to use your version of this file under the
 * MPL, indicate your decision by deleting the provisions above and replacing
 * them with the notice and other provisions required by the GPL. If you do
 * not delete the provisions above, a recipient may use your version of this
 * file under either the MPL or the GPL.
 *
 * ***** END LICENSE BLOCK ***** */

package com.google.javascript.rhino;

import java.util.Optional;

/**
 * This class implements the JavaScript scanner.
 *
 * <p>It is based on the C source files jsscan.c and jsscan.h in the jsref package.
 *
 * <p>IMPORTANT: As of 2018-03-09 it is still not possible to use Java 8 features in this file due
 * to limitations on some internal Google projects that depend on it.
 */
public final class TokenStream {
  /**
   * Is the string an ES3 keyword?
   *
   * <p>Keywords for versions of JavaScript after ES3 are not included, because the parser would
   * reject valid ES3 code that happened to newer keywords if we did that.
   *
   * <p>Since isKeyword() is used repeatedly in scanning, for performance reasons, it is implemented
   * to do the minimum number of character comparisons to ascertain whether the given name is a
   * keyword, instead of doing string search of the name against JS keywords.
   */
  @SuppressWarnings("StatementSwitchToExpressionSwitch") // Tricky code written with old switches
  public static boolean isKeyword(String name) {
        boolean id = false;
        String s = name;
        complete: {
            Optional<String> X = Optional.empty();
            int c;
            partial: switch (s.length()) {
            case 2: c=s.charAt(1);
                if (c=='f') {
                  if (s.charAt(0)=='i') {id=true; break complete;}
                } else if (c=='n') {
                  if (s.charAt(0)=='i') {id=true; break complete;}
                } else if (c=='o') {
                  if (s.charAt(0)=='d') {id=true; break complete;}
                }
                break partial;
            case 3: switch (s.charAt(0)) {
                case 'f':
                  if (s.charAt(2)=='r' && s.charAt(1)=='o') {
                    id=true; break complete;
                  } break partial;
                case 'i':
                  if (s.charAt(2)=='t' && s.charAt(1)=='n') {
                    id=true; break complete;
                  } break partial;
                case 'n':
                  if (s.charAt(2)=='w' && s.charAt(1)=='e') {
                    id=true; break complete;
                  } break partial;
                case 't':
                  if (s.charAt(2)=='y' && s.charAt(1)=='r') {
                    id=true; break complete;
                  } break partial;
                case 'v':
                  if (s.charAt(2)=='r' && s.charAt(1)=='a') {
                    id=true; break complete;
                  } break partial;
                } break partial;
            case 4: switch (s.charAt(0)) {
                case 'b': X=Optional.of("byte");id=true; break partial;
                case 'c': c=s.charAt(3);
                    if (c=='e') { if (s.charAt(2)=='s' && s.charAt(1)=='a') {
                            id=true; break complete;} }
                    else if (c=='r') {
                      if (s.charAt(2)=='a' && s.charAt(1)=='h') {
                        id=true; break complete;
                      }
                    }
                    break partial;
                case 'e': c=s.charAt(3);
                    if (c=='e') { if (s.charAt(2)=='s' && s.charAt(1)=='l') {
                            id=true; break complete;} }
                    else if (c=='m') {
                      if (s.charAt(2)=='u' && s.charAt(1)=='n') {
                            id=true; break complete;} }
                    break partial;
                case 'g': X=Optional.of("goto");id=true; break partial;
                case 'l': X=Optional.of("long");id=true; break partial;
                case 'n': X=Optional.of("null");id=true; break partial;
                case 't': c=s.charAt(3);
                    if (c=='e') { if (s.charAt(2)=='u' && s.charAt(1)=='r') {
                            id=true; break complete;} }
                    else if (c=='s') {
                      if (s.charAt(2)=='i' && s.charAt(1)=='h') {
                            id=true; break complete;} }
                    break partial;
                case 'v': X=Optional.of("void");id=true; break partial;
                case 'w': X=Optional.of("with");id=true; break partial;
                } break partial;
            case 5: switch (s.charAt(2)) {
                case 'a': X=Optional.of("class");id=true; break partial;
                case 'e': X=Optional.of("break");id=true; break partial;
                case 'i': X=Optional.of("while");id=true; break partial;
                case 'l': X=Optional.of("false");id=true; break partial;
                case 'n': c=s.charAt(0);
                    if (c=='c') { X=Optional.of("const");id=true; }
                    else if (c=='f') { X=Optional.of("final");id=true; }
                    break partial;
                case 'o': c=s.charAt(0);
                    if (c=='f') { X=Optional.of("float");id=true; }
                    else if (c=='s') { X=Optional.of("short");id=true; }
                    break partial;
                case 'p': X=Optional.of("super");id=true; break partial;
                case 'r': X=Optional.of("throw");id=true; break partial;
                case 't': X=Optional.of("catch");id=true; break partial;
                } break partial;
            case 6: switch (s.charAt(1)) {
                case 'a': X=Optional.of("native");id=true; break partial;
                case 'e': c=s.charAt(0);
                    if (c=='d') { X=Optional.of("delete");id=true; }
                    else if (c=='r') { X=Optional.of("return");id=true; }
                    break partial;
                case 'h': X=Optional.of("throws");id=true; break partial;
                case 'm': X=Optional.of("import");id=true; break partial;
                case 'o': X=Optional.of("double");id=true; break partial;
                case 't': X=Optional.of("static");id=true; break partial;
                case 'u': X=Optional.of("public");id=true; break partial;
                case 'w': X=Optional.of("switch");id=true; break partial;
                case 'x': X=Optional.of("export");id=true; break partial;
                case 'y': X=Optional.of("typeof");id=true; break partial;
                } break partial;
            case 7: switch (s.charAt(1)) {
                case 'a': X=Optional.of("package");id=true; break partial;
                case 'e': X=Optional.of("default");id=true; break partial;
                case 'i': X=Optional.of("finally");id=true; break partial;
                case 'o': X=Optional.of("boolean");id=true; break partial;
                case 'r': X=Optional.of("private");id=true; break partial;
                case 'x': X=Optional.of("extends");id=true; break partial;
                } break partial;
            case 8: switch (s.charAt(0)) {
                case 'a': X=Optional.of("abstract");id=true; break partial;
                case 'c': X=Optional.of("continue");id=true; break partial;
                case 'd': X=Optional.of("debugger");id=true; break partial;
                case 'f': X=Optional.of("function");id=true; break partial;
                case 'v': X=Optional.of("volatile");id=true; break partial;
                } break partial;
            case 9: c=s.charAt(0);
                if (c=='i') { X=Optional.of("interface");id=true; }
                else if (c=='p') { X=Optional.of("protected");id=true; }
                else if (c=='t') { X=Optional.of("transient");id=true; }
                break partial;
            case 10: c=s.charAt(1);
                if (c=='m') { X=Optional.of("implements");id=true; }
                else if (c=='n') { X=Optional.of("instanceof");id=true; }
                break partial;
            case 12: X=Optional.of("synchronized");id=true; break partial;
            }
      // partial match validate the entire string the one possibility
      if (X.isPresent() && X.get() != s && !X.get().equals(s)) {
        return false;
      }
        }
        return id;
    }

  public static boolean isJSIdentifier(String s) {
    return JSIdentifier.isJSIdentifier(s);
  }

  private TokenStream() {}
}
