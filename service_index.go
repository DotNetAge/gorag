package gorag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
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

// IndexerService 负责文件系统扫描、单文件索引、增量更新与失败记录。
type IndexerService struct {
	svc *IndexingService
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
	for _, docID := range docIDs {
		statuses := byDocID[docID]
		processed, err := s.processDocLLM(ctx, hyper, admin, docID, statuses)
		if err != nil {
			s.svc.logger.Warn("Update: 处理文档失败", "doc_id", docID, "error", err)
			continue
		}
		totalProcessed += processed
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

	var failedCount int
	for result := range results {
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
	if existing != nil && existing.Status == "indexed" {
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

	if existing != nil && existing.ContentHash == contentHash && existing.Status == "indexed" {
		s.svc.logger.Info("文件未变更（hash 命中），跳过", "path", absPath)
		return nil, nil
	}

	// 3. 先清理旧数据
	if existing != nil && len(existing.ChunkIDs) > 0 {
		if removeErr := s.removeFileIndex(ctx, absPath); removeErr != nil {
			s.svc.logger.Warn("清理旧索引失败，继续写入新数据", "path", absPath, "err", removeErr)
		}
	}

	// 4. 调用 indexer.AddFile 写入新数据
	chunks, err := s.svc.indexer.AddFile(ctx, absPath)
	if err != nil {
		s.recordFailure(absPath, contentHash, err)
		return nil, fmt.Errorf("索引文件失败: %w", err)
	}

	// 5. 成功状态写入 meta.db
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
		Status:       "indexed",
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

	return chunks, nil
}

// recordFailure 将索引失败记录写入 meta.db。
func (s *IndexerService) recordFailure(absPath, contentHash string, err error) {
	if saveErr := s.svc.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		ContentHash:  contentHash,
		Status:       "failed",
		ErrorMessage: err.Error(),
	}); saveErr != nil {
		s.svc.logger.Warn("保存失败记录到 meta.db 失败", "path", absPath, "error", saveErr)
	}
}

// removeFileIndex 删除文件的索引记录并清理所有关联 chunks。
// 全部 Remove 成功后才删除 meta.db 记录，部分失败则标记 partial_deleted。
func (s *IndexerService) removeFileIndex(ctx context.Context, absPath string) error {
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
			Status:       "partial_deleted",
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

// findStatus 在 ChunkLLMStatus 切片中按 chunkID 查找。
func findStatus(statuses []*meta.ChunkLLMStatus, chunkID string) *meta.ChunkLLMStatus {
	for _, st := range statuses {
		if st.ChunkID == chunkID {
			return st
		}
	}
	return nil
}
