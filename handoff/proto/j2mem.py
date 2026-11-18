# j2mem.py <task> label=bin ...: one run each with JSGO_MEMSTATS; prints objects/bytes
import sys,os,subprocess,json
sys.path.insert(0,"/home/user/jsgo/tools/bench"); import bench
S=os.path.dirname(os.path.abspath(__file__))
task=sys.argv[1]; t=bench.load_tasks([task])[0]
for label,binary in [a.split("=",1) for a in sys.argv[2:]]:
    d=os.path.join(S,"fb",task,label+"_mem"); os.makedirs(d,exist_ok=True); bench.make_workspace(d,"/home/user/jsgo/.cache")
    args=bench.task_jobs(t,d)[0]; env={k:v for k,v in os.environ.items() if k!="JAVA_TOOL_OPTIONS"}; env["LC_ALL"]="C.UTF-8"
    mf=os.path.join(d,"mem.json"); env["JSGO_MEMSTATS"]=mf
    with open(os.path.join(d,"stdout"),"wb") as o, open(os.path.join(d,"stderr"),"wb") as e:
        subprocess.run([binary]+args,cwd=d,env=env,stdin=subprocess.DEVNULL,stdout=o,stderr=e)
    m=json.loads(open(mf).read().strip().splitlines()[-1])
    print(task,label,"objects=%.2fM bytes=%.3fG"%(m.get("mallocs",0)/1e6,m.get("total_alloc",0)/1e9),flush=True)
