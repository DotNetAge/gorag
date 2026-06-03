import CoreML
import NaturalLanguage
import MiniRAG  // gomobile 生成的 framework

// MARK: - Qwen3 Tokenizer (BPE)

/// Qwen3 分词器，从模型目录加载 vocab.json + merges.txt
final class Qwen3Tokenizer {
    private var vocab: [String: Int] = [:]          // token → id
    private var idToToken: [Int: String] = [:]      // id → token
    private var merges: [(String, String, Int)] = [] // (左, 右, 优先级)
    private var byteEncoder: [UInt8: String] = [:]
    private var byteDecoder: [String: UInt8] = [:]

    private static let specialTokens: [String: Int] = [
        "<|endoftext|>": 151643,
        "<|im_start|>": 151644,
        "<|im_end|>": 151645,
        "<|endoftext|>": 151643,
    ]

    init(modelDir: String) throws {
        try loadVocab(modelDir + "/vocab.json")
        try loadMerges(modelDir + "/merges.txt")
        buildByteEncoders()
    }

    // MARK: - 加载

    private func loadVocab(_ path: String) throws {
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        let json = try JSONSerialization.jsonObject(with: data) as! [String: Int]
        vocab = json
        idToToken = Dictionary(uniqueKeysWithValues: json.map { ($1, $0) })
    }

    private func loadMerges(_ path: String) throws {
        let content = try String(contentsOfFile: path, encoding: .utf8)
        let lines = content.components(separatedBy: .newlines)
        for (idx, line) in lines.enumerated() {
            if line.isEmpty || line.hasPrefix("#") { continue }
            let parts = line.components(separatedBy: " ")
            if parts.count == 2 {
                merges.append((parts[0], parts[1], idx))
            }
        }
    }

    private func buildByteEncoders() {
        let bytes: [UInt8] = Array(33...126) + Array(161...254) + [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,127,128,129,130,131,132,133,134,135,136,137,138,139,140,141,142,143,144,145,146,147,148,149,150,151,152,153,154,155,156,157,158,159,160,255]
        for (i, b) in bytes.enumerated() {
            let ch = String(UnicodeScalar(i + 256)!)
            byteEncoder[b] = ch
            byteDecoder[ch] = b
        }
    }

    // MARK: - BPE 编码

    private func byteEncode(_ text: String) -> [String] {
        let utf8 = Array(text.utf8)
        return utf8.map { byteEncoder[$0] ?? String(UnicodeScalar($0)) }
    }

    private func getPairs(_ word: [String]) -> Set<[String]> {
        var pairs = Set<[String]>()
        for i in 0..<word.count - 1 {
            pairs.insert([word[i], word[i + 1]])
        }
        return pairs
    }

    private func bpe(token: String) -> String {
        var word = byteEncode(token)
        if word.count == 1 { return word[0] }

        while word.count > 1 {
            var minPair: [String]?
            var minRank = Int.max

            for pair in getPairs(word) {
                // 查找 merge 优先级
                if let rank = merges.firstIndex(where: { $0.0 == pair[0] && $0.1 == pair[1] }) {
                    if rank < minRank {
                        minRank = rank
                        minPair = pair
                    }
                }
            }

            guard let best = minPair else { break }

            var newWord: [String] = []
            var i = 0
            while i < word.count {
                if i < word.count - 1 && word[i] == best[0] && word[i + 1] == best[1] {
                    newWord.append(best[0] + best[1])
                    i += 2
                } else {
                    newWord.append(word[i])
                    i += 1
                }
            }
            word = newWord
        }
        return word.joined(separator: " ")
    }

    // MARK: - 公开接口

