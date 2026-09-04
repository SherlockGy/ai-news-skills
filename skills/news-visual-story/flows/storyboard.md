# 叙事分镜透镜：一条新闻到分镜 JSON

## 适用

- 输入是一条已写好的新闻文本，或含多条新闻的日报，用户要配图、可视化、图文叙事、分镜或生图提示词。
- 输入是已有的分镜 JSON，用户要修改后重新检查。

进入本透镜后不回读 `SKILL.md`。本文件包含从原文到分镜 JSON 的全部规则。读完按 Step 1 开始，不得先拼 Prompt。

## 取事实入口

| 何时 | 读什么 | 用途 |
|---|---|---|
| Step 6 拼 Prompt 时，必读 | `assets/style.txt`、`assets/layout.txt`、`assets/avoid.txt` | 逐字复制正文，不改一字 |
| Step 6 写 `render_contract` 时，必读 | `assets/render_contract.json` | 逐字复制 |

这些是取事实，不是并读前置。JSON 结构以 Step 7 为准。

## 硬约束

- 一次只做一条新闻。
- 画上去的每句话能回到原文。原文的主判断不被换掉。
- 摘要和洞见用原文现成的句子。
- 内容按解释价值取舍，不按正面或负面取舍。只调整语气的兜底表达不上图，决定内容性质的事实状态随主句表达。
- 不为画面方便改变原文说了什么。
- 脚本只查格式。内容对不对由本透镜的规则和 Step 8 的人工检查负责。

## Step 1 锁定一条新闻

- 输入只有一条新闻：进入 Step 2。
- 输入含多条新闻：把每条的标题列成清单，问用户做哪一条。用户说全做，也按顺序一条一条来，每条走完本透镜和生图与复核透镜再开始下一条。
- 用户只给了零散线索没有成文：先问有没有原文。没有就停下，本 skill 不替用户写新闻。

回复一句「本次做：<新闻标题>」。

## Step 2 写阅读层

分镜前先确定这条内容围绕什么展开：

| 内容 | 判断依据 | 第一张 |
|---|---|---|
| 事件新闻 | 全文围绕一次新发生的事实展开 | 发生了什么 |
| 洞见内容 | 全文围绕一个可独立理解的判断展开，事件和案例只是材料 | 核心判断 |

第一张负责让读者进入正确上下文。后续从原文中选择最能增加理解的事实和关系，不要求都直接解释第一张。两边同样重要时问用户，不自行合并路径。

选定后回复一句「叙述路径：<事件新闻或洞见内容>；起点：<发生了什么或核心判断>」。再从原文提取下面的信息，答案优先抄原文自己的句子。

| 字段 | 事件新闻 | 洞见内容 | 取不到时 |
|---|---|---|---|
| `who` | 标题或摘要里的主体 | 提出判断或提供实践依据的主要作者或机构 | 写 `null` |
| `what` | 原文的一句话摘要，原样 | 促成判断的事实、问题或实践背景 | 事件新闻不能确立起点，按下方缺失信息处理；洞见内容写 `null` |
| `why` | 原文的重要性说明、开场里的定位句、「这意味着」 | 判断为什么成立、解决什么问题 | 写 `null`，不补写理由 |
| `insight` | 原文的「判断」段、结尾的判断句 | 标题、「判断」段或结尾里的核心判断 | 事件新闻写 `null`，有 `why` 时可用它收尾；洞见内容不能确立起点，按下方缺失信息处理 |

字段取不到时，不拿相关性弱的原文段落填空，也不为补齐结构自行推导。先判断剩余内容能否独立讲清核心信息：

- 能讲清：保留 `null`，只安排有内容价值的帧，一两帧也可以。
- 讲不清：在安排分镜前告诉用户「原文缺少 <具体信息>，会影响 <具体理解>」，询问是补充调查，还是按现有信息保留到这里。用户明确选择前，不自行调查，也不自行决定继续或舍弃。

用户选择补充调查后，按其要求执行；未指定调查方式时，自行调查并记录来源，将结果作为本次原文的补充材料后继续分镜。

再做三件事：

1. 术语表 `glossary`。把图上会出现、产品经理看不懂的名称和缩写列出来，每个一句话解释。原文解释过的标 `basis: source`，用常识解释的标 `basis: common`。拿不准含义的不解释也不上图。
2. 省略清单 `omitted`。原文里不上图的内容逐条登记，标类型和一句理由：
   - `supplementary`：论据、数据细节、旁证、社区反响、个人实测、使用建议。
   - `counter`：限制、代价、涨价、截止日期等反向信息。
   - `hedge`：只调整语气的免责、待验证、不可外推、不能代替。
