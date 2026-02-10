// Package webdav provides a WebDAV interface to FruitSalade storage.
package webdav

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/webdav"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
)

// FruitFS implements webdav.FileSystem backed by S3 + PostgreSQL.
type FruitFS struct {
	storage *s3storage.Storage
}

var _ webdav.FileSystem = (*FruitFS)(nil)

func normalizePath(name string) string {
	name = filepath.Clean(name)
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return name
}

// Mkdir creates a directory.
func (fs *FruitFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	name = normalizePath(name)
	if name == "/" {
		return nil
	}

	parentPath := filepath.Dir(name)
	if parentPath == "." {
		parentPath = "/"
	}

	h := sha256.Sum256([]byte(name))
	id := fmt.Sprintf("%x", h[:8])

	row := &postgres.FileRow{
		ID:         id,
		Name:       filepath.Base(name),
		Path:       name,
		ParentPath: parentPath,
		IsDir:      true,
		ModTime:    time.Now(),
	}

	// Set owner from context claims
	if claims := auth.GetClaims(ctx); claims != nil {
		ownerID := claims.UserID
		row.OwnerID = &ownerID
	}

	return fs.storage.Metadata().UpsertFile(ctx, row)
}

// OpenFile opens or creates a file.
func (fs *FruitFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	name = normalizePath(name)

	writable := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0

	if writable {
		return &FruitFile{
			fs:       fs,
			name:     name,
			writable: true,
			buf:      &bytes.Buffer{},
			ctx:      ctx,
		}, nil
	}

	// Read mode — check if it exists
	row, err := fs.storage.Metadata().GetFileRow(ctx, name)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, os.ErrNotExist
	}

	return &FruitFile{
		fs:   fs,
		name: name,
		row:  row,
		ctx:  ctx,
	}, nil
}

// RemoveAll removes a file or directory tree.
func (fs *FruitFS) RemoveAll(ctx context.Context, name string) error {
	name = normalizePath(name)
	if name == "/" {
		return fmt.Errorf("cannot remove root")
	}

	row, err := fs.storage.Metadata().GetFileRow(ctx, name)
	if err != nil {
		return err
	}
	if row == nil {
		return os.ErrNotExist
	}

	if row.IsDir {
		// Delete children from S3
		children, _ := fs.storage.Metadata().ListDir(ctx, name)
		for _, child := range children {
			if !child.IsDir {
				s3Key := strings.TrimPrefix(child.Path, "/")
				fs.storage.DeleteObject(ctx, s3Key)
			}
		}
		_, err := fs.storage.Metadata().DeleteTree(ctx, name)
		return err
	}

	// Single file
	s3Key := strings.TrimPrefix(name, "/")
	fs.storage.DeleteObject(ctx, s3Key)
	return fs.storage.Metadata().DeleteFile(ctx, name)
}

// Rename moves a file from oldName to newName.
func (fs *FruitFS) Rename(ctx context.Context, oldName, newName string) error {
	oldName = normalizePath(oldName)
	newName = normalizePath(newName)

	row, err := fs.storage.Metadata().GetFileRow(ctx, oldName)
	if err != nil {
		return err
	}
	if row == nil {
		return os.ErrNotExist
	}

	if row.IsDir {
		return fmt.Errorf("directory rename not supported")
	}

	// Copy S3 object
	oldKey := strings.TrimPrefix(oldName, "/")
	newKey := strings.TrimPrefix(newName, "/")
	if err := fs.storage.CopyObject(ctx, oldKey, newKey); err != nil {
		return err
	}

	// Create new metadata entry
	parentPath := filepath.Dir(newName)
	if parentPath == "." {
		parentPath = "/"
	}
	h := sha256.Sum256([]byte(newName))
	newRow := &postgres.FileRow{
		ID:         fmt.Sprintf("%x", h[:8]),
		Name:       filepath.Base(newName),
		Path:       newName,
		ParentPath: parentPath,
		Size:       row.Size,
		ModTime:    time.Now(),
		IsDir:      false,
		Hash:       row.Hash,
		S3Key:      newKey,
		Version:    1,
		OwnerID:    row.OwnerID,
	}
	if err := fs.storage.Metadata().UpsertFile(ctx, newRow); err != nil {
		return err
	}

	// Delete old
	fs.storage.DeleteObject(ctx, oldKey)
	return fs.storage.Metadata().DeleteFile(ctx, oldName)
}

// Stat returns file info for a path.
func (fs *FruitFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	name = normalizePath(name)

	if name == "/" {
		return &fileInfo{name: "/", isDir: true, modTime: time.Now()}, nil
	}

	row, err := fs.storage.Metadata().GetFileRow(ctx, name)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, os.ErrNotExist
	}

	return &fileInfo{
		name:    row.Name,
		size:    row.Size,
		isDir:   row.IsDir,
		modTime: row.ModTime,
	}, nil
}

