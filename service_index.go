package gorag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/store/meta"
)

// minContentChangeForLLM 触发 LLM 重新处理的最小内容长度变化量（字符数）。
// 已处理过的分片再次发生内容变更时，若长度变化低于此阈值，视为微小改动，
// 跳过 LLM 处理以避免浪费调用。
const minContentChangeForLLM = 50

// ── 索引事件系统 ──────────────────────────────────────────────────

// IndexerEventType 表示索引过程中的事件类型。
type IndexerEventType int

const (
	EventFileChunkStarted  IndexerEventType = iota // 文件开始分块（一个文件一次）
	EventChunkVectorized                            // 分片向量化完成（每个分片一次）
	EventFileChunked                                // 文件分块+向量化全部完成（一个文件一次）
	EventIndexDirComplete                           // 单个目录索引完成
	EventDocLLMProcessed                            // 单个文档 LLM 完善完成（每个 doc 一次）
	EventUpdateDirComplete                          // 单个目录更新完成
)

// IndexerEvent 携带索引过程中各阶段的事件信息。
type IndexerEvent struct {
	Type        IndexerEventType `json:"type"`
	File        string           `json:"file"`         // 当前文件路径
	ChunkID     string           `json:"chunk_id"`     // 分片 ID（分片级别事件）
	ChunkIndex  int              `json:"chunk_index"`  // 分片序号（从 0 开始）
	TotalChunks int              `json:"total_chunks"` // 当前文件总分片数
	FileIndex   int              `json:"file_index"`   // 文件序号（从 0 开始）
	TotalFiles  int              `json:"total_files"`  // 当前目录总文件数
	DocIndex    int              `json:"doc_index"`    // 文档序号（LLM 事件，从 0 开始）
	TotalDocs   int              `json:"total_docs"`   // 总文档数（LLM 事件）
}

// IndexerService 负责文件系统扫描、单文件索引、增量更新与失败记录。
type IndexerService struct {
	svc          *IndexingService
	ProgressFn   func(file string, indexed, total int) // 可选：每处理一个文件后回调
	OnEvent      func(event IndexerEvent)              // 可选：细粒度索引事件回调
}

// Index 对指定路径执行批量索引（快路径，无 LLM）。
// targetPath 可以是文件或目录。
func (s *IndexerService) Index(ctx context.Context, targetPath string) error {
	s.svc.mu.Lock()
	defer s.svc.mu.Unlock()
	return s.reindexChangedFiles(ctx, targetPath)
}

// Update 对已索引的文件分片执行增量 LLM 处理（摘要 + 实体提取）。
//
// 流程：
//  1. 重新索引变更的文件
//  2. 对需要 LLM 处理的分片执行摘要 + 实体提取
func (s *IndexerService) Update(ctx context.Context, path string) error {
	s.svc.mu.Lock()
	defer s.svc.mu.Unlock()

	// 第一阶段：重新索引变更的文件
	s.svc.logger.Info("Update: 第一阶段 - 重新索引变更的文件", "target", path)
	if err := s.reindexChangedFiles(ctx, path); err != nil {
		s.svc.logger.Warn("Update: 重新索引阶段有部分错误", "error", err)
	}

	// 第二阶段：增量 LLM 处理
	hyper, ok := s.svc.indexer.(*indexer.HyperIndexer)
	if !ok {
		s.svc.logger.Info("Update: 索引器不是 HyperIndexer，跳过 LLM 增强（仅完成重新索引）")
		return nil
	}

	admin, ok := s.svc.indexer.(indexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("Update: 索引器不支持管理接口")
	}

	needsLLM, err := s.svc.metaStore.GetChunksNeedingLLM("", false, false, -1)
	if err != nil {
		return fmt.Errorf("Update: 查询需要 LLM 处理的分片失败: %w", err)
	}

	if len(needsLLM) == 0 {
		s.svc.logger.Info("Update: 所有分片已完成 LLM 处理，无需更新")
		return nil
	}

	s.svc.logger.Info("Update: 第二阶段 - 增量 LLM 处理", "待处理分片", len(needsLLM))

	// 按 DocID 分组（避免重复加载同一文档的 chunks）
	byDocID := make(map[string][]*meta.ChunkLLMStatus)
	for _, st := range needsLLM {
		byDocID[st.DocID] = append(byDocID[st.DocID], st)
	}

	// 对 DocID 排序后处理，保证日志与行为可复现
	docIDs := make([]string, 0, len(byDocID))
	for docID := range byDocID {
		docIDs = append(docIDs, docID)
	}
	sort.Strings(docIDs)

	var totalProcessed int
	totalDocCount := len(docIDs)
	for docIdx, docID := range docIDs {
		statuses := byDocID[docID]
		if s.ProgressFn != nil {
			// 取该文档第一个分片的路径作为当前处理文件
			docPath := statuses[0].DocPath
			s.ProgressFn(docPath, docIdx+1, totalDocCount)
		}
		processed, err := s.processDocLLM(ctx, hyper, admin, docID, statuses)
		if err != nil {
			s.svc.logger.Warn("Update: 处理文档失败", "doc_id", docID, "error", err)
			continue
		}
		totalProcessed += processed

		// 发射单个文档 LLM 完善完成事件
		if s.OnEvent != nil {
			docPath := statuses[0].DocPath
			s.OnEvent(IndexerEvent{
				Type:      EventDocLLMProcessed,
				File:      docPath,
				DocIndex:  docIdx,
				TotalDocs: totalDocCount,
			})
		}
	}

	// 发射目录更新完成事件
	if s.OnEvent != nil {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			s.OnEvent(IndexerEvent{
				Type:       EventUpdateDirComplete,
				File:       path,
				TotalFiles: totalDocCount,
			})
		}
	}

	s.svc.logger.Info("Update: 完成", "LLM 增强分片", totalProcessed)
	return nil
}

