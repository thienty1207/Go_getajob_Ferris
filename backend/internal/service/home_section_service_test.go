package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

func TestHomeSectionUpsertRejectsActiveSectionWithoutImage(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	_, err := service.Upsert(context.Background(), HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Tìm việc phù hợp", Body: "Nội dung", ImageAltText: "Ảnh giới thiệu",
	})
	if !errors.Is(err, ErrInvalidHomeSection) {
		t.Fatalf("Upsert() error = %v, want ErrInvalidHomeSection", err)
	}
	if len(repo.upserts) != 0 || assets.uploadCalls != 0 {
		t.Fatalf("side effects: upserts=%d uploads=%d, want none", len(repo.upserts), assets.uploadCalls)
	}
}

func TestHomeSectionUpsertPersistsCloudinaryMetadata(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("home-section")...)
	section, err := service.Upsert(context.Background(), HomeSectionInput{
		Slot: 2, IsActive: true, Eyebrow: "Sugoi-oniichan", Title: "Bắt đầu từ CV", Body: "Chọn việc theo location.", ImageAltText: "Ferris cầm CV", File: makeFileHeader(t, "home.png", content), UpdatedBy: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if section.Layout != model.HomeSectionImageLeft || section.StorageProvider != "CLOUDINARY" || section.ImageURL != assets.asset.SecureURL {
		t.Fatalf("section = %#v, want image-left Cloudinary metadata", section)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].CloudinaryPublicID == nil || *repo.upserts[0].CloudinaryPublicID != assets.asset.PublicID {
		t.Fatalf("persisted write = %#v, want Cloudinary public ID", repo.upserts)
	}
	if assets.uploadCalls != 1 {
		t.Fatalf("uploads = %d, want 1", assets.uploadCalls)
	}
}

func TestHomeSectionUpsertDerivesImageAltFromTitle(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("derived-alt")...)
	section, err := service.Upsert(context.Background(), HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Cơ hội phù hợp hơn", Body: "Nội dung thật", File: makeFileHeader(t, "home.png", content),
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if section.ImageAltText != "Cơ hội phù hợp hơn" {
		t.Fatalf("ImageAltText = %q, want title-derived metadata", section.ImageAltText)
	}
	if len(repo.upserts) != 1 || homeTextValue(repo.upserts[0].ImageAltText) != "Cơ hội phù hợp hơn" {
		t.Fatalf("persisted ImageAltText = %#v, want title-derived metadata", repo.upserts[0].ImageAltText)
	}
}

func TestHomeSectionUpsertValidatesImageAltBeforeUpload(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)
	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("invalid-alt")...)

	_, err := service.Upsert(context.Background(), HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Title", Body: "Body",
		ImageAltText: strings.Repeat("a", maxHomeAlt+1),
		File:         makeFileHeader(t, "home.png", content),
	})
	if !errors.Is(err, ErrInvalidHomeSection) {
		t.Fatalf("Upsert() error = %v, want ErrInvalidHomeSection", err)
	}
	if assets.uploadCalls != 0 || len(repo.upserts) != 0 {
		t.Fatalf("side effects: uploads=%d upserts=%d, want validation before upload", assets.uploadCalls, len(repo.upserts))
	}
}

func TestHomeSectionUpsertValidatesActorBeforeUpload(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)
	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("invalid-actor")...)

	_, err := service.Upsert(context.Background(), HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Title", Body: "Body",
		UpdatedBy: strings.Repeat("a", 321),
		File:      makeFileHeader(t, "home.png", content),
	})
	if !errors.Is(err, ErrInvalidHomeSection) {
		t.Fatalf("Upsert() error = %v, want ErrInvalidHomeSection", err)
	}
	if assets.uploadCalls != 0 || len(repo.upserts) != 0 {
		t.Fatalf("side effects: uploads=%d upserts=%d, want actor validation before upload", assets.uploadCalls, len(repo.upserts))
	}
}

