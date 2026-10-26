"""Rebuild four PDFs and the distribution ZIP from the reviewed Markdown sources."""
from pathlib import Path
from xml.sax.saxutils import escape
import re
import zipfile
import textwrap

from reportlab.pdfgen import canvas
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, PageBreak, LongTable, TableStyle, Preformatted
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.lib.pagesizes import A4
from pypdf import PdfReader

BASE = Path(__file__).resolve().parents[1]
PDF = BASE.parent / 'pdf'
PDF.mkdir(parents=True, exist_ok=True)
FONT = Path('C:/Windows/Fonts')
pdfmetrics.registerFont(TTFont('Calibri', str(FONT/'calibri.ttf')))
pdfmetrics.registerFont(TTFont('Calibri-Bold', str(FONT/'calibrib.ttf')))
pdfmetrics.registerFont(TTFont('Calibri-Italic', str(FONT/'calibrii.ttf')))
pdfmetrics.registerFontFamily('Calibri', normal='Calibri', bold='Calibri-Bold', italic='Calibri-Italic', boldItalic='Calibri-Bold')
TEAL = colors.HexColor('#087F8C')
INK = colors.HexColor('#173444')
MUTED = colors.HexColor('#55707D')
LIGHT = colors.HexColor('#EEF6F7')
styles = getSampleStyleSheet()
styles.add(ParagraphStyle(name='BodyCustom', fontName='Calibri', fontSize=10.1, leading=13.6, textColor=INK, spaceAfter=7, splitLongWords=True))
styles.add(ParagraphStyle(name='TitleCustom', fontName='Calibri-Bold', fontSize=30, leading=34, textColor=INK, spaceAfter=12, keepWithNext=True))
styles.add(ParagraphStyle(name='H2Custom', fontName='Calibri-Bold', fontSize=18, leading=23, textColor=TEAL, spaceBefore=15, spaceAfter=10, keepWithNext=True))
styles.add(ParagraphStyle(name='H3Custom', fontName='Calibri-Bold', fontSize=12, leading=16, textColor=INK, spaceBefore=10, spaceAfter=6, keepWithNext=True))
styles.add(ParagraphStyle(name='CellCustom', fontName='Calibri', fontSize=9.2, leading=12, textColor=INK, spaceAfter=0, splitLongWords=True))
styles.add(ParagraphStyle(name='CellHeadCustom', parent=styles['CellCustom'], fontName='Calibri-Bold', textColor=colors.white))
styles.add(ParagraphStyle(name='CodeCustom', fontName='Courier', fontSize=7.8, leading=10.3, textColor=INK, backColor=LIGHT, borderPadding=9, spaceBefore=5, spaceAfter=10))
styles.add(ParagraphStyle(name='BulletCustom', parent=styles['BodyCustom'], leftIndent=12, firstLineIndent=-9, spaceAfter=5))

def inline(s):
    s = escape(s)
    s = re.sub(r'`([^`]+)`', lambda m: '<font name="Courier" size="8.4">'+m.group(1)+'</font>', s)
    s = re.sub(r'\*\*([^*]+)\*\*', r'<b>\1</b>', s)
    return s

