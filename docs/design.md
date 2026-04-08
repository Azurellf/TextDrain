# 离线视频语音转写工具架构设计 v1

## Summary

第一版产品定位为一个本地运行的 `CLI` 工具：输入 `yt-dlp` 可下载的视频链接或本地视频文件，先提取音频，再在本机完成语音识别，输出 `txt + srt/vtt`。

核心边界是：`下载阶段允许联网`，`音频处理与转写阶段完全离线`，优先把单说话人的高质量转写做稳定，不在 v1 引入多人说话人分离、摘要或翻译。

## Key Changes

### 1. 分层架构

将系统拆成 5 层，避免下载、媒体处理、识别模型和导出格式耦合：

1. `CLI / Application Layer`
   负责命令解析、任务创建、进度展示、日志和配置加载。
   建议命令形态：
   - `textdrain transcribe <url-or-path>`
   - `textdrain fetch <url>`
   - `textdrain models`
   - `textdrain doctor`

2. `Ingestion Layer`
   统一处理输入源：
   - `URLSource`: 交给 `yt-dlp` 拉取最佳音频或视频
   - `LocalFileSource`: 校验本地视频/音频文件
   输出统一的 `MediaAsset` 对象，包含源信息、标题、原始路径、缓存路径、媒体时长、站点元数据。

3. `Media Pipeline Layer`
   负责本地媒体处理，建议基于 `ffmpeg`：
   - 视频/音频转单声道、16kHz PCM/WAV
   - 静音裁剪或音量归一化
   - 长音频切片
   - 失败时保留中间文件，便于排障
   这一层只暴露稳定接口，不让上层直接拼 `ffmpeg` 参数。

4. `ASR Layer`
   作为可替换的识别引擎抽象，v1 默认实现建议：
   - 接口：`ASREngine.transcribe(audio_path, language=None) -> Transcript`
   - 默认实现：`faster-whisper` 或 `whisper.cpp`
   建议默认优先 `faster-whisper`，原因是 Python 集成顺手、模型管理清晰、CPU/GPU 兼容更容易。
   输出统一结构：
   - `segments[]`
   - `start/end`
   - `text`
   - `language`
   - `confidence(optional)`

5. `Export Layer`
   将统一 transcript 导出为：
   - `txt`
   - `srt`
   - `vtt`
   - `json`（内部与调试使用，也建议暴露给用户）
   导出层只依赖标准 `Transcript` 数据结构，不直接依赖模型实现。

### 2. 核心运行流程

建议采用单任务流水线，后续可扩展批处理：

`Input -> Resolve Source -> Download/Locate Media -> Extract/Normalize Audio -> ASR -> Post-process -> Export`

各阶段都产出明确的中间结果和状态：
- `PENDING`
- `DOWNLOADING`
- `EXTRACTING_AUDIO`
- `TRANSCRIBING`
- `EXPORTING`
- `COMPLETED`
- `FAILED`

即便第一版只有 CLI，也建议内部做成显式任务状态机，后续接 GUI 时可直接复用。

### 3. 目录与模块边界

建议 Python 项目结构如下，使用 `uv` 管理：

- `src/textdrain/cli/`
- `src/textdrain/app/`
- `src/textdrain/domain/`
- `src/textdrain/infrastructure/downloader/`
- `src/textdrain/infrastructure/media/`
- `src/textdrain/infrastructure/asr/`
- `src/textdrain/infrastructure/exporters/`
- `src/textdrain/config/`
- `tests/`

模块职责：
- `domain`: `MediaAsset`, `Transcript`, `Segment`, `JobConfig`, `JobResult`
- `app`: 编排 use case，例如 `TranscribeVideoUseCase`
- `infrastructure`: `yt-dlp`、`ffmpeg`、ASR 模型、文件系统实现
- `cli`: 参数解析、终端输出、错误码、进度条

### 4. 外部依赖与本地资源

建议将依赖分成“必需运行时工具”和“Python 库”两类。

运行时工具：
- `ffmpeg`
- `yt-dlp`

Python 库建议：
- `typer` 或 `click`：CLI
- `pydantic`：配置与领域对象校验
- `faster-whisper`：默认 ASR 引擎
- `rich`：进度和日志展示

模型与缓存策略：
- 模型文件缓存到用户目录，例如 `~/.cache/textdrain/models/`
- 下载媒体缓存到 `~/.cache/textdrain/jobs/`
- 输出默认写到当前目录或 `./outputs/`
- 配置文件建议放到 `~/.config/textdrain/config.toml`

### 5. 配置与接口

建议公开的核心配置项：
- `model`: `tiny/base/small/medium/large-v3` 或对应 GGUF/CTranslate2 变体
- `device`: `cpu/cuda/auto`
- `language`: 默认 `auto`
- `output_formats`: `txt,srt,vtt,json`
- `keep_intermediate_files`: `true/false`
- `audio_only_download`: `true/false`

建议保留但不做复杂策略的接口：
- `DownloaderBackend`
- `ASREngine`
- `Exporter`

这样 v2 可替换为：
- 其他下载器
- 其他离线 ASR 引擎
- GUI 前端

## Public APIs / Interfaces / Types

建议先固定以下内部接口，后续实现不要改语义：

```python
class MediaSource(Protocol):
    def resolve(self, raw_input: str) -> MediaAsset: ...

class DownloaderBackend(Protocol):
    def fetch(self, source: MediaAsset, workdir: Path) -> DownloadResult: ...

class AudioProcessor(Protocol):
    def prepare(self, media_path: Path, workdir: Path) -> PreparedAudio: ...

class ASREngine(Protocol):
    def transcribe(self, audio_path: Path, language: str | None = None) -> Transcript: ...

class TranscriptExporter(Protocol):
    def export(self, transcript: Transcript, output_dir: Path) -> list[Path]: ...
```

核心数据类型建议：
- `MediaAsset`: 输入源、标题、站点、原始引用、媒体路径、时长
- `PreparedAudio`: 标准化后的 wav 路径、采样率、声道数、时长
- `Transcript`: 语言、完整文本、segments
- `TranscriptSegment`: `start`, `end`, `text`

## Test Plan

必须覆盖这些场景：

1. 本地文件输入
   - 输入本地 mp4
   - 成功提取音频并输出 `txt + srt + vtt`

2. 视频链接输入
   - 通过 `yt-dlp` 下载兼容站点音频
   - 元数据缺失或标题异常时仍能完成任务

3. 长音频转写
   - 超过 1 小时的输入
   - 内存不爆、阶段状态正确、失败可恢复

4. 非法输入与失败路径
   - 无效 URL
   - 本地文件不存在
   - `ffmpeg` 或 `yt-dlp` 缺失
   - 模型文件不存在或损坏

5. 配置行为
   - 指定语言
   - 自动识别语言
   - 切换输出格式
   - 是否保留中间文件

6. 导出正确性
   - `srt/vtt` 时间戳格式正确
   - `txt` 文本顺序与 segment 拼接一致
   - `json` 与内部 transcript 结构一致

## Assumptions

- 实现语言默认选择 Python，并用 `uv` 管理项目。
- v1 默认服务技术用户，不优先做 GUI；但内部设计必须允许后续加桌面界面。
- v1 不做 DRM、登录态站点支持、浏览器抓包能力。
- v1 不做多人说话人分离，不做翻译和摘要。
- 默认语言策略为 `auto-detect`，必要时允许用户手动指定语言。
- 默认识别引擎采用 `faster-whisper`；若后续需要更强本地可移植性，再补 `whisper.cpp` 适配器。
