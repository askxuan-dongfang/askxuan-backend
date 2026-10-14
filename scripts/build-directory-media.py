#!/usr/bin/env python3
"""Build guarded demo media fixtures and the matching H5 catalog. No network or DB writes."""
import json
from pathlib import Path
import argparse
p=argparse.ArgumentParser();p.add_argument('--h5',type=Path,required=True);args=p.parse_args()
root=Path(__file__).resolve().parent
examples=json.loads((root/'db/20261014_directory_examples.json').read_text())
catalog_path=args.h5/'src/data/directory-media.json';catalog=json.loads(catalog_path.read_text())
labels={'han_buddhism':('han','汉传佛教'),'tibetan_buddhism':('tibetan','藏传佛教'),'theravada_buddhism':('theravada','南传佛教'),'daoism':('dao','道教'),'folk':('folk','民间信俗')}
base='/c/assets/profiles/demo-20261014/'
def q(v):return "'"+str(v).replace("'","''")+"'"
def media(key,caption):return dict(url=base+key+'.jpg',thumbnail=base+key+'-thumb.jpg',caption=caption,kind='generated')
rows=['-- Generated demo-only media; exact names and empty/current demo covers are required.','SET NAMES utf8mb4;','START TRANSACTION;']
manifest={'temples':{},'masters':{}}
for kind in ['temples','masters']:
 for item in examples[kind]:
  code=item['code'];n=int(code[2:]);group,label=labels[item['belief']]
  if kind=='temples':
   name=item['name'];key=group+('01' if n%2 else '02');images=[media(group+'01',label+'场景示意 · 非场所实拍'),media(group+'02',label+'空间示意 · 共享演示素材')]
   cover=base+key+'.jpg';icon=base+key+'-thumb.jpg'
   rows.append(f'UPDATE askxuan_temple.temple SET cover_image={q(cover)} WHERE code={q(code)} AND name={q(name)} AND cover_image=\'\';')
   for i,im in enumerate(images):
    rows.append(f"INSERT INTO askxuan_temple.temple_image(temple_code,url,type,sort,caption,kind,name_guard,cover_guard) SELECT code,{q(im['url'])},'detail',{i+1},{q(im['caption'])},'generated',name,cover_image FROM askxuan_temple.temple WHERE code={q(code)} AND name={q(name)} AND cover_image={q(cover)} AND NOT EXISTS(SELECT 1 FROM askxuan_temple.temple_image WHERE temple_code={q(code)} AND url={q(im['url'])});")
  else:
   name=item['name'];key=group+'portrait'+('02' if item['track']=='platform' or item['position']=='独立居士' else '01')
   cover=base+key+'.jpg';icon=base+key+'-thumb.jpg';images=[media(key,'演示形象 · 非本人肖像'),media(group+('01' if n%2 else '02'),'交流空间示意 · 非场所实拍')]
   rows.append(f'UPDATE askxuan_master.master SET avatar={q(icon)} WHERE code={q(code)} AND dharma_name={q(name)} AND avatar=\'\';')
   for i,im in enumerate(images):
    rows.append(f"INSERT IGNORE INTO askxuan_master.master_image(master_code,url,caption,kind,sort,name_guard,avatar_guard) SELECT code,{q(im['url'])},{q(im['caption'])},'generated',{i+1},dharma_name,avatar FROM askxuan_master.master WHERE code={q(code)} AND dharma_name={q(name)} AND avatar={q(icon)};")
  entry=dict(name=name,original=cover if kind=='temples' else icon,cover=cover,icon=icon,images=images,sharedDemo=True)
  catalog[kind][code]=entry;manifest[kind][code]=entry
rows+=['COMMIT;']
(root/'db/20261014_directory_media.sql').write_text('\n'.join(rows)+'\n')
(root/'db/20261014_directory_media.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
catalog_path.write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
print('Guarded media mappings:',{k:len(v) for k,v in manifest.items()})
