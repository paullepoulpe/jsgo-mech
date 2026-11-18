import re,subprocess
from jmap import *
out=subprocess.run("grep -rnE 'boolean equals\\((final )?(@Nullable )?Object ' /home/user/jm-merge/java/src --include=*.java",shell=True,capture_output=True,text=True).stdout.strip().split('\n')
kept=[];gone=[]
for l in out:
    p,ln,_=l.split(':',2); pos=p[len(J):]+':'+ln
    c,m=where(pos)
    cs=[]
    parts=c.split('.')
    for n in range(len(parts)):
        k='.'.join(parts[n:])+'.equals'
        if k in idx: cs=idx[k];break
    (kept if cs else gone).append((pos,c,[(x[0],x[1],x[2]) for x in cs]))
print('java equals(Object):',len(out),'jsgo kept:',len(kept),'gone:',len(gone))
for g in gone: print('GONE',g[0],g[1])
for k in kept: print('KEPT',k[0],k[1],k[2][0][0],k[2][0][1],k[2][0][2][:90])
