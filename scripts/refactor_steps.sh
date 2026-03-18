#!/bin/bash

# GoRAG Steps 重构脚本 - 将 NewXxx 函数改为包级别函数，结构体小写化

echo "🔧 开始重构 Steps 库..."

# vector/search.go
echo "📝 Refactoring vector/search.go..."
sed -i '' 's/type Search struct {/type search struct {/g' infra/steps/vector/search.go
sed -i '' 's/func NewSearch(/func Search(/g' infra/steps/vector/search.go
sed -i '' 's/) \*Search {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/vector/search.go
sed -i '' 's/return &Search{/return \&search{/g' infra/steps/vector/search.go
sed -i '' 's/func (s \*Search)/func (s *search)/g' infra/steps/vector/search.go

# sparse/search.go
echo "📝 Refactoring sparse/search.go..."
sed -i '' 's/type Search struct {/type search struct {/g' infra/steps/sparse/search.go
sed -i '' 's/func NewSearch(/func Search(/g' infra/steps/sparse/search.go
sed -i '' 's/) \*Search {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/sparse/search.go
sed -i '' 's/return &Search{/return \&search{/g' infra/steps/sparse/search.go
sed -i '' 's/func (s \*Search)/func (s *search)/g' infra/steps/sparse/search.go

# graph/search_local.go
echo "📝 Refactoring graph/search_local.go..."
sed -i '' 's/type Local struct {/type local struct {/g' infra/steps/graph/search_local.go
sed -i '' 's/func NewLocal(/func Local(/g' infra/steps/graph/search_local.go
sed -i '' 's/) \*Local {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/graph/search_local.go
sed -i '' 's/return &Local{/return \&local{/g' infra/steps/graph/search_local.go
sed -i '' 's/func (s \*Local)/func (s *local)/g' infra/steps/graph/search_local.go

# graph/search_global.go
echo "📝 Refactoring graph/search_global.go..."
sed -i '' 's/type Global struct {/type global struct {/g' infra/steps/graph/search_global.go
sed -i '' 's/func NewGlobal(/func Global(/g' infra/steps/graph/search_global.go
sed -i '' 's/) \*Global {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/graph/search_global.go
sed -i '' 's/return &Global{/return \&global{/g' infra/steps/graph/search_global.go
sed -i '' 's/func (s \*Global)/func (s *global)/g' infra/steps/graph/search_global.go

# image/search.go
echo "📝 Refactoring image/search.go..."
sed -i '' 's/type Search struct {/type search struct {/g' infra/steps/image/search.go
sed -i '' 's/func NewSearch(/func Search(/g' infra/steps/image/search.go
sed -i '' 's/) \*Search {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/image/search.go
sed -i '' 's/return &Search{/return \&search{/g' infra/steps/image/search.go
sed -i '' 's/func (s \*Search)/func (s *search)/g' infra/steps/image/search.go

# fuse/rrf.go
echo "📝 Refactoring fuse/rrf.go..."
sed -i '' 's/type RRF struct {/type rrf struct {/g' infra/steps/fuse/rrf.go
sed -i '' 's/func NewRRF(/func RRF(/g' infra/steps/fuse/rrf.go
sed -i '' 's/) \*RRF {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/fuse/rrf.go
sed -i '' 's/return &RRF{/return \&rrf{/g' infra/steps/fuse/rrf.go
sed -i '' 's/func (s \*RRF)/func (s *rrf)/g' infra/steps/fuse/rrf.go

# crag/evaluate.go
echo "📝 Refactoring crag/evaluate.go..."
sed -i '' 's/type Evaluate struct {/type evaluate struct {/g' infra/steps/crag/evaluate.go
sed -i '' 's/func NewEvaluate(/func Evaluate(/g' infra/steps/crag/evaluate.go
sed -i '' 's/) \*Evaluate {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/crag/evaluate.go
sed -i '' 's/return &Evaluate{/return \&evaluate{/g' infra/steps/crag/evaluate.go
sed -i '' 's/func (s \*Evaluate)/func (s *evaluate)/g' infra/steps/crag/evaluate.go

# dedup/dedup.go
echo "📝 Refactoring dedup/dedup.go..."
sed -i '' 's/type Unique struct {/type unique struct {/g' infra/steps/dedup/dedup.go
sed -i '' 's/func NewUnique(/func Unique(/g' infra/steps/dedup/dedup.go
sed -i '' 's/) \*Unique {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/dedup/dedup.go
sed -i '' 's/return &Unique{/return \&unique{/g' infra/steps/dedup/dedup.go
sed -i '' 's/func (s \*Unique)/func (s *unique)/g' infra/steps/dedup/dedup.go

