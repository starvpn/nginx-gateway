package sites

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	internalConfig "github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type basicAuthUser struct {
	Username string `json:"username"`
	Remark   string `json:"remark"`
	hash     string
}

type basicAuthLine struct {
	raw  string
	user *basicAuthUser
}

type basicAuthFileResponse struct {
	Path   string          `json:"path"`
	Exists bool            `json:"exists"`
	Users  []basicAuthUser `json:"users"`
}

type basicAuthPathRequest struct {
	Path string `json:"path" binding:"required"`
}

type createBasicAuthUserRequest struct {
	Path     string `json:"path" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Remark   string `json:"remark"`
}

type updateBasicAuthUserRequest struct {
	Path     string `json:"path" binding:"required"`
	Password string `json:"password"`
	Remark   string `json:"remark"`
}

func resolveBasicAuthFile(path string) (string, error) {
	absPath, err := internalConfig.ResolveAbsoluteOrRelativeConfPath(path)
	if err != nil {
		return "", err
	}

	baseName := strings.ToLower(filepath.Base(absPath))
	if baseName != ".htpasswd" && !strings.HasSuffix(baseName, ".htpasswd") {
		return "", errors.New("password file must be named .htpasswd or end with .htpasswd")
	}

	if stat, err := os.Stat(absPath); err == nil && stat.IsDir() {
		return "", errors.New("password file path cannot be a directory")
	}

	return absPath, nil
}

func validateBasicAuthUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("username is required")
	}

	if !utf8.ValidString(username) || len(username) > 128 || strings.HasPrefix(username, "#") || strings.ContainsAny(username, ":/\r\n") {
		return "", errors.New("username contains invalid characters")
	}

	return username, nil
}

func validateBasicAuthPassword(password string) error {
	if password == "" {
		return errors.New("password is required")
	}

	if !utf8.ValidString(password) || len(password) > 512 {
		return errors.New("password contains invalid characters")
	}

	return nil
}

func validateBasicAuthRemark(remark string) (string, error) {
	remark = strings.TrimSpace(remark)
	if !utf8.ValidString(remark) || len(remark) > 255 || strings.ContainsAny(remark, "\r\n") {
		return "", errors.New("remark contains invalid characters")
	}

	return remark, nil
}

func parseBasicAuthLine(line string) basicAuthLine {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return basicAuthLine{raw: line}
	}

	parts := strings.Split(line, ":")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return basicAuthLine{raw: line}
	}

	return basicAuthLine{user: &basicAuthUser{
		Username: parts[0],
		hash:     parts[1],
		Remark:   strings.Join(parts[2:], ":"),
	}}
}

func readBasicAuthFile(absPath string) ([]basicAuthLine, bool, error) {
	content, err := os.ReadFile(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	lines := make([]basicAuthLine, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, parseBasicAuthLine(scanner.Text()))
	}

	return lines, true, scanner.Err()
}

func formatBasicAuthLine(line basicAuthLine) string {
	if line.user == nil {
		return line.raw
	}

	formatted := line.user.Username + ":" + line.user.hash
	if line.user.Remark != "" {
		formatted += ":" + line.user.Remark
	}

	return formatted
}

func writeBasicAuthFile(absPath string, lines []basicAuthLine) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}

	formattedLines := make([]string, len(lines))
	for i, line := range lines {
		formattedLines[i] = formatBasicAuthLine(line)
	}

	content := strings.Join(formattedLines, "\n")
	if content != "" {
		content += "\n"
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

func hashBasicAuthPassword(password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	digest := append(h.Sum(nil), salt...)

	return "{SSHA}" + base64.StdEncoding.EncodeToString(digest), nil
}

func basicAuthUsersFromLines(lines []basicAuthLine) []basicAuthUser {
	users := make([]basicAuthUser, 0)
	for _, line := range lines {
		if line.user == nil {
			continue
		}

		users = append(users, basicAuthUser{
			Username: line.user.Username,
			Remark:   line.user.Remark,
		})
	}

	return users
}

func respondBasicAuthError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"message": err.Error(),
	})
}

func GetBasicAuthFile(c *gin.Context) {
	absPath, err := resolveBasicAuthFile(c.Query("path"))
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	lines, exists, err := readBasicAuthFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, basicAuthFileResponse{
		Path:   absPath,
		Exists: exists,
		Users:  basicAuthUsersFromLines(lines),
	})
}

func EnsureBasicAuthFile(c *gin.Context) {
	var json basicAuthPathRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	absPath, err := resolveBasicAuthFile(json.Path)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	lines, _, err := readBasicAuthFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err = writeBasicAuthFile(absPath, lines); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, basicAuthFileResponse{
		Path:   absPath,
		Exists: true,
		Users:  basicAuthUsersFromLines(lines),
	})
}

func CreateBasicAuthUser(c *gin.Context) {
	var json createBasicAuthUserRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	absPath, err := resolveBasicAuthFile(json.Path)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	username, err := validateBasicAuthUsername(json.Username)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	if err = validateBasicAuthPassword(json.Password); err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	remark, err := validateBasicAuthRemark(json.Remark)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	lines, _, err := readBasicAuthFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	for _, line := range lines {
		if line.user != nil && line.user.Username == username {
			respondBasicAuthError(c, http.StatusConflict, errors.New("username already exists"))
			return
		}
	}

	hashedPassword, err := hashBasicAuthPassword(json.Password)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	lines = append(lines, basicAuthLine{user: &basicAuthUser{
		Username: username,
		hash:     hashedPassword,
		Remark:   remark,
	}})

	if err = writeBasicAuthFile(absPath, lines); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, basicAuthUser{
		Username: username,
		Remark:   remark,
	})
}

func UpdateBasicAuthUser(c *gin.Context) {
	var json updateBasicAuthUserRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	absPath, err := resolveBasicAuthFile(json.Path)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	username, err := validateBasicAuthUsername(c.Param("username"))
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	if json.Password != "" {
		if err = validateBasicAuthPassword(json.Password); err != nil {
			respondBasicAuthError(c, http.StatusBadRequest, err)
			return
		}
	}

	remark, err := validateBasicAuthRemark(json.Remark)
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	lines, _, err := readBasicAuthFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	found := false
	for _, line := range lines {
		if line.user == nil || line.user.Username != username {
			continue
		}

		found = true
		line.user.Remark = remark
		if json.Password != "" {
			line.user.hash, err = hashBasicAuthPassword(json.Password)
			if err != nil {
				cosy.ErrHandler(c, err)
				return
			}
		}
	}

	if !found {
		respondBasicAuthError(c, http.StatusNotFound, errors.New("username not found"))
		return
	}

	if err = writeBasicAuthFile(absPath, lines); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, basicAuthUser{
		Username: username,
		Remark:   remark,
	})
}

func DeleteBasicAuthUser(c *gin.Context) {
	absPath, err := resolveBasicAuthFile(c.Query("path"))
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	username, err := validateBasicAuthUsername(c.Param("username"))
	if err != nil {
		respondBasicAuthError(c, http.StatusBadRequest, err)
		return
	}

	lines, _, err := readBasicAuthFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	nextLines := make([]basicAuthLine, 0, len(lines))
	found := false
	for _, line := range lines {
		if line.user != nil && line.user.Username == username {
			found = true
			continue
		}

		nextLines = append(nextLines, line)
	}

	if !found {
		respondBasicAuthError(c, http.StatusNotFound, errors.New("username not found"))
		return
	}

	if err = writeBasicAuthFile(absPath, nextLines); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
