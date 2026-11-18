# nsmem.py <task> label=bin ...: one run each with JSGO_MEMSTATS and GODEBUG=gctrace=1; prints
# mallocs, bytes, GCs, GC CPU (sum of the gctrace cpu fields) and the run's CPU time.
import sys,os,subprocess,json,re,time
sys.path.insert(0,"/home/user/jsgo/tools/bench"); import bench
S=os.path.dirname(os.path.abspath(__file__))
task=sys.argv[1]; t=bench.load_tasks([task])[0]
for label,binary in [a.split("=",1) for a in sys.argv[2:]]:
    d=os.path.join(S,"fb",task,label+"_mem"); os.makedirs(d,exist_ok=True); bench.make_workspace(d,"/home/user/jsgo/.cache")
    args=bench.task_jobs(t,d)[0]; env={k:v for k,v in os.environ.items() if k!="JAVA_TOOL_OPTIONS"}; env["LC_ALL"]="C.UTF-8"
    mf=os.path.join(d,"mem.json"); env["JSGO_MEMSTATS"]=mf; env["GODEBUG"]="gctrace=1"
    t0=time.time()
    with open(os.path.join(d,"stdout"),"wb") as o, open(os.path.join(d,"stderr"),"wb") as e:
        p=subprocess.Popen([binary]+args,cwd=d,env=env,stdin=subprocess.DEVNULL,stdout=o,stderr=e); _,st,ru=os.wait4(p.pid,0)
    wall=time.time()-t0
    m=json.loads(open(mf).read().strip().splitlines()[-1])
    gc=0.0; n=0
    for l in open(os.path.join(d,"stderr"),errors="replace"):
        mm=re.search(r"ms clock, ([0-9.]+)\+([0-9.]+)/([0-9.]+)/([0-9.]+)\+([0-9.]+) ms cpu",l)
        if mm: gc+=sum(float(x) for x in mm.groups()); n+=1
    print(f"{task} {label} mallocs={m['mallocs']/1e6:.2f}M bytes={m['total_alloc']/1e9:.3f}G gcs={m['num_gc']} gc_cpu={gc/1000:.2f}s wall={wall:.2f} cpu={ru.ru_utime+ru.ru_stime:.2f} maxrss={ru.ru_maxrss/1024:.0f}M",flush=True)
