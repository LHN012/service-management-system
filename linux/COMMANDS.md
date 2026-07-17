# SMS Linux 指令表

## 启动与执行方式

| 指令 | 指令效果 |
| --- | --- |
| `./sms`<br>`./sms shell`<br>`./sms sh` | 进入 SMS 交互式 Shell；SMS 不监听端口，退出 Shell 不会停止业务服务。 |
| `./sms <指令>` | 不进入 Shell，直接执行一条指令，完成后退出。 |
| `./sms --root <目录> [指令]`<br>`./sms -r <目录> [指令]` | 使用指定目录作为项目、运行记录和日志根目录。目录必须已经存在。 |
| `SMS_ROOT=<目录> ./sms [指令]` | 使用环境变量指定的 SMS 根目录。 |
| `./sms --help`<br>`./sms -h` | 显示程序启动参数。 |

## Shell 操作

| 指令 | 指令效果 |
| --- | --- |
| `help`<br>`h` | 显示正式命令和简短命令。 |
| `doctor` | 检查 Linux、`/proc`、外部命令和 SMS 根目录写权限，只报告，不安装依赖。 |
| `version`<br>`v` | 显示 SMS 版本和 Go 运行时版本。 |
| `exit`<br>`q` | 退出 SMS Shell，不停止已经启动的业务进程。 |

## 项目管理

| 指令 | 指令效果 |
| --- | --- |
| `project create <项目>`<br>`p new <项目>` | 创建项目，依次录入服务数量、服务名、启动路径、端口、启动命令和重启策略。 |
| `project list`<br>`p ls` | 列出全部项目、服务数量和服务名。 |
| `project show <项目>`<br>`p info <项目>` | 显示项目配置和全部服务详情。 |
| `project status <项目>`<br>`p status <项目>` | 显示项目中各服务的运行记录、进程身份和端口核对状态。 |
| `project rename <项目> <新名称>`<br>`p mv <项目> <新名称>` | 重命名项目及其项目目录。 |

## 服务配置

| 指令 | 指令效果 |
| --- | --- |
| `service add <项目>`<br>`s add <项目>` | 向已有项目新增一个服务。 |
| `service list <项目>`<br>`s ls <项目>` | 显示项目的服务摘要。 |
| `service show <项目> <服务>`<br>`s info <项目> <服务>` | 显示指定服务的完整配置。 |
| `service status <项目> <服务>`<br>`s status <项目> <服务>` | 显示指定服务的运行状态、PID 和核对详情。 |
| `service edit <项目> <服务>`<br>`s edit <项目> <服务>` | 编辑指定服务的名称、启动路径、端口、启动命令和重启策略。 |
| `service select <项目>`<br>`s sel <项目>` | 按名称、编号、`all` 或 `none` 选择参与项目级启停的服务。堡垒机默认使用行式输入。 |

## 服务启动

| 指令 | 指令效果 |
| --- | --- |
| `project start <项目>`<br>`p up <项目>` | 启动项目中 `managed=true` 的服务；端口已被占用时按服务重启策略处理。 |
| `project start <项目> --all`<br>`p up <项目> -a` | 启动项目中的全部服务，不受 `managed` 状态限制。 |
| `service start <项目> <服务>`<br>`s up <项目> <服务>` | 启动指定服务，不受 `managed` 状态限制。 |

## 服务停止

| 指令 | 指令效果 |
| --- | --- |
| `project stop <项目>`<br>`p down <项目>` | 核对进程指纹后，向项目中 `managed=true` 服务的进程组发送 `SIGTERM`。 |
| `project stop <项目> --all`<br>`p down <项目> -a` | 核对进程指纹后停止项目中的全部服务。 |
| `service stop <项目> <服务>`<br>`s down <项目> <服务>` | 核对进程指纹后停止指定服务；不会按端口终止其他进程。 |
| `project stop <项目> [--all] --force`<br>`p down <项目> [-a] -f` | `SIGTERM` 超时后，向已验证的服务进程组发送 `SIGKILL`。 |
| `service stop <项目> <服务> --force`<br>`s down <项目> <服务> -f` | `SIGTERM` 超时后，向已验证的指定服务进程组发送 `SIGKILL`。 |

## 部署

| 指令 | 指令效果 |
| --- | --- |
| `deploy list <项目>`<br>`d ls <项目>` | 列出项目 `deploy-files/` 中可部署的文件和目录。 |
| `deploy plan <项目> <源> --target <绝对目标>` | 显示源类型、最终目标、目标现状和替换策略，不修改目标。 |
| `deploy apply <项目> <源> --target <绝对目标>`<br>`d apply <项目> <源> -t <绝对目标>` | 显示计划并确认，备份目标，完整构建暂存内容后通过 rename 切换；切换失败时恢复旧目标。 |
| `deploy apply <项目> <源> --target <绝对目标> --yes`<br>`d apply <项目> <源> -t <绝对目标> -y` | 用于已经完成外部审批的脚本，跳过 SMS 内部确认。 |
