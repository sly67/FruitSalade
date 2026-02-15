package gallery

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/logging"
	"github.com/fruitsalade/fruitsalade/fruitsalade/internal/storage"
)

// imageExtensions are file extensions treated as gallery images.
var imageExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif",
	".heic", ".heif", ".avif",
	// RAW formats
	".cr2", ".cr3", ".nef", ".arw", ".dng", ".orf", ".rw2", ".pef", ".srw", ".raf",
}

// IsImageFile checks if a file path has an image extension.
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range imageExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// Processor handles background image processing (EXIF extraction, thumbnails, plugin calls).
type Processor struct {
	store         *GalleryStore
	storageRouter *storage.Router
	pluginCaller  *PluginCaller
	queue         chan string
	wg            sync.WaitGroup
	cancel        context.CancelFunc
	workers       int
}

// NewProcessor creates a new image processor.
func NewProcessor(store *GalleryStore, router *storage.Router, pluginCaller *PluginCaller, workers int) *Processor {
	if workers <= 0 {
		workers = 2
	}
	return &Processor{
		store:         store,
		storageRouter: router,
		pluginCaller:  pluginCaller,
		queue:         make(chan string, 1000),
		workers:       workers,
	}
}

// Start launches the worker goroutines.
func (p *Processor) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
	logging.Info("gallery processor started", zap.Int("workers", p.workers))
}

// Stop signals workers to stop and waits for them to finish.
func (p *Processor) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	close(p.queue)
	p.wg.Wait()
	logging.Info("gallery processor stopped")
}

// Enqueue adds a file path to the processing queue.
func (p *Processor) Enqueue(filePath string) {
	// Create a pending row so we track it
	ctx := context.Background()
	if err := p.store.EnsureRow(ctx, filePath); err != nil {
		return
	}

	select {
	case p.queue <- filePath:
	default:
		logging.Warn("gallery processor queue full, dropping", zap.String("path", filePath))
	}
}

// ProcessExisting finds all unprocessed images and enqueues them.
func (p *Processor) ProcessExisting(ctx context.Context) {
	// First, check for images in files table with no image_metadata row
	unprocessed, err := p.store.ListUnprocessedImages(ctx, imageExtensions, 1000)
	if err != nil {
		logging.Warn("failed to list unprocessed images", zap.Error(err))
		return
	}
	for _, path := range unprocessed {
		p.Enqueue(path)
	}

	// Then, re-queue any stuck in 'pending' status
	pending, err := p.store.ListPendingProcessing(ctx, 1000)
	if err != nil {
		logging.Warn("failed to list pending images", zap.Error(err))
		return
	}
	for _, path := range pending {
		select {
		case p.queue <- path:
		default:
		}
	}

	total := len(unprocessed) + len(pending)
	if total > 0 {
		logging.Info("gallery: enqueued existing images for processing", zap.Int("count", total))
	}
}

func (p *Processor) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case filePath, ok := <-p.queue:
			if !ok {
				return
			}
			p.processImage(ctx, filePath)
		}
	}
}

func (p *Processor) processImage(ctx context.Context, filePath string) {
	if err := p.store.SetStatus(ctx, filePath, "processing"); err != nil {
		logging.Warn("gallery: failed to set processing status", zap.String("path", filePath), zap.Error(err))
		return
	}

	s3Key := strings.TrimPrefix(filePath, "/")

	// Resolve storage backend
	backend, _, err := p.storageRouter.GetDefault()
	if err != nil {
		logging.Warn("gallery: no default backend", zap.Error(err))
		p.store.SetStatus(ctx, filePath, "failed")
		return
	}

	// Read the file content
	reader, _, err := backend.GetObject(ctx, s3Key, 0, 0)
	if err != nil {
		logging.Warn("gallery: failed to read file", zap.String("path", filePath), zap.Error(err))
		p.store.SetStatus(ctx, filePath, "failed")
		return
	}

	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		logging.Warn("gallery: failed to read content", zap.String("path", filePath), zap.Error(err))
		p.store.SetStatus(ctx, filePath, "failed")
		return
	}

	// Extract EXIF
	exifData, err := ExtractExif(bytes.NewReader(content))
	if err != nil {
		logging.Warn("gallery: EXIF extraction failed", zap.String("path", filePath), zap.Error(err))
		// Continue anyway — still generate thumbnail
		exifData = &ExifData{Orientation: 1}
	}

	meta := &ImageMetadata{
		FilePath:        filePath,
		CameraMake:      exifData.CameraMake,
		CameraModel:     exifData.CameraModel,
		LensModel:       exifData.LensModel,
		FocalLength:     exifData.FocalLength,
		Aperture:        exifData.Aperture,
		ShutterSpeed:    exifData.ShutterSpeed,
		ISO:             exifData.ISO,
		Flash:           exifData.Flash,
		DateTaken:       exifData.DateTaken,
		Latitude:        exifData.Latitude,
		Longitude:       exifData.Longitude,
		Altitude:        exifData.Altitude,
		Orientation:     exifData.Orientation,
		LocationCountry: "",
		LocationCity:    "",
		LocationName:    "",
		Status:          "done",
	}

	// Get image dimensions and generate thumbnail
	ext := strings.ToLower(filepath.Ext(filePath))
	canThumbnail := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".bmp"

	if canThumbnail {
		thumbBytes, w, h, err := GenerateThumbnail(bytes.NewReader(content), exifData.Orientation)
		if err != nil {
			logging.Warn("gallery: thumbnail generation failed", zap.String("path", filePath), zap.Error(err))
		} else {
			// Store thumbnail
			thumbKey := ThumbS3Key(s3Key)
			if err := backend.PutObject(ctx, thumbKey, bytes.NewReader(thumbBytes), int64(len(thumbBytes))); err != nil {
				logging.Warn("gallery: failed to store thumbnail", zap.String("path", filePath), zap.Error(err))
			} else {
				meta.HasThumbnail = true
				meta.ThumbS3Key = thumbKey
			}
			_ = w
			_ = h
		}

		// Get full image dimensions (from EXIF or decoded)
		if exifData.Width > 0 && exifData.Height > 0 {
			meta.Width = exifData.Width
			meta.Height = exifData.Height
		} else {
			if imgW, imgH, err := ImageDimensions(bytes.NewReader(content)); err == nil {
				meta.Width = imgW
				meta.Height = imgH
			}
		}
	}

	// Save metadata
	if err := p.store.UpsertMetadata(ctx, meta); err != nil {
		logging.Warn("gallery: failed to save metadata", zap.String("path", filePath), zap.Error(err))
		return
	}

	// Call plugins
	if p.pluginCaller != nil {
		p.pluginCaller.CallPlugins(ctx, p.store, filePath, filepath.Base(filePath), int64(len(content)))
	}

	logging.Debug("gallery: processed image",
		zap.String("path", filePath),
		zap.Bool("thumbnail", meta.HasThumbnail),
		zap.Int("width", meta.Width),
		zap.Int("height", meta.Height))
}
