# 重度依赖 Parser 实现计划

**状态**: 🚀 启动  
**日期**: 2024-03-19  
**目标**: 完成 3 个独立的重度依赖 Parser 插件  

---

## 📋 **Parser 列表（独立项目）**

### 重要说明
**所有重度依赖 Parser 都是独立项目**，与 GoRAG 项目同层目录：
```
/Users/ray/workspaces/gorag/
├── gorag/              # 主项目（轻量级 Parser）
├── gorag-audio/        # Audio Parser 插件（重度依赖）
├── gorag-video/        # Video Parser 插件（重度依赖）
└── gorag-webpage/      # Webpage Parser 插件（重度依赖）
```

### 1. gorag-audio (优先级：高)
**依赖**: 
- `github.com/gopxl/beep` - Go 音频处理
- OpenAI Whisper API (可选本地部署)

**功能**:
- [ ] 支持 MP3/WAV/OGG/FLAC 格式
- [ ] 语音识别（STT）
- [ ] 时间戳标记
- [ ] 流式处理大文件
- [ ] 元数据提取（时长、采样率等）

**预计工时**: 3-4 天

---

### 2. gorag-video (优先级：中)
**依赖**:
- `github.com/u2takey/ffmpeg-go` - FFmpeg Go 绑定
- `gocv.io/gocv` - OpenCV Go 绑定（OCR）

**功能**:
- [ ] 支持 MP4/AVI/MKV 格式
- [ ] 提取音频轨道（使用 Audio Parser）
- [ ] 视频帧提取
- [ ] OCR 文字识别
- [ ] 字幕提取
- [ ] 关键帧检测

**预计工时**: 4-5 天

---

### 3. gorag-webpage (增强版) (优先级：低)
**依赖**:
- `github.com/PuerkitoBio/goquery` - HTML 解析增强
- `github.com/go-rod/rod` - 无头浏览器（可选）

**功能**:
- [ ] JavaScript 渲染内容支持
- [ ] 动态内容抓取
- [ ] 截图功能
- [ ] 链接关系图谱
- [ ] SEO 元数据提取
- [ ] 结构化数据（JSON-LD）

**预计工时**: 2-3 天

---

## 🎯 **实施策略**

### Phase 1: gorag-audio (3-4 天)
```bash
# 在项目同层创建独立项目
cd /Users/ray/workspaces/gorag/
git clone <gorag-repo> gorag-audio
cd gorag-audio

Day 1-2: 基础架构
- 初始化 Go 模块
- 实现音频文件读取
- 实现元数据提取
- 添加基础测试

Day 3-4: 语音识别
- 集成 Whisper API
- 实现时间戳标记
- 完整测试套件
- README 文档
```

### Phase 2: gorag-video (4-5 天)
```bash
cd /Users/ray/workspaces/gorag/
git clone <gorag-repo> gorag-video
cd gorag-video

Day 1-2: 视频处理基础
- FFmpeg 集成
- 音频轨道提取（复用 gorag-audio）
- 帧提取

Day 3-4: OCR 和字幕
- OpenCV 集成
- 帧 OCR 识别
- 字幕提取
- 元数据管理

Day 5: 测试和文档
- 完整测试套件
- 性能优化
- README 文档
```

### Phase 3: gorag-webpage 增强 (2-3 天)
```bash
cd /Users/ray/workspaces/gorag/
git clone <gorag-repo> gorag-webpage
cd gorag-webpage

Day 1: 无头浏览器集成
- rod 库集成
- 页面渲染
- 截图功能

Day 2: 高级功能
- 动态内容抓取
- JSON-LD 解析
- 链接分析

Day 3: 测试和文档
- 完整测试
- 文档更新
```

---

## 📊 **技术架构**

### 独立项目结构
每个重度依赖 Parser 都是独立的 Go 模块：
```
gorag-audio/
├── go.mod              # 独立模块定义
├── parser.go           # 主解析器实现
├── whisper.go          # Whisper 集成
├── metadata.go         # 元数据提取
├── parser_test.go      # 测试文件
└── README.md           # 文档
```

