import os
import shutil
from docx import Document
from docx.shared import Inches, Pt, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT
from docx.oxml import parse_xml
from docx.oxml.ns import nsdecls
import win32com.client

def set_cell_background(cell, fill_color):
    tcPr = cell._tc.get_or_add_tcPr()
    shd = parse_xml(f'<w:shd {nsdecls("w")} w:fill="{fill_color}"/>')
    tcPr.append(shd)

def set_cell_margins(cell, top=60, bottom=60, left=90, right=90):
    tcPr = cell._tc.get_or_add_tcPr()
    tcMar = parse_xml(f'''
        <w:tcMar {nsdecls("w")}>
            <w:top w:w="{top}" w:type="dxa"/>
            <w:bottom w:w="{bottom}" w:type="dxa"/>
            <w:left w:w="{left}" w:type="dxa"/>
            <w:right w:w="{right}" w:type="dxa"/>
        </w:tcMar>
    ''')
    tcPr.append(tcMar)

def add_callout(doc, text, title="TỔNG QUAN CHIẾN LƯỢC: LẤY KỸ THUẬT LÀM LÕI, LẤY NGHIỆP VỤ LÀM VŨ KHÍ", bg_color="F0F4F8", border_color="1E3A5F"):
    tbl = doc.add_table(rows=1, cols=1)
    tbl.alignment = WD_TABLE_ALIGNMENT.CENTER
    tbl.autofit = False
    tbl.columns[0].width = Inches(6.5)
    
    cell = tbl.cell(0, 0)
    set_cell_background(cell, bg_color)
    set_cell_margins(cell, top=100, bottom=100, left=150, right=150)
    
    tcPr = cell._tc.get_or_add_tcPr()
    borders = parse_xml(f'''
        <w:tcBorders {nsdecls("w")}>
            <w:top w:val="none"/>
            <w:left w:val="single" w:sz="24" w:space="0" w:color="{border_color}"/>
            <w:bottom w:val="none"/>
            <w:right w:val="none"/>
        </w:tcBorders>
    ''')
    tcPr.append(borders)
    
    p = cell.paragraphs[0]
    p.paragraph_format.space_before = Pt(2)
    p.paragraph_format.space_after = Pt(2)
    p.paragraph_format.line_spacing = 1.15
    
    if title:
        run_title = p.add_run(f"📌 {title}\n")
        run_title.bold = True
        run_title.font.name = "Arial"
        run_title.font.size = Pt(9.5)
        run_title.font.color.rgb = RGBColor(30, 58, 95)
    
    run_text = p.add_run(text)
    run_text.font.name = "Arial"
    run_text.font.size = Pt(8.5)
    run_text.font.color.rgb = RGBColor(40, 50, 60)
    
    sp_p = doc.add_paragraph()
    sp_p.paragraph_format.space_before = Pt(0)
    sp_p.paragraph_format.space_after = Pt(3)

def format_heading(p, text, level=1):
    p.paragraph_format.keep_with_next = True
    run = p.add_run(text)
    run.font.name = "Arial"
    run.bold = True
    if level == 1:
        p.paragraph_format.space_before = Pt(12)
        p.paragraph_format.space_after = Pt(3)
        run.font.size = Pt(12)
        run.font.color.rgb = RGBColor(14, 43, 82)
    elif level == 2:
        p.paragraph_format.space_before = Pt(8)
        p.paragraph_format.space_after = Pt(2)
        run.font.size = Pt(10.5)
        run.font.color.rgb = RGBColor(13, 79, 60)
    elif level == 3:
        p.paragraph_format.space_before = Pt(4)
        p.paragraph_format.space_after = Pt(2)
        run.font.size = Pt(9)
        run.font.color.rgb = RGBColor(50, 60, 75)

def format_bullet(doc, bold_prefix, text):
    p = doc.add_paragraph(style='List Bullet')
    p.paragraph_format.space_before = Pt(1)
    p.paragraph_format.space_after = Pt(1.5)
    p.paragraph_format.line_spacing = 1.15
    
    r1 = p.add_run(bold_prefix)
    r1.font.name = "Arial"
    r1.font.size = Pt(8.5)
    r1.bold = True
    r1.font.color.rgb = RGBColor(20, 30, 40)
    
    r2 = p.add_run(text)
    r2.font.name = "Arial"
    r2.font.size = Pt(8.5)
    r2.font.color.rgb = RGBColor(50, 60, 70)

