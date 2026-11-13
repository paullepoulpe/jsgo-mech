/*
 * Copyright 2008 The Closure Compiler Authors.
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
import static com.google.javascript.jscomp.base.JavaRegex.DOT;
import static com.google.javascript.jscomp.base.JavaRegex.NON_SPACE;

import com.google.common.annotations.VisibleForTesting;
import com.google.common.base.Ascii;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.debugging.sourcemap.proto.Mapping.OriginalMapping;
import com.google.javascript.jscomp.JsMessage.Hash;
import com.google.javascript.jscomp.JsMessage.Part;
import com.google.javascript.jscomp.JsMessage.PlaceholderFormatException;
import com.google.javascript.jscomp.JsMessage.PlaceholderReference;
import com.google.javascript.jscomp.JsMessage.StringPart;
import com.google.javascript.jscomp.NodeTraversal.AbstractPostOrderCallback;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.rhino.JSDocInfo;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.Token;
import com.google.re2j.Matcher;
import com.google.re2j.Pattern;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Map.Entry;
import java.util.Optional;
import java.util.Set;
import java.util.function.Function;
import org.jspecify.annotations.Nullable;

/**
 * Locates JS code that is intended to declare localizable messages.
 *
 * <p>It passes each found message to either {@link
 * JsMessageVisitor#processJsMessageDefinition(JsMessageDefinition)} for {@code goog.getMsg()} calls
 * or {@link JsMessageVisitor#processIcuTemplateDefinition(IcuTemplateDefinition)} for {@code
 * goog.i18n.messages.declareIcuTemplate()} calls.
 */
public abstract class JsMessageVisitor extends AbstractPostOrderCallback implements CompilerPass {

  private static final String MSG_FUNCTION_NAME = "getMsg";
  private static final String MSG_FUNCTION_QNAME = "goog." + MSG_FUNCTION_NAME;
  private static final String ICU_MSG_FUNCTION_NAME = "declareIcuTemplate";
  private static final String ICU_MSG_FUNCTION_QNAME =
      "goog.i18n.messages." + ICU_MSG_FUNCTION_NAME;
  private static final String MSG_FALLBACK_FUNCTION_NAME = "goog.getMsgWithFallback";

  /**
   * Expose a list of functions that are used in message extraction. Standalone tools that run
   * message extraction can use this list to filter out files that do not contain any of these
   * functions.
   */
  public static final List<String> FUNCTIONS_USED_IN_MESSAGE_EXTRACTION =
      ImmutableList.of(MSG_FUNCTION_NAME, ICU_MSG_FUNCTION_NAME);

  /**
   * Identifies a message with a specific ID which doesn't get extracted from the JS code containing
   * its declaration.
   *
   * <pre><code>
   *   // A message with ID 1234 whose original definition comes from some source other than this
   *   // JS code.
   *   const MSG_EXTERNAL_1234 = goog.getMsg("Some message.");
   * </code></pre>
   */
  private static final String MSG_EXTERNAL_PREFIX = "MSG_EXTERNAL_";

  static final DiagnosticType MESSAGE_HAS_NO_DESCRIPTION =
      DiagnosticType.warning(
          "JSC_MSG_HAS_NO_DESCRIPTION", "Message {0} has no description. Add @desc JsDoc tag.");

  static final DiagnosticType MESSAGE_HAS_NO_TEXT =
      DiagnosticType.warning(
          "JSC_MSG_HAS_NO_TEXT",
          "Message value of {0} is just an empty string. " + "Empty messages are forbidden.");

  public static final DiagnosticType MESSAGE_TREE_MALFORMED =
      DiagnosticType.error("JSC_MSG_TREE_MALFORMED", "Message parse tree malformed. {0}");

  static final DiagnosticType MESSAGE_HAS_NO_VALUE =
      DiagnosticType.error("JSC_MSG_HAS_NO_VALUE", "message node {0} has no value");

  static final DiagnosticType MESSAGE_DUPLICATE_KEY =
      DiagnosticType.error(
          "JSC_MSG_KEY_DUPLICATED",
          "duplicate message variable name found for {0}, " + "initial definition {1}:{2}");

  static final DiagnosticType MESSAGE_NODE_IS_ORPHANED =
      DiagnosticType.error(
          "JSC_MSG_ORPHANED_NODE",
          "{0}() function may be used only with MSG_* property or variable");

  public static final DiagnosticType MESSAGE_NOT_INITIALIZED_CORRECTLY =
      DiagnosticType.warning(
          "JSC_MSG_NOT_INITIALIZED_CORRECTLY",
          "Message must be initialized using a call to "
              + MSG_FUNCTION_QNAME
              + " or "
              + ICU_MSG_FUNCTION_QNAME);

  public static final DiagnosticType BAD_FALLBACK_SYNTAX =
      DiagnosticType.error(
          "JSC_MSG_BAD_FALLBACK_SYNTAX",
          String.format(
              "Bad syntax. " + "Expected syntax: %s(MSG_1, MSG_2)", MSG_FALLBACK_FUNCTION_NAME));

  public static final DiagnosticType FALLBACK_ARG_ERROR =
      DiagnosticType.error(
          "JSC_MSG_FALLBACK_ARG_ERROR", "Could not find message entry for fallback argument {0}");

  static final String MSG_PREFIX = "MSG_";

  /**
   * ScopedAliases pass transforms the goog.scope declarations to have a unique id as prefix.
   *
   * <p>After '$jscomp$scope$' expect some sequence of digits and dollar signs ending in '$MSG_
   */
  private static final Pattern SCOPED_ALIASES_PREFIX_PATTERN =
      Pattern.compile("\\$jscomp\\$scope\\$" + NON_SPACE + "+\\$MSG_");

  /**
   * Pattern for unnamed messages.
   *
   * <p>All JS messages in JS code should have unique name but messages in generated code (i.e. from
   * soy template) could have duplicated message names. Later we replace the message names with ids
   * constructed as a hash of the message content.
   *
   * <p><a href="https://github.com/google/closure-templates">Soy</a> generates messages with names
   * MSG_UNNAMED.* . This pattern recognizes such messages.
   */
  private static final Pattern MSG_UNNAMED_PATTERN = Pattern.compile("MSG_UNNAMED" + DOT + "*");

  private final JsMessage.IdGenerator idGenerator;
  final AbstractCompiler compiler;

  /**
   * The names encountered associated with their defining node and source. We use it for tracking
   * duplicated message ids in the source code.
   */
  private final Map<String, MessageLocation> messageNames = new LinkedHashMap<>();

  /** Track unnamed messages by Var, not string, as they are not guaranteed to be globally unique */
  private final Map<Var, JsMessage> unnamedMessages = new LinkedHashMap<>();

  /**
   * Set of found goog.getMsg call nodes.
   *
   * <p>When we visit goog.getMsg() node we add it, and later when we visit its parent we remove it.
   * All nodes that are left at the end of traversing are orphaned nodes. It means have no
   * corresponding var or property node.
   */
  private final Set<Node> googMsgNodes = new LinkedHashSet<>();

  /**
   * Creates JS message visitor.
   *
   * @param compiler the compiler instance
   * @param idGenerator generator that used for creating unique ID for the message
   */
  protected JsMessageVisitor(AbstractCompiler compiler, JsMessage.IdGenerator idGenerator) {

    this.compiler = compiler;
    this.idGenerator = idGenerator;

    // TODO(anatol): add flag that decides whether to process UNNAMED messages.
    // Some projects would not want such functionality (unnamed) as they don't
    // use SOY templates.
  }

  @Override
  public void process(Node externs, Node root) {
    NodeTraversal.traverse(compiler, root, this);

    for (Node msgNode : googMsgNodes) {
      compiler.report(
          JSError.make(
              msgNode, MESSAGE_NODE_IS_ORPHANED, msgNode.getFirstChild().getQualifiedName()));
    }
  }

  @Override
  public void visit(NodeTraversal traversal, Node node, Node unused) {
    collectGetMsgCall(traversal, node);
    checkMessageInitialization(traversal, node);
  }

