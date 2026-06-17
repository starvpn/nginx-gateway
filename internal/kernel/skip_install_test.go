package kernel

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPredefinedUserTest(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	originalSkipInstallation := settings.NodeSettings.SkipInstallation
	t.Cleanup(func() {
		settings.NodeSettings.SkipInstallation = originalSkipInstallation
	})

	settings.NodeSettings.SkipInstallation = true
	t.Setenv("NGINX_UI_PREDEFINED_USER_NAME", "dev-admin")
	t.Setenv("NGINX_UI_PREDEFINED_USER_PASSWORD", "dev-password")

	return db
}

func TestRegisterPredefinedUserInitializesBlankAdminUser(t *testing.T) {
	db := setupPredefinedUserTest(t)
	require.NoError(t, db.Create(&model.User{
		Model: model.Model{ID: 1},
		Name:  "admin",
	}).Error)

	registerPredefinedUser(t.Context())

	var got model.User
	require.NoError(t, db.First(&got, 1).Error)
	require.Equal(t, "dev-admin", got.Name)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("dev-password")))
	require.True(t, got.Status)
}

func TestRegisterPredefinedUserDoesNotOverwriteInitializedUser(t *testing.T) {
	db := setupPredefinedUserTest(t)
	existingHash, err := bcrypt.GenerateFromPassword([]byte("existing-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Model:    model.Model{ID: 1},
		Name:     "existing-admin",
		Password: string(existingHash),
		Status:   true,
	}).Error)

	registerPredefinedUser(t.Context())

	var got model.User
	require.NoError(t, db.First(&got, 1).Error)
	require.Equal(t, "existing-admin", got.Name)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("existing-password")))
}
