# Service Management System

Service Management System 包含两个相互独立的实现。Linux 与 Windows 各自拥有源码、依赖、文档、构建产物和运行数据，不共享 Go 模块或项目配置。

## 工作目录

| 目录 | 实现 | 配置格式 | 入口文档 |
| --- | --- | --- | --- |
| `linux/` | Go 交互式 CLI | JSON | [Linux 操作手册](linux/README.md) |
| `windows/` | Wails + React 桌面应用 | YAML | [Windows 使用说明](windows/README.md) |

```text
service-management-system/
  linux/
    docs/
    go.mod
    *.go
  windows/
    docs/
    examples/
    frontend/
    internal/
    go.mod
    go.sum
  README.md
```

## Linux

Linux 版需要 Go 1.25+，运行时依赖 `sh`、`nohup`、`kill` 和 `lsof`。

```bash
cd linux
./build.sh
./sms
```

Linux 版采用便携运行方式，不执行系统安装，不修改 `PATH`，也不启动后台守护进程。默认在可执行文件所在目录保存项目配置和日志。

完整指令见 [指令表](linux/COMMANDS.md)，模拟问题记录见 [运行模拟问题](linux/docs/simulation-issues.md)。

## Windows

Windows 版需要 Go 1.25+、Node.js 18+ 和 Microsoft Edge WebView2 Runtime。

```powershell
Set-Location windows
.\build.ps1
.\bin\service-management-system-windows.exe
```

构建脚本会在 `windows/` 内完成前端安装、检查、构建以及 Go 测试和编译。产品设计见 [Windows 产品文档](windows/docs/service-manager-prd.md)，配置示例见 [project.yml](windows/examples/project.yml)。

## 数据边界

- `linux/projects/` 与 `linux/logs/` 只属于 Linux 版。
- Windows 版默认使用 `C:\ProgramData\ServiceManagementSystem\`，开发时可通过 `SMS_WINDOWS_ROOT` 指定数据目录。
- 两个工作目录中的 `bin/`、`release/`、前端依赖和前端构建结果均为本地生成内容，不提交到 Git。
