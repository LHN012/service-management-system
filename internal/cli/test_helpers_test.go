package cli

import "sms/internal/model"

func minimalProject(code, name string) model.Project {
	return model.Project{Code: code, Name: name, ManageMode: "external"}
}
