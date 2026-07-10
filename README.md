# Service Management System

Service Management System 是一个面向单机环境的项目、进程和部署管理工具。当前版本为 `0.1.0`，提供 Linux 命令行工具和 Windows 桌面应用，适合统一管理 Java、Python、Node、Nginx 等本地业务进程。

项目不接管业务进程的安装方式，也不会在管理端退出时停止业务进程。它通过项目配置识别进程、端口和工作目录，并提供启停、状态扫描、部署预览、备份和覆盖替换能力。

## 平台与能力

| 能力 | Linux | Windows |
| --- | --- | --- |
| 使用方式 | CLI 与交互式 Shell | Wails 桌面界面 |
| 项目配置 | YAML | 图形化编辑与 YAML 存储 |
| 进程与端口扫描 | 支持 | 支持 |
| 项目/组件启停 | 支持 | 支持 |
| 文件、目录、压缩包部署 | 支持 | 支持 |
| 部署预览、备份、失败恢复 | 支持 | 支持 |
| 后台状态扫描 | 独立 Agent | 桌面进程内 Agent |

## 核心特性

- 使用项目配置统一描述后端进程、前端 Nginx 和部署规则。
- 支持按项目、后端组、前端组或单个组件启动、停止和重启。
- 根据命令行、工作目录和预期端口识别已经运行的业务进程。
- 每个项目只有一个 `deploy-files` 投放区，不区分前端和后端文件。
- 支持普通文件、目录、ZIP、TAR 和 TAR.GZ 部署。
- 部署前固定展示内容根目录、覆盖清单和备份范围，确认后才修改目标。
- 部署失败时自动恢复本次已经修改的目标条目。
- Agent 停止或桌面端退出后，已启动的业务进程继续运行。

## 仓库结构

```text
service-management-system/
  cmd/sms/                    Linux CLI 入口
  internal/                   配置、进程、部署和存储等公共逻辑
  windows/                    Windows Wails 应用
    frontend/                 React 前端
  docs/                       Linux 与 Windows 产品文档
  examples/project.yml        项目配置示例
  init                        Linux 初始化脚本
  sms                         Linux 命令入口脚本
  build.sh                    Linux amd64 构建脚本
  build-windows.ps1           Windows amd64 构建脚本
```

`bin/`、`release/`、运行数据、前端依赖和前端构建结果均为本地生成内容，不提交到源码仓库。

## Linux 快速开始

### 环境要求

- Linux amd64
- Go 1.25 或更高版本（从源码构建时需要）
- 系统提供 `/proc`、`sh`、`ps` 和 `ss`
- Java、Python、Node、Nginx、tar、gzip、unzip 按项目实际能力安装

从源码构建并初始化：

```bash
git clone https://github.com/LHN012/service-management-system.git
cd service-management-system
./build.sh
./init
```

初始化后的运行目录如下：

```text
service-management-system/
  conf/                       应用配置
  projects/                   项目配置与项目数据
    <project>/
      project.yml
      deploy-files/           统一部署投放区
      backups/
      logs/
      scripts/
  data/
    runtime/                  Agent 运行状态
    backups/                  全局备份数据
    logs/                     系统日志和初始化报告
  templates/
  tmp/deploy/                 部署解压与回滚临时目录
```

启动 Agent 并进入交互式 Shell：

```bash
./sms start
./sms status
./sms enter
```

`./sms stop` 只停止管理 Agent，不会停止已经启动的业务进程。重新启动 Agent 后会根据项目配置和系统进程重新构建运行状态。

### Linux 常用命令

```text
p -c                         创建项目
p -l                         查询项目和状态
p -s <项目>                  查看项目配置与运行状态
p -e <项目>                  使用 $EDITOR 编辑 project.yml
p -d <项目>                  删除项目管理数据

st <项目>                    启动项目
sp <项目>                    停止项目
rst <项目>                   重启项目
st|sp|rst <项目>-backend      操作项目全部后端
st|sp|rst <项目>-front        操作项目全部前端
st|sp|rst <项目>-<组件>       操作单个组件

pr -l                        查看进程与端口
pr -p <端口>                 查询端口占用
pr -s                        立即重建全部项目状态

dp <项目> [规则名]            预览并确认部署迁移
```