  /** This method is called for every Node in the sources AST. */
  private void checkMessageInitialization(NodeTraversal traversal, Node node) {
    final Node parent = node.getParent();

    final String originalMessageKey;
    String possiblyObfuscatedMessageKey;
    final Node msgNode;
    final JSDocInfo jsDocInfo;

    switch (node.getToken()) {
      case NAME -> {
        // Case: `var MSG_HELLO = 'Message';`
        if (parent == null || !NodeUtil.isNameDeclaration(parent)) {
          return;
        }

        possiblyObfuscatedMessageKey = node.getString();
        originalMessageKey = node.getOriginalName();
        msgNode = node.getFirstChild();
        jsDocInfo = parent.getJSDocInfo();
      }
      case ASSIGN -> {
        // Case: `somenamespace.someclass.MSG_HELLO = 'Message';`
        Node getProp = node.getFirstChild();
        if (!getProp.isGetProp()) {
          return;
        }

        possiblyObfuscatedMessageKey = getProp.getString();
        originalMessageKey = getProp.getOriginalName();
        msgNode = node.getLastChild();
        jsDocInfo = node.getJSDocInfo();
      }
      case STRING_KEY -> {
        // Case: `var t = {MSG_HELLO: 'Message'}`;
        if (node.isQuotedStringKey() || !node.hasChildren() || parent.isObjectPattern()) {
          // Don't require goog.getMsg() for quoted keys
          // Case: `var msgs = { 'MSG_QUOTED': anything };`
          //
          // Don't try to require goog.getMsg() for destructuring assignment targets.
          // goog.getMsg() needs to be used in a direct assignment to a variable or property
          // only.
          // Case: `var {MSG_HELLO} = anything;
          // Case: `var {something: MSG_HELLO} = anything;
          return;
        }
        checkState(parent.isObjectLit(), parent);

        possiblyObfuscatedMessageKey = node.getString();
        originalMessageKey = node.getOriginalName();
        msgNode = node.getFirstChild();
        jsDocInfo = node.getJSDocInfo();
      }
      case MEMBER_FIELD_DEF -> {
        // Case: `class Foo { MSG_HELLO = 'Message'; }`
        possiblyObfuscatedMessageKey = node.getString();
        originalMessageKey = node.getOriginalName();
        msgNode = node.getFirstChild();
        jsDocInfo = node.getJSDocInfo();
      }
      default -> {
        return;
      }
    }

    String messageKeyFromLhs =
        !originalMessageKey.isEmpty() ? originalMessageKey : possiblyObfuscatedMessageKey;

    // If we've reached this point, then messageKey is the name of a variable or a property that is
    // being assigned a value and msgNode is the Node representing the value being assigned.
    // However, we haven't actually determined yet that name looks like it should be a translatable
    // message or that the value is a call to goog.getMsg().

    // Is this a message name?
    boolean msgNodeIsACall = msgNode != null && msgNode.isCall();

    if (!isMessageName(messageKeyFromLhs)) {
      return;
    }

    if (msgNode == null) {
      compiler.report(JSError.make(node, MESSAGE_HAS_NO_VALUE, messageKeyFromLhs));
      return;
    }

    if (isLegalMessageVarAlias(msgNode)) {
      return;
    }

    // Report a warning if a qualified messageKey that looks like a message (e.g. "a.b.MSG_X")
    // doesn't use goog.getMsg().
    if (msgNodeIsACall) {
      googMsgNodes.remove(msgNode);
    } else {
      compiler.report(JSError.make(node, MESSAGE_NOT_INITIALIZED_CORRECTLY));
      return;
    }

    OriginalMapping mapping =
        compiler.getSourceMapping(traversal.getSourceName(), node.getLineno(), node.getCharno());
    final String sourceName;
    if (mapping != null) {
      sourceName = mapping.getOriginalFile() + ":" + mapping.getLineNumber();
    } else {
      sourceName = traversal.getSourceName().orElse("null") + ":" + node.getLineno();
    }

    checkState(msgNode.isCall());
    final Node fnNameNode = msgNode.getFirstChild();
    if (isDeclareIcuTemplateCallee(fnNameNode)) {
      Outcome<IcuTemplateDefinition> icuTemplateDefinitionOutcome =
          extractIcuTemplateDefinition(msgNode, jsDocInfo, messageKeyFromLhs, sourceName);
      if (icuTemplateDefinitionOutcome.isErr()) {
        MalformedException ex = (MalformedException) icuTemplateDefinitionOutcome.error();
        compiler.report(JSError.make(ex.getNode(), MESSAGE_TREE_MALFORMED, ex.getMessage()));
        return;
      }
      final IcuTemplateDefinition icuTemplateDefinition = icuTemplateDefinitionOutcome.get();
      final JsMessage extractedMessage = icuTemplateDefinition.getMessage();
      trackMessage(traversal, possiblyObfuscatedMessageKey, msgNode, extractedMessage);
      reportErrorIfEmptyMessage(node, extractedMessage);
      processIcuTemplateDefinition(icuTemplateDefinition);
    } else if (fnNameNode.matchesQualifiedName(MSG_FUNCTION_QNAME)) {
      Outcome<JsMessageDefinition> jsMessageDefinitionOutcome =
          extractJsMessageDefinition(msgNode, jsDocInfo, messageKeyFromLhs, sourceName);
      if (jsMessageDefinitionOutcome.isErr()) {
        MalformedException ex = (MalformedException) jsMessageDefinitionOutcome.error();
        compiler.report(JSError.make(ex.getNode(), MESSAGE_TREE_MALFORMED, ex.getMessage()));
        return;
      }
      final JsMessageDefinition jsMessageDefinition = jsMessageDefinitionOutcome.get();
      final JsMessage extractedMessage = jsMessageDefinition.getMessage();
      trackMessage(traversal, possiblyObfuscatedMessageKey, msgNode, extractedMessage);
      reportErrorIfEmptyMessage(node, extractedMessage);

      // goog.getMsg() calls are required to have `@desc` unless they are external.
      final Optional<String> desc = extractedMessage.getDesc();
      if ((desc.isEmpty() || desc.get().trim().isEmpty()) && !extractedMessage.isExternal()) {
        compiler.report(
            JSError.make(node, MESSAGE_HAS_NO_DESCRIPTION, extractedMessage.getKey()));
      }

      processJsMessageDefinition(jsMessageDefinition);
    } else {
      compiler.report(
          JSError.make(
              msgNode,
              MESSAGE_TREE_MALFORMED,
              "Message must be initialized using a call to "
                  + MSG_FUNCTION_QNAME
                  + " or "
                  + ICU_MSG_FUNCTION_NAME
                  + " (from goog.i18n.messages)."));
    }
  }

  private boolean isDeclareIcuTemplateCallee(Node calleeNode) {
    // NOTE: We're requiring that the imported ICU template function name match its original name.
    // This is an intentional limitation, since it should be less confusing to users as well as
    // to this program.
    // TODO(bradfordcsmith): Make this check more robust, possibly building on ModuleMetadata or
    // ProcessClosurePrimitives.
    final String qualifiedName = calleeNode.getQualifiedName();
    return !qualifiedName.isEmpty() && qualifiedName.endsWith(ICU_MSG_FUNCTION_NAME);
  }

  interface InputForBuildJsMsg {
    String getMessageKeyFromLhs();

    String getSourceName();

    String getMessageText();

    ImmutableList<Part> getMessageParts();

    Map<String, String> getPlaceholderExampleMap();

    Map<String, String> getPlaceholderOriginalCodeMap();

    Optional<String> getDescription();

    Optional<String> getMeaning();

    Optional<String> getAlternateMessageId();
  }

  private JsMessage buildJsMessage(InputForBuildJsMsg input) {
    final ImmutableList<Part> messageParts = input.getMessageParts();
    final String messageKeyFromLhs = input.getMessageKeyFromLhs();

    // non-null for `MSG_EXTERNAL_12345`
    final Optional<String> externalMessageId = getExternalMessageId(messageKeyFromLhs);

    final boolean isAnonymous;
    final boolean isExternal;
    final Optional<String> messageId;
    final String messageKeyFinal;
    if (externalMessageId.isPresent()) {
      // MSG_EXTERNAL_12345 = ...
      isAnonymous = false;
      isExternal = true;
      messageId = externalMessageId;
      messageKeyFinal = messageKeyFromLhs;
    } else {
      isExternal = false;
      // We will need to generate the message ID from a combination of the message text and
      // its "meaning". If the code explicitly specifies a "meaning" string we'll use that,
      // otherwise we'll use the message key as the meaning
      Optional<String> meaningForIdGeneration = input.getMeaning();
      if (isUnnamedMessageName(messageKeyFromLhs)) {
        // MSG_UNNAMED_XXXX = goog.getMsg(....);
        // JS code that is automatically generated uses this, since it is harder for it to create
        // message variable names that are guaranteed to be unique.
        isAnonymous = true;
        messageKeyFinal = generateKeyFromMessageText(input.getMessageText());
        if (meaningForIdGeneration.isEmpty()) {
          meaningForIdGeneration = Optional.of(messageKeyFinal);
        }
      } else {
        isAnonymous = false;
        messageKeyFinal = messageKeyFromLhs;
        if (meaningForIdGeneration.isEmpty()) {
          // Transpilation of goog.scope() may have added a prefix onto the variable name,
          // which we need to strip off when we treat it as a "meaning" for ID generation purposes.
          // Otherwise, the message ID would change if a goog.scope() call were added or removed.
          meaningForIdGeneration = Optional.of(removeScopedAliasesPrefix(messageKeyFromLhs));
        }
      }
      messageId =
          idGenerator == null
              ? meaningForIdGeneration
              : Optional.of(idGenerator.generateId(meaningForIdGeneration, messageParts));
    }

    return new JsMessage.Builder()
        .appendParts(messageParts)
        .setPlaceholderNameToExampleMap(input.getPlaceholderExampleMap())
        .setPlaceholderNameToOriginalCodeMap(input.getPlaceholderOriginalCodeMap())
        .setIsAnonymous(isAnonymous)
        .setIsExternalMsg(isExternal)
        .setKey(messageKeyFinal)
        .setSourceName(input.getSourceName())
        .setDesc(input.getDescription())
        // NOTE: JsMessageXmbWriter does NOT just take this value and write it to the XMB
        // file as the message meaning. It has its own code that defaults a null value to
        // the value of the key field, then prefixes that with a project ID, if any.
        // The intention of that seems to have been to make sure the meaning value in the XMB
        // file matches what was actually used by the ID generator above.
        .setMeaning(input.getMeaning())
        .setAlternateId(input.getAlternateMessageId())
        .setId(messageId)
        .build();
  }

