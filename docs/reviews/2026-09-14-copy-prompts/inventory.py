from pathlib import Path
import re,json,csv,collections
root=Path.cwd(); out=root/'docs/reviews/2026-09-14-copy-prompts'
files=[]
for base in ['apps','packages']:
 for p in (root/base).rglob('*'):
  if not p.is_file() or any(x in p.parts for x in ['node_modules','dist','.astro','.git','coverage','test','tests','e2e','fixtures','__fixtures__','testdata']): continue
  if p.suffix not in ['.go','.ts','.tsx','.astro','.json','.md']: continue
  if re.search(r'(?:_test|\.test|\.spec)\.',p.name): continue
  if '/store/sqlc/' in str(p): continue
  files.append(p)
# Lexical candidates, not a claim that every literal is rendered. Retain location and scope for review.
lex=re.compile(r'//[^\n]*|/\*[\s\S]*?\*/|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|`[^`]*`')
rows=[]; prompts=[]; hits=[]
pat=re.compile('狠|凭什么|逼自己|骗了你|在骗你|才危险|一刀|审判者|只看能不能|没盖到|并排说几条|别道歉|只能出现一个问号|每次开口都要|天生不信任|挣到它占|别主动用正式')
for p in sorted(files):
 s=p.read_text(errors='replace'); rel=str(p.relative_to(root))
 # full raw Chinese lines are a deliberately broad backstop for JSX text and composed prompts
 for n,line in enumerate(s.splitlines(),1):
  if re.search('[\u4e00-\u9fff]',line) and not line.lstrip().startswith(('//','*','{/*','<!--')):
   rows.append([rel,n,line.strip()])
 for m in lex.finditer(s):
  v=m.group(); line=s.count('\n',0,m.start())+1
  if v.startswith(('//','/*')): continue
  if pat.search(v): hits.append([rel,line,v])
  if p.suffix=='.go' and len(v)>100 and (re.search('prompt|system|instruction|posture|rules|routine|lens',s[max(0,m.start()-120):m.start()],re.I) or re.search('你是|只输出|只返回|输出格式',v)):
   prompts.append({'file':rel,'line':line,'text':v[1:-1]})
for name,data,header in [('text-candidates.tsv',rows,['file','line','source']),('risk-candidates.tsv',hits,['file','line','literal'])]:
 with (out/name).open('w') as f:
  w=csv.writer(f,delimiter='\t',lineterminator='\n');w.writerow(header);w.writerows(data)
(out/'prompt-literals.json').write_text(json.dumps(prompts,ensure_ascii=False,indent=2))
summary={'source_files_scanned':len(files),'chinese_source_lines':len(rows),'prompt_literal_candidates':len(prompts),'risk_literal_candidates':len(hits),'by_surface':dict(collections.Counter('/'.join(p.relative_to(root).parts[:2]) for p in files))}
(out/'inventory-summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2))
print(json.dumps(summary,ensure_ascii=False));print('\n'.join(f"{p['file']}:{p['line']} {p['text'][:65].replace(chr(10),' ')}" for p in prompts))