3. 顺序说明 `order_note`。分镜顺序与原文段落顺序不同时写一句为什么，相同写 `null`。

限制、代价、边界等信息与其他内容按同一标准取舍，不因正面或负面决定去留。只调整语气的兜底表达不上图。决定内容性质的事实状态随对应 `claim` 表达，不单独做提醒。

## Step 3 安排每张图讲什么

按 Step 2 选定的路径安排。两条路径不能混用起点。

事件新闻：

| question | 读者在问 | 张数 | 文字主体 |
|---|---|---|---|
| `what` | 发生了什么 | 1 张，第一张 | headline 一句，summary 放原文摘要 |
| `why` | 为什么重要 | 原文有时并入第一张的 deck | `why` 那句 |
| `how` | 怎么回事 | 0 到 3 张，只画有关系可画的 | 每张一个机制、结构或边界 |
| `so-what` | 所以呢 | 原文有洞见或实际意义时放最后一张 | `insight` 或 `why` 原句，headline 直接陈述实际意义 |

洞见内容：

| question | 读者在问 | 张数 | 文字主体 |
|---|---|---|---|
| `insight` | 最值得记住的判断是什么 | 1 张，第一张 | headline 直接放核心判断，出处只放 source |
| `why` | 这个判断为什么成立 | 原文有时 1 张，可与 how 合并 | 原文的事实、问题、失败或对比 |
| `how` | 其中的机制或结构是什么 | 0 到 3 张，只画原文有的关系 | 每张一个因果、结构、流程或适用边界 |
| `so-what` | 实践中怎么用 | 原文有实践动作或判断标准时放最后一张 | 原文给出的做法、选择标准或实际意义 |

一条内容 1 到 5 张，通常 3 到 5 张。张数只看原文有几个值得画成关系的东西；核心信息一两张已经讲清就收住。没有就不画，不为凑张数把数据表、结果清单、社区热度、重复论据或相关性弱的段落单独做成一张。洞见内容的原文没有实践动作时，不自行编造；有机制、结构或适用边界就用其中最重要的一项收尾，没有就停在核心判断。

每帧登记：

- `questions`：本帧回答的问题，可以多个。
  - 事件新闻第一帧含 `what`，所有帧都不使用 `insight`。原文有重要性或实际意义时再用 `why` 或 `so-what`。
  - 洞见内容第一帧含 `insight`，所有帧都不使用 `what`。原文有依据、关系或实践动作时再用 `why`、`how` 或 `so-what`。
- `claim`：这张图要让读者记住的一句话，直接陈述，能在原文找到出处。

常见误判：

- 事件新闻把价格、数据或限定做成第一张。它的第一张应是发生了什么。
- 洞见内容先介绍谁发表了什么，核心判断反而留到最后。
- 自己给新闻写一个原文没有的主题句。
- 把只调整语气的兜底表达当成独立信息。
- 为了「完整」把原文每段都做成一张。段落可以省。
- 原文没有洞见、重要性或实践动作，仍强行做一张结尾。
- 洞见内容用相关性弱的原文段落充当判断依据。

## Step 4 画面

画面的任务是讲文字讲不快的那个关系。先从原文找关系，再选形式。

| relation | 用于 | 注意 |
|---|---|---|
| `对比` | 两种条件、两种做法不同 | 不暗示原文没给的比例 |
| `流程` | 真实的步骤、机制、循环 | 箭头意味着顺序或因果，原文要支持 |
| `分支` | 一件事分成两条并行的线 | 并行不画成先后 |
| `边界` | 覆盖谁、不覆盖谁 | 范围要画清 |
| `变化` | 同一个东西从旧状态到新状态 | 必须真有前后 |
| `层级` | 分层、包含、上下游 | 不把并列画成层级 |
| `场景` | 一个具体情境比抽象图更快 | 情境细节不能编造事实 |

图文叙事的要求：

