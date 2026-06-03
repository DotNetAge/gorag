# MiniRAG — 轻量移动端 RAG 框架

MiniRAG 是 GoRAG 的移动端版本，通过 [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile) 编译为 iOS/Android 原生 framework。

**设计原则：** ML 推理（向量化）由平台侧负责，MiniRAG 只做向量存储与检索。

- **iOS:** Core ML 计算嵌入向量，MiniRAG 存储/检索
- **Android:** ML Kit / TFLite 计算嵌入向量，MiniRAG 存储/检索

---

## 目录

- [前置要求](#前置要求)
- [构建](#构建)
- [iOS: 在 Xcode 中使用](#ios-在-xcode-中使用)
- [Android: 在 Android Studio 中使用](#android-在-android-studio-中使用)
- [API 参考](#api-参考)
- [向量数据格式](#向量数据格式)
- [常见问题](#常见问题)

---

## 前置要求

### 通用

- Go 1.21+
- gomobile

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

### iOS

- Xcode 15+
- macOS (编译时)

### Android

- Android SDK (API 21+)
- Android NDK r26+ (构建时需要)
  ```bash
  # Homebrew
  brew install android-ndk
  export ANDROID_NDK_HOME="/usr/local/share/android-ndk"
  ```
- Android Studio (开发时)

---

## 构建

```bash
# iOS Framework
make minirag-ios
# 或手动：gomobile bind -target=ios -o MiniRAG.xcframework ./minirag

# Android AAR
make minirag-android
# 或手动：ANDROID_HOME=~/Library/Android/sdk ANDROID_NDK_HOME=/usr/local/share/android-ndk \
#   gomobile bind -target=android -androidapi 21 -o MiniRAG.aar ./minirag

# 全部
make minirag-all

# 清理
make minirag-clean
```

### 产物

| 平台 | 产物 | 大小 |
|------|------|------|
| iOS | `MiniRAG.xcframework/` | ~55MB |
| Android | `MiniRAG.aar` | ~20MB |

---

## iOS: 在 Xcode 中使用

### 导入 Framework

1. **目标 Target → General → Frameworks, Libraries, and Embedded Content**
2. 点击 **+** → **Add Other** → **Add Files...**
3. 选择 `MiniRAG.xcframework`
4. Embed 模式设为 **Embed & Sign**

### 实现 Embedder

```swift
import CoreML
import MiniRAG

final class CoreMLEmbedder: MiniragEmbedderProtocol {
    private let model: MLModel

    init(model: MLModel) {
        self.model = model
    }

    func embedText(_ text: String) throws -> Data {
        // 调用 Core ML 模型，返回 float32 小端字节序
        let input = try MLMultiArray(shape: [1, 512], dataType: .float32)
        // ... 填充文本特征 ...

        let output = try model.prediction(from: YourInput(text: text))
        let embedding = output.featureValue(for: "embedding")!.multiArrayValue!
        return Data(bytes: embedding.dataPointer, count: embedding.count * 4)
    }
}
```

### Swift 原生封装

创建一个封装类，隐藏 JSON 序列化细节：

```swift
import MiniRAG

struct Chunk: Decodable {
    let id: String
    let content: String
}

struct Hit: Decodable {
    let id: String
    let content: String
    let score: Double
}

final class MiniRAG {
    private let impl: MiniragNewRAG

    init(dataDir: String, dimension: Int, embedder: MiniragEmbedderProtocol) throws {
        impl = try MiniragNewRAG(dataDir: dataDir, dimension: dimension, embedder: embedder)
    }

    func addIndex(_ content: String) throws -> [Chunk] {
        let data = try impl.addIndex(content)
        return try JSONDecoder().decode([Chunk].self, from: data)
    }

    func search(_ query: String, topK: Int = 10) throws -> [Hit] {
        let data = try impl.search(query, topK: topK)
        return try JSONDecoder().decode([Hit].self, from: data)
    }

    func delete(_ id: String) throws { try impl.delete(id) }
    func close() throws { try impl.close() }
}
```

### 使用

```swift
let docDir = NSHomeDirectory() + "/Documents/minirag"
let rag = try MiniRAG(dataDir: docDir, dimension: 512, embedder: CoreMLEmbedder(model: mlModel))

let chunks = try rag.addIndex("原神是一款开放世界冒险游戏")
let results = try rag.search("开放世界", topK: 5)

try rag.delete(chunks[0].id)
rag.close()
```

---

## Android: 在 Android Studio 中使用

### 导入 AAR

1. 将 `MiniRAG.aar` 放入 `app/libs/`
2. 在 `app/build.gradle.kts` 添加：

```kotlin
android {
    // ...
}

dependencies {
    implementation(fileTree("libs") { include("*.aar") })
}
```

3. **Sync Project with Gradle Files**

### Kotlin 封装

Gomobile 生成的 Java API 使用 `byte[]` 传递向量数据。建议做一个 Kotlin 封装层：

```kotlin
package com.example.minirag

import minirag.NewRAG
import minirag.Embedder
import org.json.JSONArray
import org.json.JSONObject

// — 原生类型 —
data class Chunk(val id: String, val content: String)
data class Hit(val id: String, val content: String, val score: Double)

// — Embedder 接口（由 ML Kit / TFLite 实现）—
interface MiniRAGEmbedder {
    fun embedText(text: String): ByteArray  // float32 LE bytes
}

// — Kotlin 封装 —
class MiniRAG(
    dataDir: String,
    dimension: Int,
    embedder: MiniRAGEmbedder
) : AutoCloseable {

    private val impl: NewRAG

    init {
        val goEmbedder = object : Embedder() {
            override fun embedText(text: String): ByteArray {
                return embedder.embedText(text)
            }
        }
        impl = NewRAG(dataDir, dimension, goEmbedder)
    }

    fun addIndex(content: String): List<Chunk> {
        val json = impl.addIndex(content) ?: return emptyList()
        val arr = JSONArray(String(json))
        return (0 until arr.length()).map { i ->
            val obj = arr.getJSONObject(i)
            Chunk(obj.getString("id"), obj.getString("content"))
        }
    }

    fun search(query: String, topK: Int = 10): List<Hit> {
        val json = impl.search(query, topK) ?: return emptyList()
        val arr = JSONArray(String(json))
        return (0 until arr.length()).map { i ->
            val obj = arr.getJSONObject(i)
            Hit(obj.getString("id"), obj.getString("content"), obj.getDouble("score"))
        }
    }

    fun delete(id: String) { impl.delete(id) }
    override fun close() { impl.close() }
}
```

### ML Kit Embedder 示例

```kotlin
package com.example.minirag

import android.content.Context
import com.google.mlkit.nl.embedding.TextEmbedder
import com.google.mlkit.nl.embedding.TextEmbedderOptions
import java.nio.ByteBuffer
import java.nio.ByteOrder

class MLKitEmbedder(context: Context) : MiniRAGEmbedder {

    private val embedder: TextEmbedder

    init {
        val options = TextEmbedderOptions.Builder()
            .build()
        embedder = TextEmbedder.getClient(options)
    }

    override fun embedText(text: String): ByteArray {
        val result = embedder.embedText(text)
            .addOnSuccessListener { /* ok */ }
            // ML Kit 的同步调用方式视版本而定

        // 假设 result.embedding 是 FloatArray
        val floats = result?.embedding ?: FloatArray(512)
        val buf = ByteBuffer.allocate(floats.size * 4).order(ByteOrder.LITTLE_ENDIAN)
        floats.forEach { buf.putFloat(it) }
        return buf.array()
    }
}
```

### 使用

```kotlin
class MainActivity : AppCompatActivity() {
    private lateinit var rag: MiniRAG

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val dataDir = filesDir.absolutePath + "/minirag"
        val embedder = MLKitEmbedder(this)
        rag = MiniRAG(dataDir, 512, embedder)

        // 索引
        lifecycleScope.launch(Dispatchers.IO) {
            val chunks = rag.addIndex("原神是一款开放世界冒险游戏")
            Log.d("MiniRAG", "indexed: $chunks")
        }
    }

    override fun onDestroy() {
        rag.close()
        super.onDestroy()
    }
}
```

---

## API 参考

### Go → ObjC/Java 映射

| Go 函数 | ObjC (iOS) | Java (Android) |
|---------|-----------|----------------|
| `New(dataDir, dim, emb)` | `+newWithDataDir:dimension:embedder:error:` | `NewRAG(dataDir, dim, embedder)` |
| `AddIndex(content)` | `-addIndex:error:` → `NSData*` | `addIndex(String)` → `byte[]` |
| `Search(query, topK)` | `-search:topK:error:` → `NSData*` | `search(String, int)` → `byte[]` |
| `Delete(id)` | `-delete:error:` → `BOOL` | `delete(String)` → `void` |
| `Close()` | `-close:error:` → `BOOL` | `close()` → `void` |

### JSON 格式

**AddIndex 返回：**
```json
[{"id":"a1b2c3d4e5f6","content":"原神是一款开放世界冒险游戏"}]
```

**Search 返回：**
```json
[{"id":"a1b2c3d4e5f6","content":"原神是一款开放世界冒险游戏","score":0.956}]
```

---

## 向量数据格式

所有跨语言边界的向量以 `byte[]` / `Data` 传递，格式为 **float32 小端字节序**：

```
文本 → ML 模型 → [0.012, -0.034, 0.156, ...] (512个float32)
               → byte[0..3]  第一个 float32 的 LE 编码
               → byte[4..7]  第二个 float32 的 LE 编码
               → ...
               → 总计 512 × 4 = 2048 字节
```

---

## 常见问题

### macOS: `gomobile: command not found`

```bash
export PATH=$HOME/go/bin:$PATH
# 或永久添加
echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.zshrc
```

### iOS: `No such module MiniRAG`

检查 **Build Phases → Link Binary With Libraries** 是否已添加 `MiniRAG.xcframework`。如已添加，检查 **Framework Search Paths**。

### Android: NDK 未安装

```bash
# 方法一：通过 sdkmanager
sdkmanager --install "ndk;26.1.10909125"

# 方法二：通过 Homebrew
brew install android-ndk

# 方法三：Android Studio → SDK Manager → SDK Tools → NDK
```

### Android: `UnsatisfiedLinkError`

确保 `MiniRAG.aar` 包含了对应架构的 `.so`（gomobile 默认包含 arm64-v8a，如需 armeabi-v7a 需加参数 `-target=android/arm,android/arm64`）。
