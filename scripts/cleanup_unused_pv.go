package main

import (
	"fmt"
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
	"os"
	"time"
)

// 設定閒置天數
const unusedDays = 90

func main() {
	db.Init()
	repos := repositories.NewRepos()
	users, err := repos.User.GetAllUsers()
	if err != nil {
		fmt.Println("[ERROR] 取得 user 失敗:", err)
		os.Exit(1)
	}
	now := time.Now()
	for _, user := range users {
		projects, _ := repos.View.ListProjectsByUserID(user.UID)
		if len(projects) > 0 {
			continue // 有 project 不清理
		}
		if user.UpdatedAt.Add(unusedDays * 24 * time.Hour).After(now) {
			continue // 近期有活動不清理
		}
		pvName := "pv-user-" + user.Username
		pvcName := "pvc-user-" + user.Username
		// TODO: 檢查 PV/PVC 是否存在，存在則刪除
		errPV := utils.DeletePV(pvName)
		errPVC := utils.DeletePVC("default", pvcName)
		if errPV == nil && errPVC == nil {
			fmt.Printf("[CLEANUP] 已刪除 user %s 的 PV/PVC\n", user.Username)
		}
	}
}
