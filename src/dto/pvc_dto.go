package dto

type PVC struct {
	Name      string `json:"name" example:"my-pvc"`
	Namespace string `json:"namespace" example:"default"`
	Status    string `json:"status" example:"Bound"`
	Size      string `json:"size" example:"1Gi"`
	IsGlobal  bool   `json:"isGlobal" example:"false"`
}
