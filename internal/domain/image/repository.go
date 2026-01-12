package image

import "gorm.io/gorm"

type Repository interface {
	FindOrCreateRepository(repo *ContainerRepository) error
	FindOrCreateTag(tag *ContainerTag) error
	GetTagByDigest(repoID uint, digest string) (*ContainerTag, error)

	CreateRequest(req *ImageRequest) error
	FindRequestByID(id uint) (*ImageRequest, error)
	ListRequests(projectID *uint, status string) ([]ImageRequest, error)
	UpdateRequest(req *ImageRequest) error

	CreateAllowListRule(rule *ImageAllowList) error
	ListAllowedImages(projectID *uint) ([]ImageAllowList, error)
	FindAllowListRule(projectID *uint, repoFullName, tagName string) (*ImageAllowList, error)
	CheckImageAllowed(projectID *uint, repoFullName string, tagName string) (bool, error)
	DisableAllowListRule(id uint) error

	UpdateClusterStatus(status *ClusterImageStatus) error
	GetClusterStatus(tagID uint) (*ClusterImageStatus, error)

	WithTx(tx *gorm.DB) Repository
}
