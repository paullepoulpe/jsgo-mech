#!/usr/bin/env python3
"""Byte-for-byte comparison of the Java tree's compiler against the stock release.

Compiles a few real projects with both `java -jar <stock jar>` and the classes
built by tools/gate0/build.sh, and compares stdout, stderr, exit status and
every output file. The stock results are cached in .cache/e2e/stock.

  python3 -I tools/gate0/e2e.py [case ...]
  python3 -I tools/gate0/e2e.py --go <binary> [case ...]   # compare the Go translation instead
"""
import base64, glob, hashlib, os, shutil, subprocess, sys, tarfile, urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
CACHE = os.path.join(ROOT, ".cache")
INPUTS = os.path.join(CACHE, "e2e", "inputs")

# npm package -> (version, sha512 integrity of the registry tarball)
PACKAGES = {
    "google-closure-library": ("20230802.0.0", "sha512-h2lBEX2tjWwfR+dhFukGER15vKY+4cA3nRqwKSOsBdyizlOxTAHOWbEYiGnMoom3/SrVeQ5uG0eKsbxQ3wbsxA=="),
    "three": ("0.159.0", "sha512-eCmhlLGbBgucuo4VEA9IO3Qpc7dh8Bd4VKzr7WfW4+8hMcIfoAVi1ev0pJYN9PTTsCslbcKgBwr2wNZ1EvLInA=="),
    "lodash": ("4.17.21", "sha512-v2kDEe57lecTulaDIuNTPy3Ry4gLGJ6Z1O3vE1krgXZNrsQ+LFTGHVxVjcXPs17LhbZVGedAJv8XZ1tvj5FvSg=="),
    "react-dom": ("18.3.1", "sha512-5m4nQKp+rZRb09LNH59GM4BxTh9251/ylbKIbpe7TpGxfJ+9kv6BLkLBXIjjspbgbnIBNqlI23tRnTWT0snUIw=="),
    "rxjs": ("7.8.2", "sha512-dhKf903U/PQZY6boNNtAGdWbG85WAbjT/1xYoZIC7FAY0yWapOBQVsVrDl58W86//e1VpMNBtRV4MaXfdMySFA=="),
    "tslib": ("2.8.1", "sha512-oJFu94HQb+KVduSUQL7wnpmqnfmLsOA/nAh6b6EH0wCEoK0/mPeXU6c3wKDV83MkOuHPRHtSXKKU99IBazS/2w=="),
    "@angular/core": ("16.2.12", "sha512-GLLlDeke/NjroaLYOks0uyzFVo6HyLl7VOm0K1QpLXnYvW63W9Ql/T3yguRZa7tRkOAeFZ3jw+1wnBD4O8MoUA=="),
    "core-js": ("3.50.0", "sha512-BRWgOLKkFeCgRudR6zrs8p9XJZcE14grzKMMssoYrk6krtuEZ7MTKPIY5RzOnqsEKIR9kst7wNzphttraT+Yqw=="),
    "shaka-player": ("5.3.1", "sha512-nlhxDfgQKXfc+ecNQos/l1epXILup8C3n7CrQSm43M47vAY1WDlpIAq448YDXTRHK5vZ/4JGiDfxOVkv7vwegg=="),
    "blockly": ("8.0.5", "sha512-jgr3PHImkFYSC2pNWZ74TmXP1HdajxeRd2DQaAt+9I+XLl9RCWvInqNHcS+blsKBTn6zOIxHK6nLzRATbLrDSw=="),
}


def fetch_inputs():
    """Unpacks every package into .cache/e2e/inputs/node_modules/<name>."""
    for name, (version, integrity) in PACKAGES.items():
        dest = os.path.join(INPUTS, "node_modules", name)
        if os.path.isdir(dest):
            continue
        base = name.split("/")[-1]
        data = urllib.request.urlopen(f"https://registry.npmjs.org/{name}/-/{base}-{version}.tgz").read()
        got = "sha512-" + base64.b64encode(hashlib.sha512(data).digest()).decode()
        if got != integrity:
            sys.exit(f"integrity mismatch for {name}@{version}: {got}")
        tmp = dest + ".tmp"
        shutil.rmtree(tmp, ignore_errors=True)
        os.makedirs(tmp)
        tgz = os.path.join(CACHE, "e2e", "pkg.tgz")
        open(tgz, "wb").write(data)
        with tarfile.open(tgz) as t:
            t.extractall(tmp, filter="data")
        os.rename(os.path.join(tmp, os.listdir(tmp)[0]), dest)
        shutil.rmtree(tmp)


def files(pattern, exclude=()):
    """Sorted paths relative to INPUTS matching a recursive glob."""
    found = sorted(glob.glob(pattern, root_dir=INPUTS, recursive=True))
    return [f for f in found if not any(f.endswith(x) for x in exclude)]


NM = "node_modules/"
GCL = NM + "google-closure-library/"
SHAKA = NM + "shaka-player/"
BLOCKLY = NM + "blockly/"

