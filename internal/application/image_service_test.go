package application

import (
	"errors"
	"testing"

	"github.com/linskybing/platform-go/internal/domain/image"
	"gorm.io/gorm"
)

// fakeRepo implements the minimal methods used by ApproveRequest
type fakeRepo struct {
	reqs    map[uint]*image.ImageRequest
	created []*image.ImageAllowList
	nextID  uint
	repos   map[uint]*image.ContainerRepository
	tags    map[uint]*image.ContainerTag
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{reqs: make(map[uint]*image.ImageRequest), nextID: 1, repos: make(map[uint]*image.ContainerRepository), tags: make(map[uint]*image.ContainerTag)}
}

func (f *fakeRepo) CreateRequest(req *image.ImageRequest) error {
	if req.ID == 0 {
		req.ID = f.nextID
		f.nextID++
	}
	f.reqs[req.ID] = req
	return nil
}

func (f *fakeRepo) FindRequestByID(id uint) (*image.ImageRequest, error) {
	r, ok := f.reqs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (f *fakeRepo) UpdateRequest(req *image.ImageRequest) error {
	f.reqs[req.ID] = req
	return nil
}

func (f *fakeRepo) CreateAllowListRule(rule *image.ImageAllowList) error {
	// simulate DB assigning ID and storing created allowlist
	rule.ID = f.nextID
	f.nextID++
	if rule.TagID == nil {
		// ensure tag id exists
		t := uint(1)
		rule.TagID = &t
	}
	// Populate Repository and Tag objects from stored maps if possible
	if rule.RepositoryID != 0 {
		if r, ok := f.repos[rule.RepositoryID]; ok {
			rule.Repository = *r
		}
	}
	if rule.TagID != nil && *rule.TagID != 0 {
		if tg, ok := f.tags[*rule.TagID]; ok {
			rule.Tag = *tg
		}
	}
	f.created = append(f.created, rule)
	return nil
}

// Unused methods required by interface but not needed for this test
func (f *fakeRepo) ListRequests(projectID *uint, status string) ([]image.ImageRequest, error) {
	return nil, nil
}
func (f *fakeRepo) FindAllRequests() ([]image.ImageRequest, error)                 { return nil, nil }
func (f *fakeRepo) FindRequestsByUserID(userID uint) ([]image.ImageRequest, error) { return nil, nil }
func (f *fakeRepo) ListAllowedImages(projectID *uint) ([]image.ImageAllowList, error) {
	return nil, nil
}
func (f *fakeRepo) FindAllowListRule(projectID *uint, repoFullName, tagName string) (*image.ImageAllowList, error) {
	return nil, nil
}
func (f *fakeRepo) FindOrCreateRepository(repo *image.ContainerRepository) error {
	if repo.ID == 0 {
		repo.ID = f.nextID
		f.nextID++
	}
	// store or update
	f.repos[repo.ID] = &image.ContainerRepository{Model: repo.Model, Registry: repo.Registry, Namespace: repo.Namespace, Name: repo.Name, FullName: repo.FullName}
	// ensure FullName is set
	if f.repos[repo.ID].FullName == "" {
		f.repos[repo.ID].FullName = repo.FullName
	}
	return nil
}
func (f *fakeRepo) FindOrCreateTag(tag *image.ContainerTag) error {
	if tag.ID == 0 {
		tag.ID = f.nextID
		f.nextID++
	}
	f.tags[tag.ID] = &image.ContainerTag{Model: tag.Model, RepositoryID: tag.RepositoryID, Name: tag.Name, Digest: tag.Digest, Size: tag.Size, PushedAt: tag.PushedAt}
	return nil
}
func (f *fakeRepo) CheckImageAllowed(projectID *uint, repoFullName string, tagName string) (bool, error) {
	return false, nil
}
func (f *fakeRepo) DisableAllowListRule(id uint) error                             { return nil }
func (f *fakeRepo) UpdateClusterStatus(status *image.ClusterImageStatus) error     { return nil }
func (f *fakeRepo) GetClusterStatus(tagID uint) (*image.ClusterImageStatus, error) { return nil, nil }
func (f *fakeRepo) WithTx(tx *gorm.DB) image.Repository                            { return f }
func (f *fakeRepo) GetTagByDigest(repoID uint, digest string) (*image.ContainerTag, error) {
	return nil, nil
}

func TestApproveRequest(t *testing.T) {
	repo := newFakeRepo()
	svc := NewImageService(repo)

	// prepare a request
	req := &image.ImageRequest{
		UserID:         10,
		InputImageName: "myrepo/myimage",
		InputTag:       "v1",
		ProjectID:      nil,
		Status:         "pending",
	}
	// set ID and store in fake repo
	req.ID = 1
	repo.reqs[1] = req

	approver := uint(99)

	err := svc.ApproveRequest(1, "ok", approver)
	if err != nil {
		t.Fatalf("ApproveRequest returned error: %v", err)
	}

	// fetch updated request
	updatedReq, _ := repo.FindRequestByID(1)
	if updatedReq.Status != "approved" {
		t.Fatalf("expected request status approved, got %s", updatedReq.Status)
	}
	// verify allowed image created
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 allowed image created, got %d", len(repo.created))
	}
	ai := repo.created[0]
	if ai.Repository.FullName != req.InputImageName || ai.Tag.Name != req.InputTag {
		t.Fatalf("allowed image mismatch: %+v", ai)
	}
	if ai.RequestID == nil || *ai.RequestID != req.ID {
		t.Fatalf("request id on allowlist not set: %v", ai.RequestID)
	}
	if ai.CreatedBy != approver {
		t.Fatalf("created_by not recorded: %v", ai.CreatedBy)
	}
	if !ai.IsEnabled {
		t.Fatalf("allowlist rule not enabled")
	}
}
