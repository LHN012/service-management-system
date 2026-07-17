# SMS Linux 操作手册

本文档描述 SMS Linux 当前工作树已经实现的行为，用于理解和试用现有版本。它不是未来规划，也不包含旧版 Agent、后台扫描和复杂项目模型。

## 1. 当前定位

SMS Linux 是一个单机、交互式的 Linux 服务管理工具。当前围绕三个对象工作：

```text
项目 Project
  -> 服务 Service
     -> 启动路径、端口、启动命令、重启策略
  -> deploy-files
     -> 待部署文件、目录或压缩包
```

当前支持：

- 创建、查看和重命名项目。
- 新增、编辑和绑定项目服务。
- 按项目、全部服务或单个服务启动、停止和查看状态。
- 记录并验证 Linux boot ID、进程启动时间和进程组，防止 PID 重用误杀。
- 预览并部署普通文件、目录、ZIP、TAR、TAR.GZ 和 TGZ。
- 启动日志、原子 JSON 运行记录、部署前备份和失败恢复。
- `doctor` 环境检查和 `version` 版本查询。

当前不支持：

- 安装到系统目录、自动修改 `PATH` 或注册系统服务。
- SMS 后台守护进程和网络监听端口。
- Agent、后台状态扫描和远程管理。
- 项目或服务删除命令。
- 独立的 `restart` 命令。
- 健康检查、自动拉起和开机启动。
- 部署后的业务健康检查和基于健康检查的自动回滚。

## 2. 运行要求

- Linux。
- 从源码构建时需要 Go 1.25 或更高版本。
- 系统命令：`sh`、`nohup`、`kill`、`lsof`。
- 按服务类型准备 `java`、`python3` 或 `nginx`。
- 当前用户需要具备读取进程、监听端口、停止服务和写入部署目标的权限。

`lsof` 是当前启停流程的必需依赖。缺少它时，项目仍可配置和查看，但不能可靠执行启动、停止。

## 3. 构建与启动

进入 Linux 工作目录构建，然后直接运行：

```bash
cd linux
./build.sh
./sms
```

程序启动后进入交互式 Shell：

```text
SMS Linux
Root: /home/user/linux
Type help for commands, exit to quit.
sms>
```

输入 `help` 查看命令，输入 `exit` 或 `quit` 退出管理界面。SMS 只在当前终端等待输入，不监听网络端口；退出管理界面不会停止已经启动的业务进程。

### 根目录规则

项目配置、运行记录、服务日志和 SMS 操作日志都相对于 SMS 根目录保存。根目录按以下优先级确定：

1. 命令行 `--root <目录>` 或 `-r <目录>`。
2. 环境变量 `SMS_ROOT`。
3. 可执行文件所在目录。

正式发布包把可执行文件放在 Linux 工作目录内，因此默认根目录就是解压后的目录。

指定的根目录必须已经存在。查看启动参数：

```bash
./sms --help
```

开发环境建议始终显式指定：

```bash
./sms --root . shell
# 或
SMS_ROOT=. ./sms shell
```

服务器推荐直接解压并运行：

```text
linux/
  sms
  projects/
  logs/
```

在解压目录中执行：

```bash
cd linux
./sms
```

SMS 不执行安装、不复制自身、不创建命令链接，也不修改当前用户或系统环境。需要从其他目录调用时，直接使用完整路径，或由用户自行配置 Shell alias。

## 4. 数据目录

创建项目后会形成以下结构：

```text
linux/
  sms
  projects/
    <项目名>/
      project.json       项目和服务配置
      runtime.json       PID、进程指纹和进程组运行记录
      deploy-files/      待部署文件投放区
      backups/           部署前备份
      logs/              服务标准输出和错误日志
  logs/
    sms-YYYY-MM.log      SMS 当前月份操作日志
    archive/             已完成月份的日志归档
```

`projects/`、`logs/`、`bin/` 和本地测试临时目录已被 Git 忽略。

## 5. SMS 操作日志

SMS 自身的启动、退出和普通命令都会记录到下列文件。直接执行的 `version` 和 `doctor` 为了在日志目录不可写时仍能诊断，不初始化操作日志；交互 Shell 中执行它们时仍会记录。

```text
logs/sms-YYYY-MM.log
```

日志使用一行一条 JSON 的格式，包含时间、动作、原命令、执行结果和错误详情，例如：

