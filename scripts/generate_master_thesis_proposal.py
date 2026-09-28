"""Export the active proposal Markdown to DOCX/PDF without duplicate thesis prose."""
import argparse
import os
from pathlib import Path
import re
from xml.sax.saxutils import escape

from docx import Document
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.opc.constants import RELATIONSHIP_TYPE as RT
from docx.shared import Mm, Pt, RGBColor
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md"
PLAN = SOURCE.parent / "THESIS_V2_MASTER_PLAN.md"
MARGIN_MM = 18
BODY_PT = 10.5
INLINE = re.compile(r"(\[[^\]]+\]\([^)]+\)|\*\*[^*]+\*\*|" + chr(96) + r"[^" + chr(96) + r"]+" + chr(96) + r")")
LINK = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")


def parse_blocks(text):
    """Support the proposal's explicit subset; reject unsupported block constructs."""
    blocks, paragraph, table = [], [], []

    def flush():
        if paragraph:
            blocks.append(("p", " ".join(paragraph)))
            paragraph.clear()
        if table:
            if any(len(row) != len(table[0]) for row in table):
                raise ValueError("Uneven Markdown table")
            blocks.append(("table", list(table)))
            table.clear()

    for raw in text.splitlines():
        line = raw.strip()
        if not line:
            flush()
        elif line.startswith("|"):
            if paragraph:
                flush()
            if not re.fullmatch(r"[| :\-]+", line):
                table.append([cell.strip() for cell in line.strip("|").split("|")])
        elif match := re.match(r"^(#{1,3}) (.+)$", line):
            flush()
            blocks.append(("h" + str(len(match[1])), match[2]))
        elif match := re.match(r"^(-|\d+\.) (.+)$", line):
            flush()
            prefix = "• " if match[1] == "-" else match[1] + " "
            blocks.append(("list", prefix + match[2]))
        elif line.startswith(("#", ">", chr(96) * 3, "---")):
            raise ValueError(f"Unsupported proposal block: {line[:60]}")
        else:
            if table:
                flush()
            paragraph.append(line)
    flush()
    return blocks


def visible(text):
    return LINK.sub(r"\1", text).replace("**", "").replace(chr(96), "")


def title_from(blocks, label):
    index = blocks.index(("p", "**" + label + "**"))
    return visible(blocks[index + 1][1])


def pdf_inline(text):
    parts = []
    for token in INLINE.split(text):
        if match := LINK.fullmatch(token):
            label, target = match.groups()
            parts.append(f'<link href="{escape(target, {chr(34): "&quot;"})}">{escape(label)}</link>'
                         if target.startswith("https://") else escape(label))
        elif token.startswith("**") and token.endswith("**"):
            parts.append("<b>" + escape(token[2:-2]) + "</b>")
        else:
            parts.append(escape(token.strip(chr(96))))
    return "".join(parts)


def word_inline(paragraph, text):
    for token in INLINE.split(text):
        if match := LINK.fullmatch(token):
            label, target = match.groups()
            link = OxmlElement("w:hyperlink")
            link.set(qn("r:id"), paragraph.part.relate_to(target, RT.HYPERLINK, is_external=True))
            run, value = OxmlElement("w:r"), OxmlElement("w:t")
            value.text = label
            run.append(value)
            link.append(run)
            paragraph._p.append(link)
        else:
            bold = token.startswith("**") and token.endswith("**")
            run = paragraph.add_run(token[2:-2] if bold else token.strip(chr(96)))
            run.bold = bold


