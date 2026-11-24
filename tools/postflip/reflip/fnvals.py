# fnvals.py: the conversion functions (casts, boxings) used as values, not called, in g_*.go of the current directory; census.py counts
# only their calls (name followed by "("), so a step that passes a cast as a function value (etaconv) moves the census rows by spelling.
# usage: cd go/closure && python3 ../../tools/postflip/reflip/fnvals.py
import re,glob
t="".join(open(f).read() for f in glob.glob("g_*.go"))
pats={"icast_X":r"\bicast_\w+\b(?!\(|\[)","to_X":r"\bto_[A-Z]\w*\b(?!\()","cast_[T]":r"\bcast_\[[^\]]+\](?!\()","obj_[T]/strobj_":r"\bobj_\[[^\]]+\](?!\()|\bstrobj_\b(?!\()","(T).box_/.self":r"\)\.(box_|self)\b(?!\()","str_/Node_of_":r"\bstr_\b(?!\()|\bNode_of_\b(?!\()"}
print(" ".join(f"{k}={len(re.findall(p,t))}" for k,p in pats.items()))
