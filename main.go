package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/hoshinonyaruko/acgv3-safeguard/config"
)

func main() {
	// 配置文件路径
	const configPath = "config.yml"

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 连接到 MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/",
		cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Address)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("无法连接到 MySQL 数据库: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("无法访问 MySQL 数据库: %v", err)
	}

	// MySQL 连接成功
	fmt.Println("成功连接到 MySQL 数据库")

	// 启动文件复写逻辑
	if cfg.Paths.PayOverrideSource != "" && cfg.Paths.PayOverrideTarget != "" &&
		cfg.Paths.PluginOverrideSource != "" && cfg.Paths.PluginOverrideTarget != "" {
		go startFileOverrideRoutine(cfg.Paths.PayOverrideSource, cfg.Paths.PayOverrideTarget)
		go startFileOverrideRoutine(cfg.Paths.PluginOverrideSource, cfg.Paths.PluginOverrideTarget)
		fmt.Println("成功开启文件复写")
	} else {
		log.Println("复写目录配置未完成，跳过文件复写逻辑")
	}

	// 启动 MySQL 表保护逻辑
	if cfg.Protection.AdminTable {
		go protectAdminTable(cfg)
	}

	// 启动 PaymentTable 的保护逻辑
	if cfg.Protection.PaymentTable {
		go protectPaymentTable(cfg)
	}

	select {} // 阻塞主线程，保持程序运行
}

// startFileOverrideRoutine 启动文件复写的协程
func startFileOverrideRoutine(sourceDir, targetDir string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		err := syncFiles(sourceDir, targetDir)
		if err != nil {
			log.Printf("文件同步错误: %v", err)
		}
	}
}

func syncFiles(sourceDir, targetDir string) error {
	sourceFiles, err := listFiles(sourceDir)
	if err != nil {
		return fmt.Errorf("无法列出源文件夹: %w", err)
	}

	targetFiles, err := listFiles(targetDir)
	if err != nil {
		return fmt.Errorf("无法列出目标文件夹: %w", err)
	}

	sourceFileMap := make(map[string]struct{})
	var unchangedCount, deletedCount, totalFiles int

	// 同步文件
	for _, srcFile := range sourceFiles {
		relPath, err := filepath.Rel(sourceDir, srcFile)
		if err != nil {
			return fmt.Errorf("无法计算文件的相对路径 %s: %w", srcFile, err)
		}

		targetPath := filepath.Join(targetDir, relPath)
		totalFiles++

		if shouldCopyFile(srcFile, targetPath) {
			log.Printf("正在同步文件 %s 到 %s", srcFile, targetPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
				return fmt.Errorf("无法创建目标文件夹 %s: %w", filepath.Dir(targetPath), err)
			}
			if err := copyFile(srcFile, targetPath); err != nil {
				return fmt.Errorf("无法复制文件 %s: %w", srcFile, err)
			}
		} else {
			unchangedCount++
		}

		sourceFileMap[relPath] = struct{}{}
	}

	// 删除多余文件
	for _, tgtFile := range targetFiles {
		relPath, err := filepath.Rel(targetDir, tgtFile)
		if err != nil {
			return fmt.Errorf("无法计算目标文件的相对路径 %s: %w", tgtFile, err)
		}

		if _, exists := sourceFileMap[relPath]; !exists {
			log.Printf("目标文件 %s 在源目录中不存在，准备删除", tgtFile)
			if err := os.Remove(tgtFile); err != nil {
				return fmt.Errorf("无法删除多余文件 %s: %w", tgtFile, err)
			}
			deletedCount++
		}
	}

	log.Printf("同步完成，总文件数：%d，未改变文件数：%d，删除文件数：%d", totalFiles, unchangedCount, deletedCount)
	return nil
}