  private void trackMessage(
      NodeTraversal traversal,
      String possiblyObfuscatedMessageKey,
      Node msgNode,
      JsMessage extractedMessage) {
    // If asked to check named internal messages.
    if (!extractedMessage.isAnonymous() && !extractedMessage.isExternal()) {
      checkIfMessageDuplicated(extractedMessage.getKey(), msgNode);
    }
    if (extractedMessage.isAnonymous()) {
      trackUnnamedMessage(traversal, extractedMessage, possiblyObfuscatedMessageKey);
    } else {
      trackNormalMessage(extractedMessage, extractedMessage.getKey(), msgNode);
    }
  }

  private void reportErrorIfEmptyMessage(Node node, JsMessage extractedMessage) {
    if (extractedMessage.isEmpty()) {
      // value of the message is an empty string. Translators do not like it.
      compiler.report(JSError.make(node, MESSAGE_HAS_NO_TEXT, extractedMessage.getKey()));
    }
  }

  /**
   * Extracts an external message ID from the message key, if it contains one.
   *
   * @param messageKey the message key (usually the variable or property name)
   * @return the external ID if it is found, otherwise `null`
   */
  public static Optional<String> getExternalMessageId(String messageKey) {
    if (messageKey.startsWith(MSG_EXTERNAL_PREFIX)) {
      int start = MSG_EXTERNAL_PREFIX.length();
      int end = start;
      for (; end < messageKey.length(); end++) {
        char c = messageKey.charAt(end);
        if (c > '9' || c < '0') {
          break;
        }
      }
      if (end > start) {
        return Optional.of(messageKey.substring(start, end));
      }
    }
    return Optional.empty();
  }

  private String generateKeyFromMessageText(String msgText) {
    long nonnegativeHash = Long.MAX_VALUE & Hash.hash64(Optional.of(msgText));
    return MSG_PREFIX + Ascii.toUpperCase(Long.toString(nonnegativeHash, 36));
  }

  private void collectGetMsgCall(NodeTraversal traversal, Node call) {
    if (!call.isCall()) {
      return;
    }

    // goog.getMsg()
    final Node callee = call.getFirstChild();
    if (callee.matchesQualifiedName(MSG_FUNCTION_QNAME) || isDeclareIcuTemplateCallee(callee)) {
      googMsgNodes.add(call);
    } else if (callee.matchesQualifiedName(MSG_FALLBACK_FUNCTION_NAME)) {
      visitFallbackFunctionCall(traversal, call);
    }
  }

  /**
   * Track a message for later retrieval.
   *
   * <p>This is used for tracking duplicates, and for figuring out message fallback. Not all message
   * types are trackable, because that would require a more sophisticated analysis. e.g., function
   * f(s) { s.MSG_UNNAMED_X = 'Some untrackable message'; }
   */
  private void trackNormalMessage(JsMessage message, String msgName, Node msgNode) {
    MessageLocation location = new MessageLocation(message, msgNode);
    messageNames.put(msgName, location);
  }

  /**
   * Track an unnamed message for later retrieval.
   *
   * <p>This is used for figuring out message fallbacks. Message duplicates are allowed for unnamed
   * messages.
   */
  private void trackUnnamedMessage(NodeTraversal t, JsMessage message, String msgNameInScope) {
    Var var = t.getScope().getVar(msgNameInScope);
    if (var != null) {
      unnamedMessages.put(var, message);
    }
  }

  /**
   * Defines any special cases that are exceptions to what would otherwise be illegal message
   * assignments.
   *
   * <p>These exceptions are generally due to the pass being designed before new syntax was
   * introduced.
   *
   * @param msgNode Node representing the value assigned to the message variable or property
   */
  private static boolean isLegalMessageVarAlias(Node msgNode) {
    if (msgNode.isGetProp()
        && msgNode.isQualifiedName()
        && msgNode.getString().startsWith(MSG_PREFIX)) {
      // Case: `foo.Thing.MSG_EXAMPLE_ALIAS = bar.OtherThing.MSG_EXAMPLE;`
      //
      // This kind of construct is created by TypeScript code generation and
      // ConcretizeStaticInheritanceForInlining. Just ignore it; the message will have already been
      // extracted
      // from the base class.
      return true;
    }

    if (!msgNode.isName()) {
      return false;
    }

    String originalName =
        (!msgNode.getOriginalName().isEmpty()) ? msgNode.getOriginalName() : msgNode.getString();

    if (originalName.startsWith(MSG_PREFIX)) {
      // Creating an alias for a message is also allowed, and sometimes happens in generated code,
      // including some of the code generated by this compiler's transpilations.
      // e.g.
      // `var MSG_EXAMPLE_ALIAS = MSG_EXAMPLE;`
      // `var {MSG_HELLO_ALIAS} = MSG_HELLO;
      // `var {MSG_HELLO_ALIAS: goog$module$my$module_MSG_HELLO} = x;`).
      // `exports = {MSG_FOO}`
      // or `exports = {MSG_FOO: MSG_FOO}` when used with declareLegacyNamespace.
      return true;
    }

    return false;
  }

  /** Get a previously tracked unnamed message */
  private @Nullable JsMessage getTrackedUnnamedMessage(NodeTraversal t, String msgNameInScope) {
    Var var = t.getScope().getVar(msgNameInScope);
    if (var != null) {
      return unnamedMessages.get(var);
    }
    return null;
  }

  /** Get a previously tracked message. */
  private @Nullable JsMessage getTrackedNormalMessage(String msgName) {
    MessageLocation location = messageNames.get(msgName);
    return location == null ? null : location.message;
  }

  /**
   * Checks if message already processed. If so - it generates 'message duplicated' compiler error.
   *
   * @param msgName the name of the message
   * @param msgNode the node that represents JS message
   */
  private void checkIfMessageDuplicated(String msgName, Node msgNode) {
    if (messageNames.containsKey(msgName)) {
      MessageLocation location = messageNames.get(msgName);
      compiler.report(
          JSError.make(
              msgNode,
              MESSAGE_DUPLICATE_KEY,
              msgName,
              location.messageNode.getSourceFileName(),
              Integer.toString(location.messageNode.getLineno())));
    }
  }

  /**
   * Returns the string value associated with a node representing a JS string or several JS strings
   * added together (e.g. {@code 'str'} or {@code 's' + 't' + 'r'}).
   *
   * @param node the node from where we extract the string
   * @return String representation of the node, or an error outcome with a MalformedException if
   *     the node is not a string literal or concatenation of them
   */
  private static Outcome<String> extractStringFromStringExprNode(Node node) {
    switch (node.getToken()) {
      case STRINGLIT -> {
        return Outcome.ok(node.getString());
      }
      case TEMPLATELIT -> {
        if (node.hasOneChild()) {
          // Cooked string can be null only for tagged template literals.
          // A tagged template literal would hit the default case below.
          return Outcome.ok(node.getFirstChild().getCookedString().orElseThrow());
        } else {
          return Outcome.err(
              new MalformedException(
                  "Template literals with substitutions are not allowed.", node));
        }
      }
      case ADD -> {
        StringBuilder sb = new StringBuilder();
        for (Node child = node.getFirstChild(); child != null; child = child.getNext()) {
          Outcome<String> childString = extractStringFromStringExprNode(child);
          if (childString.isErr()) {
            return Outcome.err(childString.error());
          }
          sb.append(childString.get());
        }
        return Outcome.ok(sb.toString());
      }
      default -> {
        return Outcome.err(
            new MalformedException("literal string or concatenation expected", node));
      }
    }
  }

