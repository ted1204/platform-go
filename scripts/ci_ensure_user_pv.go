package main

import (
	"fmt"
	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/services"
	"os"
)

func main() {
	// 初始化 DB 連線
	db.Init()
	repos := repositories.NewRepos()
	userService := services.NewUserService(repos)
	count, err := userService.EnsureAllUserPV()
	if err != nil {
		fmt.Println("[ERROR] 補齊 user PV 失敗:", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] 已補齊 PV/PVC 數量: %d\n", count)
}
