#!/usr/bin/env python3
"""news-visual-story 分镜 JSON 检查脚本。

用法：
    python scripts/check_storyboard.py <storyboard.json>

只做三件事：
1. 字段齐不齐、枚举值和序号格式对不对；
2. Prompt 六个块是否按固定块与文字槽精确拼装；
3. 逐帧统计文字量，帧数与字数超出软目标只提醒。

不判断内容是否忠于原文，不判断措辞好坏。那是透镜里人工检查的事。
退出码：0 PASS（可带 WARN）、1 FAIL、2 文件读不了。
"""
from __future__ import annotations

import argparse
import json
import re
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ASSETS = ROOT / "assets"

SKILL_NAME = "news-visual-story"
SCHEMA_VERSION = 1

# 枚举值，与 flows/storyboard.md 对应表保持一致
QUESTIONS = {"what", "insight", "why", "how", "so-what"}
RELATIONS = {"对比", "流程", "分支", "边界", "变化", "层级", "场景"}
GLOSS_BASIS = {"source", "common"}
OMIT_KINDS = {"supplementary", "counter", "hedge"}

# 字数软目标：超出只提醒
SOFT_SLOT = {"headline": 40, "summary": 130, "deck": 60, "footnote": 48, "entity": 24, "source": 24}
SOFT_GLOSS = 30
SOFT_LABEL = 16
SOFT_VALUE = 20
SOFT_FRAME_TOTAL = 160
SOFT_FRAME_TOTAL_WITH_SUMMARY = 260
# 合计只算内容槽，不算日期、机构、系列、来源这四个固定槽
CONTENT_SLOTS = ("headline", "summary", "deck", "footnote")
MAX_GLOSS = 2
FRAME_COUNT_RANGE = (1, 5)

BLOCK_TAGS = ["STYLE — fixed", "LAYOUT — fixed", "STORY", "TEXT", "AVOID — frame", "AVOID — fixed"]
BLOCK_FILES = {"STYLE — fixed": "style.txt", "LAYOUT — fixed": "layout.txt", "AVOID — fixed": "avoid.txt"}
TEXT_HEAD = (
    "Render exactly these strings in a bold, clean sans-serif suitable for Chinese; "
    "no other text anywhere in the image:"
)


@dataclass
class Report:
    """检查结果：失败项、提醒项、逐帧字数。"""

    fails: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    counts: dict[str, dict[str, int]] = field(default_factory=dict)

    def fail(self, message: str) -> None:
        self.fails.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)


# ---------- 基础工具 ----------

def char_count(value: str | None) -> int:
    """字数：非空白字符算 1，连续英文字母串算 2，空格不算。"""
    if not value:
        return 0
    return sum(2 if re.fullmatch(r"[A-Za-z]+", tok) else 1 for tok in re.findall(r"[A-Za-z]+|\S", value))


def load_block(name: str) -> tuple[str, str]:
    """读取固定块文件，返回（标签，正文）。"""
    text = (ASSETS / name).read_text(encoding="utf-8").strip()
    head, sep, body = text.partition("\n")
    if not sep:
        raise ValueError(f"固定块 {name} 缺少正文")
    return head.strip("[]").strip(), body.strip()


def load_render_contract() -> dict:
    return json.loads((ASSETS / "render_contract.json").read_text(encoding="utf-8"))