  /**
   * Extract data from a call to {@code goog.getMsg(...)}.
   *
   * <p>Here's an example with all possible arguments and options present.
   *
   * <pre><code>
   *   const MSG_HELLO =
   *       goog.getMsg(
   *           'Hello, {$name}', // message template string
   *           { name: getName() }, // placeholder replacement values
   *           { // options bag
   *             html: false,
   *             unescapeHtmlEntities: true,
   *             example: { name: 'George' },
   *             original_code: { name: 'getName()' },
   *           });
   * </code></pre>
   *
   * @return an error outcome with a MalformedException if the code does not match the expected
   *     format.
   */
  private Outcome<JsMessageDefinition> extractJsMessageDefinition(
      Node msgNode, JSDocInfo jsDocInfo, String messageKeyFromLhs, String sourceName) {
    // Extract data from a call to `goog.getMsg(...)`
    // TODO(bradfordcsmith): Add these three fields to the options bag argument for goog.getMsg().
    // Specifying them there instead of in annotations will make them available to the uncompiled
    // `goog.getMsg()` call during runtime, which could enable runtime lookup of translated messages
    // in uncompiled code.
    final Optional<String> description;
    final Optional<String> meaning;
    final Optional<String> alternateMessageId;
    if (jsDocInfo == null) {
      description = Optional.empty();
      meaning = Optional.empty();
      alternateMessageId = Optional.empty();
    } else {
      description = jsDocInfo.getDescription();
      meaning = jsDocInfo.getMeaning();
      alternateMessageId = jsDocInfo.getAlternateMessageId();
    }

    // first child of the call is the `goog.getMsg` name
    // second is the message string value
    final Node msgTextNode = msgNode.getSecondChild();
    if (msgTextNode == null) {
      return Outcome.err(new MalformedException("Message string literal expected", msgNode));
    }
    // third is the optional object literal mapping placeholder names to value expressions
    final Node valuesObjLit = msgTextNode.getNext();
    // fourth is the optional options-bag argument
    final Node optionsBagArgument = valuesObjLit == null ? null : valuesObjLit.getNext();

    final Node unexpectedArgument =
        optionsBagArgument == null ? null : optionsBagArgument.getNext();
    if (unexpectedArgument != null) {
      return Outcome.err(new MalformedException("too many arguments", unexpectedArgument));
    }

    // Extract and parse the message text
    // The message string can be a string literal or a concatenation of string literals.
    // We want the whole thing as a single string here.
    Outcome<String> msgTextStringOutcome = extractStringFromStringExprNode(msgTextNode);
    if (msgTextStringOutcome.isErr()) {
      return Outcome.err(msgTextStringOutcome.error());
    }
    String msgTextString = msgTextStringOutcome.get();
    Outcome<GoogGetMsgParsedText> googGetMsgParsedTextRes =
        extractGoogGetMsgParsedText(msgTextString);
    if (googGetMsgParsedTextRes.isErr()) {
      return Outcome.err(
          new MalformedException(googGetMsgParsedTextRes.errorMessage().orElseThrow(), msgTextNode));
    }
    final GoogGetMsgParsedText googGetMsgParsedText = googGetMsgParsedTextRes.get();

    // Extract the placeholder values object
    // NOTE: The extract method returns an effectively "empty" value for `null`
    Outcome<ObjectLiteralMap> placeholderObjectLiteralMapOutcome =
        extractObjectLiteralMap(valuesObjLit);
    if (placeholderObjectLiteralMapOutcome.isErr()) {
      return Outcome.err(placeholderObjectLiteralMapOutcome.error());
    }
    final ObjectLiteralMap placeholderObjectLiteralMap = placeholderObjectLiteralMapOutcome.get();

    // Confirm that we can find a value map entry for every placeholder name referenced in the text.
    final ImmutableSet<String> placeholderNamesFromText =
        googGetMsgParsedText.getPlaceholderNames();
    Outcome<Void> requiredKeysChecked =
        placeholderObjectLiteralMap.checkForRequiredKeys(
            placeholderNamesFromText,
            placeholderName ->
                new MalformedException(
                    "Unrecognized message placeholder referenced: " + placeholderName,
                    msgTextNode));
    if (requiredKeysChecked.isErr()) {
      return Outcome.err(requiredKeysChecked.error());
    }

    // Confirm that every placeholder name for which we have a value is actually referenced in the
    // text.
    Outcome<Void> unexpectedKeysChecked =
        placeholderObjectLiteralMap.checkForUnexpectedKeys(
            placeholderNamesFromText,
            placeholderName -> "Unused message placeholder: " + placeholderName);
    if (unexpectedKeysChecked.isErr()) {
      return Outcome.err(unexpectedKeysChecked.error());
    }

    // Get a map from placeholder name to Node value
    final ImmutableMap<String, Node> placeholderValuesMap =
        placeholderObjectLiteralMap.extractAsValueMap();

    // Extract the Options bag data
    // NOTE: the extract method returns an "empty" value for `null`
    Outcome<JsMessageOptions> jsMessageOptionsOutcome = extractJsMessageOptions(optionsBagArgument);
    if (jsMessageOptionsOutcome.isErr()) {
      return Outcome.err(jsMessageOptionsOutcome.error());
    }
    final JsMessageOptions jsMessageOptions = jsMessageOptionsOutcome.get();

    // Confirm that any example or orignal_code placeholder information we have refers only to
    // placeholder names that appear in the message text.
    Outcome<Void> unknownPlaceholdersChecked =
        jsMessageOptions.checkForUnknownPlaceholders(placeholderNamesFromText);
    if (unknownPlaceholdersChecked.isErr()) {
      return Outcome.err(unknownPlaceholdersChecked.error());
    }

    final JsMessage jsExtractedMessage =
        buildJsMessage(
            new InputForBuildJsMsg() {
              @Override
              public String getMessageKeyFromLhs() {
                return messageKeyFromLhs;
              }

              @Override
              public String getSourceName() {
                return sourceName;
              }

              @Override
              public String getMessageText() {
                return googGetMsgParsedText.getText();
              }

              @Override
              public ImmutableList<Part> getMessageParts() {
                return googGetMsgParsedText.getParts();
              }

              @Override
              public ImmutableMap<String, String> getPlaceholderExampleMap() {
                return jsMessageOptions.getPlaceholderExampleMap();
              }

              @Override
              public ImmutableMap<String, String> getPlaceholderOriginalCodeMap() {
                return jsMessageOptions.getPlaceholderOriginalCodeMap();
              }

              @Override
              public Optional<String> getDescription() {
                return description;
              }

              @Override
              public Optional<String> getMeaning() {
                return meaning;
              }

              @Override
              public Optional<String> getAlternateMessageId() {
                return alternateMessageId;
              }
            });

    final JsMessageDefinition result = new JsMessageDefinition() {
      @Override
      public JsMessage getMessage() {
        return jsExtractedMessage;
      }

      @Override
      public Node getMessageNode() {
        return msgNode;
      }

      @Override
      public Node getTemplateTextNode() {
        return msgTextNode;
      }

      @Override
      public @Nullable Node getPlaceholderValuesNode() {
        return valuesObjLit;
      }

      @Override
      public ImmutableMap<String, Node> getPlaceholderValueMap() {
        return placeholderValuesMap;
      }

      @Override
      public boolean shouldEscapeLessThan() {
        return jsMessageOptions.isEscapeLessThan();
      }

      @Override
      public boolean shouldUnescapeHtmlEntities() {
        return jsMessageOptions.isUnescapeHtmlEntities();
      }
    };
    return Outcome.ok(result);
  }

