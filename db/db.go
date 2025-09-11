package db

import (
	"github.com/danglnh07/TaskManagement/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Queries struct {
	DB     *gorm.DB
	config *util.Config
}

func NewQueries(config *util.Config) *Queries {
	return &Queries{
		config: config,
	}
}

func (q *Queries) ConnectDB() error {
	var err error
	q.DB, err = gorm.Open(postgres.Open(q.config.DBConn), &gorm.Config{})
	return err
}

func (q *Queries) AutoMigration() error {
	return q.DB.AutoMigrate(&Account{}, &Task{})
}