// shouldCopyFile 使用 MD5 哈希值判断文件是否需要复制
func shouldCopyFile(srcPath, tgtPath string) bool {
	tgtInfo, err := os.Stat(tgtPath)
	if os.IsNotExist(err) {
		return true // 目标文件不存在，需要复制
	}
	if err != nil {
		log.Printf("无法获取目标文件信息: %v", err)
		return true
	}

	// 比较文件大小
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		log.Printf("无法获取源文件信息: %v", err)
		return true
	}
	if srcInfo.Size() != tgtInfo.Size() {
		return true // 文件大小不一致，需要复制
	}

	// 比较文件内容哈希
	srcHash, err := calculateFileHash(srcPath)
	if err != nil {
		log.Printf("无法计算源文件哈希: %v", err)
		return true
	}
	tgtHash, err := calculateFileHash(tgtPath)
	if err != nil {
		log.Printf("无法计算目标文件哈希: %v", err)
		return true
	}

	return srcHash != tgtHash // 只有内容不一致时才需要复制
}

// calculateFileHash 计算文件的 MD5 哈希值
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("无法计算文件哈希: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// listFiles 递归列出目录及其子目录中的所有文件
func listFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("无法访问路径 %s: %w", path, err)
		}
		// 如果是文件（非目录），加入列表
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// copyFile 复制文件，从 sourcePath 到 targetPath
func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("无法打开源文件: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("无法创建目标文件: %w", err)
	}
	defer targetFile.Close()

	if _, err := sourceFile.Seek(0, 0); err != nil {
		return fmt.Errorf("无法重置源文件指针: %w", err)
	}
	if _, err := targetFile.ReadFrom(sourceFile); err != nil {
		return fmt.Errorf("无法写入目标文件: %w", err)
	}

	return nil
}