  /**
   * Extract message data from a `declareIcuTemplate()` call.
   *
   * <p>Example:
   *
   * <pre><code>
   *   const MSG_ICU_EXAMPLE =
   *       declareIcuTemplate(
   *           `{NUM_PEOPLE, plural, offset:1
   *           =0 {I see {BEGIN_BOLD}no one at all{END_BOLD} in {INTERPOLATION_2}.}
   *           =1 {I see {INTERPOLATION_1} in {INTERPOLATION_2}.}
   *           =2 {I see {INTERPOLATION_1} and one other person in {INTERPOLATION_2}.}
   *           other {I see {INTERPOLATION_1} and some other people in {INTERPOLATION_2}.}
   *       }`,
   *       {
   *         description: 'required message description here',
   *         meaning: 'optional meaning here',
   *         alternate_message_id: 'optional alternate message ID here',
   *         // optional
   *         original_code: {
   *           'INTERPOLATION_1': '{{getPerson()}}',
   *           'INTERPOLATION_2': '{{getPlaceName()}}',
   *         },
   *         // optional
   *         example: {
   *           'INTERPOLATION_1': 'Jane Doe',
   *           'INTERPOLATION_2': 'Paris, France',
   *         }
   *       });
   * </code></pre>
   */
  private Outcome<IcuTemplateDefinition> extractIcuTemplateDefinition(
      Node msgNode, JSDocInfo jsDocInfo, String messageKeyFromLhs, String sourceName) {
    if (jsDocInfo != null) {
      // For declareIcuTemplateData() it is not valid to use the @desc, @meaning, and
      // @alternateMessageId annotations.
      // Instead, that information should go into the options bag parameter.
      if (jsDocInfo.getAlternateMessageId().isPresent()) {
        return Outcome.err(
            new MalformedException(
                "Use the 'alternate_message_id' option, not the '@alternateMessageId' annotation",
                msgNode));
      }
      if (jsDocInfo.getDescription().isPresent()) {
        return Outcome.err(
            new MalformedException(
                "Use the 'description' option, not the '@desc' annotation", msgNode));
      }
      if (jsDocInfo.getMeaning().isPresent()) {
        return Outcome.err(
            new MalformedException(
                "Use the 'meaning' option, not the '@meaning' annotation", msgNode));
      }
    }

    // The first argument is the message string
    final Node stringLiteralExpression = msgNode.getSecondChild();
    if (stringLiteralExpression == null) {
      return Outcome.err(new MalformedException("message string argument expected", msgNode));
    }
    Outcome<IcuMessageTemplateString> icuMessageTemplateStringOutcome =
        extractIcuMessageTemplateString(stringLiteralExpression);
    if (icuMessageTemplateStringOutcome.isErr()) {
      return Outcome.err(icuMessageTemplateStringOutcome.error());
    }
    final IcuMessageTemplateString icuMessageTemplateString =
        icuMessageTemplateStringOutcome.get();

    // The second argument is the options bag
    final Node optionsBagNode = stringLiteralExpression.getNext();
    if (optionsBagNode == null) {
      return Outcome.err(new MalformedException("options argument expected", msgNode));
    }
    Outcome<IcuTemplateOptions> icuTemplateOptionsOutcome =
        extractIcuTemplateOptions(optionsBagNode);
    if (icuTemplateOptionsOutcome.isErr()) {
      return Outcome.err(icuTemplateOptionsOutcome.error());
    }
    final IcuTemplateOptions icuTemplateOptions = icuTemplateOptionsOutcome.get();

    // Parsing of the template string into parts depends on which placeholders are mentioned
    // in the options. Placeholders that do not have example or original_code text  will not be
    // extracted from the template as separate parts.
    final ExtractedIcuTemplateParts extractedIcuTemplateParts =
        icuMessageTemplateString.extractParts(icuTemplateOptions.getPlaceholderNames());
    Outcome<Void> unknownPlaceholdersChecked =
        icuTemplateOptions.checkForUnknownPlaceholders(
            extractedIcuTemplateParts.extractedPlaceholderNames);
    if (unknownPlaceholdersChecked.isErr()) {
      return Outcome.err(unknownPlaceholdersChecked.error());
    }

    // There shouldn't be another argument
    final Node unexpectedArgument = optionsBagNode.getNext();
    if (unexpectedArgument != null) {
      return Outcome.err(new MalformedException("too many arguments", unexpectedArgument));
    }

    final JsMessage extractedMessage =
        buildJsMessage(
            new InputForBuildJsMsg() {
              @Override
              public String getMessageKeyFromLhs() {
                return messageKeyFromLhs;
              }

              @Override
              public String getSourceName() {
                return sourceName;
              }

              @Override
              public String getMessageText() {
                return icuMessageTemplateString.template;
              }

              @Override
              public ImmutableList<Part> getMessageParts() {
                return extractedIcuTemplateParts.extractedParts;
              }

              @Override
              public ImmutableMap<String, String> getPlaceholderExampleMap() {
                return icuTemplateOptions.getPlaceholderExampleMap();
              }

              @Override
              public ImmutableMap<String, String> getPlaceholderOriginalCodeMap() {
                return icuTemplateOptions.getPlaceholderOriginalCodeMap();
              }

              @Override
              public Optional<String> getDescription() {
                return Optional.of(icuTemplateOptions.getDescription());
              }

              @Override
              public Optional<String> getMeaning() {
                return icuTemplateOptions.getMeaning();
              }

              @Override
              public Optional<String> getAlternateMessageId() {
                return icuTemplateOptions.getAlternateMessageId();
              }
            });

    final IcuTemplateDefinition result = new IcuTemplateDefinition() {
      @Override
      public JsMessage getMessage() {
        return extractedMessage;
      }

      @Override
      public Node getMessageNode() {
        return msgNode;
      }

      @Override
      public Node getTemplateTextNode() {
        return stringLiteralExpression;
      }
    };
    return Outcome.ok(result);
  }

  private static final Pattern ICU_PLACEHOLDER_RE = Pattern.compile("\\{([A-Z_0-9]+)\\}");

  @VisibleForTesting
  static final class IcuMessageTemplateString {
    private final String template;

    IcuMessageTemplateString(String template) {
      this.template = template;
    }

    /**
     * Split the template string into plain string parts and placeholders.
     *
     * <p>Although this method will recognize all ICU placeholders in the string ("{NAME}"), it will
     * only extract as separate parts those that are named in {@code placholderNamesToExtract}.
     * Those are the ones for which we will need to create placeholder elements, so we can attach
     * example text or original code snippets to them when we put the message into an XMB file.
     */
    ExtractedIcuTemplateParts extractParts(Set<String> placholderNamesToExtract) {
      ImmutableSet.Builder<String> extractedPlaceholderNames = ImmutableSet.builder();
      ImmutableList.Builder<Part> partsBuilder = ImmutableList.builder();
      final Matcher matcher = ICU_PLACEHOLDER_RE.matcher(template);
      int remainingTemplateStartIndex = 0;
      while (matcher.find()) {
        String placeholderName = matcher.group(1);
        if (placholderNamesToExtract.contains(placeholderName)) {
          extractedPlaceholderNames.add(placeholderName);
          String nonPlaceholderPrefix =
              template.substring(remainingTemplateStartIndex, matcher.start());
          remainingTemplateStartIndex = matcher.end();
          partsBuilder.add(StringPart.create(nonPlaceholderPrefix));
          partsBuilder.add(PlaceholderReference.createForCanonicalName(placeholderName));
        } // else we have no need to create a separate part for this placeholder.
      }
      if (remainingTemplateStartIndex < template.length()) {
        final String remainingTemplate = template.substring(remainingTemplateStartIndex);
        partsBuilder.add(StringPart.create(remainingTemplate));
      }
      return new ExtractedIcuTemplateParts(extractedPlaceholderNames.build(), partsBuilder.build());
    }
  }

  static class ExtractedIcuTemplateParts {
    final ImmutableSet<String> extractedPlaceholderNames;
    final ImmutableList<Part> extractedParts;

    ExtractedIcuTemplateParts(
        ImmutableSet<String> extractedPlaceholderNames, ImmutableList<Part> extractedParts) {
      this.extractedPlaceholderNames = extractedPlaceholderNames;
      this.extractedParts = extractedParts;
    }
  }

  private Outcome<IcuMessageTemplateString> extractIcuMessageTemplateString(
      Node stringExpression) {
    Outcome<String> templateStringOutcome = extractStringFromStringExprNode(stringExpression);
    if (templateStringOutcome.isErr()) {
      return Outcome.err(templateStringOutcome.error());
    }
    final String templateString = templateStringOutcome.get();
    return Outcome.ok(new IcuMessageTemplateString(templateString));
  }

  private interface JsMessageOptions {
    // Replace `'<'` with `'&lt;'` in the message.
    boolean isEscapeLessThan();

    // Replace these escaped entities with their literal characters in the message
    // (Overrides escapeLessThan)
    // '&lt;' -> '<'
    // '&gt;' -> '>'
    // '&apos;' -> "'"
    // '&quot;' -> '"'
    // '&amp;' -> '&'
    boolean isUnescapeHtmlEntities();

    ImmutableMap<String, String> getPlaceholderExampleMap();

    ImmutableMap<String, String> getPlaceholderOriginalCodeMap();

    Outcome<Void> checkForUnknownPlaceholders(Set<String> knownPlaceholders);
  }

  private static final ImmutableSet<String> MESSAGE_OPTION_NAMES =
      ImmutableSet.of("html", "unescapeHtmlEntities", "example", "original_code");

