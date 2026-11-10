import re, unicodedata, json
from difflib import SequenceMatcher
import xml.etree.ElementTree as ET

STOP = {'ungekuerzt','ungekurzt','ungekurezt','ungek','hoerbuch','hoerspiel','lesung','gekuerzt','mit','voller','besetzung','live','und','dunkle','version','german','edition','audible','der','die','das','ein','eine','the','a','an'}

def norm(s):
    s = (s or '').lower().replace('ß','ss')
    s = unicodedata.normalize('NFKD', s)
    s = ''.join(c for c in s if not unicodedata.combining(c))
    s = re.sub(r'[^a-z0-9]+', ' ', s)
    return s.strip()

def toks(s):
    return [t for t in norm(s).split() if t not in STOP]

def clean_title(t):
    t = re.sub(r'\(.*?\)|\[.*?\]|»|«', '', t or '')
    t = re.split(r'\s+-\s+', t)[0]
    return re.sub(r'\s+', ' ', t).strip()

def title_ok(local, cand, cand_sub=''):
    a = toks(clean_title(local)) or toks(local)
    if not a: return False
    for c in (cand, f'{cand} {cand_sub}'):
        b = toks(c)
        if not b: continue
        sa, sb = ' '.join(a), ' '.join(b)
        if SequenceMatcher(None, sa, sb).ratio() >= 0.85: return True
        short, long_ = (sa, sb) if len(sa) <= len(sb) else (sb, sa)
        if len(short) >= 6 and (long_.startswith(short) or f' {short} ' in f' {long_} '): return True
        if len(short) >= 4 and long_.startswith(short + ' '): return True
    return False

def last_names(authors):
    out = set()
    for a in authors or []:
        t = norm(a).split()
        if t: out.add(t[-1])
        # "Sebastian Fitzek-Der Nachbar" style: also any token >=4 chars
        for x in t:
            if len(x) >= 4: out.add(x)
    return out

def author_ok(local_authors, cand_authors):
    la = last_names(local_authors)
    ct = set()
    for a in cand_authors or []:
        ct.update(norm(a).split())
    return bool(la & ct)

def dur_ok(local_s, cand_s, tol=0.03):
    if not cand_s or not local_s: return None
    return abs(cand_s - local_s) / local_s <= tol

# ---------- parsers: each returns list of candidates ----------
def p_audible(body):
    d = json.loads(body)
    out = []
    for p in d.get('products', []):
        ser = p.get('series') or []
        out.append(dict(id=p.get('asin'), title=p.get('title'), subtitle=p.get('subtitle') or '',
            authors=[a['name'] for a in p.get('authors', [])], narrators=[n['name'] for n in p.get('narrators', [])],
            dur_s=(p.get('runtime_length_min') or 0) * 60 or None,
            desc=bool(p.get('merchandising_summary')), series=bool(ser), publisher=bool(p.get('publisher_name')),
            isbn=False, cover=bool(p.get('product_images')), lang=p.get('language'), is_audio=True,
            seq=(ser[0].get('sequence') if ser else None), series_title=(ser[0].get('title') if ser else None)))
    return out

def p_audnexus(body):
    p = json.loads(body)
    if 'asin' not in p: return []
    return [dict(id=p.get('asin'), title=p.get('title'), subtitle=p.get('subtitle') or '',
        authors=[a['name'] for a in p.get('authors', [])], narrators=[n['name'] for n in p.get('narrators', [])],
        dur_s=(p.get('runtimeLengthMin') or 0) * 60 or None, desc=bool(p.get('summary') or p.get('description')),
        series=bool(p.get('seriesPrimary')), publisher=bool(p.get('publisherName')), isbn=bool(p.get('isbn')),
        cover=bool(p.get('image')), lang=p.get('language'), is_audio=True, genres=bool(p.get('genres')))]

NS = {'m': 'http://www.loc.gov/MARC21/slim', 's': 'http://www.loc.gov/zing/srw/'}
def _sf(rec, tag, code):
    r = []
    for df in rec.findall(f"m:datafield[@tag='{tag}']", NS):
        for sf in df.findall(f"m:subfield[@code='{code}']", NS):
            r.append((sf.text or '').strip())
    return r

def p_dnb(body):
    root = ET.fromstring(body)
    out = []
    for rec in root.iter('{http://www.loc.gov/MARC21/slim}record'):
        leader = (rec.findtext('m:leader', namespaces=NS) or '')
        t = ' '.join(_sf(rec, '245', 'a')[:1]); sub = ' '.join(_sf(rec, '245', 'b')[:1])
        names = []
        for tag in ('100', '700'):
            for n in _sf(rec, tag, 'a'):
                names.append(' '.join(reversed([x.strip() for x in n.split(',', 1)])))
        ext = ' '.join(_sf(rec, '300', 'a'))
        dur = None
        m = re.search(r'(\d+)\s*Min', ext)
        if m: dur = int(m.group(1)) * 60
        m2 = re.search(r'(\d+)\s*Std', ext)
        if m2: dur = (dur or 0) + int(m2.group(1)) * 3600
        for v in _sf(rec, '306', 'a'):
            if re.fullmatch(r'\d{6}', v): dur = int(v[:2]) * 3600 + int(v[2:4]) * 60 + int(v[4:])
        carrier = ' '.join(_sf(rec, '338', 'a') + _sf(rec, '300', 'a') + _sf(rec, '250', 'a'))
        is_audio = leader[6:7] in ('i', 'j') or bool(re.search(r'Audio|Hörbuch|CD|MP3|Min\.|Lesung', carrier))
        isbn = _sf(rec, '020', 'a')
        out.append(dict(id=(rec.findtext("m:controlfield[@tag='001']", namespaces=NS) or ''), title=t, subtitle=sub,
            authors=names, narrators=[], dur_s=dur, desc=False, series=bool(_sf(rec, '490', 'a') or _sf(rec, '830', 'a')),
            publisher=bool(_sf(rec, '264', 'b') or _sf(rec, '260', 'b')), isbn=bool(isbn), cover=False,
            lang=(_sf(rec, '041', 'a') or [None])[0], is_audio=is_audio, ext=ext[:60]))
    return out

def p_apple(body):
    d = json.loads(body)
    out = []
    for r in d.get('results', []):
        full = r.get('collectionName', '')
        t, _, sub = full.partition(': ')
        out.append(dict(id=str(r.get('collectionId')), title=t, subtitle=sub, authors=re.split(r'\s*[&,]\s*', r.get('artistName', '')),
            narrators=[], dur_s=None, desc=bool(r.get('description')), series=False, publisher=bool(r.get('copyright')),
            isbn=False, cover=bool(r.get('artworkUrl100')), lang=None, is_audio=True))
    return out

def p_ol(body):
    d = json.loads(body)
    out = []
    for r in d.get('docs', []):
        out.append(dict(id=r.get('key'), title=r.get('title'), subtitle=r.get('subtitle') or '', authors=r.get('author_name') or [],
            narrators=[], dur_s=None, desc=False, series=False, publisher=bool(r.get('publisher')), isbn=bool(r.get('isbn')),
            cover=bool(r.get('cover_i')), lang=','.join(r.get('language') or [])[:20], is_audio=False))
    return out

PARSERS = dict(audible=p_audible, audible_com=p_audible, dnb=p_dnb, applebooks=p_apple, openlibrary=p_ol, audnexus=p_audnexus)

def hits_for(src, book, cands):
    return [c for c in cands if title_ok(book['title'], c['title'], c.get('subtitle', '')) and author_ok(book['authors'], c['authors'])]
