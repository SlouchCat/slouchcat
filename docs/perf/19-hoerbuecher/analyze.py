import json, glob, os, statistics
from collections import Counter, defaultdict
from lib import *
books = json.load(open('books.json'))
def ldur(b): return b['dur_ffprobe'] if b['dur_ffprobe'] > 0 else b['dur_chapters']
SRC = ['audible', 'audible_com', 'dnb', 'applebooks', 'openlibrary']
HASDUR = {'audible': True, 'audible_com': True, 'dnb': True, 'applebooks': False, 'openlibrary': False}
logs = defaultdict(list)
for s in SRC + ['audnexus']:
    p = f'log_{s}.jsonl'
    if os.path.exists(p):
        for l in open(p): logs[s].append(json.loads(l))
res = {}   # src -> idx -> dict
for s in SRC:
    res[s] = {}
    parser = PARSERS['audible' if s.startswith('audible') else s]
    for i, b in enumerate(books):
        fs = sorted(glob.glob(f'raw/{s}/{i:03d}_*'))
        if not fs: continue
        allc = []; hs = []; tonly = []
        for f in fs:
            txt = open(f, encoding='utf-8').read()
            try: cs = parser(txt)
            except Exception: cs = []
            allc += cs
        seen = set(); uniq = []
        for c in allc:
            if c['id'] in seen: continue
            seen.add(c['id']); uniq.append(c)
        hs = hits_for(s, b, uniq)
        tonly = [c for c in uniq if title_ok(b['title'], c['title'], c.get('subtitle', '')) and not author_ok(b['authors'], c['authors'])]
        L = ldur(b)
        durs = [dur_ok(L, h['dur_s']) for h in hs]
        okd = [h for h, d in zip(hs, durs) if d]
        audio_hs = [h for h in hs if h.get('is_audio')]
        res[s][i] = dict(nreq=len(fs), hit=len(hs) > 0, nhit=len(hs), unique=len(hs) == 1, title_only=len(tonly) > 0 and not hs,
            hasdur=any(h['dur_s'] for h in hs), dur_ok=len(okd) > 0, auto=(len(hs) == 1 and len(okd) == 1),
            auto_len=(len(okd) == 1), audio_hit=len(audio_hs) > 0, best=(okd or hs or [None])[0], first_pass=fs[0].split('_')[-1].split('.')[0])
def pct(n, d): return f'{n}/{d} ({100*n/d:.0f} %)' if d else '–'
pops = {'107': set(range(len(books))), '89 (mit Audiodatei)': {i for i, b in enumerate(books) if b['nfiles'] > 0}}
for name, pop in pops.items():
    print('\n=== Population', name, len(pop))
    for s in ['audible', 'dnb', 'applebooks', 'openlibrary']:
        R = [res[s][i] for i in pop if i in res[s]]
        n = len(R)
        hit = sum(r['hit'] for r in R); uni = sum(r['unique'] for r in R); to = sum(r['title_only'] for r in R)
        hd = sum(r['hasdur'] for r in R); dok = sum(r['dur_ok'] for r in R); au = sum(r['auto'] for r in R); aul = sum(r['auto_len'] for r in R)
        aud = sum(r['audio_hit'] for r in R)
        print(f'{s:12} n={n} hit {pct(hit,n)} | unique {pct(uni,n)} | title-only(kein Autor) {pct(to,n)} | with dur {pct(hd,n)} | dur_ok {pct(dok,n)} | auto strict {pct(au,n)} | auto lenient {pct(aul,n)} | audio-hit {pct(aud,n)}')
    # audible incl .com
    ids = [i for i in pop]
    ac = res['audible_com']
    ah = [i for i in ids if res['audible'][i]['hit'] or (i in ac and ac[i]['hit'])]
    print('audible .de+.com hit', pct(len(ah), len(ids)))
    # union / order
    def anyh(srcs, f='hit'): return {i for i in ids if any(res[s].get(i, {}).get(f) for s in srcs)}
    for label, srcs in [('audible', ['audible']), ('+dnb', ['audible', 'dnb']), ('+apple', ['audible', 'dnb', 'applebooks']), ('+ol', ['audible', 'dnb', 'applebooks', 'openlibrary']), ('dnb allein', ['dnb']), ('apple allein', ['applebooks'])]:
        print(f'  Abdeckung {label:12}', pct(len(anyh(srcs)), len(ids)))
    print('  kein Treffer in irgendeiner Quelle:', pct(len(set(ids) - anyh(['audible', 'dnb', 'applebooks', 'openlibrary'])), len(ids)))
# requests and timing
print('\n=== Anfragen und Laufzeit')
for s in SRC + ['audnexus']:
    L = logs[s]
    if not L: continue
    ms = sorted(x['ms'] for x in L)
    ok = Counter(x['status'] for x in L)
    wall = None
    if os.path.exists(f'wall_{s}.json'): wall = json.load(open(f'wall_{s}.json'))['wall_s']
    byb = Counter(x['idx'] for x in L)
    print(f"{s:12} Anfragen {len(L)}  je Buch ø {len(L)/107:.2f} (max {max(byb.values())})  Status {dict(ok)}  Latenz p50 {ms[len(ms)//2]} ms p95 {ms[int(len(ms)*0.95)-1]} ms max {ms[-1]}  Bytes ø {sum(x['bytes'] for x in L)//len(L)}  Laufzeit {wall}")
# first-pass success
print('\n=== Erste Anfrage reicht (Pass 1 Treffer) / zweite nötig')
for s in ['audible', 'dnb', 'applebooks', 'openlibrary']:
    R = res[s]
    p1 = sum(1 for r in R.values() if r['hit'] and r['first_pass'] == '1' and r['nreq'] == 1)
    p2 = sum(1 for r in R.values() if r['nreq'] == 2)
    print(s, 'Treffer mit 1 Anfrage', p1, '| 2 Anfragen nötig', p2)
# fields
print('\n=== Felder des besten Treffers')
for s in ['audible', 'dnb', 'applebooks', 'openlibrary']:
    B = [r['best'] for r in res[s].values() if r['best']]
    if not B: continue
    print(s, len(B), {k: sum(1 for b in B if b.get(k)) for k in ['desc', 'series', 'publisher', 'isbn', 'cover']})
json.dump({s: {str(i): {k: v for k, v in r.items() if k != 'best'} for i, r in res[s].items()} for s in SRC}, open('res.json', 'w'))
