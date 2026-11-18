#!/bin/bash
# usage: cap2.sh <maindir> <libsjar> <outdir>
main=$1; libs=$2; out=$3; rm -rf $out; mkdir -p $out/logs $out/reports
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
deps=$(ls /home/user/work/p13/.cache/deps/*.jar | tr '\n' ':')
cp="$main:$libs:$deps"
J="env -u JAVA_TOOL_OPTIONS java"
cd $S/in
$J -cp "$cp" com.google.javascript.jscomp.CommandLineRunner --json_streams=BOTH --create_source_map=%outname%.map < stream.json > $out/stream.out 2> $out/stream.err
$J -cp "$cp" com.google.javascript.jscomp.CommandLineRunner --json_streams=OUT --js a.js --js "b=<&'.js" -O ADVANCED --chunk "m1:1" --chunk "m2:1:m1" --chunk_output_path_prefix "pre<" > $out/stream2.out 2> $out/stream2.err
for bad in '[null]' '[{"src":{}}]' '[{"src":1,"src":true,"path":null}]' '[1]' '{}' '' '[{"src":"a"},'; do echo "$bad" | $J -cp "$cp" com.google.javascript.jscomp.CommandLineRunner --json_streams=BOTH >> $out/streambad.out 2>&1; echo "exit=$?" >> $out/streambad.out; done
$J -cp "$cp" com.google.javascript.jscomp.MapDiagnosticToSuppression --output=json > $out/mapdiag.out 2>&1
$J -cp "$cp" com.google.javascript.jscomp.CommandLineRunner --js a.js --js "b=<&'.js" --instrument_for_coverage_option=PRODUCTION --instrument_mapping_report $out/mapping.txt --production_instrumentation_array_name=ist_arr --externs ext.js > $out/instr.out 2> $out/instr.err
ids=$(grep -v '^ ' $out/mapping.txt | cut -d: -f1 | head -3)
i=1; j="{"; for id in $ids; do j="$j\"$id\": $i,"; i=$((i+5)); done; echo "${j%,}}" > $out/reports/r1.json
echo "{}" > $out/reports/r2.json
$J -cp "$cp" com.google.javascript.jscomp.instrumentation.reporter.ProductionInstrumentationReporter --mapping_file $out/mapping.txt --reports_directory $out/reports --result_output $out/report.txt > $out/rep.out 2>&1
mkdir -p $out/drvc && javac -nowarn -cp "$cp" -d $out/drvc $S/drv/Drv.java 2>&1 | grep -v "^Note\|Picked up"
$J -cp "$out/drvc:$cp" Drv $out/logs a.js "b=<&'.js" > $out/drv.out 2> $out/drv.err
rm -rf $out/drvc