func TestHomeSectionUpsertRejectsMediaStripSlot(t *testing.T) {
	service := NewHomeSectionService(newHomeSectionRepositoryFake(), testConfig(1024), &homeSectionAssetStoreFake{})

	_, err := service.Upsert(context.Background(), HomeSectionInput{Slot: 4})
	if !errors.Is(err, ErrInvalidHomeSection) {
		t.Fatalf("Upsert() error = %v, want ErrInvalidHomeSection", err)
	}
}

func TestHomeSectionCreateMediaMapsValidationToMediaError(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	repo.sections[4] = model.HomeSection{ID: uuid.New(), Slot: 4, Layout: model.HomeSectionMediaStrip}
	service := NewHomeSectionService(repo, testConfig(1024), &homeSectionAssetStoreFake{})

	_, err := service.CreateMedia(context.Background(), HomeSectionMediaInput{
		SortOrder: 0, IsActive: true, ImageAltText: "", File: makeFileHeader(t, "strip.png", []byte("not-an-image")),
	})
	if !errors.Is(err, ErrInvalidHomeMedia) {
		t.Fatalf("CreateMedia() error = %v, want ErrInvalidHomeMedia", err)
	}
}

func TestHomeSectionCreateMediaDerivesImageAltWithoutUserInput(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	repo.sections[4] = model.HomeSection{ID: uuid.New(), Slot: 4, Layout: model.HomeSectionMediaStrip}
	service := NewHomeSectionService(repo, testConfig(1024), &homeSectionAssetStoreFake{})

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("media-alt")...)
	media, err := service.CreateMedia(context.Background(), HomeSectionMediaInput{
		SortOrder: 2, IsActive: true, File: makeFileHeader(t, "strip.png", content),
	})
	if err != nil {
		t.Fatalf("CreateMedia() error = %v", err)
	}
	if media.ImageAltText != "Ảnh dải Home 3" {
		t.Fatalf("ImageAltText = %q, want generated position label", media.ImageAltText)
	}
}

func TestHomeSectionCreateMediaValidatesAltBeforeUpload(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	repo.sections[4] = model.HomeSection{ID: uuid.New(), Slot: 4, Layout: model.HomeSectionMediaStrip}
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)
	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("invalid-media-alt")...)

	_, err := service.CreateMedia(context.Background(), HomeSectionMediaInput{
		SortOrder: 0, IsActive: true,
		ImageAltText: strings.Repeat("a", maxHomeAlt+1),
		File:         makeFileHeader(t, "strip.png", content),
	})
	if !errors.Is(err, ErrInvalidHomeMedia) {
		t.Fatalf("CreateMedia() error = %v, want ErrInvalidHomeMedia", err)
	}
	if assets.uploadCalls != 0 || len(repo.media) != 0 {
		t.Fatalf("side effects: uploads=%d media=%d, want validation before upload", assets.uploadCalls, len(repo.media))
	}
}

func TestHomeSectionCreateMediaRejectsNewUploadAfterTenItems(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	items := make([]model.HomeSectionMedia, 10)
	for index := range items {
		items[index] = model.HomeSectionMedia{ID: uuid.New(), SortOrder: int16(index), IsActive: true}
	}
	repo.sections[4] = model.HomeSection{ID: uuid.New(), Slot: 4, Layout: model.HomeSectionMediaStrip, Media: items}
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("media-limit")...)
	_, err := service.CreateMedia(context.Background(), HomeSectionMediaInput{
		SortOrder: 0, IsActive: true, File: makeFileHeader(t, "strip.png", content),
	})
	if !errors.Is(err, repository.ErrHomeSectionMediaLimit) {
		t.Fatalf("CreateMedia() error = %v, want ErrHomeSectionMediaLimit", err)
	}
	if assets.uploadCalls != 0 {
		t.Fatalf("uploadCalls = %d, want no Cloudinary upload after limit", assets.uploadCalls)
	}
}

