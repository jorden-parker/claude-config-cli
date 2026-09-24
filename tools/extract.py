import re, json, sys
src = open('ref.md').read()
lines = src.split('\n')
# find start of sections after index
start = next(i for i,l in enumerate(lines) if l.startswith('## Model and responses'))
lines = lines[start:]
entries = []
section = None
i = 0
cur = None
def flush():
    if cur: entries.append(cur)
while i < len(lines):
    l = lines[i]
    if l.startswith('## '):
        flush(); cur=None
        section = l[3:].strip()
    elif l.startswith('### `'):
        flush()
        key = re.match(r'### `([^`]+)`', l).group(1)
        cur = {'key': key, 'section': section, 'desc': '', 'scope':'', 'type':'', 'default':'', 'options':[], 'example':'', 'body':[]}
    elif cur is not None:
        cur['body'].append(l)
    i += 1
flush()
for e in entries:
    body = e['body']
    # description: first non-empty paragraph
    para=[]; inwarn=False
    for l in body:
        t=l.strip()
        if t.startswith('<Warning>') or t.startswith('<Note>'): inwarn=True; continue
        if t.startswith('</Warning>') or t.startswith('</Note>'): inwarn=False; continue
        if inwarn: continue
        if t=='' and para: break
        if t=='': continue
        if l.startswith('*') or l.startswith('```') or l.startswith('<'): break
        para.append(t)
    e['desc']=' '.join(para)
    # bullets
    j=0
    while j < len(body):
        l=body[j]
        m=re.match(r'\* \*\*Scope\*\*: (.*)', l)
        if m: e['scope']=re.sub(r'\[`([^`]+)`\]\([^)]*\)', r'\1', m.group(1))
        m=re.match(r'\* \*\*Type\*\*: (.*)', l)
        if m:
            e['type']=m.group(1)
            k=j+1
            while k<len(body) and body[k].startswith('  * '):
                mm=re.match(r'  \* `([^`]+)`:?\s*(.*)', body[k])
                if mm: e['options'].append({'value':mm.group(1),'help':mm.group(2)})
                else: e['options'].append({'value':body[k][4:], 'help':''})
                k+=1
        m=re.match(r'\* \*\*Default\*\*: (.*)', l)
        if m: e['default']=m.group(1)
        if l.startswith('```json') and not e['example']:
            k=j+1; ex=[]
            while k<len(body) and not body[k].startswith('```'):
                ex.append(body[k]); k+=1
            e['example']='\n'.join(ex)
        j+=1
    del e['body']
json.dump(entries, open('entries.json','w'), indent=1)
print(len(entries))
for e in entries:
    print(f"{e['key']:45} | {e['type'][:70]}")