def build_text_block(text: dict) -> str:
    """按文字槽拼 [TEXT] 块，空槽位整行省略。"""
    lines = [
        TEXT_HEAD,
        f'- Date badge (top-left): "{text.get("date", "")}"',
        f'- Entity line (top-left): "{text.get("entity", "")}"',
        f'- Series marker (top-left): "{text.get("series", "")}"',
        f'- Headline (top-left, one or two lines): "{text.get("headline", "")}"',
    ]
    if text.get("summary"):
        lines.append(f'- Summary paragraph (under the headline, up to three lines, smaller than the headline): "{text["summary"]}"')
    if text.get("deck"):
        lines.append(f'- Deck (under the headline or summary): "{text["deck"]}"')
    labels = text.get("labels") if isinstance(text.get("labels"), list) else []
    if labels:
        lines.append("- Labels (on or beside the elements they name): " + ", ".join(f'"{x}"' for x in labels))
    values = text.get("values") if isinstance(text.get("values"), list) else []
    if values:
        lines.append("- Value labels (beside the elements they describe): " + ", ".join(f'"{x}"' for x in values))
    gloss = text.get("gloss") if isinstance(text.get("gloss"), list) else []
    if gloss:
        rendered = ", ".join(f'"{g.get("term", "")}：{g.get("gloss", "")}"' for g in gloss if isinstance(g, dict))
        lines.append("- Glossary notes (small, beside the related element): " + rendered)
    if text.get("footnote"):
        lines.append(f'- Footnote (bottom-left): "{text["footnote"]}"')
    lines.append(f'- Source tag (bottom-right): "{text.get("source", "")}"')
    return "\n".join(lines)


def build_story_block(frame: dict) -> str:
    visual = frame.get("visual") if isinstance(frame.get("visual"), dict) else {}
    return "\n".join([
        f'Claim: {frame.get("claim", "")}',
        f'Relation: {visual.get("relation", "")}',
        f'Composition: {visual.get("composition", "")}',
        f'Encoding: {visual.get("encoding", "")}',
    ])


def build_avoid_block(items: list) -> str:
    cleaned = [str(x).strip().removeprefix("No ").removeprefix("no ").rstrip(".;") for x in items if str(x).strip()]
    return "No " + "; no ".join(cleaned) + "." if cleaned else "No additional avoid items for this frame."


def build_prompt(frame: dict) -> str:
    """从一帧的字段拼出完整 Prompt。测试与外部构建脚本共用。"""
    style_head, style_body = load_block("style.txt")
    layout_head, layout_body = load_block("layout.txt")
    avoid_head, avoid_body = load_block("avoid.txt")
    avoid_items = frame.get("avoid") if isinstance(frame.get("avoid"), list) else []
    return "\n\n".join([
        f"[{style_head}]\n{style_body}",
        f"[{layout_head}]\n{layout_body}",
        f"[STORY]\n{build_story_block(frame)}",
        f"[TEXT]\n{build_text_block(frame.get('text') if isinstance(frame.get('text'), dict) else {})}",
        f"[AVOID — frame]\n{build_avoid_block(avoid_items)}",
        f"[{avoid_head}]\n{avoid_body}",
    ])


def parse_blocks(prompt: object) -> tuple[dict[str, str], list[str]]:
    """把 Prompt 按 [标签] 行切成块。接受 CRLF，统一按 LF 处理。"""
    errors: list[str] = []
    if not isinstance(prompt, str):
        return {}, ["prompt 必须是字符串"]
    prompt = prompt.replace("\r\n", "\n")
    matches = list(re.finditer(r"(?m)^\[([^\]\n]+)\][ \t]*$", prompt))
    if not matches:
        return {}, ["prompt 没有任何 [BLOCK] 标签"]
    if prompt[: matches[0].start()].strip():
        errors.append("prompt 在第一个块之前含额外文本")
    blocks: dict[str, str] = {}
    for idx, match in enumerate(matches):
        tag = match.group(1).strip()
        start = match.end()
        end = matches[idx + 1].start() if idx + 1 < len(matches) else len(prompt)
        if tag in blocks:
            errors.append(f"prompt 块重复: [{tag}]")
        else:
            blocks[tag] = prompt[start:end].strip()
    return blocks, errors


# ---------- 取值辅助 ----------

def as_dict(value: object, label: str, report: Report) -> dict:
    if not isinstance(value, dict):
        report.fail(f"{label} 必须是对象")
        return {}
    return value


def as_list(value: object, label: str, report: Report) -> list:
    if not isinstance(value, list):
        report.fail(f"{label} 必须是数组")
        return []
    return value


def nonempty_str(value: object, label: str, report: Report) -> str:
    if not isinstance(value, str) or not value.strip():
        report.fail(f"{label} 必须是非空字符串")
        return ""
    return value.strip()