// processDocLLM 对单个文档中需要 LLM 处理的分片执行摘要 + 实体提取，
// 并持久化状态。返回实际处理的分片数量。
func (s *IndexerService) processDocLLM(ctx context.Context, hyper *indexer.HyperIndexer, admin indexer.IndexerAdmin, docID string, statuses []*meta.ChunkLLMStatus) (int, error) {
	allChunks, err := admin.GetChunks(ctx, docID)
	if err != nil {
		return 0, fmt.Errorf("加载文档分片失败: %w", err)
	}

	chunkByID := make(map[string]*core.Chunk, len(allChunks))
	for _, c := range allChunks {
		chunkByID[c.ID] = c
	}

	var toProcess []core.Chunk
	for _, st := range statuses {
		chunk, ok := chunkByID[st.ChunkID]
		if !ok {
			s.svc.logger.Debug("Update: 分片不在 VectorStore 中，跳过", "chunk_id", st.ChunkID)
			continue
		}

		// 已处理过的分片：检查是否需要重新处理
		if st.Summarized && st.Refilled {
			currentHash := computeChunkContentHash(chunk.Content)
			if currentHash == st.ContentHash {
				continue // 内容无变更
			}
			diff := abs(utf8.RuneCountInString(chunk.Content) - st.ContentLength)
			if diff < minContentChangeForLLM {
				s.svc.logger.Debug("Update: 分片内容微小变更，跳过 LLM",
					"chunk_id", st.ChunkID, "长度变化", diff)
				continue
			}
			s.svc.logger.Info("Update: 分片内容实质性变更，重新 LLM 处理",
				"chunk_id", st.ChunkID, "长度变化", diff)
		}

		toProcess = append(toProcess, *chunk)
	}

	if len(toProcess) == 0 {
		return 0, nil
	}

	s.svc.logger.Info("Update: 处理文档分片", "doc_id", docID, "分片数", len(toProcess))
	processedChunks, _, _, pErr := hyper.ProcessChunks(ctx, toProcess)
	if pErr != nil {
		return 0, fmt.Errorf("LLM 处理失败: %w", pErr)
	}

	processedIDs := make(map[string]bool, len(toProcess))
	for _, c := range toProcess {
		processedIDs[c.ID] = true
	}

	if err := s.saveProcessedLLMStatus(statuses, processedChunks, processedIDs); err != nil {
		return 0, err
	}
	return len(processedChunks), nil
}

// saveProcessedLLMStatus 将 ProcessChunks 返回的分片状态持久化到 meta.db。
func (s *IndexerService) saveProcessedLLMStatus(statuses []*meta.ChunkLLMStatus, processed []core.Chunk, processedIDs map[string]bool) error {
	now := time.Now()
	for _, pc := range processed {
		if !processedIDs[pc.ID] {
			continue
		}
		st := findStatus(statuses, pc.ID)
		if st == nil {
			continue
		}

		update := &meta.ChunkLLMStatus{
			ChunkID:       pc.ID,
			DocPath:       st.DocPath,
			DocID:         st.DocID,
			ContentHash:   computeChunkContentHash(pc.Content),
			ContentLength: len(pc.Content),
			Summarized:    true,
			Refilled:      true,
		}
		if !st.Summarized {
			update.LastSummarizedAt = &now
		} else {
			update.LastSummarizedAt = st.LastSummarizedAt
		}
		if !st.Refilled {
			update.LastRefilledAt = &now
		} else {
			update.LastRefilledAt = st.LastRefilledAt
		}

		if err := s.svc.metaStore.SaveChunkLLMStatus(update); err != nil {
			return fmt.Errorf("保存分片 %s 状态失败: %w", pc.ID, err)
		}
	}
	return nil
}

