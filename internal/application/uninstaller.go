package application

import (
	"errors"
	"slices"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

type Uninstaller struct {
	removables   []Removable
	prefix       prefix.Purgeable
	lock         WaitingLock
	activity     RunnerActivity
	installation install.Installation
	home         string
}

func NewUninstaller(removables []Removable, purgeable prefix.Purgeable, lock WaitingLock, activity RunnerActivity, installation install.Installation, home string) Uninstaller {
	return Uninstaller{removables: slices.Clone(removables), prefix: purgeable, lock: lock, activity: activity, installation: installation, home: home}
}

func (uninstaller Uninstaller) Uninstall(purge bool) error {
	if err := uninstaller.checkPurge(purge); err != nil {
		return err
	}
	release, err := uninstaller.lock.Acquire()
	if err != nil {
		return err
	}
	return errors.Join(uninstaller.removeWhenIdle(purge), release())
}

func (uninstaller Uninstaller) removeWhenIdle(purge bool) error {
	if err := ensureIdle(uninstaller.activity); err != nil {
		return err
	}
	return errors.Join(sequence.ForEach(slices.Values(uninstaller.removables), Removable.Remove), uninstaller.purgePrefix(purge))
}

func (uninstaller Uninstaller) checkPurge(purge bool) error {
	if !purge {
		return nil
	}
	return uninstaller.installation.PrefixRemovable(uninstaller.home, uninstaller.prefixCarriesOurTraces())
}

func (uninstaller Uninstaller) prefixCarriesOurTraces() bool {
	return uninstaller.prefix.Exists(uninstaller.installation.LauncherRelativePath()) || uninstaller.prefix.Exists(prefix.MarkerName)
}

func (uninstaller Uninstaller) purgePrefix(purge bool) error {
	if !purge {
		return nil
	}
	return uninstaller.prefix.Remove()
}

type LineRunnerRemoval struct {
	catalog runner.Catalog
}

func NewLineRunnerRemoval(catalog runner.Catalog) LineRunnerRemoval {
	return LineRunnerRemoval{catalog: catalog}
}

func (removal LineRunnerRemoval) Remove() error {
	return removal.catalog.Remove(runner.LineRunnerName)
}
