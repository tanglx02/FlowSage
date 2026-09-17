package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// APIInventory 是一次流量分析落地后的接口清单记录。
//
// Payload 存放 apianalyze.Inventory 的完整 JSON，列表页只用摘要字段，
// 详情页再解析 Payload，避免列表查询把大字段一起读出来。
type APIInventory struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id,omitempty"`
	Name          string    `json:"name"`
	Source        string    `json:"source"`
	Host          string    `json:"host,omitempty"`
	RecordCount   int       `json:"record_count"`
	APICount      int       `json:"api_count"`
	EndpointCount int       `json:"endpoint_count"`
	Payload       string    `json:"payload,omitempty"`
	OwnerUserID   string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateAPIInventory 写入一份接口清单。
func (db *DB) CreateAPIInventory(inv *APIInventory) (*APIInventory, error) {
	name := strings.TrimSpace(inv.Name)
	if name == "" {
		return nil, fmt.Errorf("清单名称不能为空")
	}
	id := uuid.New().String()
	now := time.Now()
	if _, err := db.Exec(
		`INSERT INTO api_inventories
			(id, project_id, name, source, host, record_count, api_count, endpoint_count, payload, owner_user_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, inv.ProjectID, name, inv.Source, inv.Host,
		inv.RecordCount, inv.APICount, inv.EndpointCount, inv.Payload, inv.OwnerUserID, now, now,
	); err != nil {
		return nil, fmt.Errorf("保存接口清单失败: %w", err)
	}
	inv.ID = id
	inv.Name = name
	inv.CreatedAt = now
	inv.UpdatedAt = now
	return inv, nil
}

// ListAPIInventories 列出接口清单（不含 Payload）。ownerUserID 为空表示不过滤归属。
func (db *DB) ListAPIInventories(projectID, ownerUserID string, limit, offset int) ([]*APIInventory, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	where := []string{"1 = 1"}
	args := []any{}
	if strings.TrimSpace(projectID) != "" {
		where = append(where, "project_id = ?")
		args = append(args, strings.TrimSpace(projectID))
	}
	if strings.TrimSpace(ownerUserID) != "" {
		where = append(where, "(owner_user_id = ? OR owner_user_id = '')")
		args = append(args, strings.TrimSpace(ownerUserID))
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM api_inventories WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计接口清单失败: %w", err)
	}

	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.Query(
		`SELECT id, COALESCE(project_id, ''), name, COALESCE(source, ''), COALESCE(host, ''),
		        record_count, api_count, endpoint_count, COALESCE(owner_user_id, ''), created_at, updated_at
		 FROM api_inventories WHERE `+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询接口清单失败: %w", err)
	}
	defer rows.Close()

	list := make([]*APIInventory, 0, limit)
	for rows.Next() {
		item := &APIInventory{}
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Source, &item.Host,
			&item.RecordCount, &item.APICount, &item.EndpointCount, &item.OwnerUserID,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("读取接口清单失败: %w", err)
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// GetAPIInventory 读取单份接口清单（含 Payload）。
func (db *DB) GetAPIInventory(id string) (*APIInventory, error) {
	item := &APIInventory{}
	err := db.QueryRow(
		`SELECT id, COALESCE(project_id, ''), name, COALESCE(source, ''), COALESCE(host, ''),
		        record_count, api_count, endpoint_count, COALESCE(payload, ''), COALESCE(owner_user_id, ''),
		        created_at, updated_at
		 FROM api_inventories WHERE id = ?`, id,
	).Scan(&item.ID, &item.ProjectID, &item.Name, &item.Source, &item.Host,
		&item.RecordCount, &item.APICount, &item.EndpointCount, &item.Payload, &item.OwnerUserID,
		&item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取接口清单失败: %w", err)
	}
	return item, nil
}

// DeleteAPIInventory 删除一份接口清单。
func (db *DB) DeleteAPIInventory(id string) error {
	if _, err := db.Exec("DELETE FROM api_inventories WHERE id = ?", id); err != nil {
		return fmt.Errorf("删除接口清单失败: %w", err)
	}
	return nil
}