// reindexChangedFiles 对指定路径执行增量重新索引（扫描、分块、向量化）。
//
// 复用 Index 的核心扫描 + worker pool 逻辑，但不加锁（调用方负责上锁）。
// 用于 Index 和 Update 双阶段流程的第一阶段。
//
// targetPath 可以是文件或目录。
func (s *IndexerService) reindexChangedFiles(ctx context.Context, targetPath string) error {
	// 0. 加载 .ragignore 规则
	ragignore := loadRagignore(s.svc.dataDir)

	// 1. 扫描文件
	var files []string
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("无法访问目标路径: %w", err)
	}
	if info.IsDir() {
		files, err = scanDir(targetPath, ragignore)
	} else {
		if isTextFile(targetPath) {
			files = append(files, targetPath)
		}
	}
	if err != nil {
		return fmt.Errorf("扫描文件失败: %w", err)
	}

	if len(files) == 0 {
		s.svc.logger.Info("无待索引文件")
		return nil
	}

	s.svc.logger.Info("开始重新索引变更的文件", "target", targetPath, "files", len(files))

	// 2a. 预写 pending 状态：扫描到的新文件标记为「待索引」
	for _, file := range files {
		existing, err := s.svc.metaStore.GetDocumentByPath(file)
		if err != nil || existing == nil {
			// 新文件 → 写入 pending
			info, statErr := os.Stat(file)
			if statErr != nil {
				continue
			}
			_ = s.svc.metaStore.SaveDocument(&meta.Document{
				AbsolutePath: file,
				FileName:     info.Name(),
				Extension:    filepath.Ext(file),
				SizeBytes:    info.Size(),
				ModifiedAt:   info.ModTime(),
				Status:       meta.DocStatusPending,
			})
		} else if existing.Status == meta.DocStatusPending || existing.Status == meta.DocStatusIndexing {
			// 上次异常中断留下的 pending/indexing → 保留
			continue
		}
	}

	// 2. 使用 worker pool 并发索引
	workerCount := 4
	jobs := make(chan string, len(files))
	results := make(chan indexResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.indexWorker(ctx, &wg, jobs, results)
	}

	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var completed int32
	var failedCount int
	for result := range results {
		idx := atomic.AddInt32(&completed, 1)
		if s.ProgressFn != nil {
			s.ProgressFn(result.file, int(idx), len(files))
		}
		if result.err != nil {
			failedCount++
			s.svc.logger.Error("索引文件失败", result.err, "file", result.file)
		} else {
			s.svc.logger.Info("文件索引成功", "file", result.file, "chunk_count", result.count)
		}
	}

	s.svc.logger.Info("重新索引完成",
		"total", len(files),
		"failed", failedCount,
		"success", len(files)-failedCount)

	// 2c. 发射目录索引完成事件（仅当目标为目录时）
	if s.OnEvent != nil && info.IsDir() {
		s.OnEvent(IndexerEvent{
			Type:       EventIndexDirComplete,
			File:       targetPath,
			TotalFiles: len(files),
		})
	}

	// 3. 为没有 README.md 的目录生成 Region 摘要文件
	if err := s.svc.Region().GenerateMissingReadmes(ctx, targetPath, files, s.processFile); err != nil {
		s.svc.logger.Warn("生成目录 README 失败", "error", err)
	}

	return nil
}

// indexResult 索引结果
type indexResult struct {
	file  string
	count int
	err   error
}

// indexWorker 索引 worker，单个文件 panic 不会拖垮整个 worker pool。
func (s *IndexerService) indexWorker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan string, results chan<- indexResult) {
	defer wg.Done()

	for file := range jobs {
		func(file string) {
			defer func() {
				if r := recover(); r != nil {
					s.svc.logger.Error("索引文件时发生 panic", fmt.Errorf("%v", r), "file", file)
					results <- indexResult{file: file, err: fmt.Errorf("panic: %v", r)}
				}
			}()

			select {
			case <-ctx.Done():
				results <- indexResult{file: file, err: ctx.Err()}
				return
			default:
				chunks, err := s.processFile(ctx, file)
				if err != nil {
					results <- indexResult{file: file, err: err}
					return
				}
				results <- indexResult{
					file:  file,
					count: len(chunks),
				}
			}
		}(file)
	}
}

