# 输出目录统一为“标题-视频号”方案

## Summary

将最终导出产物改为目录化输出：

- URL 输入默认写入 `outputs/<视频标题>-<视频号>/`
- 显式传入 `--output <dir>` 时，写入 `<dir>/<视频标题>-<视频号>/`
- 四种转写文件统一固定命名为 `transcript.txt`、`transcript.srt`、`transcript.vtt`、`transcript.json`
- 同目录额外写入 `metadata.json`，至少包含 `title` 和 `url`
- 本次只改最终输出目录与文件名，不改 `jobs/workdir` 缓存目录规则
- 本地文件输入没有视频号时，目录名仅使用标题

## Key Changes

### 1. 导出目录命名策略

在导出层新增“输出目录解析”逻辑，替代当前默认 `outputs/<job-id>`：

- 从 `transcript.Metadata` 读取：
  - `title`
  - `id`
  - URL 优先取 `webpage_url`，其次 `original_url`，再其次 `input`
  - `source_type`
- URL 输入目录名规则：
  - `sanitize(title) + "-" + sanitize(id)`
  - 若 `id` 缺失，回退为 `sanitize(title)`，避免生成多余尾部连字符
- 本地文件输入目录名规则：
  - `sanitize(title)`
  - 若 `title` 缺失，回退 `transcript`
- 当 `outputDir == ""` 时：
  - 基础目录为 `outputs`
  - 最终目录为 `outputs/<resolved-folder-name>`
- 当 `outputDir != ""` 时：
  - 基础目录为用户传入目录
  - 最终目录为 `<outputDir>/<resolved-folder-name>`

### 2. 固定转写文件名

调整导出器文件名策略：

- 不再基于标题生成导出文件名
- 四种格式统一写为：
  - `transcript.txt`
  - `transcript.srt`
  - `transcript.vtt`
  - `transcript.json`

现有 JSON transcript 结构保持不变，只改文件名与所在目录。

### 3. 新增 `metadata.json`

在导出目录内额外生成 `metadata.json`，最少包含：

```json
{
  "title": "...",
  "url": "..."
}
```

同时一并补充这些稳定字段，避免后续再次改格式：

- `id`
- `source_type`
- `site`
- `job_id`

字段值来源统一取 `transcript.Metadata`，不重新推断业务语义；缺失字段直接省略，不写空字符串。

### 4. 元数据准备

在 use case 现有 `transcriptMetadata(...)` 基础上继续复用现有键，不新增 CLI 参数：

- 保持 `title`、`input`、`site`、`job_id`
- 继续透传 downloader 已写入的：
  - `id`
  - `webpage_url`
  - `original_url`
- 导出器内部统一做 URL 选择：
  - `webpage_url` > `original_url` > `input`

这样可以让 URL 与本地文件都走同一套导出逻辑。

## Test Plan

补充并调整以下测试场景：

1. URL transcript 导出：
   - 输出目录为 `<base>/Bad-Title-Episode-1-QsZFBqtgI8A/`
   - 目录内存在四个固定文件名 `transcript.*`
   - 同目录存在 `metadata.json`
   - `metadata.json` 至少含 `title` 和正确 URL

2. 默认输出目录：
   - 不传 `outputDir` 时写到 `outputs/<标题-视频号>/`
   - 不再断言 `outputs/<job-id>/`

3. 显式 `--output` 目录：
   - 传入基础目录后，仍然创建一层 `<标题-视频号>` 子目录

4. 本地文件输入：
   - 目录名仅使用标题
   - 文件名仍为 `transcript.txt`
   - `metadata.json` 不要求有 URL，但应保留 `title`

5. 回退行为：
   - 缺失 `id` 的 URL 输入使用 `<标题>/`
   - 缺失 `title` 时回退为 `transcript/`
   - `metadata.json` 省略缺失字段，不写空值

6. 兼容性：
   - 现有 `txt/srt/vtt/json` 内容渲染不变
   - 仅路径、文件名、附加元数据文件发生变化

## Assumptions

- 本次范围仅限最终导出目录，不修改 `jobs` 缓存目录命名。
- 本地文件输入没有“视频号”，目录名只使用标题。
- 元数据文件格式固定为 `metadata.json`。
- `--output <dir>` 被视为输出根目录，不是最终叶子目录；导出器会在其下再创建 `<标题-视频号>` 子目录。
- “视频地址”统一取 `webpage_url`、`original_url`、`input` 的优先级回退结果。
