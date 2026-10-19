"""Small, original fixtures for validating the device import pipeline."""
from pathlib import Path
from zipfile import ZipFile, ZIP_STORED, ZIP_DEFLATED

out = Path(__file__).resolve().parents[1] / 'dist'
out.mkdir(exist_ok=True)
stream = b'BT /F1 24 Tf 60 730 Td (Quick Share - verifica PDF) Tj 0 -45 Td /F1 14 Tf (Trasferimento completo dal PC al tablet.) Tj ET'
objects = [b'<< /Type /Catalog /Pages 2 0 R >>', b'<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
           b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
           b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', b'<< /Length '+str(len(stream)).encode()+b' >>\nstream\n'+stream+b'\nendstream']
data = bytearray(b'%PDF-1.4\n'); offsets = [0]
for i, obj in enumerate(objects, 1):
    offsets.append(len(data)); data.extend(f'{i} 0 obj\n'.encode()+obj+b'\nendobj\n')
xref = len(data)
data.extend(b'xref\n0 6\n0000000000 65535 f \n')
for offset in offsets[1:]:
    data.extend(f'{offset:010d} 00000 n \n'.encode())
data.extend(f'trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n'.encode())
(out / 'QuickShare_verifica_PDF.pdf').write_bytes(data)
with ZipFile(out / 'QuickShare_verifica_EPUB.epub', 'w') as z:
    z.writestr('mimetype', 'application/epub+zip', compress_type=ZIP_STORED)
    z.writestr('META-INF/container.xml', '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>', compress_type=ZIP_DEFLATED)
    z.writestr('OEBPS/book.opf', '<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:uuid:a701fd83-7532-43ad-bc42-453ad68929f9</dc:identifier><dc:title>QuickShare verifica EPUB</dc:title><dc:language>it</dc:language><meta property="dcterms:modified">2026-10-18T11:30:00Z</meta></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="chapter"/></spine></package>', compress_type=ZIP_DEFLATED)
    z.writestr('OEBPS/chapter.xhtml', '<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xml:lang="it"><head><title>Verifica EPUB</title></head><body><h1>Quick Share</h1><p>Trasferimento completo dal PC al tablet e conversione EPUB nativa.</p></body></html>', compress_type=ZIP_DEFLATED)
    z.writestr('OEBPS/nav.xhtml', '<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Indice</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Verifica</a></li></ol></nav></body></html>', compress_type=ZIP_DEFLATED)
print('Fixtures written to', out)