def main():
    doc = Document()
    
    for section in doc.sections:
        section.top_margin = Inches(0.7)
        section.bottom_margin = Inches(0.7)
        section.left_margin = Inches(0.75)
        section.right_margin = Inches(0.75)
        
        footer = section.footer
        f_p = footer.paragraphs[0]
        f_p.alignment = WD_ALIGN_PARAGRAPH.RIGHT
        f_run = f_p.add_run("Lộ Trình Chiến Lược: Kỹ Sư SE Bảo Mật PTIT → Chuyên Gia Giải Pháp ERP Quốc Tế")
        f_run.font.name = "Arial"
        f_run.font.size = Pt(8)
        f_run.font.color.rgb = RGBColor(140, 150, 160)

    # Header
    tp = doc.add_paragraph()
    tp.paragraph_format.space_before = Pt(2)
    tp.paragraph_format.space_after = Pt(2)
    tp.alignment = WD_ALIGN_PARAGRAPH.CENTER
    r_tp = tp.add_run("HỌC VIỆN CÔNG NGHỆ BƯU CHÍNH VIỄN THÔNG (PTIT) — CHƯƠNG TRÌNH CLC\nBẢN KẾ HOẠCH PHÁT TRIỂN NĂNG LỰC & SỰ NGHIỆP CHIẾN LƯỢC")
    r_tp.font.name = "Arial"; r_tp.font.size = Pt(11); r_tp.bold = True
    r_tp.font.color.rgb = RGBColor(14, 43, 82)

    title_p = doc.add_paragraph()
    title_p.paragraph_format.space_before = Pt(4)
    title_p.paragraph_format.space_after = Pt(2)
    title_p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    t_run = title_p.add_run("TỪ KỸ SƯ PHẦN MỀM BẢO MẬT ĐẾN CHUYÊN GIA GIẢI PHÁP ERP QUỐC TẾ")
    t_run.font.name = "Arial"; t_run.font.size = Pt(12.5); t_run.bold = True
    t_run.font.color.rgb = RGBColor(180, 20, 20)

    sub_p = doc.add_paragraph()
    sub_p.paragraph_format.space_before = Pt(2)
    sub_p.paragraph_format.space_after = Pt(6)
    sub_p.alignment = WD_ALIGN_PARAGRAPH.CENTER
    s_run = sub_p.add_run("Strategic Career Blueprint: PTIT High-Quality SE, Deep Security & International ERP Solution Architect")
    s_run.font.name = "Arial"; s_run.font.size = Pt(8.5); s_run.italic = True
    s_run.font.color.rgb = RGBColor(70, 85, 100)

    add_callout(
        doc,
        "• Định vị bản thân: Technical ERP Consultant / Solution Architect (Kiến trúc sư giải pháp & can thiệp kernel lõi), tuyệt đối không dừng lại ở cấu hình bấm nút nghiệp vụ thông thường.\n"
        "• 3 Trụ cột di sản PTIT: standalone-policy-engine (< 0.35ms Engine) + secure-multitenant-saas (PostgreSQL RLS & WORM) + secure-fapi-zta-darkservices (FAPI 2.0 & OpenZiti).\n"
        "• Thị trường mục tiêu: Trung tâm tài chính Phnom Penh (Hệ thống thanh toán NBC Bakong / Banking ERP) → Global Enterprise.",
        title="TỔNG QUAN CHIẾN LƯỢC: LẤY KỸ THUẬT LÀM LÕI, LẤY NGHIỆP VỤ LÀM VŨ KHÍ"
    )

    # GIAI ĐOẠN 1
    format_heading(doc.add_paragraph(), "Giai Đoạn 1: Xây Móng Kỹ Thuật & Khắc Phục Điểm Mù (Năm 3 Kỳ 1 - Hiện Tại)", level=1)
    format_bullet(doc, "Môn bắt buộc tại PTIT CLC: ", "Cơ sở dữ liệu (3NF, ACID Transactions), Hệ điều hành & Mạng máy tính (Process/Thread, Virtual Memory, TCP/IP, REST), An toàn thông tin cơ sở.")
    format_bullet(doc, "Thực hành Odoo Foundation: ", "Cài đặt Odoo 17 trên Docker + PostgreSQL. Trải nghiệm nghiệp vụ 3 app cốt lõi (Sales, Inventory, Accounting) và cấu trúc module Odoo chuẩn.")
    format_bullet(doc, "Triệt tiêu điểm mù: ", "Soi trực tiếp các bảng PostgreSQL (sale_order, stock_picking, res_partner) qua DBeaver/pgAdmin khi AI sinh code. Chuyển toàn bộ tài liệu & prompt sang tiếng Anh 100%.")

    # GIAI ĐOẠN 2
    format_heading(doc.add_paragraph(), "Giai Đoạn 2: Làm Chủ Odoo & Tích Hợp Hệ Thống (Năm 3 Kỳ 2)", level=1)
    format_bullet(doc, "Kiến thức kỹ thuật cốt lõi: ", "Odoo ORM chuyên sâu (Python Decorators, create, write), Hệ thống phân quyền ir.model.access.csv và Record Rules (ir.rule) tương ứng với PostgreSQL RLS.")
    format_bullet(doc, "Dự án then chốt: ", "Tự xây dựng Custom Module 'custom_approval_and_branch_isolation' tích hợp với Standalone Go PDP Engine để đánh giá quyền hạn mức thời gian thực.")
    format_bullet(doc, "Điểm mù cần tránh: ", "Tuyệt đối không xóa đơn hàng đã xuất kho hoặc thanh toán trong DB. Luôn xử lý bằng cơ chế hủy chứng từ hoặc tạo bút toán đảo để bảo vệ kế toán kép.")

    # GIAI ĐOẠN 3
    format_heading(doc.add_paragraph(), "Giai Đoạn 3: Phân Chuyên Ngành SE, Nạp Tư Duy SAP & Khóa Luận (Năm 4)", level=1)
    format_bullet(doc, "Chuyên ngành tại PTIT: ", "Kỹ thuật Phần mềm (SE) - Tập trung cao độ môn Kiến trúc & Thiết kế phần mềm, Phát triển phần mềm an toàn và Hệ thống phân tán.")
    format_bullet(doc, "Chuẩn tư duy SAP Enterprise: ", "Học 3 phân hệ cốt lõi SAP MM (P2P), SAP SD (O2C), SAP FI (General Ledger, Dr/Cr). Nắm vững giao thức SAP OData (RESTful) và triết lý SAP Clean Core.")
    format_bullet(doc, "Đồ án tốt nghiệp xuất sắc: ", "Xây dựng cổng trung gian Middleware Zero Trust đồng bộ dữ liệu giữa Odoo 17 (Chi nhánh) và Mock SAP S/4HANA OData, phân quyền PDP 1M+ RPS.")
    format_bullet(doc, "Điểm mù cần tránh: ", "Không học vẹt mã T-Code SAP. Tập trung hiểu bản chất luồng dữ liệu và máy trạng thái (State Machine) trong Database.")

    # GIAI ĐOẠN 4
    format_heading(doc.add_paragraph(), "Giai Đoạn 4: Thực Tập Cùng Mentor & Chuẩn Bị Xuất Ngoại (Học Kỳ Cuối & Ra Trường)", level=1)
    format_bullet(doc, "Làm việc với Mentor: ", "Chủ động khai thác bài toán công nghệ tại Campuchia (Ngân hàng, Bán lẻ, Odoo/SAP), tiếp cận tài liệu BRD và thiết kế kỹ thuật thực tế.")
    format_bullet(doc, "Hành trang thị trường Phnom Penh: ", "Năng lực làm việc và debug độc lập, kỹ năng giao tiếp tiếng Anh thương mại, tích hợp API thanh toán quốc gia Bakong (NBC) qua chuẩn FAPI 2.0.")
    format_bullet(doc, "Hồ sơ năng lực (Portfolio): ", "CV định vị Kỹ sư SE Chất lượng cao từ PTIT, am hiểu sâu kiến trúc Odoo/PostgreSQL, chuẩn tích hợp SAP và hạ tầng bảo mật Zero Trust.")

    # BẢNG ĐỐI CHIẾU KIẾN THỨC
    format_heading(doc.add_paragraph(), "Bảng Đối Chiếu Kiến Thức Bắt Buộc & Kế Thừa Nền Tảng", level=1)
    
    tbl = doc.add_table(rows=5, cols=4)
    tbl.alignment = WD_TABLE_ALIGNMENT.CENTER
    tbl.autofit = False
    
    col_w = [Inches(1.3), Inches(1.8), Inches(1.8), Inches(1.6)]
    headers = ["Trụ Cột", "Nền Tảng Đã Có (PTIT)", "Kiến Thức ERP Bắt Buộc", "Điểm Mù Thường Gặp"]
    
    hdr = tbl.rows[0]
    for i, title in enumerate(headers):
        cell = hdr.cells[i]; cell.width = col_w[i]
        set_cell_background(cell, "1E3A5F")
        set_cell_margins(cell, 35, 35, 50, 50)
        p = cell.paragraphs[0]; r = p.add_run(title)
        r.font.name = "Arial"; r.font.size = Pt(8); r.bold = True; r.font.color.rgb = RGBColor(255, 255, 255)
        
    data = [
        ("Lập Trình & Ngôn Ngữ", "Go, TypeScript, C++, Python cơ bản", "Python chuyên sâu Odoo ORM, Decorators, Inheritance", "Chỉ biết prompt AI mà không tự trace stack trace khi lỗi."),
        ("Cơ Sở Dữ Liệu", "PostgreSQL RLS, B-Tree, NoSQL", "PostgreSQL Transaction, Locks, Query Optimization, Foreign Keys", "Không kiểm soát được deadlock hoặc làm sai lệch tồn kho."),
        ("Bảo Mật & Phân Quyền", "Zero Trust, mTLS, DPoP, PBAC/ABAC", "Odoo ACL, Record Rules, SAP GRC Access Control (SoD)", "Nhầm lẫn giữa quyền UI và quyền dữ liệu tầng CSDL."),
        ("Tư Duy Nghiệp Vụ", "Tư duy kỹ thuật, xử lý phân tán", "Luồng kinh doanh chuẩn: P2P, O2C, Kế toán kép (Dr/Cr)", "Nghĩ ERP chỉ là CRUD đơn giản mà quên tính toàn vẹn tài chính.")
    ]
    for r_idx, row_d in enumerate(data):
        row = tbl.rows[r_idx + 1]
        bg = "F7FAFC" if r_idx % 2 == 0 else "FFFFFF"
        for c_idx, val in enumerate(row_d):
            cell = row.cells[c_idx]; cell.width = col_w[c_idx]
            set_cell_background(cell, bg)
            set_cell_margins(cell, 30, 30, 45, 45)
            p = cell.paragraphs[0]; r = p.add_run(val)
            r.font.name = "Arial"; r.font.size = Pt(7.5)
            if c_idx == 0: r.bold = True
            r.font.color.rgb = RGBColor(30, 40, 50)

    # Save documents
    out_dir = r"e:\Projects\Project_TN\standalone-policy-engine\docs\career-roadmap"
    docx_path = os.path.join(out_dir, "LO_TRINH_SE_ERP_AI_VIBE_CODING.docx")
    doc.save(docx_path)
    print(f"DOCX created at: {docx_path}")

    pdf_path = os.path.join(out_dir, "LO_TRINH_SE_ERP_AI_VIBE_CODING.pdf")
    try:
        word = win32com.client.Dispatch("Word.Application")
        word.Visible = False
        doc_com = word.Documents.Open(os.path.abspath(docx_path))
        doc_com.ExportAsFixedFormat(os.path.abspath(pdf_path), 17)
        doc_com.Close(False)
        word.Quit()
        print(f"PDF exported at: {pdf_path}")
    except Exception as e:
        print(f"Error exporting PDF: {e}")

    dst_dir = os.path.expanduser(r'~\Downloads')
    for f in [docx_path, pdf_path]:
        if os.path.exists(f):
            try:
                shutil.copy2(f, os.path.join(dst_dir, os.path.basename(f)))
                print(f"Copied {os.path.basename(f)} to Downloads")
            except PermissionError:
                alt_name = "LO_TRINH_SE_ERP_AI_2029.pdf" if f.endswith(".pdf") else "LO_TRINH_SE_ERP_AI_2029.docx"
                shutil.copy2(f, os.path.join(dst_dir, alt_name))
                print(f"Copied to Downloads as fallback: {alt_name}")

if __name__ == "__main__":
    main()
