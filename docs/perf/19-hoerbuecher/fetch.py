# Fetches the raw answers of one source for every book of books.json (usage: python3 fetch.py <source> [index]).
# Sources: audible, audible_com (only books without a hit on audible.de), audnexus (needs best.py first), dnb, applebooks, openlibrary.
# Polite by design: one request per second and host (Apple 3.2 s), clear User-Agent, stops after three 403/429.
import sys, os, json, time, urllib.parse, urllib.request, urllib.error
from lib import *
UA = "Slouchcat-D0-measurement/1.0 (+https://github.com/SlouchCat/slouchcat; one-off research run, max 1 request per second)"
src = sys.argv[1]
only = int(sys.argv[2]) if len(sys.argv) > 2 else None
books = json.load(open('books.json'))
DELAY = dict(audible=1.05, audible_com=1.05, audnexus=1.05, dnb=1.05, applebooks=3.2, openlibrary=1.05)[src]
os.makedirs(f'raw/{src}', exist_ok=True)
log = open(f'log_{src}.jsonl', 'a')
last = [0.0]; bad = [0]

def get(url, name, idx, pas):
    path = f'raw/{src}/{idx:03d}_{pas}.{name}'
    if os.path.exists(path):
        return open(path, encoding='utf-8').read()
    wait = last[0] + DELAY - time.time()
    if wait > 0: time.sleep(wait)
    t0 = time.time(); last[0] = t0
    req = urllib.request.Request(url, headers={'User-Agent': UA, 'Accept': 'application/json, application/xml'})
    status = None; body = b''
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            status = r.status; body = r.read(5_000_000)
    except urllib.error.HTTPError as e:
        status = e.code; body = e.read(200000)
    except Exception as e:
        status = f'ERR {type(e).__name__}'
    ms = int((time.time() - t0) * 1000)
    text = body.decode('utf-8', 'replace')
    open(path, 'w', encoding='utf-8').write(text)
    log.write(json.dumps(dict(src=src, idx=idx, pas=pas, status=status, ms=ms, bytes=len(body), ts=t0)) + '\n'); log.flush()
    if status in (403, 429) or (isinstance(status, int) and status == 200 and text.lstrip().startswith('<!') and src != 'dnb'):
        bad[0] += 1
        if bad[0] >= 3:
            print('ABORT: source blocks', src, status); sys.exit(2)
    elif status == 200: bad[0] = 0
    return text if status == 200 else None

def cands_of(text, parser):
    if not text: return []
    try: return parser(text)
    except Exception as e: return []

def q(**kw): return urllib.parse.urlencode(kw)

def build(src, pas, b):
    t = b['title']; tc = clean_title(t) or t
    a = (b['authors'] or [''])[0]
    ln = (norm(a).split() or [''])[-1]
    if src == 'audible':
        base = 'https://api.audible.de/1.0/catalog/products?'
        g = 'product_desc,product_attrs,contributors,series,media'
        if pas == 1: return base + q(title=t, author=a, num_results=10, products_sort_by='Relevance', response_groups=g)
        if pas == 2: return base + q(title=tc, num_results=10, products_sort_by='Relevance', response_groups=g)
    if src == 'audible_com':
        return 'https://api.audible.com/1.0/catalog/products?' + q(title=tc, author=a, num_results=10, products_sort_by='Relevance', response_groups='product_desc,product_attrs,contributors,series,media')
    if src == 'dnb':
        base = 'https://services.dnb.de/sru/dnb?version=1.1&operation=searchRetrieve&recordSchema=MARC21-xml&maximumRecords=10&'
        if pas == 1: return base + q(query=f'tit="{t.replace(chr(34), " ")}" and per="{ln}"')
        if pas == 2: return base + q(query=f'tit="{tc.replace(chr(34), " ")}"')
    if src == 'applebooks':
        base = 'https://itunes.apple.com/search?country=ch&media=audiobook&limit=10&'
        if pas == 1: return base + q(term=f'{t} {a}')
        if pas == 2: return base + q(term=tc)
    if src == 'openlibrary':
        base = 'https://openlibrary.org/search.json?limit=10&fields=key,title,subtitle,author_name,isbn,publisher,first_publish_year,language,cover_i&'
        if pas == 1: return base + q(title=t, author=a)
        if pas == 2: return base + q(title=tc)
    return None

ext = 'xml' if src == 'dnb' else 'json'
t_start = time.time()
if src == 'audnexus':
    # needs ASINs found by audible
    for i, b in enumerate(books):
        if only is not None and i != only: continue
        best = json.load(open('audible_best.json')).get(str(i))
        if not best: continue
        txt = get(f"https://api.audnex.us/books/{best}?region=de", ext, i, 1)
else:
    parser = PARSERS['audible' if src.startswith('audible') else src]
    passes = [1] if src == 'audible_com' else [1, 2]
    nohit = set(json.load(open('audible_nohit.json')))
    for i, b in enumerate(books):
        if only is not None and i != only: continue
        if src == 'audible_com' and i not in nohit: continue
        got = False
        for pas in passes:
            url = build(src if src != 'audible_com' else 'audible_com', pas, b)
            if not url: continue
            txt = get(url, ext, i, pas)
            if hits_for(src, b, cands_of(txt, parser)): got = True; break
json.dump(dict(wall_s=time.time() - t_start), open(f'wall_{src}.json', 'w'))
print('done', src, round(time.time() - t_start))