# rerank/score.go
echo "📝 Refactoring rerank/score.go..."
sed -i '' 's/type Score struct {/type score struct {/g' infra/steps/rerank/score.go
sed -i '' 's/func NewScore(/func Score(/g' infra/steps/rerank/score.go
sed -i '' 's/) \*Score {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/rerank/score.go
sed -i '' 's/return &Score{/return \&score{/g' infra/steps/rerank/score.go
sed -i '' 's/func (s \*Score)/func (s *score)/g' infra/steps/rerank/score.go

# rerank/order.go
echo "📝 Refactoring rerank/order.go..."
sed -i '' 's/type Order struct {/type order struct {/g' infra/steps/rerank/order.go
sed -i '' 's/func NewOrder(/func Order(/g' infra/steps/rerank/order.go
sed -i '' 's/) \*Order {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/rerank/order.go
sed -i '' 's/return &Order{/return \&order{/g' infra/steps/rerank/order.go
sed -i '' 's/func (s \*Order)/func (s *order)/g' infra/steps/rerank/order.go

# prune/prune.go
echo "📝 Refactoring prune/prune.go..."
sed -i '' 's/type Prune struct {/type prune struct {/g' infra/steps/prune/prune.go
sed -i '' 's/func NewPrune(/func Prune(/g' infra/steps/prune/prune.go
sed -i '' 's/) \*Prune {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/prune/prune.go
sed -i '' 's/return &Prune{/return \&prune{/g' infra/steps/prune/prune.go
sed -i '' 's/func (s \*Prune)/func (s *prune)/g' infra/steps/prune/prune.go

# generate/generate.go
echo "📝 Refactoring generate/generate.go..."
sed -i '' 's/type Generate struct {/type generate struct {/g' infra/steps/generate/generate.go
sed -i '' 's/func NewGenerate(/func Generate(/g' infra/steps/generate/generate.go
sed -i '' 's/) \*Generate {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/generate/generate.go
sed -i '' 's/return &Generate{/return \&generate{/g' infra/steps/generate/generate.go
sed -i '' 's/func (s \*Generate)/func (s *generate)/g' infra/steps/generate/generate.go

# filter/filter.go
echo "📝 Refactoring filter/filter.go..."
sed -i '' 's/type FromQuery struct {/type fromQuery struct {/g' infra/steps/filter/filter.go
sed -i '' 's/func NewFromQuery(/func FromQuery(/g' infra/steps/filter/filter.go
sed -i '' 's/) \*FromQuery {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/filter/filter.go
sed -i '' 's/return &FromQuery{/return \&fromQuery{/g' infra/steps/filter/filter.go
sed -i '' 's/func (s \*FromQuery)/func (s *fromQuery)/g' infra/steps/filter/filter.go

# stepback/generate.go
echo "📝 Refactoring stepback/generate.go..."
sed -i '' 's/type Generate struct {/type generate struct {/g' infra/steps/stepback/generate.go
sed -i '' 's/func NewGenerate(/func Generate(/g' infra/steps/stepback/generate.go
sed -i '' 's/) \*Generate {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/stepback/generate.go
sed -i '' 's/return &Generate{/return \&generate{/g' infra/steps/stepback/generate.go
sed -i '' 's/func (s \*Generate)/func (s *generate)/g' infra/steps/stepback/generate.go

# hyde/generate.go
echo "📝 Refactoring hyde/generate.go..."
sed -i '' 's/type Generate struct {/type generate struct {/g' infra/steps/hyde/generate.go
sed -i '' 's/func NewGenerate(/func Generate(/g' infra/steps/hyde/generate.go
sed -i '' 's/) \*Generate {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/hyde/generate.go
sed -i '' 's/return &Generate{/return \&generate{/g' infra/steps/hyde/generate.go
sed -i '' 's/func (s \*Generate)/func (s *generate)/g' infra/steps/hyde/generate.go

# rewrite/rewrite.go
echo "📝 Refactoring rewrite/rewrite.go..."
sed -i '' 's/type Rewrite struct {/type rewrite struct {/g' infra/steps/rewrite/rewrite.go
sed -i '' 's/func NewRewrite(/func Rewrite(/g' infra/steps/rewrite/rewrite.go
sed -i '' 's/) \*Rewrite {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/rewrite/rewrite.go
sed -i '' 's/return &Rewrite{/return \&rewrite{/g' infra/steps/rewrite/rewrite.go
sed -i '' 's/func (s \*Rewrite)/func (s *rewrite)/g' infra/steps/rewrite/rewrite.go

# decompose/decompose.go
echo "📝 Refactoring decompose/decompose.go..."
sed -i '' 's/type Decompose struct {/type decompose struct {/g' infra/steps/decompose/decompose.go
sed -i '' 's/func NewDecompose(/func Decompose(/g' infra/steps/decompose/decompose.go
sed -i '' 's/) \*Decompose {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/decompose/decompose.go
sed -i '' 's/return &Decompose{/return \&decompose{/g' infra/steps/decompose/decompose.go
sed -i '' 's/func (s \*Decompose)/func (s *decompose)/g' infra/steps/decompose/decompose.go