// processFile 对单个文件执行完整索引流程（含两级增量预检）。
//
// 流程：
//  1. 两级预检：mtime + size → hash
//  2. 清理旧数据
//  3. 调用 indexer.AddFile 写入新数据
//  4. 成功/失败写入 meta.db
func (s *IndexerService) processFile(ctx context.Context, absPath string) ([]*core.Chunk, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat 文件失败: %w", err)
	}

	// 1. 两级预检：先查 mtime+size，不匹配才算 hash
	existing, _ := s.svc.metaStore.GetDocumentByPath(absPath)
	if existing != nil && existing.Status == meta.DocStatusIndexed {
		if existing.ModifiedAt.Equal(info.ModTime()) && existing.SizeBytes == info.Size() {
			s.svc.logger.Info("文件未变更（mtime+size 命中），跳过", "path", absPath)
			return nil, nil
		}
	}

	// 2. 计算内容 hash（仅 mtime/size 变化时计算）
	contentHash, err := computeFileHash(absPath)
	if err != nil {
		s.recordFailure(absPath, "", err)
		return nil, fmt.Errorf("计算文件 hash 失败: %w", err)
	}

	if existing != nil && existing.ContentHash == contentHash && existing.Status == meta.DocStatusIndexed {
		s.svc.logger.Info("文件未变更（hash 命中），跳过", "path", absPath)
		return nil, nil
	}

	// 标记为「索引进行中」
	if err := s.svc.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		ContentHash:  contentHash,
		SizeBytes:    info.Size(),
		ModifiedAt:   info.ModTime(),
		Status:       meta.DocStatusIndexing,
	}); err != nil {
		s.svc.logger.Warn("标记 indexing 状态失败，继续执行", "path", absPath, "error", err)
	}

	// 3. 先清理旧数据
	if existing != nil && len(existing.ChunkIDs) > 0 {
		if removeErr := s.RemoveFile(ctx, absPath); removeErr != nil {
			s.svc.logger.Warn("清理旧索引失败，继续写入新数据", "path", absPath, "err", removeErr)
		}
	}

	// 4. 发射文件开始分块事件
	if s.OnEvent != nil {
		s.OnEvent(IndexerEvent{
			Type: EventFileChunkStarted,
			File: absPath,
		})
	}

	// 5. 调用 indexer.AddFile 写入新数据
	chunks, err := s.svc.indexer.AddFile(ctx, absPath)
	if err != nil {
		s.recordFailure(absPath, contentHash, err)
		return nil, fmt.Errorf("索引文件失败: %w", err)
	}

	// 5b. 发射每个分片的向量化完成事件
	if s.OnEvent != nil {
		for i, c := range chunks {
			s.OnEvent(IndexerEvent{
				Type:        EventChunkVectorized,
				File:        absPath,
				ChunkID:     c.ID,
				ChunkIndex:  i,
				TotalChunks: len(chunks),
			})
		}
	}

	// 6. 成功状态写入 meta.db
	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	if err := s.svc.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		FileName:     info.Name(),
		Extension:    filepath.Ext(absPath),
		SizeBytes:    info.Size(),
		ModifiedAt:   info.ModTime(),
		ContentHash:  contentHash,
		Status:       meta.DocStatusIndexed,
		ChunkIDs:     chunkIDs,
		IndexedAt:    timePtr(time.Now()),
	}); err != nil {
		return nil, fmt.Errorf("保存文档元数据失败: %w", err)
	}

	// 6. 写入 chunk_llm_status（初始状态：未摘要、未实体提取）
	for _, c := range chunks {
		if err := s.svc.metaStore.SaveChunkLLMStatus(&meta.ChunkLLMStatus{
			ChunkID:       c.ID,
			DocPath:       absPath,
			DocID:         c.DocID,
			ContentHash:   computeChunkContentHash(c.Content),
			ContentLength: utf8.RuneCountInString(c.Content),
			Summarized:    false,
			Refilled:      false,
		}); err != nil {
			return nil, fmt.Errorf("保存分片 LLM 状态失败: %w", err)
		}
	}

	// 8. 发射文件分块+向量化全部完成事件
	if s.OnEvent != nil {
		s.OnEvent(IndexerEvent{
			Type:        EventFileChunked,
			File:        absPath,
			TotalChunks: len(chunks),
		})
	}

	return chunks, nil
}

