# render-ai-news-page Go CLI

本目录是 `render-ai-news-page` skill 的开发维护入口。普通生成只需读取 `flows/generate.md`，不需要阅读本文件。

## 命令

```text
render-ai-news-page build --report-type <daily|weekly> --source <新闻.md> --output <目标.html>
render-ai-news-page lint --report-type <daily|weekly> --source <新闻.md> --page <页面.html>
render-ai-news-page version
```

- `--report-type` 必填且只能出现一次：

  | 值 | 日期块 | 正文层级 |
  |----|--------|----------|
  | `daily` | “生成日期/生成时间”单日段落 | 保留原有日报规则：正文一级章节和“今日思想亮点” |
  | `weekly` | “日期范围/统计周期”段落或引用块 | 独立周报规则：唯一一级报刊标题、“本周判断”和后续二级章节 |

  周报日期范围只校验日期有效性与开始不晚于结束，不限制星期或固定天数。模式不匹配时 lint 失败，不自动切换。
- `build` 在内存中只解析和渲染一次，通过 lint 后才创建文件；目标存在时自动使用 `-2`、`-3` 后缀。写入后会同步、回读并对落盘内容执行最终 lint。
- `lint` 只读，要求页面可由编译时模板和源稿确定性重建。
- 所有单值参数只能出现一次。未知参数、多余位置参数、错误模式和帮助参数冲突均以退出码 `2` 失败。
- 不提供覆盖、删除、递归删除、批量处理、模板选择或 dry-run。
- 普通写入或最终 lint 失败时，CLI 会清理本次新建文件；清理也失败时会报告可能残留的精确路径。进程被强制终止或机器断电时不保证自动清理。

## 输出契约

终端输出面向人和 agent，完整 HTML 只写入目标文件。成功摘要固定包含报表类型、实际路径、源稿 SHA-256、模板 SHA-256 和 lint 状态。

lint 会完整扫描输入，默认最多展示前 100 条诊断；超限时报告总数、省略数量和完整扫描状态。当前没有额外低频结果接口。

退出码：

| 退出码 | 含义 |
|--------|------|
| `0` | 成功 |
| `2` | 命令或参数错误 |
| `3` | 源稿、模板或页面 lint 失败 |
| `4` | 文件读取、生成或写入失败 |

## 开发验证

```text
go test ./...
```

模板、CSS 或 JavaScript 变化后，还必须按 `flows/maintain.md` 使用外部 Playwright MCP 运行 `scripts/playwright-title-regression.js`，再执行实际长篇新闻的桌面端与移动端视觉验收。

## 构建

Windows PowerShell：

```powershell
.\scripts\build.ps1
```

Linux 或 macOS：

```bash
sh ./scripts/build.sh
```

两个脚本都会先执行测试，再生成：

- `bin/render-ai-news-page-windows-amd64.exe`
- `bin/render-ai-news-page-linux-amd64`
- `bin/render-ai-news-page-darwin-amd64`

脚本只覆盖用户确认的三项 amd64 平台。