def write_docx(blocks, path, title):
    doc = Document()
    section = doc.sections[0]
    section.page_width, section.page_height = Mm(210), Mm(297)
    section.top_margin = section.bottom_margin = Mm(MARGIN_MM)
    section.left_margin = section.right_margin = Mm(MARGIN_MM)
    for name in ("Normal", "Title", "Heading 1", "Heading 2"):
        style = doc.styles[name]
        style.font.name = "Arial"
        style.font.color.rgb = RGBColor(0, 0, 0)
    doc.styles["Normal"].font.size = Pt(BODY_PT)
    doc.styles["Normal"].paragraph_format.space_after = Pt(6)
    doc.styles["Normal"].paragraph_format.line_spacing = 1.1
    doc.styles["Title"].font.size = Pt(16)
    doc.styles["Heading 1"].font.size = Pt(13)
    doc.styles["Heading 2"].font.size = Pt(11.5)
    doc.core_properties.title = title
    doc.core_properties.subject = "Graduation thesis proposal generated from the active Markdown source"
    for kind, body in blocks:
        if kind == "table":
            table = doc.add_table(rows=0, cols=len(body[0]))
            table.style = "Table Grid"
            for index, row in enumerate(body):
                cells = table.add_row().cells
                props = cells[0]._tc.getparent().get_or_add_trPr()
                props.append(OxmlElement("w:cantSplit"))
                if index == 0:
                    props.append(OxmlElement("w:tblHeader"))
                for cell, text in zip(cells, row):
                    word_inline(cell.paragraphs[0], text)
                    shade = OxmlElement("w:shd")
                    shade.set(qn("w:fill"), "E4EAF0" if index == 0 else ("F5F7F9" if index % 2 else "FFFFFF"))
                    cell._tc.get_or_add_tcPr().append(shade)
                    for paragraph in cell.paragraphs:
                        for run in paragraph.runs:
                            run.font.size = Pt(9)
                            if index == 0:
                                run.bold = True
            doc.add_paragraph()
        else:
            style = {"h1": "Title", "h2": "Heading 1", "h3": "Heading 2"}.get(kind)
            paragraph = doc.add_paragraph(style=style)
            word_inline(paragraph, body)
            if kind.startswith("h"):
                paragraph.paragraph_format.keep_with_next = True
    footer = section.footer.paragraphs[0]
    footer.add_run("Đồ án tốt nghiệp Kỹ thuật Phần mềm  |  ")
    number = OxmlElement("w:fldSimple")
    number.set(qn("w:instr"), "PAGE")
    footer._p.append(number)
    for run in footer.runs:
        run.font.size = Pt(8)
    doc.save(path)


def write_pdf(blocks, path, title, font_dir):
    fonts = [font_dir / name for name in ("arial.ttf", "arialbd.ttf")]
    if not all(font.exists() for font in fonts):
        raise FileNotFoundError("Provide --font-dir containing arial.ttf and arialbd.ttf")
    for name, font in zip(("Proposal", "Proposal-Bold"), fonts):
        pdfmetrics.registerFont(TTFont(name, str(font)))
    pdfmetrics.registerFontFamily("Proposal", normal="Proposal", bold="Proposal-Bold")
    base = ParagraphStyle("body", fontName="Proposal", fontSize=BODY_PT, leading=14, spaceAfter=6)
    styles = {"p": base, "list": base}
    for kind, size in (("h1", 16), ("h2", 13), ("h3", 11.5)):
        styles[kind] = ParagraphStyle(kind, parent=base, fontName="Proposal-Bold",
                                      fontSize=size, leading=size + 3, spaceBefore=9,
                                      keepWithNext=True, alignment=TA_CENTER if kind == "h1" else 0)
    cell_style = ParagraphStyle("cell", parent=base, fontSize=9, leading=12, spaceAfter=0)
    margin = MARGIN_MM * 72 / 25.4
    width = A4[0] - 2 * margin
    story = []
    for kind, body in blocks:
        if kind == "table":
            data = [[Paragraph(pdf_inline(cell), cell_style) for cell in row] for row in body]
            table = Table(data, colWidths=[width / len(body[0])] * len(body[0]), repeatRows=1)
            table.setStyle(TableStyle([
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#e4eaf0")),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.HexColor("#f5f7f9"), colors.white]),
                ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#bac3cb")),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("TOPPADDING", (0, 0), (-1, -1), 6),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
            ]))
            story.extend([table, Spacer(1, 8)])
        else:
            story.append(Paragraph(pdf_inline(body), styles[kind]))

    def footer(canvas, document):
        canvas.setFont("Proposal", 8)
        canvas.drawString(margin, 25, "Đồ án tốt nghiệp Kỹ thuật Phần mềm")
        canvas.drawRightString(A4[0] - margin, 25, str(document.page))

    SimpleDocTemplate(str(path), pagesize=A4, leftMargin=margin, rightMargin=margin,
                      topMargin=margin, bottomMargin=margin, title=title).build(
                          story, onFirstPage=footer, onLaterPages=footer)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Validate source only; do not regenerate")
    parser.add_argument("--font-dir", type=Path,
                        default=Path(os.environ.get("WINDIR", "C:/Windows")) / "Fonts")
    args = parser.parse_args()
    blocks = parse_blocks(SOURCE.read_text(encoding="utf-8"))
    title = title_from(blocks, "Tên tiếng Việt")
    english = title_from(blocks, "Tên tiếng Anh")
    plan = PLAN.read_text(encoding="utf-8")
    if title not in plan or english not in plan:
        raise ValueError("Proposal titles differ from the locked master plan")
    if args.check:
        print(f"Source valid: {len(blocks)} blocks; locked VN/EN titles match.")
        return
    write_docx(blocks, SOURCE.with_suffix(".docx"), title)
    write_pdf(blocks, SOURCE.with_suffix(".pdf"), title, args.font_dir)
    print(f"Generated DOCX and PDF from {SOURCE}; {len(blocks)} shared blocks.")


if __name__ == "__main__":
    main()