命令既可以作为 `./sms <命令>` 直接执行，也可以在 `./sms enter` 的交互环境中执行。

## Windows 快速开始

### 环境要求

- Windows 10 或 Windows 11 amd64
- Microsoft Edge WebView2 Runtime（Windows 10/11 通常已预装）
- Go 1.25+ 与 Node.js 18+（从源码构建时需要）

构建并运行：

```powershell
git clone https://github.com/LHN012/service-management-system.git
Set-Location service-management-system
.\build-windows.ps1
.\bin\service-management-system-windows.exe
```

首次启动会在以下位置创建运行数据：

```text
C:\ProgramData\ServiceManagementSystem\
  conf\
  data\
    projects\
      <project>\
        project.yml
        deploy-files\
        backups\
        logs\
        scripts\
    runtime\
    backups\
    logs\
  templates\
  tmp\deploy\
```

开发或测试时，可以通过 `SMS_WINDOWS_ROOT` 指定独立的数据根目录。`ProgramData` 不可写时，请以管理员身份运行。

Windows 端提供总览、项目管理、进程与端口、部署迁移、环境检测、日志和系统设置页面。项目级停止和重启操作需要二次确认。

更多 Windows 说明见 [windows/README.md](windows/README.md)。

## 部署迁移

把一个项目需要部署的 JAR、配置文件、目录和压缩包全部放入该项目的 `deploy-files/`。每条 `deployRules` 规则只负责描述其中一个源条目如何迁移到目标目录。

示例：

```yaml
deployRules:
  - name: api-jar
    source: deploy-files/demo-api.jar
    targetDir: /opt/demo/backend
    targetName: demo-api.jar
    type: file
    backup: true

  - name: web-package
    source: deploy-files/web-dist.zip
    targetDir: /usr/share/nginx/html/demo
    type: archive
    archiveFormat: auto
    stripTopLevel: auto
    replaceMode: entries
    backup: true
```

部署类型的处理规则：

- `file`：复制到 `targetDir`，可通过 `targetName` 指定目标文件名。
- `directory`：把源目录内的顶层条目迁移到 `targetDir`。
- `archive`：先解压到临时目录，再把实际内容根目录下的顶层条目迁移到 `targetDir`。

压缩包可能带一个外层文件夹，也可能直接包含多个文件或文件夹。默认的 `stripTopLevel: auto` 会自动处理这两种情况：

- 只有一个外层目录时，使用该目录里面的内容。
- 直接包含文件或存在多个顶层条目时，使用解压根目录的内容。

迁移时，同名文件直接替换，同名目录整体替换；目标目录中未出现在部署包顶层的其他条目会保留。系统会拒绝路径穿越、符号链接、设备文件和没有可部署内容的压缩包。

完整配置见 [examples/project.yml](examples/project.yml)。

## 开发与测试

运行 Go 测试和静态检查：

```bash
go test ./...
go vet ./...
```

运行 Windows 前端检查和构建：

```powershell
Set-Location windows\frontend
npm ci
npm run lint
npm run build
```

构建 Linux amd64 可执行文件：

```bash
./build.sh
```

构建 Windows amd64 桌面应用：

```powershell
.\build-windows.ps1
```

生成结果位于 `bin/`。

## 当前限制

- Windows Agent 当前运行在桌面应用进程内，尚未安装为独立 Windows Service。
- Windows 当前没有系统托盘；关闭窗口会退出管理界面和 Agent，但不会停止业务进程。
- Windows 无法通过标准 CIM 稳定读取任意进程的真实工作目录，建议项目至少配置 `commandContains` 或 `expectedPorts`。
- Windows 构建产物尚未进行代码签名。
- 当前版本只管理本机，不开放远程管理接口。

## 相关文档

- [Linux 产品文档](docs/linux-service-manager-prd.md)
- [Windows 产品文档](docs/windows-service-manager-prd.md)
- [Windows 使用说明](windows/README.md)
- [项目配置示例](examples/project.yml)
