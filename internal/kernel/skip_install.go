package kernel

import (
	"context"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
)

type predefinedUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func skipInstall() {
	logger.Info("Skip installation mode enabled")

	var nodeSecret string

	err := settings.Update(func() {
		if cSettings.AppSettings.JwtSecret == "" {
			cSettings.AppSettings.JwtSecret = uuid.New().String()
		}

		if settings.NodeSettings.Secret == "" {
			nodeSecret = uuid.New().String()
			settings.NodeSettings.Secret = nodeSecret
		}
	})
	if err != nil {
		logger.Fatal(err)
	}

	if nodeSecret != "" {
		logger.Infof("Secret: %s", nodeSecret)
	}
}

func registerPredefinedUser(ctx context.Context) {
	// when skip installation mode is enabled, the predefined user will be created
	if !settings.NodeSettings.SkipInstallation {
		return
	}
	pUser := &predefinedUser{}

	err := env.ParseWithOptions(pUser, env.Options{
		Prefix:                "NGINX_UI_PREDEFINED_USER_",
		UseFieldNameByDefault: true,
	})

	if err != nil {
		logger.Fatal(err)
	}

	name := strings.TrimSpace(pUser.Name)
	password := strings.TrimSpace(pUser.Password)
	if name == "" || password == "" {
		return
	}

	db := model.UseDB()
	if db == nil {
		logger.Error("registerPredefinedUser: database is not initialized")
		return
	}

	// Only initialize when no user has a password yet. InitUser creates a
	// passwordless admin placeholder before this hook runs; that placeholder is
	// still uninstalled and safe to initialize. Never overwrite a real user.
	var initializedUsers int64
	if err := db.Model(&model.User{}).Where("password <> ''").Count(&initializedUsers).Error; err != nil {
		logger.Error(err)
		return
	}
	if initializedUsers > 0 {
		return
	}

	// Create a new user with the predefined name and password
	pwd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(err)
		return
	}

	predefined := &model.User{
		Name:     name,
		Password: string(pwd),
		Status:   true,
	}
	result := db.Model(&model.User{}).Where("id = ?", 1).Updates(predefined)
	if result.Error != nil {
		logger.Error(result.Error)
		return
	}
	if result.RowsAffected > 0 {
		return
	}

	predefined.Model.ID = 1
	if err := db.Create(predefined).Error; err != nil {
		logger.Error(err)
	}
}
