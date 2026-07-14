# Service Management System for Windows

Windows `0.1.0` 使用 Wails + React 构建桌面管理界面，Go 后端负责本机项目配置、进程启停、端口扫描、部署迁移和日志审计。

## 运行

双击 `service-management-system-windows.exe`。首次启动会创建：

```text
C:\ProgramData\ServiceManagementSystem\
  conf\
  data\
    projects\
    runtime\
    backups\
    logs\
  templates\
  tmp\deploy\
```

没有管理员权限或 `ProgramData` 不可写时，应使用管理员身份运行。开发和测试时可通过 `SMS_WINDOWS_ROOT` 指定独立数据目录。

## 已实现

- 总览、项目管理、项目编辑和运行状态。
- Java、Python、Node、Nginx 等进程与监听端口查看。
- 项目、后端和前端启动、停止、重启。
- 项目级停止与重启的两次确认。
- 统一 `deploy-files` 投放区、文件/目录/压缩包部署预览、备份、覆盖和失败恢复。
- 环境检测、系统设置、系统日志、项目日志和操作审计。
- 本地 Desktop Agent 定时重建状态，不开放远程管理端口。

## 当前边界

- 当前 Agent 运行在桌面应用进程内，尚未安装为独立 Windows Service。
- 当前版本未提供系统托盘；关闭窗口会退出管理界面和 Agent，但不会停止业务进程。
- Windows 无法通过标准 CIM 稳定读取任意进程的真实工作目录；首版使用命令行、监听端口和可执行文件目录完成匹配。建议项目至少配置 `commandContains` 或 `expectedPorts`。
- 运行需要 Microsoft Edge WebView2 Runtime。Windows 10/11 通常已预装。

## 从源码构建

需要 Go 1.25+、Node.js 18+ 和 WebView2 Runtime：

```powershell
.\windows\build.ps1
.\windows\bin\service-management-system-windows.exe
```

开发界面：

```powershell
cd windows\frontend
npm install
npm run dev
```
