cd /home/user/jm-merge/tools/postflip
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
run() { n=$(echo "$*" | tr ' ' '_'); timeout 900 go run ./$1 -n "${@:2}" ../../go/closure > $S/sweep/$n.txt 2>&1; echo "== $*"; tail -4 $S/sweep/$n.txt | cut -c1-160; }
export -f run
