package main

import (
	// errors 用来汇总批量停止过程中各服务的错误。
	"errors"
	// fmt 用来输出停止结果。
	"fmt"
	// io 让停止流程可以写入终端，也方便后面测试。
	"io"
	// time 用来等待端口释放。
	"time"
)

// isStopCommand 判断当前输入是不是关闭服务命令。
// 支持：
//
//	sp <project>
//	stop <project>
//	sp -all <project>
//	sp -i <project> <service>
func isStopCommand(fields []string) bool {
	if len(fields) != 2 && len(fields) != 3 && len(fields) != 4 {
		return false
	}
	if fields[0] != "sp" && fields[0] != "stop" {
		return false
	}
	if len(fields) == 2 {
		return true
	}
	if len(fields) == 3 {
		return fields[1] == "-all"
	}
	return fields[1] == "-i"
}

// stopServices 按命令参数选择服务并关闭。
func stopServices(output io.Writer, root string, fields []string, force bool) error {
	projectName, services, err := selectServicesForLifecycle(root, fields, "sp")
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return fmt.Errorf("no services selected")
	}

	runtime, err := loadProjectRuntime(root, projectName)
	if err != nil {
		return err
	}

	var stopErrors []error
	for _, service := range services {
		if err := stopOneService(output, root, projectName, service, runtime, force); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("stop %s: %w", service.Name, err))
			continue
		}
	}

	if err := saveProjectRuntime(root, projectName, runtime); err != nil {
		stopErrors = append(stopErrors, fmt.Errorf("save runtime: %w", err))
	}
	return errors.Join(stopErrors...)
}

// stopOneService only signals the process recorded and verified by SMS.
func stopOneService(output io.Writer, root string, projectName string, service Service, runtime ProjectRuntime, force bool) error {
	if err := validatePort(service.Port); err != nil {
		return fmt.Errorf("service %s port invalid: %w", service.Name, err)
	}

	pids, err := portListeningPIDs(service.Port)
	if err != nil {
		return err
	}
	recorded, ok := runtime.Services[service.Name]
	if !ok {
		if len(pids) > 0 {
			return fmt.Errorf("port %d is occupied by unmanaged pid(s) %s; SMS will not stop them", service.Port, joinInts(pids))
		}
		fmt.Fprintf(output, "%s/%s not running, port=%d\n", projectName, service.Name, service.Port)
		return nil
	}
	if !serviceRuntimeAlive(recorded) {
		delete(runtime.Services, service.Name)
		if len(pids) > 0 {
			return fmt.Errorf("recorded pid %d is no longer running; port %d is occupied by unmanaged pid(s) %s", recorded.PID, service.Port, joinInts(pids))
		}
		fmt.Fprintf(output, "%s/%s not running; removed stale runtime pid=%d\n", projectName, service.Name, recorded.PID)
		return nil
	}
	if _, err := verifyProcessIdentity(recorded.PID, recorded.ProcessIdentity); err != nil {
		delete(runtime.Services, service.Name)
		return err
	}

	fmt.Fprintf(output, "stopping verified process %s/%s, pid=%d, pgid=%d, port=%d\n", projectName, service.Name, recorded.PID, recorded.ProcessGroupID, service.Port)
	if len(pids) > 0 && !containsPID(pids, recorded.PID) {
		fmt.Fprintf(output, "warning: configured port %d is owned by other pid(s) %s; they will not be signaled\n", service.Port, joinInts(pids))
	}

	if err := terminateRuntimeProcess(recorded, force, 5*time.Second); err != nil {
		if errors.Is(err, errTerminationTimeout) {
			return fmt.Errorf("%w; rerun with --force to send SIGKILL", err)
		}
		return err
	}

	delete(runtime.Services, service.Name)
	remaining, err := portListeningPIDs(service.Port)
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("verified process stopped, but port %d is still occupied by pid(s) %s", service.Port, joinInts(remaining))
	}
	fmt.Fprintf(output, "stopped %s/%s, port=%d\n", projectName, service.Name, service.Port)
	return nil
}

// waitPortFree 等待端口不再被监听。
func waitPortFree(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pids, err := portListeningPIDs(port)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("port %d is still occupied", port)
}