# name -> (flags, input files, output files to compare)
def cases():
    return {
        # Type checking and diagnostics over the whole Closure Library.
        "checks": (
            ["--checks_only", "--jscomp_warning=checkTypes", "--dependency_mode=SORT_ONLY"],
            files(GCL + "closure/goog/**/*.js", ["_test.js"]) + files(GCL + "third_party/closure/goog/**/*.js", ["_test.js"]),
            [],
        ),
        # SIMPLE over three large untyped bundles, with a source map, on four threads.
        "bundle": (
            ["--compilation_level=SIMPLE", "--num_parallel_threads=4", "--js_output_file=out.js", "--create_source_map=out.js.map"],
            [NM + "three/build/three.js", NM + "lodash/lodash.js", NM + "react-dom/umd/react-dom.development.js"],
            ["out.js", "out.js.map"],
        ),
        # ES modules and CommonJS down-levelled to ES5.
        "downlevel": (
            ["--compilation_level=SIMPLE", "--language_out=ECMASCRIPT5", "--module_resolution=NODE", "--process_common_js_modules",
             "--package_json_entry_names=es2015,module,main", "--dependency_mode=PRUNE",
             "--entry_point=node_modules/@angular/core/fesm2022/core.mjs", "--entry_point=node_modules/core-js/stable/index.js",
             "--js_output_file=out.js"],
            [NM + p + "/package.json" for p in ("@angular/core", "rxjs", "rxjs/operators", "tslib", "core-js")]
            + [NM + "@angular/core/fesm2022/core.mjs"] + files(NM + "rxjs/dist/esm/**/*.js") + [NM + "tslib/tslib.es6.js"]
            + [f for d in ("stable", "es", "web", "modules", "internals") for f in files(NM + f"core-js/{d}/**/*.js")],
            ["out.js"],
        ),
        # ADVANCED with type checking and a source map over typed Closure-style code.
        "advanced": (
            ["--compilation_level=ADVANCED", "--language_out=ECMASCRIPT5", "--dependency_mode=SORT_ONLY", "--jscomp_warning=checkTypes",
             "--jscomp_off=lintChecks", "--jscomp_off=deprecated", "--jscomp_off=missingPolyfill", "--jscomp_off=visibility",
             "--extra_annotation_name=listens", "--extra_annotation_name=exportDoc", "--extra_annotation_name=exportInterface",
             "--generate_exports", "-D", "COMPILED=true", "-D", "goog.ENABLE_DEBUG_LOADER=false", "-D", "goog.DEBUG=false",
             "-D", "goog.asserts.ENABLE_ASSERTS=false", "-D", "shaka.log.MAX_LOG_LEVEL=0", "-D", 'shaka.Player.version="v5.3.1"',
             "--js_output_file=out.js", "--create_source_map=out.js.map"],
            [GCL + "closure/goog/base.js", SHAKA + "dist/locales.js"] + files(SHAKA + "externs/**/*.js", ["mediasession.js"])
            + [f for d in ("lib", "ui", "third_party") for f in files(SHAKA + d + "/**/*.js")]
            + [f for d in ("core", "blocks", "generators") for f in files(BLOCKLY + d + "/**/*.js")],
            ["out.js", "out.js.map"],
        ),
    }


def run(argv, name, flags, inputs, outdir):
    """Runs one compile in a fresh directory that sees the inputs as node_modules/."""
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir)
    os.symlink(os.path.join(INPUTS, "node_modules"), os.path.join(outdir, "node_modules"))
    flagfile = os.path.join(outdir, "flags.txt")
    with open(flagfile, "w") as f:
        for a in flags + ["--js=" + i for i in inputs]:
            f.write('"' + a.replace("\\", "\\\\").replace('"', '\\"') + '"\n')
    env = {k: v for k, v in os.environ.items() if k not in ("JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVA_OPTIONS")}
    env["LC_ALL"] = "C.UTF-8"
    with open(os.path.join(outdir, "stdout"), "wb") as o, open(os.path.join(outdir, "stderr"), "wb") as e:
        p = subprocess.run(argv + ["--flagfile=flags.txt"], cwd=outdir, env=env, stdout=o, stderr=e)
    open(os.path.join(outdir, "exit"), "w").write(str(p.returncode))


def main():
    os.makedirs(INPUTS, exist_ok=True)
    fetch_inputs()
    stock = ["java", "-Xss8m", "-jar", os.path.join(CACHE, "closure-unshaded.jar")]
    built = ["java", "-Xss8m", "-cp", os.path.join(CACHE, "build", "main") + ":" + os.path.join(CACHE, "build", "stock-libs.jar"),
             "com.google.javascript.jscomp.CommandLineRunner"]
    args = sys.argv[1:]
    if args[:1] == ["--go"]:
        built = [os.path.abspath(args[1])]
        args = args[2:]
    all_cases = cases()
    bad = 0
    for name in args or all_cases:
        flags, inputs, outputs = all_cases[name]
        sdir = os.path.join(CACHE, "e2e", "stock", name)
        if not os.path.exists(os.path.join(sdir, "exit")):
            run(stock, name, flags, inputs, sdir)
        bdir = os.path.join(CACHE, "e2e", "built", name)
        run(built, name, flags, inputs, bdir)
        diffs = []
        for f in ["stdout", "stderr", "exit"] + outputs:
            a, b = os.path.join(sdir, f), os.path.join(bdir, f)
            if not (os.path.exists(a) and os.path.exists(b)) or open(a, "rb").read() != open(b, "rb").read():
                diffs.append(f)
        size = sum(os.path.getsize(os.path.join(sdir, f)) for f in outputs if os.path.exists(os.path.join(sdir, f)))
        print(f"{name}: {'DIFF in ' + ', '.join(diffs) if diffs else 'MATCH'} "
              f"({len(inputs)} inputs, exit {open(os.path.join(sdir, 'exit')).read()}, {size} output bytes, "
              f"{os.path.getsize(os.path.join(sdir, 'stderr'))} stderr bytes)")
        bad += bool(diffs)
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