def render_md(source, destination, short_title):
    lines = source.read_text(encoding='utf-8').splitlines()
    if source.parent.name == 'frontend':
        lines[0] = '# Teknis Frontend SiagaRS'
    elif source.parent.name == 'backend':
        lines[0] = '# Teknis Backend SiagaRS'
    flow = []
    i = 0
    width = A4[0]-100
    while i < len(lines):
        line = lines[i].strip()
        if not line:
            i += 1
            continue
        if line == '---PAGE---':
            flow.append(PageBreak()); i += 1; continue
        if line.startswith('```'):
            code=[]; i+=1
            while i<len(lines) and not lines[i].startswith('```'):
                code.extend(textwrap.wrap(lines[i], width=88, replace_whitespace=False, drop_whitespace=False) or [''])
                i+=1
            flow.append(Preformatted('\n'.join(code), styles['CodeCustom'])); i+=1; continue
        if line.startswith('|'):
            rows=[]
            while i<len(lines) and lines[i].strip().startswith('|'):
                cells=lines[i].strip().strip('|').split('|')
                if not all(re.fullmatch(r'[\s:-]+', x) for x in cells):
                    rows.append([x.strip() for x in cells])
                i+=1
            n=max(map(len,rows))
            ratios=([0.25,0.75] if n==2 else [0.14,0.43,0.43]) if n<=3 else [1/n]*n
            data=[[Paragraph(inline(c), styles['CellHeadCustom' if k==0 else 'CellCustom']) for c in row] for k,row in enumerate(rows)]
            table=LongTable(data, colWidths=[width*x for x in ratios], repeatRows=1, hAlign='LEFT',
                rowSplitRange=(3,-2) if len(data)>6 else None)
            table.setStyle(TableStyle([
                ('BACKGROUND',(0,0),(-1,0),TEAL),
                ('ROWBACKGROUNDS',(0,1),(-1,-1),[LIGHT,colors.white]),
                ('VALIGN',(0,0),(-1,-1),'TOP'),
                ('LEFTPADDING',(0,0),(-1,-1),8),('RIGHTPADDING',(0,0),(-1,-1),8),
                ('TOPPADDING',(0,0),(-1,-1),7),('BOTTOMPADDING',(0,0),(-1,-1),7),
                ('LINEBELOW',(0,0),(-1,0),0.6,TEAL),
                ('LINEBELOW',(0,1),(-1,-1),0.3,colors.HexColor('#D9E6E9'))]))
            flow.extend([table,Spacer(1,9)]); continue
        if line.startswith('# '):
            flow.append(Paragraph(inline(line[2:]),styles['TitleCustom'])); i+=1; continue
        if line.startswith('## '):
            flow.append(Paragraph(inline(line[3:]),styles['H2Custom'])); i+=1; continue
        if line.startswith('### '):
            flow.append(Paragraph(inline(line[4:]),styles['H3Custom'])); i+=1; continue
        if line.startswith('- ') or re.match(r'^\d+\. ',line):
            flow.append(Paragraph('- '+inline(re.sub(r'^(?:- |\d+\. )','',line)),styles['BulletCustom'])); i+=1; continue
        paragraph=[line]; i+=1
        while i<len(lines) and lines[i].strip() and not lines[i].startswith(('#','|','- ','```','---PAGE---')):
            paragraph.append(lines[i].strip()); i+=1
        flow.append(Paragraph(inline(' '.join(paragraph)),styles['BodyCustom']))

    def decoration(c, doc):
        c.saveState()
        c.setFillColor(TEAL); c.rect(0,A4[1]-9,A4[0],9,fill=1,stroke=0)
        c.setFont('Calibri-Bold',8); c.setFillColor(MUTED)
        c.drawString(50,A4[1]-32,'MALASNGODING  /  SIAGARS V0.1')
        c.setFont('Calibri',8); c.drawRightString(A4[0]-50,A4[1]-32,'PRE-PRODUCTION  |  23 OKT 2026')
        c.setStrokeColor(colors.HexColor('#D9E6E9')); c.line(50,43,A4[0]-50,43)
        c.setFont('Calibri',8); c.setFillColor(MUTED)
        c.drawString(50,29,short_title)
        c.drawRightString(A4[0]-50,29,f'{doc.page}')
        c.restoreState()
    doc=SimpleDocTemplate(str(destination), pagesize=A4, leftMargin=50,rightMargin=50,topMargin=55,bottomMargin=57,
        title=short_title,author='Tim MalasNgoding',subject='SiagaRS V0.1 pre-production specification', pageCompression=1)
    doc.build(flow,onFirstPage=decoration,onLaterPages=decoration)
    reader=PdfReader(destination)
    assert len(reader.pages)>0
    for j,p in enumerate(reader.pages):
        assert len((p.extract_text() or '').strip())>100, (destination,j)
    print(f'{destination.name}: {len(reader.pages)} pages, {destination.stat().st_size:,} bytes')

JOBS=[
    (BASE/'sources/Scope_MVP.md','SiagaRS_V0.1_Scope_MVP.pdf','Ringkasan scope MVP'),
    (BASE/'sources/Teknis_Sistem.md','SiagaRS_V0.1_Teknis_Sistem.pdf','Spesifikasi teknis sistem'),
    (BASE/'frontend/PRD.md','SiagaRS_V0.1_Teknis_Frontend.pdf','Spesifikasi teknis frontend'),
    (BASE/'backend/PRD.md','SiagaRS_V0.1_Teknis_Backend.pdf','Spesifikasi teknis backend'),
]
for src, name, title in JOBS:
    render_md(src,PDF/name,title)

with zipfile.ZipFile(BASE/'SiagaRS_V0.1_Preproduction.zip','w',compression=zipfile.ZIP_DEFLATED) as z:
    z.write(BASE/'README.md','README.md')
    for rel in ['frontend/PRD.md','frontend/AGENTS.md','backend/PRD.md','backend/AGENTS.md']:
        z.write(BASE/rel,rel)
    for _,name,_ in JOBS:
        z.write(PDF/name,'pdf/'+name)
print('ZIP created: four PDFs, four project Markdown files, one package README.')
