package analyzer

import (
	"github.com/devos-os/d-env/internal/modules/docker"
	"github.com/devos-os/d-env/internal/modules/general"
	"github.com/devos-os/d-env/internal/modules/git"
	"github.com/devos-os/d-env/internal/modules/infra"
)

type Report struct {
	General general.GeneralData
	Git     git.GitData
	Docker  docker.DockerData
	Infra   infra.InfraData
}

func Analyze(root string) Report {
	return Report{
		General: general.Analyze(root),
		Git:     git.Analyze(root),
		Docker:  docker.Analyze(root),
		Infra:   infra.Analyze(root),
	}
}