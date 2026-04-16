package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/formatter"
	"github.com/DotNetAge/gorag/utils"
)

// UI 终端 UI 工具
type UI struct {
	colors *ColorScheme
}

// ColorScheme 颜色方案
type ColorScheme struct {
	Success string
	Error   string
	Info    string
	Warning string
	Title   string
	Highlight string
	Dim     string
	Reset   string
}

// DefaultColors 默认颜色方案
func DefaultColors() *ColorScheme {
	return &ColorScheme{
		Success:   formatter.Green,
		Error:     formatter.Red,
		Info:      formatter.Cyan,
		Warning:   formatter.Yellow,
		Title:     formatter.Bold + formatter.Magenta,
		Highlight: formatter.Bold + formatter.Cyan,
		Dim:       formatter.Dim,
		Reset:     formatter.Reset,
	}
}

// NewUI 创建 UI 实例
func NewUI() *UI {
	return &UI{colors: DefaultColors()}
}

// Success 打印成功消息
func (ui *UI) Success(format string, args ...any) {
	fmt.Printf("%s✓%s %s\n", ui.colors.Success, ui.colors.Reset, fmt.Sprintf(format, args...))
}

// Error 打印错误消息
func (ui *UI) Error(format string, args ...any) {
	fmt.Printf("%s✗%s %s\n", ui.colors.Error, ui.colors.Reset, fmt.Sprintf(format, args...))
}

// Info 打印信息消息
func (ui *UI) Info(format string, args ...any) {
	fmt.Printf("%sℹ%s %s\n", ui.colors.Info, ui.colors.Reset, fmt.Sprintf(format, args...))
}

// Warning 打印警告消息
func (ui *UI) Warning(format string, args ...any) {
	fmt.Printf("%s!%s %s\n", ui.colors.Warning, ui.colors.Reset, fmt.Sprintf(format, args...))
}

// Title 打印标题
func (ui *UI) Title(format string, args ...any) {
	fmt.Printf("\n%s%s%s\n", ui.colors.Title, fmt.Sprintf(format, args...), ui.colors.Reset)
}

// Section 打印分节
func (ui *UI) Section(name string) {
	fmt.Printf("\n%s▸ %s%s\n", ui.colors.Highlight, name, ui.colors.Reset)
}

// Item 打印列表项
func (ui *UI) Item(format string, args ...any) {
	fmt.Printf("  %s•%s %s\n", ui.colors.Dim, ui.colors.Reset, fmt.Sprintf(format, args...))
}

// KeyValue 打印键值对
func (ui *UI) KeyValue(key, value string) {
	fmt.Printf("  %s%s:%s %s\n", ui.colors.Dim, key, ui.colors.Reset, value)
}

// ProgressBar 进度条
type ProgressBar struct {
	total     int64
	current   int64
	width     int
	desc      string
	startTime time.Time
	ui        *UI
}

// NewProgressBar 创建进度条
func (ui *UI) NewProgressBar(desc string, total int64) *ProgressBar {
	return &ProgressBar{
		total:     total,
		current:   0,
		width:     40,
		desc:      desc,
		startTime: time.Now(),
		ui:        ui,
	}
}

// Update 更新进度
func (p *ProgressBar) Update(current int64) {
	p.current = current
	p.render()
}

// Increment 增加进度
func (p *ProgressBar) Increment(n int64) {
	p.current += n
	p.render()
}

// Done 完成进度
func (p *ProgressBar) Done() {
	p.current = p.total
	p.render()
	fmt.Println()
}

// render 渲染进度条
func (p *ProgressBar) render() {
	percentage := float64(0)
	if p.total > 0 {
		percentage = float64(p.current) / float64(p.total) * 100
	}

	// 进度条
	completed := int(percentage / 100 * float64(p.width))
	bar := strings.Repeat("█", completed) + strings.Repeat("░", p.width-completed)

	// 大小格式化
	currentStr := formatBytes(p.current)
	totalStr := formatBytes(p.total)

	// 时间计算
	elapsed := time.Since(p.startTime).Seconds()
	var speed string
	if elapsed > 0 && p.current > 0 {
		speedBps := float64(p.current) / elapsed
		speed = formatBytes(int64(speedBps)) + "/s"
	} else {
		speed = "---"
	}

	// 清除行并重新打印
	fmt.Printf("\r%s", strings.Repeat(" ", 100))
	fmt.Printf("\r%s %s %s│%s%s%s│ %s%.1f%%%s %s/%s [%s]",
		p.ui.colors.Info,
		p.desc,
		p.ui.colors.Dim,
		p.ui.colors.Success,
		bar,
		p.ui.colors.Dim,
		p.ui.colors.Highlight,
		percentage,
		p.ui.colors.Dim,
		currentStr,
		totalStr,
		speed,
	)
}

// formatBytes 格式化字节大小
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Spinner 旋转加载动画
type Spinner struct {
	frames  []string
	current int
	message string
	ui      *UI
	running bool
	stopCh  chan struct{}
}

// NewSpinner 创建旋转动画
func (ui *UI) NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		message: message,
		ui:      ui,
		running: false,
		stopCh:  make(chan struct{}),
	}
}

// Start 开始动画
func (s *Spinner) Start() {
	s.running = true
	go func() {
		for s.running {
			fmt.Printf("\r%s%s%s %s%s", s.ui.colors.Info, s.frames[s.current], s.ui.colors.Reset, s.message, strings.Repeat(" ", 10))
			s.current = (s.current + 1) % len(s.frames)
			time.Sleep(80 * time.Millisecond)
		}
	}()
}

// Stop 停止动画
func (s *Spinner) Stop() {
	s.running = false
	fmt.Printf("\r%s", strings.Repeat(" ", 80))
	fmt.Printf("\r")
}

// UpdateMessage 更新消息
func (s *Spinner) UpdateMessage(message string) {
	s.message = message
}

// DownloadObserver 下载观察者（UI 实现）
type DownloadObserver struct {
	progressBars map[string]*ProgressBar
	currentFile  string
	ui           *UI
}

// NewDownloadObserver 创建下载观察者
func NewDownloadObserver() *DownloadObserver {
	return &DownloadObserver{
		progressBars: make(map[string]*ProgressBar),
		ui:           NewUI(),
	}
}

// OnEvent 处理下载事件
func (o *DownloadObserver) OnEvent(event utils.DownloadEvent) {
	switch event.Type {
	case utils.EventStart:
		if event.File != "" {
			// 文件开始下载
			o.progressBars[event.File] = o.ui.NewProgressBar(
				fmt.Sprintf("下载 %s", filepath.Base(event.File)),
				event.Total,
			)
			o.currentFile = event.File
		} else {
			// 整体开始
			fmt.Println(event.Message)
		}

	case utils.EventProgress:
		if bar, exists := o.progressBars[event.File]; exists {
			bar.Update(event.Current)
		}

	case utils.EventComplete:
		if bar, exists := o.progressBars[event.File]; exists {
			bar.Done()
		}

	case utils.EventError:
		o.ui.Error("%s: %s", event.File, event.Message)

	case utils.EventAllComplete:
		// 全部完成，无需额外处理
	}
}
