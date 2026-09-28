"""Focused source/export consistency checks; not a substitute for visual QA."""
import re
import unittest
from urllib.parse import unquote
from xml.etree import ElementTree as ET
from zipfile import ZipFile

from pypdf import PdfReader

import generate_master_thesis_proposal as proposal


OVERVIEWS = (
    "README.md",
    "docs/00_MASTER_INDEX.md",
    "docs/thesis-proposal/THESIS_V2_MASTER_PLAN.md",
    "docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md",
    "docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md",
    "docs/thesis-proposal/SE_ERP_CAREER_ROADMAP.md",
    "docs/technical-spec/THESIS_CHAPTER_MAPPING.md",
    "custom_addons/pdp_authorizer/README.md",
)
W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"


def compact(text):
    return re.sub(r"\s+", "", text)


class ProposalChecks(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.blocks = proposal.parse_blocks(proposal.SOURCE.read_text(encoding="utf-8"))

    def test_locked_titles_across_overviews(self):
        titles = [proposal.title_from(self.blocks, label)
                  for label in ("Tên tiếng Việt", "Tên tiếng Anh")]
        for path in OVERVIEWS:
            text = (proposal.ROOT / path).read_text(encoding="utf-8")
            for title in titles:
                with self.subTest(path=path, title=title):
                    self.assertIn(title, text)

    def test_local_links(self):
        for name in (*OVERVIEWS, "ACTIVE_TASK.md", "docs/thesis-proposal/THESIS_V2_TASK_BOARD.md"):
            path = proposal.ROOT / name
            for _, link in proposal.LINK.findall(path.read_text(encoding="utf-8")):
                if "://" in link or link.startswith("#"):
                    continue
                target = unquote(link.split("#", 1)[0])
                with self.subTest(path=name, link=target):
                    self.assertTrue((path.parent / target).exists())

    def test_supported_parser_and_rejections(self):
        self.assertEqual(proposal.parse_blocks("# Heading\n\n- Item\n\n1. Next"),
                         [("h1", "Heading"), ("list", "• Item"), ("list", "1. Next")])
        self.assertEqual(proposal.parse_blocks("| A | B |\n|---|---|\n| C | D |"),
                         [("table", [["A", "B"], ["C", "D"]])])
        for source in ("```text\nunsupported\n```", "| A | B |\n| C |"):
            with self.subTest(source=source), self.assertRaises(ValueError):
                proposal.parse_blocks(source)

    def test_all_source_blocks_in_both_exports(self):
        with ZipFile(proposal.SOURCE.with_suffix(".docx")) as archive:
            root = ET.fromstring(archive.read("word/document.xml"))
        word = compact(" ".join(node.text or "" for node in root.iter(W + "t")))
        pdf = PdfReader(proposal.SOURCE.with_suffix(".pdf"))
        pdf_text = compact(" ".join(page.extract_text() for page in pdf.pages))
        for kind, body in self.blocks:
            texts = [cell for row in body for cell in row] if kind == "table" else [body]
            for text in texts:
                expected = compact(proposal.visible(text))
                for label, output in (("DOCX", word), ("PDF", pdf_text)):
                    with self.subTest(format=label, text=text[:60]):
                        self.assertIn(expected, output)
        self.assertEqual(len(list(root.iter(W + "tbl"))), 3)
        links = [annotation.get_object().get("/A", {}).get("/URI", "")
                 for page in pdf.pages for annotation in page.get("/Annots", [])]
        self.assertGreaterEqual(sum(link.startswith("https://") for link in links), 7)


if __name__ == "__main__":
    unittest.main()
