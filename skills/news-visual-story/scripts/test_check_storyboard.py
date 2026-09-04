#!/usr/bin/env python3
"""check_storyboard.py 的单元测试。只用标准库。

运行：
    python scripts/test_check_storyboard.py
"""
from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_storyboard as cs  # noqa: E402


def make_frame(index: int, total: int, questions: list[str], *, summary: str | None = None) -> dict:
    """构造一帧合法数据，prompt 用脚本自己的拼装函数生成。"""
    frame = {
        "index": index,
        "questions": questions,
        "claim": f"第 {index} 帧的一句话",
        "text": {
            "date": "09.03",
            "entity": "Google · Gemini 3.8",
            "series": f"{index}/{total}",
            "headline": "Flash 开始承担前沿 Coding Agent",
            "summary": summary,
            "deck": None,
            "labels": ["Gemini 3.8 Flash"],
            "values": [],
            "gloss": [{"term": "Fairwind Program", "gloss": "Google 面向受信防守方的准入计划"}],
            "footnote": None,
            "source": "Google 官方发布",
        },
        "visual": {"relation": "分支", "composition": "一个发布分成两条道", "encoding": "两条道对应两个版本"},
        "avoid": ["price tag as hero object"],
    }
    frame["prompt"] = cs.build_prompt(frame)
    return frame


def make_frames(total: int) -> list[dict]:
    """构造 total 帧：首帧 what，末帧 so-what，中间 how。"""
    frames = []
    for i in range(1, total + 1):
        if i == 1:
            frames.append(make_frame(i, total, ["what", "why"], summary="摘要"))
        elif i == total:
            frames.append(make_frame(i, total, ["so-what"]))
        else:
            frames.append(make_frame(i, total, ["how"]))
    return frames


def make_doc(total: int = 3) -> dict:
    """构造一份合法分镜 JSON，默认 3 帧。"""
    return {
        "skill": "news-visual-story",
        "schema_version": 1,
        "date": "2026-09-03",
        "render_contract": cs.load_render_contract(),
        "story": {
            "id": "gemini-3-8",
            "entity": "Google · Gemini 3.8",
            "source_title": "Google 发布 Gemini 3.8 Flash",
            "who": "Google",
            "what": "Google 发布 Gemini 3.8 Flash 与 Flash Cyber。",
            "why": "Flash 被推向长程 Coding / Agent 主力模型。",
            "insight": "Flash 的副模型定位正在变窄。",
            "source_refs": ["https://blog.google/"],
        },
        "reading": {
            "glossary": [{"term": "Fairwind Program", "gloss": "Google 面向受信防守方的准入计划", "basis": "source"}],
            "omitted": [{"text": "HLE 54.9%", "kind": "supplementary", "reason": "读者不会拿去用"}],
            "order_note": None,
        },
        "frames": make_frames(total),
    }


def rebuild(frame: dict) -> None:
    frame["prompt"] = cs.build_prompt(frame)


class CharCountTest(unittest.TestCase):
    def test_rules(self) -> None:
        self.assertEqual(cs.char_count("中文"), 2)
        self.assertEqual(cs.char_count("Flash 开始"), 4)
        self.assertEqual(cs.char_count("Flash开始"), 4)
        self.assertEqual(cs.char_count("$0.75/1M Token"), 11)
        self.assertEqual(cs.char_count(None), 0)
        self.assertEqual(cs.char_count(""), 0)


class ValidDocTest(unittest.TestCase):
    def test_valid_doc_passes(self) -> None:
        report = cs.validate(make_doc())
        self.assertEqual(report.fails, [])
        self.assertEqual(report.warnings, [])
        self.assertIn("1", report.counts)

    def test_total_excludes_fixed_slots(self) -> None:
        doc = make_doc()
        text = doc["frames"][1]["text"]
        expected = cs.char_count(text["headline"]) + cs.char_count("Gemini 3.8 Flash") + cs.char_count("Fairwind Program") + cs.char_count("Google 面向受信防守方的准入计划")
        report = cs.validate(doc)
        self.assertEqual(report.counts["2"]["total"], expected)

    def test_frame_count_6_warns_only(self) -> None:
        doc = make_doc(6)
        report = cs.validate(doc)
        self.assertEqual(report.fails, [])
        self.assertTrue(any("帧数 6" in w for w in report.warnings))

    def test_frame_count_1_to_5_no_warn(self) -> None:
        for total in (1, 2, 3, 4, 5):
            report = cs.validate(make_doc(total))
            self.assertFalse(any("帧数" in w for w in report.warnings), total)

    def test_insight_null_passes(self) -> None:
        doc = make_doc()
        doc["story"]["insight"] = None
        self.assertEqual(cs.validate(doc).fails, [])

    def test_crlf_prompt_accepted(self) -> None:
        doc = make_doc()
        doc["frames"][0]["prompt"] = doc["frames"][0]["prompt"].replace("\n", "\r\n")
        self.assertEqual(cs.validate(doc).fails, [])


