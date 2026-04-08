# TextDrain v1 技术选型方案

## Summary

第一版明确采用 `Go CLI` 作为主程序语言与交付形态，面向 `跨平台 CPU 优先` 的本地环境，输入为 `yt-dlp` 兼容视频链接或本地媒体文件，转写目标为 `单一语言的中文或英文`。

整体路线采用“`Go 负责编排 + 本地成熟工具负责重计算`”：下载使用 `yt-dlp`，媒体处理使用 `ffmpeg`，离线 ASR 使用 `whisper.cpp`。这条路线比纯 Go ASR 更稳，也比嵌 Python 更易部署。

## Key Choices

### 1. 主语言与工程组织

- 主语言选 `Go 1.24+`。
- 原因：CLI 体验成熟，单二进制分发友好，并发与任务编排清晰，后续接 GUI 或本地服务也容易扩展。
- 包管理与构建使用 Go 原生工具链；项目结构保持 `cmd/ + internal/ + pkg/` 风格。
- 配置文件建议使用 `TOML`，Go 侧采用 `BurntSushi/toml` 或 `pelletier/go-toml/v2`，默认推荐 `pelletier/go-toml/v2`。

### 2. 输入与下载层

- 视频站点下载技术选 `yt-dlp`。
- Go 不直接重写下载逻辑，采用 `exec.CommandContext` 调用 `yt-dlp` CLI，并通过 `--dump-single-json` 获取元数据、通过模板参数控制输出路径。
- 原因：站点兼容性完全依赖 `yt-dlp` 社区维护，Go 自己实现站点适配没有性价比。
- 对外接口固定为：
  - `SourceResolver`
  - `Downloader`
  - `MediaMetadata`
- URL 输入和本地文件输入都先归一到统一的 `MediaAsset` 结构，避免后续管线分叉。

### 3. 媒体处理层

- 音频提取与标准化技术选 `ffmpeg`。
- Go 侧同样通过子进程调用 `ffmpeg`，不选 CGO 绑定。
- 标准输出音频格式固定为：
  - `wav`
  - `mono`
  - `16kHz`
  - `s16le`
- v1 只做轻量预处理：
  - 音频抽取
  - 采样率/声道统一
  - 可选响度归一化
  - 可选静音切分
- 不在 v1 引入复杂降噪、VAD 拼段算法或自研 DSP。
- 原因：`ffmpeg` 跨平台稳定，维护成本远低于在 Go 中拼装音频处理链。

### 4. 离线 ASR 层

- 识别引擎选 `whisper.cpp`。
- 接入方式优先选 `Go 调用 whisper.cpp SDK 接口`，但实现上应保留 `CLI fallback`：
  - 首选：通过 Go binding 或轻量 CGO 封装调用 `whisper.cpp` 核心 API
  - 备选：调用 `whisper-cli` 命令行
- 默认模型格式选 `GGUF`。
- 默认模型建议：
  - 中文：`small` 起步
  - 英文：`base` 或 `small`
  - 默认统一推荐 `small`，在稳定和 CPU 速度之间折中
- 语言策略：
  - 默认 `auto`
  - 用户可显式传 `zh` 或 `en`
  - 由于输入通常是单语言，v1 不做段级多语种处理
- 选择 `whisper.cpp` 的原因：
  - 本地离线能力成熟
  - CPU 友好
  - 模型文件与二进制部署清晰
  - 比 `faster-whisper + Python` 更适合 Go CLI 主项目

### 5. 导出与结果结构

- 输出格式选 `txt + srt + vtt + json`，其中 `json` 作为标准内部交换格式。
- 核心结果类型建议固定：
  - `Transcript`
  - `TranscriptSegment`
  - `ExportArtifact`
- 导出逻辑完全基于统一 transcript 结构，不让导出层感知下载器或 ASR 引擎细节。

### 6. CLI 与可观测性

- CLI 框架选 `cobra`。
- 原因：Go CLI 生态最成熟，子命令、补全、帮助文档、后续扩展都更顺手。
- 日志建议：
  - 结构化日志用 `slog`
  - 终端人类可读输出单独封装，不直接暴露底层错误
- 进度展示：
  - 下载阶段直接透传 `yt-dlp` 进度并做解析
  - 音频处理与转写阶段由 Go 自己维护阶段状态
- 命令建议保留：
  - `textdrain transcribe`
  - `textdrain fetch`
  - `textdrain models`
  - `textdrain doctor`

## Public APIs / Interfaces / Types

建议固定这些 Go 接口，后续实现不要改语义：

```go
type SourceResolver interface {
    Resolve(ctx context.Context, input string) (MediaAsset, error)
}

type Downloader interface {
    Fetch(ctx context.Context, asset MediaAsset, workdir string) (DownloadResult, error)
}

type AudioProcessor interface {
    Prepare(ctx context.Context, mediaPath string, workdir string, opts AudioOptions) (PreparedAudio, error)
}

type ASREngine interface {
    Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (Transcript, error)
}

type Exporter interface {
    Export(ctx context.Context, transcript Transcript, outputDir string, formats []OutputFormat) ([]string, error)
}
```

核心数据结构建议：
- `MediaAsset`: 输入类型、标题、原始引用、站点、媒体路径、时长、语言提示
- `PreparedAudio`: wav 路径、采样率、声道、时长
- `Transcript`: 语言、完整文本、segments、engine info
- `TranscriptSegment`: `StartMs`, `EndMs`, `Text`

## Test Plan

必须覆盖以下技术选型相关场景：

1. 依赖探测
- 未安装 `yt-dlp`
- 未安装 `ffmpeg`
- 未找到 `whisper.cpp` 动态库、静态库或可执行文件
- `doctor` 能给出明确修复提示

2. 输入处理
- 本地 mp4 / mp3 正常进入统一管线
- URL 输入能正确获取元数据并下载媒体
- 非法 URL、本地不存在文件时错误可读

3. 媒体处理
- `ffmpeg` 能稳定产出单声道 16kHz wav
- 超长输入不会把整段音频全读入内存
- 中间文件保留和清理策略符合配置

4. ASR 行为
- `zh` 明确指定时不误走英文参数
- `en` 明确指定时正常转写
- `auto` 在中文和英文单语言输入下结果可接受
- SDK 调用失败时可切换到 CLI fallback（如果实现 fallback）

5. 导出正确性
- `txt` 为完整拼接文本
- `srt` / `vtt` 时间戳格式正确
- `json` 可完整表达 transcript 结构

## Assumptions

- v1 不自研视频网站解析逻辑，站点能力完全跟随 `yt-dlp`。
- v1 不自研 ASR 模型与训练流程，只集成 `whisper.cpp`。
- v1 不依赖 GPU；若后续要做高性能版本，再增加 CUDA / Metal 优化分支。
- `Go 调用 whisper.cpp SDK` 是首选实现，但为了跨平台落地，架构上必须允许回退到 `whisper.cpp` CLI。
- 第一版优先“稳定可跑、安装路径清楚”，不优先追求极限识别质量或最快速度。