# cache/cache.go
echo "📝 Refactoring cache/cache.go..."
sed -i '' 's/type Check struct {/type check struct {/g' infra/steps/cache/cache.go
sed -i '' 's/func NewCheck(/func Check(/g' infra/steps/cache/cache.go
sed -i '' 's/) \*Check {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/cache/cache.go
sed -i '' 's/return &Check{/return \&check{/g' infra/steps/cache/cache.go
sed -i '' 's/func (s \*Check)/func (s *check)/g' infra/steps/cache/cache.go

# cache/store.go
echo "📝 Refactoring cache/store.go..."
sed -i '' 's/type Store struct {/type store struct {/g' infra/steps/cache/store.go
sed -i '' 's/func NewStore(/func Store(/g' infra/steps/cache/store.go
sed -i '' 's/) \*Store {/) pipeline.Step[*entity.PipelineState] {/g' infra/steps/cache/store.go
sed -i '' 's/return &Store{/return \&store{/g' infra/steps/cache/store.go
sed -i '' 's/func (s \*Store)/func (s *store)/g' infra/steps/cache/store.go

# indexing steps
echo "📝 Refactoring indexing/discover.go..."
sed -i '' 's/type Discover struct {/type discover struct {/g' infra/steps/indexing/discover.go
sed -i '' 's/func NewDiscover(/func Discover(/g' infra/steps/indexing/discover.go
sed -i '' 's/) \*Discover {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/discover.go
sed -i '' 's/return &Discover{/return \&discover{/g' infra/steps/indexing/discover.go
sed -i '' 's/func (s \*Discover)/func (s *discover)/g' infra/steps/indexing/discover.go

echo "📝 Refactoring indexing/multi.go..."
sed -i '' 's/type Multi struct {/type multi struct {/g' infra/steps/indexing/parse.go
sed -i '' 's/func NewMulti(/func Multi(/g' infra/steps/indexing/parse.go
sed -i '' 's/) \*Multi {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/parse.go
sed -i '' 's/return &Multi{/return \&multi{/g' infra/steps/indexing/parse.go
sed -i '' 's/func (s \*Multi)/func (s *multi)/g' infra/steps/indexing/parse.go

echo "📝 Refactoring indexing/semantic.go..."
sed -i '' 's/type Semantic struct {/type semantic struct {/g' infra/steps/indexing/chunk.go
sed -i '' 's/func NewSemantic(/func Semantic(/g' infra/steps/indexing/chunk.go
sed -i '' 's/) \*Semantic {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/chunk.go
sed -i '' 's/return &Semantic{/return \&semantic{/g' infra/steps/indexing/chunk.go
sed -i '' 's/func (s \*Semantic)/func (s *semantic)/g' infra/steps/indexing/chunk.go

echo "📝 Refactoring indexing/batch.go..."
sed -i '' 's/type Batch struct {/type batch struct {/g' infra/steps/indexing/embed.go
sed -i '' 's/func NewBatch(/func Batch(/g' infra/steps/indexing/embed.go
sed -i '' 's/) \*Batch {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/embed.go
sed -i '' 's/return &Batch{/return \&batch{/g' infra/steps/indexing/embed.go
sed -i '' 's/func (s \*Batch)/func (s *batch)/g' infra/steps/indexing/embed.go

echo "📝 Refactoring indexing/upsert.go..."
sed -i '' 's/type Upsert struct {/type upsert struct {/g' infra/steps/indexing/store.go
sed -i '' 's/func NewUpsert(/func Upsert(/g' infra/steps/indexing/store.go
sed -i '' 's/) \*Upsert {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/store.go
sed -i '' 's/return &Upsert{/return \&upsert{/g' infra/steps/indexing/store.go
sed -i '' 's/func (s \*Upsert)/func (s *upsert)/g' infra/steps/indexing/store.go

echo "📝 Refactoring indexing/entities.go..."
sed -i '' 's/type Entities struct {/type entities struct {/g' infra/steps/indexing/extract.go
sed -i '' 's/func NewEntities(/func Entities(/g' infra/steps/indexing/extract.go
sed -i '' 's/) \*Entities {/) pipeline.Step[*indexing.State] {/g' infra/steps/indexing/extract.go
sed -i '' 's/return &Entities{/return \&entities{/g' infra/steps/indexing/extract.go
sed -i '' 's/func (s \*Entities)/func (s *entities)/g' infra/steps/indexing/extract.go

echo "✅ 重构完成！"
