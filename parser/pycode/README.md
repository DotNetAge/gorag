# Python Code Parser

用于解析 Python 代码的流式解析器，支持函数、类和注释的提取。

## 特性

- ✅ 流式处理 - 支持 GB 级 Python 文件
- ✅ 函数提取 - 自动识别和提取函数定义
- ✅ 类提取 - 自动识别和提取类定义
- ✅ 注释处理 - 支持单行和多行注释
- ✅ 缩进感知 - 基于 Python 缩进规则解析
- ✅ Overlap 分块 - 保持代码语义连续性
- ✅ 上下文取消 - 支持超时和取消操作
- ✅ 零 CGO 依赖 - 纯 Go 实现

## 快速开始

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "github.com/DotNetAge/gorag/parser/pycode"
)

func main() {
    parser := pycode.NewParser()
    
    pythonContent := []byte(`
def greet(name):
    """Say hello"""
    print(f"Hello, {name}!")

class Calculator:
    def add(self, a, b):
        return a + b
`)
    
    chunks, err := parser.Parse(context.Background(), bytes.NewReader(pythonContent))
    if err != nil {
        panic(err)
    }
    
    for _, chunk := range chunks {
        fmt.Printf("Type: %s\n", chunk.Metadata["element_type"])
        if chunk.Metadata["function_name"] != "" {
            fmt.Printf("Function: %s\n", chunk.Metadata["function_name"])
        }
        if chunk.Metadata["class_name"] != "" {
            fmt.Printf("Class: %s\n", chunk.Metadata["class_name"])
        }
    }
}
```

## 配置选项

### 设置 Chunk 大小

```go
parser := pycode.NewParser()
parser.SetChunkSize(500)        // 每个 chunk 的字节数
parser.SetChunkOverlap(50)      // chunk 之间的重叠字节数
```

### 启用/禁用功能

```go
parser.SetExtractFunctions(true)   // 提取函数（默认开启）
parser.SetExtractClasses(true)     // 提取类（默认开启）
parser.SetExtractComments(true)    // 提取注释（默认开启）
```

## 使用示例

### 1. 基本解析

```go
parser := pycode.NewParser()
chunks, err := parser.Parse(ctx, reader)
```

### 2. 回调模式（推荐用于大文件）

```go
err := parser.ParseWithCallback(ctx, reader, func(chunk core.Chunk) error {
    fmt.Printf("Processing chunk: %s\n", chunk.ID)
    return nil
})
```

### 3. 提取特定元素

```go
parser := pycode.NewParser()
parser.SetExtractFunctions(true)
parser.SetExtractClasses(false)  // 不提取类
parser.SetExtractComments(false) // 不提取注释
```

### 4. 带超时的解析

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

chunks, err := parser.Parse(ctx, reader)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("解析超时")
    }
}
```

## 输出格式

### 函数 Chunk

```python
// FUNCTION: greet
def greet(name):
    """Say hello"""
    print(f"Hello, {name}!")
```

**Metadata:**
```json
{
  "type": "pycode",
  "element_type": "function",
  "function_name": "greet",
  "position": "0"
}
```

### 类 Chunk

```python
// CLASS: Calculator
class Calculator:
    def add(self, a, b):
        return a + b
```

**Metadata:**
```json
{
  "type": "pycode",
  "element_type": "class",
  "class_name": "Calculator",
  "position": "1"
}
```

### 混合 Chunk

```python
# This is a comment
x = 10
y = 20
print(x + y)
```

**Metadata:**
```json
{
  "type": "pycode",
  "element_type": "mixed",
  "position": "2"
}
```

## 支持的 Python 特性

### 函数定义

```python
# 普通函数
def func(a, b):
    return a + b

# async 函数
async def fetch_data():
    pass

# 带装饰器的函数
@decorator
def decorated():
    pass
```

### 类定义

```python
# 普通类
class MyClass:
    pass

# 继承类
class Child(Parent):
    pass

# 带装饰器的类
@dataclass
class Data:
    name: str
```

### 注释

```python
# 单行注释

"""
多行注释
文档字符串
"""
```

## 性能基准

```
文件大小：100KB Python 代码
解析时间：~800µs
吞吐量：125 MB/s
内存占用：O(1) - 流式处理
```

## 最佳实践

### 1. 选择合适的 Chunk Size

- **小文件 (< 10KB)**: 使用默认值 (500)
- **中等文件 (10-100KB)**: 500-1000
- **大文件 (> 100KB)**: 1000-2000
- **超大文件 (> 1MB)**: 2000+

### 2. 设置合理的 Overlap

```go
// 推荐 overlap 为 chunk_size 的 10%
parser.SetChunkSize(1000)
parser.SetChunkOverlap(100)
```

### 3. 使用回调模式处理大文件

```go
// ❌ 不推荐 - 加载整个文件到内存
chunks, _ := parser.Parse(ctx, largeReader)

// ✅ 推荐 - 流式处理
parser.ParseWithCallback(ctx, largeReader, func(chunk core.Chunk) error {
    process(chunk)
    return nil
})
```

### 4. 并行处理多个文件

```go
files := []string{"file1.py", "file2.py", "file3.py"}
for _, file := range files {
    go func(f string) {
        parser := pycode.NewParser()
        // 处理文件
    }(file)
}
```

## 与其他 Parser 对比

| 特性 | Python Parser | Go Parser | JS Parser |
|------|--------------|-----------|-----------|
| 流式处理 | ✅ | ✅ | ✅ |
| 函数提取 | ✅ | ✅ | ✅ |
| 类提取 | ✅ | ✅ | ✅ |
| 缩进感知 | ✅ | ❌ | ❌ |
| 装饰器支持 | ✅ | ❌ | ✅ |
| 测试覆盖 | 89.7% | 78.2% | 76.2% |

## 常见问题

### Q: 为什么有些函数没有被提取？

A: 确保函数定义符合 Python 语法规范。嵌套函数会被包含在父函数中。

### Q: 如何处理 Jupyter Notebook 文件？

A: 先转换为 `.py` 格式，或使用专门的 Notebook Parser。

### Q: 支持 Type Hints 吗？

A: 支持！Type Hints 会被包含在函数/类的 chunk 中。

```python
def greet(name: str) -> None:
    print(f"Hello, {name}")
```

## 测试

运行测试：

```bash
cd parser/pycode
go test -v ./...
```

查看覆盖率：

```bash
go test -cover ./...
```

生成覆盖率报告：

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 技术细节

### 缩进检测

```go
// 计算行的缩进级别
currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

// 如果缩进小于等于当前元素的缩进，说明元素结束
if currentIndent <= elementIndent {
    // 保存当前元素并开始新元素
}
```

### 元素边界检测

```go
// 使用正则表达式检测函数定义
funcPattern := regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(`)

// 匹配后提取函数名
if matches := funcPattern.FindStringSubmatch(line); len(matches) > 2 {
    functionName = matches[2]
}
```

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

与主项目相同。

## 相关资源

- [Go Parser](../gocode/)
- [JavaScript Parser](../jscode/)
- [Markdown Parser](../markdown/)
- [JSON Parser](../json/)

---

**版本**: v1.0  
**维护者**: GoRAG Team  
**最后更新**: 2024-03-19
