# fb.py <task> <rounds> label=bin ...: rounds alternate forward and backward order; per-run lines to stderr
import sys,os,subprocess,time,json
sys.path.insert(0,"/home/user/jsgo/tools/bench"); import bench
S=os.path.dirname(os.path.abspath(__file__))
task,rounds=sys.argv[1],int(sys.argv[2]); bins=[a.split("=",1) for a in sys.argv[3:]]
t=bench.load_tasks([task])[0]
for r in range(rounds):
    seq=bins if r%2==0 else bins[::-1]
    for label,binary in seq:
        d=os.path.join(S,"fb",task,label); os.makedirs(d,exist_ok=True); bench.make_workspace(d,"/home/user/jsgo/.cache")
        args=bench.task_jobs(t,d)[0]; env={k:v for k,v in os.environ.items() if k!="JAVA_TOOL_OPTIONS"}; env["LC_ALL"]="C.UTF-8"
        argv=["/bin/sh","-c",'ulimit -v 9000000; export GOMEMLIMIT=6GiB; exec timeout 900 "$@"',"x",binary]+args
        t0=time.time()
        with open(os.path.join(d,"stdout"),"wb") as o, open(os.path.join(d,"stderr"),"wb") as e:
            p=subprocess.Popen(argv,cwd=d,env=env,stdin=subprocess.DEVNULL,stdout=o,stderr=e); _,st,ru=os.wait4(p.pid,0)
        print(f"  {task} {label} wall={time.time()-t0:.2f} cpu={ru.ru_utime+ru.ru_stime:.2f} round={r} load={os.getloadavg()[0]:.2f}",file=sys.stderr,flush=True)
