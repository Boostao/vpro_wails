package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestWorkingUnitEmptyChoicesAndServerResolvedFamilyFile(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
 CREATE TABLE Ordinary_SU(PlotNumber TEXT,SiteUnit TEXT);
 CREATE TABLE BadSchema_SU(PlotNumber TEXT,SiteUnit INTEGER);`); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("Ordinary"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"env", "su"} {
		rows, err := service.GetWorkingUnitChoices(mode)
		if err != nil || rows == nil || len(rows) != 0 {
			t.Fatalf("empty %s choices silently fallback: %#v %v", mode, rows, err)
		}
	}
	projects.mu.Lock()
	projects.activeSU = "BadSchema"
	projects.mu.Unlock()
	if _, err := service.GetWorkingUnitChoices("su"); err == nil {
		t.Fatal("nontext SU schema accepted")
	}
	projects.mu.Lock()
	projects.activeSU = "Ordinary"
	projects.mu.Unlock()
	if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES('FAMILY');
 INSERT INTO Sample_Admin(Plot,UserSiteUnit) VALUES('FAMILY','server-resolved')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(projects.root, "projects", "Sample.db"), filepath.Join(projects.root, "projects", "Families.db")); err != nil {
		t.Fatal(err)
	}
	rows, err := service.GetWorkingUnitChoices("env")
	if err != nil || !reflect.DeepEqual(workingUnitCodes(rows), []string{"server-resolved"}) {
		t.Fatalf("lookup guessed project filename rather than resolving it: %#v %v", rows, err)
	}
}

func TestWorkingUnitConcurrentChoicesPreferencesAndContext(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	if _, err := db.Exec(`CREATE TABLE Choice_SU(PlotNumber TEXT,SiteUnit TEXT);
 INSERT INTO Choice_SU VALUES('off-project','001')`); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("Choice"); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	failures := make(chan error, 60)
	for worker := 0; worker < 3; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 10; i++ {
				var err error
				switch worker {
				case 0:
					_, err = service.GetWorkingUnitChoices("env")
				case 1:
					_, err = service.SetWorkingUnitMode([]string{"env", "master", "su"}[i%3])
				case 2:
					_, err = projects.SelectSU([]string{"None", "Choice"}[i%2])
				}
				if err != nil {
					failures <- err
				}
			}
		}(worker)
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent server-resolved operation: %v", err)
	}
}

func TestWorkingUnitForcedPreferenceFailureRetainsPriorMode(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	if _, err := db.Exec(`CREATE TABLE Choice_SU(PlotNumber TEXT,SiteUnit TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("Choice"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkingUnitMode("env"); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("None"); err != nil {
		t.Fatal(err)
	}
	service.replaceFile = func(string, string) error { return errors.New("controlled force-master failure") }
	if _, err := service.GetWorkingUnitMode(); err == nil {
		t.Fatal("forced preference failure returned success-shaped master fallback")
	}
	if mode, err := readWorkingUnitPreference(service.settingsPath); err != nil || mode != "env" {
		t.Fatalf("failed forced mode changed prior preference: %q %v", mode, err)
	}
}