### 与主项目的关系
```go
// gorag-audio/go.mod
module github.com/DotNetAge/gorag-audio

require (
    github.com/DotNetAge/gorag v1.0.0  // 依赖主项目
    github.com/gopxl/beep v1.2.0
)
```

### 使用方式
```go
import (
    "github.com/DotNetAge/gorag"
    "github.com/DotNetAge/gorag-audio"
)

func main() {
    // 注册 Audio Parser
    engine.RegisterParser("audio", audio.NewParser())
    
    // 使用
    chunks, _ := engine.ParseFile(ctx, "audio.mp3")
}
```

---

## 🔧 **开发环境准备**

### 工作目录结构
```bash
# 所有项目都在 gorag 目录下
cd /Users/ray/workspaces/gorag/
ls -la
# 显示:
# gorag/          (主项目)
# gorag-audio/    (Audio Parser 插件)
# gorag-video/    (Video Parser 插件)
# gorag-webpage/  (Webpage Parser 插件)
```

### 安装系统依赖
```bash
# macOS
cd /Users/ray/workspaces/gorag/
brew install ffmpeg opencv sox

# Ubuntu/Debian
sudo apt-get install ffmpeg libopencv-dev sox

# Windows
choco install ffmpeg opencv sox
```

### 创建独立项目
```bash
# Audio Parser
cd /Users/ray/workspaces/gorag/
git clone https://github.com/DotNetAge/gorag.git gorag-audio
cd gorag-audio
go mod init github.com/DotNetAge/gorag-audio
go get github.com/gopxl/beep

# Video Parser
cd /Users/ray/workspaces/gorag/
git clone https://github.com/DotNetAge/gorag.git gorag-video
cd gorag-video
go mod init github.com/DotNetAge/gorag-video
go get github.com/u2takey/ffmpeg-go
go get gocv.io/gocv

# Webpage Parser
cd /Users/ray/workspaces/gorag/
git clone https://github.com/DotNetAge/gorag.git gorag-webpage
cd gorag-webpage
go mod init github.com/DotNetAge/gorag-webpage
go get github.com/PuerkitoBio/goquery
go get github.com/go-rod/rod
```

---

## 📈 **质量要求**

### 测试覆盖
- 单元测试：80%+
- 集成测试：包含真实文件测试
- 性能基准：包含在测试中

### 文档要求
- README.md（中英文）
- API 文档注释
- 使用示例代码
- 性能基准数据

---

## ⚠️ **注意事项**

### 独立项目管理
- 每个 Parser 都有独立的 Git 仓库
- 可以独立发布版本
- 依赖管理独立于主项目
- 可以选择性安装需要的 Parser

### CGO 依赖
- 需要编译 C/C++ 库
- 跨平台兼容性需要注意
- 提供预编译二进制（可选）
- CI/CD 需要配置对应环境

### 性能优化
- 使用 goroutine 并行处理
- 缓存中间结果
- 支持断点续传
- 内存使用监控

### 安全考虑
- 文件上传大小限制
- 恶意文件检测
- 资源使用监控
- 沙箱环境运行（推荐）

---

## 🎯 **成功标准**

### 项目级别
- [ ] 3 个独立项目都成功创建
- [ ] 每个项目都有完整的 Git 历史
- [ ] 独立的版本号管理
- [ ] 可以独立发布和分发

### 质量要求
- [ ] 所有 Parser 通过测试
- [ ] 平均覆盖率 80%+
- [ ] 性能达到预期基准
- [ ] 文档完整清晰
- [ ] 支持主流文件格式
- [ ] 生产环境可用

### 集成能力
- [ ] 可以无缝集成到 GoRAG 主项目
- [ ] 遵循统一的 Parser 接口
- [ ] 提供插件注册机制
- [ ] 文档说明清晰完整

---

*📍 位置：`/Users/ray/workspaces/gorag/gorag/.docs/`*  
*📅 创建日期：2024-03-19*  
*👤 作者：ImgSeeker*  
*🚀 下一步：开始 Audio Parser 实现*