func TestHomeSectionDeleteMediaCommitsMetadataBeforeProviderCleanup(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	mediaID := uuid.New()
	repo.media[mediaID] = model.HomeSectionMedia{ID: mediaID, CloudinaryPublicID: "ferris/home-sections/slot-4-hash"}
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	if err := service.DeleteMedia(context.Background(), mediaID); err != nil {
		t.Fatalf("DeleteMedia() error = %v", err)
	}
	if len(repo.media) != 0 || len(repo.cleanupJobs) != 1 {
		t.Fatalf("delete side effects: media=%#v cleanup=%#v", repo.media, repo.cleanupJobs)
	}
	if len(assets.destroyed) != 0 {
		t.Fatalf("provider asset was destroyed inside request path: %#v", assets.destroyed)
	}

	if err := service.processCleanupBatch(context.Background()); err != nil {
		t.Fatalf("processCleanupBatch() error = %v", err)
	}
	if len(assets.destroyed) != 1 || assets.destroyed[0] != "ferris/home-sections/slot-4-hash" {
		t.Fatalf("destroyed = %#v, want queued provider asset", assets.destroyed)
	}
}

func TestHomeSectionCleanupRetainsJobWhenCloudinaryFailsAndRetries(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	mediaID := uuid.New()
	repo.media[mediaID] = model.HomeSectionMedia{ID: mediaID, CloudinaryPublicID: "ferris/home-sections/slot-4-hash"}
	assets := &homeSectionAssetStoreFake{destroyErrors: []error{errors.New("cloudinary unavailable"), nil}}
	service := NewHomeSectionService(repo, testConfig(1024), assets)

	if err := service.DeleteMedia(context.Background(), mediaID); err != nil {
		t.Fatalf("DeleteMedia() error = %v", err)
	}
	if err := service.processCleanupBatch(context.Background()); !errors.Is(err, ErrHomeSectionStorage) {
		t.Fatalf("first processCleanupBatch() error = %v, want ErrHomeSectionStorage", err)
	}
	if len(repo.cleanupJobs) != 1 || len(repo.completedCleanup) != 0 {
		t.Fatalf("failed provider cleanup was acknowledged: jobs=%#v completed=%#v", repo.cleanupJobs, repo.completedCleanup)
	}
	if err := service.processCleanupBatch(context.Background()); err != nil {
		t.Fatalf("retry processCleanupBatch() error = %v", err)
	}
	if len(repo.cleanupJobs) != 0 || len(repo.completedCleanup) != 1 {
		t.Fatalf("successful retry was not acknowledged: jobs=%#v completed=%#v", repo.cleanupJobs, repo.completedCleanup)
	}
	for index, observation := range assets.destroyContexts {
		if observation.Err != nil || !observation.HasDeadline {
			t.Fatalf("destroy context %d = %#v, want fresh bounded context", index, observation)
		}
	}
}

func TestHomeSectionReplacementCleanupSurvivesCanceledRequest(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	repo.sections[1] = model.HomeSection{
		ID: uuid.New(), Slot: 1, Layout: model.HomeSectionContentLeft, IsActive: true,
		Title: "Cũ", Body: "Nội dung cũ", ImageAltText: "Ảnh cũ",
		ImageContentHash: strings.Repeat("a", 64), StorageProvider: "CLOUDINARY",
		CloudinaryPublicID: "ferris/home-sections/slot-1-old", ImageURL: "https://res.cloudinary.com/example/old.png",
	}
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	repo.cancelAfterMutation = cancelRequest

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("replacement")...)
	if _, err := service.Upsert(requestCtx, HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Mới", Body: "Nội dung mới",
		File: makeFileHeader(t, "replacement.png", content),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if requestCtx.Err() == nil {
		t.Fatal("test request context was not canceled after the database mutation")
	}
	if len(assets.destroyed) != 0 {
		t.Fatalf("replacement destroyed old asset on canceled request context: %#v", assets.destroyed)
	}
	if err := service.processCleanupBatch(context.Background()); err != nil {
		t.Fatalf("processCleanupBatch() error = %v", err)
	}
	if len(assets.destroyed) != 1 || assets.destroyed[0] != "ferris/home-sections/slot-1-old" {
		t.Fatalf("destroyed = %#v, want old queued asset", assets.destroyed)
	}
}

