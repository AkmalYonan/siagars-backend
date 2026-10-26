from pathlib import Path
import re
import zipfile
from PIL import Image, ImageDraw
import pypdfium2 as pdfium
from pypdf import PdfReader

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'tmp/pdfs/siagars-qa'
OUT.mkdir(parents=True, exist_ok=True)
for path in sorted((ROOT/'output/pdf').glob('SiagaRS_V0.1_*.pdf')):
    doc=pdfium.PdfDocument(str(path))
    thumbs=[]
    for idx,page in enumerate(doc):
        img=page.render(scale=1.35).to_pil().convert('RGB')
        img.save(OUT/f'{path.stem}-{idx+1:02}.png')
        img.thumbnail((495,701))
        tile=Image.new('RGB',(515,735),'#E5ECEF')
        tile.paste(img,((515-img.width)//2,10))
        ImageDraw.Draw(tile).text((12,715),f'{path.stem} / {idx+1}',fill='#173444')
        thumbs.append(tile)
    sheet=Image.new('RGB',(1030,735*((len(thumbs)+1)//2)),'#E5ECEF')
    for i,t in enumerate(thumbs):sheet.paste(t,((i%2)*515,(i//2)*735))
    sheet.save(OUT/f'{path.stem}-contact.png')
    reader=PdfReader(path)
    text='\n'.join(p.extract_text() or '' for p in reader.pages)
    assert '\ufffd' not in text
    print(path.name, 'pages',len(reader.pages),'chars',len(text))
    for i,p in enumerate(reader.pages):
        lines=(p.extract_text() or '').splitlines()
        print(' ',i+1,'start:', ' / '.join(lines[2:4])[:110], 'end:', ' / '.join(lines[-2:])[:110])

bundle=ROOT/'output/siagars-v0.1/SiagaRS_V0.1_Preproduction.zip'
with zipfile.ZipFile(bundle) as z:
    assert z.testzip() is None
    assert len(z.namelist())==9
    print('ZIP OK',z.namelist())
print('Contact sheets:',OUT)
