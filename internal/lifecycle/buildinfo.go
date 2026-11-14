package lifecycle

import "strings"

const (
	BuildModeDev     = "dev"
	BuildModeAlpha   = "alpha"
	BuildModeRelease = "release"

	DefaultDevAppVersion  = "2.0.1-dev"
	DefaultProdAppVersion = "2.0.1"
)

// Linker-injectable build identity variables:
//   -X github.com/boostao/vpro-wails/internal/lifecycle.appVersion=2.1.0-alpha.1
//   -X github.com/boostao/vpro-wails/internal/lifecycle.buildMode=alpha
//   -X github.com/boostao/vpro-wails/internal/lifecycle.gitCommit=30e86f6
//   -X github.com/boostao/vpro-wails/internal/lifecycle.buildDate=2026-11-14T16:30:00Z
var (
	appVersion = ""
	buildMode  = ""
	gitCommit  = ""
	buildDate  = ""
)

type BuildInfo struct {
	AppVersion     string `json:"appVersion"`
	BuildMode      string `json:"buildMode"`
	GitCommit      string `json:"gitCommit"`
	BuildDate      string `json:"buildDate"`
	IsPrerelease   bool   `json:"isPrerelease"`
	UnsignedNotice bool   `json:"unsignedNotice"`
}

func ResolveBuildInfo(rawVersion, rawMode, rawCommit, rawDate string, productionTag bool) BuildInfo {
	ver := strings.TrimPrefix(strings.TrimSpace(rawVersion), "v")
	explicitMode := strings.ToLower(strings.TrimSpace(rawMode))

	core := strings.SplitN(ver, "+", 2)[0]
	prereleaseSuffix := ""
	if idx := strings.IndexByte(core, '-'); idx >= 0 {
		prereleaseSuffix = strings.ToLower(core[idx+1:])
	}
	isDevSuffix := prereleaseSuffix == "dev" || strings.HasPrefix(prereleaseSuffix, "dev.")
	hasClientPrerelease := prereleaseSuffix != "" && !isDevSuffix

	var mode string
	switch explicitMode {
	case BuildModeDev:
		mode = BuildModeDev
	case BuildModeAlpha, "preview", "prerelease", "rc", "beta":
		mode = BuildModeAlpha
	case BuildModeRelease, "prod", "production", "stable":
		if hasClientPrerelease {
			mode = BuildModeAlpha
		} else {
			mode = BuildModeRelease
		}
	default:
		if !productionTag || isDevSuffix {
			mode = BuildModeDev
		} else if hasClientPrerelease {
			mode = BuildModeAlpha
		} else {
			mode = BuildModeRelease
		}
	}

	if ver == "" {
		if mode == BuildModeDev {
			ver = DefaultDevAppVersion
		} else {
			ver = DefaultProdAppVersion
		}
	}

	commit := strings.TrimSpace(rawCommit)
	if commit == "" && mode == BuildModeDev {
		commit = "local"
	}

	return BuildInfo{
		AppVersion:     ver,
		BuildMode:      mode,
		GitCommit:      commit,
		BuildDate:      strings.TrimSpace(rawDate),
		IsPrerelease:   mode == BuildModeAlpha || hasClientPrerelease,
		UnsignedNotice: true,
	}
}

func CurrentBuildInfo() BuildInfo {
	return ResolveBuildInfo(appVersion, buildMode, gitCommit, buildDate, compiledProductionTag)
}