def str_or_null(value: object, label: str, report: Report) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str):
        report.fail(f"{label} 必须是字符串或 null")
        return None
    return value


def str_list(value: object, label: str, report: Report) -> list[str]:
    items = as_list(value, label, report)
    out: list[str] = []
    for i, item in enumerate(items):
        if not isinstance(item, str) or not item.strip():
            report.fail(f"{label}[{i}] 必须是非空字符串")
            continue
        out.append(item.strip())
    return out


def in_enum(value: object, allowed: set[str], label: str, report: Report) -> bool:
    if not isinstance(value, str) or value not in allowed:
        report.fail(f"{label} 非法: {value!r}，允许 {sorted(allowed)}")
        return False
    return True


# ---------- 各段校验 ----------

def validate_top(doc: dict, report: Report) -> str:
    """顶层字段。返回 MM.DD 供各帧 date 比对。"""
    if doc.get("skill") != SKILL_NAME:
        report.fail(f"skill 必须为 {SKILL_NAME}")
    if doc.get("schema_version") != SCHEMA_VERSION:
        report.fail(f"schema_version 必须为 {SCHEMA_VERSION}")
    date = doc.get("date")
    if not isinstance(date, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", date):
        report.fail("date 必须是 YYYY-MM-DD")
        mmdd = "??.??"
    else:
        mmdd = date[5:7] + "." + date[8:10]
    if doc.get("render_contract") != load_render_contract():
        report.fail("render_contract 必须与 assets/render_contract.json 精确一致")
    return mmdd


def validate_story(doc: dict, report: Report) -> dict:
    story = as_dict(doc.get("story"), "story", report)
    sid = nonempty_str(story.get("id"), "story.id", report)
    if sid and not re.fullmatch(r"[a-z0-9][a-z0-9-]*", sid):
        report.fail("story.id 只能用小写字母、数字和连字符")
    for key in ("entity", "source_title"):
        nonempty_str(story.get(key), f"story.{key}", report)
    for key in ("who", "what", "why", "insight"):
        if key not in story:
            report.fail(f"story.{key} 必须存在，取不到时写 null")
        elif story.get(key) is not None:
            nonempty_str(story.get(key), f"story.{key}", report)
    str_list(story.get("source_refs", []), "story.source_refs", report)
    return story


def validate_reading(doc: dict, report: Report) -> set[str]:
    """阅读层。返回术语表里的 term 集合，供各帧 gloss 比对。"""
    reading = as_dict(doc.get("reading"), "reading", report)
    terms: set[str] = set()
    for i, raw in enumerate(as_list(reading.get("glossary", []), "reading.glossary", report)):
        item = as_dict(raw, f"reading.glossary[{i}]", report)
        term = nonempty_str(item.get("term"), f"reading.glossary[{i}].term", report)
        nonempty_str(item.get("gloss"), f"reading.glossary[{i}].gloss", report)
        in_enum(item.get("basis"), GLOSS_BASIS, f"reading.glossary[{i}].basis", report)
        if term:
            terms.add(term)
    for i, raw in enumerate(as_list(reading.get("omitted", []), "reading.omitted", report)):
        item = as_dict(raw, f"reading.omitted[{i}]", report)
        nonempty_str(item.get("text"), f"reading.omitted[{i}].text", report)
        in_enum(item.get("kind"), OMIT_KINDS, f"reading.omitted[{i}].kind", report)
        nonempty_str(item.get("reason"), f"reading.omitted[{i}].reason", report)
    str_or_null(reading.get("order_note"), "reading.order_note", report)
    return terms


def validate_text(text: dict, label: str, mmdd: str, entity: str, series: str, has_what: bool, terms: set[str], report: Report) -> dict[str, int]:
    """文字槽格式与字数统计。返回本帧各槽字数。"""
    counts: dict[str, int] = {}
    if text.get("date") != mmdd:
        report.fail(f"{label} text.date 必须为 {mmdd}")
    if text.get("entity") != entity:
        report.fail(f"{label} text.entity 必须与 story.entity 一致")
    if text.get("series") != series:
        report.fail(f"{label} text.series 应为 {series!r}")
    for key in ("headline", "source"):
        nonempty_str(text.get(key), f"{label} text.{key}", report)
    summary = str_or_null(text.get("summary"), f"{label} text.summary", report)
    if summary and not has_what:
        report.fail(f"{label} summary 只允许出现在含 what 的帧")
    str_or_null(text.get("deck"), f"{label} text.deck", report)
    str_or_null(text.get("footnote"), f"{label} text.footnote", report)
    labels = str_list(text.get("labels", []), f"{label} text.labels", report)
    values = str_list(text.get("values", []), f"{label} text.values", report)
    gloss_items = as_list(text.get("gloss", []), f"{label} text.gloss", report)
    if len(gloss_items) > MAX_GLOSS:
        report.fail(f"{label} gloss 超过 {MAX_GLOSS} 条")
    gloss_total = 0
    for i, raw in enumerate(gloss_items):
        item = as_dict(raw, f"{label} text.gloss[{i}]", report)
        term = nonempty_str(item.get("term"), f"{label} text.gloss[{i}].term", report)
        gloss = nonempty_str(item.get("gloss"), f"{label} text.gloss[{i}].gloss", report)
        if term and term not in terms:
            report.warn(f"{label} gloss 术语 {term!r} 不在 reading.glossary 里")
        n = char_count(gloss)
        gloss_total += n + char_count(term)
        if n > SOFT_GLOSS:
            report.warn(f"{label} gloss {term!r} 解释 {n} 字，软目标 {SOFT_GLOSS}")

    # 逐槽字数：固定槽只单独提醒，不计入合计
    for key in ("date", "entity", "series", "headline", "summary", "deck", "footnote", "source"):
        val = text.get(key)
        n = char_count(val if isinstance(val, str) else None)
        counts[key] = n
        if key in SOFT_SLOT and n > SOFT_SLOT[key]:
            report.warn(f"{label} {key} {n} 字，软目标 {SOFT_SLOT[key]}")
    for x in labels:
        if char_count(x) > SOFT_LABEL:
            report.warn(f"{label} label {x!r} {char_count(x)} 字，软目标 {SOFT_LABEL}")
    for x in values:
        if char_count(x) > SOFT_VALUE:
            report.warn(f"{label} value {x!r} {char_count(x)} 字，软目标 {SOFT_VALUE}")
    counts["labels"] = sum(char_count(x) for x in labels)
    counts["values"] = sum(char_count(x) for x in values)
    counts["gloss"] = gloss_total
    total = sum(counts[k] for k in CONTENT_SLOTS) + counts["labels"] + counts["values"] + counts["gloss"]
    counts["total"] = total
    soft_total = SOFT_FRAME_TOTAL_WITH_SUMMARY if summary else SOFT_FRAME_TOTAL
    if total > soft_total:
        report.warn(f"{label} 内容合计 {total} 字，软目标 {soft_total}")
    return counts


def validate_prompt(frame: dict, label: str, report: Report) -> None:
    """Prompt 六块必须与固定块和字段精确拼装。"""
    blocks, errors = parse_blocks(frame.get("prompt"))
    for error in errors:
        report.fail(f"{label}: {error}")
    if list(blocks) != BLOCK_TAGS:
        report.fail(f"{label} prompt 块顺序/集合必须严格为 {BLOCK_TAGS}")
        return
    for tag, filename in BLOCK_FILES.items():
        _, body = load_block(filename)
        if blocks.get(tag) != body:
            report.fail(f"{label} [{tag}] 必须与 assets/{filename} 正文精确一致")
    if blocks.get("STORY") != build_story_block(frame):
        report.fail(f"{label} [STORY] 必须由 claim 与 visual 三字段精确拼装")
    text = frame.get("text") if isinstance(frame.get("text"), dict) else {}
    if blocks.get("TEXT") != build_text_block(text):
        report.fail(f"{label} [TEXT] 必须按文字槽精确拼装")
    avoid_items = frame.get("avoid") if isinstance(frame.get("avoid"), list) else []
    if blocks.get("AVOID — frame") != build_avoid_block(avoid_items):
        report.fail(f"{label} [AVOID — frame] 与 avoid 不一致")


def validate_frames(doc: dict, story: dict, mmdd: str, terms: set[str], report: Report) -> None:
    frames = as_list(doc.get("frames"), "frames", report)
    n = len(frames)
    if n == 0:
        report.fail("frames 不能为空")
        return
    if not (FRAME_COUNT_RANGE[0] <= n <= FRAME_COUNT_RANGE[1]):
        report.warn(f"帧数 {n}，通常 {FRAME_COUNT_RANGE[0]} 到 {FRAME_COUNT_RANGE[1]} 张")
    entity = story.get("entity") if isinstance(story.get("entity"), str) else ""
    narrative_path: str | None = None
    for i, raw in enumerate(frames):
        frame = as_dict(raw, f"frames[{i}]", report)
        label = f"帧 {i + 1}"
        if frame.get("index") != i + 1:
            report.fail(f"{label} index 必须为 {i + 1}")
        # questions 不是数组时 str_list 已报错，下面的空值与首帧检查不再重复报
        raw_questions = frame.get("questions")
        questions = str_list(raw_questions, f"{label} questions", report)
        if isinstance(raw_questions, list):
            if not questions:
                report.fail(f"{label} questions 不能为空")
            for q in questions:
                in_enum(q, QUESTIONS, f"{label} questions", report)
            if i == 0 and questions:
                starts_with_event = "what" in questions
                starts_with_insight = "insight" in questions
                if starts_with_event == starts_with_insight:
                    report.fail("第一帧 questions 必须且只能含 what 或 insight 其中一个")
                elif starts_with_event:
                    narrative_path = "event"
                    what = story.get("what")
                    if not isinstance(what, str) or not what.strip():
                        report.fail("事件新闻的 story.what 必须是非空字符串")
                else:
                    narrative_path = "insight"
                    insight = story.get("insight")
                    if insight is None or (isinstance(insight, str) and not insight.strip()):
                        report.fail("洞见内容的 story.insight 必须是非空字符串")
            if narrative_path == "event" and "insight" in questions:
                report.fail(f"{label} 事件新闻不能使用 insight")
            if narrative_path == "insight" and "what" in questions:
                report.fail(f"{label} 洞见内容不能使用 what")
        nonempty_str(frame.get("claim"), f"{label} claim", report)
        visual = as_dict(frame.get("visual"), f"{label} visual", report)
        in_enum(visual.get("relation"), RELATIONS, f"{label} visual.relation", report)
        for key in ("composition", "encoding"):
            nonempty_str(visual.get(key), f"{label} visual.{key}", report)
        str_list(frame.get("avoid", []), f"{label} avoid", report)
        text = as_dict(frame.get("text"), f"{label} text", report)
        counts = validate_text(text, label, mmdd, entity, f"{i + 1}/{n}", "what" in questions, terms, report)
        report.counts[str(i + 1)] = counts
        validate_prompt(frame, label, report)


def validate(doc: object) -> Report:
    report = Report()
    if not isinstance(doc, dict):
        report.fail("顶层 JSON 必须是对象")
        return report
    mmdd = validate_top(doc, report)
    story = validate_story(doc, report)
    terms = validate_reading(doc, report)
    validate_frames(doc, story, mmdd, terms, report)
    return report


# ---------- 入口 ----------

def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="news-visual-story 分镜 JSON 检查")
    parser.add_argument("path", type=Path)
    args = parser.parse_args(argv)
    try:
        doc = json.loads(args.path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        print(f"FAIL: 无法读取 JSON: {exc}")
        return 2
    report = validate(doc)
    print("FAIL" if report.fails else "PASS")
    for item in report.fails:
        print("-", item)
    for item in report.warnings:
        print("WARN -", item)
    for idx, counts in report.counts.items():
        print(f"帧 {idx} 内容 {counts.get('total', 0)} 字", "含摘要" if counts.get("summary") else "")
    return 1 if report.fails else 0


if __name__ == "__main__":
    raise SystemExit(main())
