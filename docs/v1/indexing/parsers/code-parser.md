# 代码文件解析器

代码文件解析需要提取注释、文档字符串和代码结构信息。支持 Go、Python、Java、TypeScript、JavaScript 等语言。

> 📋 完整 Metadata 规范：[代码文件 Metadata 提取规范](../parser-metadata.md#代码文件-metadata-gopythonjavatypescriptjavascript)

## 代码文件解析流程（以 Go 为例）

```mermaid
flowchart TD
    A[开始解析] --> B[读取源代码内容]
    B --> C[解析AST抽象语法树]
    C --> D[提取包信息]
    D --> E[提取包注释]
    E --> F[遍历声明节点]
    F --> G{是否为函数/类型声明?}
    G -->|是| H[提取文档注释]
    G -->|否| I[跳过]
    H --> J[累积注释内容]
    I --> J
    J --> K[构建Document对象]
    K --> L[添加到元数据: 包名/导入/描述]
    L --> M[发送到通道]
    M --> N[关闭通道]
```

## 元数据提取策略

- 提取包名（package name）
- 提取包注释作为文档描述
- 提取导入的包列表
- 提取函数和类型的文档注释（doc comments）

## 实现要点

### 1. Go 代码解析

- 使用 `go/parser` 和 `go/ast` 解析 AST
- 提取包名：`f.Name.Name`
- 提取导入：遍历 `f.Imports`
- 提取函数：遍历 `f.Decls` 中的 `*ast.FuncDecl`
- 提取文档注释：`f.Doc.Text()` 和 `fn.Doc.Text()`
- 计算注释占比：注释行数 / 总行数

### 2. Python 代码解析

- 使用 `go-python` 或正则表达式
- 提取模块级 docstring
- 提取函数/类的 docstring
- 统计 import 语句

### 3. Java/TypeScript/JavaScript 解析

- 使用相应的 AST 解析器
- 提取 JSDoc/TSDoc/Javadoc 注释
- 统计类、函数、接口数量
- 保留类型信息（TypeScript）

### 4. 注释提取

- 提取文件头注释
- 提取函数/类/方法注释
- 提取内联注释（可选）
- 保持注释的格式和结构
