# SMS Linux 运行模拟问题记录

## 1. 当前定位

SMS Linux 是一个随解压目录运行的前台运维命令工具，不是服务器守护进程，也不执行系统安装。

正式使用方式：

```bash
cd linux
./sms
```

直接命令方式：

```bash
./sms project list
./sms service start <项目> <服务>
```

固定边界：

- 不向系统目录复制文件。
- 不提供安装或卸载流程。
- 不自动修改 `PATH`、Shell 配置或系统服务。
- 不常驻后台，不监听网络端口。
- 输入 `exit` 只退出 SMS，不停止已经启动的业务服务。
- 配置、运行记录和日志默认保存在 `sms` 所在目录。

## 2. 已确定的运行结构

```text
linux/
  sms
  projects/
  logs/
    sms-YYYY-MM.log
    archive/
```

源码构建方式：

```bash
cd linux
./build.sh
./sms
```

发布包应直接包含已经编译并具有执行权限的 `sms`，用户解压后即可运行。

## 3. 已处理事项

- 无参数启动进入 `sms>` 交互模式。
- 支持直接执行单条命令后退出。
- 启动界面显示 SMS 实际根目录。
- 支持正式命令和日常简短命令。
- SMS 操作日志按月写入，跨月后归档。
- 运行方式统一为解压目录内直接执行 `./sms`。
- 停止服务前验证 boot ID、PID 启动时间和进程组，端口只用于核对。
- `SIGKILL` 只允许通过 `--force` 作用于已验证进程组。
- 部署支持 `plan`、确认、同文件系统暂存切换和切换失败恢复。
- 项目配置和运行记录使用原子写入，状态修改命令使用文件锁。
- 已实现 `version`、`doctor` 和项目/服务状态查询。
- 启动失败会验证进程身份后清理刚创建的进程组。

## 4. 当前待处理问题

### 4.1 进程和端口查询仍需扩展

目标命令：

```text
process list
process show <PID>
process stop <PID>
process kill <PID>
port list
```

Java、Python 等类型只是查询筛选条件，不能成为自动终止依据。

### 4.2 操作日志字段需要补全

当前已有时间、操作类型、原始命令、结果和错误详情。仍需增加：

- 执行者。
- 日志级别。
- 执行耗时。
- 结构化失败原因。
- 密码、令牌等敏感参数脱敏。

### 4.3 服务创建引导需要简化

创建项目后引导输入服务数量，输入 `0` 明确跳过。每个服务只收集：

```text
服务名称
服务类型
服务所在路径
占用端口号
```

标准类型使用本地全局命令模板，只有 `Other` 类型需要用户填写启动命令。Java JAR 或 Python 入口存在多个候选文件时，再引导用户选择。

## 5. 交互模拟记录

### 5.1 启动 SMS

输入：

```bash
./sms
```

预期回传：

```text
SMS Linux
Root: /home/user/linux
Type help for commands, exit to quit.
sms>
```

SMS 此时只等待终端输入，不监听端口。

### 5.2 帮助命令

`help` 和 `h` 均显示当前已经实现的命令。未实现命令不应提前出现在正式帮助中。

后续建议支持 `help project` 等分层帮助以及 `?` 别名。

### 5.3 创建项目

输入：

```text
project create zzfwpt
```

目标交互：

```text
项目创建成功：zzfwpt
请输入要添加的服务数量（输入 0 跳过）：2
```

项目应先保存，再逐个保存服务。添加中途失败或取消时，已经成功保存的内容不能丢失。

### 5.4 添加服务

目标交互：

```text
服务名称：backed
请选择服务类型：
  1. Java
  2. Python
  3. Node
  4. Static
  5. Other
服务所在路径：./backed
占用端口号：8080
```

执行模型：

```text
进入服务目录 -> 读取类型命令模板 -> 解析入口文件 -> 显示最终命令 -> 执行 -> 校验端口
```

服务路径必须显示最终解析结果；不存在、无权访问或端口无效时拒绝保存。

### 5.5 操作日志

成功示例：

```json
{"timestamp":"2026-07-17T14:32:08+08:00","level":"INFO","actor":"user","command":"project create zzfwpt","action":"project.create","result":"success","durationMs":42}
```

失败示例：

```json
{"timestamp":"2026-07-17T14:33:10+08:00","level":"WARN","actor":"user","command":"project create zzfwpt","action":"project.create","result":"failed","reason":"project already exists","durationMs":3}
```

### 5.6 退出 SMS

输入：

```text
exit
```

预期回传：

```text
bye
```

SMS 进程退出，已经启动的业务服务继续运行。
