package sivi

import (
	"os"

	"github.com/boostao/vpro-wails/resources"
)

func init() {
	_ = os.Chdir(resources.RepoRoot())
}
