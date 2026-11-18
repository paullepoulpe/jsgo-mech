# enumfix.py: in g_*.go files changed against HEAD, *E -> E for each enum the enums step made an integer type
import re,glob,subprocess,os
os.chdir('/home/user/jm-merge/go/closure')
enums=set()
for f in glob.glob('g_*.go'):
    for m in re.finditer(r'^type (\w+) uint8$', open(f).read(), re.M): enums.add(m.group(1))
changed=subprocess.run(['git','diff','--name-only','HEAD','--','.'],capture_output=True,text=True).stdout.split()
rx=re.compile(r'\*(' + '|'.join(sorted(enums,key=len,reverse=True)) + r')\b')
for f in changed:
    f=os.path.basename(f)
    if not f.startswith('g_') or not os.path.exists(f): continue
    t=open(f).read(); t2=rx.sub(r'\1',t)
    if t2!=t: open(f,'w').write(t2); print("enumfix",f,len(rx.findall(t)))
