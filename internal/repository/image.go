package repository

import (
	"errors"

	"github.com/linskybing/platform-go/internal/domain/image"
	"gorm.io/gorm"
)

type ImageRepo interface {
	image.Repository
}

type DBImageRepo struct {
	db *gorm.DB
}

func NewImageRepo(db *gorm.DB) *DBImageRepo {
	return &DBImageRepo{
		db: db,
	}
}

func (r *DBImageRepo) FindOrCreateRepository(repo *image.ContainerRepository) error {
	return r.db.Where(image.ContainerRepository{FullName: repo.FullName}).
		Attrs(image.ContainerRepository{
			Registry:  repo.Registry,
			Namespace: repo.Namespace,
			Name:      repo.Name,
		}).
		FirstOrCreate(repo).Error
}

func (r *DBImageRepo) FindOrCreateTag(tag *image.ContainerTag) error {
	return r.db.Where(image.ContainerTag{RepositoryID: tag.RepositoryID, Name: tag.Name}).
		FirstOrCreate(tag).Error
}

func (r *DBImageRepo) GetTagByDigest(repoID uint, digest string) (*image.ContainerTag, error) {
	var tag image.ContainerTag
	err := r.db.Where("repository_id = ? AND digest = ?", repoID, digest).First(&tag).Error
	return &tag, err
}

func (r *DBImageRepo) CreateRequest(req *image.ImageRequest) error {
	return r.db.Create(req).Error
}

func (r *DBImageRepo) FindRequestByID(id uint) (*image.ImageRequest, error) {
	var req image.ImageRequest
	err := r.db.First(&req, id).Error
	return &req, err
}

func (r *DBImageRepo) ListRequests(projectID *uint, status string) ([]image.ImageRequest, error) {
	var reqs []image.ImageRequest
	query := r.db.Model(&image.ImageRequest{})

	if projectID != nil {
		query = query.Where("project_id = ?", projectID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&reqs).Error
	return reqs, err
}

func (r *DBImageRepo) UpdateRequest(req *image.ImageRequest) error {
	return r.db.Save(req).Error
}

func (r *DBImageRepo) CreateAllowListRule(rule *image.ImageAllowList) error {
	return r.db.Create(rule).Error
}

func (r *DBImageRepo) ListAllowedImages(projectID *uint) ([]image.ImageAllowList, error) {
	var rules []image.ImageAllowList
	query := r.db.Preload("Repository").Preload("Tag").Where("is_enabled = ?", true)

	if projectID != nil {
		query = query.Where("project_id = ? OR project_id IS NULL", projectID)
	}

	err := query.Order("id DESC").Find(&rules).Error
	return rules, err
}

func (r *DBImageRepo) CheckImageAllowed(projectID *uint, repoFullName string, tagName string) (bool, error) {
	var count int64
	query := r.db.Model(&image.ImageAllowList{}).
		Joins("JOIN container_repositories r ON r.id = image_allow_lists.repository_id").
		Joins("LEFT JOIN container_tags t ON t.id = image_allow_lists.tag_id").
		Where("r.full_name = ?", repoFullName).
		Where("image_allow_lists.is_enabled = ?", true)

	if projectID != nil {
		query = query.Where("(image_allow_lists.project_id = ? OR image_allow_lists.project_id IS NULL)", *projectID)
	} else {
		query = query.Where("image_allow_lists.project_id IS NULL")
	}

	query = query.Where("(t.name = ? OR image_allow_lists.tag_id IS NULL)", tagName)

	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DBImageRepo) DisableAllowListRule(id uint) error {
	return r.db.Model(&image.ImageAllowList{}).Where("id = ?", id).Update("is_enabled", false).Error
}

func (r *DBImageRepo) UpdateClusterStatus(status *image.ClusterImageStatus) error {
	var existing image.ClusterImageStatus
	err := r.db.Where("tag_id = ?", status.TagID).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(status).Error
	}

	status.ID = existing.ID
	status.CreatedAt = existing.CreatedAt
	return r.db.Save(status).Error
}

func (r *DBImageRepo) GetClusterStatus(tagID uint) (*image.ClusterImageStatus, error) {
	var status image.ClusterImageStatus
	err := r.db.Where("tag_id = ?", tagID).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *DBImageRepo) FindAllowListRule(projectID *uint, repoFullName, tagName string) (*image.ImageAllowList, error) {
	var rule image.ImageAllowList
	query := r.db.Preload("Repository").Preload("Tag").
		Joins("JOIN container_repositories r ON r.id = image_allow_lists.repository_id").
		Joins("LEFT JOIN container_tags t ON t.id = image_allow_lists.tag_id").
		Where("r.full_name = ?", repoFullName).
		Where("image_allow_lists.is_enabled = ?", true)

	if projectID != nil {
		query = query.Where("(image_allow_lists.project_id = ? OR image_allow_lists.project_id IS NULL)", projectID)
	} else {
		query = query.Where("image_allow_lists.project_id IS NULL")
	}

	query = query.Where("(t.name = ? OR image_allow_lists.tag_id IS NULL)", tagName)

	err := query.First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *DBImageRepo) WithTx(tx *gorm.DB) image.Repository {
	if tx == nil {
		return r
	}
	return &DBImageRepo{
		db: tx,
	}
}
