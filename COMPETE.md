# CompeteAI 显式记忆（COMPETE.md）

本文件类似 Claude Code 的 `CLAUDE.md`，用于定义项目级长期显式规则。启动任务时会自动加载到 Agent 上下文。

## 报告规范

- `report.style`: 专业、客观、中文简体，避免夸张营销用语
- `report.section_order`: ["执行摘要", "功能对比", "SWOT", "定价分析", "用户画像", "结论与建议"]
- `terminology.zh_cn`: 统一使用「竞品分析」「功能矩阵」「来源追溯」等术语

## 分析维度

- `analysis.default_dimensions`: ["功能", "定价", "SWOT", "用户画像", "来源可信度"]

## 来源规则

- `source.trusted_domains`: ["github.com", "official", "docs.", "pricing"]
- `source.forbidden_phrases`: ["据传闻", "据说", "可能大概"]

## QA 门槛

- `qa.min_score`: 75

## 实体别名（人工映射）

- `entity.alias.github_copilot`: GitHub Copilot
- `entity.alias.cursor`: Cursor
