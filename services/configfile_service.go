package repositories

func CreateConfigFile(cf *models.ConfigFile) error {
    return db.DB.Create(cf).Error
}

func GetConfigFileByID(id uint) (*models.ConfigFile, error) {
    var cf models.ConfigFile
    if err := db.DB.First(&cf, id).Error; err != nil {
        return nil, err
    }
    return &cf, nil
}

func UpdateConfigFile(cf *models.ConfigFile) error {
    if cf.CFID == 0 {
        return errors.New("missing ConfigFile ID")
    }
    return db.DB.Save(cf).Error
}

func DeleteConfigFile(id uint) error {
    return db.DB.Delete(&models.ConfigFile{}, id).Error
}

func ListConfigFiles() ([]models.ConfigFile, error) {
    var list []models.ConfigFile
    if err := db.DB.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func GetConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error) {
    var files []models.ConfigFile
    if err := db.DB.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
        return nil, err
    }
    return files, nil
}