```json
{"timestamp":"2026-07-15T20:30:00+08:00","action":"project.list","command":"ls","result":"success"}
```

常见动作包括 `sms.start`、`sms.stop`、`project.create`、`project.list`、`service.start`、`service.stop` 和 `deploy`。可以直接查询：

```bash
tail -n 100 ./logs/sms-$(date +%Y-%m).log
tail -f ./logs/sms-$(date +%Y-%m).log
```

每次写日志时，SMS 都会检查是否存在已结束月份的明文日志。进入新月份后的第一次启动或命令操作，会把上月日志归档为：

```text
logs/archive/sms-YYYY-MM.tar.gz
```

只有归档成功后才会删除上月 `.log` 文件。正常情况下每个月生成一个归档；如果同月归档文件已经存在但又发现额外日志，为避免覆盖数据，会生成 `-part-2`、`-part-3` 等补充分包。

## 6. 建议理解顺序

首次试用建议按以下顺序操作：

```text
p new demo
p info demo
s sel demo
p up demo
p down demo
d ls demo
```

先使用测试端口、测试进程和测试部署目录，不要直接指向生产服务。

## 7. 命令总览

| 目的 | 正式命令 | 简短命令 |
| --- | --- | --- |
| 查看帮助 | `help` | `h` |
| 创建项目 | `project create <项目>` | `p new <项目>` |
| 查看项目列表 | `project list` | `p ls` |
| 查看项目详情 | `project show <项目>` | `p info <项目>` |
| 查看项目状态 | `project status <项目>` | `p status <项目>` |
| 重命名项目 | `project rename <项目> <新名称>` | `p mv <项目> <新名称>` |
| 新增服务 | `service add <项目>` | `s add <项目>` |
| 查看服务列表 | `service list <项目>` | `s ls <项目>` |
| 查看服务详情 | `service show <项目> <服务>` | `s info <项目> <服务>` |
| 查看服务状态 | `service status <项目> <服务>` | `s status <项目> <服务>` |
| 编辑服务 | `service edit <项目> <服务>` | `s edit <项目> <服务>` |
| 选择托管服务 | `service select <项目>` | `s sel <项目>` |
| 启动托管服务 | `project start <项目>` | `p up <项目>` |
| 启动全部服务 | `project start <项目> --all` | `p up <项目> -a` |
| 启动单个服务 | `service start <项目> <服务>` | `s up <项目> <服务>` |
| 停止托管服务 | `project stop <项目>` | `p down <项目>` |
| 停止全部服务 | `project stop <项目> --all` | `p down <项目> -a` |
| 停止单个服务 | `service stop <项目> <服务>` | `s down <项目> <服务>` |
| 查看部署投放区 | `deploy list <项目>` | `d ls <项目>` |
| 预览部署 | `deploy plan <项目> <源> --target <绝对目标>` | - |
| 执行部署 | `deploy apply <项目> <源> --target <绝对目标>` | `d apply <项目> <源> -t <绝对目标>` |
| 检查环境 | `doctor` | - |
| 查看版本 | `version` | `v` |
| 退出 | `exit` | `q` |

正式命令和简短命令执行完全相同的内部动作。旧版 `create project`、`ls -i`、`st`、`sp`、`dp` 等指令暂时保留兼容，但不再作为主命令展示。

交互 Shell 使用空格拆分参数，不支持通过引号保留路径中的空格。直接执行模式使用操作系统已经解析好的参数，可以正确接收包含空格的单个参数。

命令既可以在 Shell 中执行，也可以由服务器脚本直接执行：

```bash
./sms project list
./sms p info demo
```

## 8. 创建项目

执行：

```text
sms> project create demo
project created: /path/to/projects/demo/project.json
service count (0 to skip): 2
```

空项目会先保存，随后服务数量可以输入 `0`。每完成一个服务就立即原子保存，因此堡垒机连接中断时已经保存的服务不会丢失。保存前会校验名称、重复服务、端口、启动路径和命令；相对启动路径在录入时会解析为绝对路径，路径不存在时拒绝保存。

随后为每个服务填写：

| 提示 | 含义 |
| --- | --- |
| `name` | 项目内唯一的服务名，不能包含空格 |
| `start path` | JAR、Python 脚本、Nginx 配置或自定义程序路径 |
| `port` | 服务必须监听的 TCP 端口，范围 `1-65535` |
| `custom start command` | 直接执行的启动命令；留空时自动推断 |
| `restart mode` | 端口已被占用时选择 `kill-start` 或 `command` |
| `restart command` | `command` 模式下执行的命令 |

