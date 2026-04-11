# TextDrain MVP 详细任务列表

## Summary

目标是把 MVP 拆成一组可以直接开工的实施任务，按依赖顺序推进，做到每一项都有明确产出、完成标准和边界。

本计划以 `Go CLI + yt-dlp + ffmpeg + whisper.cpp(whisper-cli)` 为固定路线，优先交付端到端可运行版本，再补可诊断性和工程质量。

## Implementation Tasks

### 1. 项目初始化（已完成）

- 建立 Go 项目骨架：`cmd/textdrain`、`internal/app`、`internal/domain`、`internal/infra`、`internal/cli`。
- 初始化 `go.mod`，确定 Go 版本为 `1.24+`。
- 接入 `cobra` 作为 CLI 框架。
- 接入 `slog` 日志封装，区分用户输出与内部日志。
- 定义基础目录约定：
  - 配置目录 `~/.config/textdrain/`
  - 缓存目录 `~/.cache/textdrain/jobs/`
  - 模型目录 `~/.cache/textdrain/models/`
- 完成标准：`textdrain --help` 可运行，项目结构和入口稳定。

### 2. 领域模型与接口定义（已完成）

- 定义核心类型：
  - `MediaAsset`
  - `DownloadResult`
  - `PreparedAudio`
  - `Transcript`
  - `TranscriptSegment`
  - `AudioOptions`
  - `TranscribeOptions`
  - `OutputFormat`
- 定义核心接口：
  - `SourceResolver`
  - `Downloader`
  - `AudioProcessor`
  - `ASREngine`
  - `Exporter`
- 定义任务状态枚举：
  - `PENDING`
  - `RESOLVING`
  - `DOWNLOADING`
  - `EXTRACTING_AUDIO`
  - `TRANSCRIBING`
  - `EXPORTING`
  - `COMPLETED`
  - `FAILED`
- 完成标准：接口和类型足以支撑端到端流程，不再需要在实现时补充关键字段。

### 3. 配置系统（已完成）

- 设计 `config.toml` 结构。
- 实现配置加载优先级：
  - CLI 参数
  - 配置文件
  - 默认值
- MVP 仅支持以下配置项：
  - `model`
  - `language`
  - `output_formats`
  - `keep_intermediate_files`
  - `model_dir`
  - `jobs_dir`
- 处理配置缺失时的默认值注入。
- 完成标准：应用在无配置文件时也能运行，并能读取用户配置覆盖默认值。

### 4. CLI 命令面（已完成）

- 实现 `textdrain transcribe <url-or-path>`。
- 实现 `textdrain doctor`。
- 实现 `textdrain models --list`。
- 为 `transcribe` 添加参数：
  - `--lang auto|zh|en`
  - `--model <name>`
  - `--output <dir>`
  - `--keep-intermediate`
- 统一命令退出码约定：
  - 成功返回 `0`
  - 参数错误、依赖错误、运行错误分别使用固定非零码
- 完成标准：三个命令帮助信息完整，参数解析稳定，错误提示可读。

### 5. 依赖探测与运行环境检查（已完成）

- 实现外部依赖探测：
  - `yt-dlp`
  - `ffmpeg`
  - `whisper-cli`
- 实现模型文件探测：
  - 检查默认模型目录
  - 检查指定模型名对应文件是否存在
- `doctor` 输出要包含：
  - 工具是否可执行
  - 版本信息
  - 默认路径
  - 缺失时的安装建议
- 完成标准：用户执行 `doctor` 能在不进入转写流程前定位环境问题。

### 6. 输入解析与源统一

- 实现输入判定：
  - 本地存在路径则视为本地文件
  - 否则视为 URL
- 本地文件路径做基础校验：
  - 是否存在
  - 是否可读
- URL 输入做基础合法性校验。
- 将两种输入统一转换为 `MediaAsset`。
- `MediaAsset` 至少包含：
  - 输入类型
  - 原始输入
  - 标题
  - 站点
  - 工作目录
  - 原始媒体路径
  - 时长
  - 语言提示
- 完成标准：上层 use case 不再关心输入源差异。

### 7. URL 下载器实现

- 封装 `yt-dlp` 调用器。
- 实现元数据获取：
  - 使用 `--dump-single-json`
  - 解析标题、站点、时长、原始 URL
- 实现下载策略：
  - 优先下载最佳音频
  - 音频不可用时退回最合适媒体文件
- 统一输出文件命名，避免标题中的非法字符。
- 处理下载过程中的临时路径和最终路径。
- 完成标准：给定一个兼容链接，可稳定得到本地媒体文件和结构化元数据。

### 8. 本地媒体处理器实现

- 封装 `ffmpeg` 调用器。
- 实现音频标准化流程：
  - 提取音频
  - 转换为 `wav`
  - 单声道
  - `16kHz`
  - `s16le`
- 保证处理中不把整段媒体加载进 Go 内存。
- 预留可选音频处理参数结构，但 MVP 默认仅执行标准化。
- 完成标准：任意 `ffmpeg` 可读媒体都可产出统一 wav 文件。

### 9. ASR 引擎实现

