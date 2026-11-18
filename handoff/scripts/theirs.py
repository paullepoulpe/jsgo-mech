# theirs.py: resolve every conflict hunk of the unmerged files to the cherry-picked side
import re,subprocess,os
os.chdir('/home/user/jm-merge')
for f in subprocess.run(['git','diff','--name-only','--diff-filter=U'],capture_output=True,text=True).stdout.split():
    s=open(f).read()
    s2,k=re.subn(r'<<<<<<< HEAD\n(?:.*?\n)?=======\n(.*?)>>>>>>> [0-9a-f]{7}[^\n]*\n', lambda m: m.group(1), s, flags=re.S)
    assert '<<<<<<<' not in s2, f
    open(f,'w').write(s2); subprocess.run(['git','add',f]); print("theirs",f,k)