自动启动命令规则：

| 启动路径 | 自动命令 |
| --- | --- |
| `*.jar` | `java -jar "<路径>"` |
| `*.py` | `python3 "<路径>"` |
| `*.conf` 或路径中包含 `nginx` | `nginx -c "<路径>" -g 'daemon off;'` |

其他类型必须填写自定义启动命令。

### 重启策略

`kill-start`：端口已被占用时，只允许停止 `runtime.json` 中属于当前服务且进程指纹仍匹配的进程。无法验证身份时拒绝发送信号。JAR 和 Python 服务默认使用此策略。

`command`：当端口已被占用时，询问是否执行 `restartCommand`，执行后不再创建新进程。Nginx 默认使用配置检查加 reload 命令。

重启策略只在执行 `st` 且端口已被占用时生效。端口空闲时始终执行启动命令。

## 9. 查看和编辑项目

查看所有项目：

```text
sms> project list
```

查看完整配置：

```text
sms> project show demo
```

查看精简的服务列表：

```text
sms> service list demo
```

重命名项目：

```text
sms> project rename demo demo2
```

编辑服务：

```text
sms> service edit demo api
```

编辑启动命令时：

- 直接回车保留当前命令。
- 输入 `auto` 根据新的启动路径重新推断。
- 输入其他内容保存为自定义命令。

新增服务：

```text
sms> service add demo
```

查看项目或单个服务的实际状态：

```text
sms> project status demo
sms> service status demo api
```

状态会区分 `running`、`stopped`、`stale`、`unverified`、`unmanaged` 和 `degraded`。端口被其他进程占用时只报告，不会自动终止该进程。

## 10. 绑定托管服务

`Managed` 表示执行项目级 `project start <项目>` 和 `project stop <项目>` 时是否包含该服务。新建服务默认没有选择。

执行：

```text
sms> service select demo
```

默认使用适合堡垒机和普通 SSH 终端的行式选择。可以输入 `all`、`none`、服务名、编号或逗号分隔的组合，例如：

```text
api,web
1,3
all
none
```

只有显式设置 `SMS_KEYBOARD_UI=1` 且输入输出都是真实终端时，才启用方向键和空格多选。

`-all` 和 `-i` 启停命令不受 `Managed` 限制。

## 11. 启动服务

启动已绑定服务：

```text
sms> project start demo
```

启动项目全部服务：

```text
sms> project start demo --all
```

启动单个服务：

```text
sms> service start demo api
```

启动流程：

1. 使用 `lsof` 检查配置端口。
2. 端口被占用时按重启策略询问处理方式。
3. 使用 `nohup` 在后台执行启动命令。
4. 在独立会话和进程组中运行服务，把标准输出和错误追加到 `projects/<项目>/logs/<服务>.log`。
5. 记录 boot ID、进程启动时间、进程组、可执行文件、工作目录和命令行。
6. 最多等待 15 秒，确认启动 PID 正在监听配置端口。
7. 成功后立即原子写入 `runtime.json`；写入失败时清理刚启动的进程。

端口检查超时后，SMS 会先核对刚记录的进程指纹，再清理这个进程组。如果启动命令会自行 fork 或 daemonize，最终监听端口的 PID 可能与 SMS 启动得到的 PID 不同，当前版本仍会判断为启动失败。Nginx 的自动命令使用 `daemon off` 是为了避免这个问题。

## 12. 停止服务

停止已绑定服务：

```text
sms> project stop demo
```

停止全部或单个服务：

```text
sms> project stop demo --all
sms> service stop demo api
```

停止逻辑以 `runtime.json` 为准。SMS 会核对 Linux boot ID、PID 启动时间和进程组，只向身份匹配的进程组发送 `SIGTERM`。配置端口仅用于状态核对，端口上的其他 PID 不会被发送信号。

进程 5 秒后仍未退出时，普通停止会失败并保留运行记录。确认确实需要强制终止后，显式执行：

```text
sms> service stop demo api --force
sms> project stop demo --all --force
```

`--force` 仍然只会向已经通过指纹验证的进程组发送 `SIGKILL`。旧版本生成、不包含进程指纹的 `runtime.json` 不会被信任；需要人工核对旧进程，停止后再由新版本重新启动。