- 封装 `whisper-cli` 调用器。
- 组装模型路径、语言参数、输出参数。
- 支持语言模式：
  - `auto`
  - `zh`
  - `en`
- 解析 `whisper.cpp` 输出结果，构建统一 `Transcript` 结构。
- 保留 engine metadata，例如模型名、执行方式、语言模式。
- 完成标准：对标准 wav 可得到完整 transcript 和 segment 列表。

### 10. 导出器实现

- 实现 `txt` 导出。
- 实现 `srt` 导出。
- 实现 `vtt` 导出。
- 实现 `json` 导出。
- 文件命名统一基于安全化标题或输入文件名。
- 默认输出目录为 `outputs/<job-id>/`。
- 完成标准：四种格式的文件结构稳定，导出逻辑只依赖 `Transcript`。

### 11. 端到端编排用例

- 实现 `TranscribeUseCase`。
- 串联以下步骤：
  - 初始化任务目录
  - 解析输入
  - 如为 URL 则下载
  - 音频标准化
  - ASR 转写
  - 导出结果
  - 清理中间文件
- 在每个阶段更新任务状态。
- 统一处理错误并附带阶段信息。
- 完成标准：一个命令可完整跑通端到端流程。

### 12. 中间文件与目录管理

- 为每次任务生成唯一 `job-id`。
- 为 job 创建独立工作目录。
- 管理以下文件生命周期：
  - 原始下载文件
  - 标准化 wav
  - 转写临时结果
  - 最终导出文件
- 实现 `keep_intermediate_files` 开关。
- 完成标准：调试时可保留工件，默认情况下目录干净可控。

### 13. 用户输出与错误信息

- 定义统一错误类型：
  - 参数错误
  - 配置错误
  - 外部依赖错误
  - 下载错误
  - 媒体处理错误
  - ASR 错误
  - 导出错误
- 为每类错误输出固定格式：
  - 所属阶段
  - 核心原因
  - 建议修复动作
- 终端输出显示简洁进度：
  - 当前阶段
  - 关键文件路径
  - 最终导出位置
- 完成标准：用户不看日志也能定位大部分问题。

### 14. 模型发现与展示

- 实现模型目录扫描。
- 支持读取本地已有 `GGUF` 模型文件。
- `models --list` 输出：
  - 模型文件名
  - 路径
  - 大小
  - 是否为默认模型
- 不在 MVP 中实现模型下载。
- 完成标准：用户能知道当前有哪些可用模型以及默认使用哪个。

### 15. 测试体系

- 为领域类型和导出器写单元测试。
- 为配置解析和输入判定写单元测试。
- 为 `yt-dlp`、`ffmpeg`、`whisper-cli` 调用器写接口级测试，采用 mock 命令输出。
- 为 `TranscribeUseCase` 写端到端集成测试，至少覆盖：
  - 本地音频输入
  - 本地视频输入
  - URL 输入
  - 缺依赖
  - 模型缺失
- 准备 `testdata/`，包含最小可用音视频样本和命令输出样本。
- 完成标准：关键路径有自动化测试保护，不依赖真实联网环境才能回归。

### 16. 文档与交付说明

- 编写 README 的 MVP 使用说明。
- 补充安装前提：
  - Go 版本
  - `yt-dlp`
  - `ffmpeg`
  - `whisper.cpp`
  - 模型文件准备方式
- 补充典型命令示例：
  - 本地文件转写
  - URL 转写
  - 指定语言
  - 指定输出目录
  - 环境检查
- 补充故障排查章节。
- 完成标准：新用户按文档能完成首次运行。

## Milestones

### M1: 可运行骨架

- 完成任务 1-4
- 产出：CLI 空壳、配置系统、命令入口

### M2: 端到端基础链路

- 完成任务 5-11
- 产出：本地文件和 URL 都可转写，输出四种格式

### M3: 可维护与可诊断

- 完成任务 12-14
- 产出：目录管理、错误体系、模型管理、doctor 完整

### M4: 质量收尾

- 完成任务 15-16
- 产出：测试、文档、可发布 MVP

## Test Plan

1. `doctor` 在依赖齐全和依赖缺失两种情况下输出正确。
2. `transcribe` 对本地 `mp3` 和 `mp4` 都能成功导出四种结果。
3. `transcribe` 对 `yt-dlp` 兼容 URL 能完成下载和转写。
4. `--lang zh`、`--lang en`、`--lang auto` 分别能传递正确参数给 `whisper-cli`。
5. `keep_intermediate_files` 的开关行为正确。
6. `srt`、`vtt` 时间戳合法，`txt` 与 `json` 内容一致。
7. 外部命令异常退出时，错误信息包含阶段和简短修复建议。

## Assumptions

- 以 CLI 可交付为第一优先级，因此 MVP 明确使用 `whisper-cli` 而不是 `whisper.cpp` SDK 集成。
- `fetch` 命令延后，不进入 MVP 必做列表。
- 音频预处理只做到格式标准化，不在 MVP 中加入降噪、VAD、切块策略。
- 模型下载与自动安装不进入 MVP，先依赖用户手动准备本地模型。