    func encode(_ text: String, maxLength: Int = 128) -> (inputIds: [Int], attentionMask: [Int]) {
        // 处理特殊 token
        var processed = text
        for (token, id) in Self.specialTokens {
            processed = processed.replacingOccurrences(of: token, with: " \(token) ")
        }

        let words = processed.components(separatedBy: .whitespacesAndNewlines).filter { !$0.isEmpty }
        var tokens: [Int] = []

        for word in words {
            if let sid = Self.specialTokens[word] {
                tokens.append(sid)
                continue
            }
            let bpeTokens = bpe(token: word).components(separatedBy: " ")
            for bt in bpeTokens {
                if let id = vocab[bt] {
                    tokens.append(id)
                }
            }
        }

        // 固定长度：截断或填充
        let eodId = Self.specialTokens["<|endoftext|>"]!
        tokens.append(eodId)  // 追加 EOS

        let actualLen = min(tokens.count, maxLength)
        let truncated = Array(tokens.prefix(maxLength))
        let padLen = max(0, maxLength - actualLen)

        let inputIds = truncated + [Int](repeating: 0, count: padLen)
        let attentionMask = [Int](repeating: 1, count: actualLen) + [Int](repeating: 0, count: padLen)

        return (inputIds, attentionMask)
    }
}

// MARK: - Qwen3 Core ML Embedder

/// 用 Qwen3 Core ML 模型计算文本嵌入向量
final class Qwen3Embedder {
    private let model: MLModel
    private let tokenizer: Qwen3Tokenizer
    private let maxLength: Int

    /// 输出维度（Qwen3-Embedding-0.6B = 1024）
    let dimension: Int

    init(modelURL: URL, tokenizerDir: String, maxLength: Int = 128) throws {
        self.maxLength = maxLength
        self.dimension = 1024
        self.tokenizer = try Qwen3Tokenizer(modelDir: tokenizerDir)

        let config = MLModelConfiguration()
        config.computeUnits = .cpuAndNeuralEngine  // iPhone 17 ANE
        self.model = try MLModel(contentsOf: modelURL, configuration: config)
    }

    /// 将文本转为嵌入向量，返回 float32 小端字节序 Data
    func embedText(_ text: String) throws -> Data {
        let (inputIds, attentionMask) = tokenizer.encode(text, maxLength: maxLength)

        // 构造 Core ML 输入
        let inputArray = try MLMultiArray(
            shape: [1, maxLength as NSNumber],
            dataType: .int32
        )
        let maskArray = try MLMultiArray(
            shape: [1, maxLength as NSNumber],
            dataType: .int32
        )

        for i in 0..<maxLength {
            inputArray[i] = NSNumber(value: inputIds[i])
            maskArray[i] = NSNumber(value: attentionMask[i])
        }

        let input = try MLDictionaryFeatureProvider(
            dictionary: [
                "input_ids": inputArray,
                "attention_mask": maskArray,
            ]
        )

        // 推理
        let output = try model.prediction(from: input)

        // 取出嵌入向量（输出名 var_5199）
        let embedding = output.featureValue(for: "var_5199")!.multiArrayValue!
        let count = embedding.count

        // 转为 float32 小端字节序 Data
        var bytes = Data(capacity: count * 4)
        let ptr = UnsafeMutablePointer<Float32>.allocate(capacity: count)
        ptr.initialize(repeating: 0, count: count)
        for i in 0..<count {
            ptr[i] = embedding[i].floatValue
        }
        bytes.append(ptr, count: count * 4)
        ptr.deallocate()

        return bytes
    }
}

// MARK: - MiniRAG SwiftUI 使用

struct Chunk: Decodable {
    let id: String
    let content: String
}

struct Hit: Decodable {
    let id: String
    let content: String
    let score: Double
}

/// MiniRAG 的 Swift 封装（隐藏 JSON 序列化细节）
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

// MARK: - SwiftUI 示例页面

import SwiftUI

@MainActor
final class RAGViewModel: ObservableObject {
    @Published var documents: [String] = []
    @Published var queryText = ""
    @Published var results: [Hit] = []
    @Published var status = ""
    @Published var isReady = false

    private var rag: MiniRAG?
    private var embedder: Qwen3Embedder?