- 文字说结论，画面讲关系，两者拼在一起。标签和数值贴在它们描述的元素旁边，不放成独立一栏。
- 遮住标题看画面，要能看出一种关系。看不出就是装饰图，换关系或换形式。
- 画面表达的方向、状态、范围和先后关系与 `claim`、`headline` 一致，不添加文字没有表达的含义。
- 不用长度、面积、角度编码数值。生图模型画不准比例，数值差异用数字本身加等宽卡片表达。
- 图标不能暗示原文没有的意思。拿不准的写进本帧 `avoid`。
- 一条新闻内主角物件尽量用同一个，建议，不强制。

每帧登记 `visual.relation`、`visual.composition`、`visual.encoding`。`composition` 写画什么、放哪，`encoding` 写画面的哪个部分对应 claim 的哪个部分。

## Step 5 文字

槽位：`date`、`entity`、`series`、`headline`、`summary`、`deck`、`labels`、`values`、`gloss`、`footnote`、`source`。

写法：

- `headline`：一句直接陈述，一到两行。决定内容性质的事实状态写进主句，不追加只调整语气的兜底表达。
- `summary`：只用于事件新闻含 `what` 的第一张，原文一句话摘要原样。原文没有摘要时用阅读层的 `what`。洞见内容第一张不放摘要，核心判断直接进 headline。
- `deck`：一句，通常放 `why`；`story.why` 为 `null` 时留空。
- `labels`、`values`：贴在画面元素上的短标签和数值。数字只放读者会拿去用的；放了就带单位和比较对象，写在同一个 value 里。
- `gloss`：术语解释，每帧最多两条，每条一句，格式「术语：解释」。只解释本帧出现的术语。
- `footnote`：一句，可空。只放当前帧需要的补充信息，不预设出现位置或来源字段。
- `source`：原文的出处机构、媒体或作者。归因由它承担，句子里不再重复出处；同一帧混了两个来源时才在 value 里标。
- 图内叙述以中文为主，产品名、基准名、术语保留英文。中英文之间加空格，建议。

字数是软目标，脚本只提醒不拦。计数：一个非空白字符算 1，一个连续英文字母串算 2，空格不算。

| 槽位 | 软目标 |
|---|---|
| headline | 40 |
| summary | 130 |
| deck | 60 |
| footnote | 48 |
| 单条 gloss | 30 |
| 单个 label | 16 |
| 单个 value | 20 |
| 一帧合计 | 160，带 summary 的帧 260。只算 headline、summary、deck、labels、values、gloss、footnote，不算日期、机构、系列、来源 |

超软目标先问自己：这些字读者会不会看。会看就留，接受字号变小；不会看就删。

## Step 6 拼装 Prompt

六个块，固定顺序，块之间空一行，块标签独占一行，第一个块从第一行开始：

```text
[STYLE — fixed]
<assets/style.txt 去掉首行标签后的正文，逐字>

[LAYOUT — fixed]
<assets/layout.txt 去掉首行标签后的正文，逐字>

[STORY]
Claim: <frame.claim>
Relation: <visual.relation>
Composition: <visual.composition>
Encoding: <visual.encoding>

[TEXT]
<见下>

[AVOID — frame]
<见下>

[AVOID — fixed]
<assets/avoid.txt 去掉首行标签后的正文，逐字>
```

标签里的破折号是 `—`，从固定块文件首行复制。

`[TEXT]` 按槽位拼装，空槽位整行省略。下面定义字段与 Prompt 行的对应关系，不示范新闻内容。尖括号表示字段引用；写入 `frames[].prompt` 时必须替换为该帧的真实值，最终 Prompt 不得保留字段引用：

```text
Render exactly these strings in a bold, clean sans-serif suitable for Chinese; no other text anywhere in the image:
- Date badge (top-left): "<text.date>"
- Entity line (top-left): "<text.entity>"
- Series marker (top-left): "<text.series>"
- Headline (top-left, one or two lines): "<text.headline>"
- Summary paragraph (under the headline, up to three lines, smaller than the headline): "<text.summary>"
- Deck (under the headline or summary): "<text.deck>"
- Labels (on or beside the elements they name): "<text.labels[i]>"
- Value labels (beside the elements they describe): "<text.values[i]>"
- Glossary notes (small, beside the related element): "<text.gloss[i].term>：<text.gloss[i].gloss>"
- Footnote (bottom-left): "<text.footnote>"
- Source tag (bottom-right): "<text.source>"
```

多个 label、value、gloss 用 `, ` 连接，每个用英文双引号包住。gloss 渲染为「术语：解释」，中间是全角冒号。