class FieldTest(unittest.TestCase):
    def test_missing_story_what_fails(self) -> None:
        for value in ("", None):
            with self.subTest(value=value):
                doc = make_doc()
                doc["story"]["what"] = value
                self.assertTrue(any("story.what" in f for f in cs.validate(doc).fails))

    def test_bad_story_id_fails(self) -> None:
        doc = make_doc()
        doc["story"]["id"] = "Gemini 3.8"
        self.assertTrue(any("story.id" in f for f in cs.validate(doc).fails))

    def test_insight_wrong_type_fails(self) -> None:
        doc = make_doc()
        doc["story"]["insight"] = 42
        self.assertTrue(any("story.insight" in f for f in cs.validate(doc).fails))

    def test_render_contract_mismatch_fails(self) -> None:
        doc = make_doc()
        doc["render_contract"]["transparent_background"] = True
        self.assertTrue(any("render_contract" in f for f in cs.validate(doc).fails))

    def test_bad_omit_kind_fails(self) -> None:
        doc = make_doc()
        doc["reading"]["omitted"][0]["kind"] = "condition"
        self.assertTrue(any("omitted[0].kind" in f for f in cs.validate(doc).fails))


class FrameTest(unittest.TestCase):
    def test_first_frame_without_narrative_start_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["questions"] = ["how"]
        doc["frames"][0]["text"]["summary"] = None
        rebuild(doc["frames"][0])
        self.assertTrue(any("第一帧" in f for f in cs.validate(doc).fails))

    def test_insight_path_passes_without_so_what(self) -> None:
        doc = make_doc()
        doc["frames"][0]["questions"] = ["insight"]
        doc["frames"][0]["text"]["summary"] = None
        doc["frames"][2]["questions"] = ["how"]
        rebuild(doc["frames"][0])
        rebuild(doc["frames"][2])
        self.assertEqual(cs.validate(doc).fails, [])

    def test_first_frame_cannot_mix_event_and_insight(self) -> None:
        doc = make_doc()
        doc["frames"][0]["questions"] = ["what", "insight"]
        rebuild(doc["frames"][0])
        self.assertTrue(any("只能含 what 或 insight" in f for f in cs.validate(doc).fails))

    def test_insight_path_requires_story_insight(self) -> None:
        doc = make_doc()
        doc["story"]["insight"] = None
        doc["frames"][0]["questions"] = ["insight"]
        doc["frames"][0]["text"]["summary"] = None
        rebuild(doc["frames"][0])
        self.assertTrue(any("story.insight" in f for f in cs.validate(doc).fails))

    def test_optional_story_fields_accept_null(self) -> None:
        doc = make_doc(1)
        doc["story"]["who"] = None
        doc["story"]["what"] = None
        doc["story"]["why"] = None
        doc["frames"][0]["questions"] = ["insight"]
        doc["frames"][0]["text"]["summary"] = None
        rebuild(doc["frames"][0])
        self.assertEqual(cs.validate(doc).fails, [])

    def test_insight_path_can_end_after_core_judgment(self) -> None:
        doc = make_doc(1)
        doc["frames"][0]["questions"] = ["insight"]
        doc["frames"][0]["text"]["summary"] = None
        rebuild(doc["frames"][0])
        self.assertEqual(cs.validate(doc).fails, [])

    def test_event_path_rejects_later_insight(self) -> None:
        doc = make_doc()
        doc["frames"][1]["questions"] = ["insight"]
        rebuild(doc["frames"][1])
        self.assertTrue(any("事件新闻不能使用 insight" in f for f in cs.validate(doc).fails))

    def test_insight_path_rejects_later_what(self) -> None:
        doc = make_doc()
        doc["frames"][0]["questions"] = ["insight"]
        doc["frames"][0]["text"]["summary"] = None
        doc["frames"][1]["questions"] = ["what"]
        rebuild(doc["frames"][0])
        rebuild(doc["frames"][1])
        self.assertTrue(any("洞见内容不能使用 what" in f for f in cs.validate(doc).fails))

    def test_missing_questions_reports_once(self) -> None:
        doc = make_doc()
        del doc["frames"][1]["questions"]
        fails = cs.validate(doc).fails
        self.assertEqual(sum("帧 2 questions" in f for f in fails), 1)

    def test_event_path_can_end_without_so_what(self) -> None:
        doc = make_doc(1)
        self.assertEqual(cs.validate(doc).fails, [])

    def test_summary_on_how_frame_fails(self) -> None:
        doc = make_doc()
        doc["frames"][1]["text"]["summary"] = "不该出现的摘要"
        rebuild(doc["frames"][1])
        self.assertTrue(any("summary 只允许" in f for f in cs.validate(doc).fails))

    def test_three_gloss_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["text"]["gloss"] = [{"term": f"T{i}", "gloss": "x"} for i in range(3)]
        rebuild(doc["frames"][0])
        self.assertTrue(any("gloss 超过" in f for f in cs.validate(doc).fails))

    def test_gloss_term_not_in_glossary_warns(self) -> None:
        doc = make_doc()
        doc["frames"][0]["text"]["gloss"] = [{"term": "未登记术语", "gloss": "解释"}]
        rebuild(doc["frames"][0])
        report = cs.validate(doc)
        self.assertEqual(report.fails, [])
        self.assertTrue(any("不在 reading.glossary" in w for w in report.warnings))

    def test_series_mismatch_fails(self) -> None:
        doc = make_doc()
        doc["frames"][1]["text"]["series"] = "2/9"
        rebuild(doc["frames"][1])
        self.assertTrue(any("series" in f for f in cs.validate(doc).fails))

    def test_date_mismatch_fails(self) -> None:
        doc = make_doc()
        doc["frames"][1]["text"]["date"] = "09.02"
        rebuild(doc["frames"][1])
        self.assertTrue(any("text.date" in f for f in cs.validate(doc).fails))

    def test_entity_mismatch_fails(self) -> None:
        doc = make_doc()
        doc["frames"][1]["text"]["entity"] = "别家"
        rebuild(doc["frames"][1])
        self.assertTrue(any("text.entity" in f for f in cs.validate(doc).fails))

    def test_bad_relation_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["visual"]["relation"] = "漂亮"
        rebuild(doc["frames"][0])
        self.assertTrue(any("relation" in f for f in cs.validate(doc).fails))

    def test_long_headline_warns_not_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["text"]["headline"] = "一" * 60
        rebuild(doc["frames"][0])
        report = cs.validate(doc)
        self.assertEqual(report.fails, [])
        self.assertTrue(any("headline 60" in w for w in report.warnings))

    def test_many_labels_do_not_fail(self) -> None:
        doc = make_doc()
        doc["frames"][0]["text"]["labels"] = [f"标签{i}" for i in range(8)]
        rebuild(doc["frames"][0])
        self.assertEqual(cs.validate(doc).fails, [])


