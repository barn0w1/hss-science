package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitHub Actionsのマトリックスに渡す構造体
type BackendService struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Build string `json:"build"`
}

// 全サービスの一覧（一元管理）
var allBackendServices = []BackendService{
	{Name: "identity-service", Path: "server/services/identity-service", Build: "./services/identity-service/..."},
	{Name: "myaccount-bff", Path: "server/services/myaccount-bff", Build: "./services/myaccount-bff/..."},
	{Name: "blob-service", Path: "server/services/blob-service", Build: "./services/blob-service/..."},
}

func main() {
	// 1. 変更されたファイルのリストを取得
	// PRの場合は target branch (main) との差分を取る
	baseRef := os.Getenv("GITHUB_BASE_REF")
	if baseRef == "" {
		baseRef = "main" // pushイベントなどのフォールバック
	}

	cmd := exec.Command("git", "diff", "--name-only", fmt.Sprintf("origin/%s...HEAD", baseRef))
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running git diff: %v\n", err)
		os.Exit(1)
	}

	changedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")

	// 2. 変更影響を解析
	needsAll := false
	changedServices := make(map[string]bool)

	for _, file := range changedFiles {
		// 共通部分が変更されたら全サービスをCI対象にする
		if strings.HasPrefix(file, "server/internal/") ||
			strings.HasPrefix(file, "api/proto/") ||
			file == "server/go.mod" || file == "server/go.sum" {
			needsAll = true
			break
		}

		// 個別サービスの変更を検知
		for _, svc := range allBackendServices {
			if strings.HasPrefix(file, svc.Path) {
				changedServices[svc.Name] = true
			}
		}
	}

	// 3. 実行対象のリストを作成
	var targetServices []BackendService
	if needsAll {
		targetServices = allBackendServices
	} else {
		for _, svc := range allBackendServices {
			if changedServices[svc.Name] {
				targetServices = append(targetServices, svc)
			}
		}
	}

	// 4. GitHub ActionsのOutputにJSONを出力
	matrixJSON, _ := json.Marshal(targetServices)
	fmt.Printf("Generated Matrix: %s\n", string(matrixJSON)) // ログ確認用

	// GITHUB_OUTPUTへの書き込み
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(fmt.Sprintf("backend_matrix=%s\n", string(matrixJSON)))
		}
	}
}