`[AVOID — frame]`：`avoid` 非空时写 `No <项1>; no <项2>.`，每项去掉开头的 `No `、`no ` 和结尾的 `.`、`;`；为空时写 `No additional avoid items for this frame.`

`Composition` 与 `Encoding` 只写画什么、怎么对应 claim。建议不写「惊艳」「杰作」「电影感」「超精细」「霓虹」这类生图套话，它们会把画面带向通用 AI 商业海报风，脚本不检查这一条。

## Step 7 JSON 结构与脚本检查

下面是字段结构契约，不是可直接运行的 JSON。`string`、`integer` 和 `Array<...>` 表示类型；生成文件时必须写入实际 JSON 值。字段应该写什么以上文的原文提取规则为准，本节不提供新闻内容示例。

```text
{
  "skill": "news-visual-story",
  "schema_version": 1,
  "date": string(YYYY-MM-DD),
  "render_contract": { "aspect": "16:9", "size": "1920x1080", "background_color": "#F6F1E7", "transparent_background": false },
  "story": {
    "id": string(小写字母、数字和连字符),
    "entity": string,
    "source_title": string,
    "who": string | null,
    "what": string | null,
    "why": string | null,
    "insight": string | null,
    "source_refs": string[]
  },
  "reading": {
    "glossary": Array<{ "term": string, "gloss": string, "basis": "source" | "common" }>,
    "omitted": Array<{ "text": string, "kind": "supplementary" | "counter" | "hedge", "reason": string }>,
    "order_note": string | null
  },
  "frames": [
    {
      "index": integer,
      "questions": Array<"what" | "insight" | "why" | "how" | "so-what">,
      "claim": string,
      "text": {
        "date": string(MM.DD),
        "entity": string,
        "series": string(i/N),
        "headline": string,
        "summary": string | null,
        "deck": string | null,
        "labels": string[],
        "values": string[],
        "gloss": Array<{ "term": string, "gloss": string }>,
        "footnote": string | null,
        "source": string
      },
      "visual": {
        "relation": "对比" | "流程" | "分支" | "边界" | "变化" | "层级" | "场景",
        "composition": string,
        "encoding": string
      },
      "avoid": string[],
      "prompt": string(按 Step 6 拼装)
    }
  ]
}
```

- `frames[].index` 从 1 连续；`series` 为 `i/N`。
- `text.date` 是顶层 `date` 的 `MM.DD` 形式。
- `text.entity` 与 `story.entity` 相同。
- 第一帧必须只选择一个叙述起点。含 `what` 表示事件新闻，含 `insight` 表示洞见内容，不能同时含二者。
- 事件新闻的 `story.what` 必须是非空字符串，所有帧都不能使用 `insight`。
- 洞见内容的 `story.insight` 必须是非空字符串，所有帧都不能使用 `what`。
- `story.who`、`story.what`、`story.why`、`story.insight` 取不到时必须显式写 `null`，适用路径要求的起点字段除外。
- `summary` 只允许出现在含 `what` 的帧。
- `gloss` 每帧最多两条，term 应在 `reading.glossary` 里出现过。

在 skill 根目录运行：

```bash
python scripts/check_storyboard.py <分镜.json>
```

- `PASS` 表示字段齐、枚举和序号对、Prompt 六块与固定块和文字槽精确一致。它不证明内容对。
- `FAIL` 逐条修正后重跑。
- `WARN` 是帧数超过 5、字数超软目标或 gloss 术语不在术语表，看一眼决定留不留。

## Step 8 人工检查

脚本 PASS 后逐帧过三项，全部通过才算分镜完成：

1. 不误导。画上去的每个数字带单位和比较对象；每条 claim 能在原文找到出处；headline 与画面方向一致；`omitted` 登记的内容没有混进任何槽位。
2. 遮标题。只看 `visual.composition` 描述的画面，能说出一种关系。
3. 不依赖原文。图上出现的名称和缩写，要么读者本来就懂，要么在 `gloss` 里有解释。

再按叙述路径整体看一遍：

- 事件新闻：第一张回答发生了什么；后续只展开原文实际提供的重要性、关系或意义，没有为补结尾而凑内容。
- 洞见内容：第一张直接给核心判断；后续只展开原文实际提供的依据、关系或实践意义，事件与出处没有抢占起点。

## 收尾

回复一句「分镜完成：<新闻标题>，共 N 帧，脚本 PASS，人工三项通过；下一步读取 flows/render.md」，然后读取该文件。不在本透镜内调用生图工具。
