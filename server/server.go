package server

import (
	"petroapp/db"

	"github.com/sirupsen/logrus"
)

type Server struct {
	Database db.DB
	Logger   *logrus.Logger
}
