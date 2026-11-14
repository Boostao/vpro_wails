package lifecycle

import (
	"context"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

func TestResolveBuildInfoTiers(t *testing.T) {
	cases := []struct {
		name           string
		rawVersion     string
		rawMode        string
		rawCommit      string
		rawDate        string
		productionTag  bool
		wantVersion    string
		wantMode       string
		wantCommit     string
		wantPrerelease bool
	}{
		{
			name:           "default dev binary without production tag",
			productionTag:  false,
			wantVersion:    DefaultDevAppVersion,
			wantMode:       BuildModeDev,
			wantCommit:     "local",
			wantPrerelease: false,
		},
		{
			name:           "dev binary with git sha",
			rawCommit:      "30e86f6",
			productionTag:  false,
			wantVersion:    DefaultDevAppVersion,
			wantMode:       BuildModeDev,
			wantCommit:     "30e86f6",
			wantPrerelease: false,
		},
		{
			name:           "production alpha prerelease inferred from SemVer",
			rawVersion:     "v2.1.0-alpha.1",
			rawCommit:      "30e86f6abc",
			rawDate:        "2026-11-14T16:00:00Z",
			productionTag:  true,
			wantVersion:    "2.1.0-alpha.1",
			wantMode:       BuildModeAlpha,
			wantCommit:     "30e86f6abc",
			wantPrerelease: true,
		},
		{
			name:           "production rc prerelease inferred as alpha tier",
			rawVersion:     "2.1.0-rc.2+build.4",
			productionTag:  true,
			wantVersion:    "2.1.0-rc.2+build.4",
			wantMode:       BuildModeAlpha,
			wantCommit:     "",
			wantPrerelease: true,
		},
		{
			name:           "explicit alpha buildMode override",
			rawVersion:     "2.1.0",
			rawMode:        "alpha",
			productionTag:  false,
			wantVersion:    "2.1.0",
			wantMode:       BuildModeAlpha,
			wantCommit:     "",
			wantPrerelease: true,
		},
		{
			name:           "production clean SemVer resolves to release",
			rawVersion:     "v2.1.0",
			rawCommit:      "deadbeef1234",
			productionTag:  true,
			wantVersion:    "2.1.0",
			wantMode:       BuildModeRelease,
			wantCommit:     "deadbeef1234",
			wantPrerelease: false,
		},
		{
			name:           "production empty version falls back to DefaultProdAppVersion",
			productionTag:  true,
			wantVersion:    DefaultProdAppVersion,
			wantMode:       BuildModeRelease,
			wantCommit:     "",
			wantPrerelease: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBuildInfo(tc.rawVersion, tc.rawMode, tc.rawCommit, tc.rawDate, tc.productionTag)
			if got.AppVersion != tc.wantVersion ||
				got.BuildMode != tc.wantMode ||
				got.GitCommit != tc.wantCommit ||
				got.IsPrerelease != tc.wantPrerelease ||
				!got.UnsignedNotice {
				t.Fatalf("ResolveBuildInfo(%q, %q, %q, prod=%v) = %+v, want ver=%q mode=%q sha=%q pre=%v",
					tc.rawVersion, tc.rawMode, tc.rawCommit, tc.productionTag, got,
					tc.wantVersion, tc.wantMode, tc.wantCommit, tc.wantPrerelease)
			}
		})
	}
}

func TestHelpAndStartupExposeBuildTierAndHideDevPathsOutsideDev(t *testing.T) {
	origVer, origMode, origCommit := appVersion, buildMode, gitCommit
	t.Cleanup(func() {
		appVersion, buildMode, gitCommit = origVer, origMode, origCommit
	})

	dataDir := t.TempDir()
	configDir := t.TempDir()
	prefs, err := vprocore.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewHelpService(prefs)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetDataDir(dataDir)
	ctx := context.Background()

	// 1. Dev mode exposes configPath and dataDir diagnostics.
	appVersion = "2.0.1-dev"
	buildMode = "dev"
	gitCommit = "abc1234"
	devAbout, err := svc.GetAboutInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if devAbout.BuildMode != BuildModeDev || devAbout.AppVersion != "2.0.1-dev" || devAbout.GitCommit != "abc1234" ||
		devAbout.ConfigPath == "" || devAbout.DataDir != dataDir {
		t.Fatalf("expected dev AboutInfo to expose dev diagnostics, got %+v", devAbout)
	}

	// 2. Alpha mode exposes prerelease + unsigned notice and omits local dev paths.
	appVersion = "2.1.0-alpha.1"
	buildMode = "alpha"
	gitCommit = "def5678"
	alphaAbout, err := svc.GetAboutInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if alphaAbout.BuildMode != BuildModeAlpha || alphaAbout.AppVersion != "2.1.0-alpha.1" ||
		!alphaAbout.IsPrerelease || !alphaAbout.UnsignedNotice ||
		alphaAbout.ConfigPath != "" || alphaAbout.DataDir != "" {
		t.Fatalf("expected alpha AboutInfo without dev path leakage, got %+v", alphaAbout)
	}

	startup := (&StartupService{State: StartupState{Ready: true, ReleaseChannel: vprocore.ReleaseChannelStable}}).GetState()
	if startup.BuildMode != BuildModeAlpha || startup.AppVersion != "2.1.0-alpha.1" || !startup.IsPrerelease || !startup.UnsignedNotice {
		t.Fatalf("expected StartupState to carry alpha build identity, got %+v", startup)
	}

	// 3. Release mode reports clean release identity.
	appVersion = "2.1.0"
	buildMode = "release"
	relAbout, err := svc.GetAboutInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if relAbout.BuildMode != BuildModeRelease || relAbout.AppVersion != "2.1.0" || relAbout.IsPrerelease ||
		relAbout.ConfigPath != "" || relAbout.DataDir != "" {
		t.Fatalf("expected release AboutInfo, got %+v", relAbout)
	}
}
