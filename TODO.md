# TODOs

## Completed ✅

- [x] Project-level image management implementation
  - Updated AllowedImage model with ProjectID and IsGlobal fields
  - Project managers can add images to their projects
  - Admin-approved images are global and visible to all
  - Image validation on job creation and pod deployment
  
### Image System Architecture
```
Global Images (IsGlobal=true):
  - Approved by admin
  - Visible to all projects
  - Can be used in any job/pod

Project Images (IsGlobal=false, ProjectID set):
  - Added by project managers
  - Only visible to that project
  - Can be used in that project's jobs/pods

Validation Points:
  1. Job Creation (K8sService.CreateJob)
     - Validates image before creating K8s job
     - Checks global images + project-specific images
     
  2. ConfigFile Instance (ConfigFileService.CreateInstance)
     - Parses YAML to extract container images
     - Validates each image before deployment
     - Supports Pods, Deployments, etc.
```

### API Endpoints
```
GET  /images/allowed?project_id={id}  - List allowed images (global + project)
POST /projects/:project_id/images     - Add image to project (manager+)
POST /image-requests                   - Request new image
PUT  /image-requests/:id/approve      - Approve request (admin)
```

### Database Schema
```sql
allowed_images:
  - name, tag: image identifier
  - project_id: NULL for global, set for project-scoped
  - is_global: true for admin-approved global images
  - created_by: user who added the image
```

## Previous Completions

- [x] Harbor allowlist API design and implementation
- [x] Frontend job form image dropdown integration  
- [x] Form message/comment support (backend & frontend)
- [x] Database migrations for all new models

## Next Steps (Optional Enhancements)

- [ ] Add UI for project managers to manage project images
  - Project settings page with image management tab
  - Add/remove images from project allowlist
  - View global images available to project

- [ ] Add bulk image import
  - Allow uploading a list of images
  - Batch validation and approval workflow

- [ ] Add image scanning/security checks
  - Integration with vulnerability scanning tools
  - Block images with critical vulnerabilities

- [ ] Add Harbor registry sync
  - Auto-sync available images from Harbor
  - Show only images that exist in registry