func TestHomeSectionUpsertQueuesUploadedAssetWhenDatabaseWriteFails(t *testing.T) {
	repo := newHomeSectionRepositoryFake()
	repo.upsertErr = errors.New("database unavailable")
	assets := &homeSectionAssetStoreFake{}
	service := NewHomeSectionService(repo, testConfig(1024), assets)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	content := append([]byte("\x89PNG\r\n\x1a\n"), []byte("orphan")...)
	_, err := service.Upsert(requestCtx, HomeSectionInput{
		Slot: 1, IsActive: true, Title: "Title", Body: "Body",
		File: makeFileHeader(t, "orphan.png", content),
	})
	if err == nil {
		t.Fatal("Upsert() error = nil, want database failure")
	}
	if len(repo.cleanupJobs) != 1 || repo.cleanupJobs[0].PublicID != assets.asset.PublicID {
		t.Fatalf("cleanup jobs = %#v, want uploaded unowned asset", repo.cleanupJobs)
	}
	if len(repo.enqueueContexts) != 1 || repo.enqueueContexts[0].Err != nil || !repo.enqueueContexts[0].HasDeadline {
		t.Fatalf("enqueue context = %#v, want fresh bounded context", repo.enqueueContexts)
	}
}

type homeSectionAssetStoreFake struct {
	asset           PromotionAsset
	uploadResults   []PromotionAsset
	uploadCalls     int
	uploadErr       error
	destroyed       []string
	destroyErr      error
	destroyErrors   []error
	destroyContexts []homeCleanupContextObservation
	events          *[]string
}

func (s *homeSectionAssetStoreFake) UploadHomeSection(_ context.Context, _ []byte, _ string, slot int16, _ string) (PromotionAsset, error) {
	s.uploadCalls++
	if s.uploadErr != nil {
		return PromotionAsset{}, s.uploadErr
	}
	if len(s.uploadResults) > 0 {
		s.asset = s.uploadResults[0]
		s.uploadResults = s.uploadResults[1:]
	} else {
		s.asset = PromotionAsset{
			PublicID:  fmt.Sprintf("ferris/home-sections/slot-%d-owned-%d", slot, s.uploadCalls),
			AssetID:   fmt.Sprintf("asset-%d", s.uploadCalls),
			SecureURL: fmt.Sprintf("https://res.cloudinary.com/example/image/upload/home-%d.png", s.uploadCalls),
		}
	}
	return s.asset, nil
}

func (s *homeSectionAssetStoreFake) Destroy(ctx context.Context, publicID string) error {
	if s.events != nil {
		*s.events = append(*s.events, "destroy")
	}
	s.destroyed = append(s.destroyed, publicID)
	s.destroyContexts = append(s.destroyContexts, observeHomeCleanupContext(ctx))
	if len(s.destroyErrors) > 0 {
		err := s.destroyErrors[0]
		s.destroyErrors = s.destroyErrors[1:]
		return err
	}
	return s.destroyErr
}

type homeSectionRepositoryFake struct {
	sections            map[int16]model.HomeSection
	media               map[uuid.UUID]model.HomeSectionMedia
	upserts             []model.HomeSectionWrite
	cleanupJobs         []repository.HomeAssetCleanup
	completedCleanup    []uuid.UUID
	upsertErr           error
	deleteErr           error
	enqueueErr          error
	claimErr            error
	completeErr         error
	cancelAfterMutation func()
	enqueueContexts     []homeCleanupContextObservation
	events              []string
}

