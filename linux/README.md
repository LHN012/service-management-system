# Service Management System for Linux

Linux `0.1.0` 提供命令行工具和独立 Agent，用于管理本机项目配置、业务进程、监听端口和部署迁移。

## 构建与初始化

在仓库根目录执行：

```bash
./linux/build.sh
./linux/init
```

构建结果为 `linux/bin/sms-core`。初始化产生的配置、项目、日志、备份和临时目录均位于 `linux/` 下，并已加入根目录 `.gitignore`。

## 运行

```bash
./linux/sms start
./linux/sms status
./linux/sms enter
```

常用交互命令：

```text
p -c                         创建项目
p -l                         查询项目
p -s <项目>                  查看项目和运行状态
p -e <项目>                  编辑 project.yml

st|sp|rst <项目>             启动、停止或重启项目
st|sp|rst <项目>-<组件>      操作单个组件

pr -l                        查看进程与端口
pr -p <端口>                 查询端口
pr -s                        重建运行状态

dp <项目> [规则名]           预览并确认部署迁移
```

`sms stop` 只停止管理 Agent，不会停止业务进程。

完整配置与部署说明见仓库根目录 [README.md](../README.md) 和 [examples/project.yml](../examples/project.yml)。
