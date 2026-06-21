// Package domain 定义所有核心领域模型。
// 本文件包含用户实体，用于认证与授权。
package domain

import "time"

// User 用户实体，存储在 users 表中，用于登录和 JWT 签发。
// 注意：此实体直接对应数据库行，密码字段存储 bcrypt 哈希值。
type User struct {
	Id       int64     // 用户 ID（自增主键）
	Email    string    // 邮箱（唯一，用于登录）
	UserName string    // 用户名（显示名）
	Password string    // bcrypt 哈希密码
	Ctime    time.Time // 创建时间
}