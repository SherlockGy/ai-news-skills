# 维护透镜：改规则、固定块、脚本

## 适用

- 改本 skill 的规则、字段、字数目标、术语解释尺度、每张图的内容顺序。
- 改 `assets/` 固定块或渲染契约。
- 改 `scripts/check_storyboard.py` 及其测试。
- 改触发描述。

进入本透镜后先确认维护类型，再动文件。改结构前通读被改文件全文，不做局部替换。

## Step 1 确认维护类型

| 维护类型 | 先读 | 改哪里 | 必须同步 |
|---|---|---|---|
| 改分镜规则 | `flows/storyboard.md` 全文 | Step 2 归属表指定的那一处 | 若复核清单也涉及，同步 `flows/render.md` |
| 改复核项或返修表 | `flows/render.md` 全文 | Step 3、Step 4 | 若能在分镜期预防，同步 `flows/storyboard.md` |
| 改字数软目标 | 脚本常量 | 脚本 `SOFT_*` 与 `flows/storyboard.md` Step 5 表 | 两处数值一致 |
| 改文字槽位 | 脚本 `build_text_block` | 脚本、`flows/storyboard.md` Step 5 与 Step 6 | 三处一致；测试里的 `make_frame` 同步 |
| 改固定块 | 三个 txt 与 `render_contract.json` | `assets/` | 脚本读文件比对，不需改脚本；已有分镜 JSON 的 prompt 需重新拼装 |
| 改枚举值 | 脚本常量 | 脚本 `QUESTIONS`、`RELATIONS`、`OMIT_KINDS` 与透镜对应表 | 两处一致 |
| 加透镜 | `SKILL.md` 意图表 | `flows/<lens>.md` 与意图表新行 | 透镜内重复硬约束，结尾写下一步 |
| 调 description | `SKILL.md` frontmatter 与意图表 | 只改触发词 | 触发词与意图表信号一致 |

确认后回复一句「走维护，类型：<上表哪一行>；先读 <文件>」。

## Step 2 规则单一归属表

每条规则只在一处维护。改任一处前先查本表，再同步副本。

| 规则 | 归属 | 有意重复的副本 |
|---|---|---|
| 一次一条新闻 | `SKILL.md` 行为覆写 | `flows/storyboard.md` 硬约束与 Step 1 |
| 忠实的定义、省略可记 | `SKILL.md` 边界 | `flows/storyboard.md` 硬约束 |
| 内容按解释价值取舍、语气性兜底不上图、事实状态随主句表达 | `SKILL.md` 边界 | `flows/storyboard.md` 硬约束、Step 2 与 Step 5 |
| 省略清单的登记与渲染执行 | `flows/storyboard.md` Step 2 | `flows/storyboard.md` Step 8；`flows/render.md` 清单第 7 项与 Step 4 |
| 阅读层字段与取法 | `flows/storyboard.md` Step 2 | 脚本 `validate_story`、`validate_reading` |
| 内容类型的叙述路径、缺失信息处理、每张图的内容顺序与张数 | `flows/storyboard.md` Step 2、Step 3 | 脚本 `QUESTIONS`、路径检查与帧数提醒；`flows/render.md` 40 秒测试 |
| 关系类型表 | `flows/storyboard.md` Step 4 | 脚本 `RELATIONS` |
| 图文叙事要求、方向一致、不用长度编码数值、图标失真 | `flows/storyboard.md` Step 4 | `flows/render.md` 清单第 4、5、6 项 |
| 文字槽写法与软目标 | `flows/storyboard.md` Step 5 | 脚本 `SOFT_*`、`build_text_block` |
| 术语解释尺度 | `SKILL.md` 边界 | `flows/storyboard.md` Step 2 与 Step 5 |
| Prompt 六块与 TEXT 拼法 | `flows/storyboard.md` Step 6 | 脚本 `build_prompt` 系列函数 |
| 固定块正文 | `assets/*.txt` | 无，脚本读文件比对 |
| JSON 结构 | `flows/storyboard.md` Step 7 | 脚本 |
| 复核清单与返修表 | `flows/render.md` Step 3、4 | 无 |
| 交付目录约定 | `flows/render.md` Step 1 | 无 |
| 40 秒测试 | `flows/render.md` Step 5 | `SKILL.md` 行为覆写第一句；叙述路径与 `flows/storyboard.md` Step 3 一致 |

## Step 3 新增规则的放置判断

新增一条约束前先回答：

- 它改变透镜选择或全局边界吗：是则放 `SKILL.md`，并在相关透镜重复。
- 它影响分镜 JSON 怎么写吗：是则放 `flows/storyboard.md` 对应 Step。
- 它只能在看到图片后判断吗：是则放 `flows/render.md` 清单，并考虑能否在分镜期预防。
- 它只是 Prompt 或 JSON 格式说明吗：
  - 是则只写字段引用、类型和固定值，不写具体新闻内容。
  - 完整实例只用于测试，不放进执行透镜。
- 它是历史踩坑、测试记录、设计取舍吗：是则不进透镜，写进 `CHANGELOG.md` 或提交说明。

放置后检查两件事：规则文字用本 skill 的语境写，如「兜底句不上图」而不是「注意措辞」；规则是硬还是软要写明，软的用「建议」，硬的用「必须」。管画了什么不能错的才硬，管怎么画的都软。

## Step 4 脚本边界

脚本只做三件事：字段齐不齐、枚举和序号格式对不对、Prompt 六块是否精确拼装。字数逐帧统计并对超软目标提醒。

不做的事，也不要加：

- 不判断内容是否指向原文。
- 不判断措辞好坏、有没有兜底句、有没有评价词。
- 不判断图片。
- 不建叠字工具。

改脚本后运行测试：

```bash
python scripts/test_check_storyboard.py
```

全部通过才算改完。

## Step 5 待扩展清单

| 项 | 触发条件 | 落点 |
|---|---|---|
| 日报总览帧 | 用户要把当天多条新闻放一张总览 | 新透镜或 `flows/storyboard.md` 新 Step；需定义总览的 question 与文字槽 |
| 概念深挖 | 用户要给某个概念做一组解释图，而不是图内一句话 | 新透镜；外部研究需登记来源 |
| 渲染后像素检查脚本 | 交付目录 PNG 多，人工读尺寸麻烦 | 新脚本，只报不拦 |

新增待扩展项时写清触发条件与落点，不写实现细节。

## Step 6 description 校准

- 只写触发条件：对象、典型任务、关键词。
- 不写流程、目录、理念。
- 触发词与 `SKILL.md` 意图表信号列一致，改一边必须改另一边。

## Step 7 自检

维护完成后逐项确认：

- `SKILL.md` 意图表每一行指向存在的文件，每个 `flows/*.md` 都被意图表指向。
- 每个透镜读完可直接执行或指向唯一下一步，没有「可参考」「按需」承载关键路径。
- Step 2 归属表里的副本措辞一致。
- 脚本常量与透镜表一致，测试全过。
- 硬软标注清楚，管怎么画的规则没有写成「必须」。
- description 只含触发条件。
- `CHANGELOG.md` 追加本次条目，只写规则变化。
