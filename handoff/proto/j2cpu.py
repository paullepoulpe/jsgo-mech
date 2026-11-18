# j2prof.py <task> label=bin ...: one run each with JSGO_CPU_PROFILE=<fb/task/label_prof/cpu.prof>
import sys,os,subprocess
sys.path.insert(0,"/home/user/jsgo/tools/bench"); import bench
S=os.path.dirname(os.path.abspath(__file__))
task=sys.argv[1]; t=bench.load_tasks([task])[0]
for label,binary in [a.split("=",1) for a in sys.argv[2:]]:
    d=os.path.join(S,"fb",task,label+"_prof"); os.makedirs(d,exist_ok=True); bench.make_workspace(d,"/home/user/jsgo/.cache")
    args=bench.task_jobs(t,d)[0]; env={k:v for k,v in os.environ.items() if k!="JAVA_TOOL_OPTIONS"}; env["LC_ALL"]="C.UTF-8"
    env["JSGO_CPU_PROFILE"]=os.path.join(d,"cpu.prof")
    with open(os.path.join(d,"stdout"),"wb") as o, open(os.path.join(d,"stderr"),"wb") as e:
        subprocess.run([binary]+args,cwd=d,env=env,stdin=subprocess.DEVNULL,stdout=o,stderr=e)
    print(task,label,os.path.join(d,"cpu.prof"),flush=True)
