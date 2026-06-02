package logic

import (
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"gorm.io/gorm"
)

type logic struct {
}

func (l logic) GetDefaultDb() *gorm.DB {
	db, _ := facade.GetDbFactory().Channel("default")
	return db
}

func (l logic) RebuildImageName(imageName string, namespacePrefix string) string {
	if namespacePrefix == "" {
		return imageName
	}
	if _, err := digest.Parse(imageName); err == nil {
		return imageName
	}

	namespacePrefix = strings.TrimRight(namespacePrefix, "/")
	imageName = strings.TrimLeft(imageName, "/")

	return namespacePrefix + "/" + imageName
}
