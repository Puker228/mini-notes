package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlitedriver "github.com/ncruces/go-sqlite3/driver"
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

	createdAt := time.Now().UTC()
	now := createdAt.Format(timeLayout)

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

func (s *Service) Restore(ctx context.Context, archive io.ReaderAt, size int64) error {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}

	manifest, err := readManifest(reader)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return err
	}

	tmpDBPath := filepath.Join(s.backupDir, ".restore_db.sqlite.tmp")
	if err := extractZipEntry(reader, manifest.DBFile, tmpDBPath); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmpDBPath)
	}()

	staging := s.uploadsDir + ".restore.tmp"
	_ = os.RemoveAll(staging)
	if err := extractUploads(reader, manifest.UploadsDir, staging); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}

	if err := s.restoreDB(ctx, tmpDBPath); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}

	if err := swapDir(staging, s.uploadsDir); err != nil {
		return err
	}

	return nil
}

func (s *Service) restoreDB(ctx context.Context, tmpDBPath string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		sqliteConn, ok := driverConn.(sqlitedriver.Conn)
		if !ok {
			return errors.New("backup restore: unexpected sqlite driver connection")
		}
		return sqliteConn.Raw().Restore("main", "file:"+filepath.ToSlash(tmpDBPath))
	})
}

func readManifest(reader *zip.Reader) (Manifest, error) {
	var manifest Manifest

	file, err := openZipEntry(reader, "manifest.json")
	if err != nil {
		return manifest, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.DBFile == "" {
		manifest.DBFile = "db.sqlite"
	}
	if manifest.UploadsDir == "" {
		manifest.UploadsDir = "uploads"
	}
	return manifest, nil
}

func openZipEntry(reader *zip.Reader, name string) (io.ReadCloser, error) {
	for _, file := range reader.File {
		if file.Name == name {
			return file.Open()
		}
	}
	return nil, fmt.Errorf("backup archive missing %q", name)
}

func extractZipEntry(reader *zip.Reader, name, destPath string) error {
	src, err := openZipEntry(reader, name)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}

func extractUploads(reader *zip.Reader, uploadsDir, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	prefix := uploadsDir + "/"
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file.Name, prefix)
		if rel == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(rel))
		if target != destDir && !strings.HasPrefix(target, destDir+string(os.PathSeparator)) {
			return fmt.Errorf("backup archive contains invalid path %q", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(file *zip.File, destPath string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}

func swapDir(staging, dst string) error {
	old := dst + ".old"
	_ = os.RemoveAll(old)

	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(staging, dst); err != nil {
		_ = os.Rename(old, dst) // rollback
		return err
	}

	_ = os.RemoveAll(old)
	return nil
}

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
