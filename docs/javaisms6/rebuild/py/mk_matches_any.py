# matches_any.tsv: census.py's two `any` regexes, dumped with offsets (cwd = go/closure).
# Verbatim from the JAVAISMS6 session (it ran as an inline heredoc; not checked in).
import re,glob
for f in sorted(glob.glob("g_*.go")):
    t=open(f).read()
    for m in re.finditer(r"^\s+\w+\s+any\b",t,re.M):
        k="ret" if re.match(r"\s+return\s+any\b",m.group(0)) else "slot"
        off=m.end()-3
        print(f"{f}\t{off}\t{k}\t{t.count(chr(10),0,m.start())+1+ (1 if m.group(0).startswith(chr(10)) else 0)}\t{m.group(0).strip()}")
    for m in re.finditer(r"\b\w+ any[,)]",t):
        ls=t.rfind("\n",0,m.start())+1; pre=t[ls:m.start()]; line=t[ls:t.find("\n",m.start())]
        st=[]
        for ch in pre:
            if ch in '([': st.append(ch)
            elif ch in ')]' and st: st.pop()
        if st and st[-1]=='[': k="tparam"
        elif re.match(r"func (icast_|to_|gcast_|is_)\w*(\[[^]]*\])?\(\w+ any\)",line): k="helper"
        else: k="slot"
        print(f"{f}\t{m.start()}\t{k}\t{t.count(chr(10),0,m.start())+1}\t{m.group(0)}")