  private Outcome<JsMessageOptions> extractJsMessageOptions(@Nullable Node optionsBag) {
    Outcome<ObjectLiteralMap> objectLiteralMapOutcome = extractObjectLiteralMap(optionsBag);
    if (objectLiteralMapOutcome.isErr()) {
      return Outcome.err(objectLiteralMapOutcome.error());
    }
    ObjectLiteralMap objectLiteralMap = objectLiteralMapOutcome.get();
    Outcome<Void> unexpectedKeysChecked =
        objectLiteralMap.checkForUnexpectedKeys(
            MESSAGE_OPTION_NAMES, optionName -> "Unknown option: " + optionName);
    if (unexpectedKeysChecked.isErr()) {
      return Outcome.err(unexpectedKeysChecked.error());
    }

    Outcome<Boolean> isEscapeLessThanOutcome = objectLiteralMap.getBooleanValueOrFalse("html");
    if (isEscapeLessThanOutcome.isErr()) {
      return Outcome.err(isEscapeLessThanOutcome.error());
    }
    final boolean isEscapeLessThan = isEscapeLessThanOutcome.get();
    Outcome<Boolean> isUnescapeHtmlEntitiesOutcome =
        objectLiteralMap.getBooleanValueOrFalse("unescapeHtmlEntities");
    if (isUnescapeHtmlEntitiesOutcome.isErr()) {
      return Outcome.err(isUnescapeHtmlEntitiesOutcome.error());
    }
    final boolean isUnescapeHtmlEntities = isUnescapeHtmlEntitiesOutcome.get();

    final Node exampleValueNode = objectLiteralMap.getValueNode("example");
    Outcome<ObjectLiteralMap> exampleObjectLiteralMapOutcome =
        extractObjectLiteralMap(exampleValueNode);
    if (exampleObjectLiteralMapOutcome.isErr()) {
      return Outcome.err(exampleObjectLiteralMapOutcome.error());
    }
    final ObjectLiteralMap exampleObjectLiteralMap = exampleObjectLiteralMapOutcome.get();
    Outcome<ImmutableMap<String, String>> placeholderExamplesMapOutcome =
        exampleObjectLiteralMap.extractAsStringToStringMap();
    if (placeholderExamplesMapOutcome.isErr()) {
      return Outcome.err(placeholderExamplesMapOutcome.error());
    }
    final ImmutableMap<String, String> placeholderExamplesMap =
        placeholderExamplesMapOutcome.get();

    final Node originalCodeValueNode = objectLiteralMap.getValueNode("original_code");
    Outcome<ObjectLiteralMap> originalCodeObjectLiteralMapOutcome =
        extractObjectLiteralMap(originalCodeValueNode);
    if (originalCodeObjectLiteralMapOutcome.isErr()) {
      return Outcome.err(originalCodeObjectLiteralMapOutcome.error());
    }
    final ObjectLiteralMap originalCodeObjectLiteralMap =
        originalCodeObjectLiteralMapOutcome.get();
    Outcome<ImmutableMap<String, String>> placeholderOriginalCodeMapOutcome =
        originalCodeObjectLiteralMap.extractAsStringToStringMap();
    if (placeholderOriginalCodeMapOutcome.isErr()) {
      return Outcome.err(placeholderOriginalCodeMapOutcome.error());
    }
    final ImmutableMap<String, String> placeholderOriginalCodeMap =
        placeholderOriginalCodeMapOutcome.get();

    // NOTE: The getX() methods below should all do little to no computation.
    // In particular, all checking for MalformedExceptions must be done before creating this object.
    final JsMessageOptions result = new JsMessageOptions() {
      @Override
      public boolean isEscapeLessThan() {
        return isEscapeLessThan;
      }

      @Override
      public boolean isUnescapeHtmlEntities() {
        return isUnescapeHtmlEntities;
      }

      @Override
      public ImmutableMap<String, String> getPlaceholderExampleMap() {
        return placeholderExamplesMap;
      }

      @Override
      public ImmutableMap<String, String> getPlaceholderOriginalCodeMap() {
        return placeholderOriginalCodeMap;
      }

      @Override
      public Outcome<Void> checkForUnknownPlaceholders(Set<String> knownPlaceholders) {
        Outcome<Void> exampleChecked =
            exampleObjectLiteralMap.checkForUnexpectedKeys(
                knownPlaceholders, unknownName -> "Unknown placeholder: " + unknownName);
        if (exampleChecked.isErr()) {
          return Outcome.err(exampleChecked.error());
        }
        return originalCodeObjectLiteralMap.checkForUnexpectedKeys(
            knownPlaceholders, unknownName -> "Unknown placeholder: " + unknownName);
      }
    };
    return Outcome.ok(result);
  }

  private interface IcuTemplateOptions {
    String getDescription();

    Optional<String> getMeaning();

    Optional<String> getAlternateMessageId();

    ImmutableMap<String, String> getPlaceholderExampleMap();

    ImmutableMap<String, String> getPlaceholderOriginalCodeMap();

    Outcome<Void> checkForUnknownPlaceholders(Set<String> knownPlaceholders);

    /** All the placeholder names mentioned in the options. */
    ImmutableSet<String> getPlaceholderNames();
  }

  /**
   * Property names that are expected to appear in the options bag argument of declareIcuTemplate().
   */
  private static final ImmutableSet<String> ICU_TEMPLATE_OPTION_NAMES =
      ImmutableSet.of("description", "meaning", "alternate_message_id", "example", "original_code");

  /**
   * Extract the options from the second argument to a `declareIcuTemplate()` call
   *
   * <p>Example:
   *
   * <pre><code>
   *   {
   *     description: 'required message description here',
   *     meaning: 'optional meaning here',
   *     alternate_message_id: 'optional alternate message ID here',
   *     // optional
   *     original_code: {
   *       'INTERPOLATION_1': '{{getPerson()}}',
   *       'INTERPOLATION_2': '{{getPlaceName()}}',
   *     },
   *     // optional
   *     example: {
   *       'INTERPOLATION_1': 'Jane Doe',
   *       'INTERPOLATION_2': 'Paris, France',
   *     }
   *   }
   * </code></pre>
   */
  private Outcome<IcuTemplateOptions> extractIcuTemplateOptions(Node optionsBag) {
    Outcome<ObjectLiteralMap> objectLiteralMapOutcome = extractObjectLiteralMap(optionsBag);
    if (objectLiteralMapOutcome.isErr()) {
      return Outcome.err(objectLiteralMapOutcome.error());
    }
    ObjectLiteralMap objectLiteralMap = objectLiteralMapOutcome.get();
    Outcome<Void> unexpectedKeysChecked =
        objectLiteralMap.checkForUnexpectedKeys(
            ICU_TEMPLATE_OPTION_NAMES, optionName -> "Unknown option: " + optionName);
    if (unexpectedKeysChecked.isErr()) {
      return Outcome.err(unexpectedKeysChecked.error());
    }

    // required description string
    final @Nullable Node descriptionNode = objectLiteralMap.getValueNode("description");
    if (descriptionNode == null) {
      return Outcome.err(
          new MalformedException("'description' option field is missing", optionsBag));
    }
    Outcome<String> descriptionOutcome = extractStringFromStringExprNode(descriptionNode);
    if (descriptionOutcome.isErr()) {
      return Outcome.err(descriptionOutcome.error());
    }
    final String description = descriptionOutcome.get();

    // optional meaning string
    final @Nullable Node meaningNode = objectLiteralMap.getValueNode("meaning");
    final Optional<String> meaning;
    if (meaningNode == null) {
      meaning = Optional.empty();
    } else {
      Outcome<String> meaningOutcome = extractStringFromStringExprNode(meaningNode);
      if (meaningOutcome.isErr()) {
        return Outcome.err(meaningOutcome.error());
      }
      meaning = Optional.ofNullable(meaningOutcome.get());
    }

    // optional alternate message ID
    final @Nullable Node alternateMessageIdNode =
        objectLiteralMap.getValueNode("alternate_message_id");
    final Optional<String> alternateMessageId;
    if (alternateMessageIdNode == null) {
      alternateMessageId = Optional.empty();
    } else {
      Outcome<String> alternateMessageIdOutcome =
          extractStringFromStringExprNode(alternateMessageIdNode);
      if (alternateMessageIdOutcome.isErr()) {
        return Outcome.err(alternateMessageIdOutcome.error());
      }
      alternateMessageId = Optional.ofNullable(alternateMessageIdOutcome.get());
    }

    // optional map of placeholder names to example string values
    final Node exampleValueNode = objectLiteralMap.getValueNode("example");
    Outcome<ObjectLiteralMap> exampleObjectLiteralMapOutcome =
        extractObjectLiteralMap(exampleValueNode);
    if (exampleObjectLiteralMapOutcome.isErr()) {
      return Outcome.err(exampleObjectLiteralMapOutcome.error());
    }
    final ObjectLiteralMap exampleObjectLiteralMap = exampleObjectLiteralMapOutcome.get();
    Outcome<ImmutableMap<String, String>> placeholderExamplesMapOutcome =
        exampleObjectLiteralMap.extractAsStringToStringMap();
    if (placeholderExamplesMapOutcome.isErr()) {
      return Outcome.err(placeholderExamplesMapOutcome.error());
    }
    final ImmutableMap<String, String> placeholderExamplesMap =
        placeholderExamplesMapOutcome.get();

    // optional map of placeholder names to original_code string values
    final Node originalCodeValueNode = objectLiteralMap.getValueNode("original_code");
    Outcome<ObjectLiteralMap> originalCodeObjectLiteralMapOutcome =
        extractObjectLiteralMap(originalCodeValueNode);
    if (originalCodeObjectLiteralMapOutcome.isErr()) {
      return Outcome.err(originalCodeObjectLiteralMapOutcome.error());
    }
    final ObjectLiteralMap originalCodeObjectLiteralMap =
        originalCodeObjectLiteralMapOutcome.get();
    Outcome<ImmutableMap<String, String>> placeholderOriginalCodeMapOutcome =
        originalCodeObjectLiteralMap.extractAsStringToStringMap();
    if (placeholderOriginalCodeMapOutcome.isErr()) {
      return Outcome.err(placeholderOriginalCodeMapOutcome.error());
    }
    final ImmutableMap<String, String> placeholderOriginalCodeMap =
        placeholderOriginalCodeMapOutcome.get();

    // All the placeholder names mentioned in the 2 optional maps.
    final ImmutableSet<String> placeholderNames =
        ImmutableSet.<String>builder()
            .addAll(placeholderExamplesMap.keySet())
            .addAll(placeholderOriginalCodeMap.keySet())
            .build();

    for (String placeholderName : placeholderNames) {
      if (!JsMessage.isCanonicalPlaceholderNameFormat(placeholderName)) {
        return Outcome.err(
            new MalformedException(
                String.format("Placeholder not in UPPER_SNAKE_CASE: %s", placeholderName),
                optionsBag));
      }
    }

    // NOTE: The getX() methods below should all do little to no computation.
    // In particular, all checking for MalformedExceptions must be done before creating this object.
    final IcuTemplateOptions result = new IcuTemplateOptions() {
      @Override
      public String getDescription() {
        return description;
      }

      @Override
      public Optional<String> getMeaning() {
        return meaning;
      }

      @Override
      public Optional<String> getAlternateMessageId() {
        return alternateMessageId;
      }

      @Override
      public ImmutableMap<String, String> getPlaceholderExampleMap() {
        return placeholderExamplesMap;
      }

      @Override
      public ImmutableMap<String, String> getPlaceholderOriginalCodeMap() {
        return placeholderOriginalCodeMap;
      }

      @Override
      public ImmutableSet<String> getPlaceholderNames() {
        return placeholderNames;
      }

      @Override
      public Outcome<Void> checkForUnknownPlaceholders(Set<String> knownPlaceholders) {
        Outcome<Void> exampleChecked =
            exampleObjectLiteralMap.checkForUnexpectedKeys(
                knownPlaceholders, unknownName -> "Unknown placeholder: " + unknownName);
        if (exampleChecked.isErr()) {
          return Outcome.err(exampleChecked.error());
        }
        return originalCodeObjectLiteralMap.checkForUnexpectedKeys(
            knownPlaceholders, unknownName -> "Unknown placeholder: " + unknownName);
      }
    };
    return Outcome.ok(result);
  }

