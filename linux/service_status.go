package main

import (
	"fmt"
	"io"
	"strings"
)

type serviceStatus struct {
	State  string
	PID    int
	Detail string
}

func printServiceStatus(output io.Writer, root string, projectName string, serviceName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}
	project, _, err := loadProject(root, projectName)
	if err != nil {
		return err
	}
	runtimeState, err := loadProjectRuntime(root, projectName)
	if err != nil {
		return err
	}

	services := project.Services
	if serviceName != "" {
		if err := validateServiceName(serviceName); err != nil {
			return err
		}
		index := findServiceIndex(project, serviceName)
		if index < 0 {
			return fmt.Errorf("service not found: %s", serviceName)
		}
		services = []Service{project.Services[index]}
	}

	fmt.Fprintf(output, "Status of %s:\n", project.Name)
	fmt.Fprintf(output, "%-18s %-11s %-8s %-8s %s\n", "service", "state", "pid", "port", "detail")
	for _, service := range services {
		status := inspectServiceStatus(service, runtimeState)
		pid := "-"
		if status.PID > 0 {
			pid = fmt.Sprintf("%d", status.PID)
		}
		fmt.Fprintf(output, "%-18s %-11s %-8s %-8d %s\n", service.Name, status.State, pid, service.Port, status.Detail)
	}
	return nil
}

func inspectServiceStatus(service Service, runtimeState ProjectRuntime) serviceStatus {
	pids, portErr := portListeningPIDs(service.Port)
	recorded, exists := runtimeState.Services[service.Name]
	if !exists {
		if portErr != nil {
			return serviceStatus{State: "unknown", Detail: portErr.Error()}
		}
		if len(pids) > 0 {
			return serviceStatus{State: "unmanaged", Detail: "port pid(s)=" + joinInts(pids)}
		}
		return serviceStatus{State: "stopped"}
	}

	status := serviceStatus{PID: recorded.PID}
	if !serviceRuntimeAlive(recorded) {
		status.State = "stale"
		status.Detail = "recorded process is not running"
		return status
	}
	if _, err := verifyProcessIdentity(recorded.PID, recorded.ProcessIdentity); err != nil {
		status.State = "unverified"
		status.Detail = err.Error()
		return status
	}
	if portErr != nil {
		status.State = "running"
		status.Detail = "identity verified; port check unavailable: " + portErr.Error()
		return status
	}
	if containsPID(pids, recorded.PID) {
		status.State = "running"
		status.Detail = "identity and port verified"
		return status
	}
	status.State = "degraded"
	status.Detail = "identity verified; configured port is not owned by recorded pid"
	if len(pids) > 0 {
		status.Detail += "; other pid(s)=" + joinInts(pids)
	}
	status.Detail = strings.TrimSpace(status.Detail)
	return status
}