// recordFailure 将索引失败记录写入 meta.db。
func (s *IndexerService) recordFailure(absPath, contentHash string, err error) {
	if saveErr := s.svc.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		ContentHash:  contentHash,
		Status:       meta.DocStatusFailed,
		ErrorMessage: err.Error(),
	}); saveErr != nil {
		s.svc.logger.Warn("保存失败记录到 meta.db 失败", "path", absPath, "error", saveErr)
	}
}

// RemoveFile 删除指定文件的索引记录并清理所有关联 chunks。
// 全部 Remove 成功后才删除 meta.db 记录，部分失败则标记 partial_deleted。
func (s *IndexerService) RemoveFile(ctx context.Context, absPath string) error {
	doc, err := s.svc.metaStore.GetDocumentByPath(absPath)
	if err != nil {
		return fmt.Errorf("查询文档元数据失败: %w", err)
	}
	if doc == nil {
		return nil
	}

	if len(doc.ChunkIDs) == 0 {
		return s.svc.metaStore.DeleteDocument(absPath)
	}

	admin, ok := s.svc.indexer.(indexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("索引器不支持管理接口，无法删除 chunk")
	}

	var failedChunks []string
	for _, chunkID := range doc.ChunkIDs {
		if err := admin.Remove(ctx, chunkID); err != nil {
			s.svc.logger.Error("删除 chunk 失败", err, "chunk_id", chunkID)
			failedChunks = append(failedChunks, chunkID)
		}
	}

	if len(failedChunks) > 0 {
		return s.svc.metaStore.SaveDocument(&meta.Document{
			AbsolutePath: doc.AbsolutePath,
			ContentHash:  doc.ContentHash,
			Status:       meta.DocStatusPartialDeleted,
			ChunkIDs:     failedChunks,
			ErrorMessage: fmt.Sprintf("部分 chunk 删除失败：%d/%d", len(failedChunks), len(doc.ChunkIDs)),
		})
	}

	if err := s.svc.metaStore.DeleteDocument(absPath); err != nil {
		return err
	}
	// 清理 chunk_llm_status
	if err := s.svc.metaStore.DeleteChunkLLMStatusByDocPath(absPath); err != nil {
		s.svc.logger.Warn("清理 chunk_llm_status 失败", "path", absPath, "error", err)
	}
	return nil
}

// RemoveDir 删除指定目录下所有文件的索引记录及关联图数据。
func (s *IndexerService) RemoveDir(ctx context.Context, dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}

	docs, err := s.svc.metaStore.ListDocumentsWithProgress("", absPath)
	if err != nil {
		return fmt.Errorf("查询文档列表失败: %w", err)
	}

	if len(docs) == 0 {
		s.svc.logger.Debug("目录下无已索引的文件", "dir", absPath)
		return nil
	}

	var failed int
	for _, doc := range docs {
		if err := s.RemoveFile(ctx, doc.AbsolutePath); err != nil {
			s.svc.logger.Error("删除文件索引失败", err, "path", doc.AbsolutePath)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("删除目录索引失败：%d/%d 个文件删除失败", failed, len(docs))
	}
	return nil
}

// UnfinishedWorkCount 返回中断未完成的索引任务概况。
// 返回 pending（待索引）和 indexing（索引中）状态的文件数量。
// 系统启动时可调用此方法判断上次是否存在中断任务。
func (s *IndexerService) UnfinishedWorkCount() (pending, indexing int, err error) {
	counts, err := s.svc.metaStore.CountDocumentsByStatus()
	if err != nil {
		return 0, 0, fmt.Errorf("查询文档状态统计失败: %w", err)
	}
	pending = counts[meta.DocStatusPending]
	indexing = counts[meta.DocStatusIndexing]
	return
}

// NeedsUpdate 检查是否有分片尚未经过 LLM 处理（summarized=false 或 refilled=false）。
// 用于系统启动时判断是否需要执行 Update 阶段。
func (s *IndexerService) NeedsUpdate(ctx context.Context) (bool, error) {
	needsLLM, err := s.svc.metaStore.GetChunksNeedingLLM("", false, false, 1)
	if err != nil {
		return false, fmt.Errorf("查询需要 LLM 处理的分片失败: %w", err)
	}
	return len(needsLLM) > 0, nil
}

// UsageStats 返回 token 用量统计（总 tokens + 最新模型名）。
func (s *IndexerService) UsageStats() (totalTokens int64, model string, err error) {
	return s.svc.metaStore.QueryTotalUsageStats()
}

// findStatus 在 ChunkLLMStatus 切片中按 chunkID 查找。
func findStatus(statuses []*meta.ChunkLLMStatus, chunkID string) *meta.ChunkLLMStatus {
	for _, st := range statuses {
		if st.ChunkID == chunkID {
			return st
		}
	}
	return nil
}
