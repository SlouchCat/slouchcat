import json,glob
from collections import Counter
from lib import *
books=json.load(open('books.json'))
def ldur(b): return b['dur_ffprobe'] if b['dur_ffprobe']>0 else b['dur_chapters']
best=json.load(open('audible_best.json'))
# audnexus
n=ok=0; f=Counter(); dd=[]
for i,a in best.items():
    n+=1
    p=f'raw/audnexus/{int(i):03d}_1.json'
    try: c=p_audnexus(open(p).read())
    except Exception: c=[]
    if not c: continue
    ok+=1; c=c[0]
    for k in ['desc','series','publisher','isbn','cover','genres']:
        if c.get(k): f[k]+=1
    ad=[x for x in p_audible(open(glob.glob(f'raw/audible/{int(i):03d}_*')[-1]).read()) if x['id']==a]
    if not ad: ad=[x for f2 in glob.glob(f'raw/audible/{int(i):03d}_*') for x in p_audible(open(f2).read()) if x['id']==a]
    if ad and ad[0]['dur_s'] and c['dur_s']: dd.append(abs(ad[0]['dur_s']-c['dur_s']))
print('audnexus',n,ok,dict(f),'dur identical to audible', sum(1 for d in dd if d<=60),'of',len(dd))
# tolerance sensitivity (audible, lenient and strict) + merged duplicates
def audible_hits(i,b):
    cs=[]
    for f2 in sorted(glob.glob(f'raw/audible/{i:03d}_*')): cs+=p_audible(open(f2).read())
    seen=set();u=[]
    for c in cs:
        if c['id'] not in seen: seen.add(c['id']);u.append(c)
    return hits_for('a',b,u)
for tol in (0.01,0.02,0.03,0.05,0.10):
    strict=lenient=merged=0
    for i,b in enumerate(books):
        hs=audible_hits(i,b); L=ldur(b)
        ok=[h for h in hs if dur_ok(L,h['dur_s'],tol)]
        if len(hs)==1 and len(ok)==1: strict+=1
        if len(ok)==1: lenient+=1
        # merge duplicate listings: same normalized title and duration within 1 min
        groups=[]
        for h in ok:
            for g in groups:
                if norm(g['title'])==norm(h['title']) and abs((g['dur_s'] or 0)-(h['dur_s'] or 0))<=60: break
            else: groups.append(h)
        if len(groups)==1: merged+=1
    print(f'tol {tol*100:.0f} %: strict {strict}  lenient {lenient}  lenient+merged-duplicates {merged}  of 107')
