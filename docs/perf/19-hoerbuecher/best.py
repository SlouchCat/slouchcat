import json, os, glob
from lib import *
books = json.load(open('books.json'))
def ldur(b): return b['dur_ffprobe'] if b['dur_ffprobe'] > 0 else b['dur_chapters']
best = {}; nohit = []
for i, b in enumerate(books):
    hs = []
    for f in sorted(glob.glob(f'raw/audible/{i:03d}_*.json')):
        hs = hits_for('audible', b, p_audible(open(f).read()))
        if hs: break
    if not hs: nohit.append(i); continue
    ok = [h for h in hs if dur_ok(ldur(b), h['dur_s'])]
    best[str(i)] = (ok or hs)[0]['id']
json.dump(best, open('audible_best.json', 'w')); json.dump(nohit, open('audible_nohit.json', 'w'))
print(len(best), len(nohit))