func newHomeSectionRepositoryFake() *homeSectionRepositoryFake {
	return &homeSectionRepositoryFake{sections: make(map[int16]model.HomeSection), media: make(map[uuid.UUID]model.HomeSectionMedia)}
}

func (r *homeSectionRepositoryFake) ListHomeSections(context.Context, bool) ([]model.HomeSection, error) {
	sections := make([]model.HomeSection, 0, len(r.sections))
	for _, section := range r.sections {
		sections = append(sections, section)
	}
	return sections, nil
}

func (r *homeSectionRepositoryFake) GetHomeSection(_ context.Context, slot int16) (model.HomeSection, error) {
	section, ok := r.sections[slot]
	if !ok {
		return model.HomeSection{}, repository.ErrHomeSectionNotFound
	}
	section.Media = append([]model.HomeSectionMedia(nil), section.Media...)
	return section, nil
}

func (r *homeSectionRepositoryFake) UpsertHomeSection(_ context.Context, write model.HomeSectionWrite) (model.HomeSection, error) {
	if r.upsertErr != nil {
		return model.HomeSection{}, r.upsertErr
	}
	id := uuid.New()
	existing, exists := r.sections[write.Slot]
	if exists {
		id = existing.ID
	}
	section := model.HomeSection{ID: id, Slot: write.Slot, Layout: write.Layout, IsActive: write.IsActive, Eyebrow: homeTextValue(write.Eyebrow), Title: homeTextValue(write.Title), Body: homeTextValue(write.Body), ImageAltText: homeTextValue(write.ImageAltText), ImageContentHash: homeTextValue(write.ImageContentHash), StorageProvider: homeTextValue(write.StorageProvider), CloudinaryPublicID: homeTextValue(write.CloudinaryPublicID), ImageURL: homeTextValue(write.CloudinarySecureURL), CloudinaryAssetID: homeTextValue(write.CloudinaryAssetID), TargetURL: homeTextValue(write.TargetURL), UpdatedBy: homeTextValue(write.UpdatedBy)}
	if exists {
		section.Media = existing.Media
		if write.ImageContentHash == nil {
			section.ImageContentHash = existing.ImageContentHash
			section.StorageProvider = existing.StorageProvider
			section.CloudinaryPublicID = existing.CloudinaryPublicID
			section.ImageURL = existing.ImageURL
			section.CloudinaryAssetID = existing.CloudinaryAssetID
		}
	}
	r.sections[write.Slot] = section
	r.upserts = append(r.upserts, write)
	r.events = append(r.events, "upsert")
	if exists && existing.CloudinaryPublicID != "" && existing.CloudinaryPublicID != section.CloudinaryPublicID {
		r.enqueueCleanup(existing.CloudinaryPublicID)
	}
	if r.cancelAfterMutation != nil {
		r.cancelAfterMutation()
	}
	return section, nil
}

func (r *homeSectionRepositoryFake) CreateHomeSectionMedia(_ context.Context, write model.HomeSectionMediaWrite) (model.HomeSectionMedia, error) {
	if _, ok := r.sections[4]; !ok {
		return model.HomeSectionMedia{}, repository.ErrHomeSectionNotFound
	}
	media := model.HomeSectionMedia{ID: uuid.New(), SortOrder: write.SortOrder, IsActive: write.IsActive, ImageAltText: write.ImageAltText, ImageContentHash: write.ImageContentHash, StorageProvider: write.StorageProvider, CloudinaryPublicID: write.CloudinaryPublicID, ImageURL: write.CloudinarySecureURL, CloudinaryAssetID: write.CloudinaryAssetID, TargetURL: homeTextValue(write.TargetURL)}
	r.media[media.ID] = media
	return media, nil
}

func (r *homeSectionRepositoryFake) GetHomeSectionMedia(_ context.Context, id uuid.UUID) (model.HomeSectionMedia, error) {
	media, ok := r.media[id]
	if !ok {
		return model.HomeSectionMedia{}, repository.ErrHomeSectionMediaNotFound
	}
	return media, nil
}

