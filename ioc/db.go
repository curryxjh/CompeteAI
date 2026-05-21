package ioc

import (
	"CompeteAI/internal/repository/dao"
	"CompeteAI/settings"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func InitDB() *gorm.DB {
	cfg := settings.Conf.MySQLConfig
	if cfg == nil {
		panic("mysql config is nil, check config file")
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Info),
	})
	if err != nil {
		panic(fmt.Errorf("open mysql: %w", err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Errorf("get sql.DB: %w", err))
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if err := dao.InitTable(db); err != nil {
		panic(err)
	}
	return db
}
