#!/bin/bash
# usage: cap.sh <maindir> <libsjar> <outdir>
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd $S/in; main=$1; libs=$2; out=$3; mkdir -p $out
deps=$(ls /home/user/work/p13/.cache/deps/*.jar | tr '\n' ':')
run() { name=$1; shift; env -u JAVA_TOOL_OPTIONS java -cp "$main:$libs:$deps" "$@" >$out/$name.out 2>$out/$name.err </dev/null; echo "exit=$?" >>$out/$name.err; }
C=com.google.javascript.jscomp.CommandLineRunner
run help $C --help
run unknown $C --no_such_flag
run missingarg $C --js_output_file
run badbool $C --debug=maybe
run badbool2 $C --debug maybe
run badenum $C --formatting=NOPE
run badenum2 $C --compilation_level NOPE
run badint $C --summary_detail_level=x
run version $C --version
run helpmd $C --help --markdown-help
run jscomperr $C --jscomp_error
run posdash $C -x
run short $C -O NOPE
run dupbool $C --debug --debug=false --help
run multi $C --js a.js --js=b.js extra.js --help
run lint com.google.javascript.jscomp.LinterMain --nope
run lint2 com.google.javascript.jscomp.LinterMain --fix
run mapdiag com.google.javascript.jscomp.MapDiagnosticToSuppression --nope
run rep com.google.javascript.jscomp.instrumentation.reporter.ProductionInstrumentationReporter --nope
run helpmd2 $C --help --help_markdown
run helpvals $C --compilation_level ADVANCED --formatting PRETTY_PRINT --formatting=SINGLE_QUOTES --externs a.js --externs b.js --define A=1 -D B --js x.js --jscomp_error '*' --browser_resolver_prefix_replacements a=b --browser_resolver_prefix_replacements c= --polymer_version 2 --debug --strict_mode_input=false --parse_inline_source_maps false --env CUSTOM -W VERBOSE --charset "" --js_output_file "a b" --help
run atmissing $C @/nonexistent/file
run atfile $C @$S/in/at.txt
run atfile2 $C @$S/in/at2.txt
run mapbad $C --browser_resolver_prefix_replacements x
run mapbad2 $C --browser_resolver_prefix_replacements =x
run mapmissing $C --browser_resolver_prefix_replacements
run polybad $C --polymer_version x
run intbad2 $C --num_parallel_threads 1.5
run omissing $C -O
run dashonly $C -
run spacetok $C "--js $S/in/a.js"
run spacetok2 $C "--compilation_level BAD"
run spaceeq $C "--nope x=y"
run dashalias $C --checks-only=false --help
run dashalias2 $C --checks-only=maybe
run dashaliasenum $C --dev-mode=x
run boolfile $C --debug true --js $S/in/a.js
run boolfile2 $C --debug TRUE --checks_only yes $S/in/a.js
run boolfile3 $C --debug=0 --checks_only off -- $S/in/a.js
run quoted $C "--js='$S/in/a.js'" --jscomp_off="'checkTypes'" --jscomp_warning '"checkVars"'
run flagfile $C --flagfile $S/in/flags.txt
run flagfile2 $C --flagfile $S/in/flags2.txt
run flagfile3 $C --flagfile $S/in/flags3.txt
run flagfilemissing $C --flagfile /nonexistent/ff
run enumdash $C --json_streams none --dependency_mode prune-legacy --entry_point x --help
run enumlist $C --formatting pretty-print --formatting BAD
run envbad $C --env=node
run instrbad $C --instrument_for_coverage_option NOPE
run pipebad $C --source_map_location_mapping nopipe
run noinputs $C --js 'nonexistent*.js' zzz
run lint3 com.google.javascript.jscomp.LinterMain --resolution_mode=NOPE
run lint4 com.google.javascript.jscomp.LinterMain --fix --resolution_mode NOPE
run lint5 com.google.javascript.jscomp.LinterMain --fix true
run lint6 com.google.javascript.jscomp.LinterMain --resolution_mode
run lint7 com.google.javascript.jscomp.LinterMain --resolution_mode node $S/in/a.js
run lint8 com.google.javascript.jscomp.LinterMain x --fix=NOPE
run mapdiag2 com.google.javascript.jscomp.MapDiagnosticToSuppression extra
run mapdiag3 com.google.javascript.jscomp.MapDiagnosticToSuppression --output=bad
run mapdiag4 com.google.javascript.jscomp.MapDiagnosticToSuppression --output md
run mapdiag5 com.google.javascript.jscomp.MapDiagnosticToSuppression --help
run mapdiag6 com.google.javascript.jscomp.MapDiagnosticToSuppression --output
run rep3 com.google.javascript.jscomp.instrumentation.reporter.ProductionInstrumentationReporter --mapping_file a --result_output b
run rep4 com.google.javascript.jscomp.instrumentation.reporter.ProductionInstrumentationReporter --reports_directory a extra
run rep2 com.google.javascript.jscomp.instrumentation.reporter.ProductionInstrumentationReporter