  private static Outcome<Boolean> extractBooleanStringKeyValue(@Nullable Node stringKeyNode) {
    if (stringKeyNode == null) {
      return Outcome.ok(false);
    } else {
      final Node valueNode = stringKeyNode.getOnlyChild();
      if (valueNode.isTrue()) {
        return Outcome.ok(true);
      } else if (valueNode.isFalse()) {
        return Outcome.ok(false);
      } else {
        return Outcome.err(
            new MalformedException(
                stringKeyNode.getString() + ": Literal true or false expected", valueNode));
      }
    }
  }

  /**
   * Represents the contents of a object literal Node in the AST.
   *
   * <p>The object literal may not have any computed keys or methods.
   *
   * <p>Use {@link #extractObjectLiteralMap(Node)} to get an instance.
   */
  public interface ObjectLiteralMap {
    Outcome<Boolean> getBooleanValueOrFalse(String key);

    /**
     * Returns a map from object property names the Node values they have in the AST, building the
     * map first, if necessary.
     */
    ImmutableMap<String, Node> extractAsValueMap();

    /**
     * Returns a map from object property names to string values, building it first, if necessary.
     *
     * <p>Returns an error outcome with a MalformedException if any of the values are not actually
     * simple string literals or concatenations of string literals.
     */
    Outcome<ImmutableMap<String, String>> extractAsStringToStringMap();

    /**
     * Get the value node for a key.
     *
     * <p>This method avoids the work of extracting the full value map.
     */
    @Nullable Node getValueNode(String key);

    /**
     * Returns an error outcome with a {@link MalformedException} if the object literal has keys not
     * in the expected set.
     */
    Outcome<Void> checkForUnexpectedKeys(
        Set<String> expectedKeys, Function<String, String> createErrorMessage);

    /**
     * Returns an error outcome with a {@link MalformedException} if the object literal does not
     * have one all the keys in the required set.
     */
    Outcome<Void> checkForRequiredKeys(
        Set<String> requiredKeys, Function<String, MalformedException> createException);
  }

  private static class ObjectLiteralMapImpl implements ObjectLiteralMap {
    private final Map<String, Node> stringToStringKeyMap;
    // The result of extractValueMap(). It will be populated if requested.
    private ImmutableMap<String, Node> valueMap = null;
    // The result of extractStringMap(). It will be populated if requested.
    private ImmutableMap<String, String> stringMap = null;

    private ObjectLiteralMapImpl(Map<String, Node> stringToStringKeyMap) {
      this.stringToStringKeyMap = stringToStringKeyMap;
    }

    @Override
    public @Nullable Node getValueNode(String key) {
      final Node stringKeyNode = stringToStringKeyMap.get(key);
      return stringKeyNode == null ? null : stringKeyNode.getOnlyChild();
    }

    @Override
    public Outcome<Boolean> getBooleanValueOrFalse(String key) {
      final Node stringKeyNode = stringToStringKeyMap.get(key);
      return extractBooleanStringKeyValue(stringKeyNode);
    }

    @Override
    public ImmutableMap<String, Node> extractAsValueMap() {
      if (valueMap == null) {
        ImmutableMap.Builder<String, Node> builder = ImmutableMap.builder();
        for (Entry<String, Node> entry : stringToStringKeyMap.entrySet()) {
          builder.put(entry.getKey(), entry.getValue().getOnlyChild());
        }
        valueMap = builder.buildOrThrow();
      }
      return valueMap;
    }

    @Override
    public Outcome<ImmutableMap<String, String>> extractAsStringToStringMap() {
      if (stringMap == null) {
        ImmutableMap.Builder<String, String> builder = ImmutableMap.builder();
        for (Entry<String, Node> entry : stringToStringKeyMap.entrySet()) {
          Outcome<String> value =
              extractStringFromStringExprNode(entry.getValue().getOnlyChild());
          if (value.isErr()) {
            return Outcome.err(value.error());
          }
          builder.put(entry.getKey(), value.get());
        }
        stringMap = builder.buildOrThrow();
      }
      return Outcome.ok(stringMap);
    }

    @Override
    public Outcome<Void> checkForUnexpectedKeys(
        Set<String> expectedKeys, Function<String, String> createErrorMessage) {
      for (String key : stringToStringKeyMap.keySet()) {
        if (!expectedKeys.contains(key)) {
          return Outcome.err(
              new MalformedException(
                  createErrorMessage.apply(key), stringToStringKeyMap.get(key)));
        }
      }
      return Outcome.ok();
    }

    @Override
    public Outcome<Void> checkForRequiredKeys(
        Set<String> requiredKeys, Function<String, MalformedException> createException) {
      for (String requiredKey : requiredKeys) {
        if (!stringToStringKeyMap.containsKey(requiredKey)) {
          return Outcome.err(createException.apply(requiredKey));
        }
      }
      return Outcome.ok();
    }
  }

