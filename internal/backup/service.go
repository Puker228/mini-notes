package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	db         *sql.DB
	backupDir  string
	uploadsDir string
}

func NewService(db *sql.DB, backupDir, uploadsDir string) *Service {
	return &Service{
		db:         db,
		backupDir:  backupDir,
		uploadsDir: uploadsDir,
	}
}

const timeLayout = "2006-01-02T15-04-05Z07-00"

func (s *Service) createDBBackup(ctx context.Context, now string) (string, error) {
	backupName := ".backup_" + now + ".sqlite.tmp"
	backupPath := filepath.Join(s.backupDir, backupName)

	_, err := s.db.ExecContext(ctx, "VACUUM INTO ?", backupPath)
	if err != nil {
		return "", err
	}
	return backupPath, nil
}

func (s *Service) Save(ctx context.Context) error {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return err
	}

	now := time.Now().UTC().Format(timeLayout)

	dbBackupPath, err := s.createDBBackup(ctx, now)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(dbBackupPath)
	}()

	zipPath := filepath.Join(s.backupDir, "backup_"+now+".zip")
	zipFile, err := os.OpenFile(zipPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(zipFile)
	closeAndRemove := func() {
		_ = archive.Close()
		_ = zipFile.Close()
		_ = os.Remove(zipPath)
	}

	if err := addFileToZip(archive, dbBackupPath, "db.sqlite"); err != nil {
		closeAndRemove()
		return err
	}
	if err := addDirToZip(archive, s.uploadsDir, "uploads"); err != nil {
		closeAndRemove()
		return err
	}

	manifest := Manifest{
		CreatedAt:  createdAt,
		AppVersion: "unknown",
		DBFile:     "db.sqlite",
		UploadsDir: "uploads",
	}
	if err := addJSONToZip(archive, "manifest.json", manifest); err != nil {
		closeAndRemove()
		return err
	}

	if err := archive.Close(); err != nil {
		_ = zipFile.Close()
		_ = os.Remove(zipPath)
		return err
	}
	if err := zipFile.Close(); err != nil {
		_ = os.Remove(zipPath)
		return err
	}

	return nil
}

func (s *Service) Restore(ctx context.Context) error { return nil }

func addDirToZip(archive *zip.Writer, srcDir, archiveDir string) error {
	_, err := archive.CreateHeader(&zip.FileHeader{
		Name:     filepath.ToSlash(archiveDir) + "/",
		Method:   zip.Store,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}

	return filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		archivePath := filepath.ToSlash(filepath.Join(archiveDir, relPath))
		if entry.IsDir() {
			_, err := archive.CreateHeader(&zip.FileHeader{
				Name:     archivePath + "/",
				Method:   zip.Store,
				Modified: info.ModTime(),
			})
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		return addFileToZip(archive, path, archivePath)
	})
}

func addFileToZip(archive *zip.Writer, srcPath, archivePath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate

	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func addJSONToZip(archive *zip.Writer, archivePath string, value any) error {
	writer, err := archive.Create(filepath.ToSlash(archivePath))
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
