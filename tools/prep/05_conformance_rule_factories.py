#!/usr/bin/env python3
"""Prep step 05 (InvalidRequirementSpec): rule constructors -> init methods + static factories.

Run from the repo root; rewrites java/src/com/google/javascript/jscomp/ConformanceRules.java.
Idempotent (a second run finds no constructor left to rewrite).

Rule, applied to every nested class C of ConformanceRules that has a constructor
    C(AbstractCompiler compiler, Requirement requirement) throws InvalidRequirementSpec { BODY }
(all subclasses of AbstractRule, and AbstractRule itself):
  * the constructor becomes the method
        Outcome<Void> initC(AbstractCompiler compiler, Requirement requirement) { BODY' }
    (same modifiers) where BODY' is BODY with
      - `super(compiler, requirement);` replaced by a call of the superclass's init method whose
        error outcome is propagated,
      - `throw new InvalidRequirementSpec(..);` replaced by
        `return Outcome.err(new InvalidRequirementSpec(..));`,
      - `return Outcome.ok();` appended;
    C is left with the implicit no-argument constructor;
  * a non-abstract C gets, in front of initC,
        static Outcome<Rule> create(AbstractCompiler compiler, Requirement requirement)
    which does `new C()`, calls initC and returns the rule or initC's error;
  * the instance fields of C that the constructor assigned (`final T f;` without initializer)
    lose `final`;
  * `new C(compiler, requirement)` and `C::new` are NOT rewritten here (CheckConformance.initRule
    and the custom rule registry are edited by hand), nor are calls of other methods that throw
    InvalidRequirementSpec inside BODY.
"""
import re

PATH = "java/src/com/google/javascript/jscomp/ConformanceRules.java"
CTOR = re.compile(
    r"^    (?P<mods>(?:public |private |protected )?)(?P<name>[A-Z]\w*)"
    r"\(AbstractCompiler compiler, Requirement requirement\)\s*"
    r"throws InvalidRequirementSpec \{\n",
    re.M,
)


def main():
    s = open(PATH).read()
    count = 0
    pos = 0
    while True:
        m = CTOR.search(s, pos)
        if not m:
            break
        name = m.group("name")
        # The class header: the closest preceding "class NAME" at nesting depth 1.
        heads = list(re.finditer(r"^  (?P<mods>[a-z ]*)class %s\b(?P<rest>[^{]*)\{" % name, s[: m.start()], re.M))
        head = heads[-1]
        abstract = "abstract" in head.group("mods")
        ext = re.search(r"extends (\w+)", head.group("rest"))
        end = s.index("\n    }\n", m.end()) + 1  # start of the line "    }"
        body = s[m.end() : end]
        if ext:
            sup = ext.group(1)
            call = "      super(compiler, requirement);\n"
            assert body.count(call) == 1, name
            body = body.replace(
                call,
                "      Outcome<Void> superInited = init%s(compiler, requirement);\n"
                "      if (superInited.isErr()) {\n"
                "        return Outcome.err(superInited.error());\n"
                "      }\n" % sup,
            )
        body = re.sub(
            r"throw new InvalidRequirementSpec\(((?:[^;]|\n)*?)\);",
            lambda t: "return Outcome.err(new InvalidRequirementSpec(" + t.group(1) + "));",
            body,
        )
        # A rewritten throw that no longer fits in 100 columns is wrapped after "Outcome.err(".
        body = re.sub(
            r"^( *)return Outcome\.err\((new InvalidRequirementSpec\(.*\))\);$",
            lambda t: t.group(0)
            if len(t.group(0)) <= 100
            else "%sreturn Outcome.err(\n%s    %s);" % (t.group(1), t.group(1), t.group(2)),
            body,
            flags=re.M,
        )
        body += "      return Outcome.ok();\n"
        factory = ""
        if not abstract:
            factory = (
                "    static Outcome<Rule> create(AbstractCompiler compiler, Requirement requirement) {\n"
                "      %s rule = new %s();\n"
                "      Outcome<Void> inited = rule.init%s(compiler, requirement);\n"
                "      if (inited.isErr()) {\n"
                "        return Outcome.err(inited.error());\n"
                "      }\n"
                "      return Outcome.ok(rule);\n"
                "    }\n\n" % (name, name, name)
            )
        method = "    %sOutcome<Void> init%s(AbstractCompiler compiler, Requirement requirement) {\n" % (
            m.group("mods"),
            name,
        )
        if len(method) > 101:
            method = "    %sOutcome<Void> init%s(\n        AbstractCompiler compiler, Requirement requirement) {\n" % (
                m.group("mods"),
                name,
            )
        # Fields of this class (depth-2 declarations between the class header and the constructor).
        fields = s[head.end() : m.start()]
        fields = re.sub(r"^(    (?:private |protected |public )?)final ([^=;()]+;)$", r"\1\2", fields, flags=re.M)
        s = s[: head.end()] + fields + factory + method + body + s[end:]
        pos = head.end() + len(fields) + len(factory) + len(method) + len(body)
        count += 1
    open(PATH, "w").write(s)
    print("constructors rewritten:", count)


if __name__ == "__main__":
    main()