## 13. 部署文件

先把待部署内容放入：

```text
projects/<项目>/deploy-files/
```

查看投放区：

```text
sms> deploy list demo
```

先预览源、目标、目标现状和替换策略：

```text
sms> deploy plan demo demo-api.jar --target /opt/demo/backend/demo-api.jar
```

部署普通文件：

```text
sms> deploy apply demo demo-api.jar --target /opt/demo/backend/demo-api.jar
```

`deploy apply` 默认再次显示计划并等待确认。服务器脚本已经完成外部审批时，可以显式添加 `--yes` 或 `-y` 跳过交互确认。

如果目标是已存在的目录，文件会使用原文件名放入该目录：

```text
sms> deploy apply demo demo-api.jar --target /opt/demo/backend
```

部署目录或压缩包：

```text
sms> deploy apply demo web-dist --target /var/www/demo
sms> deploy apply demo web-dist.zip --target /var/www/demo
```

部署规则：

| 源类型 | 当前行为 |
| --- | --- |
| 文件 | 替换目标文件，或复制到目标目录下 |
| 目录 | 先完整复制到目标同文件系统的暂存目录，再切换目标 |
| 压缩包 | 先解压并复制到暂存目录，再切换目标 |

ZIP、TAR、TAR.GZ 和 TGZ 受到支持。压缩包只有一个顶层目录时，会自动去掉这一层。压缩包路径穿越、链接条目以及部署源目录中的符号链接会被拒绝。

源路径必须相对于项目的 `deploy-files/`，目标必须是绝对路径，且不能是文件系统根目录。目标原来存在时，会先备份到：

```text
projects/<项目>/backups/<时间戳-随机后缀>/<目标绝对路径>
```

复制或解压失败时目标保持不变。最终 rename 切换失败时，SMS 会立即恢复旧目标。这个保证只覆盖文件系统替换，不包含业务健康检查；新文件内容本身不可用时仍需要人工回滚到备份。

## 14. 配置文件示例

`project.json` 当前结构如下：

```json
{
  "schemaVersion": 1,
  "name": "demo",
  "services": [
    {
      "name": "api",
      "startPath": "/opt/demo/backend/demo-api.jar",
      "port": 8080,
      "startCommand": "java -jar \"/opt/demo/backend/demo-api.jar\"",
      "commandSource": "auto",
      "restartMode": "kill-start",
      "managed": true
    }
  ]
}
```

建议通过交互命令修改配置。手工修改后至少检查：服务名唯一、端口有效、启动路径正确、命令可在对应目录运行。

## 15. 常见问题

### `lsof is unavailable`

安装 `lsof` 并确认当前用户的 `PATH` 可以找到它。

### `port must be between 1 and 65535`

当前服务缺少有效端口。早期生成的 `project.json` 可能没有 `port` 字段，需要通过 `ed -svc` 补充或手工修正。

### `process exited immediately`

启动命令很快退出。查看：

```bash
tail -n 100 projects/<项目>/logs/<服务>.log
```

### `expected port ... was not detected`

进程仍未监听配置端口，或者实际监听者是启动命令派生出的另一个 PID。检查日志、配置端口和 `lsof` 输出。

### `runtime record has no process identity`

这是旧版本留下的运行记录。SMS 为避免 PID 重用误杀，不会使用这条记录停止进程。先人工核对并处理旧进程，再由当前版本重新启动服务。

### 部署提示权限错误

确认当前用户对目标目录及其父目录具有写权限。不要在不理解目标范围时直接使用 `sudo` 运行整个 SMS。

## 16. 当前安全边界

当前已经实现进程指纹验证、显式强制停止、暂存部署、切换失败恢复、原子 JSON 写入、单实例状态修改锁和高风险路径测试。仍有以下边界：

1. 当前只包含从无版本 JSON 到 schema v1 的迁移，后续结构变化仍需逐版补充迁移器。
2. 操作日志尚未包含执行者、耗时和敏感参数脱敏。
3. 部署切换后不执行应用健康检查。
4. 自行 daemonize 并脱离原进程组的服务不受支持。
5. 当前环境已完成 Linux 交叉编译，但发布前仍应在目标发行版执行真实启停集成测试。
6. 解压操作没有为压缩比或展开体积设置硬上限，`deploy-files` 只应接收经过运维审核的发布物。
