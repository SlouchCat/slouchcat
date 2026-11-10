# Reads the metadata.json of every audiobook folder (read-only) and the duration of its audio files with ffprobe.
# Writes books.json (kept out of the repository: it holds Marco's titles). Usage: python3 books.py
import json,os,subprocess,sys
root="/mnt/nerdnas/medien/Audiobookshelf/Hörbücher"
AUD={'.m4b','.mp3','.m4a','.flac','.ogg','.opus'}
out=[]
for dp,dn,fn in os.walk(root):
    if 'metadata.json' not in fn: continue
    d=json.load(open(os.path.join(dp,'metadata.json')))
    files=sorted(f for f in fn if os.path.splitext(f)[1].lower() in AUD)
    ch=d.get('chapters') or []
    chdur=max([c['end'] for c in ch],default=0)
    tot=0
    for f in files:
        r=subprocess.run(['ffprobe','-v','error','-show_entries','format=duration','-of','csv=p=0',os.path.join(dp,f)],capture_output=True,text=True)
        try: tot+=float(r.stdout.strip())
        except: pass
    out.append(dict(rel=os.path.relpath(dp,root),title=d.get('title'),subtitle=d.get('subtitle'),authors=d.get('authors'),narrators=d.get('narrators'),series=d.get('series'),asin=d.get('asin'),isbn=d.get('isbn'),language=d.get('language'),year=d.get('publishedYear'),publisher=d.get('publisher'),nfiles=len(files),dur_ffprobe=tot,dur_chapters=chdur))
json.dump(out,open('books.json','w'),ensure_ascii=False,indent=1)
print(len(out), sum(1 for b in out if b['nfiles']==0))
