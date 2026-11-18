#!/usr/bin/env python3
# usage: m.py <runs> <tasks,comma> <label=binary> [label=binary ...]   (interleaved; medians)
# Each run: wall, user+sys CPU, peak RSS, mallocs/bytes (JSGO_MEMSTATS), sha of out.js.
import sys, os, json, subprocess, time, hashlib, statistics
sys.path.insert(0, "/home/user/jsgo/tools/bench")
import bench
S = '/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/proto'
runs = int(sys.argv[1]); tasks = sys.argv[2].split(","); bins = [a.split("=", 1) for a in sys.argv[3:]]
res = {}
for task in tasks:
    t = bench.load_tasks([task])[0]
    for r in range(runs):
        for label, binary in bins:
            d = os.path.join(S, "m", task, label)
            os.makedirs(d, exist_ok=True)
            bench.make_workspace(d, "/home/user/jsgo/.cache")
            args = bench.task_jobs(t, d)[0]
            env = {k: v for k, v in os.environ.items() if k != "JAVA_TOOL_OPTIONS"}
            env["LC_ALL"] = "C.UTF-8"
            ms = os.path.join(d, "memstats.json")
            env["JSGO_MEMSTATS"] = ms
            if os.path.exists(ms): os.remove(ms)
            argv = ["/bin/sh", "-c", 'ulimit -v 9000000; export GOMEMLIMIT=6GiB; exec timeout 900 "$@"', "x", binary] + args
            t0 = time.time()
            with open(os.path.join(d, "stdout"), "wb") as o, open(os.path.join(d, "stderr"), "wb") as e:
                p = subprocess.Popen(argv, cwd=d, env=env, stdin=subprocess.DEVNULL, stdout=o, stderr=e)
                _, st, ru = os.wait4(p.pid, 0)
            wall = time.time() - t0
            h = hashlib.sha256()
            for f in ("out.js", "stdout", "stderr"):
                fp = os.path.join(d, f)
                if os.path.exists(fp): h.update(open(fp, "rb").read())
            m = json.load(open(ms)) if os.path.exists(ms) else {"mallocs": 0, "total_alloc": 0, "num_gc": 0}
            res.setdefault((task, label), []).append((wall, ru.ru_utime + ru.ru_stime, ru.ru_maxrss / 1024, m.get("mallocs",0), m.get("total_alloc",0), h.hexdigest()[:10], os.waitstatus_to_exitcode(st), m.get("num_gc",0)))
            print(f"  {task} {label} wall={wall:.2f} cpu={ru.ru_utime+ru.ru_stime:.2f} load={os.getloadavg()[0]:.2f}", file=sys.stderr, flush=True)
for (task, label), v in res.items():
    md = lambda i: statistics.median(x[i] for x in v)
    print(f"{task:10s} {label:12s} wall={md(0):6.2f}s cpu={md(1):6.2f}s rss={md(2):5.0f}M mallocs={md(3)/1e6:7.1f}M bytes={md(4)/1e9:6.2f}G gc={md(7):.0f} out={','.join(sorted(set(x[5] for x in v)))} exit={sorted(set(x[6] for x in v))}")