// protectAdminTable 周期性删除 acg_manage 表中 id != 1 的行，并记录日志
func protectAdminTable(cfg *config.Config) {
	// 配置日志
	logFile, err := os.OpenFile("acg_manage_cleanup.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法创建日志文件: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	// 数据库连接配置
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True",
		cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Address, "faka") // 替换 "faka" 为实际数据库名称

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	for {
		// 查询 `id != 1` 的所有行
		rows, err := db.Query("SELECT * FROM acg_manage WHERE id != 1;")
		if err != nil {
			logger.Printf("查询失败: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		var rowsToDelete []map[string]interface{}
		cols, _ := rows.Columns()
		for rows.Next() {
			row := make(map[string]interface{})
			columnPointers := make([]interface{}, len(cols))
			for i := range columnPointers {
				columnPointers[i] = new(interface{})
			}

			if err := rows.Scan(columnPointers...); err != nil {
				logger.Printf("扫描行失败: %v", err)
				continue
			}

			for i, colName := range cols {
				row[colName] = *(columnPointers[i].(*interface{}))
			}
			rowsToDelete = append(rowsToDelete, row)
		}
		rows.Close()

		if len(rowsToDelete) > 0 {
			// 记录删除的行
			logDeletion(rowsToDelete, logger)

			// 删除 `id != 1` 的行
			_, err := db.Exec("DELETE FROM acg_manage WHERE id != 1;")
			if err != nil {
				logger.Printf("删除失败: %v", err)
			} else {
				logger.Printf("成功删除 %d 行", len(rowsToDelete))
			}
		} else {
			logger.Println("未发现新增管理员")
		}

		time.Sleep(5 * time.Second)
	}
}

// logDeletion 将删除的行记录到日志
func logDeletion(deletedRows []map[string]interface{}, logger *log.Logger) {
	for _, row := range deletedRows {
		rowJSON, err := json.Marshal(row)
		if err != nil {
			logger.Printf("无法序列化行数据: %v", err)
			continue
		}
		logger.Printf("删除的行: %s", rowJSON)
	}
}

// protectPaymentTable 保护 acg_pay 表，防止新增、删除、修改
func protectPaymentTable(cfg *config.Config) {
	// 配置日志
	logFile, err := os.OpenFile("acg_pay_protection.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法创建日志文件: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	// 数据库连接配置
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True",
		cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Address, "faka") // 替换 "faka" 为实际数据库名称

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	// 获取初始状态
	initialState, err := fetchTableState(db, "acg_pay")
	if err != nil {
		logger.Fatalf("获取初始状态失败: %v", err)
	}
	logger.Println("成功加载初始状态")

	for {
		currentState, err := fetchTableState(db, "acg_pay")
		if err != nil {
			logger.Printf("获取当前状态失败: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// 检查和还原表状态
		err = restoreTableState(db, "acg_pay", initialState, currentState, logger)
		if err != nil {
			logger.Printf("还原表状态失败: %v", err)
		}

		time.Sleep(5 * time.Second)
	}
}

// fetchTableState 获取表的完整状态
func fetchTableState(db *sql.DB, tableName string) ([]map[string]interface{}, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s;", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var state []map[string]interface{}

	for rows.Next() {
		row := make(map[string]interface{})
		columnPointers := make([]interface{}, len(cols))
		for i := range columnPointers {
			columnPointers[i] = new(interface{})
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		for i, colName := range cols {
			row[colName] = *(columnPointers[i].(*interface{}))
		}
		state = append(state, row)
	}
	return state, nil
}

func restoreTableState(db *sql.DB, tableName string, initialState, currentState []map[string]interface{}, logger *log.Logger) error {
	initialMap := sliceToMap(initialState)
	currentMap := sliceToMap(currentState)

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("启动事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// 检查新增记录
	for key, currentRow := range currentMap {
		if _, exists := initialMap[key]; !exists {
			// 删除新增记录
			logger.Printf("发现新增记录: %v", currentRow)
			query, params := generateDeleteQuery(tableName, currentRow)
			if _, err := tx.Exec(query, params...); err != nil {
				return fmt.Errorf("删除新增记录失败: %w", err)
			}
		}
	}

	// 检查删除记录
	for key, initialRow := range initialMap {
		if _, exists := currentMap[key]; !exists {
			// 还原删除记录
			query, params := generateInsertQuery(tableName, initialRow)
			logger.Printf("还原删除记录: %v", initialRow)
			if _, err := tx.Exec(query, params...); err != nil {
				return fmt.Errorf("还原删除记录失败: %w", err)
			}
		}
	}

	// 检查修改记录
	for key, initialRow := range initialMap {
		if currentRow, exists := currentMap[key]; exists {
			if !isRowEqual(initialRow, currentRow) {
				// 还原修改记录
				logger.Printf("发现被修改的记录: %v", currentRow)
				query, params := generateUpdateQuery(tableName, initialRow)
				if _, err := tx.Exec(query, params...); err != nil {
					return fmt.Errorf("还原修改记录失败: %w", err)
				}
			}
		}
	}

	return nil
}

func generateDeleteQuery(tableName string, row map[string]interface{}) (string, []interface{}) {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?;", tableName)
	return query, []interface{}{row["id"]}
}

func generateInsertQuery(tableName string, row map[string]interface{}) (string, []interface{}) {
	columns := ""
	placeholders := ""
	values := []interface{}{}
	for col, val := range row {
		columns += fmt.Sprintf("%s, ", col)
		placeholders += "?, "
		values = append(values, val)
	}
	columns = columns[:len(columns)-2]
	placeholders = placeholders[:len(placeholders)-2]
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", tableName, columns, placeholders)
	return query, values
}

func generateUpdateQuery(tableName string, row map[string]interface{}) (string, []interface{}) {
	query := fmt.Sprintf("UPDATE %s SET ", tableName)
	params := []interface{}{}
	for col, val := range row {
		query += fmt.Sprintf("%s = ?, ", col)
		params = append(params, val)
	}
	query = query[:len(query)-2]
	query += " WHERE id = ?;"
	params = append(params, row["id"])
	return query, params
}

func isRowEqual(row1, row2 map[string]interface{}) bool {
	for k, v1 := range row1 {
		if v2, ok := row2[k]; !ok || !reflect.DeepEqual(v1, v2) {
			return false
		}
	}
	return true
}

// sliceToMap 将表状态转换为以 id 为键的 map
func sliceToMap(slice []map[string]interface{}) map[interface{}]map[string]interface{} {
	result := make(map[interface{}]map[string]interface{})
	for _, row := range slice {
		result[row["id"]] = row
	}
	return result
}
