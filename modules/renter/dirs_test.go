package renter

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/turtledex/TurtleDexCore/build"
	"github.com/turtledex/TurtleDexCore/modules"
	"github.com/turtledex/TurtleDexCore/modules/renter/filesystem"
	"github.com/turtledex/TurtleDexCore/modules/renter/filesystem/ttdxdir"
	"github.com/turtledex/TurtleDexCore/siatest/dependencies"
	"github.com/turtledex/errors"
)

// FileListCollect returns information on all of the files stored by the
// renter at the specified folder. The 'cached' argument specifies whether
// cached values should be returned or not.
func (r *Renter) FileListCollect(siaPath modules.TurtleDexPath, recursive, cached bool) ([]modules.FileInfo, error) {
	var files []modules.FileInfo
	var mu sync.Mutex
	err := r.FileList(siaPath, recursive, cached, func(fi modules.FileInfo) {
		mu.Lock()
		files = append(files, fi)
		mu.Unlock()
	})
	// Sort slices by TurtleDexPath.
	sort.Slice(files, func(i, j int) bool {
		return files[i].TurtleDexPath.String() < files[j].TurtleDexPath.String()
	})
	return files, err
}

// TestRenterCreateDirectories checks that the renter properly created metadata files
// for direcotries
func TestRenterCreateDirectories(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	rt, err := newRenterTesterWithDependency(t.Name(), &dependencies.DependencyDisableRepairAndHealthLoops{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	// Test creating directory
	siaPath, err := modules.NewTurtleDexPath("foo/bar/baz")
	if err != nil {
		t.Fatal(err)
	}
	err = rt.renter.CreateDir(siaPath, modules.DefaultDirPerm)
	if err != nil {
		t.Fatal(err)
	}

	// Confirm that directory metadata files were created in all directories
	if err := rt.checkDirInitialized(modules.RootTurtleDexPath()); err != nil {
		t.Fatal(err)
	}
	siaPath, err = modules.NewTurtleDexPath("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.checkDirInitialized(siaPath); err != nil {
		t.Fatal(err)
	}
	siaPath, err = modules.NewTurtleDexPath("foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.checkDirInitialized(siaPath); err != nil {
		t.Fatal(err)
	}
	siaPath, err = modules.NewTurtleDexPath("foo/bar/baz")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.checkDirInitialized(siaPath); err != nil {
		t.Fatal(err)
	}
}

// checkDirInitialized is a helper function that checks that the directory was
// initialized correctly and the metadata file exist and contain the correct
// information
func (rt *renterTester) checkDirInitialized(siaPath modules.TurtleDexPath) (err error) {
	siaDir, err := rt.renter.staticFileSystem.OpenTurtleDexDir(siaPath)
	if err != nil {
		return fmt.Errorf("unable to load directory %v metadata: %v", siaPath, err)
	}
	defer func() {
		err = errors.Compose(err, siaDir.Close())
	}()
	fullpath := siaPath.TurtleDexDirMetadataSysPath(rt.renter.staticFileSystem.Root())
	if _, err := os.Stat(fullpath); err != nil {
		return err
	}

	// Check that metadata is default value
	metadata, err := siaDir.Metadata()
	if err != nil {
		return err
	}
	// Check Aggregate Fields
	if metadata.AggregateHealth != ttdxdir.DefaultDirHealth {
		return fmt.Errorf("AggregateHealth not initialized properly: have %v expected %v", metadata.AggregateHealth, ttdxdir.DefaultDirHealth)
	}
	if !metadata.AggregateLastHealthCheckTime.IsZero() {
		return fmt.Errorf("AggregateLastHealthCheckTime should be a zero timestamp: %v", metadata.AggregateLastHealthCheckTime)
	}
	if metadata.AggregateModTime.IsZero() {
		return fmt.Errorf("AggregateModTime not initialized: %v", metadata.AggregateModTime)
	}
	if metadata.AggregateMinRedundancy != ttdxdir.DefaultDirRedundancy {
		return fmt.Errorf("AggregateMinRedundancy not initialized properly: have %v expected %v", metadata.AggregateMinRedundancy, ttdxdir.DefaultDirRedundancy)
	}
	if metadata.AggregateStuckHealth != ttdxdir.DefaultDirHealth {
		return fmt.Errorf("AggregateStuckHealth not initialized properly: have %v expected %v", metadata.AggregateStuckHealth, ttdxdir.DefaultDirHealth)
	}
	// Check TurtleDexDir Fields
	if metadata.Health != ttdxdir.DefaultDirHealth {
		return fmt.Errorf("Health not initialized properly: have %v expected %v", metadata.Health, ttdxdir.DefaultDirHealth)
	}
	if !metadata.LastHealthCheckTime.IsZero() {
		return fmt.Errorf("LastHealthCheckTime should be a zero timestamp: %v", metadata.LastHealthCheckTime)
	}
	if metadata.ModTime.IsZero() {
		return fmt.Errorf("ModTime not initialized: %v", metadata.ModTime)
	}
	if metadata.MinRedundancy != ttdxdir.DefaultDirRedundancy {
		return fmt.Errorf("MinRedundancy not initialized properly: have %v expected %v", metadata.MinRedundancy, ttdxdir.DefaultDirRedundancy)
	}
	if metadata.StuckHealth != ttdxdir.DefaultDirHealth {
		return fmt.Errorf("StuckHealth not initialized properly: have %v expected %v", metadata.StuckHealth, ttdxdir.DefaultDirHealth)
	}
	path, err := siaDir.Path()
	if err != nil {
		return err
	}
	if path != rt.renter.staticFileSystem.DirPath(siaPath) {
		return fmt.Errorf("Expected path to be %v, got %v", path, rt.renter.staticFileSystem.DirPath(siaPath))
	}
	return nil
}

// TestDirInfo probes the DirInfo method
func TestDirInfo(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	rt, err := newRenterTesterWithDependency(t.Name(), &dependencies.DependencyDisableRepairAndHealthLoops{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	// Create directory
	siaPath, err := modules.NewTurtleDexPath("foo/")
	if err != nil {
		t.Fatal(err)
	}
	err = rt.renter.CreateDir(siaPath, modules.DefaultDirPerm)
	if err != nil {
		t.Fatal(err)
	}
	// Check that DirInfo returns the same information as stored in the metadata
	fooDirInfo, err := rt.renter.staticFileSystem.DirInfo(siaPath)
	if err != nil {
		t.Fatal(err)
	}
	rootDirInfo, err := rt.renter.staticFileSystem.DirInfo(modules.RootTurtleDexPath())
	if err != nil {
		t.Fatal(err)
	}
	fooEntry, err := rt.renter.staticFileSystem.OpenTurtleDexDir(siaPath)
	if err != nil {
		t.Fatal(err)
	}
	rootEntry, err := rt.renter.staticFileSystem.OpenTurtleDexDir(modules.RootTurtleDexPath())
	if err != nil {
		t.Fatal(err)
	}
	err = compareDirectoryInfoAndMetadata(fooDirInfo, fooEntry)
	if err != nil {
		t.Fatal(err)
	}
	err = compareDirectoryInfoAndMetadata(rootDirInfo, rootEntry)
	if err != nil {
		t.Fatal(err)
	}
}

// TestRenterListDirectory verifies that the renter properly lists the contents
// of a directory
func TestRenterListDirectory(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	rt, err := newRenterTesterWithDependency(t.Name(), &dependencies.DependencyDisableRepairAndHealthLoops{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	// Create directory
	siaPath, err := modules.NewTurtleDexPath("foo/")
	if err != nil {
		t.Fatal(err)
	}
	err = rt.renter.CreateDir(siaPath, modules.DefaultDirPerm)
	if err != nil {
		t.Fatal(err)
	}

	// Upload a file
	_, err = rt.renter.newRenterTestFile()
	if err != nil {
		t.Fatal(err)
	}

	// Confirm that we get expected number of FileInfo and DirectoryInfo.
	directories, err := rt.renter.DirList(modules.RootTurtleDexPath())
	if err != nil {
		t.Fatal(err)
	}
	// 5 Directories because of root, foo, home, var, and snapshots
	if len(directories) != 5 {
		t.Fatal("Expected 5 DirectoryInfos but got", len(directories))
	}
	files, err := rt.renter.FileListCollect(modules.RootTurtleDexPath(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatal("Expected 1 FileInfo but got", len(files))
	}

	// Refresh the directories blocking.
	for _, dir := range directories {
		err = rt.renter.managedBubbleMetadata(dir.TurtleDexPath)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Wait for root directory to show proper number of files and subdirs.
	err = build.Retry(100, 100*time.Millisecond, func() error {
		root, err := rt.renter.staticFileSystem.OpenTurtleDexDir(modules.RootTurtleDexPath())
		if err != nil {
			return err
		}
		rootMD, err := root.Metadata()
		if err != nil {
			return err
		}
		// Check the aggregate and ttdxdir fields.
		//
		// Expecting /home, /home/user, /var, /var/skynet, /snapshots, /foo
		if rootMD.AggregateNumSubDirs != 6 {
			return fmt.Errorf("Expected 6 subdirs in aggregate but got %v", rootMD.AggregateNumSubDirs)
		}
		if rootMD.NumSubDirs != 4 {
			return fmt.Errorf("Expected 4 subdirs but got %v", rootMD.NumSubDirs)
		}
		if rootMD.AggregateNumFiles != 1 {
			return fmt.Errorf("Expected 1 file in aggregate but got %v", rootMD.AggregateNumFiles)
		}
		if rootMD.NumFiles != 1 {
			return fmt.Errorf("Expected 1 file but got %v", rootMD.NumFiles)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the directory information matches the on disk information
	err = build.Retry(100, 100*time.Millisecond, func() error {
		rootDir, err := rt.renter.staticFileSystem.OpenTurtleDexDir(modules.RootTurtleDexPath())
		if err != nil {
			return err
		}
		fooDir, err := rt.renter.staticFileSystem.OpenTurtleDexDir(siaPath)
		if err != nil {
			return err
		}
		homeDir, err := rt.renter.staticFileSystem.OpenTurtleDexDir(modules.HomeFolder)
		if err != nil {
			return err
		}
		snapshotsDir, err := rt.renter.staticFileSystem.OpenTurtleDexDir(modules.BackupFolder)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Compose(err, rootDir.Close(), fooDir.Close(), homeDir.Close(), snapshotsDir.Close())
		}()

		// Refresh Directories
		directories, err = rt.renter.DirList(modules.RootTurtleDexPath())
		if err != nil {
			return err
		}
		// Sort directories.
		sort.Slice(directories, func(i, j int) bool {
			return strings.Compare(directories[i].TurtleDexPath.String(), directories[j].TurtleDexPath.String()) < 0
		})
		if err = compareDirectoryInfoAndMetadataCustom(directories[0], rootDir, false); err != nil {
			return err
		}
		if err = compareDirectoryInfoAndMetadataCustom(directories[1], fooDir, false); err != nil {
			return err
		}
		if err = compareDirectoryInfoAndMetadataCustom(directories[2], homeDir, false); err != nil {
			return err
		}
		if err = compareDirectoryInfoAndMetadataCustom(directories[3], snapshotsDir, false); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// compareDirectoryInfoAndMetadata is a helper that compares the information in
// a DirectoryInfo struct and a TurtleDexDirSetEntry struct
func compareDirectoryInfoAndMetadata(di modules.DirectoryInfo, siaDir *filesystem.DirNode) error {
	return compareDirectoryInfoAndMetadataCustom(di, siaDir, true)
}

// compareDirectoryInfoAndMetadataCustom is a helper that compares the
// information in a DirectoryInfo struct and a TurtleDexDirSetEntry struct with the
// option to ignore fields based on differences in persistence
func compareDirectoryInfoAndMetadataCustom(di modules.DirectoryInfo, siaDir *filesystem.DirNode, checkTimes bool) error {
	md, err := siaDir.Metadata()
	if err != nil {
		return err
	}

	// Compare Aggregate Fields
	if md.AggregateHealth != di.AggregateHealth {
		return fmt.Errorf("AggregateHealths not equal, %v and %v", md.AggregateHealth, di.AggregateHealth)
	}
	aggregateMaxHealth := math.Max(md.AggregateHealth, md.AggregateStuckHealth)
	if di.AggregateMaxHealth != aggregateMaxHealth {
		return fmt.Errorf("AggregateMaxHealths not equal %v and %v", di.AggregateMaxHealth, aggregateMaxHealth)
	}
	aggregateMaxHealthPercentage := modules.HealthPercentage(aggregateMaxHealth)
	if di.AggregateMaxHealthPercentage != aggregateMaxHealthPercentage {
		return fmt.Errorf("AggregateMaxHealthPercentage not equal %v and %v", di.AggregateMaxHealthPercentage, aggregateMaxHealthPercentage)
	}
	if md.AggregateMinRedundancy != di.AggregateMinRedundancy {
		return fmt.Errorf("AggregateMinRedundancy not equal, %v and %v", md.AggregateMinRedundancy, di.AggregateMinRedundancy)
	}
	if md.AggregateNumFiles != di.AggregateNumFiles {
		return fmt.Errorf("AggregateNumFiles not equal, %v and %v", md.AggregateNumFiles, di.AggregateNumFiles)
	}
	if md.AggregateNumStuckChunks != di.AggregateNumStuckChunks {
		return fmt.Errorf("AggregateNumStuckChunks not equal, %v and %v", md.AggregateNumStuckChunks, di.AggregateNumStuckChunks)
	}
	if md.AggregateNumSubDirs != di.AggregateNumSubDirs {
		return fmt.Errorf("AggregateNumSubDirs not equal, %v and %v", md.AggregateNumSubDirs, di.AggregateNumSubDirs)
	}
	if md.AggregateSize != di.AggregateSize {
		return fmt.Errorf("AggregateSizes not equal, %v and %v", md.AggregateSize, di.AggregateSize)
	}
	if md.NumStuckChunks != di.AggregateNumStuckChunks {
		return fmt.Errorf("NumStuckChunks not equal, %v and %v", md.NumStuckChunks, di.AggregateNumStuckChunks)
	}

	// Compare Aggregate Time Fields
	if checkTimes {
		if di.AggregateLastHealthCheckTime != md.AggregateLastHealthCheckTime {
			return fmt.Errorf("AggregateLastHealthCheckTimes not equal %v and %v", di.AggregateLastHealthCheckTime, md.AggregateLastHealthCheckTime)
		}
		if di.AggregateMostRecentModTime != md.AggregateModTime {
			return fmt.Errorf("AggregateModTimes not equal %v and %v", di.AggregateMostRecentModTime, md.AggregateModTime)
		}
	}

	// Compare Directory Fields
	if md.Health != di.Health {
		return fmt.Errorf("healths not equal, %v and %v", md.Health, di.Health)
	}
	maxHealth := math.Max(md.Health, md.StuckHealth)
	if di.MaxHealth != maxHealth {
		return fmt.Errorf("MaxHealths not equal %v and %v", di.MaxHealth, maxHealth)
	}
	maxHealthPercentage := modules.HealthPercentage(maxHealth)
	if di.MaxHealthPercentage != maxHealthPercentage {
		return fmt.Errorf("MaxHealthPercentage not equal %v and %v", di.MaxHealthPercentage, maxHealthPercentage)
	}
	if md.MinRedundancy != di.MinRedundancy {
		return fmt.Errorf("MinRedundancy not equal, %v and %v", md.MinRedundancy, di.MinRedundancy)
	}
	if md.NumFiles != di.NumFiles {
		return fmt.Errorf("NumFiles not equal, %v and %v", md.NumFiles, di.NumFiles)
	}
	if md.NumStuckChunks != di.NumStuckChunks {
		return fmt.Errorf("NumStuckChunks not equal, %v and %v", md.NumStuckChunks, di.NumStuckChunks)
	}
	if md.NumSubDirs != di.NumSubDirs {
		return fmt.Errorf("NumSubDirs not equal, %v and %v", md.NumSubDirs, di.NumSubDirs)
	}
	if md.Size != di.DirSize {
		return fmt.Errorf("Sizes not equal, %v and %v", md.Size, di.DirSize)
	}
	if md.StuckHealth != di.StuckHealth {
		return fmt.Errorf("stuck healths not equal, %v and %v", md.StuckHealth, di.StuckHealth)
	}

	// Compare Directory Time Fields
	if checkTimes {
		if di.LastHealthCheckTime != md.LastHealthCheckTime {
			return fmt.Errorf("LastHealthCheckTimes not equal %v and %v", di.LastHealthCheckTime, md.LastHealthCheckTime)
		}
		if di.MostRecentModTime != md.ModTime {
			return fmt.Errorf("ModTimes not equal %v and %v", di.MostRecentModTime, md.ModTime)
		}
	}
	return nil
}
