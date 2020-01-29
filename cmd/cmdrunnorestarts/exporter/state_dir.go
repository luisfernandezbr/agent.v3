package exporter

import (
	"fmt"
	"os"

	"github.com/pinpt/agent/pkg/fs"
)

func (s *Exporter) backupRestoreStateDir() error {
	locs := s.opts.FSConf

	stateExists, err := fs.Exists(locs.State)
	if err != nil {
		return err
	}
	if !stateExists {
		err = os.MkdirAll(locs.State, 0755)
		if err != nil {
			return fmt.Errorf("could not create dir to save state, err: %v", err)
		}
		return nil
	}

	backupExists, err := fs.Exists(locs.Backup)
	if err != nil {
		return err
	}

	if backupExists {

		s.logger.Info("previous export/upload did not finish since we found a backup dir, restoring previous state and trying again")

		// restore the backup, but also keep backup, so we could restore to it again

		if err := os.RemoveAll(locs.LastProcessedFile); err != nil {
			return err
		}

		if err := fs.CopyFile(locs.LastProcessedFileBackup, locs.LastProcessedFile); err != nil {
			// would happen when running first historical because backup does not have any state yet, but we should be able to recover to that in case of error
			if !os.IsNotExist(err) {
				return err
			}
		}

		if err := os.RemoveAll(locs.RipsrcCheckpoints); err != nil {
			return err
		}

		if err := fs.CopyDir(locs.RipsrcCheckpointsBackup, locs.RipsrcCheckpoints); err != nil {
			// would happen when running first historical because backup does not have any state yet, but we should be able to recover to that in case of error
			if !os.IsNotExist(err) {
				return err
			}
		}

		return nil
	}

	// save backup

	err = os.MkdirAll(locs.Backup, 0755)
	if err != nil {
		return err
	}

	if err := fs.CopyFile(locs.LastProcessedFile, locs.LastProcessedFileBackup); err != nil {
		// would happen if export did not create last processed file, which is very unlikely
		if !os.IsNotExist(err) {
			return err
		}
	}

	if err := fs.CopyDir(locs.RipsrcCheckpoints, locs.RipsrcCheckpointsBackup); err != nil {
		// would happen if export did not have any ripsrc data
		if !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (s *Exporter) deleteBackupStateDir() error {
	locs := s.opts.FSConf
	if err := os.RemoveAll(locs.Backup); err != nil {
		return fmt.Errorf("error deleting export backup file: %v", err)
	}
	return nil
}
