package main

import (
	"time"

	"sms/internal/deploy"
	"sms/internal/model"
	"sms/internal/service"
)

type AgentSummary struct {
	Status    string    `json:"status"`
	Mode      string    `json:"mode"`
	StartedAt time.Time `json:"startedAt"`
	LastScan  time.Time `json:"lastScan"`
}

type ProjectSummary struct {
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	ManageMode       string    `json:"manageMode"`
	Status           string    `json:"status"`
	Backends         int       `json:"backends"`
	RunningBackends  int       `json:"runningBackends"`
	Frontends        int       `json:"frontends"`
	RunningFrontends int       `json:"runningFrontends"`
	Ports            []int     `json:"ports"`
	LastScanAt       time.Time `json:"lastScanAt"`
}

type EnvironmentItem struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Required bool   `json:"required"`
}

type ActivityItem struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	Result string    `json:"result"`
	Error  string    `json:"error,omitempty"`
}

type Dashboard struct {
	Agent        AgentSummary      `json:"agent"`
	Projects     []ProjectSummary  `json:"projects"`
	Environment  []EnvironmentItem `json:"environment"`
	Recent       []ActivityItem    `json:"recent"`
	ProcessCount int               `json:"processCount"`
	PortCount    int               `json:"portCount"`
	DataRoot     string            `json:"dataRoot"`
}

type ProcessDTO struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	Command string `json:"command"`
	CWD     string `json:"cwd"`
	User    string `json:"user"`
	Ports   []int  `json:"ports"`
}

type OperationResult struct {
	Results []service.Result `json:"results"`
	Runtime model.Runtime    `json:"runtime"`
}

type DeployPreview struct {
	ID            string          `json:"id"`
	ProjectCode   string          `json:"projectCode"`
	Rule          string          `json:"rule"`
	SourcePath    string          `json:"sourcePath"`
	ContentRoot   string          `json:"contentRoot"`
	Changes       []deploy.Change `json:"changes"`
	DefaultBackup bool            `json:"defaultBackup"`
}

type DeployResult struct {
	Rule        string   `json:"rule"`
	Changes     int      `json:"changes"`
	BackupPaths []string `json:"backupPaths"`
}

type ProjectDetail struct {
	Project model.Project `json:"project"`
	Runtime model.Runtime `json:"runtime"`
}