// FruitFile implements webdav.File.
type FruitFile struct {
	fs       *FruitFS
	name     string
	row      *postgres.FileRow
	writable bool
	buf      *bytes.Buffer
	ctx      context.Context

	// Read state
	reader io.ReadCloser
	offset int64
	size   int64
}

var _ webdav.File = (*FruitFile)(nil)

func (f *FruitFile) Close() error {
	if f.reader != nil {
		f.reader.Close()
		f.reader = nil
	}

	if !f.writable || f.buf == nil || f.buf.Len() == 0 {
		return nil
	}

	// Upload buffer to S3
	name := f.name
	content := f.buf.Bytes()
	s3Key := strings.TrimPrefix(name, "/")

	if err := f.fs.storage.PutObject(f.ctx, s3Key, bytes.NewReader(content), int64(len(content))); err != nil {
		return err
	}

	// Compute hash
	h := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", h)

	// Check for existing entry (versioning)
	newVersion := 1
	existing, _ := f.fs.storage.Metadata().GetFileRow(f.ctx, name)
	if existing != nil && !existing.IsDir {
		newVersion = existing.Version + 1
	}

	parentPath := filepath.Dir(name)
	if parentPath == "." {
		parentPath = "/"
	}
	idH := sha256.Sum256([]byte(name))
	row := &postgres.FileRow{
		ID:         fmt.Sprintf("%x", idH[:8]),
		Name:       filepath.Base(name),
		Path:       name,
		ParentPath: parentPath,
		Size:       int64(len(content)),
		ModTime:    time.Now(),
		IsDir:      false,
		Hash:       hashStr,
		S3Key:      s3Key,
		Version:    newVersion,
	}

	if claims := auth.GetClaims(f.ctx); claims != nil && existing == nil {
		ownerID := claims.UserID
		row.OwnerID = &ownerID
	}

	if err := f.fs.storage.Metadata().UpsertFile(f.ctx, row); err != nil {
		return err
	}

	logging.Debug("webdav file written",
		zap.String("path", name),
		zap.Int("size", len(content)),
		zap.Int("version", newVersion))

	return nil
}

func (f *FruitFile) Read(p []byte) (int, error) {
	if f.writable {
		return 0, fmt.Errorf("file opened for writing")
	}

	// Lazy fetch from S3
	if f.reader == nil {
		if f.row == nil {
			return 0, io.EOF
		}
		reader, size, err := f.fs.storage.GetContent(f.ctx, f.row.ID, f.offset, 0)
		if err != nil {
			return 0, err
		}
		f.reader = reader
		f.size = size
	}

	n, err := f.reader.Read(p)
	f.offset += int64(n)
	return n, err
}

func (f *FruitFile) Write(p []byte) (int, error) {
	if !f.writable {
		return 0, fmt.Errorf("file not opened for writing")
	}
	return f.buf.Write(p)
}

func (f *FruitFile) Seek(offset int64, whence int) (int64, error) {
	var totalSize int64
	if f.row != nil {
		totalSize = f.row.Size
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		newOffset = totalSize + offset
	}

	if newOffset < 0 {
		return 0, fmt.Errorf("negative seek position")
	}

	// If we have an open reader and are seeking, close it to re-fetch
	if f.reader != nil && newOffset != f.offset {
		f.reader.Close()
		f.reader = nil
	}

	f.offset = newOffset
	return newOffset, nil
}

func (f *FruitFile) Readdir(count int) ([]os.FileInfo, error) {
	if f.row != nil && !f.row.IsDir {
		return nil, fmt.Errorf("not a directory")
	}

	children, err := f.fs.storage.ListDir(f.ctx, f.name)
	if err != nil {
		return nil, err
	}

	var infos []os.FileInfo
	for _, child := range children {
		infos = append(infos, &fileInfo{
			name:    child.Name,
			size:    child.Size,
			isDir:   child.IsDir,
			modTime: child.ModTime,
		})
	}

	if count > 0 && len(infos) > count {
		infos = infos[:count]
	}

	return infos, nil
}

func (f *FruitFile) Stat() (os.FileInfo, error) {
	if f.row != nil {
		return &fileInfo{
			name:    f.row.Name,
			size:    f.row.Size,
			isDir:   f.row.IsDir,
			modTime: f.row.ModTime,
		}, nil
	}
	// Root or new file
	if f.name == "/" {
		return &fileInfo{name: "/", isDir: true, modTime: time.Now()}, nil
	}
	if f.writable {
		return &fileInfo{
			name:    filepath.Base(f.name),
			size:    int64(f.buf.Len()),
			modTime: time.Now(),
		}, nil
	}
	return nil, os.ErrNotExist
}

// fileInfo implements os.FileInfo.
type fileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (fi *fileInfo) Name() string      { return fi.name }
func (fi *fileInfo) Size() int64       { return fi.size }
func (fi *fileInfo) IsDir() bool       { return fi.isDir }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) Sys() interface{}  { return nil }

func (fi *fileInfo) Mode() os.FileMode {
	if fi.isDir {
		return os.ModeDir | 0755
	}
	return 0644
}