class PromptTest(unittest.TestCase):
    def test_prompt_drift_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["prompt"] = doc["frames"][0]["prompt"].replace("#F6F1E7", "#FFFFFF", 1)
        self.assertTrue(any("STYLE" in f for f in cs.validate(doc).fails))

    def test_prompt_text_block_mismatch_fails(self) -> None:
        doc = make_doc()
        doc["frames"][0]["text"]["headline"] = "改了标题但没重拼"
        self.assertTrue(any("[TEXT]" in f for f in cs.validate(doc).fails))

    def test_prompt_missing_block_fails(self) -> None:
        doc = make_doc()
        prompt = doc["frames"][0]["prompt"]
        doc["frames"][0]["prompt"] = prompt[: prompt.index("[AVOID — fixed]")].rstrip()
        self.assertTrue(any("块顺序/集合" in f for f in cs.validate(doc).fails))

    def test_avoid_block_formats(self) -> None:
        self.assertEqual(cs.build_avoid_block([]), "No additional avoid items for this frame.")
        self.assertEqual(cs.build_avoid_block(["No cash pile", "infinite loop."]), "No cash pile; no infinite loop.")

    def test_text_block_omits_empty_slots(self) -> None:
        text = make_frame(1, 1, ["what"])["text"]
        text["labels"] = []
        text["gloss"] = []
        block = cs.build_text_block(text)
        self.assertNotIn("Labels", block)
        self.assertNotIn("Glossary", block)
        self.assertNotIn("Summary", block)
        self.assertIn("Source tag", block)


class MainTest(unittest.TestCase):
    def _write(self, doc: object) -> Path:
        tmp = tempfile.NamedTemporaryFile("w", suffix=".json", delete=False, encoding="utf-8")
        with tmp:
            json.dump(doc, tmp, ensure_ascii=False)
        self.addCleanup(Path(tmp.name).unlink)
        return Path(tmp.name)

    def test_exit_0_on_pass(self) -> None:
        self.assertEqual(cs.main([str(self._write(make_doc()))]), 0)

    def test_exit_1_on_fail(self) -> None:
        doc = make_doc()
        doc["story"]["what"] = ""
        self.assertEqual(cs.main([str(self._write(doc))]), 1)

    def test_exit_2_on_unreadable(self) -> None:
        self.assertEqual(cs.main([str(Path(tempfile.gettempdir()) / "不存在的文件.json")]), 2)


if __name__ == "__main__":
    unittest.main(verbosity=1)