    func setup() {
        Task {
            do {
                status = "正在加载 Qwen3 模型..."

                let modelDir = FileManager.default.urls(
                    for: .documentDirectory, in: .userDomainMask
                ).first!.appendingPathComponent("qwen3")

                // 从 app bundle 复制模型到文档目录（首次运行）
                try copyBundleModelIfNeeded(to: modelDir)

                // 初始化 Qwen3 Embedder
                let modelURL = modelDir.appendingPathComponent("Qwen3Embedding_int8.mlpackage")
                embedder = try Qwen3Embedder(
                    modelURL: modelURL,
                    tokenizerDir: modelDir.path
                )

                // 初始化 MiniRAG
                let dataDir = modelDir.appendingPathComponent("minirag_data").path
                rag = try MiniRAG(
                    dataDir: dataDir,
                    dimension: embedder!.dimension,
                    embedder: MiniragEmbedderProtocol { [weak self] text in
                        try await self?.embedder?.embedText(text) ?? Data()
                    }
                )

                isReady = true
                status = "✅ 就绪"
            } catch {
                status = "❌ \(error.localizedDescription)"
            }
        }
    }

    func addDocument(_ text: String) {
        guard let rag else { return }
        Task {
            do {
                status = "正在索引..."
                let chunks = try rag.addIndex(text)
                documents.append(text)
                status = "✅ 已索引 \(chunks.count) 个分片"
            } catch {
                status = "❌ \(error.localizedDescription)"
            }
        }
    }

    func search() {
        guard let rag, !queryText.isEmpty else { return }
        Task {
            do {
                status = "搜索中..."
                let hits = try rag.search(queryText, topK: 5)
                results = hits
                status = "✅ 找到 \(hits.count) 条结果"
            } catch {
                status = "❌ \(error.localizedDescription)"
            }
        }
    }

    private func copyBundleModelIfNeeded(to dest: URL) throws {
        let fm = FileManager.default
        if !fm.fileExists(atPath: dest.path) {
            try fm.createDirectory(at: dest, withIntermediateDirectories: true)

            // 假设模型在 app bundle 的 Models 目录下
            if let bundleURL = Bundle.main.url(forResource: "Qwen3Embedding_int8", withExtension: "mlpackage") {
                try fm.copyItem(at: bundleURL, to: dest.appendingPathComponent("Qwen3Embedding_int8.mlpackage"))
            }
            // 复制 tokenizer 文件
            for file in ["vocab.json", "merges.txt", "tokenizer.json", "config.json"] {
                if let url = Bundle.main.url(forResource: file, withExtension: nil, subdirectory: "Models") {
                    try fm.copyItem(at: url, to: dest.appendingPathComponent(file))
                }
            }
        }
    }
}

struct ContentView: View {
    @StateObject private var vm = RAGViewModel()
    @State private var newDocText = ""

    var body: some View {
        NavigationStack {
            List {
                // 状态
                Section {
                    HStack {
                        Circle()
                            .fill(vm.isReady ? Color.green : Color.gray)
                            .frame(width: 10, height: 10)
                        Text(vm.status)
                            .font(.caption)
                    }
                }

                // 添加文档
                Section("添加文档") {
                    TextField("输入文本...", text: $newDocText, axis: .vertical)
                        .lineLimit(3)
                    Button("索引") {
                        vm.addDocument(newDocText)
                        newDocText = ""
                    }
                    .disabled(!vm.isReady || newDocText.isEmpty)

                    ForEach(vm.documents, id: \.self) { doc in
                        Text(doc).font(.caption).lineLimit(2)
                    }
                }

                // 搜索
                Section("搜索") {
                    TextField("输入查询...", text: $vm.queryText)
                    Button("搜索") { vm.search() }
                        .disabled(!vm.isReady || vm.queryText.isEmpty)
                }

                // 结果
                Section("结果 (\(vm.results.count))") {
                    ForEach(Array(vm.results.enumerated()), id: \.offset) { _, hit in
                        VStack(alignment: .leading) {
                            Text(hit.content).font(.body)
                            Text("score: \(String(format: "%.3f", hit.score))")
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    }
                }
            }
            .navigationTitle("MiniRAG + Qwen3")
        }
        .onAppear { vm.setup() }
    }
}

// MARK: - 闭包转 gomobile protocol 适配

/// MiniragEmbedderProtocol 的闭包封装
/// gomobile 生成的 ObjC 协议需要 NSObject 子类实现
final class ClosureEmbedder: MiniragEmbedderProtocol {
    private let block: (String) throws -> Data

    init(_ block: @escaping (String) throws -> Data) {
        self.block = block
    }

    func embedText(_ text: String) throws -> Data {
        try block(text)
    }
}
