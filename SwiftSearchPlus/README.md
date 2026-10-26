# SwiftSearch Plus

**An independent Windows x64 file-search utility written in Go and Win32.**

SwiftSearch Plus is an unofficial, independent implementation inspired by the need for fast NTFS file search with a more readable interface. It is not the original SwiftSearch C++ project, and it is not affiliated with or endorsed by the original author.

- [简体中文](#简体中文)
- [English](#english)

> **Status: experimental.** The code can be cross-compiled for Windows x64, but it has not yet completed comprehensive runtime testing on a real Windows system. Please test carefully before relying on it for important work.

## 简体中文

### 功能

- 原生 Windows Win32 图形界面，支持简体中文和 English。
- 在本地 NTFS 卷上读取文件系统元数据进行搜索，需要管理员权限；启动时会请求 UAC 提权。
- 文件名或路径匹配，支持通配符匹配。
- 按文件类别过滤：全部文件、仅文件、仅文件夹、图片、视频、音频、文档、压缩包、程序/安装包。
- 自定义包含扩展名和排除扩展名。
- 结果列包含名称、类型、大小、创建时间、最后写入时间、上次访问时间和完整路径。
- 列标题排序、右键菜单、打开文件、在资源管理器中定位及复制路径。
- 本地索引缓存与 USN Journal 增量更新逻辑。
- 不依赖第三方 Go 模块。

### 系统要求

- Windows 10/11 x64。
- NTFS 卷。FAT、exFAT、网络共享和其他非 NTFS 文件系统不在支持范围内。
- 搜索 NTFS 卷需要管理员权限；程序会尝试通过 UAC 重新启动自身。
- 从源码编译需要 Go 1.23+ 和 Python 3（用于嵌入应用图标及 DPI manifest）。

### 从源码编译

1. 安装 [Go](https://go.dev/dl/) 1.23 或更新版本。
2. 安装 [Python 3](https://www.python.org/downloads/) 并启用 `py` 启动器。
3. 克隆或下载本仓库。
4. 在 Windows 上双击 `build.cmd`。
5. 成品生成于 `build/SwiftSearchPlus.exe`。

也可以在仓库目录中运行普通的 Go 编译命令（这样不会嵌入自定义图标和应用 manifest）：

```bat
go build -trimpath -ldflags="-H=windowsgui -s -w" -o build\SwiftSearchPlus-unicon.exe .
```

### 使用提示

1. 启动程序并批准 UAC 请求。
2. 选择要搜索的 NTFS 卷。
3. 输入文件名或路径片段。
4. 可选设置显示类型、包含扩展名和排除扩展名，然后点击“搜索”。
5. 点击结果列标题可排序；右键结果可打开、定位或复制路径。

扩展名示例：`jpg; png; pdf`。输入时可以带或不带前导点。

### 当前限制

- 目前仅面向 Windows x64 和 NTFS 卷。
- 首次构建索引可能需要一段时间；大型卷的结果数量、磁盘速度和文件系统状态可能会影响响应时间。
- 该版本尚未在多种 Windows 版本、不同 DPI 比例和大型真实磁盘上完成全面验证。窗口缩放、搜索取消、权限提升、索引缓存及 USN 增量更新仍需实际测试。
- 请勿把当前实验版视为已经稳定的正式发行版本。

### 项目来源与名称说明

本仓库包含的是独立编写的 Go/Win32 实现，不包含原版 SwiftSearch 的 C++ 源码，也不包含从原版可执行文件提取的文件。原版项目可在 [SourceForge 上的 SwiftSearch 项目页](https://sourceforge.net/projects/swiftsearch/) 查看。此链接仅用于说明相关项目背景；本项目非官方，也未获原作者背书。

本仓库中的代码和自行制作的图标依照根目录 `LICENSE` 中的 MIT License 发布。该许可证不适用于任何未包含在本仓库中的第三方项目或资产。

## English

### Features

- Native Win32 GUI for Windows x64, with Simplified Chinese and English UI options.
- Searches NTFS volumes by reading filesystem metadata; administrator elevation is requested at startup.
- Filename and path matching, including wildcard patterns.
- File-category filters, custom extension inclusion, and extension exclusion.
- Result columns for name, type, size, creation time, last-write time, last-access time, and full path.
- Column sorting, context menu, open, show in Explorer, and copy-path actions.
- Local index cache and NTFS USN Journal incremental-update logic.
- No third-party Go module dependencies.

### Build

Requirements: Windows 10/11 x64, Go 1.23+, and Python 3 with the `py` launcher. Run `build.cmd`; the executable is written to `build/SwiftSearchPlus.exe`.

### Project status

This project is experimental and has not completed comprehensive Windows runtime testing. Search correctness, responsiveness on large volumes, elevation, cache refresh, and DPI behavior should be verified before general use.

### Upstream reference

The original SwiftSearch project is available on [SourceForge](https://sourceforge.net/projects/swiftsearch/). This repository is an independent Go/Win32 implementation, does not include the original C++ source or original icon, and is not affiliated with or endorsed by the original project or author. The code and newly created icon in this repository are licensed under the MIT License in `LICENSE`.