  /**
   * Returns an object to represent an object literal Node from the AST.
   *
   * <p>The object literal's members must all be `STRING_KEY` nodes.
   *
   * <p>No duplicate string keys are allowed.
   *
   * <p>A `null` argument is treated as if it were an empty object literal.
   *
   * @return An error outcome with a MalformedException if the Node does not meet the above
   *     requirements.
   */
  public static Outcome<ObjectLiteralMap> extractObjectLiteralMap(@Nullable Node objLit) {
    if (objLit == null) {
      return Outcome.ok(new ObjectLiteralMapImpl(ImmutableMap.of()));
    }
    if (!objLit.isObjectLit()) {
      return Outcome.err(new MalformedException("object literal expected", objLit));
    }
    final LinkedHashMap<String, Node> stringToStringKeyMap = new LinkedHashMap<>();
    for (Node stringKey = objLit.getFirstChild();
        stringKey != null;
        stringKey = stringKey.getNext()) {
      if (!stringKey.isStringKey()) {
        return Outcome.err(new MalformedException("string key expected", stringKey));
      }
      final String key = stringKey.getString();
      if (stringToStringKeyMap.containsKey(key)) {
        return Outcome.err(new MalformedException("duplicate string key: " + key, stringKey));
      }
      stringToStringKeyMap.put(stringKey.getString(), stringKey);
    }
    return Outcome.ok(new ObjectLiteralMapImpl(stringToStringKeyMap));
  }

  public static Outcome<ImmutableList<Part>> parseJsMessageTextIntoParts(String originalMsgText) {
    Outcome<GoogGetMsgParsedText> googGetMsgParsedTextRes =
        extractGoogGetMsgParsedText(originalMsgText);
    if (googGetMsgParsedTextRes.isErr()) {
      return Outcome.err(googGetMsgParsedTextRes.error());
    }
    GoogGetMsgParsedText googGetMsgParsedText = googGetMsgParsedTextRes.get();
    return Outcome.ok(googGetMsgParsedText.getParts());
  }

  private static Outcome<GoogGetMsgParsedText> extractGoogGetMsgParsedText(
      final String originalMsgText) {
    String msgText = originalMsgText;
    ImmutableList.Builder<Part> partsBuilder = ImmutableList.builder();
    ImmutableSet.Builder<String> placeholderNamesBuilder = ImmutableSet.builder();
    while (true) {
      int phBegin = msgText.indexOf(JsMessage.PH_JS_PREFIX);
      if (phBegin < 0) {
        // Just a string literal
        partsBuilder.add(StringPart.create(msgText));
        break;
      } else {
        if (phBegin > 0) {
          // A string literal followed by a placeholder
          partsBuilder.add(StringPart.create(msgText.substring(0, phBegin)));
        }

        // A placeholder. Find where it ends
        int phEnd = msgText.indexOf(JsMessage.PH_JS_SUFFIX, phBegin);
        if (phEnd < 0) {
          return Outcome.err(new PlaceholderFormatException("Placeholder incorrectly formatted"));
        }

        String phName = msgText.substring(phBegin + JsMessage.PH_JS_PREFIX.length(), phEnd);
        if (!JsMessage.isLowerCamelCaseWithNumericSuffixes(phName)) {
          return Outcome.err(
              new PlaceholderFormatException("Placeholder name not in lowerCamelCase: " + phName));
        }
        placeholderNamesBuilder.add(phName);
        partsBuilder.add(PlaceholderReference.createForJsName(phName));
        int nextPos = phEnd + JsMessage.PH_JS_SUFFIX.length();
        if (nextPos < msgText.length()) {
          // Iterate on the rest of the message value
          msgText = msgText.substring(nextPos);
        } else {
          // The message is parsed
          break;
        }
      }
    }
    final ImmutableSet<String> placeholderNames = placeholderNamesBuilder.build();
    final ImmutableList<Part> parts = partsBuilder.build();
    // NOTE: The methods defined below should do little to no computation, and definitely should
    // not throw exceptions.
    GoogGetMsgParsedText parsedText = new GoogGetMsgParsedText() {
      @Override
      public String getText() {
        return originalMsgText;
      }

      @Override
      public ImmutableSet<String> getPlaceholderNames() {
        return placeholderNames;
      }

      @Override
      public ImmutableList<Part> getParts() {
        return parts;
      }
    };
    return Outcome.ok(parsedText);
  }

  private interface GoogGetMsgParsedText {
    String getText();

    ImmutableSet<String> getPlaceholderNames();

    ImmutableList<Part> getParts();
  }

  /** Visit a call to goog.getMsgWithFallback. */
  private void visitFallbackFunctionCall(NodeTraversal t, Node call) {
    // Check to make sure the function call looks like:
    // goog.getMsgWithFallback(MSG_1, MSG_2);
    // or:
    // goog.getMsgWithFallback(some.import.MSG_1, some.import.MSG_2);
    if (!call.hasXChildren(3)
        || !isMessageIdentifier(call.getSecondChild())
        || !isMessageIdentifier(call.getLastChild())) {
      compiler.report(JSError.make(call, BAD_FALLBACK_SYNTAX));
      return;
    }

    Node firstArg = call.getSecondChild();
    JsMessage firstMessage = getJsMessageFromNode(t, firstArg);
    if (firstMessage == null) {
      compiler.report(JSError.make(firstArg, FALLBACK_ARG_ERROR, firstArg.getQualifiedName()));
      return;
    }

    Node secondArg = firstArg.getNext();
    JsMessage secondMessage = getJsMessageFromNode(t, secondArg);
    if (secondMessage == null) {
      compiler.report(JSError.make(secondArg, FALLBACK_ARG_ERROR, secondArg.getQualifiedName()));
      return;
    }

    processMessageFallback(call, firstMessage, secondMessage);
  }

  /**
   * Processes a found JS message that was defined with {@code goog.getMsg()}.
   *
   * <p>Several examples of "standard" processing routines are:
   *
   * <ol>
   *   <li>extract all JS messages
   *   <li>replace JS messages with localized versions for some specific language
   *   <li>check that messages have correct syntax and present in localization bundle
   * </ol>
   */
  protected abstract void processJsMessageDefinition(JsMessageDefinition definition);

  /** Processes a found call to {@code goog.i18n.messages.declareIcuTemplate()} */
  protected abstract void processIcuTemplateDefinition(IcuTemplateDefinition definition);

  /**
   * Processes the goog.getMsgWithFallback primitive. goog.getMsgWithFallback(MSG_1, MSG_2);
   *
   * <p>By default, does nothing.
   */
  void processMessageFallback(Node callNode, JsMessage message1, JsMessage message2) {}

  /** Returns whether the given JS identifier is a valid JS message name. */
  boolean isMessageName(String identifier) {
    return identifier.startsWith(MSG_PREFIX) || isScopedAliasesPrefix(identifier);
  }

  private static boolean isMessageIdentifier(Node node) {
    String qname = node.getQualifiedName();
    return !qname.isEmpty() && qname.contains(MSG_PREFIX);
  }

  /**
   * Extracts a message name (e.g. MSG_FOO) from either a NAME node or a GETPROP node. This should
   * cover all of the following cases:
   *
   * <ol>
   *   <li>a NAME node (e.g. MSG_FOO)
   *   <li>a NAME node which is the product of renaming (e.g. $module$contents$MSG_FOO)
   *   <li>a GETPROP node (e.g. some.import.MSG_FOO)
   * </ol>
   */
  private @Nullable JsMessage getJsMessageFromNode(NodeTraversal t, Node node) {
    String messageName = node.getQualifiedName();
    if (messageName.isEmpty() || !messageName.contains(MSG_PREFIX)) {
      return null;
    }

    String messageKey = messageName.substring(messageName.indexOf(MSG_PREFIX));
    if (isUnnamedMessageName(messageKey)) {
      return getTrackedUnnamedMessage(t, messageName);
    } else {
      return getTrackedNormalMessage(messageKey);
    }
  }

  /** Returns whether the given message name is in the unnamed namespace. */
  private static boolean isUnnamedMessageName(String identifier) {
    return MSG_UNNAMED_PATTERN.matcher(identifier).matches();
  }

  /**
   * Checks a node's type.
   *
   * @return an error outcome with a MalformedException if the node is null or the wrong type
   */
  protected Outcome<Void> checkNode(@Nullable Node node, Token type) {
    if (node == null) {
      return Outcome.err(
          new MalformedException("Expected node type " + type + "; found: null", node));
    }
    if (node.getToken() != type) {
      return Outcome.err(
          new MalformedException(
              "Expected node type " + type + "; found: " + node.getToken(), node));
    }
    return Outcome.ok();
  }

  static class MalformedException extends Exception {
    private static final long serialVersionUID = 1L;

    private final Node node;

    MalformedException(String message, Node node) {
      super(message);
      this.node = node;
    }

    Node getNode() {
      return node;
    }
  }

  private static class MessageLocation {
    private final JsMessage message;
    private final Node messageNode;

    private MessageLocation(JsMessage message, Node messageNode) {
      this.message = message;
      this.messageNode = messageNode;
    }
  }

  public static boolean isScopedAliasesPrefix(String name) {
    return SCOPED_ALIASES_PREFIX_PATTERN.matcher(name).lookingAt();
  }

  public static String removeScopedAliasesPrefix(String name) {
    return SCOPED_ALIASES_PREFIX_PATTERN.matcher(name).replaceFirst("MSG_");
  }
}