func (r *homeSectionRepositoryFake) UpdateHomeSectionMedia(_ context.Context, id uuid.UUID, update model.HomeSectionMediaUpdate) (model.HomeSectionMedia, error) {
	media, ok := r.media[id]
	if !ok {
		return model.HomeSectionMedia{}, repository.ErrHomeSectionMediaNotFound
	}
	if update.SortOrder != nil {
		media.SortOrder = *update.SortOrder
	}
	if update.IsActive != nil {
		media.IsActive = *update.IsActive
	}
	if update.ImageAltText != nil {
		media.ImageAltText = *update.ImageAltText
	}
	if update.TargetURL != nil {
		media.TargetURL = *update.TargetURL
	}
	r.media[id] = media
	return media, nil
}

func (r *homeSectionRepositoryFake) DeleteHomeSectionMedia(_ context.Context, id uuid.UUID) (model.HomeSectionMedia, error) {
	if r.deleteErr != nil {
		return model.HomeSectionMedia{}, r.deleteErr
	}
	media, ok := r.media[id]
	if !ok {
		return model.HomeSectionMedia{}, repository.ErrHomeSectionMediaNotFound
	}
	delete(r.media, id)
	r.events = append(r.events, "delete")
	r.enqueueCleanup(media.CloudinaryPublicID)
	if r.cancelAfterMutation != nil {
		r.cancelAfterMutation()
	}
	return media, nil
}

func (r *homeSectionRepositoryFake) EnqueueHomeAssetCleanup(ctx context.Context, publicID string) error {
	r.events = append(r.events, "enqueue-unowned")
	r.enqueueContexts = append(r.enqueueContexts, observeHomeCleanupContext(ctx))
	if r.enqueueErr != nil {
		return r.enqueueErr
	}
	r.enqueueCleanup(publicID)
	return nil
}

func (r *homeSectionRepositoryFake) ClaimHomeAssetCleanup(context.Context, int, time.Duration) ([]repository.HomeAssetCleanup, error) {
	r.events = append(r.events, "claim")
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	return append([]repository.HomeAssetCleanup(nil), r.cleanupJobs...), nil
}

func (r *homeSectionRepositoryFake) CompleteHomeAssetCleanup(_ context.Context, id uuid.UUID) error {
	r.events = append(r.events, "complete")
	if r.completeErr != nil {
		return r.completeErr
	}
	r.completedCleanup = append(r.completedCleanup, id)
	for index := range r.cleanupJobs {
		if r.cleanupJobs[index].ID == id {
			r.cleanupJobs = append(r.cleanupJobs[:index], r.cleanupJobs[index+1:]...)
			break
		}
	}
	return nil
}

func (r *homeSectionRepositoryFake) enqueueCleanup(publicID string) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" || r.hasAssetReference(publicID) {
		return
	}
	for _, job := range r.cleanupJobs {
		if job.PublicID == publicID {
			return
		}
	}
	r.cleanupJobs = append(r.cleanupJobs, repository.HomeAssetCleanup{ID: uuid.New(), PublicID: publicID})
	r.events = append(r.events, "enqueue")
}

func (r *homeSectionRepositoryFake) hasAssetReference(publicID string) bool {
	for _, section := range r.sections {
		if section.CloudinaryPublicID == publicID {
			return true
		}
	}
	for _, media := range r.media {
		if media.CloudinaryPublicID == publicID {
			return true
		}
	}
	return false
}

type homeCleanupContextObservation struct {
	Err         error
	HasDeadline bool
}

func observeHomeCleanupContext(ctx context.Context) homeCleanupContextObservation {
	_, hasDeadline := ctx.Deadline()
	return homeCleanupContextObservation{Err: ctx.Err(), HasDeadline: hasDeadline}
}

func homeTextValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
